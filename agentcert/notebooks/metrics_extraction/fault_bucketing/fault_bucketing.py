"""
Fault Bucketing Pipeline for Multi-Fault Langfuse Traces.

Implements the Log Preprocessing & Fault Bucketing algorithm described in
Section 1.4 of the AgentCert Methodologies wiki. Streams Langfuse trace events
in chronological order, identifies fault lifecycle phases (detection →
investigation → remediation → verification → confirmation), and uses an LLM
classifier to assign interleaved events into per-fault buckets.

Supports both multi-fault traces (multiple fault_detected events) and
single-fault traces (creates one pass-through bucket).

Output: per-fault JSON files for downstream metrics extraction.
"""

import argparse
import asyncio
import json
import logging
import sys
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Literal, Optional

from pydantic import BaseModel, Field

# Optional imports — gracefully handle if not available
try:
    from utils.azure_openai_util import AzureLLMClient
    from utils.load_config import ConfigLoader
    from utils.setup_logging import logger
except ImportError:
    AzureLLMClient = None
    ConfigLoader = None
    logger = logging.getLogger(__name__)
    logging.basicConfig(level=logging.INFO)


# ---------------------------------------------------------------------------
# 1. Pydantic Models
# ---------------------------------------------------------------------------

class EventClassification(BaseModel):
    """LLM classifier output for a single trace event."""

    event_id: str = Field(description="The unique identifier of the event being classified.")
    related_faults: List[str] = Field(
        description="One or more fault IDs this event relates to. "
        "A single event can apply to multiple faults."
    )
    fault_detected: Optional[str] = Field(
        default=None,
        description="If this event represents the agent detecting a fault, "
        "which fault_id was detected. Null otherwise.",
    )
    fault_mitigated: Optional[str] = Field(
        default=None,
        description="If this event represents the agent mitigating a fault, "
        "which fault_id was mitigated. Null otherwise.",
    )
    has_quantitative_value: bool = Field(
        default=False,
        description="Whether the event contains a measurable numeric value "
        "(latency, count, threshold, etc.).",
    )
    has_qualitative_value: bool = Field(
        default=False,
        description="Whether the event contains a subjective or descriptive "
        "assessment (severity label, root-cause hypothesis, etc.).",
    )
    has_cost_token_details: bool = Field(
        default=False,
        description="Whether the event contains LLM cost or token usage information.",
    )
    confidence: float = Field(
        default=0.0,
        description="Confidence score (0-1) for the classification.",
    )


class BatchClassificationResult(BaseModel):
    """Wrapper for a batch of LLM-classified events."""

    classifications: List[EventClassification] = Field(
        description="List of per-event classification results."
    )


@dataclass
class FaultBucket:
    """Container for events related to a single fault lifecycle."""

    fault_id: str
    fault_name: str
    severity: Optional[str] = None
    target_pod: Optional[str] = None
    namespace: Optional[str] = None
    detection_signals: List[str] = field(default_factory=list)
    events: List[Dict[str, Any]] = field(default_factory=list)
    status: str = "active"  # "active" or "closed"
    detected_at: Optional[str] = None
    mitigated_at: Optional[str] = None

    def to_dict(self) -> Dict[str, Any]:
        """Serialize the bucket metadata and events."""
        return {
            "fault_id": self.fault_id,
            "fault_name": self.fault_name,
            "severity": self.severity,
            "target_pod": self.target_pod,
            "namespace": self.namespace,
            "detection_signals": self.detection_signals,
            "status": self.status,
            "detected_at": self.detected_at,
            "mitigated_at": self.mitigated_at,
            "event_count": len(self.events),
            "events": self.events,
        }


# ---------------------------------------------------------------------------
# 2. LLM Classifier System Prompt
# ---------------------------------------------------------------------------

