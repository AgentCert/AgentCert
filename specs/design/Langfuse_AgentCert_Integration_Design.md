# Langfuse and AgentCert Integration - Design Document

**Document Version**: 1.0  
**Author**: GitHub Copilot (Architect)  
**Date**: January 20, 2026  
**Branch**: `feature/langfuse-agentcert-integration`

---

## Executive Summary

This design document outlines the integration strategy for **Langfuse** with the **AgentCert** platform (AI Agent Benchmarking for Chaos Engineering). Langfuse serves as the centralized observability and data storage platform for capturing AI agent telemetry, traces, metrics, and evaluation scores during chaos engineering benchmarks.

---

## 1. Overview

### 1.1 What is Langfuse?

Langfuse is an **open-source LLM engineering platform** that provides:

| Feature | Description |
|---------|-------------|
| **Tracing** | Capture every LLM call, agent action, and decision with full context |
| **Metrics & Scoring** | Store and analyze performance metrics (TTD, TTR, success rates) |
| **Sessions** | Group related traces for multi-turn agent interactions |
| **Dashboards** | Built-in visualization for traces, metrics, and analytics |
| **API-First** | REST API and SDKs for Python, JavaScript/TypeScript |
| **Self-Hosting** | Can be self-hosted or use Langfuse Cloud |

### 1.2 Why Langfuse for AgentCert?

| Requirement | Langfuse Capability |
|-------------|---------------------|
| Capture agent actions during chaos scenarios | Tracing with spans for each action |
| Store TTD, TTR, success metrics | Scores API for storing evaluation metrics |
| Historical analysis and comparison | Built-in analytics and query APIs |
| Real-time monitoring | Event streaming and live traces |
| No custom database infrastructure | Langfuse handles all data persistence |
| Graphical dashboards | Pre-built visualization components |

### 1.3 Integration Scope

```
┌─────────────────────────────────────────────────────────────────┐
│                      AgentCert Platform                         │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │   Web UI    │  │  GraphQL    │  │   Agent Registry        │ │
│  │  (React)    │  │   Server    │  │   Service               │ │
│  └──────┬──────┘  └──────┬──────┘  └───────────┬─────────────┘ │
│         │                │                      │               │
│         │                │     ┌────────────────┘               │
│         │                │     │                                │
│         ▼                ▼     ▼                                │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │              Langfuse Integration Layer                   │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐   │  │
│  │  │ Go Client   │  │ TS Client   │  │ NAT Integration │   │  │
│  │  │ (Backend)   │  │ (Frontend)  │  │ (Evaluation)    │   │  │
│  │  └──────┬──────┘  └──────┬──────┘  └────────┬────────┘   │  │
│  └─────────┼────────────────┼──────────────────┼────────────┘  │
│            │                │                  │                │
└────────────┼────────────────┼──────────────────┼────────────────┘
             │                │                  │
             ▼                ▼                  ▼
    ┌─────────────────────────────────────────────────────────┐
    │                    Langfuse Platform                     │
    │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐ │
    │  │  Traces  │  │  Scores  │  │ Sessions │  │Dashboards│ │
    │  └──────────┘  └──────────┘  └──────────┘  └──────────┘ │
    └─────────────────────────────────────────────────────────┘
```

---

## 2. Architecture Design

### 2.1 High-Level Data Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           DATA FLOW PIPELINE                             │
└─────────────────────────────────────────────────────────────────────────┘

1. AGENT REGISTRATION
   ┌────────────────┐     ┌─────────────────┐     ┌─────────────────┐
   │  User/API      │ ──► │ Agent Registry  │ ──► │    Langfuse     │
   │  Register Agent│     │   Service       │     │  Create User    │
   └────────────────┘     └─────────────────┘     └─────────────────┘

