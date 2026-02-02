"""
Pydantic models for IT-Ops Agent evaluation metrics extraction.
Extracts both quantitative and qualitative metrics from agent run reports.
"""

import json
import re
from datetime import datetime
from enum import Enum
from pathlib import Path
from typing import Any, Dict, List, Optional, Union

from pydantic import BaseModel, Field, computed_field
from utils.setup_logging import logger

_original_json_encoder_default = getattr(json.JSONEncoder, "default")


class BaseModelWrapper(BaseModel):
    """Base model wrapper to ensure compatibility with TypedDict."""

    def get(self, key: str, default: Optional[Any] = None) -> Optional[Any]:
        """Get the value of a specific key."""
        return getattr(self, key, default)

    def to_dict(self) -> Dict[str, Any]:
        """Convert the model to a dictionary, handling nested Pydantic models."""
        return self.model_dump(exclude_none=True, mode="json")

    def to_json(self) -> str:
        """Convert the model to a JSON string."""
        return json.dumps(self.to_dict())


class RAICheckStatus(str, Enum):
    """Enum for RAI (Responsible AI) check status."""

    PASSED = "Passed"
    FAILED = "Failed"
    NOT_EVALUATED = "Not Evaluated"


class SecurityComplianceStatus(str, Enum):
    """Enum for security and compliance status."""

    COMPLIANT = "Compliant"
    NON_COMPLIANT = "Non-Compliant"
    PARTIALLY_COMPLIANT = "Partially Compliant"
    NOT_EVALUATED = "Not Evaluated"


class ToolCall(BaseModelWrapper):
    """Model for individual tool calls made by the agent."""

    tool_name: str = Field(description="Name of the tool called")
    arguments: Optional[Dict[str, Any]] = Field(
        default=None, description="Arguments passed to the tool"
    )
    response_summary: Optional[str] = Field(
        default=None, description="Summary of the tool response"
    )
    was_successful: bool = Field(
        default=True, description="Whether the tool call was successful"
    )
    timestamp: Optional[str] = Field(
        default=None, description="Timestamp of the tool call"
    )


class FaultInfo(BaseModelWrapper):
    """Model for fault injection information."""

    fault_type: str = Field(description="Type of fault injected (e.g., Misconfig)")
    target_service: str = Field(description="Service where fault was injected")
    namespace: str = Field(description="Kubernetes namespace")


class MetricsExtractionResult(BaseModelWrapper):
    """Result of metrics extraction operation."""

    success: bool = Field(description="Whether extraction was successful")
    metrics: Optional[dict] = Field(
        default=None, description="Extracted metrics if successful"
    )
    errors: List[str] = Field(
        default_factory=list, description="List of errors encountered during extraction"
    )
    warnings: List[str] = Field(
        default_factory=list, description="List of warnings during extraction"
    )


# Pydantic models for LLM structured output
class LLMQuantitativeExtraction(BaseModelWrapper):
    """Model for LLM to extract quantitative metrics."""

    session_id: Optional[str] = Field(
        default=None, description="Session ID (UUID format)"
    )
    namespace: Optional[str] = Field(default=None, description="Kubernetes namespace")
    deployment_name: Optional[str] = Field(
        default=None, description="Deployment/application name"
    )
    fault_injection_time: Optional[str] = Field(
        default=None, description="Time of fault injection in seconds"
    )
    agent_fault_detection_time: Optional[str] = Field(
        default=None, description="timestamp when the agent detected the fault"
    )
    agent_fault_mitigation_time: Optional[str] = Field(
        default=None, description="timestamp when the agent mitigated the fault"
    )
    framework_overhead_seconds: Optional[float] = Field(
        default=None, description="Framework overhead in seconds"
    )
    fault_detected: str = Field(
        default="Unknown", description="Type of fault detected by the agent"
    )
    trajectory_steps: int = Field(
        default=0, description="Number of steps in the agent trajectory"
    )
    input_tokens: int = Field(
        default=0, description="Total number of input tokens used"
    )
    output_tokens: int = Field(
        default=0, description="Total number of output tokens used"
    )
    fault_type: Optional[str] = Field(
        default=None, description="Type of fault (e.g., Misconfig)"
    )
    fault_target_service: Optional[str] = Field(
        default=None, description="Service where fault was injected"
    )
    fault_namespace: Optional[str] = Field(
        default=None, description="Namespace of the faulty service"
    )
    tool_calls: List[Dict[str, Any]] = Field(
        default_factory=list,
        description="List of tool calls with name, arguments, success status",
    )


class LLMQualitativeExtraction(BaseModelWrapper):
    """Model for LLM to extract qualitative metrics."""

    rai_check_status: str = Field(
        default="Not Evaluated", description="'Passed', 'Failed', or 'Not Evaluated'"
    )
    rai_check_notes: Optional[str] = Field(
        default=None, description="RAI compliance notes"
    )
    trajectory_efficiency_score: Optional[float] = Field(
        default=None, description="Efficiency score 0-10"
    )
    trajectory_efficiency_notes: Optional[str] = Field(
        default=None, description="Efficiency assessment"
    )
    security_compliance_status: str = Field(
        default="Not Evaluated",
        description="'Compliant', 'Non-Compliant', 'Partially Compliant', or 'Not Evaluated'",
    )
    security_compliance_notes: Optional[str] = Field(
        default=None, description="Security compliance notes"
    )
    acceptance_criteria_met: Optional[bool] = Field(
        default=None, description="Whether acceptance criteria were met"
    )
    acceptance_criteria_notes: Optional[str] = Field(
        default=None, description="Acceptance criteria evaluation"
    )
    response_quality_score: Optional[float] = Field(
        default=None, description="Response quality score 0-10"
    )
    response_quality_notes: Optional[str] = Field(
        default=None, description="Response quality assessment"
    )
    reasoning_judgement: Optional[str] = Field(
        default=None, description="Overall reasoning judgement"
    )
    reasoning_score: Optional[int] = Field(
        default=None, description="Reasoning score 0-10"
    )
    known_limitations: List[str] = Field(
        default_factory=list, description="List of observed limitations"
    )
    recommendations: List[str] = Field(
        default_factory=list, description="List of recommendations"
    )
    agent_summary: str = Field(
        default="",
        description="A concise summary of the agent's actions and findings and remediation steps",
    )
