# Analysis: Direct Experiment-to-Langfuse Tracing (Without Flash-Agent)

**Branch**: `feature/analysis-tasks`  
**Date**: 2026-03-26  
**Status**: Analysis Only — No Code Changes

---

## 1. Objective

When a user triggers a new experiment from the ChaosCenter UI/API, log a **rich trace** to Langfuse containing:
- Experiment ID, name, timestamps
- All fault names with their configuration (targets, duration, probes)
- Fault verdicts, probe success percentages
- Resiliency scores

**Constraint**: This must happen entirely within the **ChaosCenter GraphQL server** — Flash-Agent must NOT be in the loop.

---

## 2. Current State: What Already Exists

### 2.1 Existing Langfuse Integration

The ChaosCenter GraphQL server **already has** a Langfuse integration:

| Component | File | Purpose |
|-----------|------|---------|
| **Tracer singleton** | [pkg/observability/langfuse_tracer.go](../chaoscenter/graphql/server/pkg/observability/langfuse_tracer.go) | Manages global `LangfuseTracer` with async trace channel |
| **HTTP client** | [pkg/agent_registry/langfuse_client.go](../chaoscenter/graphql/server/pkg/agent_registry/langfuse_client.go) | REST calls to Langfuse `/api/public/traces`, `/api/public/ingestion` |
| **Env vars** | `LANGFUSE_HOST`, `LANGFUSE_PUBLIC_KEY`, `LANGFUSE_SECRET_KEY`, `LANGFUSE_ORG_ID`, `LANGFUSE_PROJECT_ID` | Configured via `.env.langfuse` or K8s secrets |

### 2.2 Where Traces Are Currently Created