2. BENCHMARK EXECUTION
   ┌────────────────┐     ┌─────────────────┐     ┌─────────────────┐
   │  User starts   │ ──► │  NAT Runtime    │ ──► │    Langfuse     │
   │   benchmark    │     │  (Evaluator)    │     │  Create Trace   │
   └────────────────┘     └─────────────────┘     └─────────────────┘
                                │                        │
                                ▼                        │
   ┌────────────────┐     ┌─────────────────┐           │
   │  AI Agent      │ ◄── │  NAT invokes    │           │
   │  Under Test    │     │  agent actions  │           │
   └────────────────┘     └────────┬────────┘           │
                                   │                     │
                                   ▼                     ▼
                          ┌─────────────────┐     ┌─────────────────┐
                          │  NAT captures   │ ──► │    Langfuse     │
                          │  agent actions  │     │  Stream Spans   │
                          └─────────────────┘     └─────────────────┘
                                   │
                                   ▼
                          ┌─────────────────┐     ┌─────────────────┐
                          │  NAT evaluates  │ ──► │    Langfuse     │
                          │  TTD, TTR, etc. │     │  Submit Scores  │
                          └─────────────────┘     └─────────────────┘

3. ANALYTICS & REPORTING
   ┌────────────────┐     ┌─────────────────┐     ┌─────────────────┐
   │  Web UI        │ ──► │  GraphQL Server │ ──► │    Langfuse     │
   │  Dashboard     │     │  Query Metrics  │     │   Query API     │
   └────────────────┘     └─────────────────┘     └─────────────────┘
```

### 2.2 Langfuse Data Model Mapping

| Langfuse Entity | AgentCert Usage | Example |
|-----------------|-----------------|---------|
| **Project** | Benchmark Environment | `agentcert-production`, `agentcert-staging` |
| **User** | Registered AI Agent | Agent metadata (name, version, capabilities) |
| **Session** | Benchmark Project Run | Group of related benchmark executions |
| **Trace** | Single Benchmark Execution | Full trace from fault injection to remediation |
| **Span** | Agent Action | `query_pod_status`, `restart_deployment`, `scale_replicas` |
| **Generation** | LLM Call (if agent uses LLM) | `analyze_logs`, `generate_remediation_plan` |
| **Score** | Performance Metric | TTD: 12.5s, TTR: 45.2s, Success: 100% |
| **Tag** | Scenario Metadata | `pod-crash`, `network-latency`, `agent-v1.2.0` |

---

## 3. Integration Components

### 3.1 Component Overview

| Component | Technology | Location | Purpose |
|-----------|------------|----------|---------|
| **Langfuse Go Client** | Go | `chaoscenter/graphql/server/pkg/langfuse/` | Backend API client for GraphQL server |
| **Langfuse TypeScript Client** | TypeScript | `chaoscenter/web/src/api/langfuse/` | Frontend API client for React app |
| **NAT-Langfuse Bridge** | Python | NAT runtime container | Stream traces from NAT evaluator |
| **Langfuse Configuration** | YAML/Env | Kubernetes ConfigMaps/Secrets | Store API keys and endpoints |

### 3.2 Go Client (Backend)

**Location**: `chaoscenter/graphql/server/pkg/langfuse/`

```
pkg/langfuse/
├── client.go           # Main Langfuse HTTP client
├── models.go           # Request/response data models
├── traces.go           # Trace API operations
├── scores.go           # Score API operations
├── users.go            # User API operations (agent metadata)
├── sessions.go         # Session API operations
├── errors.go           # Custom error types
├── config.go           # Configuration handling
└── langfuse_test.go    # Unit tests
```

**Key Interfaces**:

```go
// LangfuseClient provides methods to interact with Langfuse API
type LangfuseClient interface {
    // User Operations (for agent metadata)
    CreateOrUpdateUser(ctx context.Context, user UserPayload) error
    GetUser(ctx context.Context, userID string) (*User, error)
    
    // Trace Operations
    CreateTrace(ctx context.Context, trace TracePayload) (*Trace, error)
    GetTrace(ctx context.Context, traceID string) (*Trace, error)
    ListTraces(ctx context.Context, filter TraceFilter) (*TraceList, error)
    
    // Score Operations
    CreateScore(ctx context.Context, score ScorePayload) error
    GetScores(ctx context.Context, traceID string) ([]Score, error)
    
    // Session Operations
    CreateSession(ctx context.Context, session SessionPayload) error
    GetSession(ctx context.Context, sessionID string) (*Session, error)
    
    // Metrics & Analytics
    GetMetrics(ctx context.Context, filter MetricsFilter) (*MetricsResponse, error)
    GetAgentComparison(ctx context.Context, agentIDs []string) (*ComparisonResponse, error)
}
```

### 3.3 TypeScript Client (Frontend)

**Location**: `chaoscenter/web/src/api/langfuse/`

```
src/api/langfuse/
├── LangfuseClient.ts      # Main API client class
├── types.ts               # TypeScript interfaces
├── hooks/
│   ├── useTraces.ts       # React hook for traces
│   ├── useScores.ts       # React hook for scores
│   └── useMetrics.ts      # React hook for metrics
└── index.ts               # Exports
```

### 3.4 Configuration

**Environment Variables**:

```bash
# Langfuse API Configuration
LANGFUSE_PUBLIC_KEY=pk-lf-xxxxx      # Public key for frontend
LANGFUSE_SECRET_KEY=sk-lf-xxxxx      # Secret key for backend
LANGFUSE_BASE_URL=https://cloud.langfuse.com  # Or self-hosted URL
LANGFUSE_PROJECT_ID=your-project-id  # Default project ID
LANGFUSE_ENABLED=true                 # Enable/disable integration
```

**Kubernetes Secret**:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: langfuse-credentials
  namespace: litmus
type: Opaque
data:
  LANGFUSE_SECRET_KEY: <base64-encoded-key>
  LANGFUSE_PUBLIC_KEY: <base64-encoded-key>
```

