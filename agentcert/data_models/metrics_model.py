"""
Backward-compatibility shim.
The canonical location is now metric_extraction.schema.metrics_model.
"""

from metric_extraction.schema.metrics_model import (  # noqa: F401
    BaseModelWrapper,
    FaultInfo,
    LLMQualitativeExtraction,
    LLMQuantitativeExtraction,
    MetricsExtractionResult,
    RAICheckStatus,
    SecurityComplianceStatus,
    ToolCall,
)
