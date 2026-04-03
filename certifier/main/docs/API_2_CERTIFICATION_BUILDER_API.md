# Certification Builder API
**API 2: POST /api/v1/aggregation-certification**

---

## Overview

This API aggregates metrics from multiple runs, calculates certification scores, and generates certification documents (JSON, HTML, PDF).

---

## API: POST /api/v1/aggregation-certification

### Purpose
Aggregate extracted metrics across multiple runs for an agent-experiment combination, generate certification scores, and render certification reports.

### Request

```json
{
  "agent_id": "agent_v2_4_1",
  "agent_name": "Agent V2.4.1",
  "experiment_id": "exp_001",
  "certification_run_id": "cert_run_001",
  "runs_per_fault": 30,
  "storage_config": {
    "type": "blob_storage | mongodb | hybrid",
    "container_name": "cert-artifacts"
  }
}
```

**Parameters:**
- `agent_id` (required): Agent identifier
- `agent_name` (required): Human-readable agent name for certification scorecard
- `experiment_id` (required): Experiment identifier
- `certification_run_id` (optional): Certification run identifier
- `runs_per_fault` (optional): Expected runs per fault (default: 30)
- `storage_config.type` (required): `blob_storage` | `mongodb` | `hybrid`
- `storage_config.container_name` (required): Azure blob container or MongoDB database name

**Note:**
- API automatically fetches all per-run metric documents for the agent_id-experiment_id combination
- Storage credentials loaded from application config/KeyVault at startup
- For MongoDB: uses experiment_id to filter metrics; for blob storage: uses container_name


### Response (Success - 200)

```json
{
  "status": "success",
  "data": {
    "agent_id": "agent_v2_4_1",
    "agent_name": "Agent V2.4.1",
    "certification_run_id": "cert_run_001",
    "total_documents": 30,
    "total_fault_categories": 3,
    "fault_categories": ["critical", "high", "medium"],
    "storage_paths": {
      "aggregated_scorecard": "cert-artifacts/agent_v2_4_1/exp_001/aggregated_scorecard.json",
      "certification_report": "cert-artifacts/agent_v2_4_1/exp_001/certification_report.json",
      "html_report": "cert-artifacts/agent_v2_4_1/exp_001/certification_report.html",
      "pdf_report": "cert-artifacts/agent_v2_4_1/exp_001/certification_report.pdf",
      "logs": "cert-artifacts/agent_v2_4_1/exp_001/pipeline.log"
    }
  }
}
```

### Response (Error - 400/500)

```json
{
  "status": "error",
  "error_code": "INVALID_REQUEST | METRICS_NOT_FOUND | AGGREGATION_FAILED | CERT_GENERATION_FAILED | RENDERING_ERROR | STORAGE_ERROR",
  "message": "Human-readable error message",
  "details": {
    "failed_stage": "metrics_fetch | aggregation | certification_building | rendering | storage | mongodb_storage",
    "error": "..."
  }
}
```

---

## Processing Steps

**Input:** agent_id, agent_name, experiment_id, certification_run_id (optional), runs_per_fault (optional), storage_config

**Step 1: Validate Input**
- Validate: agent_id, agent_name, experiment_id provided; storage_config valid
- Note: certification_run_id and runs_per_fault are optional metadata
- Output: Validated inputs
- Logs: "Input validation passed"

**Step 2: Fetch All Extracted Metrics**
- Input: agent_id, experiment_id, storage_config
- Process:
  - **If storage_type = mongodb:**
    ```
    db.ExtractionMetadata.find({
      agent_id: "agent_v2_4_1",
      experiment_id: "exp_001"
    })
    ```
  - **If storage_type = blob_storage | hybrid:**
    - Discover metrics.json files in: `{container}/{agent_id}/{experiment_id}/`
    - Load and parse each file
- **Output:** List[PerRunMetrics]
  ```
  [
    {
      "run_id": "run_001",
      "agent_id": "agent_v2_4_1",
      "fault_category": "critical",
      "quantitative": {...metrics...},
      "qualitative": {...metrics...}
    },
    {
      "run_id": "run_002",
      "agent_id": "agent_v2_4_1",
      "fault_category": "critical",
      "quantitative": {...metrics...},
      "qualitative": {...metrics...}
    },
    {
      "run_id": "run_003",
      "agent_id": "agent_v2_4_1",
      "fault_category": "high",
      "quantitative": {...metrics...},
      "qualitative": {...metrics...}
    }
  ]
  ```

- Logs: "Queried metrics", "Found {m} metric documents"

**Step 3: Aggregation**
- Input:
  - `agent_id`, `agent_name`, `experiment_id`, `certification_run_id`, `runs_per_fault`
  - Per-run metric documents from Step 2
- Process:
  - Get distinct fault_categories from documents
  - For each fault_category:
    - Group documents by fault_category
    - Compute numeric aggregates: mean, median, std_dev, p95, min, max
    - Compute derived rates: detection success, mitigation success, compliance rates
    - Compute boolean aggregates: PII detection, hallucination detection rates
    - **LLM Council:** Synthesize textual summaries (rai_check, security, recommendations)
    - Assemble fault_category scorecard

- **Output:** Dict[fault_category → AggregatedMetrics]
  ```json
  {
    "critical": {
      "total_runs": 3,
      "faults_tested": ["disk-fill"],
      "numeric_metrics": {...},
      "derived_metrics": {
        "fault_detection_success_rate": 0.25,
        "fault_mitigation_success_rate": 1.0,
        "rai_compliance_rate": 1.0,
        "security_compliance_rate": 1.0
      },
      "boolean_status_metrics": {...},
      "textual_metrics": {
        "rai_check_summary": {...},
        "security_compliance_summary": {...},
        "known_limitations": [...],
        "recommendations": [...]
      }
    },
    "high": {...},
    "medium": {...}
  }
  ```

