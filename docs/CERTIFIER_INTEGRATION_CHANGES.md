# Certifier Integration Changes

**Date:** April 28, 2026  
**Scope:** `D:\Studies\AgentCert` — GraphQL server, agent-sidecar, trace extractor  
**Goal:** Make the certifier's `FaultBucketingPipeline` work deterministically with real Langfuse traces

---

## Problem Statement

The certifier (`D:\Studies\certifier`) requires:
1. A **flat JSON array** of span objects as input (not a nested trace record)
2. **`fault: <name>` SPAN** observations in the trace so it can create per-fault buckets deterministically without LLM
3. An **`experiment_context` SPAN** before any fault span so it can extract `agent_id`, `experiment_id`, `run_id` from chronological scan
4. **`run_id`** key in observation metadata (certifier doesn't recognise `experiment_run_id`)
5. **`agent_version`** in observation metadata for output reporting

None of these existed in traces before this change.

---

## Architecture: Where Each Piece Lives

```
Experiment Start (GraphQL server, handler.go)
  │
  ├─ [NEW] experiment_context SPAN → Langfuse (T)
  │        carries: agent_id, agent_name, experiment_id, run_id, fault_names
  │
  ├─ [NEW] fault: <name> SPAN per fault → Langfuse (T+1s)
  │        carries: ground_truth (full fault_description, ideal_course_of_action,
  │                 ideal_tool_usage_trajectory from chaos hub YAML)
  │
  └─ Workflow submitted to Argo → flash-agent starts ~60s later

flash-agent scan (every ~65s, via sidecar → LiteLLM → Azure OpenAI)
  │
  └─ proxy.py intercepts each LLM call
       [UPDATED] injects run_id + agent_version into metadata
       → LiteLLM Langfuse callback → GENERATION obs on agent trace

trace_extractor.py
  [UPDATED] --certifier-output <path> → flat [{span}, ...] array

certifier FaultBucketingPipeline
  reads flat array
  → finds experiment_context → extracts agent_id, experiment_id, run_id
  → finds fault: * spans → creates per-fault buckets (deterministic, no LLM)
  → assigns LLM observations to buckets via LLM classifier
  → outputs per-fault JSON files with full ground truth
```

---

## Files Changed

### 1. `chaoscenter/graphql/server/pkg/chaos_experiment_run/handler/handler.go`

**What:** Added goroutine call after `traceExperimentExecution()` at experiment start.

**Where:** After `applyUninstallAllPatchToWorkflowSpec()`, before agentId injection loop.

```go
go func(tid string, templates []v1alpha1.Template, expCtx observability.ExperimentContextForTrace) {
    faultNames := ops.ExtractChaosEngineFaults(templates)
    if len(faultNames) > 0 {
        groundTruth := ops.LoadFaultGroundTruthsDecoded(faultNames)
        if groundTruth == nil {
            groundTruth = make(map[string]interface{})
        }
        lft := observability.GetLangfuseTracer()
        lft.EmitFaultSpansForTrace(context.Background(), tid, faultNames, groundTruth, expCtx)
    }
}(notifyID, workflowManifest.Spec.Templates, observability.ExperimentContextForTrace{
    AgentID:        traceAgentID,
    AgentName:      traceAgentName,
    AgentPlatform:  traceAgentPlatform,
    ExperimentID:   workflow.ExperimentID,
    ExperimentName: workflow.Name,
    Namespace:      *infra.InfraNamespace,  // guarded nil-check inline
})
```

**Why goroutine:** Fire-and-forget — Langfuse write must not block experiment submission to Argo.

**Why `context.Background()`:** The request context `ctx` may be cancelled once the handler returns, before the goroutine completes.

---

### 2. `chaoscenter/graphql/server/pkg/observability/langfuse_tracer.go`

**What:** Added `ExperimentContextForTrace` struct and `EmitFaultSpansForTrace()` method.

#### `ExperimentContextForTrace` struct
```go
type ExperimentContextForTrace struct {
    AgentID        string
    AgentName      string
    AgentPlatform  string
    ExperimentID   string
    ExperimentName string
    Namespace      string
}
```

#### `EmitFaultSpansForTrace()` method
- Emits **`experiment_context` SPAN at T** with all identity fields in metadata
- Emits **`fault: <name>` SPAN at T+1s** per fault with full `ground_truth` in both `input` and `metadata`
- Uses existing `client.CreateObservation()` → POSTs to `/api/public/ingestion`
- T+1s offset guarantees `experiment_context` sorts before any `fault:` span chronologically

**Span shapes:**

`experiment_context`:
```json
{
  "name": "experiment_context",
  "type": "SPAN",
  "metadata": {
    "agent_id": "...",
    "agent_name": "...",
    "experiment_id": "...",
    "run_id": "<notifyID>",
    "namespace": "...",
    "fault_names": ["pod-delete", "disk-fill", ...]
  }
}
```

`fault: <name>`:
```json
{
  "name": "fault: pod-delete",
  "type": "SPAN",
  "input": {
    "fault_name": "pod-delete",
    "ground_truth": {
      "fault_description_goal_remediation": { "goal": "...", "remediation": "...", "symptoms": [...] },
      "ideal_course_of_action": [...],
      "ideal_tool_usage_trajectory": [...]
    }
  },
  "metadata": {
    "action": "fault_injection",
    "fault_name": "pod-delete",
    "ground_truth": { ... },
    "llm_used": false,
    "tokens_consumed": 0
  }
}
```

**No data leakage:** These are pure Langfuse writes from the GraphQL server. Flash-agent never reads from Langfuse — it only writes via LiteLLM callback. Ground truth never enters LLM `messages`.

---

### 3. `chaoscenter/graphql/server/pkg/chaos_experiment/ops/service.go`

**What:** Added two exported wrappers around existing private functions so `handler.go` can call them at run time.

```go
// ExtractChaosEngineFaults — exported wrapper
func ExtractChaosEngineFaults(templates []v1alpha1.Template) []string {
    return extractChaosEngineFaults(templates)
}

// LoadFaultGroundTruthsDecoded — exported wrapper returning decoded map
// (not base64) for direct use in Langfuse span payloads
func LoadFaultGroundTruthsDecoded(faultNames []string) map[string]interface{} {
    b64 := loadFaultGroundTruths(faultNames)
    // base64 decode + json.Unmarshal → returns map[fault_name]ground_truth_data
}
```

**Why new wrappers instead of calling private functions directly:** Go visibility rules — `extractChaosEngineFaults` and `loadFaultGroundTruths` are in package `ops`; `handler.go` is in package `handler`. Private functions are not accessible across packages.

---

### 4. `agent-sidecar/proxy.py`

**What:** Three additions to metadata injection in `_inject_metadata()`.

#### 4a. `run_id` alias
```python
if context.get("experiment_run_id"):
    metadata["experiment_run_id"] = context["experiment_run_id"]
    metadata["run_id"] = context["experiment_run_id"]  # ← NEW alias
```
**Why:** Certifier's `_extract_agent_metadata()` looks for `run_id` key exactly (`d.get("run_id") or d.get("experiment.run_id")`). Without this alias, `bucket.run_id` is `None` and MongoDB stores the result under a null key.

#### 4b. `agent_version` injection
```python
if "agent_version" in context:
    metadata["agent_version"] = context["agent_version"]  # ← NEW
```
**Why:** Certifier requires `agent_version` for output metadata. Without it, `bucket.agent_version = None`.

#### 4c. `AGENT_VERSION` in `_CONTEXT_KEYS`
```python
_CONTEXT_KEYS = (
    "NOTIFY_ID", "EXPERIMENT_ID", "EXPERIMENT_RUN_ID",
    "WORKFLOW_NAME", "AGENT_NAME", "AGENT_ROLE", "AGENT_ID",
    "AGENT_VERSION",  # ← NEW
)
```
**Why:** `_load_context()` reads ConfigMap files by iterating `_CONTEXT_KEYS`. Without this, `AGENT_VERSION` is never read from `/etc/agent/metadata/AGENT_VERSION`.

---

### 5. `local-custom/scripts/trace_extractor.py`

**What:** Added `--certifier-output` CLI flag that writes a flat observations-only JSON array.

```python
p.add_argument(
    "--certifier-output",
    default="",
    help="If set, also write a flat observations-only JSON array to this path (certifier input format)",
)
```

At end of `main()`:
```python
if args.certifier_output:
    flat_obs = []
    for r in records:
        flat_obs.extend(r.get("observations") or [])
    with open(cert_out, "w", encoding="utf-8") as f:
        json.dump(flat_obs, f, indent=2, ensure_ascii=False)
```

**Why:** `FaultBucketingPipeline._load_trace()` validates `isinstance(data, list)` and raises `FaultBucketingError` if not. The full extractor output is `[{trace_record, observations: [...]}]` — a list of trace objects, not a flat span array. The certifier needs the inner observations array flattened across all traces.

**Usage:**
```bash
python3 trace_extractor.py \
  --env-file /mnt/d/Studies/AgentCert/local-custom/config/.env \
  --from-ist "2026-04-28 20:00" \
  --output /mnt/d/Studies/certifier/trace_dump/extracted_traces.json \
  --certifier-output /mnt/d/Studies/certifier/trace_dump/certifier_input.json
```

Then pass `certifier_input.json` to `FaultBucketingPipeline`.

---

## Gap Analysis: Before vs After

| # | What certifier needs | Before | After |
|---|---|---|---|
| 1 | Flat `[{span}...]` input | Wrapped trace record — **BLOCKER** | `--certifier-output` flag ✅ |
| 2 | `fault: *` spans for deterministic bucketing | 0 spans in trace — **BLOCKER** | Emitted from handler.go at T+1s ✅ |
| 3 | `experiment_context` span before fault spans | Missing — metadata scan got None for all fields | Emitted from handler.go at T ✅ |
| 4 | `run_id` key in obs metadata | Only `experiment_run_id` — bucket.run_id = None | `run_id` alias added in proxy.py ✅ |
| 5 | `agent_version` in obs metadata | Not injected anywhere | Added to `_CONTEXT_KEYS` + injection ✅ |
| 6 | `ground_truth` in fault span input/metadata | Not present | Full chaos hub YAML data in both fields ✅ |

---

## What Is NOT Changed

- Flash-agent source code — remains completely blind to fault injection details
- LiteLLM proxy configuration — no changes
- agent-charts Helm charts — no changes
- Ground truth data path to sidecar (ConfigMap → proxy.py → LiteLLM metadata) — unchanged
- LLM `messages` — proxy.py never touches messages, only metadata
- Data leakage surface — no new paths added that expose fault data to the LLM
