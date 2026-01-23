"""
Pydantic models for IT-Ops Agent evaluation metrics extraction.
Extracts both quantitative and qualitative metrics from agent run reports.
"""

import re
from datetime import datetime
from enum import Enum
from pathlib import Path
from typing import Any, Dict, List, Optional, Union

from pydantic import BaseModel, Field, computed_field


class DetectionAccuracy(str, Enum):
    """Enum for detection accuracy status."""

    CORRECT = "Correct"
    INCORRECT = "Incorrect"
    UNKNOWN = "Unknown"


class SubmissionStatus(str, Enum):
    """Enum for submission status."""

    VALID = "VALID_SUBMISSION"
    INVALID = "INVALID_SUBMISSION"
    NOT_SUBMITTED = "NOT_SUBMITTED"


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


class ToolCall(BaseModel):
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


class FaultInfo(BaseModel):
    """Model for fault injection information."""

    fault_type: str = Field(description="Type of fault injected (e.g., Misconfig)")
    target_service: str = Field(description="Service where fault was injected")
    namespace: str = Field(description="Kubernetes namespace")


class QuantitativeMetrics(BaseModel):
    """
    Quantitative metrics extracted from the agent run report.
    These are measurable, numeric metrics.
    """

    # Time metrics
    time_to_detection_seconds: Optional[float] = Field(
        default=None, description="Time taken to detect the anomaly (TTD) in seconds"
    )
    time_to_mitigation_seconds: Optional[float] = Field(
        default=None, description="Time taken to mitigate/fix the issue in seconds"
    )
    framework_overhead_seconds: Optional[float] = Field(
        default=None, description="Framework overhead time in seconds"
    )

    # Success/Failure metrics
    detection_accuracy: DetectionAccuracy = Field(
        default=DetectionAccuracy.UNKNOWN,
        description="Whether the detection was correct or incorrect",
    )
    submission_status: SubmissionStatus = Field(
        default=SubmissionStatus.NOT_SUBMITTED, description="Status of the submission"
    )

    @computed_field
    @property
    def success_rate(self) -> float:
        """Calculate success rate based on detection accuracy."""
        if self.detection_accuracy == DetectionAccuracy.CORRECT:
            return 1.0
        elif self.detection_accuracy == DetectionAccuracy.INCORRECT:
            return 0.0
        return 0.0

    @computed_field
    @property
    def failure_rate(self) -> float:
        """Calculate failure rate based on detection accuracy."""
        return 1.0 - self.success_rate

    # Trajectory metrics
    trajectory_steps: int = Field(
        default=0, description="Number of steps/actions taken in the trajectory"
    )
    tool_calls_total: int = Field(
        default=0, description="Total number of tool calls made"
    )
    successful_tool_calls: int = Field(
        default=0, description="Number of successful tool calls"
    )
    failed_tool_calls: int = Field(default=0, description="Number of failed tool calls")

    @computed_field
    @property
    def tool_call_accuracy(self) -> float:
        """Calculate tool call accuracy as ratio of successful calls to total calls."""
        if self.tool_calls_total == 0:
            return 0.0
        return self.successful_tool_calls / self.tool_calls_total

    # Token usage metrics
    input_tokens: int = Field(default=0, description="Number of input tokens consumed")
    output_tokens: int = Field(
        default=0, description="Number of output tokens generated"
    )

    @computed_field
    @property
    def total_tokens(self) -> int:
        """Calculate total token usage."""
        return self.input_tokens + self.output_tokens

    # Cost metrics (estimated based on token usage)
    estimated_cost_usd: Optional[float] = Field(
        default=None, description="Estimated cost in USD based on token usage"
    )

    def calculate_cost(
        self,
        input_token_cost_per_1k: float = 0.005,
        output_token_cost_per_1k: float = 0.015,
    ) -> float:
        """
        Calculate estimated cost based on token usage.
        Default rates are for GPT-4o model.
        """
        input_cost = (self.input_tokens / 1000) * input_token_cost_per_1k
        output_cost = (self.output_tokens / 1000) * output_token_cost_per_1k
        self.estimated_cost_usd = round(input_cost + output_cost, 6)
        return self.estimated_cost_usd


