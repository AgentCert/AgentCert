# Experiment End-to-End Flow

Complete technical walkthrough: from UI click → IDs generated → workflow runs → LLM calls → traces land in Langfuse.

---

## 1. High-Level Overview

```
User clicks "Run Experiment" in UI
        │
        ▼
GraphQL mutation: RunChaosExperiment
        │
        ├─ Generate notifyID (UUID)          ← THIS is the Langfuse trace ID
        ├─ Generate experimentRunID (UUID)
        ├─ Resolve agentID from MongoDB
        │
        ▼
Patch workflow manifest (server-side, before sending to Argo):
  ├─ Inject notify_id as workflow label
  ├─ Inject agentId as workflow parameter
  ├─ Inject pre-cleanup-wait step (dynamic)
  ├─ Inject uninstall-all step (dynamic)
  ├─ Inject install-agent --set flags (experiment context + GT data)
  └─ Normalize install-application readiness
        │
        ▼
Submit patched manifest to Argo (via subscriber → Kubernetes)
        │
        ▼
Argo runs steps sequentially:
  1. install-application   → deploys app (e.g. sock-shop) via Helm
  2. install-agent         → deploys flash-agent + sidecar via Helm
                              └─ registers agent in MongoDB (self-registration)
                              └─ writes NOTIFY_ID, GROUND_TRUTH_JSON to ConfigMap
  3. install-chaos-faults  ┐
     load-test             ┘ (parallel)
  4. pod-cpu-hog  ┐
  5. pod-delete   ├─ fault steps (sequential)
  6. ...          ┘
  7. dynamic-pre-cleanup-wait  → configurable sleep (PRE_CLEANUP_WAIT_SECONDS)
  8. cleanup-chaos-resources ┐
     delete-loadtest         ┘ (parallel)
  9. uninstall-all           → helm uninstall agent + app
        │
        ▼
During steps 2–9: flash-agent runs scan loop every ~60s
  └─ Each scan → 2 LLM calls (tool_selection + llm_analysis)
     └─ Sidecar intercepts → injects metadata → forwards to LiteLLM
        └─ LiteLLM → Azure OpenAI GPT-4
           └─ LiteLLM Langfuse callback → writes observation to trace
```

---

## 2. ID Generation Chain

### 2.1 notifyID

**Source:** `handler.go:RunChaosWorkFlow`
```go
notifyID = uuid.New().String()   // e.g. "2fd29cb8-6c1b-4ab5-aa26-165396614fe6"
```

**Role: This is the single most important ID.** It ties together:
- Argo workflow label: `notify_id: <notifyID>`
- OTEL span trace ID (used to start/close the experiment span)
- Langfuse trace ID (UUID form → LLM generation trace)
- Langfuse trace ID (hex form, no hyphens → OTEL span trace)
- Flash-agent ConfigMap: `NOTIFY_ID=<notifyID>`
- Sidecar proxy: injects as `metadata.trace_id` on every LLM call

> **Two-trace problem:** Langfuse stores the UUID form and hex form as two separate trace records (same 16 bytes, different string). The UUID-form trace holds all LLM generations (what certification scores). The hex-form trace holds OTEL spans. A dual-upsert in `traceExperimentExecution` names both traces `agent-cert-exp-gt`.

### 2.2 experimentID

**Source:** MongoDB document ID created when the experiment was first saved via UI.
- Stable across all runs of the same experiment definition.
- Referenced in workflow label: `workflow_id`
- Stored in: `dbChaosExperiment.ChaosExperimentRequest.ExperimentID`

### 2.3 revisionID

**Source:** UUID generated on each "Save" of the experiment definition.
- Identifies which version of the manifest was used for this run.
- Stored in: `dbChaosExperiment.Revision[].RevisionID`

### 2.4 agentID (flash-agent)

**Source:** `agent_registry/service.go:RegisterAgent`
```go
agentID := uuid.New().String()
```

**Generation flow:**
1. User deploys experiment → Argo runs `install-agent` step
2. `install-agent` binary calls GraphQL `RegisterAgent` mutation
3. Server generates `agentID = uuid.New().String()`
4. Stores in MongoDB (`agent_registry` collection) keyed by namespace
5. Returns `agentID` to `install-agent`
6. `install-agent` calls `helm upgrade --set agentId=<uuid>` → written to ConfigMap
7. On re-run: server looks up existing agent by namespace (`GetAgentByNamespace`) and reuses the same UUID

**Key:** If an agent already exists in that namespace, registration returns the existing record (idempotent). So the same flash-agent pod across multiple experiment runs will always have the same `agentID`.

---

## 3. Workflow Manifest Patching (Server-Side)

Before the manifest is sent to Argo, the server applies patches in this order:

