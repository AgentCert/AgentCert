"""
Metrics Extraction Package for IT-Ops Agent Certification.

This package provides tools for extracting quantitative and qualitative
metrics from IT-Ops agent run reports using LLMs (Azure OpenAI).

Main Components:
    - MetricsExtractor: Main class for extracting metrics from report files
    - extract_metrics_with_llm: Async function for LLM-based extraction (recommended)
    - extract_metrics_from_file: Sync function for regex-based extraction (fallback)

Usage (LLM-based - recommended):
    import asyncio
    from notebooks.metrics_extraction import extract_metrics_with_llm

    result = asyncio.run(extract_metrics_with_llm("path/to/pid_run_1.txt"))
    if result.success:
        metrics = result.metrics
        print(f"TTD: {metrics.quantitative.time_to_detection_seconds}")
        print(f"Recommendations: {metrics.qualitative.recommendations}")

Usage (Regex-based fallback):
    from notebooks.metrics_extraction import extract_metrics_from_file

    result = extract_metrics_from_file("path/to/pid_run_1.txt")
    if result.success:
        metrics = result.metrics
        print(f"TTD: {metrics.quantitative.time_to_detection_seconds}")
"""

from notebooks.metrics_extraction.metrics_extractor import (  # LLM-based extraction (recommended); Regex-based extraction (fallback); LLM extraction models
    LLMQualitativeExtraction,
    LLMQuantitativeExtraction,
    MetricsExtractor,
    extract_metrics_from_file,
    extract_metrics_from_multiple_files,
    extract_metrics_from_multiple_files_with_llm,
    extract_metrics_with_llm,
)

__all__ = [
    "MetricsExtractor",
    # LLM-based extraction (recommended)
    "extract_metrics_with_llm",
    "extract_metrics_from_multiple_files_with_llm",
    # Regex-based extraction (fallback)
    "extract_metrics_from_file",
    "extract_metrics_from_multiple_files",
    # LLM extraction models
    "LLMQuantitativeExtraction",
    "LLMQualitativeExtraction",
]