| Hook Point | Function | File:Line | What's Traced |
|------------|----------|-----------|---------------|
| Experiment run start | `traceExperimentExecution()` | [handler.go:1474](../chaoscenter/graphql/server/pkg/chaos_experiment_run/handler/handler.go#L1474) | Single trace with experimentID, experimentName, infraID, namespace. FaultName hardcoded to `"chaos-workflow"` |
| Workflow event received | `traceExperimentObservation()` | [handler.go:1512](../chaoscenter/graphql/server/pkg/chaos_experiment_run/handler/handler.go#L1512) | Observation with execution data snapshot + metrics (phase, eventType) |
| Experiment completed | `completeExperimentExecution()` | [handler.go:1496](../chaoscenter/graphql/server/pkg/chaos_experiment_run/handler/handler.go#L1496) | Completion event with status + result |
| Resiliency scored | `scoreExperimentRun()` | [handler.go:1560+](../chaoscenter/graphql/server/pkg/chaos_experiment_run/handler/handler.go#L1560) | Scores: resiliency_score, experiments_passed/failed/etc, total_experiments_count |

### 2.3 What's Missing

The current tracing creates **one flat trace per experiment run** with metadata-only snapshots. It does **NOT** include:

| Gap | Detail |
|-----|--------|
| **No per-fault spans** | Individual faults (pod-cpu-hog, disk-fill, etc.) are not traced as child spans |
| **No fault config** | Target workload, namespace, duration, chaos parameters not captured |
| **No probe details** | Probe names, types, modes, success criteria not included |
| **No fault timeline** | Individual fault start/end times not recorded as separate observations |
| **No fault verdicts per-span** | Pass/Fail/Awaited verdicts only appear in aggregate scores |
| **Flat structure** | Everything under one trace with no parent-child hierarchy |

---

## 3. Experiment Flow: Where Fault Data is Available

### 3.1 Flow Diagram

```
User clicks "Run" ──► GraphQL mutation: runChaosExperiment(experimentID, projectID)
       │
       ▼
  RunChaosWorkFlow()  ◄── handler.go:1651
       │
       ├── 1. Generate notifyID (trace correlation ID)
       ├── 2. traceExperimentExecution()  ◄── CURRENT: creates Langfuse trace
       ├── 3. Load latest experiment revision (manifest YAML)
       ├── 4. Parse Argo Workflow + ChaosEngine templates     ◄── ★ FAULT DATA HERE
       ├── 5. Inject probes, normalize runtime, build params
       ├── 6. SendExperimentToSubscriber() → WebSocket to agent
       └── 7. Return { notifyID }
                │
                ▼  (async, minutes later)
  ChaosExperimentRunEvent()  ◄── handler.go:2152, called by subscriber
       │
       ├── 1. Parse ExecutionData (Argo node statuses)
       ├── 2. Extract ChaosData per node (verdict, probe%)     ◄── ★ RESULTS HERE
       ├── 3. ProcessCompletedExperimentRun() → resiliency score
       ├── 4. traceExperimentObservation()  ◄── CURRENT: adds observation
       ├── 5. scoreExperimentRun()          ◄── CURRENT: adds scores
       └── 6. Update MongoDB with final results
```

### 3.2 Data Available at Each Stage

#### Stage A: At Experiment Trigger Time (`RunChaosWorkFlow`)

The manifest is fully parsed at this point. The code already iterates over every template:

```go
// handler.go ~line 1770-1800
for i, template := range workflowManifest.Spec.Templates {
    artifacts := template.Inputs.Artifacts
    for j := range artifacts {
        data := artifacts[j].Raw.Data
        var meta chaosTypes.ChaosEngine
        yaml.Unmarshal([]byte(data), &meta)
        if meta.Kind == "chaosengine" {
            // ★ Full ChaosEngine available here:
            // - meta.Spec.Experiments[].Name      → fault name (e.g. "pod-cpu-hog")
            // - meta.Spec.AppInfo.AppNS            → target namespace
            // - meta.Spec.AppInfo.AppLabel          → target label selector
            // - meta.Spec.AppInfo.AppKind           → Deployment/StatefulSet/etc
            // - meta.Spec.Experiments[].Spec.Components.ENV  → chaos params (DURATION, CPU_CORES, etc.)
            // - annotation["probeRef"]              → probe definitions (JSON)
        }
    }
}
```

**Available fault data at trigger time:**

| Field | Source | Example |
|-------|--------|---------|
| Fault name | `meta.Spec.Experiments[0].Name` | `pod-cpu-hog` |
| Target kind | `meta.Spec.AppInfo.AppKind` | `deployment` |
| Target label | `meta.Spec.AppInfo.AppLabel` | `name=carts` |
| Target namespace | `meta.Spec.AppInfo.AppNS` | `sock-shop` |
| Chaos duration | `ENV["TOTAL_CHAOS_DURATION"]` | `30` |
| Chaos params | `ENV["CPU_CORES"]`, `ENV["MEMORY_CONSUMPTION"]`, etc. | `1`, `500` |
| Probe names | `annotation["probeRef"]` → JSON array | `check-frontend-access-url` |
| Probe type | From parsed probe ref | `httpProbe`, `k8sProbe` |
| Probe mode | From parsed probe ref | `Continuous`, `Edge` |
| Weightage | `rev.Weightages[].Weightage` | `10` |

#### Stage B: At Completion Time (`ChaosExperimentRunEvent`)

The subscriber reports back `ExecutionData` containing per-node results:

| Field | Source | Example |
|-------|--------|---------|
| Fault verdict | `node.ChaosExp.ExperimentVerdict` | `Pass`, `Fail`, `Awaited` |
| Probe success % | `node.ChaosExp.ProbeSuccessPercentage` | `100` |
| Engine name | `node.ChaosExp.EngineName` | `pod-cpu-hog-abc123` |
| Experiment pod | `node.ChaosExp.ExperimentPod` | `pod-cpu-hog-abc123-runner` |
| Fault start time | `node.StartedAt` | RFC3339 timestamp |
| Fault end time | `node.FinishedAt` | RFC3339 timestamp |
| Fault phase | `node.Phase` | `Succeeded`, `Failed` |
| Fail step | `node.ChaosExp.FailStep` | `ChaosInject` |
| Resiliency score | computed | `85.0` |

---

## 4. Proposed Enhancement: Rich Fault-Level Tracing

### 4.1 Target Trace Structure (Langfuse)

Compare with the **local trace** (from Flash-Agent) which already produces OTEL-style hierarchy:

```
Trace: experiment-run-{notifyID}
├── SPAN: agent-scan ({experimentName})           ← root span with full summary
├── GENERATION: llm-tool-selection                 ← (Flash-Agent only, skip)
├── SPAN: mcp-kubernetes-request                   ← (Flash-Agent only, skip)
├── GENERATION: llm-analysis                       ← (Flash-Agent only, skip)
├── SPAN: Step 1: install-application              ← infrastructure step
├── SPAN: Step 2: normalize-install-readiness      ← infrastructure step
├── SPAN: Step 7: pod-cpu-hog → deployment/carts   ← ★ FAULT with all details
├── SPAN: Step 8: pod-delete → deployment/catalogue
├── SPAN: Step 10: pod-memory-hog → deployment/orders [Pass]
├── SPAN: Step 11: disk-fill → statefulset/catalogue-db [Pass]
└── SPAN: Step 12: cleanup-chaos-resources
```

**Proposed server-side trace should mirror this WITHOUT Flash-Agent:**

```
Trace: experiment-run-{notifyID}
│   metadata: { experimentID, experimentName, infraID, projectID, timestamp }
│
├── SPAN: experiment-triggered
│     input:  { faultCount, cronSyntax, infraName }
│     output: { notifyID, manifestSize }
│
├── SPAN: fault: pod-cpu-hog → deployment/carts [sock-shop]
│     input:  { target, namespace, duration: 30s, params: {CPU_CORES: 1} }
│     output: { probes: ["check-frontend-access-url (HTTP, Continuous)"] }
│     metadata: { weightage: 10 }
│
├── SPAN: fault: pod-delete → deployment/catalogue [sock-shop]
│     input:  { target, namespace, duration: 30s, params: {CHAOS_INTERVAL: 10} }
│     output: { probes: ["check-catalogue-access-url (HTTP, Edge)"] }
│
├── SPAN: fault: pod-memory-hog → deployment/orders [sock-shop]
│     input:  { target, namespace, duration: 30s, params: {MEMORY_CONSUMPTION: 500} }
│     output: { probes: ["check-frontend-access-url (HTTP, Continuous)"] }
│
├── SPAN: fault: disk-fill → statefulset/catalogue-db [sock-shop]
│     input:  { target, namespace, duration: 30s, params: {FILL_PERCENTAGE: 100} }
│     output: { probes: ["check-catalogue-db-cr-status (k8sProbe, Continuous)"] }
│
└── (later, on completion event:)
    ├── EVENT: fault-result: pod-cpu-hog → Awaited (0%)
    ├── EVENT: fault-result: pod-delete → No Exporter Data
    ├── EVENT: fault-result: pod-memory-hog → Pass (100%)
    ├── EVENT: fault-result: disk-fill → Pass (100%)
    └── SCORE: resiliency_score = 85.0
```

### 4.2 Implementation Points (Two Hook Locations)

#### Hook 1: At Trigger Time — `RunChaosWorkFlow()` (handler.go ~line 1770)

**Where**: Inside the existing `for i, template := range workflowManifest.Spec.Templates` loop that already parses ChaosEngines.

**What to add**: After parsing each ChaosEngine, create a Langfuse observation/span per fault:

```
For each ChaosEngine template:
  → Extract: faultName, appKind, appLabel, appNS, chaosParams, probeRefs
  → Call: tracer.TraceExperimentObservation() with:
       TraceID:  notifyID
       Name:     "fault: {faultName} → {appKind}/{appLabel} [{appNS}]"
       Type:     "SPAN"
       Input:    { target, namespace, duration, params }
       Output:   { probes: [...] }
       Metadata: { weightage, engineName }
```

**Data already available** — the loop already has `meta` (parsed ChaosEngine) and `annotation` (with probeRef).

#### Hook 2: At Completion Time — `ChaosExperimentRunEvent()` (handler.go ~line 2200)

**Where**: Inside `ProcessCompletedExperimentRun()` or in the existing `traceExperimentObservation()` call, expand to iterate nodes.

**What to add**: For each `ChaosEngine` node in `executionData.Nodes`, create a Langfuse observation:

```
For each node where node.Type == "ChaosEngine" && node.ChaosExp != nil:
  → Extract: verdict, probeSuccessPercentage, startedAt, finishedAt
  → Call: tracer.TraceExperimentObservation() with:
       TraceID:  notifyID (traceID)
       Name:     "fault-result: {faultName} → {verdict} ({probeSuccess}%)"
       Type:     "EVENT"
       Input:    { engineName, experimentPod }
       Output:   { verdict, probeSuccessPercentage, failStep }
       Metadata: { startedAt, finishedAt, duration }
```

### 4.3 Files to Modify

| File | Change | Scope |
|------|--------|-------|
| [handler.go](../chaoscenter/graphql/server/pkg/chaos_experiment_run/handler/handler.go) | Add `traceFaultSpans()` call inside ChaosEngine template loop (~line 1800) | ~30 lines new code |
| [handler.go](../chaoscenter/graphql/server/pkg/chaos_experiment_run/handler/handler.go) | Add `traceFaultResults()` call after `ProcessCompletedExperimentRun()` (~line 2215) | ~25 lines new code |
| [langfuse_tracer.go](../chaoscenter/graphql/server/pkg/observability/langfuse_tracer.go) | No changes needed — existing `TraceExperimentObservation()` already supports SPAN type | None |
| [langfuse_client.go](../chaoscenter/graphql/server/pkg/agent_registry/langfuse_client.go) | No changes needed — `CreateObservation()` already calls `/api/public/ingestion` | None |

### 4.4 No New Dependencies Required

- The Langfuse client (`langfuse_client.go`) already supports creating observations of any type (`SPAN`, `EVENT`, `GENERATION`)
- The tracer (`langfuse_tracer.go`) already has a buffered async channel
- The ChaosEngine parsing loop already exists in `RunChaosWorkFlow()`
- The ExecutionData node iteration already exists in `ProcessCompletedExperimentRun()`

---

## 5. Comparison: Flash-Agent Trace vs. Proposed Server Trace

| Aspect | Flash-Agent Trace (current local) | Proposed Server Trace |
|--------|-----------------------------------|-----------------------|
| **Trigger** | Flash-Agent scans & triggers | User clicks "Run" in UI |
| **Source** | Flash-Agent Python SDK (OTEL) | ChaosCenter Go server (Langfuse REST) |
| **LLM spans** | Yes (tool-selection, analysis) | No (no LLM involvement) |
| **Fault spans** | Yes (from LLM analysis output) | Yes (from manifest parsing + execution data) |
| **Real-time** | Post-hoc (after scan completes) | **Real-time** (at trigger + on each event) |
| **Fault config** | Extracted from manifest by LLM | Extracted directly from ChaosEngine YAML |
| **Fault results** | From chaos-exporter logs | From subscriber's ExecutionData |
| **Probe details** | From manifest analysis | From probeRef annotations |
| **Completeness** | Depends on LLM + exporter state | **100%** — all faults always captured |

---

## 6. Data Flow Summary

```
┌─────────────────────────────────────────────────────────────────┐
│                    ChaosCenter GraphQL Server                    │
│                                                                  │
│  runChaosExperiment() ──────────────────────────────────────►   │
│       │                                                          │
│       ├─ 1. Create trace (notifyID)         ──► Langfuse Trace  │
│       │                                                          │
│       ├─ 2. Parse manifest                                       │
│       │    └─ For each ChaosEngine:                              │
│       │        ├─ Extract fault config                           │
│       │        └─ Create fault SPAN          ──► Langfuse Span  │ ◄── NEW
│       │                                                          │
│       ├─ 3. Send to subscriber ──► K8s cluster                   │
│       │                                                          │
│  (async) ChaosExperimentRunEvent() ◄──────── subscriber reports │
│       │                                                          │
│       ├─ 4. Parse ExecutionData                                  │
│       │    └─ For each ChaosEngine node:                         │
│       │        ├─ Extract verdict + probe%                       │
│       │        └─ Create result EVENT        ──► Langfuse Event │ ◄── NEW
│       │                                                          │
│       └─ 5. Score resiliency                 ──► Langfuse Score │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 7. Estimated Effort & Risk

| Aspect | Assessment |
|--------|------------|
| **Code changes** | ~55 lines of new Go code in `handler.go` (two new helper functions) |
| **Existing infra** | 100% reuse — no new dependencies, clients, or configs |
| **Testing** | Can test locally by setting `LANGFUSE_*` env vars on graphql-server |
| **Risk** | Low — all tracing is fire-and-forget (async channel), failures don't block experiment execution |
| **Backward compat** | Full — new observations are additive, existing traces unaffected |
| **Flash-Agent** | Completely uninvolved — all changes in graphql-server only |

---

## 8. Open Questions

1. **Should fault SPANs be created even for non-ChaosEngine templates?** (e.g., install-application, load-test infrastructure steps) — The local Flash-Agent trace does include these as "infrastructure" spans.

2. **Should the server trace and Flash-Agent trace coexist?** If both are enabled, the same experiment will have two traces in Langfuse. Consider using a shared `sessionID` (e.g., experimentRunID) to group them.

3. **Langfuse version requirement**: The server currently uses `/api/public/traces` and `/api/public/ingestion` REST endpoints (not OTEL). These work on Langfuse v2+, so **no Langfuse upgrade is required** for this change (unlike the Flash-Agent OTEL fix which needs v3.6.0+).

---

## 9. OTEL Compliance Analysis: Server Traces vs. Flash-Agent Traces

### 9.1 The Problem: Two Different Protocols

The Flash-Agent and the ChaosCenter GraphQL server use **fundamentally different protocols** to send data to Langfuse:

| Aspect | Flash-Agent (Python) | GraphQL Server (Go) |
|--------|---------------------|---------------------|
| **Protocol** | OTLP (OpenTelemetry Line Protocol) | Langfuse REST API (proprietary) |
| **SDK** | OpenTelemetry SDK v1.40.0 + Langfuse SDK v4.0.0 | Custom HTTP client (`langfuse_client.go`) |
| **Endpoint** | `POST /api/public/otel` (OTLP receiver) | `POST /api/public/traces` + `POST /api/public/ingestion` |
| **Auth** | OTEL headers (`Authorization: Bearer <public_key>`) | HTTP Basic Auth (`publicKey:secretKey`) |
| **Format** | OTLP JSON/Protobuf (W3C trace context) | Langfuse-native JSON |
| **Trace IDs** | 128-bit hex (W3C standard) | UUID strings |
| **Span hierarchy** | Parent-child via `parentSpanId` | Flat observations under a trace |

### 9.2 Field-by-Field Comparison from the Trace JSON

Looking at the Flash-Agent trace (`Local setup trace.json`), each span has OTEL resource attributes:

```json
// Flash-Agent span metadata (OTEL-compliant) ✅
{
  "resourceAttributes": {
    "telemetry.sdk.language": "python",
    "telemetry.sdk.name": "opentelemetry",
    "telemetry.sdk.version": "1.40.0",
    "service.name": "unknown_service"
  },
  "scope": {
    "name": "langfuse-sdk",
    "version": "4.0.0",
    "attributes": {
      "public_key": "pk-lf-c07594f1-40c5-401e-a5e4-13399f095a92"
    }
  }
}
```

The server's `ExperimentTrace` struct sends **none of these**:

```json
// Server trace payload (NOT OTEL-compliant) ❌
{
  "id": "notifyID-uuid",
  "name": "chaos-workflow",
  "experimentId": "...",
  "faultName": "...",
  "sessionId": "...",
  "startTime": 1711234567890,
  "status": "RUNNING",
  "input": { ... },
  "metadata": { ... }
}
// Missing: resourceAttributes, scope, telemetry.sdk.*, service.name
// Missing: W3C traceId/spanId format, parentSpanId, spanKind, statusCode
```

### 9.3 Structural Differences

| OTEL Attribute | Flash-Agent Trace | Server Trace | Gap |
|----------------|-------------------|--------------|-----|
| `traceId` (128-bit hex) | `ab17d1c1e655e161` | UUID like `3f4a2b1c-...` | **Mismatch** — not W3C format |
| `spanId` (64-bit hex) | `8f19b12a7b16bad1` | `obs-1711234567890` (nano timestamp) | **Mismatch** — not hex |
| `parentSpanId` | Implicit via `depth` field | Not sent at all | **Missing** — no parent-child |
| `telemetry.sdk.language` | `python` | Not sent | **Missing** |
| `telemetry.sdk.name` | `opentelemetry` | Not sent | **Missing** |
| `telemetry.sdk.version` | `1.40.0` | Not sent | **Missing** |
| `service.name` | `unknown_service` | Not sent | **Missing** |
| `scope.name` | `langfuse-sdk` | Not sent | **Missing** |
| `scope.version` | `4.0.0` | Not sent | **Missing** |
| Span `type` | `SPAN`, `GENERATION` | `SPAN`, `EVENT` | Partial match |
| Span `depth` | `0` (root), `1` (child) | Flat — all observations at same level | **Missing** — no hierarchy |
| `input` / `output` | Structured JSON | Structured JSON | **Match** ✅ |
| `metadata` | Structured JSON | Structured JSON | **Match** ✅ |
| `startTime` / `endTime` | ISO 8601 (`2026-03-25T10:07:43.996Z`) | ISO 8601 (RFC3339) | **Match** ✅ |

### 9.4 Verdict: NOT OTEL-Compliant

**The proposed server-side changes (as described in Section 4) would NOT produce OTEL-compliant traces.** They would produce Langfuse-native traces that look different from Flash-Agent traces in Langfuse.

Key incompatibilities:
1. **No OTEL resource attributes** — data science team cannot filter by `telemetry.sdk.*` or `service.name`
2. **No span hierarchy** — Langfuse REST observations are flat, not parent-child
3. **No W3C trace context** — trace/span IDs are UUIDs, not 128/64-bit hex
4. **Different ingestion path** — REST → Langfuse internal model vs. OTLP → Langfuse OTEL receiver → internal model
5. **No instrumentation scope** — no `scope.name`/`scope.version` metadata

### 9.5 Option Analysis: How to Make Server Traces OTEL-Compliant

#### Option A: Add Go OTEL SDK to GraphQL Server (Recommended)

Replace the custom Langfuse REST client with the standard Go OTEL SDK exporting to Langfuse's OTLP endpoint.

```
Current:  handler.go → langfuse_client.go → POST /api/public/traces (REST)
Proposed: handler.go → Go OTEL SDK → OTLP exporter → POST /api/public/otel (OTLP)
```

| Aspect | Detail |
|--------|--------|
| **Go OTEL SDK** | `go.opentelemetry.io/otel` (mature, CNCF project) |
| **OTLP Exporter** | `go.opentelemetry.io/otel/exporters/otlp/otlptracehttp` |
| **Endpoint** | Same Langfuse OTEL endpoint: `http://100.78.130.20:3001/api/public/otel` |
| **Auth** | HTTP header: `Authorization: Bearer <LANGFUSE_PUBLIC_KEY>` |
| **Resource** | `resource.NewWithAttributes(service.name="chaoscenter-server", telemetry.sdk.language="go")` |
| **Trace IDs** | Auto-generated W3C 128-bit hex |
| **Parent-child** | Native via `tracer.Start(ctx, "spanName")` + context propagation |
| **Effort** | ~100 lines: init tracer provider + replace `LangfuseTracer` calls with OTEL spans |
| **New deps** | `go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/exporters/otlp/otlptracehttp`, `go.opentelemetry.io/otel/sdk` |

**Resulting trace would look like:**

```json
{
  "resourceAttributes": {
    "telemetry.sdk.language": "go",
    "telemetry.sdk.name": "opentelemetry",
    "telemetry.sdk.version": "1.33.0",
    "service.name": "chaoscenter-graphql-server"
  },
  "traceId": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6",
  "spans": [
    {
      "spanId": "1a2b3c4d5e6f7a8b",
      "name": "experiment-run",
      "parentSpanId": "",
      "kind": "INTERNAL",
      "attributes": { "experiment.id": "...", "experiment.name": "..." }
    },
    {
      "spanId": "2b3c4d5e6f7a8b9c",
      "parentSpanId": "1a2b3c4d5e6f7a8b",
      "name": "fault: pod-cpu-hog → deployment/carts",
      "kind": "INTERNAL",
      "attributes": { "fault.name": "pod-cpu-hog", "fault.target": "deployment/carts" }
    }
  ]
}
```

#### Option B: Enhance Langfuse REST Client with OTEL Metadata (Partial Compliance)

Keep the REST client but manually add OTEL-like fields to the JSON payloads.

| Aspect | Detail |
|--------|--------|
| **Change** | Add `resourceAttributes`, `scope`, W3C-format IDs to `ExperimentTrace` struct |
| **Endpoint** | Still `POST /api/public/traces` (REST) |
| **Effort** | ~40 lines: extend structs + populate OTEL fields |
| **Compliance** | **Partial** — Langfuse stores the metadata but it's NOT true OTLP |
| **Limitation** | Langfuse REST API doesn't support `parentSpanId` — hierarchy still flat |
| **Limitation** | No W3C trace context propagation (no `traceparent` header) |
| **Limitation** | Data science queries on OTEL fields may not work consistently |

#### Option C: Dual Output (REST + OTEL)

Keep existing REST tracing for backward compatibility, ADD OTEL export alongside.

| Aspect | Detail |
|--------|--------|
| **Change** | Add OTEL tracer provider + call both REST and OTEL |
| **Effort** | ~120 lines |
| **Trade-off** | Duplicate traces in Langfuse (one REST, one OTEL) unless REST is removed |

### 9.6 Recommendation

**Option A (Go OTEL SDK)** is the correct path:

1. **True OTEL compliance** — same protocol as Flash-Agent
2. **Structural match** — parent-child hierarchy, W3C IDs, resource attributes
3. **Future-proof** — can export to any OTEL backend (Jaeger, Grafana Tempo, etc.)
4. **Data science consistency** — both traces queryable with same OTEL field names
5. **Industry standard** — Go OTEL SDK is production-grade (used by Kubernetes, Istio, etc.)

The `langfuse_client.go` REST approach should be **deprecated** once OTEL is implemented, but can coexist during migration.

### 9.7 Trace Structure Comparison (After Option A)

```
=== Flash-Agent Trace (Python OTEL SDK → Langfuse OTLP) ===

Trace: ab17d1c1e655e161
├── SPAN: agent-scan (depth=0)
│   resource: { sdk.language: python, sdk.name: opentelemetry, sdk.version: 1.40.0 }
│   scope:    { name: langfuse-sdk, version: 4.0.0 }
│
├── GENERATION: llm-tool-selection (depth=1, parentSpan=root)
├── SPAN: mcp-kubernetes-request (depth=1)
├── GENERATION: llm-analysis (depth=1)
├── SPAN: Step 7: pod-cpu-hog → deployment/carts (depth=1)
│   input:  { fault_type, target, params: {CPU_CORES: 1} }
│   output: { verdict, probe_success_percentage }
├── SPAN: Step 10: pod-memory-hog → deployment/orders [Pass] (depth=1)
└── SPAN: Step 11: disk-fill → statefulset/catalogue-db [Pass] (depth=1)


=== Server Trace (Go OTEL SDK → Langfuse OTLP) — After Option A ===

Trace: c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0
├── SPAN: experiment-run (depth=0, kind=INTERNAL)
│   resource: { sdk.language: go, sdk.name: opentelemetry, sdk.version: 1.33.0,
│               service.name: chaoscenter-graphql-server }
│   scope:    { name: chaoscenter-observability }
│   attrs:    { experiment.id, experiment.name, infra.id, project.id }
│
├── SPAN: fault: pod-cpu-hog → deployment/carts [sock-shop] (depth=1, parent=root)
│   attrs:  { fault.name, fault.target.kind, fault.target.label, fault.duration }
│   input:  { params: {CPU_CORES: 1, TOTAL_CHAOS_DURATION: 30} }
│   output: { probes: ["check-frontend-access-url"] }
│
├── SPAN: fault: pod-delete → deployment/catalogue [sock-shop] (depth=1)
├── SPAN: fault: pod-memory-hog → deployment/orders [sock-shop] (depth=1)
├── SPAN: fault: disk-fill → statefulset/catalogue-db [sock-shop] (depth=1)
│
│   (later, on completion event, child spans updated or new spans created:)
├── SPAN: fault-result: pod-cpu-hog → Awaited (0%) (depth=1)
├── SPAN: fault-result: pod-memory-hog → Pass (100%) (depth=1)
└── SPAN: fault-result: disk-fill → Pass (100%) (depth=1)

Both traces:
  ✅ Same OTLP endpoint: POST /api/public/otel
  ✅ Same W3C trace ID format (128-bit hex)
  ✅ Same parent-child hierarchy
  ✅ Same resourceAttributes structure
  ✅ Same scope metadata
  ✅ Queryable with same OTEL field names
```

### 9.8 Required Go Dependencies for Option A

```go
// go.mod additions
require (
    go.opentelemetry.io/otel                           v1.33.0
    go.opentelemetry.io/otel/sdk                       v1.33.0
    go.opentelemetry.io/otel/exporters/otlp/otlptracehttp v1.33.0
    go.opentelemetry.io/otel/trace                     v1.33.0
)
```

### 9.9 Implementation Changes Summary (Option A)

| File | Change |
|------|--------|
| `go.mod` | Add OTEL SDK dependencies |
| **NEW**: `pkg/observability/otel_tracer.go` | Initialize OTEL TracerProvider with OTLP HTTP exporter pointing to Langfuse |
| `handler.go` (`RunChaosWorkFlow`) | Replace `traceExperimentExecution()` with OTEL `tracer.Start(ctx, "experiment-run")` |
| `handler.go` (ChaosEngine loop) | Replace proposed REST observation with OTEL `tracer.Start(ctx, "fault: ...")` child span |
| `handler.go` (`ChaosExperimentRunEvent`) | Replace `traceExperimentObservation()` with OTEL span events/child spans |
| `langfuse_tracer.go` | Deprecate (keep for backward compat) or remove |
| `langfuse_client.go` | Deprecate `TraceExperiment()` and `CreateObservation()` (keep `CreateOrUpdateUser`, `CreateScore`) |

### 9.10 Env Var Alignment

| Current (REST) | Proposed (OTEL) | Notes |
|----------------|-----------------|-------|
| `LANGFUSE_HOST` | `OTEL_EXPORTER_OTLP_ENDPOINT` | Value: `http://100.78.130.20:3001/api/public/otel` |
| `LANGFUSE_PUBLIC_KEY` | `OTEL_EXPORTER_OTLP_HEADERS` | Value: `Authorization=Bearer pk-lf-...` |
| `LANGFUSE_SECRET_KEY` | (not needed for OTEL) | OTEL uses public key only |
| (new) | `OTEL_SERVICE_NAME` | Value: `chaoscenter-graphql-server` |

---

## 10. Architectural Decision: OTEL Module Placement (In-Repo vs External)

### 10.1 Question

Should the OTEL module (OTEL SDK, OTLP exporter, Trace API) live **inside** the AgentCert/ChaosCenter repo, or be extracted into a **separate external repository/Go module**?

### 10.2 Verdict: Keep OTEL Inside the ChaosCenter Server

An external module is **overhead, not good design** for this project's current and foreseeable scale.

### 10.3 Why "External Module" Sounds Appealing But Doesn't Apply Here

The instinct to extract OTEL into a shared library comes from a valid principle — DRY / single responsibility. But that principle doesn't apply here because:

#### 10.3.1 The Go OTEL SDK Is Already the Shared Library

We're not writing a tracing framework. The implementation is **~100 lines of glue code** that:
- Calls `otel.SetTracerProvider(...)` at startup
- Calls `tracer.Start(ctx, "experiment-run")` in handler.go
- Calls `span.End()` + `span.SetAttributes(...)` at completion

This is **application-level instrumentation**, not a reusable library. Extracting it creates dependency management problems (versioning, Go module replace directives, CI/CD coordination) with zero reuse benefit.

#### 10.3.2 Only One Go Consumer Exists

| Service | Language | Needs OTEL? | How It Gets OTEL |
|---------|----------|-------------|------------------|
| **Flash-Agent** | Python | Already has it | `azure-monitor-opentelemetry` + env vars (auto-instrumentation) |
| **GraphQL Server** | Go | **Needs it** | **~100 lines in `pkg/observability/`** |
| Subscriber | Go | No | Reports to server, doesn't trace independently |
| Event Tracker | Go | No | Internal event relay component |
| Authentication | Go | No | Auth service, not traced to Langfuse |
| Web Frontend | JS/TS | No | UI layer, not traced |

The GraphQL server is the **only Go service** that needs OTEL tracing to Langfuse. There is no second consumer to justify a shared module.

### 10.4 Concrete Overhead of an External Module

| Overhead | Impact |
|----------|--------|
| **Separate Git repo** | PRs spanning 2 repos for a single feature; merge ordering issues |
| **Go module versioning** | `go.mod` replace directives during development; must tag releases for each change |
| **CI/CD coupling** | Build pipeline must pull external module; version pinning across services |
| **Debugging** | Stack traces cross module boundaries; IDE jump-to-definition breaks across repos |
| **Onboarding** | New developers must understand multi-repo structure for a ~100-line package |
| **Testing** | Integration tests need both repos checked out; mock boundaries more complex |
| **Change velocity** | Every trace schema change (new span attributes, new span names) requires: (1) PR to external module, (2) tag release, (3) PR to ChaosCenter to bump version — **3 PRs instead of 1** |

### 10.5 Comparison: External vs In-Repo

| Criterion | External Module | In-Repo (`pkg/observability/`) |
|-----------|----------------|-------------------------------|
| **Code volume** | Same ~100 lines, but in a separate repo | Same ~100 lines, colocated with handler.go |
| **Dependencies** | `go.opentelemetry.io/otel` (same either way) | `go.opentelemetry.io/otel` (same either way) |
| **Change workflow** | 3 PRs per trace change (module → tag → consumer) | **1 PR per trace change** |
| **Build complexity** | External module fetch + version pin | Zero — already in the Go module |
| **Debugging** | Cross-repo stack traces | Single-repo, full IDE support |
| **Testing** | Separate test suite + integration tests across repos | Tests colocated with handler tests |
| **Reuse benefit** | None today (1 consumer) | N/A |
| **Future extraction** | Already done | Trivial to extract later if needed |

### 10.6 Industry Precedent

Every major OTEL-instrumented project keeps tracer initialization **inside the service binary**:

| Project | Language | OTEL Placement |
|---------|----------|---------------|
| Kubernetes (kubelet, kube-apiserver) | Go | `staging/src/k8s.io/component-base/tracing/` — in-repo |
| Istio (pilot, envoy proxy) | Go | `pilot/pkg/features/` — in-repo |
| Grafana | Go | `pkg/infra/tracing/` — in-repo |
| Jaeger | Go | `cmd/*/` — in each service binary |
| CockroachDB | Go | `pkg/obsservice/` — in-repo |

The **OTEL SDK itself** (`go.opentelemetry.io/otel`) is the external, reusable dependency. Application code that calls the SDK is always in-process.

### 10.7 Analogy: How Flash-Agent Already Does It

The Flash-Agent (Python) takes the same approach:
- **External dependency**: `opentelemetry-sdk` via pip (CNCF-maintained)
- **In-repo usage**: Environment variable configuration + auto-instrumentation
- **No separate Python package** was created to wrap the OTEL setup

The Go server should follow the identical pattern:
- **External dependency**: `go.opentelemetry.io/otel` via Go modules (CNCF-maintained)
- **In-repo glue**: `pkg/observability/otel_tracer.go` (~80 lines init + config)
- **In-repo usage**: `handler.go` calls `tracer.Start(ctx, "span-name")`

### 10.8 When an External Module WOULD Make Sense

An external OTEL library would be justified **only if**:

1. **3+ Go services** all need identical OTEL tracer setup with the same resource attributes, exporters, and sampling config — today there is only 1
2. **Custom OTEL exporter or sampler** that needs independent versioning and release cadence — not applicable here (we use the standard OTLP HTTP exporter)
3. **Platform SDK** consumed by multiple teams across different repositories — AgentCert is a single-team product
4. **Multi-language shared config** — impossible; Go and Python OTEL SDKs are fundamentally different APIs

### 10.9 Scalability Concerns Addressed

| Scalability Dimension | External Module Helps? | Reality |
|-----------------------|-----------------------|---------|
| **Data volume scalability** | No | Handled by OTEL SDK's built-in batching + Langfuse ingestion capacity |
| **Service count scalability** | Only if 3+ services need it | Only 1 Go service needs it today |
| **Feature scalability** (adding spans/attributes) | Makes it harder (multi-repo PRs) | **Easier in-repo** — handler + tracer change in same PR |
| **Team scalability** | Only if multiple teams maintain it | Single team today |
| **Backend portability** (swap Langfuse for Jaeger) | Same either way | OTEL SDK supports multiple exporters via config, no code change needed |

### 10.10 Recommended File Structure

```
chaoscenter/graphql/server/
├── pkg/observability/
│   ├── otel_tracer.go        ← NEW: ~80 lines
│   │   - InitOTELTracer(): creates TracerProvider + OTLP HTTP exporter
│   │   - ShutdownOTELTracer(): graceful flush on server shutdown
│   │   - GetTracer(): returns named tracer for "chaoscenter-observability"
│   │
│   └── langfuse_tracer.go    ← EXISTING: deprecate trace/observation methods
│       - Keep: ScoreExperimentExecution() (scores use REST /api/public/ingestion)
│       - Deprecate: TraceExperimentExecution(), CompleteExperimentExecution()
│       - Deprecate: TraceExperimentObservation()
│
├── pkg/chaos_experiment_run/handler/
│   └── handler.go             ← EDIT: replace REST trace calls with OTEL spans
│       - RunChaosWorkFlow():           tracer.Start(ctx, "experiment-run")
│       - ChaosEngine parse loop:       tracer.Start(ctx, "fault: pod-cpu-hog → ...")
│       - ChaosExperimentRunEvent():    span.SetAttributes() + span.End()
│
└── go.mod                     ← ADD: go.opentelemetry.io/otel + exporters
```

### 10.11 Decision Summary

| Decision | Keep OTEL in-repo under `pkg/observability/` |
|----------|----------------------------------------------|
| **Rationale** | Single consumer, ~100 lines of glue code, zero reuse case today |
| **Trade-off accepted** | If a 2nd Go service needs OTEL later, we extract then (trivial) |
| **Trade-off avoided** | Multi-repo coordination tax for every trace schema change |
| **Precedent** | Kubernetes, Istio, Grafana, Jaeger all keep OTEL init in-repo |
| **Principle** | "Extract when you have 3 consumers, not 1" (Rule of Three) |

> **Bottom line**: The Go OTEL SDK (`go.opentelemetry.io/otel`) is the external, reusable component. Our `otel_tracer.go` is application configuration — it belongs next to the application code it configures. Ship it in one PR. Extract only if a second Go service needs the same tracer setup — evaluate when that actually happens.
