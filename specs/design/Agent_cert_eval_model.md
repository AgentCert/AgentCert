# Agent Certification Evaluation Framework - Data Model

## Table of Contents
1. [System Flow Analysis](#system-flow-analysis)
2. [Evaluation Data Model Overview](#evaluation-data-model-overview)
3. [MongoDB Schema Design](#mongodb-schema-design)
4. [Evaluation Criteria Categories](#evaluation-criteria-categories)
5. [Certification Levels](#certification-levels)

---

## System Flow Analysis

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            AGENT ECOSYSTEM                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌──────────┐    ┌──────────┐    ┌─────────────┐    ┌─────────────────┐    │
│  │  Agents  │───▶│   Apps   │───▶│ MCP Servers │───▶│   Environment   │    │
│  └────┬─────┘    └────┬─────┘    └──────┬──────┘    │  (Kubernetes)   │    │
│       │               │                  │           └────────┬────────┘    │
│       │               │                  │                    │             │
│       ▼               ▼                  ▼                    ▼             │
│  ┌────────────────────────────────────────────────────────────────────┐    │
│  │                         ITOps Agent                                  │    │
│  │  (Collects: Agent Logs, Fault Logs, App Logs, K8s Metrics)          │    │
│  └────────────────────────────────┬─────────────────────────────────────┘    │
│                                   │                                          │
│                                   ▼                                          │
│                          ┌─────────────────┐                                │
│                          │    Langfuse     │                                │
│                          │  (Traces Gen)   │                                │
│                          └────────┬────────┘                                │
│                                   │                                          │
│                                   ▼                                          │
│  ┌────────────────────────────────────────────────────────────────────┐    │
│  │              AGENT EVALUATION FRAMEWORK (NEW)                       │    │
│  │  ┌─────────────┐  ┌──────────────┐  ┌────────────────────┐         │    │
│  │  │ Log Analysis│  │Trace Analysis│  │ Telemetry Analysis │         │    │
│  │  └──────┬──────┘  └──────┬───────┘  └─────────┬──────────┘         │    │
│  │         │                │                    │                     │    │
│  │         └────────────────┼────────────────────┘                     │    │
│  │                          ▼                                          │    │
│  │              ┌───────────────────────┐                              │    │
│  │              │   Evaluation Engine   │                              │    │
│  │              │  (Metrics & Scoring)  │                              │    │
│  │              └───────────┬───────────┘                              │    │
│  │                          │                                          │    │
│  │                          ▼                                          │    │
│  │              ┌───────────────────────┐                              │    │
│  │              │   Agent Certificate   │                              │    │
│  │              │      Generation       │                              │    │
│  │              └───────────────────────┘                              │    │
│  └────────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Evaluation Data Model Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        EVALUATION DATA MODEL                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────┐         ┌──────────────────────┐                       │
│  │     agents      │◄────────│  evaluation_runs     │                       │
│  │  (from registry)│         │                      │                       │
│  └─────────────────┘         │  - run_id            │                       │
│         │                    │  - agent_id          │                       │
│         │                    │  - status            │                       │
│         │                    │  - started_at        │                       │
│         │                    │  - completed_at      │                       │
│         │                    │  - config            │                       │
│         │                    │  - summary_metrics   │                       │
│         │                    └──────────┬───────────┘                       │
│         │                               │                                    │
│         │    ┌──────────────────────────┼──────────────────────────┐        │
│         │    │                          │                          │        │
│         ▼    ▼                          ▼                          ▼        │
│  ┌────────────────┐          ┌────────────────────┐     ┌─────────────────┐ │
│  │ trace_analyses │          │  log_analyses      │     │telemetry_metrics│ │
│  │                │          │                    │     │                 │ │
│  │ - trace_id     │          │ - source_type      │     │ - metric_type   │ │
│  │ - langfuse_ref │          │ - log_level_dist   │     │ - value         │ │
│  │ - span_count   │          │ - error_patterns   │     │ - timestamp     │ │
│  │ - total_tokens │          │ - anomalies        │     │ - dimensions    │ │
│  │ - latency_ms   │          │ - time_range       │     │                 │ │
│  │ - success_rate │          └────────────────────┘     └─────────────────┘ │
│  │ - reasoning_q  │                                                         │
│  └────────────────┘                                                         │
│         │                                                                    │
│         └─────────────────────┬──────────────────────────────────────┐      │
│                               ▼                                      ▼      │
│                    ┌─────────────────────────┐          ┌──────────────────┐│
│                    │  evaluation_criteria    │          │ evaluation_scores││
│                    │                         │          │                  ││
│                    │  - criteria_id          │◄─────────│ - criteria_id    ││
│                    │  - name                 │          │ - score (0-100)  ││
│                    │  - category             │          │ - evidence       ││
│                    │  - weight               │          │ - recommendations││
│                    │  - thresholds           │          │ - raw_data       ││
│                    │  - formula              │          └──────────────────┘│
│                    └─────────────────────────┘                    │         │
│                                                                   │         │
│                                                                   ▼         │
│                                                    ┌───────────────────────┐│
│                                                    │ agent_certificates    ││
│                                                    │                       ││
│                                                    │ - certificate_id      ││
│                                                    │ - agent_id            ││
│                                                    │ - evaluation_run_id   ││
│                                                    │ - certification_level ││
│                                                    │ - overall_score       ││
│                                                    │ - issued_at           ││
│                                                    │ - expires_at          ││
│                                                    │ - status              ││
│                                                    └───────────────────────┘│
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## MongoDB Schema Design

### 1. `evaluation_runs` Collection

```javascript
{
  "_id": ObjectId,
  "run_id": "uuid-string",
  "project_id": "project-uuid",
  
  // Agent Reference
  "agent_id": "agent-uuid",  // Links to Agent Registry
  "agent_version": "1.2.0",
  "agent_type": "conversational | autonomous | workflow",
  
  // Evaluation Configuration
  "config": {
    "evaluation_type": "full | quick | targeted",
    "criteria_ids": ["criteria-1", "criteria-2"],
    "time_window": {
      "start": "ISO-timestamp",
      "end": "ISO-timestamp"
    },
    "data_sources": {
      "langfuse_enabled": true,
      "langfuse_project_id": "project-id",
      "log_sources": ["app", "kubernetes", "fault"],
      "telemetry_enabled": true
    },
    "thresholds": {
      "min_trace_count": 100,
      "min_log_entries": 1000
    }
  },
  
  // Status
  "status": "pending | running | completed | failed | cancelled",
  "progress": {
    "current_phase": "trace_collection | log_analysis | scoring | certificate_gen",
    "percentage": 75,
    "message": "Analyzing trace patterns..."
  },
  
  // Timestamps
  "started_at": "ISO-timestamp",
  "completed_at": "ISO-timestamp",
  
  // Summary Metrics (computed after completion)
  "summary_metrics": {
    "overall_score": 85.5,
    "certification_level": "gold | silver | bronze | failed",
    "total_traces_analyzed": 1500,
    "total_logs_analyzed": 25000,
    "total_spans_analyzed": 45000,
    "evaluation_duration_ms": 120000
  },
  
  // Audit
  "created_by": {
    "user_id": "user-uuid",
    "username": "admin"
  },
  "is_removed": false,
  "created_at": "ISO-timestamp",
  "updated_at": "ISO-timestamp"
}
```

### 2. `trace_analyses` Collection

```javascript
{
  "_id": ObjectId,
  "analysis_id": "uuid-string",
  "evaluation_run_id": "run-uuid",
  "project_id": "project-uuid",
  
  // Langfuse Reference
  "langfuse_data": {
    "project_id": "langfuse-project",
    "trace_ids": ["trace-1", "trace-2", ...],
    "session_ids": ["session-1", "session-2"],
    "time_range": {
      "start": "ISO-timestamp",
      "end": "ISO-timestamp"
    }
  },
  
  // Aggregated Trace Metrics
  "trace_metrics": {
    "total_traces": 1500,
    "successful_traces": 1425,
    "failed_traces": 75,
    "success_rate": 95.0,
    
    // Latency Distribution
    "latency": {
      "p50_ms": 250,
      "p90_ms": 800,
      "p95_ms": 1200,
      "p99_ms": 2500,
      "avg_ms": 320,
      "max_ms": 5000
    }
  },
  
  // Span Analysis
  "span_metrics": {
    "total_spans": 45000,
    "avg_spans_per_trace": 30,
    "span_type_distribution": {
      "llm": 15000,
      "tool_call": 12000,
      "retrieval": 8000,
      "generation": 10000
    },
    "error_spans": 450,
    "error_rate": 1.0
  },
  
  // Token Analysis
  "token_metrics": {
    "total_input_tokens": 5000000,
    "total_output_tokens": 2500000,
    "total_tokens": 7500000,
    "avg_tokens_per_trace": 5000,
    "cost_estimate_usd": 125.50,
    
    // Model Distribution
    "model_usage": {
      "gpt-4": { "traces": 800, "tokens": 4000000 },
      "gpt-3.5-turbo": { "traces": 700, "tokens": 3500000 }
    }
  },
  
  // Reasoning Quality Analysis
  "reasoning_analysis": {
    "coherence_score": 88.5,
    "completeness_score": 82.0,
    "relevance_score": 90.0,
    "hallucination_detected": 15,
    "self_correction_count": 45
  },
  
  // Tool Usage Patterns
  "tool_usage": {
    "total_tool_calls": 8500,
    "unique_tools": ["web_search", "calculator", "code_exec", "file_read"],
    "tool_success_rate": 96.5,
    "tool_call_distribution": {
      "web_search": 3000,
      "calculator": 2000,
      "code_exec": 2500,
      "file_read": 1000
    },
    "tool_error_patterns": [
      { "tool": "code_exec", "error_type": "timeout", "count": 120 },
      { "tool": "web_search", "error_type": "rate_limit", "count": 45 }
    ]
  },
  
  // Fault Goal and Remediation
  "fault_goal_remediation": {
    "faults_injected": [
      {
        "fault_id": "fault-uuid",
        "fault_type": "llm-api-latency | llm-api-error | mcp-connection-drop | token-limit-exhaust | tool-call-failure",
        "fault_name": "LLM API Latency Injection",
        "goal": "Validate agent graceful degradation under LLM API latency conditions",
        "injection_time": "ISO-timestamp",
        "duration_ms": 60000,
        "parameters": {
          "latency_ms": 3000,
          "target_hosts": ["api.openai.com"]
        }
      }
    ],
    "remediation_actions": [
      {
        "fault_id": "fault-uuid",
        "action_type": "retry | fallback | circuit_breaker | graceful_degradation | escalation",
        "action_taken": "Switched to fallback model gpt-3.5-turbo",
        "action_timestamp": "ISO-timestamp",
        "time_to_remediate_ms": 1500,
        "success": true,
        "user_impact": "minimal | moderate | significant | critical",
        "data_loss": false
      }
    ],
    "remediation_metrics": {
      "total_faults_injected": 15,
      "faults_successfully_remediated": 14,
      "remediation_success_rate": 93.3,
      "avg_time_to_remediate_ms": 2500,
      "max_time_to_remediate_ms": 8000,
      "faults_requiring_manual_intervention": 1
    }
  },
  
  // Ideal Course of Action Analysis
  "ideal_course_of_action": {
    "scenarios_evaluated": 50,
    "alignment_score": 87.5,  // How well agent followed ideal path
    "analysis": [
      {
        "scenario_id": "scenario-uuid",
        "scenario_type": "error_handling | fault_recovery | user_request | multi_step_task",
        "description": "Agent response when LLM API returns 429 rate limit error",
        "ideal_actions": [
          {
            "step": 1,
            "action": "Detect rate limit error from API response",
            "expected_behavior": "Parse error code and identify as transient failure"
          },
          {
            "step": 2,
            "action": "Implement exponential backoff",
            "expected_behavior": "Wait 1s, then 2s, then 4s before retry"
          },
          {
            "step": 3,
            "action": "Retry with same request or fallback to alternative model",
            "expected_behavior": "Attempt retry up to 3 times, then switch to fallback"
          },
          {
            "step": 4,
            "action": "Inform user if degraded service",
            "expected_behavior": "Transparent communication about potential delays"
          }
        ],
        "actual_actions": [
          {
            "step": 1,
            "action": "Detected rate limit error",
            "matched_ideal": true,
            "timestamp": "ISO-timestamp"
          },
          {
            "step": 2,
            "action": "Implemented fixed 2s delay instead of exponential backoff",
            "matched_ideal": false,
            "deviation": "Used fixed delay instead of exponential backoff",
            "timestamp": "ISO-timestamp"
          },
          {
            "step": 3,
            "action": "Retried and succeeded on second attempt",
            "matched_ideal": true,
            "timestamp": "ISO-timestamp"
          }
        ],
        "alignment_score": 75.0,
        "deviations": [
          {
            "step": 2,
            "expected": "Exponential backoff (1s, 2s, 4s)",
            "actual": "Fixed 2s delay",
            "severity": "minor | moderate | critical",
            "impact": "Potential for continued rate limiting under heavy load"
          }
        ],
        "recommendations": [
          "Implement exponential backoff with jitter for rate limit handling",
          "Add circuit breaker pattern for repeated failures"
        ]
      }
    ],
    "summary": {
      "fully_aligned_scenarios": 42,
      "partially_aligned_scenarios": 6,
      "misaligned_scenarios": 2,
      "common_deviations": [
        {
          "pattern": "Fixed delay instead of exponential backoff",
          "occurrence_count": 8,
          "recommended_fix": "Implement configurable backoff strategy"
        },
        {
          "pattern": "Missing user notification on degraded service",
          "occurrence_count": 3,
          "recommended_fix": "Add proactive user communication for service degradation"
        }
      ],
      "best_practices_followed": [
        "Graceful error handling",
        "Fallback model usage",
        "Request retry on transient failures"
      ],
      "best_practices_missing": [
        "Exponential backoff with jitter",
        "Circuit breaker pattern",
        "Proactive user notification"
      ]
    }
  },
  
  // Anomalies Detected
  "anomalies": [
    {
      "type": "latency_spike",
      "timestamp": "ISO-timestamp",
      "trace_id": "trace-uuid",
      "severity": "warning | critical",
      "description": "Latency exceeded 5s threshold"
    }
  ],
  
  "created_at": "ISO-timestamp"
}
```

### 3. `log_analyses` Collection

```javascript
{
  "_id": ObjectId,
  "analysis_id": "uuid-string",
  "evaluation_run_id": "run-uuid",
  "project_id": "project-uuid",
  
  // Source Configuration
  "sources": {
    "agent_logs": true,
    "app_logs": true,
    "kubernetes_logs": true,
    "fault_logs": true  // From Fault Studio injections
  },
  
  // Time Range
  "time_range": {
    "start": "ISO-timestamp",
    "end": "ISO-timestamp"
  },
  
  // Volume Metrics
  "volume_metrics": {
    "total_entries": 25000,
    "by_source": {
      "agent": 8000,
      "app": 10000,
      "kubernetes": 5000,
      "fault": 2000
    }
  },
  
  // Log Level Distribution
  "level_distribution": {
    "debug": 5000,
    "info": 15000,
    "warn": 3500,
    "error": 1200,
    "fatal": 300
  },
  
  // Error Analysis
  "error_analysis": {
    "total_errors": 1500,
    "error_rate": 6.0,
    "error_patterns": [
      {
        "pattern_id": "ERR001",
        "message_template": "Connection timeout to {service}",
        "count": 450,
        "first_seen": "ISO-timestamp",
        "last_seen": "ISO-timestamp",
        "affected_components": ["mcp-server", "db-connector"]
      },
      {
        "pattern_id": "ERR002",
        "message_template": "LLM API rate limit exceeded",
        "count": 120,
        "first_seen": "ISO-timestamp",
        "last_seen": "ISO-timestamp",
        "affected_components": ["llm-gateway"]
      }
    ],
    "error_timeline": [
      { "timestamp": "ISO-timestamp", "error_count": 45 },
      { "timestamp": "ISO-timestamp", "error_count": 23 }
    ]
  },
  
  // Fault Injection Correlation
  "fault_correlation": {
    "faults_injected": 15,
    "faults_detected_in_logs": 14,
    "detection_rate": 93.3,
    "recovery_patterns": [
      {
        "fault_type": "network_latency",
        "avg_recovery_time_ms": 5000,
        "graceful_recovery_rate": 85.0
      }
    ]
  },
  
  // Anomaly Detection
  "anomalies": [
    {
      "type": "error_burst",
      "timestamp": "ISO-timestamp",
      "duration_ms": 30000,
      "error_count": 150,
      "baseline_error_count": 10,
      "severity": "critical"
    }
  ],
  
  // Kubernetes Specific (if applicable)
  "kubernetes_metrics": {
    "pod_restarts": 3,
    "oom_kills": 1,
    "container_errors": 5,
    "resource_pressure_events": 2
  },
  
  "created_at": "ISO-timestamp"
}
```

### 4. `telemetry_metrics` Collection

```javascript
{
  "_id": ObjectId,
  "metrics_id": "uuid-string",
  "evaluation_run_id": "run-uuid",
  "project_id": "project-uuid",
  
  // Time Range
  "time_range": {
    "start": "ISO-timestamp",
    "end": "ISO-timestamp",
    "granularity": "1m | 5m | 1h"
  },
  
  // Performance Metrics
  "performance": {
    "response_time": {
      "p50_ms": 200,
      "p90_ms": 500,
      "p95_ms": 800,
      "p99_ms": 1500,
      "avg_ms": 280
    },
    "throughput": {
      "requests_per_second": 50,
      "peak_rps": 120,
      "total_requests": 180000
    }
  },
  
  // Resource Utilization
  "resource_utilization": {
    "cpu": {
      "avg_percent": 45.0,
      "max_percent": 85.0,
      "throttle_events": 5
    },
    "memory": {
      "avg_mb": 512,
      "max_mb": 768,
      "oom_events": 0
    },
    "network": {
      "bytes_in": 1024000000,
      "bytes_out": 512000000,
      "error_rate": 0.01
    }
  },
  
  // Availability
  "availability": {
    "uptime_percent": 99.95,
    "downtime_incidents": 1,
    "total_downtime_seconds": 180,
    "mtbf_hours": 720,  // Mean Time Between Failures
    "mttr_seconds": 120  // Mean Time To Recovery
  },
  
  // Cost Metrics
  "cost_metrics": {
    "compute_cost_usd": 45.50,
    "llm_api_cost_usd": 125.00,
    "storage_cost_usd": 5.00,
    "total_cost_usd": 175.50,
    "cost_per_request_usd": 0.001
  },
  
  // Rate Limiting / Throttling
  "rate_limiting": {
    "throttled_requests": 250,
    "throttle_rate": 0.14,
    "backoff_events": 45
  },
  
  // Time Series Data (sampled)
  "time_series": {
    "response_time_series": [
      { "timestamp": "ISO", "p50": 200, "p99": 1200 }
      // ... sampled data points
    ],
    "throughput_series": [
      { "timestamp": "ISO", "rps": 45 }
      // ...
    ]
  },
  
  "created_at": "ISO-timestamp"
}
```

### 5. `evaluation_criteria` Collection

```javascript
{
  "_id": ObjectId,
  "criteria_id": "uuid-string",
  "project_id": "project-uuid",  // null for global criteria
  
  "name": "Response Reliability",
  "description": "Measures the consistency and reliability of agent responses",
  "category": "reliability | performance | safety | accuracy | cost_efficiency | resilience",
  
  // Weight in overall score
  "weight": 0.20,  // 20% of total score
  
  // Scoring Configuration
  "scoring": {
    "type": "threshold | formula | ml_model",
    
    // For threshold-based scoring
    "thresholds": {
      "excellent": { "min": 95, "score": 100 },
      "good": { "min": 85, "score": 80 },
      "acceptable": { "min": 70, "score": 60 },
      "poor": { "min": 50, "score": 40 },
      "failed": { "min": 0, "score": 0 }
    },
    
    // For formula-based scoring
    "formula": {
      "expression": "(success_rate * 0.6) + (100 - error_rate) * 0.4",
      "variables": ["success_rate", "error_rate"],
      "data_sources": ["trace_analyses", "log_analyses"]
    }
  },
  
  // Data Requirements
  "data_requirements": {
    "min_data_points": 100,
    "required_sources": ["traces", "logs"],
    "optional_sources": ["telemetry"]
  },
  
  // Certification Level Thresholds
  "certification_thresholds": {
    "gold": 90,
    "silver": 75,
    "bronze": 60
  },
  
  // Active / Built-in
  "is_builtin": true,
  "is_active": true,
  
  "created_at": "ISO-timestamp",
  "updated_at": "ISO-timestamp"
}
```

### 6. `evaluation_scores` Collection

```javascript
{
  "_id": ObjectId,
  "score_id": "uuid-string",
  "evaluation_run_id": "run-uuid",
  "criteria_id": "criteria-uuid",
  "project_id": "project-uuid",
  
  // Score
  "score": 87.5,
  "max_score": 100,
  "normalized_score": 0.875,
  
  // Certification Level for this criteria
  "level": "gold | silver | bronze | failed",
  
  // Evidence
  "evidence": {
    "data_points_analyzed": 1500,
    "key_metrics": {
      "success_rate": 95.0,
      "error_rate": 5.0,
      "avg_latency_ms": 320
    },
    "supporting_data": [
      {
        "source": "trace_analyses",
        "metric": "trace_metrics.success_rate",
        "value": 95.0,
        "weight": 0.6
      },
      {
        "source": "log_analyses", 
        "metric": "error_analysis.error_rate",
        "value": 6.0,
        "weight": 0.4
      }
    ]
  },
  
  // Trends
  "trend": {
    "direction": "improving | stable | declining",
    "previous_score": 82.0,
    "change_percent": 6.7
  },
  
  // Recommendations
  "recommendations": [
    {
      "priority": "high | medium | low",
      "category": "performance",
      "title": "Reduce LLM API Latency",
      "description": "P99 latency is above threshold. Consider caching or model optimization.",
      "potential_impact": "+5 points"
    }
  ],
  
  // Issues Found
  "issues": [
    {
      "severity": "critical | warning | info",
      "code": "PERF_001",
      "title": "High Tail Latency",
      "description": "P99 latency (2500ms) exceeds acceptable threshold (2000ms)",
      "affected_traces": 75,
      "sample_trace_ids": ["trace-1", "trace-2"]
    }
  ],
  
  "created_at": "ISO-timestamp"
}
```

### 7. `agent_certificates` Collection

```javascript
{
  "_id": ObjectId,
  "certificate_id": "uuid-string",
  "project_id": "project-uuid",
  
  // Agent Reference
  "agent_id": "agent-uuid",
  "agent_name": "Customer Support Agent v2",
  "agent_version": "2.1.0",
  
  // Evaluation Reference
  "evaluation_run_id": "run-uuid",
  
  // Certification Details
  "certification": {
    "level": "gold | silver | bronze | failed",
    "overall_score": 87.5,
    "issued_at": "ISO-timestamp",
    "valid_from": "ISO-timestamp",
    "expires_at": "ISO-timestamp",  // e.g., 90 days validity
    "status": "active | expired | revoked | superseded"
  },
  
  // Score Breakdown
  "score_breakdown": {
    "reliability": { "score": 92, "weight": 0.25, "weighted_score": 23.0 },
    "performance": { "score": 85, "weight": 0.20, "weighted_score": 17.0 },
    "safety": { "score": 95, "weight": 0.25, "weighted_score": 23.75 },
    "accuracy": { "score": 80, "weight": 0.20, "weighted_score": 16.0 },
    "cost_efficiency": { "score": 75, "weight": 0.10, "weighted_score": 7.5 }
  },
  
  // Key Findings Summary
  "summary": {
    "strengths": [
      "Excellent error handling with 95% graceful recovery rate",
      "High response consistency across different input types"
    ],
    "areas_for_improvement": [
      "P99 latency could be reduced with caching",
      "Token usage is higher than benchmark"
    ],
    "critical_issues": []
  },
  
  // Comparison with Previous
  "comparison": {
    "previous_certificate_id": "prev-cert-uuid",
    "previous_level": "silver",
    "previous_score": 78.0,
    "improvement_percent": 12.2
  },
  
  // Metadata for Certificate Display
  "metadata": {
    "evaluation_period": {
      "start": "ISO-timestamp",
      "end": "ISO-timestamp"
    },
    "data_volume": {
      "traces_analyzed": 15000,
      "logs_analyzed": 250000,
      "time_span_hours": 168  // 7 days
    },
    "evaluator_version": "1.0.0"
  },
  
  // Revocation Info (if applicable)
  "revocation": {
    "revoked_at": null,
    "reason": null,
    "revoked_by": null
  },
  
  // Audit
  "created_by": {
    "user_id": "user-uuid",
    "username": "system"
  },
  "created_at": "ISO-timestamp",
  "updated_at": "ISO-timestamp"
}
```

---

## Evaluation Criteria Categories

| Category | Weight | What it Measures | Data Sources |
|----------|--------|-----------------|--------------|
| **Reliability** | 25% | Consistent behavior, uptime, graceful degradation | Traces, Logs, Telemetry |
| **Performance** | 20% | Latency, throughput, resource efficiency | Traces, Telemetry |
| **Safety** | 25% | Error handling, fault tolerance, no harmful outputs | Logs, Traces, Fault injection |
| **Accuracy** | 20% | Correctness, hallucination rate, reasoning quality | Traces (Langfuse) |
| **Cost Efficiency** | 10% | Token usage, API costs, resource utilization | Traces, Telemetry |

---

## Certification Levels

| Level | Score Range | Meaning |
|-------|-------------|---------|
| 🥇 **Gold** | 90-100 | Production-ready, exceeds standards |
| 🥈 **Silver** | 75-89 | Production-ready, meets standards |
| 🥉 **Bronze** | 60-74 | Acceptable, minor improvements needed |
| ❌ **Failed** | 0-59 | Not production-ready |

---

## Document History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-02-10 | 1.0 | GitHub Copilot | Initial data model design for Agent Evaluation Framework |
