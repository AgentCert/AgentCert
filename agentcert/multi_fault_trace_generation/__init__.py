"""Multi-fault trace generation package — generates OTEL-compliant Langfuse-format traces
for ITOps agent scenarios with multiple simultaneous Kubernetes faults."""

from multi_fault_trace_generation.schema.data_models import (
    ClusterScanResult,
    FaultDefinition,
    FaultInvestigationResult,
    FaultPriority,
    FinalStabilityCheck,
    MultiFaultScenario,
    PostRemediationCheck,
    RemediationResult,
    SingleFaultDetail,
    ToolCallDetail,
    TriageDecision,
)
from multi_fault_trace_generation.scripts.trace_generator import (
    MultiFaultTraceGenerator,
)
from multi_fault_trace_generation.scripts.tools_registry import (
    AVAILABLE_TOOLS,
)
