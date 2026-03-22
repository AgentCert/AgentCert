"""
LLM-based event classifier for the Fault Bucketing pipeline.

Sends batches of trace events to Azure OpenAI for classification into
per-fault buckets: detects new faults, identifies mitigations, and assigns
events to known faults.
"""

import json
import logging
from typing import Any, Dict, List, Optional

from .data_models import (
    BatchClassificationResult,
    EventClassification,
    FaultBucket,
)

# Optional imports — gracefully handle if not available
try:
    from utils.azure_openai_util import AzureLLMClient
    from utils.setup_logging import logger
except ImportError:
    AzureLLMClient = None
    logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# System Prompt
# ---------------------------------------------------------------------------

FAULT_CLASSIFIER_SYSTEM_PROMPT = """You are an expert fault-event classifier for IT-Ops agent traces.

You will receive:
1. A list of currently **known faults** (may be empty initially) with their IDs, names, and target resources.
2. Optionally, a list of **injected faults** (ground truth from the chaos engineering platform) that the agent is expected to detect.
3. A **batch of trace events** (tool calls, LLM generations, agent actions) with their timestamps and content.

Your task is to:
A. **Discover fault detections**: Identify events where the agent FIRST RECOGNIZES a fault.
B. **Classify events**: Assign each event to one or more known faults.
C. **Detect mitigations**: Identify events that confirm a fault has been remediated.

## Fault Detection Rules

- Set **fault_detected** to the fault name when an event represents the agent
  FIRST RECOGNIZING that a specific fault or problem exists. This includes:
  - Explicit fault detection messages or logs from the agent
  - Agent observations that identify a specific failure (e.g. pod crash, high latency, disk full)
  - Diagnostic conclusions that name a specific fault
- The fault name should be descriptive and concise (e.g. "pod-delete", "disk-fill", "network-latency").
- When fault_detected is set, also populate **detected_fault_severity** (critical/high/medium/low),
  **detected_fault_target_pod**, **detected_fault_namespace**, and **detected_fault_signals**
  from the event content where available.
- Do NOT set fault_detected for events that merely investigate or gather information about
  an already-detected fault.

## Fault Mitigation Rules

- Set **fault_mitigated** to the fault name/ID when an event confirms that a fault has been
  SUCCESSFULLY REMEDIATED. This includes:
  - Explicit confirmation messages that a fix has been applied and verified
  - Verification checks that show the fault condition no longer exists
  - Agent conclusions that remediation is complete
- Do NOT set fault_mitigated for events that merely attempt remediation without confirmation.

## Classification Rules

- **related_faults**: Assign each event to one or more fault IDs it relates to. An event
  can relate to multiple faults (e.g. shared diagnostic steps, triage reasoning).
- If an event detects a NEW fault (fault_detected is set), include that fault name in
  related_faults as well.
- Events before any fault is detected (initial scans, cluster overview) may not relate to
  any specific fault — set related_faults to an empty list for those.
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
- Triage/priority reasoning events may relate to ALL known faults.

## Output Format

Return a JSON object with a single key "classifications" containing an array of
EventClassification objects, one per event in the batch. Every event must appear
in the output exactly once, identified by its event_id."""


# ---------------------------------------------------------------------------
# Classifier
# ---------------------------------------------------------------------------

class FaultEventClassifier:
    """Classifies trace events into fault buckets using an LLM."""

    MODEL_NAME = "extraction_model"

    def __init__(self, config: Dict[str, Any]):
        self.config = config
        self._llm_client: Optional[Any] = None
        self.total_input_tokens = 0
        self.total_output_tokens = 0

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

    def build_user_message(
        self,
        batch: List[Dict[str, Any]],
        known_faults: Dict[str, FaultBucket],
        injected_faults: Dict[str, FaultBucket],
    ) -> str:
        """Build the user message for the LLM classifier."""

        # Known faults context (active + closed)
        faults_context = []
        for fid, bucket in known_faults.items():
            faults_context.append({
                "fault_id": fid,
                "fault_name": bucket.fault_name,
                "severity": bucket.severity,
                "target_pod": bucket.target_pod,
                "namespace": bucket.namespace,
                "detection_signals": bucket.detection_signals,
            })

        # Injected faults context (ground truth from chaos engineering)
        injected_context = []
        for fid, bucket in injected_faults.items():
            injected_context.append({
                "fault_id": fid,
                "fault_name": bucket.fault_name,
                "ground_truth": bucket.ground_truth,
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

        message = "## Known Faults\n\n"
        if faults_context:
            message += f"```json\n{json.dumps(faults_context, indent=2)}\n```\n\n"
        else:
            message += "No faults have been identified yet. Look for fault detection events in this batch.\n\n"

        if injected_context:
            message += (
                "## Injected Faults (Ground Truth)\n\n"
                f"```json\n{json.dumps(injected_context, indent=2)}\n```\n\n"
                "These faults were injected by the chaos engineering platform. "
                "The agent should detect and remediate them during its investigation.\n\n"
            )

        message += (
            "## Event Batch\n\n"
            f"```json\n{json.dumps(events_for_llm, indent=2)}\n```\n\n"
            "Classify each event. Identify any events that represent new fault "
            "detections or fault mitigations. "
            "Return a JSON object with a 'classifications' array."
        )
        return message

    async def classify_batch(
        self,
        batch: List[Dict[str, Any]],
        known_faults: Dict[str, FaultBucket],
        injected_faults: Dict[str, FaultBucket],
    ) -> List[EventClassification]:
        """Send a batch of events to the LLM for classification.

        Falls back to assigning all events to every known fault on failure.
        """
        try:
            client = self._get_llm_client()
            user_message = self.build_user_message(
                batch, known_faults, injected_faults
            )

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
                return self.fallback_classify(batch, known_faults)

        except Exception as e:
            logger.error(f"LLM classification failed: {e}. Using fallback.")
            return self.fallback_classify(batch, known_faults)

    @staticmethod
    def fallback_classify(
        batch: List[Dict[str, Any]],
        known_faults: Dict[str, FaultBucket],
    ) -> List[EventClassification]:
        """Assign every event to ALL known faults as a conservative fallback."""
        all_fault_ids = list(known_faults.keys())
        return [
            EventClassification(
                event_id=evt.get("id", "unknown"),
                related_faults=all_fault_ids,
                confidence=0.3,
            )
            for evt in batch
        ]
