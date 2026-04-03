# Fault Profiling API
**API 1: POST /api/v1/bucketing-extraction**

---

## Overview

This API extracts fault buckets and metrics from a single agent run trace. It performs fault classification using LLM batch processing, extracts quantitative and qualitative metrics, and stores results in blob storage with optional MongoDB metadata.

---

## API: POST /api/v1/bucketing-extraction

### Purpose
Extract fault buckets and metrics from a single trace for fault profiling and analysis.

### Request

```json
{
  "agent_id": "agent_v2_4_1",
  "experiment_id": "exp_001",
  "run_id": "run_001",
  "trace_file_path": "s3://bucket/traces/trace.json",
  "llm_batch_size": 5,
  "storage_config": {
    "type": "blob_storage | mongodb | hybrid",
    "container_name": "cert-artifacts"
  }
}
```

**Parameters:**
- `agent_id` (required): Agent identifier
- `experiment_id` (required): Experiment identifier
- `run_id` (required): Run identifier
- `trace_file_path` (required): Path to trace file (local or remote)
- `llm_batch_size` (optional): Batch size for LLM fault classification (default: 5)
- `storage_config.type` (required): `blob_storage` | `mongodb` | `hybrid`
- `storage_config.container_name` (required): Azure blob container or MongoDB database name

**Note:**
- API loads trace from file_path as dictionary
- Storage credentials loaded from application config/KeyVault at startup

### Response (Success - 200)

```json
{
  "status": "success",
  "data": {
    "extraction_id": "uuid",
    "agent_id": "agent_v2_4_1",
    "experiment_id": "exp_001",
    "run_id": "run_001",
    "total_logs": 1523,
    "total_faults_detected": 3,
    "faults": [
      {
        "fault_id": "pod-delete",
        "fault_name": "Pod Deletion",
        "severity": "critical",
        "status": "closed",
        "detected_at": "2026-04-03T10:30:02Z",
        "mitigated_at": "2026-04-03T10:35:10Z"
      }
    ],
    "storage_paths": {
      "fault_buckets_dir": "cert-artifacts/agent_v2_4_1/exp_001/run_001/fault_buckets/",
      "metrics_dir": "cert-artifacts/agent_v2_4_1/exp_001/run_001/metrics/",
      "manifest": "cert-artifacts/agent_v2_4_1/exp_001/run_001/manifest.json",
      "logs": "cert-artifacts/agent_v2_4_1/exp_001/run_001/pipeline.log"
    }
  }
}
```

### Response (Error - 400/500)

```json
{
  "status": "error",
  "error_code": "INVALID_REQUEST | TRACE_NOT_FOUND | BUCKETING_FAILED | METRICS_EXTRACTION_FAILED | STORAGE_ERROR | MONGODB_ERROR",
  "message": "Human-readable error message",
  "details": {
    "failed_stage": "validation | trace_loading | bucketing | metrics_extraction | storage | mongodb_storage",
    "error": "..."
  }
}
```

---

## Processing Steps

**Input:** agent_id, experiment_id, run_id, trace_file_path, llm_batch_size, storage_config

**Step 1: Validate Input**
- Validate: agent_id, experiment_id, run_id, trace_file_path provided; storage_config valid
- Output: Validated inputs
- Logs: "Input validation passed"

**Step 2: Load Trace**
- Input: trace_file_path
- Process:
  - Load trace JSON/dict from file_path
  - Parse trace structure
- **Output:** trace dict
- Logs: "Loaded trace", "Total logs: {count}"

**Step 3: Fault Bucketing**
- Input: trace dict, llm_batch_size
- Process:
  - Extract FAULT_DATA events from trace logs
  - Batch events by llm_batch_size
  - Classify each batch via LLM to identify fault types and severities
  - Group logs into fault buckets by fault_id
- **Output:** Dict[fault_id → FaultBucket]
- Logs: "Bucketed {n} events into {m} fault categories"

**Step 4: Metric Extraction**
- Input: Fault buckets dict, trace dict
- Process:
  - For each fault bucket:
    - Extract quantitative metrics (trajectory steps, tokens, tool calls, timing)
    - Extract qualitative metrics (action correctness, response quality, reasoning quality, hallucination detection, PII detection)
    - Support LLM-based extraction with token tracking
- **Output:** Dict[fault_id → ExtractedMetrics]
- Logs: "Extracted metrics for {n} faults", "Total tokens used: {count}"