FAULT_CLASSIFIER_SYSTEM_PROMPT = """You are an expert fault-event classifier for IT-Ops agent traces.

You will receive:
1. A list of currently **active faults** with their IDs, names, descriptions, and target resources.
2. A **batch of trace events** (tool calls, LLM generations, agent actions) with their IDs, timestamps, and content.

Your task is to classify **each event** in the batch against the active faults.

## Classification Rules

- **related_faults**: Assign each event to one or more fault IDs it relates to. An event
  (e.g. a shared diagnostic log-parsing step) can relate to multiple faults.
- **fault_detected**: Set to the fault_id ONLY if this event represents the agent
  **first recognizing** that a specific fault exists. Null otherwise.
- **fault_mitigated**: Set to the fault_id ONLY if this event represents the agent
  **completing remediation** for a fault (e.g. successful fix confirmation). Null otherwise.
- **has_quantitative_value**: True if the event contains measurable numeric data
  (latency, restart count, disk usage percentage, memory bytes, etc.).
- **has_qualitative_value**: True if the event contains subjective assessments
  (severity labels, root-cause hypotheses, reasoning narratives, etc.).
- **has_cost_token_details**: True if the event contains LLM token counts or cost info.
- **confidence**: Your confidence in the classification (0.0 to 1.0).

## Context Clues

- Events with names containing a fault name (e.g. "investigate:disk-fill", "remediate:pod-delete")
  clearly belong to that fault.
- Events under a parent span (parentObservationId) typically relate to the same fault as the parent.
- GENERATION spans contain LLM reasoning — examine the content to determine which fault(s)
  the reasoning addresses.
- Tool calls (k8s_pods_list, prom_query, etc.) should be assigned based on the parameters
  and output content — which fault's symptoms or resources do they inspect?
- Triage/priority reasoning events may relate to ALL active faults.

## Output Format

Return a JSON object with a single key "classifications" containing an array of
EventClassification objects, one per event in the batch. Every event must appear
in the output exactly once, identified by its event_id."""


# ---------------------------------------------------------------------------
# 3. Helper: Parse JSON fields from trace events
# ---------------------------------------------------------------------------

def _safe_parse_json(value: Any) -> Any:
    """Parse a JSON string field; return as-is if already parsed or unparseable."""
    if isinstance(value, str):
        try:
            return json.loads(value)
        except (json.JSONDecodeError, TypeError):
            return value
    return value


def _parse_iso_timestamp(ts: Optional[str]) -> Optional[datetime]:
    """Parse an ISO-8601 timestamp string to datetime, or None on failure."""
    if not ts:
        return None
    try:
        # Handle trailing 'Z' and various ISO formats
        ts_clean = ts.replace("Z", "+00:00")
        return datetime.fromisoformat(ts_clean)
    except (ValueError, TypeError):
        return None


# ---------------------------------------------------------------------------
# 4. FaultBucketingPipeline
# ---------------------------------------------------------------------------

