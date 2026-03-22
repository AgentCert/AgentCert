"""
Mock OTEL-compliant trace generator for ITOps agent fault scenarios.

Generates Langfuse-format trace JSON files that mimic an autonomous ITOps agent
detecting, diagnosing, and remediating Kubernetes infrastructure faults.
Uses an LLM to produce realistic reasoning and context for each span.

Usage:
    python trace_data_gen.py --fault-name "pod-network-latency" \
        --fault-description "Injects network latency into pod traffic causing slow responses" \
        --output-dir ../../../agentcert/data/langfuse_minio_traces

    python trace_data_gen.py --interactive  # prompts for fault name and description
"""

import argparse
import asyncio
import json
import logging
import random
import string
import sys
import uuid
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional

from pydantic import BaseModel, Field

# Optional imports - gracefully handle if not available
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
# Pydantic models for LLM structured output
# ---------------------------------------------------------------------------

class FaultScenario(BaseModel):
    """LLM-generated fault scenario details for a given fault type."""

    target_pod_prefix: str = Field(
        description="Realistic Kubernetes pod name prefix (e.g., 'myapp-deployment-7b9f5d6c4f')"
    )
    target_namespace: str = Field(
        default="default",
        description="Kubernetes namespace for the target pod",
    )
    detection_methods: List[str] = Field(
        description="Ordered list of detection methods the agent would use "
        "(e.g., ['latency_spike', 'health_check_timeout', 'periodic_redetection'])"
    )
    initial_detection_message: str = Field(
        description="Human-readable message for the initial fault detection span"
    )
    symptoms: List[str] = Field(
        description="Observable symptoms the agent would notice "
        "(e.g., ['increased response times', 'connection timeouts'])"
    )
    severity: str = Field(
        description="Fault severity: 'low', 'medium', 'high', or 'critical'"
    )
    expected_impact: str = Field(
        description="Description of the expected impact on the system"
    )
    remediation_actions: List[str] = Field(
        description="Ordered list of remediation steps the agent would take"
    )
    remediation_tools: List[str] = Field(
        description="Kubernetes tools/commands the agent would invoke "
        "(e.g., ['kubectl rollout restart', 'kubectl delete pod'])"
    )
    typical_ttd_seconds: float = Field(
        description="Typical time-to-detect in seconds for this fault type"
    )
    typical_ttr_seconds: float = Field(
        description="Typical time-to-remediate in seconds for this fault type"
    )
    log_excerpts: List[str] = Field(
        description="3-5 realistic log lines that would appear during this fault"
    )
    resource_metrics: Dict[str, Any] = Field(
        description="Resource metrics snapshot (cpu_millicores, memory_mb, etc.) "
        "that would be observed during the fault"
    )


class VerificationReasoning(BaseModel):
    """LLM-generated verification reasoning for a single verification cycle."""

    reasoning_text: str = Field(
        description="Detailed reasoning paragraph explaining the agent's analysis "
        "of whether the fault is persistent or transient"
    )
    fault_persistent: bool = Field(
        description="Whether the agent concludes the fault is persistent"
    )
    confidence_score: float = Field(
        description="Agent confidence score between 0.0 and 1.0"
    )
    severity: str = Field(
        description="Assessed severity: 'low', 'medium', 'high', or 'critical'"
    )
    expected_impact: str = Field(
        description="Impact assessment for this verification cycle"
    )
    remediation_priority: str = Field(
        description="Priority: 'low', 'medium', 'high', or 'critical'"
    )


class RemediationReasoning(BaseModel):
    """LLM-generated remediation reasoning."""

    reasoning_text: str = Field(
        description="Detailed reasoning paragraph explaining the remediation action taken"
    )
    action_taken: str = Field(
        description="Specific remediation action performed"
    )
    tool_used: str = Field(
        description="Tool or command used for remediation"
    )
    recovery_time_seconds: float = Field(
        description="Time taken for the system to recover after remediation"
    )
    confidence_score: float = Field(
        description="Confidence that remediation was successful (0.0 to 1.0)"
    )


class ConfirmationReasoning(BaseModel):
    """LLM-generated post-remediation confirmation reasoning."""

    reasoning_text: str = Field(
        description="Detailed reasoning paragraph confirming system stability post-remediation"
    )
    system_stable: bool = Field(description="Whether the system is confirmed stable")
    fault_resolved: bool = Field(description="Whether the fault is fully resolved")
    recovery_quality: str = Field(
        description="Quality of recovery: 'low', 'medium', or 'high'"
    )
    confidence_score: float = Field(
        description="Confidence in the stability assessment (0.0 to 1.0)"
    )


