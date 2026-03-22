"""
Aggregation package for AgentCert metrics.

Provides fault-category and agent-level scorecard aggregation
from per-run metrics stored in MongoDB.
"""

from .aggregation import AggregationOrchestrator, MetricsQueryService, ScorecardAssembler, ScorecardStorage
from .llm_council import LLMCouncil
from .numeric_aggregation import (
    compute_boolean_aggregates,
    compute_derived_rates,
    compute_numeric_aggregates,
    compute_stats,
)

__all__ = [
    "AggregationOrchestrator",
    "MetricsQueryService",
    "ScorecardAssembler",
    "ScorecardStorage",
    "LLMCouncil",
    "compute_boolean_aggregates",
    "compute_derived_rates",
    "compute_numeric_aggregates",
    "compute_stats",
]