---

## 4. Integration Points

### 4.1 Agent Registry → Langfuse

**When**: On agent registration, update, status change, deletion

**Data Synced**:
- Agent ID (as Langfuse User ID)
- Agent name, version, vendor
- Capabilities list
- Container image
- Status (REGISTERED, ACTIVE, INACTIVE, DELETED)
- Timestamps (created, updated, last health check)

**Implementation**:
```go
// In pkg/agent_registry/service.go
func (s *service) RegisterAgent(ctx context.Context, input RegisterAgentInput) (*Agent, error) {
    // ... validation and creation logic ...
    
    // Sync to Langfuse (async, non-blocking)
    go func() {
        if err := s.langfuseClient.CreateOrUpdateUser(context.Background(), UserPayload{
            ID:   agent.AgentID,
            Name: agent.Name,
            Metadata: map[string]interface{}{
                "version":      agent.Version,
                "vendor":       agent.Vendor,
                "capabilities": agent.Capabilities,
                "status":       agent.Status,
                "namespace":    agent.Namespace,
            },
        }); err != nil {
            s.logger.Warnf("Failed to sync agent to Langfuse: %v", err)
        }
    }()
    
    return agent, nil
}
```

### 4.2 NAT Runtime → Langfuse

**When**: During benchmark execution

**Data Streamed**:
1. **Trace Creation**: When benchmark starts
2. **Spans**: For each agent action (query, remediation, verification)
3. **Generations**: For LLM calls (if agent uses LLM)
4. **Scores**: Evaluation metrics (TTD, TTR, success rate)

**NAT Integration** (Python):
```python
from langfuse import Langfuse

langfuse = Langfuse(
    public_key=os.environ["LANGFUSE_PUBLIC_KEY"],
    secret_key=os.environ["LANGFUSE_SECRET_KEY"],
    host=os.environ["LANGFUSE_BASE_URL"]
)

# Create trace for benchmark run
trace = langfuse.trace(
    name="benchmark-run",
    user_id=agent_id,
    session_id=benchmark_project_id,
    metadata={
        "scenario": "pod-crash",
        "agent_version": "1.2.0",
        "cluster": "production"
    }
)

# Record agent action as span
span = trace.span(
    name="query_pod_status",
    input={"namespace": "default", "pod": "nginx-xxx"},
    output={"status": "CrashLoopBackOff"}
)

# Submit evaluation score
trace.score(
    name="time_to_detect",
    value=12.5,
    comment="Agent detected pod crash in 12.5 seconds"
)
```