class TraceSpanBatch(BaseModel):
    """Batch of verification reasoning outputs for multiple detection cycles."""

    verification_cycles: List[VerificationReasoning] = Field(
        description="Verification reasoning for each detection cycle"
    )


class InvestigationStep(BaseModel):
    """A single step in the agent's pre-detection investigation phase."""

    action: str = Field(
        description="Action name: one of 'environment_scan', 'log_collection', "
        "'metrics_collection', 'anomaly_detection', 'tool_invocation'"
    )
    method: str = Field(
        description="Specific method used (e.g., 'kubectl_get_pods', 'log_tail', "
        "'prometheus_query', 'describe_pod', 'kubectl_top')"
    )
    command_or_query: str = Field(
        description="Exact command or query the agent executed "
        "(e.g., 'kubectl get pods -n default', 'kubectl logs <pod> --tail=100')"
    )
    raw_output: str = Field(
        description="Realistic raw output the agent received from the command "
        "(multi-line terminal output, truncated if long)"
    )
    reasoning: str = Field(
        description="Agent's internal reasoning paragraph about what it observed "
        "and what to investigate next"
    )
    anomalies_found: List[str] = Field(
        description="List of anomalies or suspicious observations found in this step "
        "(empty list if nothing abnormal)"
    )
    confidence_of_issue: float = Field(
        description="Agent's confidence (0.0 to 1.0) that there is a real issue "
        "based on evidence so far"
    )


class InvestigationPhase(BaseModel):
    """LLM-generated pre-detection investigation phase for the agent."""

    steps: List[InvestigationStep] = Field(
        description="Ordered sequence of 4-6 investigation steps the agent takes "
        "before arriving at a fault detection. Steps should progress from broad "
        "environment scanning to focused anomaly identification, with increasing "
        "confidence that something is wrong."
    )
    final_hypothesis: str = Field(
        description="Agent's concluding hypothesis about the fault after investigation "
        "(this leads into the fault_detected span)"
    )


# ---------------------------------------------------------------------------
# Trace generation engine
# ---------------------------------------------------------------------------