class FaultBucketingPipeline:
    """
    Preprocesses a Langfuse trace by separating interleaved events into
    per-fault buckets so each fault's lifecycle can be evaluated independently.

    Algorithm (from Section 1.4, §6.2):
      1. Initialize empty active-faults list and bucket dictionary.
      2. Stream events in temporal order.
      3. On fault-detection event → add to active-faults, create bucket.
      4. On other events → batch → send to LLM classifier → assign to bucket(s).
      5. On mitigation confirmation → close bucket, remove from active faults.
      6. Output per-fault JSON files.
    """

    DEFAULT_BATCH_SIZE = 10
    MODEL_NAME = "extraction_model"

    def __init__(
        self,
        trace_file_path: str,
        output_dir: str,
        config: Optional[Dict[str, Any]] = None,
        batch_size: int = DEFAULT_BATCH_SIZE,
    ):
        self.trace_file_path = Path(trace_file_path)
        self.output_dir = Path(output_dir)
        self.batch_size = batch_size

        # Load config
        if config:
            self.config = config
        elif ConfigLoader:
            self.config = ConfigLoader.load_config()
        else:
            self.config = {}

        # LLM client (initialized lazily)
        self._llm_client: Optional[Any] = None

        # Pipeline state
        self.active_faults: Dict[str, FaultBucket] = {}
        self.closed_faults: Dict[str, FaultBucket] = {}
        self.unclassified_events: List[Dict[str, Any]] = []

        # Token tracking
        self.total_input_tokens = 0
        self.total_output_tokens = 0

    # ------------------------------------------------------------------
    # LLM client (lazy init)
    # ------------------------------------------------------------------

    def _get_llm_client(self) -> Any:
        """Get or create the AzureLLMClient singleton."""
        if self._llm_client is None:
            if AzureLLMClient is None:
                raise RuntimeError(
                    "AzureLLMClient is not available. "
                    "Install the required dependencies."
                )
            self._llm_client = AzureLLMClient(config=self.config)
        return self._llm_client

    # ------------------------------------------------------------------
    # 4a. Load and sort events
    # ------------------------------------------------------------------

    def _load_trace(self) -> List[Dict[str, Any]]:
        """Load the trace JSON file and validate it's a list of span objects."""
        if not self.trace_file_path.exists():
            raise FileNotFoundError(
                f"Trace file not found: {self.trace_file_path}"
            )
        with open(self.trace_file_path, "r", encoding="utf-8") as f:
            data = json.load(f)

        if not isinstance(data, list):
            raise ValueError(
                f"Expected a JSON array of trace events, "
                f"got {type(data).__name__}"
            )
        logger.info(
            f"Loaded {len(data)} events from {self.trace_file_path.name}"
        )
        return data

    @staticmethod
    def _sort_events_chronologically(
        events: List[Dict[str, Any]],
    ) -> List[Dict[str, Any]]:
        """Sort trace events by startTime (ISO-8601). Nulls sort last."""

        def _sort_key(event: Dict[str, Any]):
            ts = _parse_iso_timestamp(event.get("startTime"))
            # Push events without timestamps to the end
            return ts if ts else datetime.max.replace(tzinfo=None)

        return sorted(events, key=_sort_key)

    # ------------------------------------------------------------------
    # 4b. Fault-detection event identification (no hardcoding)
    # ------------------------------------------------------------------

    @staticmethod
    def _is_fault_detection_event(event: Dict[str, Any]) -> bool:
        """
        Determine whether an event is a fault-detection event.

        Heuristics (derived from trace schema, not hardcoded to specific faults):
          - metadata.action == "fault_detected"
          - Event name starts with "fault_detected"
          - Input contains both "fault_name" and "severity" keys
        """
        # Check metadata
        metadata = _safe_parse_json(event.get("metadata", {}))
        if isinstance(metadata, dict):
            if metadata.get("action") == "fault_detected":
                return True

        # Check event name
        name = event.get("name", "")
        if name.startswith("fault_detected"):
            return True

        # Check input for fault-detection payload
        input_data = _safe_parse_json(event.get("input", {}))
        if isinstance(input_data, dict):
            if "fault_name" in input_data and "severity" in input_data and "detection_signals" in input_data:
                return True

        return False

    @staticmethod
    def _extract_fault_info(event: Dict[str, Any]) -> FaultBucket:
        """
        Extract fault metadata from a fault-detection event and return
        a new FaultBucket shell.
        """
        input_data = _safe_parse_json(event.get("input", {}))
        metadata = _safe_parse_json(event.get("metadata", {}))

        fault_name = ""
        target_pod = ""
        namespace = ""
        severity = ""
        detection_signals: List[str] = []
        detected_at = ""

        if isinstance(input_data, dict):
            fault_name = input_data.get("fault_name", "")
            target_pod = input_data.get("pod", "")
            namespace = input_data.get("namespace", "")
            severity = input_data.get("severity", "")
            detection_signals = input_data.get("detection_signals", [])
            detected_at = input_data.get("detected_at", event.get("startTime", ""))

        if isinstance(metadata, dict):
            if not fault_name:
                fault_name = metadata.get("fault_name", "")
            if not severity:
                severity = metadata.get("severity", "")
            if not detected_at:
                detected_at = metadata.get("timestamp", "")

        # Build a stable, unique fault_id from available info
        fault_id = fault_name
        if target_pod:
            fault_id = f"{fault_name}_{target_pod}"
        if namespace:
            fault_id = f"{fault_id}_{namespace}"

        return FaultBucket(
            fault_id=fault_id,
            fault_name=fault_name,
            severity=severity or None,
            target_pod=target_pod or None,
            namespace=namespace or None,
            detection_signals=detection_signals,
            events=[event],  # include the detection event itself
            status="active",
            detected_at=detected_at or None,
        )

    # ------------------------------------------------------------------
    # 4c. Mitigation / confirmation detection
    # ------------------------------------------------------------------

    def _is_mitigation_confirmed(
        self, event: Dict[str, Any]
    ) -> Optional[str]:
        """
        Check if this event confirms fault mitigation. Returns the matching
        fault_id if so, else None.

        Checks:
          - metadata.action == "confirm" with a matching fault_name
          - metadata.action == "verify" with fault_resolved == true in output
        """
        metadata = _safe_parse_json(event.get("metadata", {}))
        if not isinstance(metadata, dict):
            return None

        action = metadata.get("action", "")

        if action == "confirm":
            fault_name_in_meta = metadata.get("fault_name", "")
            # Match against active faults by fault_name
            for fid, bucket in self.active_faults.items():
                if bucket.fault_name == fault_name_in_meta:
                    return fid

        if action == "verify":
            output_data = _safe_parse_json(event.get("output", {}))
            if isinstance(output_data, dict) and output_data.get("fault_resolved") is True:
                fault_name_in_meta = metadata.get("fault_name", "")
                for fid, bucket in self.active_faults.items():
                    if bucket.fault_name == fault_name_in_meta:
                        return fid

        return None

    # ------------------------------------------------------------------
    # 4d. Parent-child association (optimization to reduce LLM calls)
    # ------------------------------------------------------------------

    def _resolve_fault_from_parent(
        self, event: Dict[str, Any], events_by_id: Dict[str, Dict[str, Any]]
    ) -> Optional[str]:
        """
        If an event has a parentObservationId whose name contains a known
        fault name (e.g. "investigate:disk-fill"), return the matching
        fault_id. This avoids an LLM call for child spans of known parents.
        """
        parent_id = event.get("parentObservationId")
        if not parent_id:
            return None

        parent_event = events_by_id.get(parent_id)
        if not parent_event:
            return None

        parent_name = (parent_event.get("name") or "").lower()
        parent_metadata = _safe_parse_json(parent_event.get("metadata", {}))
        parent_fault_name = ""
        if isinstance(parent_metadata, dict):
            parent_fault_name = (parent_metadata.get("fault_name") or "").lower()

        for fid, bucket in self.active_faults.items():
            bn = bucket.fault_name.lower()
            if bn and (bn in parent_name or bn == parent_fault_name):
                return fid

        # Also check closed faults (child events may arrive after parent closes)
        for fid, bucket in self.closed_faults.items():
            bn = bucket.fault_name.lower()
            if bn and (bn in parent_name or bn == parent_fault_name):
                return fid

        return None

    # ------------------------------------------------------------------
    # 4e. Name / metadata based association
    # ------------------------------------------------------------------

    def _resolve_fault_from_name(
        self, event: Dict[str, Any]
    ) -> Optional[str]:
        """
        If the event name or metadata.fault_name directly references a known
        fault, return the matching fault_id.
        E.g. "investigate:disk-fill", "remediate:pod-delete", "verify:pod-network-latency"
        """
        name = (event.get("name") or "").lower()
        metadata = _safe_parse_json(event.get("metadata", {}))
        meta_fault = ""
        if isinstance(metadata, dict):
            meta_fault = (metadata.get("fault_name") or "").lower()

        all_faults = {**self.active_faults, **self.closed_faults}
        for fid, bucket in all_faults.items():
            bn = bucket.fault_name.lower()
            if bn and (bn in name or bn == meta_fault):
                return fid

        return None

    # ------------------------------------------------------------------
    # 4f. LLM batch classification
    # ------------------------------------------------------------------

    @staticmethod
    def _create_event_batches(
        events: List[Dict[str, Any]], batch_size: int
    ) -> List[List[Dict[str, Any]]]:
        """Split events into batches of the given size, preserving order."""
        return [
            events[i : i + batch_size]
            for i in range(0, len(events), batch_size)
        ]

    def _build_classifier_user_message(
        self,
        batch: List[Dict[str, Any]],
        active_faults: Dict[str, FaultBucket],
    ) -> str:
        """Build the user message for the LLM classifier."""

        # Active faults context
        faults_context = []
        for fid, bucket in active_faults.items():
            faults_context.append({
                "fault_id": fid,
                "fault_name": bucket.fault_name,
                "severity": bucket.severity,
                "target_pod": bucket.target_pod,
                "namespace": bucket.namespace,
                "detection_signals": bucket.detection_signals,
            })

        # Compact event representation for the batch
        events_for_llm = []
        for evt in batch:
            events_for_llm.append({
                "event_id": evt.get("id"),
                "type": evt.get("type"),
                "name": evt.get("name"),
                "startTime": evt.get("startTime"),
                "endTime": evt.get("endTime"),
                "parentObservationId": evt.get("parentObservationId"),
                "input": evt.get("input"),
                "output": evt.get("output"),
                "metadata": evt.get("metadata"),
            })

        message = (
            "## Active Faults\n\n"
            f"```json\n{json.dumps(faults_context, indent=2)}\n```\n\n"
            "## Event Batch\n\n"
            f"```json\n{json.dumps(events_for_llm, indent=2)}\n```\n\n"
            "Classify each event above against the active faults. "
            "Return a JSON object with a 'classifications' array."
        )
        return message

    async def _classify_batch(
        self,
        batch: List[Dict[str, Any]],
        active_faults: Dict[str, FaultBucket],
    ) -> List[EventClassification]:
        """
        Send a batch of events to the LLM for classification against
        currently active faults. Falls back to assigning all events to
        every active fault on failure.
        """
        if not active_faults:
            # No active faults to classify against
            return []

        try:
            client = self._get_llm_client()
            user_message = self._build_classifier_user_message(batch, active_faults)

            result, usage = await client.with_structured_output(
                model_name=self.MODEL_NAME,
                messages=[{"role": "user", "content": user_message}],
                output_format=BatchClassificationResult,
                temperature=0.1,
                max_tokens=4000,
                system_prompt=FAULT_CLASSIFIER_SYSTEM_PROMPT,
            )

            # Track tokens
            if isinstance(usage, dict):
                self.total_input_tokens += usage.get("input_tokens", 0)
                self.total_output_tokens += usage.get("output_tokens", 0)

            if isinstance(result, BatchClassificationResult):
                return result.classifications
            elif isinstance(result, dict) and "classifications" in result:
                return [
                    EventClassification.model_validate(c)
                    for c in result["classifications"]
                ]
            else:
                logger.warning(
                    "LLM returned unexpected format, using fallback classification"
                )
                return self._fallback_classify(batch, active_faults)

        except Exception as e:
            logger.error(f"LLM classification failed: {e}. Using fallback.")
            return self._fallback_classify(batch, active_faults)

    @staticmethod
    def _fallback_classify(
        batch: List[Dict[str, Any]],
        active_faults: Dict[str, FaultBucket],
    ) -> List[EventClassification]:
        """Assign every event to ALL active faults as a conservative fallback."""
        all_fault_ids = list(active_faults.keys())
        return [
            EventClassification(
                event_id=evt.get("id", "unknown"),
                related_faults=all_fault_ids,
                confidence=0.3,
            )
            for evt in batch
        ]

    # ------------------------------------------------------------------
    # 4g. Place classified events into buckets
    # ------------------------------------------------------------------

    def _place_event_in_buckets(
        self, event: Dict[str, Any], classification: EventClassification
    ) -> None:
        """Place an event into the bucket(s) indicated by its classification."""
        placed = False
        all_buckets = {**self.active_faults, **self.closed_faults}

        for fault_id in classification.related_faults:
            bucket = all_buckets.get(fault_id)
            if bucket:
                bucket.events.append(event)
                placed = True

        if not placed:
            self.unclassified_events.append(event)

    # ------------------------------------------------------------------
    # 4h. Close a fault bucket
    # ------------------------------------------------------------------

    def _close_fault(self, fault_id: str, mitigated_at: Optional[str] = None) -> None:
        """Move a fault from active to closed."""
        if fault_id in self.active_faults:
            bucket = self.active_faults.pop(fault_id)
            bucket.status = "closed"
            bucket.mitigated_at = mitigated_at
            self.closed_faults[fault_id] = bucket
            logger.info(
                f"Fault bucket closed: {fault_id} "
                f"({len(bucket.events)} events)"
            )

    # ------------------------------------------------------------------
    # 5. Main orchestration
    # ------------------------------------------------------------------

    async def run(self) -> Dict[str, FaultBucket]:
        """
        Execute the fault bucketing pipeline.

        Returns:
            Dictionary of fault_id → FaultBucket (all buckets, active + closed).
        """
        # Load and sort
        raw_events = self._load_trace()
        sorted_events = self._sort_events_chronologically(raw_events)

        # Build lookup by id for parent-child resolution
        events_by_id: Dict[str, Dict[str, Any]] = {
            evt.get("id", ""): evt for evt in sorted_events if evt.get("id")
        }

        # Separate fault-detection events from others
        remaining_events: List[Dict[str, Any]] = []

        for event in sorted_events:
            if self._is_fault_detection_event(event):
                bucket = self._extract_fault_info(event)
                if bucket.fault_id in self.active_faults:
                    # Duplicate detection (same fault) → append event to existing
                    self.active_faults[bucket.fault_id].events.append(event)
                else:
                    self.active_faults[bucket.fault_id] = bucket
                    logger.info(
                        f"Fault detected: {bucket.fault_id} "
                        f"(severity={bucket.severity}, pod={bucket.target_pod})"
                    )
            else:
                remaining_events.append(event)

        # If no fault-detection events found → single-fault trace fallback
        if not self.active_faults and not self.closed_faults:
            logger.info(
                "No fault_detected events found. "
                "Treating as single-fault trace (one bucket)."
            )
            single_bucket = FaultBucket(
                fault_id="single_fault",
                fault_name="unknown",
                events=sorted_events,
                status="closed",
                detected_at=sorted_events[0].get("startTime") if sorted_events else None,
                mitigated_at=sorted_events[-1].get("endTime") if sorted_events else None,
            )
            self.closed_faults["single_fault"] = single_bucket
            self._write_output()
            return {**self.active_faults, **self.closed_faults}

        # ---- Process remaining events ----
        # Phase A: deterministic assignment (parent-child + name matching)
        needs_llm: List[Dict[str, Any]] = []

        for event in remaining_events:
            # Try deterministic resolution first
            fault_id = self._resolve_fault_from_name(event)
            if not fault_id:
                fault_id = self._resolve_fault_from_parent(event, events_by_id)

            if fault_id:
                # Deterministically assigned
                target = self.active_faults.get(fault_id) or self.closed_faults.get(fault_id)
                if target:
                    target.events.append(event)
                else:
                    needs_llm.append(event)

                # Check for mitigation confirmation
                confirmed_id = self._is_mitigation_confirmed(event)
                if confirmed_id:
                    mitigation_ts = event.get("startTime") or event.get("endTime")
                    self._close_fault(confirmed_id, mitigated_at=mitigation_ts)
            else:
                needs_llm.append(event)

        # Phase B: Identify cross-fault events (shared context)
        # Events that occur before any fault detection or after all faults
        # are closed are cross-fault shared events (cluster scans, triage
        # reasoning, final stability checks) and belong to ALL buckets.
        earliest_detection = None
        for bucket in list(self.active_faults.values()) + list(self.closed_faults.values()):
            ts = _parse_iso_timestamp(bucket.detected_at)
            if ts and (earliest_detection is None or ts < earliest_detection):
                earliest_detection = ts

        cross_fault_events: List[Dict[str, Any]] = []
        still_needs_llm: List[Dict[str, Any]] = []

        for event in needs_llm:
            event_ts = _parse_iso_timestamp(event.get("startTime"))
            metadata = _safe_parse_json(event.get("metadata", {}))
            action = metadata.get("action", "") if isinstance(metadata, dict) else ""

            # Cross-fault heuristics:
            # 1. Events before the first fault detection (initial scans)
            is_pre_detection = (
                event_ts and earliest_detection and event_ts < earliest_detection
            )
            # 2. Triage reasoning or final stability checks span all faults
            is_cross_fault_action = action in (
                "triage_reasoning",
                "final_stability_check",
                "final_stability_reasoning",
                "success_confirmed",
            )
            # 3. Event name indicates cross-fault scope
            name_lower = (event.get("name") or "").lower()
            is_cross_fault_name = any(
                kw in name_lower
                for kw in ("triage", "final_stability", "success_confirmed", "cluster_scan")
            )

            if is_pre_detection or is_cross_fault_action or is_cross_fault_name:
                cross_fault_events.append(event)
            else:
                still_needs_llm.append(event)

        # Assign cross-fault events to ALL buckets
        if cross_fault_events:
            all_current_buckets = {**self.active_faults, **self.closed_faults}
            for event in cross_fault_events:
                for bucket in all_current_buckets.values():
                    bucket.events.append(event)
            logger.info(
                f"Assigned {len(cross_fault_events)} cross-fault events "
                f"to all {len(all_current_buckets)} buckets"
            )

        needs_llm = still_needs_llm

        # Phase C: LLM classification for remaining unresolved events
        if needs_llm and self.active_faults:
            logger.info(
                f"Classifying {len(needs_llm)} events via LLM "
                f"(batch_size={self.batch_size})"
            )
            batches = self._create_event_batches(needs_llm, self.batch_size)

            for batch_idx, batch in enumerate(batches):
                # Use current snapshot of active faults for classification.
                # Include closed faults too since events may relate to
                # already-resolved faults that are still being verified.
                classification_faults = {
                    **self.active_faults,
                    **self.closed_faults,
                }

                classifications = await self._classify_batch(
                    batch, classification_faults
                )

                # Build a map of event_id → classification
                classification_map: Dict[str, EventClassification] = {
                    c.event_id: c for c in classifications
                }

                for event in batch:
                    eid = event.get("id", "")
                    classification = classification_map.get(eid)

                    if classification:
                        self._place_event_in_buckets(event, classification)

                        # Check if LLM flagged mitigation
                        if classification.fault_mitigated:
                            mid = classification.fault_mitigated
                            mitigation_ts = event.get("startTime")
                            self._close_fault(mid, mitigated_at=mitigation_ts)
                    else:
                        # Event wasn't in LLM response (shouldn't happen, but handle)
                        self.unclassified_events.append(event)

                logger.info(
                    f"Batch {batch_idx + 1}/{len(batches)} classified "
                    f"({len(batch)} events)"
                )
        elif needs_llm and not self.active_faults:
            # All faults already closed but events remain → assign to
            # closed faults using deterministic matching or mark unclassified.
            for event in needs_llm:
                fault_id = self._resolve_fault_from_name(event)
                if fault_id and fault_id in self.closed_faults:
                    self.closed_faults[fault_id].events.append(event)
                else:
                    self.unclassified_events.append(event)

        # Re-sort events within each bucket to maintain chronological order
        all_buckets = {**self.active_faults, **self.closed_faults}
        for bucket in all_buckets.values():
            bucket.events = self._sort_events_chronologically(bucket.events)

        # Log summary
        total_events = sum(len(b.events) for b in all_buckets.values())
        logger.info(
            f"Bucketing complete: {len(all_buckets)} buckets, "
            f"{total_events} events assigned, "
            f"{len(self.unclassified_events)} unclassified, "
            f"LLM tokens used: {self.total_input_tokens + self.total_output_tokens}"
        )

        # Write output
        self._write_output()

        return all_buckets

    # ------------------------------------------------------------------
    # 6. Output writer
    # ------------------------------------------------------------------

    def _write_output(self) -> None:
        """Write per-fault bucket JSON files and a summary manifest."""
        self.output_dir.mkdir(parents=True, exist_ok=True)

        trace_stem = self.trace_file_path.stem  # filename without extension
        # Truncate trace stem to keep filenames within OS limits
        max_stem = 80
        short_stem = trace_stem[:max_stem] if len(trace_stem) > max_stem else trace_stem

        all_buckets = {**self.active_faults, **self.closed_faults}
        manifest_entries: List[Dict[str, Any]] = []

        for fault_id, bucket in all_buckets.items():
            # Sanitize fault_id for use in filenames
            safe_id = (
                fault_id.replace("/", "_")
                .replace("\\", "_")
                .replace(" ", "_")
                .replace(":", "_")
            )
            # Use fault_name (shorter) for the filename; keep full fault_id in JSON
            safe_name = bucket.fault_name.replace("/", "_").replace(" ", "_")
            bucket_filename = f"{short_stem}_bucket_{safe_name}.json"
            bucket_path = self.output_dir / bucket_filename

            bucket_output = bucket.to_dict()

            with open(bucket_path, "w", encoding="utf-8") as f:
                json.dump(bucket_output, f, indent=2, default=str)

            manifest_entries.append({
                "fault_id": fault_id,
                "fault_name": bucket.fault_name,
                "severity": bucket.severity,
                "target_pod": bucket.target_pod,
                "namespace": bucket.namespace,
                "status": bucket.status,
                "event_count": len(bucket.events),
                "detected_at": bucket.detected_at,
                "mitigated_at": bucket.mitigated_at,
                "output_file": bucket_filename,
            })

            logger.info(
                f"Wrote bucket file: {bucket_filename} "
                f"({len(bucket.events)} events)"
            )

        # Write unclassified events if any
        if self.unclassified_events:
            unclassified_filename = f"{short_stem}_unclassified.json"
            unclassified_path = self.output_dir / unclassified_filename
            with open(unclassified_path, "w", encoding="utf-8") as f:
                json.dump(self.unclassified_events, f, indent=2, default=str)
            logger.info(
                f"Wrote unclassified events: {unclassified_filename} "
                f"({len(self.unclassified_events)} events)"
            )

        # Write manifest
        manifest_filename = f"{short_stem}_bucketing_manifest.json"
        manifest_path = self.output_dir / manifest_filename
        manifest = {
            "trace_file": self.trace_file_path.name,
            "total_faults": len(all_buckets),
            "total_events_assigned": sum(
                len(b.events) for b in all_buckets.values()
            ),
            "unclassified_event_count": len(self.unclassified_events),
            "llm_tokens_used": {
                "input_tokens": self.total_input_tokens,
                "output_tokens": self.total_output_tokens,
                "total_tokens": self.total_input_tokens + self.total_output_tokens,
            },
            "buckets": manifest_entries,
        }

        with open(manifest_path, "w", encoding="utf-8") as f:
            json.dump(manifest, f, indent=2, default=str)

        logger.info(f"Wrote manifest: {manifest_filename}")