### 4.3 GraphQL Server → Langfuse

**When**: Analytics queries from Web UI

**GraphQL Resolvers**:

```graphql
type Query {
    # Get benchmark metrics from Langfuse
    getBenchmarkMetrics(
        projectId: ID!
        agentId: ID
        scenarioId: ID
        timeRange: TimeRange
    ): BenchmarkMetrics!
    
    # Compare multiple agents
    compareAgents(
        projectId: ID!
        agentIds: [ID!]!
        scenarioId: ID
    ): AgentComparison!
    
    # Get agent traces
    getAgentTraces(
        agentId: ID!
        limit: Int
        offset: Int
    ): TraceList!
    
    # Get trace details
    getTraceDetails(traceId: ID!): TraceDetails!
}

type BenchmarkMetrics {
    avgTTD: Float!
    avgTTR: Float!
    successRate: Float!
    totalRuns: Int!
    traces: [TraceSummary!]!
}
```

### 4.4 Web UI → Langfuse

**Dashboard Components**:

| Component | Data Source | Visualization |
|-----------|-------------|---------------|
| Agent Performance Chart | Langfuse Scores API | Line chart (TTD/TTR over time) |
| Success Rate Gauge | Langfuse Scores API | Circular gauge |
| Recent Traces List | Langfuse Traces API | Table with drill-down |
| Agent Comparison | Langfuse Metrics API | Bar chart comparison |
| Trace Timeline | Langfuse Trace Details | Interactive timeline |

---

## 5. Implementation Phases

### Phase 1: Foundation (Week 1-2)

| Task | Description | Effort |
|------|-------------|--------|
| 1.1 | Set up Langfuse Cloud account or self-hosted instance | 2 hours |
| 1.2 | Create Langfuse Go client package structure | 4 hours |
| 1.3 | Implement HTTP client with retry logic | 8 hours |
| 1.4 | Implement User API (create, update, get) | 4 hours |
| 1.5 | Add configuration loading (env vars, secrets) | 4 hours |
| 1.6 | Write unit tests for Go client | 8 hours |
| **Total** | | **30 hours** |

### Phase 2: Agent Registry Integration (Week 2-3)

| Task | Description | Effort |
|------|-------------|--------|
| 2.1 | Integrate Langfuse client with Agent Registry service | 8 hours |
| 2.2 | Implement async sync on agent registration | 4 hours |
| 2.3 | Implement sync on agent update/delete | 4 hours |
| 2.4 | Add sync status tracking in MongoDB | 4 hours |
| 2.5 | Handle sync failures gracefully | 4 hours |
| 2.6 | Write integration tests | 8 hours |
| **Total** | | **32 hours** |

### Phase 3: Trace & Score APIs (Week 3-4)

| Task | Description | Effort |
|------|-------------|--------|
| 3.1 | Implement Trace API operations in Go client | 8 hours |
| 3.2 | Implement Score API operations in Go client | 4 hours |
| 3.3 | Implement Session API operations in Go client | 4 hours |
| 3.4 | Create GraphQL resolvers for metrics queries | 8 hours |
| 3.5 | Create GraphQL resolvers for trace queries | 8 hours |
| 3.6 | Write unit and integration tests | 8 hours |
| **Total** | | **40 hours** |

### Phase 4: NAT Integration (Week 4-5)