class TraceGenerator:
    """Generates OTEL-compliant Langfuse-format trace files using LLM."""

    SYSTEM_PROMPT = (
        "You are an expert Kubernetes ITOps AI agent simulator. "
        "You generate realistic trace data that mimics an autonomous agent "
        "detecting, diagnosing, verifying, remediating, and confirming "
        "Kubernetes infrastructure faults. "
        "Your outputs must be technically accurate, referencing real Kubernetes "
        "concepts (pods, deployments, readiness probes, resource limits, etc.). "
        "Use realistic pod names, namespaces, log excerpts, and metrics."
    )

    def __init__(self, llm_client: Optional[Any] = None, model_name: str = "extraction_model"):
        self.llm_client = llm_client
        self.model_name = model_name

    async def _call_llm_structured(
        self, prompt: str, output_format, system_prompt: str = None, max_retries: int = 2
    ):
        """Call LLM with structured output parsing and retry on validation failure."""
        if self.llm_client is None:
            raise RuntimeError(
                "LLM client not initialized. Set AZURE_OPENAI_ENDPOINT, "
                "AZURE_OPENAI_API_KEY, and AZURE_OPENAI_CHAT_DEPLOYMENT_NAME env vars."
            )

        # Augment prompt with explicit instruction to return data, not schema
        schema_json = json.dumps(output_format.model_json_schema(), indent=2)
        augmented_prompt = (
            f"{prompt}\n\n"
            f"IMPORTANT: Respond ONLY with a JSON object containing actual generated data "
            f"(not the schema definition). The JSON must conform to this schema:\n"
            f"{schema_json}\n\n"
            f"Return the JSON object directly with populated values. "
            f"Do NOT return the schema itself or wrap it in a 'properties' key."
        )

        last_error = None
        for attempt in range(max_retries + 1):
            response, cost = await self.llm_client.with_structured_output(
                model_name=self.model_name,
                messages=augmented_prompt,
                output_format=output_format,
                system_prompt=system_prompt or self.SYSTEM_PROMPT,
            )
            logger.info(f"LLM call cost (attempt {attempt + 1}): {cost}")

            # If response is already the correct Pydantic model, return it
            if isinstance(response, output_format):
                return response

            # If dict fallback, try manual parsing
            if isinstance(response, dict):
                # The LLM may have returned raw data that just failed validation
                # Try to extract from common wrapper keys
                raw = response.get("response", response)
                if isinstance(raw, str):
                    try:
                        raw = json.loads(raw.strip().strip("`").removeprefix("json").strip())
                    except (json.JSONDecodeError, ValueError):
                        last_error = f"Could not parse raw text as JSON on attempt {attempt + 1}"
                        logger.warning(last_error)
                        continue

                if isinstance(raw, dict):
                    try:
                        return output_format.model_validate(raw)
                    except Exception as e:
                        last_error = f"Pydantic validation failed on attempt {attempt + 1}: {e}"
                        logger.warning(last_error)
                        continue

        raise RuntimeError(
            f"Failed to get valid structured output after {max_retries + 1} attempts. "
            f"Last error: {last_error}"
        )

    async def _call_llm(self, prompt: str, system_prompt: str = None) -> str:
        """Call LLM for free-form text generation."""
        if self.llm_client is None:
            raise RuntimeError("LLM client not initialized.")
        response, cost = await self.llm_client.call_llm(
            model_name=self.model_name,
            messages=prompt,
            system_prompt=system_prompt or self.SYSTEM_PROMPT,
        )
        logger.info(f"LLM call cost: {cost}")
        return response if isinstance(response, str) else json.dumps(response)

    # --- Span builders ---

    @staticmethod
    def _make_span_id() -> str:
        return str(uuid.uuid4())

    @staticmethod
    def _ts(dt: datetime) -> str:
        return dt.strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z"

    @staticmethod
    def _make_agent_id() -> str:
        return "".join(random.choices(string.hexdigits[:16], k=24))

    def _build_span(
        self,
        *,
        span_type: str,
        name: str,
        start_time: datetime,
        input_data: Dict[str, Any],
        output_data: Any,
        metadata: Dict[str, Any],
    ) -> Dict[str, Any]:
        span_id = self._make_span_id()
        short_id = span_id[:8]
        return {
            "id": span_id,
            "type": span_type,
            "name": f"{name} ({short_id})",
            "startTime": self._ts(start_time),
            "endTime": None,
            "depth": 0,
            "input": json.dumps(input_data),
            "output": json.dumps(output_data) if isinstance(output_data, dict) else output_data,
            "metadata": json.dumps(metadata),
        }

    # --- Core generation ---

    async def generate_scenario(self, fault_name: str, fault_description: str) -> FaultScenario:
        """Use LLM to generate a detailed fault scenario."""
        prompt = (
            f"Generate a detailed Kubernetes fault scenario for the following fault:\n\n"
            f"Fault Name: {fault_name}\n"
            f"Fault Description: {fault_description}\n\n"
            f"Provide realistic Kubernetes pod names, detection methods, symptoms, "
            f"remediation steps, log excerpts, and resource metrics that an ITOps agent "
            f"would encounter when handling this fault. "
            f"The target pod should be a realistic deployment with a hash suffix. "
            f"Log excerpts should look like real kubectl or container logs. "
            f"Resource metrics should reflect the fault's impact (e.g., high CPU for stress, "
            f"high latency for network faults)."
        )
        return await self._call_llm_structured(prompt, FaultScenario)

    async def generate_verification_cycles(
        self,
        fault_name: str,
        fault_description: str,
        scenario: FaultScenario,
        num_cycles: int,
    ) -> List[VerificationReasoning]:
        """Generate verification reasoning for multiple detection-verify cycles."""
        prompt = (
            f"Generate {num_cycles} verification reasoning cycles for an ITOps agent "
            f"handling the following fault:\n\n"
            f"Fault Name: {fault_name}\n"
            f"Fault Description: {fault_description}\n"
            f"Target Pod: {scenario.target_pod_prefix}\n"
            f"Symptoms: {', '.join(scenario.symptoms)}\n"
            f"Detection Methods: {', '.join(scenario.detection_methods)}\n\n"
            f"For the first 1-2 cycles, the agent should observe the fault developing "
            f"but may not yet confirm persistence (transient observation). "
            f"In the middle cycles, the agent should confirm the fault as persistent "
            f"with increasing severity. "
            f"In the final cycles, the agent should observe remediation taking effect "
            f"and the fault resolving.\n\n"
            f"Each cycle should have unique, detailed reasoning text referencing specific "
            f"Kubernetes metrics, pod status, and log evidence."
        )
        batch = await self._call_llm_structured(prompt, TraceSpanBatch)
        return batch.verification_cycles

    async def generate_remediation_reasoning(
        self,
        fault_name: str,
        fault_description: str,
        scenario: FaultScenario,
    ) -> RemediationReasoning:
        """Generate remediation reasoning."""
        prompt = (
            f"Generate detailed remediation reasoning for an ITOps agent "
            f"remediating the following fault:\n\n"
            f"Fault Name: {fault_name}\n"
            f"Fault Description: {fault_description}\n"
            f"Target Pod: {scenario.target_pod_prefix}\n"
            f"Available Tools: {', '.join(scenario.remediation_tools)}\n"
            f"Remediation Steps: {', '.join(scenario.remediation_actions)}\n\n"
            f"Describe the specific action taken, which tool was used, "
            f"and how long recovery took. Be technically precise with "
            f"Kubernetes commands and concepts."
        )
        return await self._call_llm_structured(prompt, RemediationReasoning)

    async def generate_confirmation_reasoning(
        self,
        fault_name: str,
        pod_name: str,
    ) -> ConfirmationReasoning:
        """Generate post-remediation confirmation reasoning."""
        prompt = (
            f"Generate post-remediation confirmation reasoning for an ITOps agent "
            f"confirming system stability after remediating '{fault_name}'.\n\n"
            f"Target Pod: {pod_name}\n"
            f"Expected stable state: pod Running, ready=True, 0 restarts, "
            f"healthy resource metrics, normal log output.\n\n"
            f"Describe the agent's verification that the system is stable, "
            f"referencing specific metrics, logs, and Kubernetes status fields."
        )
        return await self._call_llm_structured(prompt, ConfirmationReasoning)

    async def generate_investigation(
        self,
        fault_name: str,
        fault_description: str,
        scenario: FaultScenario,
    ) -> InvestigationPhase:
        """Generate the pre-detection investigation phase."""
        prompt = (
            f"Generate a realistic pre-detection investigation phase for an ITOps agent "
            f"that is monitoring a Kubernetes cluster and gradually discovers a fault.\n\n"
            f"Fault Name: {fault_name}\n"
            f"Fault Description: {fault_description}\n"
            f"Target Pod: {scenario.target_pod_prefix}\n"
            f"Namespace: {scenario.target_namespace}\n"
            f"Expected Symptoms: {', '.join(scenario.symptoms)}\n"
            f"Log Excerpts (for reference): {json.dumps(scenario.log_excerpts)}\n"
            f"Resource Metrics (for reference): {json.dumps(scenario.resource_metrics)}\n\n"
            f"Generate 4-6 ordered investigation steps that the agent takes BEFORE it "
            f"formally identifies the fault. The steps should follow this progression:\n"
            f"1. Routine environment scan — agent checks overall cluster health (kubectl get pods, etc.)\n"
            f"2. Log collection — agent notices something and pulls logs from relevant pods\n"
            f"3. Log/metrics analysis — agent reasons about what the logs/metrics show, "
            f"uses an LLM or internal heuristics\n"
            f"4. Deeper investigation — agent runs targeted commands (describe pod, kubectl top, "
            f"network diagnostics, etc.) to narrow down the issue\n"
            f"5. Anomaly identification — agent correlates evidence and forms a hypothesis\n\n"
            f"Each step must include:\n"
            f"- The exact kubectl/CLI command or API query the agent ran\n"
            f"- Realistic multi-line terminal output that would result from that command, "
            f"showing the fault's symptoms gradually emerging\n"
            f"- The agent's internal reasoning about what it sees and what to check next\n"
            f"- A growing confidence score (starting low ~0.1-0.3, ending at ~0.7-0.85)\n\n"
            f"The final_hypothesis should be a concise statement like "
            f"'Container in pod X is being forcefully killed, causing repeated restarts.'"
        )
        return await self._call_llm_structured(prompt, InvestigationPhase)

    async def generate_trace(
        self,
        fault_name: str,
        fault_description: str,
        num_detection_cycles: int = 5,
        num_pods: int = 3,
    ) -> List[Dict[str, Any]]:
        """
        Generate a complete OTEL-compliant trace for a fault scenario.

        The trace follows the lifecycle:
          0. Investigation phase: environment_scan -> log_collection ->
             log_analysis_reasoning -> metrics_collection ->
             anomaly_detection_reasoning (agent discovers the fault)
          1. fault_detected (initial) -> verify -> verify_reasoning -> success_confirmed
             (repeated for early transient observations)
          2. fault_detected (persistent) -> verify -> verify_reasoning -> remediate
          3. confirm -> confirm_reasoning
          4. Additional pod cycles (fault_detected -> verify -> confirm)
          5. load_generation_complete -> success_confirmed (final)

        Args:
            fault_name: Short fault identifier (e.g., "pod-network-latency")
            fault_description: Human-readable description of the fault
            num_detection_cycles: Number of detection-verify cycles per pod
            num_pods: Number of pods affected during the experiment
        """
        logger.info(f"Generating trace for fault: {fault_name}")

        # Step 1: Generate fault scenario via LLM
        scenario = await self.generate_scenario(fault_name, fault_description)
        logger.info(f"Scenario generated: {scenario.target_pod_prefix}")

        # Step 2: Generate investigation phase via LLM
        investigation = await self.generate_investigation(
            fault_name, fault_description, scenario
        )
        logger.info(
            f"Investigation phase generated: {len(investigation.steps)} steps"
        )

        # Step 3: Generate verification reasoning cycles via LLM
        total_cycles = num_detection_cycles * num_pods
        verifications = await self.generate_verification_cycles(
            fault_name, fault_description, scenario, min(total_cycles, 12)
        )

        # Step 4: Generate remediation reasoning via LLM
        remediation = await self.generate_remediation_reasoning(
            fault_name, fault_description, scenario
        )

        # Step 5: Build the trace spans
        spans: List[Dict[str, Any]] = []
        agent_id = self._make_agent_id()
        base_time = datetime.now(timezone.utc)
        current_time = base_time
        experiment_start = base_time
        verification_idx = 0

        # Generate pod suffixes
        pod_suffixes = [
            "".join(random.choices(string.ascii_lowercase + string.digits, k=5))
            for _ in range(num_pods)
        ]
        pod_names = [f"{scenario.target_pod_prefix}-{s}" for s in pod_suffixes]

        # --- Phase 0: Investigation spans ---
        primary_pod = pod_names[0]
        for step in investigation.steps:
            step_gap = random.uniform(3.0, 10.0)
            current_time += timedelta(seconds=step_gap)
            tokens = random.randint(300, 800) if step.confidence_of_issue > 0.3 else 0

            # Main action span (environment_scan, log_collection, etc.)
            action_input = {
                "pod": primary_pod,
                "namespace": scenario.target_namespace,
                "command": step.command_or_query,
                "message": f"Agent executing: {step.command_or_query}",
                "agent_id": agent_id,
                "timestamp": self._ts(current_time),
            }

            action_span = self._build_span(
                span_type="SPAN",
                name=step.action,
                start_time=current_time,
                input_data=action_input,
                output_data=step.raw_output,
                metadata={
                    "action": step.action,
                    "method": step.method,
                    "timestamp": self._ts(current_time),
                    "details": {
                        "command": step.command_or_query,
                        "anomalies_found": step.anomalies_found,
                        "confidence_of_issue": step.confidence_of_issue,
                    },
                    "llm_used": False,
                    "tokens_consumed": 0,
                    "confidence_score": step.confidence_of_issue,
                },
            )
            spans.append(action_span)

            # Reasoning span (GENERATION type — agent thinks about what it saw)
            reasoning_gap = random.uniform(1.0, 3.0)
            current_time += timedelta(seconds=reasoning_gap)

            reasoning_input = {
                "pod": primary_pod,
                "action_performed": step.action,
                "command": step.command_or_query,
                "raw_output_summary": step.raw_output[:500],
                "anomalies_found": step.anomalies_found,
                "confidence_of_issue": step.confidence_of_issue,
                "agent_id": agent_id,
                "timestamp": self._ts(current_time),
            }

            reasoning_span = self._build_span(
                span_type="GENERATION",
                name=f"{step.action}_reasoning",
                start_time=current_time,
                input_data=reasoning_input,
                output_data=step.reasoning,
                metadata={
                    "confidence_score": step.confidence_of_issue,
                    "llm_used": tokens > 0,
                    "tokens_consumed": tokens,
                },
            )
            spans.append(reasoning_span)

        logger.info(
            f"Investigation phase: {len(investigation.steps)} steps -> "
            f"{len(investigation.steps) * 2} spans"
        )

        # --- Phase 1+: Detection, verification, remediation cycles ---
        for pod_idx, pod_name in enumerate(pod_names):
            cycles_for_pod = num_detection_cycles if pod_idx == 0 else max(2, num_detection_cycles - 1)

            for cycle in range(cycles_for_pod):
                v = verifications[min(verification_idx, len(verifications) - 1)]
                verification_idx += 1

                is_initial = (pod_idx == 0 and cycle == 0)
                is_persistent = v.fault_persistent
                detection_gap = random.uniform(5.0, 15.0)
                verify_gap = random.uniform(4.0, 8.0)
                current_time += timedelta(seconds=detection_gap)

                # --- fault_detected span ---
                if is_initial:
                    detection_method = "investigation_confirmed"
                    detection_message = scenario.initial_detection_message
                    input_data = {
                        "pod": primary_pod,
                        "experiment_type": fault_name,
                        "message": detection_message,
                        "investigation_hypothesis": investigation.final_hypothesis,
                        "agent_id": agent_id,
                        "detected_at": self._ts(current_time),
                    }
                elif cycle == 0:
                    detection_method = "readiness_drop"
                    detection_message = f"Pod {pod_name} became unready"
                    input_data = {
                        "pod": pod_name,
                        "previous_ready": True,
                        "current_ready": False,
                        "message": detection_message,
                        "agent_id": agent_id,
                        "detected_at": self._ts(current_time),
                    }
                else:
                    detection_method = "periodic_redetection"
                    detection_message = f"Periodic re-detection during chaos experiment (new evidence)"
                    input_data = {
                        "pod": pod_name,
                        "experiment_type": fault_name,
                        "message": detection_message,
                        "ready": not is_persistent,
                        "agent_id": agent_id,
                        "detected_at": self._ts(current_time),
                    }

                fault_detected_span = self._build_span(
                    span_type="SPAN",
                    name="fault_detected",
                    start_time=current_time,
                    input_data=input_data,
                    output_data={"status": "logged"},
                    metadata={
                        "action": "fault_detected",
                        "method": detection_method,
                        "timestamp": self._ts(current_time),
                        "details": input_data,
                        "llm_used": False,
                        "tokens_consumed": 0,
                        "confidence_score": None,
                    },
                )
                spans.append(fault_detected_span)

                # --- verify span + verify_reasoning generation ---
                current_time += timedelta(seconds=verify_gap)
                tokens_consumed = random.randint(700, 1200)

                seconds_since_detection = round(verify_gap, 2)
                log_excerpt = "\n".join(
                    scenario.log_excerpts[:3]
                ) if scenario.log_excerpts else ""

                verify_input = {
                    "pod": pod_name,
                    "message": "Verifying fault is persistent and not a transient issue",
                    "ready": not is_persistent,
                    "logs_collected": True,
                    "detection_context": {
                        "method": detection_method,
                        "timestamp": self._ts(current_time - timedelta(seconds=verify_gap)),
                        "ready": not is_persistent,
                        "restart_count": 0 if not is_persistent else random.randint(0, 2),
                        "cpu_millicores": scenario.resource_metrics.get("cpu_millicores"),
                        "memory_mb": scenario.resource_metrics.get("memory_mb"),
                        "log_excerpt": log_excerpt,
                    },
                    "detection_method": detection_method,
                    "detection_message": detection_message,
                    "seconds_since_detection": seconds_since_detection,
                    "fault_persistent": v.fault_persistent,
                    "severity": v.severity,
                    "expected_impact": v.expected_impact,
                    "remediation_priority": v.remediation_priority,
                    "agent_id": agent_id,
                    "detected_at": self._ts(current_time),
                }

                verify_span = self._build_span(
                    span_type="SPAN",
                    name="verify",
                    start_time=current_time,
                    input_data=verify_input,
                    output_data={"status": "logged"},
                    metadata={
                        "action": "verify",
                        "method": "fault_verification",
                        "timestamp": self._ts(current_time),
                        "details": verify_input,
                        "llm_used": True,
                        "tokens_consumed": tokens_consumed,
                        "confidence_score": v.confidence_score,
                    },
                )
                spans.append(verify_span)

                # --- verify_reasoning generation span ---
                reasoning_span = self._build_span(
                    span_type="GENERATION",
                    name="verify_reasoning",
                    start_time=current_time,
                    input_data=verify_input,
                    output_data=v.reasoning_text,
                    metadata={"confidence_score": v.confidence_score},
                )
                spans.append(reasoning_span)

                # --- outcome: success_confirmed or remediate ---
                current_time += timedelta(seconds=random.uniform(0.5, 2.0))

                if not is_persistent:
                    # Fault not persistent -> success_confirmed
                    success_span = self._build_span(
                        span_type="SPAN",
                        name="success_confirmed",
                        start_time=current_time,
                        input_data={
                            "pod": pod_name,
                            "message": "Fault analysis completed successfully",
                            "status": "healthy",
                            "ready": True,
                            "agent_id": agent_id,
                            "detected_at": self._ts(current_time),
                        },
                        output_data={"status": "logged"},
                        metadata={
                            "action": "success_confirmed",
                            "method": "final_confirmation",
                            "timestamp": self._ts(current_time),
                            "details": {
                                "pod": pod_name,
                                "message": "Fault analysis completed successfully",
                                "status": "healthy",
                                "ready": True,
                                "agent_id": agent_id,
                                "detected_at": self._ts(current_time),
                            },
                            "llm_used": False,
                            "tokens_consumed": 0,
                            "confidence_score": None,
                        },
                    )
                    spans.append(success_span)
                else:
                    # Fault is persistent -> remediate
                    recovery_time = remediation.recovery_time_seconds + random.uniform(-1.0, 2.0)
                    recovery_time = max(1.0, recovery_time)

                    remediate_span = self._build_span(
                        span_type="SPAN",
                        name="remediate",
                        start_time=current_time,
                        input_data={
                            "pod": pod_name,
                            "previous_ready": False,
                            "current_ready": True,
                            "message": f"Pod {pod_name} recovered and is ready (TTR from fault detection)",
                            "recovery_time_seconds": round(recovery_time, 6),
                            "agent_id": agent_id,
                            "detected_at": self._ts(current_time),
                        },
                        output_data={"status": "logged"},
                        metadata={
                            "action": "remediate",
                            "method": "readiness_recovered",
                            "timestamp": self._ts(current_time),
                            "details": {
                                "pod": pod_name,
                                "action_taken": remediation.action_taken,
                                "tool_used": remediation.tool_used,
                                "recovery_time_seconds": round(recovery_time, 6),
                                "agent_id": agent_id,
                            },
                            "llm_used": True,
                            "tokens_consumed": random.randint(400, 800),
                            "confidence_score": remediation.confidence_score,
                        },
                    )
                    spans.append(remediate_span)

                    # After remediation, add confirm + confirm_reasoning
                    current_time += timedelta(seconds=recovery_time)

                    confirmation = await self.generate_confirmation_reasoning(
                        fault_name, pod_name
                    )

                    confirm_input = {
                        "pod": pod_name,
                        "message": "Confirming system is stable post-remediation",
                        "ready": True,
                        "stable": confirmation.system_stable,
                        "fault_resolved": confirmation.fault_resolved,
                        "system_stable": confirmation.system_stable,
                        "recovery_quality": confirmation.recovery_quality,
                        "agent_id": agent_id,
                        "detected_at": self._ts(current_time),
                    }

                    confirm_span = self._build_span(
                        span_type="SPAN",
                        name="confirm",
                        start_time=current_time,
                        input_data=confirm_input,
                        output_data={"status": "logged"},
                        metadata={
                            "action": "confirm",
                            "method": "remediation_confirmation",
                            "timestamp": self._ts(current_time),
                            "details": confirm_input,
                            "llm_used": True,
                            "tokens_consumed": random.randint(600, 800),
                            "confidence_score": confirmation.confidence_score,
                        },
                    )
                    spans.append(confirm_span)

                    confirm_reasoning_span = self._build_span(
                        span_type="GENERATION",
                        name="confirm_reasoning",
                        start_time=current_time,
                        input_data=confirm_input,
                        output_data=confirmation.reasoning_text,
                        metadata={"confidence_score": confirmation.confidence_score},
                    )
                    spans.append(confirm_reasoning_span)

        # --- Final: load_generation_complete + success_confirmed ---
        current_time += timedelta(seconds=random.uniform(10.0, 30.0))
        experiment_duration = (current_time - experiment_start).total_seconds()
        total_requests = random.randint(10, 25)
        successful_requests = random.randint(
            int(total_requests * 0.5), total_requests
        )
        failed_requests = total_requests - successful_requests

        load_complete_span = self._build_span(
            span_type="SPAN",
            name="load_generation_complete",
            start_time=current_time,
            input_data={
                "agent_id": agent_id,
                "total_requests": total_requests,
                "successful_requests": successful_requests,
                "failed_requests": failed_requests,
                "success_rate_percent": round(
                    (successful_requests / total_requests) * 100, 2
                ),
                "duration_seconds": round(experiment_duration, 6),
                "qps": 0.5,
                "detected_at": self._ts(current_time),
            },
            output_data={"status": "logged"},
            metadata={
                "action": "load_generation_complete",
                "method": "load_generator",
                "timestamp": self._ts(current_time),
                "details": {
                    "agent_id": agent_id,
                    "total_requests": total_requests,
                    "successful_requests": successful_requests,
                    "failed_requests": failed_requests,
                    "duration_seconds": round(experiment_duration, 6),
                },
                "llm_used": False,
                "tokens_consumed": 0,
                "confidence_score": None,
            },
        )
        spans.append(load_complete_span)

        # Final success_confirmed
        current_time += timedelta(seconds=random.uniform(0.5, 2.0))
        final_pod = pod_names[-1]
        final_success_span = self._build_span(
            span_type="SPAN",
            name="success_confirmed",
            start_time=current_time,
            input_data={
                "pod": final_pod,
                "message": "Fault analysis completed successfully",
                "status": "healthy",
                "ready": True,
                "agent_id": agent_id,
                "detected_at": self._ts(current_time),
            },
            output_data={"status": "logged"},
            metadata={
                "action": "success_confirmed",
                "method": "final_confirmation",
                "timestamp": self._ts(current_time),
                "details": {
                    "pod": final_pod,
                    "message": "Fault analysis completed successfully",
                    "status": "healthy",
                    "ready": True,
                    "agent_id": agent_id,
                    "detected_at": self._ts(current_time),
                },
                "llm_used": False,
                "tokens_consumed": 0,
                "confidence_score": None,
            },
        )
        spans.append(final_success_span)

        logger.info(f"Generated {len(spans)} spans for fault '{fault_name}'")
        return spans

    async def generate_and_save(
        self,
        fault_name: str,
        fault_description: str,
        output_dir: str,
        num_detection_cycles: int = 5,
        num_pods: int = 3,
    ) -> Path:
        """Generate a trace and save it to a JSON file."""
        spans = await self.generate_trace(
            fault_name=fault_name,
            fault_description=fault_description,
            num_detection_cycles=num_detection_cycles,
            num_pods=num_pods,
        )

        output_path = Path(output_dir)
        output_path.mkdir(parents=True, exist_ok=True)

        experiment_id = "".join(random.choices(string.hexdigits[:16], k=24))
        filename = f"trace-exp_{experiment_id}.json"
        filepath = output_path / filename

        with open(filepath, "w", encoding="utf-8") as f:
            json.dump(spans, f, indent=2, ensure_ascii=False)

        logger.info(f"Trace saved to {filepath} ({len(spans)} spans)")
        return filepath