**Step 5: Store Artifacts to Blob Storage**
- Store: fault_buckets/ → `{container}/{agent_id}/{experiment_id}/{run_id}/fault_buckets/bucket_{fault_id}.json`
- Store: metrics/ → `{container}/{agent_id}/{experiment_id}/{run_id}/metrics/metrics_{fault_id}.json`
- Store: manifest.json → `{container}/{agent_id}/{experiment_id}/{run_id}/manifest.json`
- Store: pipeline.log → `{container}/{agent_id}/{experiment_id}/{run_id}/pipeline.log`
- Logs: "Stored fault buckets", "Stored metrics"

**Step 6: Store Metadata to MongoDB** (if storage_config.type = mongodb | hybrid)
- Create ExtractionMetadata document with storage_paths, llm_tokens, processing_time, fault_count
- Create FaultMetadata documents (one per fault) with fault details, blob_paths, quantitative_summary
- Logs: "Stored extraction metadata"

**Step 7: Return Response**
- Collect artifact paths
- Return extraction_id and storage_paths
- Return HTTP 200

---

## MongoDB Schema

### Collection: ExtractionMetadata
Stores metadata for each bucketing-extraction run.

```json
{
  "_id": "ObjectId",
  "extraction_id": "uuid",
  "experiment_id": "exp_001",
  "run_id": "run_001",
  "agent_id": "agent_v2_4_1",
  "status": "success | failed",
  "created_at": "2026-04-03T10:30:00Z",
  "storage_paths": {
    "fault_buckets_dir": "cert-artifacts/agent_v2_4_1/exp_001/run_001/fault_buckets/",
    "metrics_dir": "cert-artifacts/agent_v2_4_1/exp_001/run_001/metrics/",
    "manifest": "cert-artifacts/agent_v2_4_1/exp_001/run_001/manifest.json",
    "logs": "cert-artifacts/agent_v2_4_1/exp_001/run_001/pipeline.log"
  },
  "llm_tokens": {
    "input_tokens": 4250,
    "output_tokens": 1850,
    "total_tokens": 6100
  },
  "processing_time_seconds": 45.23,
  "fault_count": 3,
  "error_message": null
}
```

**Indexes:**
- `{experiment_id: 1, run_id: 1, agent_id: 1}` (compound)
- `{agent_id: 1, created_at: -1}` (for history)
- `{extraction_id: 1}` (unique)

---

### Collection: FaultMetadata
Stores metadata for each detected fault.

```json
{
  "_id": "ObjectId",
  "fault_id": "pod-delete",
  "extraction_id": "uuid",
  "experiment_id": "exp_001",
  "run_id": "run_001",
  "agent_id": "agent_v2_4_1",
  "fault_name": "pod-delete",
  "severity": "critical",
  "target_pod": "pod-1",
  "namespace": "default",
  "status": "closed",
  "event_count": 35,
  "detected_at": "2026-04-03T10:30:02Z",
  "mitigated_at": "2026-04-03T10:35:10Z",
  "blob_paths": {
    "bucket_file": "cert-artifacts/agent_v2_4_1/exp_001/run_001/fault_buckets/bucket_pod-delete.json",
    "metrics_file": "cert-artifacts/agent_v2_4_1/exp_001/run_001/metrics/metrics_pod-delete.json"
  },
  "quantitative_summary": {
    "trajectory_steps": 35,
    "input_tokens": 1200,
    "output_tokens": 450,
    "tool_calls": 8
  },
  "created_at": "2026-04-03T10:30:00Z"
}
```

**Indexes:**
- `{experiment_id: 1, run_id: 1, agent_id: 1}` (compound)
- `{agent_id: 1, extraction_id: 1}` (compound)
- `{fault_id: 1}` (for fault lookup)
- `{created_at: -1}` (for sorting)

---

## Error Codes

| Code | HTTP | Cause | Action |
|------|------|-------|--------|
| INVALID_REQUEST | 400 | Missing required fields or invalid format | Validate request body |
| TRACE_NOT_FOUND | 404 | Trace file not found at path | Verify trace_file_path |
| BUCKETING_FAILED | 500 | Fault bucketing failed | Check LLM availability and trace format |
| METRICS_EXTRACTION_FAILED | 500 | Metric extraction failed | Check trace data quality |
| STORAGE_ERROR | 500 | Blob storage operation failed | Check storage credentials |
| MONGODB_ERROR | 500 | MongoDB operation failed | Check MongoDB connection |

---

## Storage Structure

```
{container_name}/{agent_id}/{experiment_id}/{run_id}/
├── fault_buckets/
│   ├── bucket_pod-delete.json
│   ├── bucket_network-loss.json
│   └── ...
├── metrics/
│   ├── metrics_pod-delete.json
│   ├── metrics_network-loss.json
│   └── ...
├── manifest.json
└── pipeline.log
```