| Task | Description | Effort |
|------|-------------|--------|
| 4.1 | Configure NAT runtime with Langfuse SDK | 4 hours |
| 4.2 | Implement trace creation on benchmark start | 4 hours |
| 4.3 | Implement span streaming for agent actions | 8 hours |
| 4.4 | Implement score submission for evaluations | 4 hours |
| 4.5 | Add custom chaos evaluators (TTD, TTR) | 16 hours |
| 4.6 | Test end-to-end data flow | 8 hours |
| **Total** | | **44 hours** |

### Phase 5: Frontend Integration (Week 5-6)

| Task | Description | Effort |
|------|-------------|--------|
| 5.1 | Create TypeScript Langfuse API client | 8 hours |
| 5.2 | Create React hooks for data fetching | 8 hours |
| 5.3 | Build Agent Performance Dashboard | 16 hours |
| 5.4 | Build Trace Explorer component | 16 hours |
| 5.5 | Build Agent Comparison view | 8 hours |
| 5.6 | Add real-time updates via polling/subscriptions | 8 hours |
| **Total** | | **64 hours** |

### Phase 6: Testing & Documentation (Week 6-7)

| Task | Description | Effort |
|------|-------------|--------|
| 6.1 | End-to-end integration testing | 16 hours |
| 6.2 | Performance testing (trace volume, latency) | 8 hours |
| 6.3 | Write API documentation | 8 hours |
| 6.4 | Write user guide for Langfuse dashboards | 4 hours |
| 6.5 | Create runbooks for operations | 4 hours |
| **Total** | | **40 hours** |

---

## 6. Effort Summary

| Phase | Description | Hours | Weeks |
|-------|-------------|-------|-------|
| Phase 1 | Foundation (Go Client) | 30 | 1-2 |
| Phase 2 | Agent Registry Integration | 32 | 2-3 |
| Phase 3 | Trace & Score APIs | 40 | 3-4 |
| Phase 4 | NAT Integration | 44 | 4-5 |
| Phase 5 | Frontend Integration | 64 | 5-6 |
| Phase 6 | Testing & Documentation | 40 | 6-7 |
| **Total** | | **250 hours** | **~7 weeks** |

**Team Size**: 2 developers  
**Calendar Time**: ~4 weeks (with parallel work)

---

## 7. Technical Specifications