- Logs: "Aggregated metrics for {n} fault categories"

**Step 4: Build Certification JSON**
- Input: Aggregated fault-category scorecards from Step 3
- Process:
  - Apply certification logic to aggregated metrics, calculate scores and levels
  - Generate certification JSON with agent_id, experiment_id, timestamp, and other metadata
- **Output:** CertificationJSON (see sample: `certifier/cert_builder/data/output/certification_report.json`)
- Logs: "Certification built"

**Step 5: Render Certification Reports**
- Input: CertificationJSON
- Process:
  - Render HTML report from template
  - Render PDF from HTML
- **Output:**
  - certification_report.html
  - certification_report.pdf
- Logs: "Rendered reports"

**Step 6: Store Artifacts to Blob Storage**
- Store: aggregated_scorecard.json → `{container}/{agent_id}/{experiment_id}/aggregated_scorecard.json`
- Store: certification_report.json → `{container}/{agent_id}/{experiment_id}/certification_report.json`
- Store: certification_report.html → `{container}/{agent_id}/{experiment_id}/certification_report.html`
- Store: certification_report.pdf → `{container}/{agent_id}/{experiment_id}/certification_report.pdf`
- Store: pipeline.log → `{container}/{agent_id}/{experiment_id}/pipeline.log`
- Logs: "Stored artifacts"

**Step 7: Store Metadata to MongoDB** (if storage_config.type = mongodb | hybrid)
- Create CertificationMetadata document with storage_paths, summary, processing_time
- Create AggregatedCategoryMetadata documents (one per fault_category) with numeric and derived metrics
- Logs: "Stored certification metadata"

**Step 8: Return Response**
- Collect artifact paths
- Return certification_id and storage_paths
- Return HTTP 200

---

## MongoDB Schema

### Collection: CertificationMetadata
Stores metadata for each certification run.

```json
{
  "_id": "ObjectId",
  "certification_id": "uuid",
  "agent_id": "agent_v2_4_1",
  "agent_name": "Agent V2.4.1",
  "experiment_id": "exp_001",
  "certification_run_id": "cert_run_001",
  "status": "success | failed",
  "created_at": "2026-04-03T11:00:00Z",
  "storage_paths": {
    "aggregated_scorecard": "cert-artifacts/agent_v2_4_1/exp_001/aggregated_scorecard.json",
    "certification_report": "cert-artifacts/agent_v2_4_1/exp_001/certification_report.json",
    "html_report": "cert-artifacts/agent_v2_4_1/exp_001/certification_report.html",
    "pdf_report": "cert-artifacts/agent_v2_4_1/exp_001/certification_report.pdf",
    "logs": "cert-artifacts/agent_v2_4_1/exp_001/pipeline.log"
  },
  "summary": {
    "total_documents": 30,
    "total_fault_categories": 3,
    "fault_categories": ["critical", "high", "medium"]
  },
  "processing_time_seconds": 23.45,
  "error_message": null
}
```

**Indexes:**
- `{certification_id: 1}` (unique)
- `{agent_id: 1, experiment_id: 1}` (compound)
- `{agent_id: 1, created_at: -1}` (for history)
- `{certification_run_id: 1}` (if present)

---

### Collection: AggregatedCategoryMetadata
Stores aggregated metrics per fault category.

```json
{
  "_id": "ObjectId",
  "fault_category": "critical",
  "certification_id": "uuid",
  "agent_id": "agent_v2_4_1",
  "experiment_id": "exp_001",
  "total_runs": 10,
  "faults_tested": ["disk-fill", "cpu-spike"],
  "numeric_metrics": {
    "trajectory_steps": {
      "mean": 35.3,
      "std_dev": 2.1,
      "min": 33,
      "max": 38,
      "p50": 35.0,
      "p95": 38.5
    },
    "input_tokens": {
      "mean": 4250,
      "std_dev": 150,
      "min": 4100,
      "max": 4400
    }
  },
  "derived_metrics": {
    "fault_detection_success_rate": 0.95,
    "fault_mitigation_success_rate": 0.90,
    "rai_compliance_rate": 1.0,
    "security_compliance_rate": 0.95
  },
  "created_at": "2026-04-03T11:00:00Z"
}
```

**Indexes:**
- `{certification_id: 1, fault_category: 1}` (compound)
- `{agent_id: 1, experiment_id: 1}` (compound)
- `{created_at: -1}` (for sorting)

---

## Error Codes

| Code | HTTP | Cause | Action |
|------|------|-------|--------|
| INVALID_REQUEST | 400 | Missing required fields or invalid format | Validate request body |
| METRICS_NOT_FOUND | 404 | No metrics found for agent_id-experiment_id | Ensure fault-profiling ran first for this agent/experiment |
| AGGREGATION_FAILED | 500 | Statistical aggregation failed | Check metrics data quality |
| CERT_GENERATION_FAILED | 500 | Certification JSON generation failed | Review aggregated metrics |
| RENDERING_ERROR | 500 | HTML/PDF rendering failed | Check template availability |
| STORAGE_ERROR | 500 | Blob storage operation failed | Check storage credentials |
| MONGODB_ERROR | 500 | MongoDB operation failed | Check MongoDB connection |

---

## Storage Structure

```
{container_name}/{agent_id}/{experiment_id}/
├── aggregated_scorecard.json
├── certification_report.json
├── certification_report.html
├── certification_report.pdf
└── pipeline.log
```