class QualitativeMetrics(BaseModel):
    """
    Qualitative metrics extracted from the agent run report.
    These are descriptive, assessment-based metrics.
    """

    # RAI Check
    rai_check_status: RAICheckStatus = Field(
        default=RAICheckStatus.NOT_EVALUATED, description="Responsible AI check status"
    )
    rai_check_notes: Optional[str] = Field(
        default=None, description="Notes or details about RAI compliance"
    )

    # Trajectory Efficiency
    trajectory_efficiency_score: Optional[float] = Field(
        default=None, ge=0, le=10, description="Score for trajectory efficiency (0-10)"
    )
    trajectory_efficiency_notes: Optional[str] = Field(
        default=None, description="Assessment of trajectory efficiency"
    )

    # Security and Compliance
    security_compliance_status: SecurityComplianceStatus = Field(
        default=SecurityComplianceStatus.NOT_EVALUATED,
        description="Security and compliance status",
    )
    security_compliance_notes: Optional[str] = Field(
        default=None, description="Notes about security and compliance assessment"
    )

    # Acceptance Criteria Evaluation
    acceptance_criteria_met: Optional[bool] = Field(
        default=None, description="Whether acceptance criteria were met"
    )
    acceptance_criteria_notes: Optional[str] = Field(
        default=None, description="Evaluation of acceptance criteria"
    )

    # Quality of Response
    response_quality_score: Optional[float] = Field(
        default=None,
        ge=0,
        le=10,
        description="Score for quality of response and explanation (0-10)",
    )
    response_quality_notes: Optional[str] = Field(
        default=None, description="Assessment of response quality"
    )

    # Reasoning Judgement (from evaluation)
    reasoning_judgement: Optional[str] = Field(
        default=None, description="Detailed reasoning judgement from evaluation"
    )
    reasoning_score: Optional[int] = Field(
        default=None, ge=0, le=10, description="Reasoning score from evaluation (0-10)"
    )

    # Known Limitations
    known_limitations: List[str] = Field(
        default_factory=list,
        description="List of known limitations and failure modes identified",
    )

    # Recommendations
    recommendations: List[str] = Field(
        default_factory=list, description="List of recommendations for improvement"
    )


class AgentRunMetrics(BaseModel):
    """
    Complete metrics model for an IT-Ops agent run.
    Combines session info, fault info, quantitative and qualitative metrics.
    """

    # Session Information
    session_id: Optional[str] = Field(
        default=None, description="Unique session identifier"
    )
    run_file_path: Optional[str] = Field(
        default=None, description="Path to the source run file"
    )
    extraction_timestamp: str = Field(
        default_factory=lambda: datetime.now().isoformat(),
        description="Timestamp when metrics were extracted",
    )

    # Environment Information
    namespace: Optional[str] = Field(
        default=None, description="Kubernetes namespace used"
    )
    deployment_name: Optional[str] = Field(
        default=None, description="Name of the deployed application"
    )

    # Fault Information
    fault_info: Optional[FaultInfo] = Field(
        default=None, description="Information about the injected fault"
    )

    # Tool calls made during the run
    tool_calls: List[ToolCall] = Field(
        default_factory=list, description="List of tool calls made during the run"
    )

    # Quantitative Metrics
    quantitative: QuantitativeMetrics = Field(
        default_factory=QuantitativeMetrics, description="Quantitative metrics"
    )

    # Qualitative Metrics
    qualitative: QualitativeMetrics = Field(
        default_factory=QualitativeMetrics, description="Qualitative metrics"
    )

    def to_summary_dict(self) -> Dict[str, Any]:
        """Return a summary dictionary of key metrics."""
        return {
            "session_id": self.session_id,
            "detection_accuracy": self.quantitative.detection_accuracy.value,
            "time_to_detection_seconds": self.quantitative.time_to_detection_seconds,
            "success_rate": self.quantitative.success_rate,
            "failure_rate": self.quantitative.failure_rate,
            "trajectory_steps": self.quantitative.trajectory_steps,
            "tool_call_accuracy": self.quantitative.tool_call_accuracy,
            "total_tokens": self.quantitative.total_tokens,
            "estimated_cost_usd": self.quantitative.estimated_cost_usd,
            "reasoning_score": self.qualitative.reasoning_score,
            "fault_service": (
                self.fault_info.target_service if self.fault_info else None
            ),
        }


class MetricsExtractionResult(BaseModel):
    """Result of metrics extraction operation."""

    success: bool = Field(description="Whether extraction was successful")
    metrics: Optional[AgentRunMetrics] = Field(
        default=None, description="Extracted metrics if successful"
    )
    errors: List[str] = Field(
        default_factory=list, description="List of errors encountered during extraction"
    )
    warnings: List[str] = Field(
        default_factory=list, description="List of warnings during extraction"
    )
