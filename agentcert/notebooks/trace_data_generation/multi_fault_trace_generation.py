"""
Multi-fault OTEL-compliant trace generator for ITOps agent scenarios.

Generates a single Langfuse-format trace JSON simulating a scenario where
multiple Kubernetes faults are injected simultaneously and an autonomous
ITOps agent detects, triages, and remediates all of them using the tools
defined in available_tools.md (Kubernetes + Prometheus).

The trace follows a realistic agent lifecycle:
  1. Cluster health scan (agent checks overall state via Pods: List, Events: List)
  2. Multi-fault detection (agent discovers multiple simultaneous issues)
  3. Triage & prioritization (agent reasons about which faults to address first)
  4. Per-fault remediation loop (investigate -> tool_call -> verify -> confirm)
  5. Cross-fault correlation & final stability confirmation

Usage:
    python multi_fault_trace_generation.py --faults-file faults.json \
        --output-dir ../../../agentcert/data/langfuse_minio_traces

    python multi_fault_trace_generation.py --interactive

    python multi_fault_trace_generation.py \
        --fault "pod-delete:Deletes a running pod causing downtime" \
        --fault "pod-network-latency:Injects network latency into pod traffic" \
        --fault "disk-fill:Fills the disk on a node causing I/O pressure"
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
# Available tools registry (mirrors available_tools.md)
# ---------------------------------------------------------------------------

AVAILABLE_TOOLS = {
    # Kubernetes tools
    "k8s_config_view": {
        "name": "Configuration: View",
        "description": "Get the current Kubernetes configuration content as a kubeconfig YAML",
        "category": "kubernetes",
    },
    "k8s_events_list": {
        "name": "Events: List",
        "description": "List Kubernetes events (warnings, errors, state changes) for debugging and troubleshooting",
        "category": "kubernetes",
    },
    "k8s_namespaces_list": {
        "name": "Namespaces: List",
        "description": "List all Kubernetes namespaces in the current cluster",
        "category": "kubernetes",
    },
    "k8s_node_log": {
        "name": "Node: Log",
        "description": "Get logs from a Kubernetes node (kubelet, kube-proxy, or other system logs)",
        "category": "kubernetes",
    },
    "k8s_node_stats": {
        "name": "Node: Stats Summary",
        "description": "Get detailed resource usage statistics from a Kubernetes node via kubelet Summary API",
        "category": "kubernetes",
    },
    "k8s_nodes_top": {
        "name": "Nodes: Top",
        "description": "List the resource consumption (CPU and memory) for Kubernetes Nodes",
        "category": "kubernetes",
    },
    "k8s_pods_delete": {
        "name": "Pods: Delete",
        "description": "Delete a Kubernetes Pod in the provided namespace",
        "category": "kubernetes",
    },
    "k8s_pods_exec": {
        "name": "Pods: Exec",
        "description": "Execute a command in a Kubernetes Pod (shell access, run commands in container)",
        "category": "kubernetes",
    },
    "k8s_pods_get": {
        "name": "Pods: Get",
        "description": "Get a Kubernetes Pod in the provided namespace with the provided name",
        "category": "kubernetes",
    },
    "k8s_pods_list": {
        "name": "Pods: List",
        "description": "List all Kubernetes pods from all namespaces",
        "category": "kubernetes",
    },
    "k8s_pods_list_ns": {
        "name": "Pods: List in Namespace",
        "description": "List all Kubernetes pods in the specified namespace",
        "category": "kubernetes",
    },
    "k8s_pods_log": {
        "name": "Pods: Log",
        "description": "Get the logs of a Kubernetes Pod",
        "category": "kubernetes",
    },
    "k8s_pods_run": {
        "name": "Pods: Run",
        "description": "Run a Kubernetes Pod with the provided container image",
        "category": "kubernetes",
    },
    "k8s_pods_top": {
        "name": "Pods: Top",
        "description": "List the resource consumption (CPU and memory) for Kubernetes Pods",
        "category": "kubernetes",
    },
    "k8s_resources_create_update": {
        "name": "Resources: Create or Update",
        "description": "Create or update a Kubernetes resource by providing YAML or JSON",
        "category": "kubernetes",
    },
    "k8s_resources_delete": {
        "name": "Resources: Delete",
        "description": "Delete a Kubernetes resource by apiVersion, kind, namespace, and name",
        "category": "kubernetes",
    },
    "k8s_resources_get": {
        "name": "Resources: Get",
        "description": "Get a Kubernetes resource by apiVersion, kind, namespace, and name",
        "category": "kubernetes",
    },
    "k8s_resources_list": {
        "name": "Resources: List",
        "description": "List Kubernetes resources by apiVersion and kind",
        "category": "kubernetes",
    },
    "k8s_resources_scale": {
        "name": "Resources: Scale",
        "description": "Get or update the scale of a Kubernetes resource",
        "category": "kubernetes",
    },
    # Prometheus tools
    "prom_health_check": {
        "name": "Health Check",
        "description": "Health check endpoint for container monitoring and status verification",
        "category": "prometheus",
    },
    "prom_query": {
        "name": "Execute PromQL Query",
        "description": "Execute a PromQL instant query against Prometheus",
        "category": "prometheus",
    },
    "prom_range_query": {
        "name": "Execute PromQL Range Query",
        "description": "Execute a PromQL range query with start/end time and step interval",
        "category": "prometheus",
    },
    "prom_list_metrics": {
        "name": "List Available Metrics",
        "description": "List all available metrics in Prometheus with optional pagination",
        "category": "prometheus",
    },
    "prom_metric_metadata": {
        "name": "Get Metric Metadata",
        "description": "Get metadata (type, help, unit) for metrics",
        "category": "prometheus",
    },
    "prom_scrape_targets": {
        "name": "Get Scrape Targets",
        "description": "Get information about all scrape targets",
        "category": "prometheus",
    },
}


# ---------------------------------------------------------------------------
# Pydantic models for LLM structured output
# ---------------------------------------------------------------------------

class FaultDefinition(BaseModel):
    """Input definition for a single fault to inject."""

    name: str = Field(description="Short fault identifier (e.g., 'pod-delete')")
    description: str = Field(description="Human-readable description of the fault")


class MultiFaultScenario(BaseModel):
    """LLM-generated scenario for multiple simultaneous faults."""

    cluster_name: str = Field(
        description="Realistic Kubernetes cluster name (e.g., 'prod-us-east-1')"
    )
    affected_namespaces: List[str] = Field(
        description="Namespaces affected by the faults"
    )
    fault_scenarios: List["SingleFaultDetail"] = Field(
        description="Detailed scenario for each injected fault"
    )
    cross_fault_interactions: List[str] = Field(
        description="Descriptions of how the faults interact or compound "
        "(e.g., 'network latency amplifies pod restart recovery time')"
    )
    overall_severity: str = Field(
        description="Overall cluster severity: 'medium', 'high', or 'critical'"
    )
    triage_order: List[str] = Field(
        description="Recommended fault remediation order by fault name "
        "(highest priority first)"
    )


class SingleFaultDetail(BaseModel):
    """LLM-generated details for one fault in a multi-fault scenario."""

    fault_name: str = Field(description="Fault identifier matching input")
    target_pod_prefix: str = Field(
        description="Realistic Kubernetes pod name prefix with deployment hash"
    )
    target_namespace: str = Field(description="Namespace for the target pod")
    severity: str = Field(description="Fault severity: 'low', 'medium', 'high', or 'critical'")
    symptoms: List[str] = Field(description="Observable symptoms for this fault")
    detection_signals: List[str] = Field(
        description="Signals agent would notice (e.g., 'pod CrashLoopBackOff', 'high latency')"
    )
    log_excerpts: List[str] = Field(
        description="3-5 realistic log lines during this fault"
    )
    resource_metrics: Dict[str, Any] = Field(
        description="Resource metrics snapshot during the fault"
    )
    remediation_tools: List[str] = Field(
        description="Tool keys from AVAILABLE_TOOLS the agent would use "
        "(e.g., ['k8s_pods_log', 'k8s_pods_delete', 'k8s_resources_scale'])"
    )
    remediation_actions: List[str] = Field(
        description="Ordered remediation steps the agent would take"
    )
    typical_ttd_seconds: float = Field(
        description="Typical time-to-detect in seconds"
    )
    typical_ttr_seconds: float = Field(
        description="Typical time-to-remediate in seconds"
    )


class ClusterScanResult(BaseModel):
    """LLM-generated cluster health scan output."""

    pods_list_output: str = Field(
        description="Realistic 'kubectl get pods --all-namespaces' output showing "
        "multiple pods in various states including the faulty ones"
    )
    events_output: str = Field(
        description="Realistic 'kubectl get events --all-namespaces' output showing "
        "warning events related to the injected faults"
    )
    nodes_top_output: str = Field(
        description="Realistic 'kubectl top nodes' output showing resource usage"
    )
    initial_anomalies: List[str] = Field(
        description="Anomalies the agent notices from the scan results"
    )
    agent_reasoning: str = Field(
        description="Agent's internal reasoning about the cluster state and what "
        "to investigate further"
    )


class TriageDecision(BaseModel):
    """LLM-generated triage reasoning for multiple faults."""

    reasoning_text: str = Field(
        description="Detailed paragraph explaining the agent's triage logic: "
        "why faults are prioritized in a certain order, cross-fault impact analysis"
    )
    prioritized_faults: List["FaultPriority"] = Field(
        description="Faults ordered by remediation priority (highest first)"
    )
    estimated_total_remediation_seconds: float = Field(
        description="Agent's estimate of total time to remediate all faults"
    )
    risk_assessment: str = Field(
        description="Overall risk if faults are left unaddressed"
    )


class FaultPriority(BaseModel):
    """Priority assignment for a single fault during triage."""

    fault_name: str = Field(description="Fault identifier")
    priority: int = Field(description="Priority rank (1 = highest)")
    severity: str = Field(description="Assessed severity")
    reason: str = Field(description="Why this fault has this priority")
    blocks_other_faults: bool = Field(
        description="Whether remediating this fault is prerequisite for others"
    )


class FaultInvestigationResult(BaseModel):
    """LLM-generated investigation result for a specific fault."""

    tool_calls: List["ToolCallDetail"] = Field(
        description="Ordered sequence of 3-5 tool calls the agent makes to investigate "
        "this specific fault. Each tool call uses a tool from AVAILABLE_TOOLS."
    )
    diagnosis: str = Field(
        description="Agent's diagnosis after investigation"
    )
    root_cause: str = Field(
        description="Identified root cause of the fault"
    )
    confidence_score: float = Field(
        description="Agent's confidence in the diagnosis (0.0 to 1.0)"
    )


class ToolCallDetail(BaseModel):
    """A single tool invocation by the agent."""

    tool_key: str = Field(
        description="Tool key from AVAILABLE_TOOLS (e.g., 'k8s_pods_log')"
    )
    tool_name: str = Field(
        description="Human-readable tool name (e.g., 'Pods: Log')"
    )
    input_params: Dict[str, Any] = Field(
        description="Parameters passed to the tool (e.g., {'namespace': 'default', 'pod': 'myapp-xyz'})"
    )
    raw_output: str = Field(
        description="Realistic raw output returned by the tool (multi-line terminal output)"
    )
    agent_reasoning: str = Field(
        description="Agent's reasoning about the tool output and next steps"
    )
    anomalies_found: List[str] = Field(
        description="Anomalies found in this tool's output"
    )


class RemediationResult(BaseModel):
    """LLM-generated remediation execution result for a fault."""

    tool_calls: List[ToolCallDetail] = Field(
        description="Ordered tool calls the agent makes to remediate the fault"
    )
    action_summary: str = Field(
        description="Summary of remediation action taken"
    )
    recovery_time_seconds: float = Field(
        description="Time for the system to recover after remediation"
    )
    success: bool = Field(description="Whether remediation succeeded")
    confidence_score: float = Field(
        description="Confidence that remediation was successful (0.0 to 1.0)"
    )


class PostRemediationCheck(BaseModel):
    """LLM-generated post-remediation verification for a fault."""

    tool_calls: List[ToolCallDetail] = Field(
        description="Tool calls to verify the fault is resolved"
    )
    fault_resolved: bool = Field(description="Whether the fault is confirmed resolved")
    system_stable: bool = Field(description="Whether the system is stable post-remediation")
    reasoning_text: str = Field(
        description="Detailed reasoning confirming stability"
    )
    confidence_score: float = Field(
        description="Confidence in the stability assessment (0.0 to 1.0)"
    )


class FinalStabilityCheck(BaseModel):
    """LLM-generated final cross-fault stability confirmation."""

    tool_calls: List[ToolCallDetail] = Field(
        description="Final verification tool calls across all faults"
    )
    all_faults_resolved: bool = Field(
        description="Whether all faults are confirmed resolved"
    )
    cluster_health: str = Field(
        description="Overall cluster health: 'healthy', 'degraded', or 'critical'"
    )
    reasoning_text: str = Field(
        description="Comprehensive reasoning about cluster-wide stability"
    )
    confidence_score: float = Field(
        description="Overall confidence (0.0 to 1.0)"
    )
    recommendations: List[str] = Field(
        description="Post-incident recommendations for preventing recurrence"
    )


# ---------------------------------------------------------------------------
# Multi-fault trace generation engine
# ---------------------------------------------------------------------------

class MultiFaultTraceGenerator:
    """Generates OTEL-compliant Langfuse-format traces for multi-fault scenarios."""

    SYSTEM_PROMPT = (
        "You are an expert Kubernetes ITOps AI agent simulator. "
        "You generate realistic trace data that mimics an autonomous agent "
        "handling MULTIPLE simultaneous Kubernetes infrastructure faults. "
        "The agent uses specific tools (Kubernetes API, Prometheus) to detect, "
        "triage, investigate, remediate, and confirm resolution of faults. "
        "Your outputs must be technically accurate, referencing real Kubernetes "
        "concepts, realistic pod names, log excerpts, metrics, and tool outputs. "
        "The agent must demonstrate cross-fault awareness — understanding how "
        "one fault impacts another and prioritizing accordingly.\n\n"
        "Available Tools:\n" +
        "\n".join(
            f"- {k}: {v['name']} — {v['description']}"
            for k, v in AVAILABLE_TOOLS.items()
        )
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

            if isinstance(response, output_format):
                return response

            if isinstance(response, dict):
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

    # --- OTEL span helpers ---

    @staticmethod
    def _make_span_id() -> str:
        return str(uuid.uuid4())

    @staticmethod
    def _ts(dt: datetime) -> str:
        return dt.strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z"

    @staticmethod
    def _make_trace_id() -> str:
        return "".join(random.choices(string.hexdigits[:16], k=32))

    @staticmethod
    def _make_agent_id() -> str:
        return "".join(random.choices(string.hexdigits[:16], k=24))

    def _build_span(
        self,
        *,
        trace_id: str,
        parent_span_id: Optional[str],
        span_type: str,
        name: str,
        start_time: datetime,
        end_time: Optional[datetime],
        input_data: Dict[str, Any],
        output_data: Any,
        metadata: Dict[str, Any],
    ) -> Dict[str, Any]:
        span_id = self._make_span_id()
        short_id = span_id[:8]
        span = {
            "id": span_id,
            "traceId": trace_id,
            "type": span_type,
            "name": f"{name} ({short_id})",
            "startTime": self._ts(start_time),
            "endTime": self._ts(end_time) if end_time else None,
            "depth": 1 if parent_span_id else 0,
            "parentObservationId": parent_span_id,
            "input": json.dumps(input_data),
            "output": json.dumps(output_data) if isinstance(output_data, dict) else output_data,
            "metadata": json.dumps(metadata),
        }
        return span

    def _build_tool_span(
        self,
        *,
        trace_id: str,
        parent_span_id: Optional[str],
        tool_call: ToolCallDetail,
        start_time: datetime,
        duration_seconds: float,
        agent_id: str,
    ) -> List[Dict[str, Any]]:
        """Build a tool_call SPAN + a reasoning GENERATION span for one tool invocation."""
        end_time = start_time + timedelta(seconds=duration_seconds)
        reasoning_start = end_time + timedelta(seconds=random.uniform(0.2, 1.0))
        reasoning_end = reasoning_start + timedelta(seconds=random.uniform(0.5, 2.0))

        tool_info = AVAILABLE_TOOLS.get(tool_call.tool_key, {})
        tokens = random.randint(200, 600) if tool_call.agent_reasoning else 0

        tool_span = self._build_span(
            trace_id=trace_id,
            parent_span_id=parent_span_id,
            span_type="SPAN",
            name=f"tool_call:{tool_call.tool_key}",
            start_time=start_time,
            end_time=end_time,
            input_data={
                "tool_key": tool_call.tool_key,
                "tool_name": tool_call.tool_name,
                "category": tool_info.get("category", "unknown"),
                "parameters": tool_call.input_params,
                "agent_id": agent_id,
                "timestamp": self._ts(start_time),
            },
            output_data=tool_call.raw_output,
            metadata={
                "action": "tool_call",
                "method": tool_call.tool_key,
                "tool_name": tool_call.tool_name,
                "tool_category": tool_info.get("category", "unknown"),
                "timestamp": self._ts(start_time),
                "duration_seconds": round(duration_seconds, 3),
                "anomalies_found": tool_call.anomalies_found,
                "llm_used": False,
                "tokens_consumed": 0,
                "confidence_score": None,
            },
        )

        reasoning_span = self._build_span(
            trace_id=trace_id,
            parent_span_id=parent_span_id,
            span_type="GENERATION",
            name=f"tool_reasoning:{tool_call.tool_key}",
            start_time=reasoning_start,
            end_time=reasoning_end,
            input_data={
                "tool_key": tool_call.tool_key,
                "tool_output_summary": tool_call.raw_output[:500],
                "anomalies_found": tool_call.anomalies_found,
                "agent_id": agent_id,
                "timestamp": self._ts(reasoning_start),
            },
            output_data=tool_call.agent_reasoning,
            metadata={
                "action": "tool_reasoning",
                "llm_used": True,
                "tokens_consumed": tokens,
                "confidence_score": None,
            },
        )

        return [tool_span, reasoning_span]

    # --- LLM generation methods ---

    async def generate_multi_fault_scenario(
        self, faults: List[FaultDefinition]
    ) -> MultiFaultScenario:
        fault_list = "\n".join(
            f"  {i+1}. {f.name}: {f.description}" for i, f in enumerate(faults)
        )
        available_tool_keys = ", ".join(AVAILABLE_TOOLS.keys())
        prompt = (
            f"Generate a detailed multi-fault Kubernetes scenario where ALL of the "
            f"following faults are injected simultaneously into a cluster:\n\n"
            f"{fault_list}\n\n"
            f"For each fault, provide realistic pod names, symptoms, detection signals, "
            f"log excerpts, resource metrics, and remediation steps. "
            f"The remediation_tools for each fault must use tool keys from this list: "
            f"{available_tool_keys}\n\n"
            f"Also describe how the faults interact with each other (cross-fault impacts) "
            f"and provide a recommended triage order based on severity and dependencies."
        )
        return await self._call_llm_structured(prompt, MultiFaultScenario)

    async def generate_cluster_scan(
        self, scenario: MultiFaultScenario, faults: List[FaultDefinition]
    ) -> ClusterScanResult:
        fault_details = "\n".join(
            f"  - {fs.fault_name}: pod={fs.target_pod_prefix}, ns={fs.target_namespace}, "
            f"symptoms={fs.symptoms}"
            for fs in scenario.fault_scenarios
        )
        prompt = (
            f"Generate realistic cluster health scan output for a Kubernetes cluster "
            f"'{scenario.cluster_name}' that has these {len(faults)} active faults:\n\n"
            f"{fault_details}\n\n"
            f"The agent runs these tools for the initial scan:\n"
            f"  1. Pods: List (all namespaces) — show pods in various states\n"
            f"  2. Events: List — show warning events from the faults\n"
            f"  3. Nodes: Top — show node resource usage\n\n"
            f"The outputs should clearly show symptoms of ALL the injected faults "
            f"but interspersed with normal healthy pods/events. The agent's reasoning "
            f"should note multiple anomalies across different namespaces/pods."
        )
        return await self._call_llm_structured(prompt, ClusterScanResult)

    async def generate_triage_decision(
        self, scenario: MultiFaultScenario
    ) -> TriageDecision:
        fault_summary = "\n".join(
            f"  - {fs.fault_name} (severity={fs.severity}): {', '.join(fs.symptoms[:2])}"
            for fs in scenario.fault_scenarios
        )
        prompt = (
            f"The ITOps agent has detected {len(scenario.fault_scenarios)} simultaneous faults "
            f"in cluster '{scenario.cluster_name}':\n\n"
            f"{fault_summary}\n\n"
            f"Cross-fault interactions:\n"
            f"{chr(10).join('  - ' + i for i in scenario.cross_fault_interactions)}\n\n"
            f"Generate the agent's triage reasoning. The agent must decide which fault "
            f"to remediate first based on severity, dependencies, and cross-fault impact. "
            f"Explain the reasoning in detail."
        )
        return await self._call_llm_structured(prompt, TriageDecision)

    async def generate_fault_investigation(
        self, fault_detail: SingleFaultDetail, scenario: MultiFaultScenario
    ) -> FaultInvestigationResult:
        available_tool_keys = ", ".join(AVAILABLE_TOOLS.keys())
        prompt = (
            f"Generate the investigation phase for an ITOps agent diagnosing this fault:\n\n"
            f"Fault: {fault_detail.fault_name}\n"
            f"Target Pod: {fault_detail.target_pod_prefix}\n"
            f"Namespace: {fault_detail.target_namespace}\n"
            f"Symptoms: {', '.join(fault_detail.symptoms)}\n"
            f"Detection Signals: {', '.join(fault_detail.detection_signals)}\n\n"
            f"Context: This is one of {len(scenario.fault_scenarios)} concurrent faults. "
            f"Other active faults: "
            f"{', '.join(fs.fault_name for fs in scenario.fault_scenarios if fs.fault_name != fault_detail.fault_name)}\n\n"
            f"Generate 3-5 tool calls the agent makes to investigate. Each tool_call must use "
            f"a tool_key from: {available_tool_keys}\n"
            f"Include realistic raw_output for each tool (multi-line terminal output showing "
            f"the fault symptoms). The agent should progressively build confidence in the diagnosis."
        )
        return await self._call_llm_structured(prompt, FaultInvestigationResult)

    async def generate_fault_remediation(
        self, fault_detail: SingleFaultDetail, investigation: FaultInvestigationResult
    ) -> RemediationResult:
        available_tool_keys = ", ".join(AVAILABLE_TOOLS.keys())
        prompt = (
            f"Generate remediation execution for this fault:\n\n"
            f"Fault: {fault_detail.fault_name}\n"
            f"Diagnosis: {investigation.diagnosis}\n"
            f"Root Cause: {investigation.root_cause}\n"
            f"Target Pod: {fault_detail.target_pod_prefix}\n"
            f"Namespace: {fault_detail.target_namespace}\n"
            f"Available Remediation Tools: {', '.join(fault_detail.remediation_tools)}\n"
            f"Planned Actions: {', '.join(fault_detail.remediation_actions)}\n\n"
            f"Generate 1-3 tool calls the agent makes to remediate. Each tool_call must use "
            f"a tool_key from: {available_tool_keys}\n"
            f"Include realistic output showing the remediation being applied."
        )
        return await self._call_llm_structured(prompt, RemediationResult)

    async def generate_post_remediation_check(
        self, fault_detail: SingleFaultDetail
    ) -> PostRemediationCheck:
        available_tool_keys = ", ".join(AVAILABLE_TOOLS.keys())
        prompt = (
            f"Generate post-remediation verification for this fault:\n\n"
            f"Fault: {fault_detail.fault_name} (just remediated)\n"
            f"Target Pod: {fault_detail.target_pod_prefix}\n"
            f"Namespace: {fault_detail.target_namespace}\n\n"
            f"Generate 1-2 tool calls to verify the fault is resolved. Each tool_call must use "
            f"a tool_key from: {available_tool_keys}\n"
            f"The output should show the pod/service returning to healthy state."
        )
        return await self._call_llm_structured(prompt, PostRemediationCheck)

    async def generate_final_stability_check(
        self, scenario: MultiFaultScenario, faults: List[FaultDefinition]
    ) -> FinalStabilityCheck:
        available_tool_keys = ", ".join(AVAILABLE_TOOLS.keys())
        prompt = (
            f"Generate a final cross-fault stability check. All {len(faults)} faults have "
            f"been remediated in cluster '{scenario.cluster_name}':\n\n"
            f"Faults remediated: {', '.join(f.name for f in faults)}\n\n"
            f"Generate 2-3 tool calls for a comprehensive cluster-wide health check. "
            f"Each tool_call must use a tool_key from: {available_tool_keys}\n"
            f"The outputs should confirm all pods are healthy, no warning events, "
            f"and metrics are within normal ranges. Provide recommendations for prevention."
        )
        return await self._call_llm_structured(prompt, FinalStabilityCheck)

    # --- Core trace assembly ---

    async def generate_trace(
        self,
        faults: List[FaultDefinition],
        num_detection_cycles: int = 3,
    ) -> List[Dict[str, Any]]:
        """
        Generate a complete OTEL-compliant trace for a multi-fault scenario.

        Trace lifecycle:
          Phase 0: Cluster health scan (Pods: List, Events: List, Nodes: Top)
          Phase 1: Multi-fault detection (fault_detected spans for each fault)
          Phase 2: Triage & prioritization (reasoning GENERATION)
          Phase 3: Per-fault loop: investigate -> remediate -> verify -> confirm
          Phase 4: Final cross-fault stability check
        """
        logger.info(f"Generating multi-fault trace for {len(faults)} faults: "
                     f"{', '.join(f.name for f in faults)}")

        trace_id = self._make_trace_id()
        agent_id = self._make_agent_id()
        base_time = datetime.now(timezone.utc)
        current_time = base_time
        spans: List[Dict[str, Any]] = []

        # ── Step 1: Generate scenario via LLM ──
        scenario = await self.generate_multi_fault_scenario(faults)
        logger.info(f"Scenario generated: cluster={scenario.cluster_name}, "
                     f"severity={scenario.overall_severity}")

        # Build fault lookup by name
        fault_detail_map = {fs.fault_name: fs for fs in scenario.fault_scenarios}

        # ── Phase 0: Cluster health scan ──
        logger.info("Phase 0: Generating cluster health scan...")
        scan = await self.generate_cluster_scan(scenario, faults)

        # Span: cluster_health_scan (parent span for phase 0)
        scan_parent_id = self._make_span_id()
        scan_start = current_time
        current_time += timedelta(seconds=random.uniform(2.0, 5.0))

        # Tool call: Pods: List
        pods_list_span = self._build_span(
            trace_id=trace_id,
            parent_span_id=None,
            span_type="SPAN",
            name="tool_call:k8s_pods_list",
            start_time=scan_start,
            end_time=scan_start + timedelta(seconds=random.uniform(1.0, 3.0)),
            input_data={
                "tool_key": "k8s_pods_list",
                "tool_name": "Pods: List",
                "parameters": {"all_namespaces": True},
                "agent_id": agent_id,
                "timestamp": self._ts(scan_start),
            },
            output_data=scan.pods_list_output,
            metadata={
                "action": "tool_call",
                "method": "k8s_pods_list",
                "tool_name": "Pods: List",
                "tool_category": "kubernetes",
                "llm_used": False,
                "tokens_consumed": 0,
                "confidence_score": None,
            },
        )
        spans.append(pods_list_span)

        # Tool call: Events: List
        current_time += timedelta(seconds=random.uniform(1.0, 2.0))
        events_span = self._build_span(
            trace_id=trace_id,
            parent_span_id=None,
            span_type="SPAN",
            name="tool_call:k8s_events_list",
            start_time=current_time,
            end_time=current_time + timedelta(seconds=random.uniform(1.0, 3.0)),
            input_data={
                "tool_key": "k8s_events_list",
                "tool_name": "Events: List",
                "parameters": {"all_namespaces": True},
                "agent_id": agent_id,
                "timestamp": self._ts(current_time),
            },
            output_data=scan.events_output,
            metadata={
                "action": "tool_call",
                "method": "k8s_events_list",
                "tool_name": "Events: List",
                "tool_category": "kubernetes",
                "llm_used": False,
                "tokens_consumed": 0,
                "confidence_score": None,
            },
        )
        spans.append(events_span)

        # Tool call: Nodes: Top
        current_time += timedelta(seconds=random.uniform(1.0, 2.0))
        nodes_span = self._build_span(
            trace_id=trace_id,
            parent_span_id=None,
            span_type="SPAN",
            name="tool_call:k8s_nodes_top",
            start_time=current_time,
            end_time=current_time + timedelta(seconds=random.uniform(1.0, 2.0)),
            input_data={
                "tool_key": "k8s_nodes_top",
                "tool_name": "Nodes: Top",
                "parameters": {},
                "agent_id": agent_id,
                "timestamp": self._ts(current_time),
            },
            output_data=scan.nodes_top_output,
            metadata={
                "action": "tool_call",
                "method": "k8s_nodes_top",
                "tool_name": "Nodes: Top",
                "tool_category": "kubernetes",
                "llm_used": False,
                "tokens_consumed": 0,
                "confidence_score": None,
            },
        )
        spans.append(nodes_span)

        # Reasoning span: initial scan analysis
        current_time += timedelta(seconds=random.uniform(1.0, 3.0))
        scan_reasoning_span = self._build_span(
            trace_id=trace_id,
            parent_span_id=None,
            span_type="GENERATION",
            name="cluster_scan_reasoning",
            start_time=current_time,
            end_time=current_time + timedelta(seconds=random.uniform(1.0, 2.0)),
            input_data={
                "anomalies_detected": scan.initial_anomalies,
                "num_faults_suspected": len(faults),
                "agent_id": agent_id,
                "timestamp": self._ts(current_time),
            },
            output_data=scan.agent_reasoning,
            metadata={
                "action": "cluster_scan_reasoning",
                "llm_used": True,
                "tokens_consumed": random.randint(500, 900),
                "confidence_score": None,
            },
        )
        spans.append(scan_reasoning_span)

        # ── Phase 1: Multi-fault detection ──
        logger.info("Phase 1: Generating fault detection spans...")
        current_time += timedelta(seconds=random.uniform(2.0, 5.0))

        for fd in scenario.fault_scenarios:
            current_time += timedelta(seconds=random.uniform(1.0, 4.0))
            detection_span = self._build_span(
                trace_id=trace_id,
                parent_span_id=None,
                span_type="SPAN",
                name="fault_detected",
                start_time=current_time,
                end_time=current_time + timedelta(seconds=random.uniform(0.1, 0.5)),
                input_data={
                    "fault_name": fd.fault_name,
                    "pod": fd.target_pod_prefix,
                    "namespace": fd.target_namespace,
                    "severity": fd.severity,
                    "detection_signals": fd.detection_signals,
                    "message": f"Fault detected: {fd.fault_name} on {fd.target_pod_prefix}",
                    "agent_id": agent_id,
                    "detected_at": self._ts(current_time),
                },
                output_data={"status": "logged"},
                metadata={
                    "action": "fault_detected",
                    "method": "multi_fault_scan",
                    "fault_name": fd.fault_name,
                    "severity": fd.severity,
                    "timestamp": self._ts(current_time),
                    "details": {
                        "pod": fd.target_pod_prefix,
                        "namespace": fd.target_namespace,
                        "symptoms": fd.symptoms,
                        "detection_signals": fd.detection_signals,
                    },
                    "llm_used": False,
                    "tokens_consumed": 0,
                    "confidence_score": None,
                },
            )
            spans.append(detection_span)

        # ── Phase 2: Triage & prioritization ──
        logger.info("Phase 2: Generating triage decision...")
        triage = await self.generate_triage_decision(scenario)
        current_time += timedelta(seconds=random.uniform(3.0, 6.0))

        triage_span = self._build_span(
            trace_id=trace_id,
            parent_span_id=None,
            span_type="GENERATION",
            name="triage_reasoning",
            start_time=current_time,
            end_time=current_time + timedelta(seconds=random.uniform(2.0, 4.0)),
            input_data={
                "num_faults": len(scenario.fault_scenarios),
                "faults": [
                    {"name": fp.fault_name, "severity": fp.severity, "priority": fp.priority}
                    for fp in triage.prioritized_faults
                ],
                "cross_fault_interactions": scenario.cross_fault_interactions,
                "agent_id": agent_id,
                "timestamp": self._ts(current_time),
            },
            output_data=triage.reasoning_text,
            metadata={
                "action": "triage_reasoning",
                "llm_used": True,
                "tokens_consumed": random.randint(800, 1500),
                "triage_order": [fp.fault_name for fp in triage.prioritized_faults],
                "risk_assessment": triage.risk_assessment,
                "estimated_total_remediation_seconds": triage.estimated_total_remediation_seconds,
                "confidence_score": None,
            },
        )
        spans.append(triage_span)

        # ── Phase 3: Per-fault remediation loop (in triage priority order) ──
        remediation_order = [fp.fault_name for fp in triage.prioritized_faults]
        logger.info(f"Phase 3: Remediating faults in order: {remediation_order}")

        for fault_name in remediation_order:
            fd = fault_detail_map.get(fault_name)
            if fd is None:
                logger.warning(f"Fault '{fault_name}' from triage not found in scenario details, skipping")
                continue

            logger.info(f"  Processing fault: {fd.fault_name}")

            # --- 3a: Investigation ---
            investigation = await self.generate_fault_investigation(fd, scenario)
            current_time += timedelta(seconds=random.uniform(2.0, 5.0))

            # Investigation parent span
            invest_start = current_time
            invest_parent_span = self._build_span(
                trace_id=trace_id,
                parent_span_id=None,
                span_type="SPAN",
                name=f"investigate:{fd.fault_name}",
                start_time=invest_start,
                end_time=None,  # filled after tool calls
                input_data={
                    "fault_name": fd.fault_name,
                    "pod": fd.target_pod_prefix,
                    "namespace": fd.target_namespace,
                    "agent_id": agent_id,
                    "timestamp": self._ts(current_time),
                },
                output_data={"diagnosis": investigation.diagnosis, "root_cause": investigation.root_cause},
                metadata={
                    "action": "investigate",
                    "fault_name": fd.fault_name,
                    "llm_used": True,
                    "tokens_consumed": 0,
                    "confidence_score": investigation.confidence_score,
                },
            )
            invest_parent_id = invest_parent_span["id"]

            # Investigation tool calls
            for tc in investigation.tool_calls:
                current_time += timedelta(seconds=random.uniform(1.5, 4.0))
                duration = random.uniform(1.0, 3.0)
                tc_spans = self._build_tool_span(
                    trace_id=trace_id,
                    parent_span_id=invest_parent_id,
                    tool_call=tc,
                    start_time=current_time,
                    duration_seconds=duration,
                    agent_id=agent_id,
                )
                spans.extend(tc_spans)
                current_time += timedelta(seconds=duration + random.uniform(0.5, 1.5))

            # Set investigation end time and add
            invest_parent_span["endTime"] = self._ts(current_time)
            spans.append(invest_parent_span)

            # Investigation diagnosis reasoning
            current_time += timedelta(seconds=random.uniform(1.0, 2.0))
            diagnosis_span = self._build_span(
                trace_id=trace_id,
                parent_span_id=None,
                span_type="GENERATION",
                name=f"diagnosis_reasoning:{fd.fault_name}",
                start_time=current_time,
                end_time=current_time + timedelta(seconds=random.uniform(1.0, 2.0)),
                input_data={
                    "fault_name": fd.fault_name,
                    "investigation_summary": investigation.diagnosis,
                    "root_cause": investigation.root_cause,
                    "agent_id": agent_id,
                    "timestamp": self._ts(current_time),
                },
                output_data=investigation.diagnosis,
                metadata={
                    "action": "diagnosis_reasoning",
                    "fault_name": fd.fault_name,
                    "llm_used": True,
                    "tokens_consumed": random.randint(400, 800),
                    "confidence_score": investigation.confidence_score,
                },
            )
            spans.append(diagnosis_span)

            # --- 3b: Remediation ---
            remediation = await self.generate_fault_remediation(fd, investigation)
            current_time += timedelta(seconds=random.uniform(1.0, 3.0))

            remediate_start = current_time
            remediate_parent_span = self._build_span(
                trace_id=trace_id,
                parent_span_id=None,
                span_type="SPAN",
                name=f"remediate:{fd.fault_name}",
                start_time=remediate_start,
                end_time=None,
                input_data={
                    "fault_name": fd.fault_name,
                    "pod": fd.target_pod_prefix,
                    "namespace": fd.target_namespace,
                    "action_summary": remediation.action_summary,
                    "agent_id": agent_id,
                    "timestamp": self._ts(current_time),
                },
                output_data={
                    "success": remediation.success,
                    "recovery_time_seconds": round(remediation.recovery_time_seconds, 3),
                },
                metadata={
                    "action": "remediate",
                    "fault_name": fd.fault_name,
                    "llm_used": True,
                    "tokens_consumed": 0,
                    "confidence_score": remediation.confidence_score,
                },
            )
            remediate_parent_id = remediate_parent_span["id"]

            for tc in remediation.tool_calls:
                current_time += timedelta(seconds=random.uniform(1.5, 4.0))
                duration = random.uniform(1.0, 5.0)
                tc_spans = self._build_tool_span(
                    trace_id=trace_id,
                    parent_span_id=remediate_parent_id,
                    tool_call=tc,
                    start_time=current_time,
                    duration_seconds=duration,
                    agent_id=agent_id,
                )
                spans.extend(tc_spans)
                current_time += timedelta(seconds=duration + random.uniform(0.5, 1.5))

            # Add recovery wait time
            current_time += timedelta(seconds=remediation.recovery_time_seconds)
            remediate_parent_span["endTime"] = self._ts(current_time)
            spans.append(remediate_parent_span)

            # --- 3c: Post-remediation verification ---
            post_check = await self.generate_post_remediation_check(fd)
            current_time += timedelta(seconds=random.uniform(2.0, 5.0))

            verify_start = current_time
            verify_parent_span = self._build_span(
                trace_id=trace_id,
                parent_span_id=None,
                span_type="SPAN",
                name=f"verify:{fd.fault_name}",
                start_time=verify_start,
                end_time=None,
                input_data={
                    "fault_name": fd.fault_name,
                    "pod": fd.target_pod_prefix,
                    "namespace": fd.target_namespace,
                    "agent_id": agent_id,
                    "timestamp": self._ts(current_time),
                },
                output_data={
                    "fault_resolved": post_check.fault_resolved,
                    "system_stable": post_check.system_stable,
                },
                metadata={
                    "action": "verify",
                    "method": "post_remediation_verification",
                    "fault_name": fd.fault_name,
                    "llm_used": True,
                    "tokens_consumed": 0,
                    "confidence_score": post_check.confidence_score,
                },
            )
            verify_parent_id = verify_parent_span["id"]

            for tc in post_check.tool_calls:
                current_time += timedelta(seconds=random.uniform(1.0, 3.0))
                duration = random.uniform(1.0, 2.5)
                tc_spans = self._build_tool_span(
                    trace_id=trace_id,
                    parent_span_id=verify_parent_id,
                    tool_call=tc,
                    start_time=current_time,
                    duration_seconds=duration,
                    agent_id=agent_id,
                )
                spans.extend(tc_spans)
                current_time += timedelta(seconds=duration + random.uniform(0.3, 1.0))

            verify_parent_span["endTime"] = self._ts(current_time)
            spans.append(verify_parent_span)

            # Verify reasoning
            current_time += timedelta(seconds=random.uniform(0.5, 1.5))
            verify_reasoning_span = self._build_span(
                trace_id=trace_id,
                parent_span_id=None,
                span_type="GENERATION",
                name=f"verify_reasoning:{fd.fault_name}",
                start_time=current_time,
                end_time=current_time + timedelta(seconds=random.uniform(1.0, 2.0)),
                input_data={
                    "fault_name": fd.fault_name,
                    "fault_resolved": post_check.fault_resolved,
                    "system_stable": post_check.system_stable,
                    "agent_id": agent_id,
                    "timestamp": self._ts(current_time),
                },
                output_data=post_check.reasoning_text,
                metadata={
                    "action": "verify_reasoning",
                    "fault_name": fd.fault_name,
                    "llm_used": True,
                    "tokens_consumed": random.randint(400, 700),
                    "confidence_score": post_check.confidence_score,
                },
            )
            spans.append(verify_reasoning_span)

            # confirm span
            current_time += timedelta(seconds=random.uniform(0.5, 1.5))
            confirm_span = self._build_span(
                trace_id=trace_id,
                parent_span_id=None,
                span_type="SPAN",
                name=f"confirm:{fd.fault_name}",
                start_time=current_time,
                end_time=current_time + timedelta(seconds=random.uniform(0.1, 0.5)),
                input_data={
                    "fault_name": fd.fault_name,
                    "pod": fd.target_pod_prefix,
                    "message": f"Fault '{fd.fault_name}' remediated and confirmed stable",
                    "fault_resolved": post_check.fault_resolved,
                    "system_stable": post_check.system_stable,
                    "agent_id": agent_id,
                    "detected_at": self._ts(current_time),
                },
                output_data={"status": "logged"},
                metadata={
                    "action": "confirm",
                    "method": "remediation_confirmation",
                    "fault_name": fd.fault_name,
                    "llm_used": False,
                    "tokens_consumed": 0,
                    "confidence_score": post_check.confidence_score,
                },
            )
            spans.append(confirm_span)

        # ── Phase 4: Final cross-fault stability check ──
        logger.info("Phase 4: Generating final stability check...")
        final_check = await self.generate_final_stability_check(scenario, faults)
        current_time += timedelta(seconds=random.uniform(5.0, 10.0))

        final_start = current_time
        final_parent_span = self._build_span(
            trace_id=trace_id,
            parent_span_id=None,
            span_type="SPAN",
            name="final_stability_check",
            start_time=final_start,
            end_time=None,
            input_data={
                "num_faults_remediated": len(faults),
                "faults": [f.name for f in faults],
                "agent_id": agent_id,
                "timestamp": self._ts(current_time),
            },
            output_data={
                "all_faults_resolved": final_check.all_faults_resolved,
                "cluster_health": final_check.cluster_health,
            },
            metadata={
                "action": "final_stability_check",
                "llm_used": True,
                "tokens_consumed": 0,
                "confidence_score": final_check.confidence_score,
            },
        )
        final_parent_id = final_parent_span["id"]

        for tc in final_check.tool_calls:
            current_time += timedelta(seconds=random.uniform(1.0, 3.0))
            duration = random.uniform(1.0, 3.0)
            tc_spans = self._build_tool_span(
                trace_id=trace_id,
                parent_span_id=final_parent_id,
                tool_call=tc,
                start_time=current_time,
                duration_seconds=duration,
                agent_id=agent_id,
            )
            spans.extend(tc_spans)
            current_time += timedelta(seconds=duration + random.uniform(0.5, 1.0))

        final_parent_span["endTime"] = self._ts(current_time)
        spans.append(final_parent_span)

        # Final stability reasoning
        current_time += timedelta(seconds=random.uniform(1.0, 2.0))
        final_reasoning_span = self._build_span(
            trace_id=trace_id,
            parent_span_id=None,
            span_type="GENERATION",
            name="final_stability_reasoning",
            start_time=current_time,
            end_time=current_time + timedelta(seconds=random.uniform(2.0, 4.0)),
            input_data={
                "all_faults_resolved": final_check.all_faults_resolved,
                "cluster_health": final_check.cluster_health,
                "recommendations": final_check.recommendations,
                "agent_id": agent_id,
                "timestamp": self._ts(current_time),
            },
            output_data=final_check.reasoning_text,
            metadata={
                "action": "final_stability_reasoning",
                "llm_used": True,
                "tokens_consumed": random.randint(600, 1200),
                "confidence_score": final_check.confidence_score,
                "recommendations": final_check.recommendations,
            },
        )
        spans.append(final_reasoning_span)

        # Final success_confirmed
        current_time += timedelta(seconds=random.uniform(0.5, 2.0))
        experiment_duration = (current_time - base_time).total_seconds()
        success_span = self._build_span(
            trace_id=trace_id,
            parent_span_id=None,
            span_type="SPAN",
            name="success_confirmed",
            start_time=current_time,
            end_time=current_time + timedelta(seconds=0.1),
            input_data={
                "message": "All faults remediated and cluster stability confirmed",
                "num_faults": len(faults),
                "faults_remediated": [f.name for f in faults],
                "total_duration_seconds": round(experiment_duration, 3),
                "cluster_health": final_check.cluster_health,
                "agent_id": agent_id,
                "detected_at": self._ts(current_time),
            },
            output_data={"status": "logged"},
            metadata={
                "action": "success_confirmed",
                "method": "multi_fault_final_confirmation",
                "timestamp": self._ts(current_time),
                "total_duration_seconds": round(experiment_duration, 3),
                "faults_count": len(faults),
                "all_resolved": final_check.all_faults_resolved,
                "llm_used": False,
                "tokens_consumed": 0,
                "confidence_score": final_check.confidence_score,
            },
        )
        spans.append(success_span)

        logger.info(f"Generated {len(spans)} spans for {len(faults)} faults "
                     f"(duration: {experiment_duration:.1f}s)")
        return spans

    async def generate_and_save(
        self,
        faults: List[FaultDefinition],
        output_dir: str,
        num_detection_cycles: int = 3,
    ) -> Path:
        """Generate a multi-fault trace and save it to a JSON file."""
        spans = await self.generate_trace(
            faults=faults,
            num_detection_cycles=num_detection_cycles,
        )

        output_path = Path(output_dir)
        output_path.mkdir(parents=True, exist_ok=True)

        experiment_id = "".join(random.choices(string.hexdigits[:16], k=24))
        fault_names_slug = "_".join(f.name for f in faults)[:60]
        filename = f"trace-multi_{fault_names_slug}_{experiment_id}.json"
        filepath = output_path / filename

        with open(filepath, "w", encoding="utf-8") as f:
            json.dump(spans, f, indent=2, ensure_ascii=False)

        logger.info(f"Multi-fault trace saved to {filepath} ({len(spans)} spans)")
        return filepath


# ---------------------------------------------------------------------------
# CLI entry point
# ---------------------------------------------------------------------------

def _parse_fault_arg(value: str) -> FaultDefinition:
    """Parse a 'name:description' fault argument."""
    if ":" not in value:
        raise argparse.ArgumentTypeError(
            f"Invalid fault format '{value}'. Expected 'name:description'"
        )
    name, description = value.split(":", 1)
    return FaultDefinition(name=name.strip(), description=description.strip())


async def main():
    parser = argparse.ArgumentParser(
        description="Generate OTEL-compliant multi-fault traces for ITOps agent scenarios"
    )
    parser.add_argument(
        "--fault",
        type=_parse_fault_arg,
        action="append",
        dest="faults",
        help="Fault in 'name:description' format. Repeat for multiple faults. "
             "Example: --fault 'pod-delete:Deletes a running pod'",
    )
    parser.add_argument(
        "--faults-file",
        type=str,
        help="Path to JSON file with fault definitions: "
             '[{"name": "...", "description": "..."}, ...]',
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
        default=3,
        help="Number of detection-verify cycles per fault (default: 3)",
    )
    parser.add_argument(
        "--interactive",
        action="store_true",
        help="Interactive mode: prompt for faults one at a time",
    )
    parser.add_argument(
        "--model",
        type=str,
        default="extraction_model",
        help="Model key from configs.json to use (default: extraction_model)",
    )
    parser.add_argument(
        "--num-traces",
        type=int,
        default=1,
        help="Number of trace files to generate (default: 1)",
    )

    args = parser.parse_args()

    # Collect faults from various sources
    all_faults: List[FaultDefinition] = []

    if args.faults:
        all_faults.extend(args.faults)

    if args.faults_file:
        faults_path = Path(args.faults_file)
        if not faults_path.exists():
            parser.error(f"Faults file not found: {args.faults_file}")
        with open(faults_path, "r", encoding="utf-8") as f:
            faults_data = json.load(f)
        for fd in faults_data:
            all_faults.append(FaultDefinition(**fd))

    if args.interactive:
        print("Enter faults one at a time. Type 'done' when finished.\n")
        while True:
            name = input("Fault name (or 'done'): ").strip()
            if name.lower() == "done":
                break
            description = input("Fault description: ").strip()
            if name and description:
                all_faults.append(FaultDefinition(name=name, description=description))
                print(f"  Added: {name}\n")

    if not all_faults:
        parser.error(
            "No faults provided. Use --fault, --faults-file, or --interactive."
        )

    if len(all_faults) < 2:
        parser.error(
            "Multi-fault generation requires at least 2 faults. "
            "For single-fault, use trace_data_gen.py instead."
        )

    # Initialize LLM client
    if ConfigLoader is None or AzureLLMClient is None:
        logger.error(
            "Required utilities not available. "
            "Run from the agentcert/ directory with the correct conda environment."
        )
        sys.exit(1)

    config = ConfigLoader.load_config()
    llm_client = AzureLLMClient(config=config)

    generator = MultiFaultTraceGenerator(
        llm_client=llm_client, model_name=args.model
    )

    print(f"\nGenerating {args.num_traces} trace(s) for {len(all_faults)} faults: "
          f"{', '.join(f.name for f in all_faults)}\n")

    for i in range(args.num_traces):
        if args.num_traces > 1:
            print(f"--- Trace {i + 1}/{args.num_traces} ---")

        filepath = await generator.generate_and_save(
            faults=all_faults,
            output_dir=args.output_dir,
            num_detection_cycles=args.num_cycles,
        )
        print(f"Trace generated: {filepath}")

    await llm_client.close()


if __name__ == "__main__":
    asyncio.run(main())