### 3.1 `applyInstallApplicationReadinessPatch`
Inserts a `normalize-install-application-readiness` step after `install-application`.  
Waits for all pods in `{{workflow.parameters.appNamespace}}` to be Running.  
Image: `litmuschaos/k8s:latest` (kubectl only, no hardcoded app name).

### 3.2 `applyPreCleanupWaitPatch`
Inserts `dynamic-pre-cleanup-wait` step before `cleanup-chaos-resources`.  
Duration: `PRE_CLEANUP_WAIT_SECONDS` env var (default `0`, currently set to `120`).  
Image: `busybox:1.36`.

### 3.3 `applyUninstallAllPatch` *(newly added)*
Appends `uninstall-all` as the very last step.  
Runs `helm uninstall` for both agent and app using Argo workflow parameters:
```sh
NAMESPACE="{{workflow.parameters.appNamespace}}"
AGENT_RELEASE="{{workflow.parameters.agentFolder}}"
APP_RELEASE="${NAMESPACE}"
helm uninstall "${AGENT_RELEASE}" -n "${NAMESPACE}" --ignore-not-found || true
helm uninstall "${APP_RELEASE}" -n "${NAMESPACE}" --ignore-not-found || true
```
Image: `INSTALL_AGENT_IMAGE` (ships with helm binary). Fully dynamic — no app/agent names hardcoded.

### 3.4 `injectExperimentContextArgs`
Appends `--set` args to the `install-agent` container template.  
Key values injected:

| `--set` key | Value |
|---|---|
| `agentId` | `{{workflow.parameters.agentId}}` (resolved from MongoDB at submit time) |
| `agent.config.NOTIFY_ID` | `{{workflow.labels.notify_id}}` (the trace ID) |
| `agent.config.EXPERIMENT_ID` | `{{workflow.labels.workflow_id}}` |
| `agent.config.EXPERIMENT_RUN_ID` | `{{workflow.uid}}` |
| `agent.config.WORKFLOW_NAME` | `{{workflow.labels.experiment_name}}` |
| `agent.config.GROUND_TRUTH_JSON` | base64-encoded ground truth blob |
| `sidecar.enabled` | `true` |
| `sidecar.upstream` | LiteLLM proxy URL |

> Argo resolves all `{{workflow.*}}` references at runtime before starting each step — so these values are correct even on the first scan.

---

## 4. Ground Truth (GT) Data Flow

```
chaoshub-faults/<hub>/<category>/<fault>/ground_truth.yaml   ← file on disk
        │
        ▼ loadFaultGroundTruths() in service.go
Reads all ground_truth.yaml files for all faults in this workflow
Merges into: { "pod-delete": {...}, "pod-cpu-hog": {...}, ... }
JSON → base64-encode
        │
        ▼ install-agent --set agent.config.GROUND_TRUTH_JSON=<base64>
ConfigMap: GROUND_TRUTH_JSON=<base64>
        │
        ▼ ConfigMap mounted at /etc/agent/metadata/ in sidecar container
agent-sidecar/proxy.py: _load_ground_truth_metadata()
        │
        ▼ Only injected for generation_name == "llm_analysis"
metadata.is_ground_truth_data = true
metadata.gt_block_type = "llm_analysis"
metadata.fault_names = ["pod-delete", "pod-cpu-hog", ...]
metadata.expected_output = "<ground truth answer text>"
        │
        ▼ LiteLLM → Langfuse callback
Langfuse observation: metadata contains full GT block
```

**Hub file location pattern (generic, not hardcoded):**
```
<hubBase>/*/<hub>/<category>/<fault>/ground_truth.yaml
```
Works for any hub name, any category, any fault — wildcards used.

---

## 5. LLM Call Flow (Per Scan)

Every ~65 seconds (60s interval + ~3-4s execution):

```
flash-agent (Python, sock-shop namespace)
        │
        │ POST http://localhost:4001/v1/chat/completions
        │ body: { messages: [...], metadata: { generation_name: "tool_selection" } }
        ▼
agent-sidecar proxy (port 4001, same pod)
        │ Reads /etc/agent/metadata/{NOTIFY_ID, AGENT_ID, GROUND_TRUTH_JSON, ...}
        │ Merges into metadata:
        │   trace_id = NOTIFY_ID
        │   session_id = EXPERIMENT_ID
        │   generation_name = "tool_selection"  (no GT injected here)
        ▼
LiteLLM proxy (litellm namespace, port 4000)
        │ Forwards to Azure OpenAI GPT-4
        │ Langfuse callback: writes generation observation to trace
        ▼
flash-agent receives tool decision (e.g. "kubernetes")
        │
        │ Calls MCP server (kubernetes-mcp-server) → gathers pod/event/ChaosResult data
        │
        │ POST http://localhost:4001/v1/chat/completions
        │ body: { messages: [...], metadata: { generation_name: "llm_analysis" } }
        ▼
agent-sidecar proxy
        │ Injects ALL metadata including GT block (because gen_name == "llm_analysis")
        ▼
LiteLLM proxy → Azure OpenAI GPT-4
        │ Langfuse callback: writes generation observation with GT data to trace
        ▼
flash-agent logs: "Scan complete | health=95 | issues=0"
```