# ---------------------------------------------------------------------------
# CLI entry point
# ---------------------------------------------------------------------------

async def main():
    parser = argparse.ArgumentParser(
        description="Generate OTEL-compliant mock traces for ITOps agent fault scenarios"
    )
    parser.add_argument(
        "--fault-name",
        type=str,
        help="Short fault identifier (e.g., 'pod-network-latency')",
    )
    parser.add_argument(
        "--fault-description",
        type=str,
        help="Human-readable description of the fault",
    )
    parser.add_argument(
        "--output-dir",
        type=str,
        default=str(
            Path(__file__).resolve().parent.parent.parent
            / "data"
            / "langfuse_minio_traces"
        ),
        help="Output directory for trace JSON files",
    )
    parser.add_argument(
        "--num-cycles",
        type=int,
        default=5,
        help="Number of detection-verify cycles per pod (default: 5)",
    )
    parser.add_argument(
        "--num-pods",
        type=int,
        default=3,
        help="Number of affected pods (default: 3)",
    )
    parser.add_argument(
        "--interactive",
        action="store_true",
        help="Interactive mode: prompt for fault name and description",
    )
    parser.add_argument(
        "--model",
        type=str,
        default="extraction_model",
        help="Model key from configs.json to use (default: extraction_model)",
    )

    args = parser.parse_args()

    if args.interactive:
        args.fault_name = input("Fault name (e.g., pod-network-latency): ").strip()
        args.fault_description = input("Fault description: ").strip()

    if not args.fault_name or not args.fault_description:
        parser.error("--fault-name and --fault-description are required (or use --interactive)")

    # Initialize LLM client
    if ConfigLoader is None or AzureLLMClient is None:
        logger.error(
            "Required utilities not available. "
            "Run from the agentcert/ directory with the correct conda environment."
        )
        sys.exit(1)

    config = ConfigLoader.load_config()
    llm_client = AzureLLMClient(config=config)

    generator = TraceGenerator(llm_client=llm_client, model_name=args.model)

    filepath = await generator.generate_and_save(
        fault_name=args.fault_name,
        fault_description=args.fault_description,
        output_dir=args.output_dir,
        num_detection_cycles=args.num_cycles,
        num_pods=args.num_pods,
    )

    print(f"\nTrace generated: {filepath}")

    await llm_client.close()


if __name__ == "__main__":
    for i in range(5):
        asyncio.run(main())