### 7.1 Langfuse API Endpoints Used

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/public/users` | POST | Create/update agent as user |
| `/api/public/traces` | POST | Create new trace |
| `/api/public/traces` | GET | List/query traces |
| `/api/public/traces/{id}` | GET | Get trace details |
| `/api/public/spans` | POST | Create span (agent action) |
| `/api/public/generations` | POST | Create generation (LLM call) |
| `/api/public/scores` | POST | Submit score (metric) |
| `/api/public/scores` | GET | Query scores |
| `/api/public/sessions` | POST | Create session |
| `/api/public/metrics` | GET | Get aggregated metrics |

### 7.2 Authentication

```go
// HTTP request with Langfuse authentication
req.Header.Set("Authorization", "Bearer "+secretKey)
req.Header.Set("Content-Type", "application/json")
```

### 7.3 Error Handling

| Error Scenario | Handling Strategy |
|----------------|-------------------|
| Network timeout | Retry with exponential backoff (3 attempts) |
| Rate limiting (429) | Respect Retry-After header |
| Authentication error (401) | Log error, mark sync as failed |
| Server error (5xx) | Retry with backoff |
| Validation error (400) | Log error with details, don't retry |

### 7.4 Performance Considerations

| Consideration | Strategy |
|---------------|----------|
| High trace volume | Batch spans before sending (every 1s or 100 spans) |
| Real-time updates | Poll Langfuse API every 5s for active benchmarks |
| Dashboard performance | Cache metrics with 30s TTL |
| Frontend data transfer | Paginate traces (limit 50 per page) |

---

## 8. Security Considerations

| Aspect | Mitigation |
|--------|------------|
| API key storage | Store in Kubernetes Secrets, never in code |
| Key rotation | Support multiple keys with graceful rotation |
| Data privacy | Mask sensitive data in traces (credentials, PII) |
| Network security | Use HTTPS only, validate TLS certificates |
| Access control | Langfuse project-level access via API keys |

---

## 9. Monitoring & Observability

### 9.1 Metrics to Track

| Metric | Description |
|--------|-------------|
| `langfuse_sync_success_total` | Counter of successful syncs |
| `langfuse_sync_failure_total` | Counter of failed syncs |
| `langfuse_api_latency_seconds` | Histogram of API latency |
| `langfuse_traces_created_total` | Counter of traces created |
| `langfuse_scores_submitted_total` | Counter of scores submitted |

### 9.2 Alerts

| Alert | Condition | Action |
|-------|-----------|--------|
| Langfuse Sync Failures | >5 failures in 5 minutes | Check API key, network |
| High API Latency | p95 > 2 seconds | Check Langfuse status |
| Trace Creation Errors | Error rate > 1% | Investigate payload issues |

---

## 10. Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| Langfuse Cloud/Self-hosted | Latest | Observability platform |
| Go HTTP client | Standard library | API communication |
| Langfuse Python SDK | ^2.0.0 | NAT integration |
| langfuse-js | ^3.0.0 | Frontend TypeScript client |

---

## 11. Risks & Mitigations

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Langfuse API changes | Medium | Low | Pin SDK versions, monitor changelog |
| Rate limiting | Medium | Medium | Implement batching, respect limits |
| Langfuse downtime | High | Low | Graceful degradation, queue traces locally |
| High trace volume costs | Medium | Medium | Implement sampling for non-critical traces |
| Performance impact | Medium | Low | Async operations, background sync |

---

## 12. Success Criteria

| Criteria | Target |
|----------|--------|
| Agent metadata synced to Langfuse | 100% of registrations |
| Trace capture rate | >99% of benchmark runs |
| API call success rate | >99.5% |
| End-to-end latency (trace to dashboard) | <10 seconds |
| Dashboard load time | <3 seconds |

---

## 13. Next Steps

1. **Review this design document** with stakeholders
2. **Set up Langfuse environment** (Cloud or self-hosted)
3. **Begin Phase 1** implementation (Go client foundation)
4. **Create tracking issues** for each task in the implementation plan
5. **Schedule weekly reviews** to track progress

---

## Appendix A: File Structure

```
chaoscenter/
├── graphql/server/
│   └── pkg/
│       ├── langfuse/                    # NEW: Langfuse Go client
│       │   ├── client.go
│       │   ├── models.go
│       │   ├── traces.go
│       │   ├── scores.go
│       │   ├── users.go
│       │   ├── sessions.go
│       │   ├── errors.go
│       │   ├── config.go
│       │   └── langfuse_test.go
│       └── agent_registry/
│           └── langfuse_sync.go         # NEW: Langfuse sync logic
├── web/src/
│   └── api/
│       └── langfuse/                    # NEW: TypeScript client
│           ├── LangfuseClient.ts
│           ├── types.ts
│           └── hooks/
│               ├── useTraces.ts
│               ├── useScores.ts
│               └── useMetrics.ts
└── nat-runtime/                         # NAT evaluator (Python)
    └── langfuse_integration.py          # NEW: NAT-Langfuse bridge
```

---

## Appendix B: Sample API Responses

### Trace Response
```json
{
  "id": "trace-123",
  "name": "benchmark-pod-crash",
  "userId": "agent-456",
  "sessionId": "project-789",
  "metadata": {
    "scenario": "pod-crash",
    "cluster": "production"
  },
  "observations": [
    {
      "id": "span-1",
      "type": "SPAN",
      "name": "query_pod_status",
      "startTime": "2026-01-20T10:00:00Z",
      "endTime": "2026-01-20T10:00:01Z"
    }
  ],
  "scores": [
    {
      "name": "time_to_detect",
      "value": 12.5
    },
    {
      "name": "success_rate",
      "value": 100
    }
  ]
}
```

---

**Document End**