**2 LLM calls per scan. ~65s between scans.**

---

## 6. OTEL + Langfuse Trace Architecture

### Two traces per experiment run

| Trace form | How created | Content |
|---|---|---|
| UUID (`2fd29cb8-6c1b-...`) | Langfuse REST API upsert from graphql server | LLM generations (scoring target) |
| Hex (`2fd29cb86c1b...`) | OTEL spans sent directly to Langfuse OTLP endpoint | Experiment/fault OTEL spans |

Both are named `agent-cert-exp-gt` via the dual-upsert in `traceExperimentExecution`. Certification scoring reads the **UUID-form trace** (the LLM generation trace).

### OTEL span hierarchy

```
experiment-run (root span, traceID = notifyID)
  └─ fault: F1 (blind alias for pod-cpu-hog)
  └─ fault: F2 (blind alias for pod-delete)
  └─ fault: F3 (...)
  └─ workflow-node spans (step-level)
```

BLIND_TRACES=yes: fault spans use `F1`, `F2`, ... aliases. Full fault names only appear in GT metadata inside `llm_analysis` observations.

### Langfuse tracer init

- **OTEL tracer:** `OTEL_EXPORTER_OTLP_ENDPOINT` + `OTEL_EXPORTER_OTLP_HEADERS` (set from `AGENT_OTEL_EXPORTER_OTLP_*` in .env via sync script)
- **Langfuse REST tracer:** `LANGFUSE_HOST` + `LANGFUSE_PUBLIC_KEY` + `LANGFUSE_SECRET_KEY`

Both are initialized at graphql server startup.

---

## 7. Complete Step-by-Step Sequence (numbered)

1. **UI → GraphQL** `RunChaosExperiment(experimentID, projectID)`
2. **Server** fetches latest revision manifest from MongoDB
3. **Server** generates `notifyID = uuid.New()` → this becomes the Langfuse trace ID
4. **Server** starts OTEL experiment span keyed on `notifyID`
5. **Server** calls Langfuse REST to upsert trace record (UUID + hex forms)
6. **Server** patches manifest: readiness, pre-cleanup-wait, uninstall-all, install-agent --set args
7. **Server** injects `notify_id` label and `agentId` parameter into manifest
8. **Server** sends patched manifest to subscriber (via websocket/store)
9. **Subscriber** submits manifest to Argo as a Workflow resource in `litmus-exp` namespace
10. **Argo** labels the workflow with `notify_id=<notifyID>`, `workflow_id=<experimentID>`
11. **Argo: install-application** → Helm deploys app (e.g. sock-shop) in `{{appNamespace}}`
12. **Argo: normalize-install-application-readiness** → waits for pods to be Running
13. **Argo: install-agent** → Helm deploys flash-agent + sidecar in `{{appNamespace}}`
    - `install-agent` binary registers agent → MongoDB gets `agentID`
    - ConfigMap written: `NOTIFY_ID`, `AGENT_ID`, `EXPERIMENT_ID`, `GROUND_TRUTH_JSON`
    - flash-agent pod starts → scan loop begins
14. **Flash-agent scan loop starts** (every ~65s):
    - LLM call 1: `tool_selection` → sidecar injects context metadata → LiteLLM → GPT-4 → Langfuse
    - MCP call: fetch kubernetes cluster state
    - LLM call 2: `llm_analysis` → sidecar injects context + GT metadata → LiteLLM → GPT-4 → Langfuse
15. **Argo: fault steps** (pod-cpu-hog, pod-delete, etc.) → ChaosEngines created → chaos injected
    - OTEL fault spans emitted per fault (F1, F2, ... in blind mode)
16. **Argo: dynamic-pre-cleanup-wait** → sleeps `PRE_CLEANUP_WAIT_SECONDS` (120s)
    - Flash-agent continues scanning, now sees faults in ChaosResults
17. **Argo: cleanup-chaos-resources** (parallel with delete-loadtest)
    - Deletes ChaosEngine CRDs for this run
18. **Argo: delete-loadtest** → Helm reverts/removes app workloads
19. **Argo: uninstall-all** → `helm uninstall <agentFolder> -n <appNamespace>` + `helm uninstall <appNamespace> -n <appNamespace>`
    - Flash-agent pod terminated → scan loop stops
20. **Workflow completes** (Argo phase: Succeeded/Failed)
21. **Server subscriber receives workflow completion event**
    - Calls `completeExperimentExecution` → closes OTEL experiment span
    - Updates MongoDB experiment run record with final phase/result
    - Scores experiment run (fault pass/fail counts → Langfuse scores)
