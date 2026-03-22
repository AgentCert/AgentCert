"""
Metrics Extraction Package for IT-Ops Agent Certification.

This package provides tools for extracting quantitative and qualitative
metrics from IT-Ops agent run traces using LLMs (Azure OpenAI).

Main Components:
    - TraceMetricsExtractor: Extracts metrics from Langfuse trace files
    - FaultBucketingPipeline: Splits interleaved traces into per-fault buckets
    - AggregationOrchestrator: Aggregates per-run metrics into scorecards

Usage:
    import asyncio
    from notebooks.metrics_extraction import TraceMetricsExtractor

    extractor = TraceMetricsExtractor(fault_config)
    result = asyncio.run(extractor.extract("path/to/trace.json"))
"""

from notebooks.metrics_extraction.metric_extraction_from_trace import (
    TraceMetricsExtractor,
    ExtractionResult,
    TokenUsage,
    extract_metrics_from_trace,
    extract_metrics_from_trace_async,
)
from notebooks.metrics_extraction.fault_bucketing import FaultBucketingPipeline
from notebooks.metrics_extraction.aggregation import AggregationOrchestrator

__all__ = [
    "TraceMetricsExtractor",
    "ExtractionResult",
    "TokenUsage",
    "extract_metrics_from_trace",
    "extract_metrics_from_trace_async",
    "FaultBucketingPipeline",
    "AggregationOrchestrator",
]