# ---------------------------------------------------------------------------
# 7. CLI Entry Point
# ---------------------------------------------------------------------------

def main():
    """CLI entry point for running the fault bucketing pipeline."""
    parser = argparse.ArgumentParser(
        description="Fault Bucketing Pipeline — preprocess Langfuse traces "
        "into per-fault buckets for metrics extraction."
    )
    parser.add_argument(
        "--trace-file",
        required=True,
        help="Path to the Langfuse trace JSON file.",
    )
    parser.add_argument(
        "--output-dir",
        required=True,
        help="Directory where per-fault bucket JSON files will be written.",
    )
    parser.add_argument(
        "--batch-size",
        type=int,
        default=FaultBucketingPipeline.DEFAULT_BATCH_SIZE,
        help=f"Number of events per LLM classification batch "
        f"(default: {FaultBucketingPipeline.DEFAULT_BATCH_SIZE}).",
    )

    args = parser.parse_args()

    # Load config
    config = {}
    if ConfigLoader:
        try:
            config = ConfigLoader.load_config()
        except Exception as e:
            logger.warning(f"Could not load config: {e}. Using defaults.")

    pipeline = FaultBucketingPipeline(
        trace_file_path=args.trace_file,
        output_dir=args.output_dir,
        config=config,
        batch_size=args.batch_size,
    )

    result = asyncio.run(pipeline.run())

    # Print summary
    print(f"\nFault Bucketing Complete")
    print(f"{'=' * 50}")
    for fault_id, bucket in result.items():
        print(
            f"  [{bucket.status.upper():>6}] {fault_id}: "
            f"{len(bucket.events)} events "
            f"(severity={bucket.severity})"
        )
    print(f"  Unclassified: {len(pipeline.unclassified_events)} events")
    print(f"  Output: {pipeline.output_dir}")


if __name__ == "__main__":
    main()
