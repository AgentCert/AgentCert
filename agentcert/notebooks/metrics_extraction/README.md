# Metrics Extractor from Trace

## Overview

`metrics_extractor_from_trace.py` extracts **quantitative** and **qualitative** evaluation metrics from [Langfuse](https://langfuse.com/) trace files produced by IT-Ops agent runs. It uses an LLM (Azure OpenAI) to interpret raw trace spans and produces structured metrics conforming to the `LLMQuantitativeExtraction` and `LLMQualitativeExtraction` Pydantic models defined in `agentcert/data_models/metrics_model.py`.

The extractor is **trace-format-agnostic** -- it works with any Langfuse trace that shares the same key structure (id, type, name, startTime, endTime, input, output, metadata), regardless of the specific value terminologies used by different agents or experiments.

## Key Design Decisions

### Batch Processing

Large traces can contain hundreds of spans that would exceed LLM context limits if sent in a single request. The extractor splits spans into chronologically-sorted batches of **15 spans each** (`BATCH_SIZE = 15`), processes each batch independently, and then aggregates the results.

### Hybrid LLM + Code Aggregation

A critical design principle is the **separation of concerns between LLM and code**:

- **LLM handles**: Interpreting unstructured text, extracting raw values from trace spans, consolidating descriptive/narrative fields across batches.
- **Code handles**: All mathematical operations -- sums, averages, ratios, timestamp arithmetic, deduplication of lists. This avoids LLM arithmetic errors.

During aggregation, code-computed numeric values **always override** any values returned by the LLM.

## Architecture

```
                    +-----------------------+
                    |   Trace JSON File     |
                    |   (Langfuse spans)    |
                    +-----------+-----------+
                                |
                                v
                    +-----------+-----------+
                    | TraceMetricsExtractor |
                    +-----------+-----------+
                                |
                    +-----------+-----------+
                    |   _create_batches()   |
                    | (sort + chunk spans)  |
                    +-----------+-----------+
                                |
               +----------------+----------------+
               |                                 |
               v                                 v
    +----------+----------+           +----------+----------+
    |  Quantitative Path  |           |  Qualitative Path   |
    +----------+----------+           +----------+----------+
               |                                 |
               v                                 v
    Per-batch LLM extraction          Per-batch LLM extraction
    (_extract_batch_quantitative)     (_extract_batch_qualitative)
               |                                 |
               v                                 v
    Code aggregation                  Code aggregation
    (_aggregate_quantitative_in_code) (_aggregate_qualitative_in_code)
         +                                  +
    LLM text consolidation            LLM text synthesis
    (_aggregate_quantitative_metrics) (_aggregate_qualitative_metrics)
               |                                 |
               +----------------+----------------+
                                |
                                v
                    +-----------+-----------+
                    |   ExtractionResult    |
                    | (quant + qual + usage)|
                    +-----------+-----------+
                                |
                         (optional)
                                v
                    +-----------+-----------+
                    |       MongoDB         |
                    +-----------------------+
```

## Data Classes

### `TokenUsage`

Tracks cumulative LLM token consumption (input, output, total) across all LLM calls during a single extraction run.

### `ExtractionResult`

Wraps the final output:

| Field                | Type                        | Description                                    |
|----------------------|-----------------------------|------------------------------------------------|
| `quantitative`       | `LLMQuantitativeExtraction` | Structured quantitative metrics                |
| `qualitative`        | `LLMQualitativeExtraction`  | Structured qualitative metrics                 |
| `token_usage`        | `TokenUsage`                | Total tokens consumed by the extraction        |
| `mongodb_document_id`| `Optional[str]`             | MongoDB document ID if persistence was enabled |

## Extracted Metrics

### Quantitative Metrics (`LLMQuantitativeExtraction`)

| Category                  | Fields                                                                                                         |
|---------------------------|----------------------------------------------------------------------------------------------------------------|
| **Experiment**            | `experiment_id`                                                                                                |
| **Time**                  | `fault_injection_time`, `agent_fault_detection_time`, `agent_fault_mitigation_time`, `time_to_detect`, `time_to_mitigate`, `framework_overhead_seconds` |
| **Fault Info**            | `fault_detected`, `fault_type`, `fault_target_service`, `fault_namespace`                                      |
| **Trajectory**            | `trajectory_steps`, `input_tokens`, `output_tokens`                                                            |
| **Tool Calls**            | `tool_calls` (list of tool name, arguments, success status, response summary, timestamp)                       |
| **Security**              | `pii_detection`, `average_time_for_pii_detection_seconds`, `number_of_pii_instances_detected`, `pii_redaction_percentage`, `authentication_success_rate`, `non_authentication_access`, `malicious_prompts_detected` |
| **Ground-Truth Accuracy** | `tool_selection_accuracy`, `action_correctness`, `argument_accuracy`, `optimal_toolcall_deviations`, `action_efficiency` |

### Qualitative Metrics (`LLMQualitativeExtraction`)

| Category                | Fields                                                                                   |
|-------------------------|------------------------------------------------------------------------------------------|
| **RAI Compliance**      | `rai_check_status`, `rai_check_notes`                                                    |
| **Trajectory Efficiency** | `trajectory_efficiency_score` (0-10), `trajectory_efficiency_notes`                    |
| **Privacy**             | `Anonymization_implementation`, `pii_detection`                                          |
| **Security**            | `security_compliance_status`, `security_compliance_notes`                                |
| **Content Safety**      | `content_safety_check`, `content_safety_notes`                                           |
| **Acceptance Criteria** | `acceptance_criteria_met`, `acceptance_criteria_notes`                                   |
| **Response Quality**    | `response_quality_score` (0-10), `response_quality_notes`                                |
| **Reasoning**           | `reasoning_judgement`, `reasoning_score` (0-10)                                          |
| **Hallucination**       | `hallucination_detection`, `hallucination_score` (0-1)                                   |
| **Behavioural**         | `plan_adherence`, `collateral_damage`                                                    |
| **Summary**             | `agent_summary`, `known_limitations`, `recommendations`                                  |

## Aggregation Logic

### Quantitative Code Aggregation (`_aggregate_quantitative_in_code`)

| Strategy             | Fields                                                                                                         |
|----------------------|----------------------------------------------------------------------------------------------------------------|
| **First non-null**   | `experiment_id`, `fault_injection_time`, `agent_fault_detection_time`, `agent_fault_mitigation_time`, `fault_type`, `fault_target_service`, `fault_namespace`, `framework_overhead_seconds` |
| **Longest string**   | `fault_detected` (picks the most detailed description)                                                         |
| **Sum across batches** | `input_tokens`, `output_tokens`, `number_of_pii_instances_detected`, `malicious_prompts_detected`, `non_authentication_access`, `optimal_toolcall_deviations` |
| **Boolean OR**       | `pii_detection`                                                                                                |
| **List merge**       | `tool_calls` (concatenated from all batches)                                                                   |
| **Ratio (num/den)**  | `tool_selection_accuracy`, `action_correctness`, `argument_accuracy`, `action_efficiency`, `authentication_success_rate`, `pii_redaction_percentage` |
| **Timestamp diff**   | `time_to_detect` (detection - injection), `time_to_mitigate` (mitigation - injection), `average_time_for_pii_detection_seconds` |

### Qualitative Code Aggregation (`_aggregate_qualitative_in_code`)

| Strategy               | Fields                                                              |
|------------------------|---------------------------------------------------------------------|
| **Average scores**     | `trajectory_efficiency_score`, `response_quality_score`, `reasoning_score` |
| **Ratio (count-based)**| `hallucination_score` (hallucination_count / total_response_count)   |
| **Boolean OR**         | `hallucination_detection`, `pii_detection`                          |
| **Boolean AND**        | `acceptance_criteria_met` (true only if ALL batches met criteria)   |
| **Deduplicated merge** | `known_limitations`, `recommendations`                              |

## Dependencies

| Package              | Purpose                                     |
|----------------------|---------------------------------------------|
| `asyncio`            | Async LLM calls                             |
| `pydantic`           | Structured data models for metrics          |
| `azure_openai_util`  | Azure OpenAI LLM client (structured output) |
| `mongodb_util`       | Optional MongoDB persistence                |
| `load_config`        | Configuration loading from `configs.json`   |
| `setup_logging`      | Centralized logging                         |

All project-internal dependencies (`utils.*`, `data_models.*`) are imported with graceful fallback -- the module can be imported standalone (with reduced functionality) if the full project environment is unavailable.

## Usage

### Command Line

```bash
# Basic extraction (prints results to stdout)
python metrics_extractor_from_trace.py <trace_file.json>

# Extract and store to MongoDB
python metrics_extractor_from_trace.py <trace_file.json> --store
```

### Programmatic (Synchronous)

```python
from metrics_extractor_from_trace import extract_metrics_from_trace

result = extract_metrics_from_trace("path/to/trace.json")

print(result.quantitative.time_to_detect)
print(result.qualitative.agent_summary)
print(result.token_usage.total_tokens)
```

### Programmatic (Async)

```python
import asyncio
from metrics_extractor_from_trace import extract_metrics_from_trace_async

result = asyncio.run(
    extract_metrics_from_trace_async(
        "path/to/trace.json",
        store_to_mongodb=True,
    )
)

print(f"MongoDB doc ID: {result.mongodb_document_id}")
```

### With Custom Configuration

```python
from metrics_extractor_from_trace import TraceMetricsExtractor

config = {
    "extraction_model": {
        "endpoint": "https://my-openai.openai.azure.com/",
        "api_key": "...",
        "deployment": "gpt-4o",
    },
    "mongodb": {
        "connection_string": "mongodb+srv://..."
    }
}

extractor = TraceMetricsExtractor(config=config)
result = extractor.extract_metrics("trace.json", store_to_mongodb=True)
```

## Input Format

The extractor expects a JSON file containing an array of Langfuse span objects. Each span should have:

```json
[
  {
    "id": "span-uuid",
    "type": "SPAN | GENERATION",
    "name": "span-label",
    "startTime": "2025-01-15T10:30:00.000Z",
    "endTime": "2025-01-15T10:30:05.000Z",
    "input": "{\"role\": \"user\", \"content\": \"...\"}",
    "output": "{\"role\": \"assistant\", \"content\": \"...\"}",
    "metadata": "{\"action_type\": \"diagnose\", \"confidence\": 0.95}"
  }
]
```

## Output Format

The `ExtractionResult.to_dict()` method produces:

```json
{
  "quantitative": {
    "experiment_id": "exp-001",
    "time_to_detect": 5.2,
    "time_to_mitigate": 81.3,
    "fault_detected": "pod-delete transient",
    "trajectory_steps": 45,
    "input_tokens": 12500,
    "output_tokens": 3200,
    "tool_calls": [...],
    "tool_selection_accuracy": 0.85,
    "action_efficiency": 0.72,
    ...
  },
  "qualitative": {
    "rai_check_status": "Passed",
    "trajectory_efficiency_score": 7.5,
    "security_compliance_status": "Compliant",
    "hallucination_detection": false,
    "agent_summary": "The agent detected a pod-delete fault...",
    "known_limitations": ["Limited log depth analysis"],
    "recommendations": ["Increase diagnostic breadth"],
    ...
  },
  "token_usage": {
    "input_tokens": 45000,
    "output_tokens": 8500,
    "total_tokens": 53500
  }
}
```

## MongoDB Storage

When `store_to_mongodb=True`, the extractor persists metrics using the synchronous `pymongo` client (to avoid `motor` async cleanup issues at interpreter shutdown). The document includes:

- Full quantitative and qualitative metrics
- Metadata: trace file name, total span count, extraction token usage

The MongoDB client is initialized lazily and closed immediately after insertion.