22. **LiteLLM flushes remaining callbacks** to Langfuse (async, ~30-60s lag)
23. **Langfuse trace closes** (duration becomes non-null) → safe to fetch

---

## 8. Hardcoded / Dev-Only Values

These values have env var overrides but fall back to hardcoded defaults if not set in `.env`:

| Value | Location | Env Var Override | Required in Prod? |
|---|---|---|---|
| `"sk-litellm-local-dev"` | `service.go:1909` | `LITELLM_MASTER_KEY` | Yes — set in .env |
| `"http://litellm-proxy.litellm.svc.cluster.local:4000/v1"` | `service.go:1919` | `OPENAI_BASE_URL` | Yes — set via build-flash-agent.sh |
| `"http://kubernetes-mcp-server.litmus-exp.svc.cluster.local:8081/mcp"` | `service.go:1931` | `K8S_MCP_URL` | Yes — set via build-flash-agent.sh |
| `"http://prometheus-mcp-server.litmus-exp.svc.cluster.local:9090/mcp"` | `service.go:1936` | `PROM_MCP_URL` | Yes — set via build-flash-agent.sh |
| `"litmus-exp"` (chaos namespace) | `service.go:1941` | `CHAOS_NAMESPACE` | Yes — set via build-flash-agent.sh |
| `"agentcert/agent-sidecar:latest"` | `service.go:1957` | `AGENT_SIDECAR_IMAGE` | Yes — set in .env |
| `"http://litmusportal-server-service.litmus-chaos.svc.cluster.local:9004/query"` | `service.go:1969` | `SERVER_ADDR` | Yes — set in .env |
| `"litmus-project-1"` | `service.go:1973` | `LITMUS_PROJECT_ID` | Only if install-agent needs to self-register; rarely needed |
| `"agentcert/agentcert-install-agent:latest"` | `service.go:1493,1546,1644`, `handler.go:602` | `INSTALL_AGENT_IMAGE` | Yes — set in .env |
| `"busybox:1.36"` | `service.go:1398`, `handler.go:519` | None — always this image | Fine; busybox is stable |
| `"litmuschaos/k8s:latest"` | `service.go:635,1247` | None — always this image | Fine; standard Litmus image |
| `"/tmp/default"` | `service.go:1834` | `utils.Config.DefaultChaosHubPath` | Set by hub sync at startup |

### Values that are genuinely always dev-local (intentional)

| Value | Why it's OK |
|---|---|
| `busybox:1.36` | Pre-cleanup-wait only needs `sleep` — stable lightweight image |
| `litmuschaos/k8s:latest` | RBAC patch and readiness check — standard upstream Litmus image |
| Cluster-internal service DNS (e.g. `*.svc.cluster.local`) | These are correct for in-cluster; only wrong from WSL/outside |

### WSL-specific caveat

`*.svc.cluster.local` DNS is **not resolvable from WSL**. All scripts that run from WSL (fetch_langfuse_traces.py, build scripts) auto-remap or use `localhost:<port-forward>` instead.

---

## 9. Key Env Vars Reference

| Env Var | Used By | Purpose |
|---|---|---|
| `LITELLM_MASTER_KEY` | graphql server → install-agent | LiteLLM auth key |
| `FLASH_AGENT_IMAGE` | install-agent helm --set | Flash-agent container image |
| `AGENT_SIDECAR_IMAGE` | graphql server → install-agent | Sidecar image injected via --set |
| `INSTALL_AGENT_IMAGE` | graphql server | Image for install-agent + uninstall-all templates |
| `K8S_MCP_URL` | graphql server → install-agent | Kubernetes MCP server URL |
| `PROM_MCP_URL` | graphql server → install-agent | Prometheus MCP server URL |
| `CHAOS_NAMESPACE` | graphql server → install-agent | Namespace for chaos infra (litmus-exp) |
| `SERVER_ADDR` | graphql server → install-agent | GraphQL server URL for self-registration |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | graphql server | OTLP endpoint for OTEL spans |
| `OTEL_EXPORTER_OTLP_HEADERS` | graphql server | Auth header for OTLP |
| `LANGFUSE_HOST` | graphql server + fetch script | Langfuse REST API base URL |
| `LANGFUSE_PUBLIC_KEY` / `LANGFUSE_SECRET_KEY` | graphql server + fetch script | Langfuse auth |
| `BLIND_TRACES` | graphql server | `yes` → fault names masked as F1/F2/... in OTEL spans |
| `PRE_CLEANUP_WAIT_SECONDS` | graphql server | Sleep before cleanup (currently 120) |
| `LITMUS_PROJECT_ID` | graphql server → install-agent | Fallback project ID for self-registration |
