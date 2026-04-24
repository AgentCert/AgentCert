# AgentCert Trace Workflow, GT Metadata, and Changelog

## Purpose
This document explains how the current workflow operates end-to-end, why `fault.alias: F1` appears in traces, where Ground Truth (GT) metadata is stored, and what changed during the recent fixes.

## Quick Answers
- Seeing `fault.alias: F1` is expected when `BLIND_TRACES=yes`.
- GT is intentionally not part of the LLM prompt input.
- GT is injected by sidecar into metadata for `llm_analysis` calls and usually appears in Langfuse exports under `metadata.requester_metadata`.
- If you search for the literal text `Ground Truth`, you may not find it. Search for `fault_names`, `expected_output`, or `gt_metadata_present`.

---

## 1. End-to-End Flow

### 1.1 Control Plane and Experiment Setup
1. GraphQL server prepares experiment context (IDs, workflow labels, GT payload from hub).
2. Flash-agent metadata ConfigMap is created/updated with keys such as:
   - `NOTIFY_ID`
   - `EXPERIMENT_ID`
   - `EXPERIMENT_RUN_ID`
   - `WORKFLOW_NAME`
   - `GROUND_TRUTH_JSON` (base64-encoded JSON)
3. Flash-agent pod runs with two containers:
   - `agent` (flash-agent)
   - `agent-sidecar` (metadata injection proxy)
4. Both containers mount `/etc/agent/metadata` from the ConfigMap.

### 1.2 Flash-Agent Runtime Loop
1. Flash-agent collects MCP data (Kubernetes/Prometheus).
2. It performs tool selection (`generation_name=tool_selection`).
3. It performs analysis (`generation_name=llm_analysis`) by sending request to sidecar (`http://localhost:4001/v1`).

### 1.3 Sidecar Metadata Injection
For each LLM request, sidecar:
1. Reads live context from mounted metadata files.
2. Merges context into request `metadata`.
3. Ensures canonical trace fields (`trace_id`, `trace_name`, `session_id`, `user_id`, agent fields).
4. For analysis generations only (`llm-analysis` or `llm_analysis`):
   - Decodes `GROUND_TRUTH_JSON`
   - Injects:
     - `fault_names`
     - `expected_output`
   - Injects GT identifiers:
     - `gt_metadata_present=true`
     - `gt_metadata_source=GROUND_TRUTH_JSON`
     - `gt_metadata_version=v1`

### 1.4 LiteLLM and Langfuse
1. Sidecar forwards request to LiteLLM proxy.
2. LiteLLM sends trace events to Langfuse using callbacks.
3. Langfuse export often nests user metadata into `requester_metadata`.

---

## 2. Why `fault.alias: F1` Appears

`BLIND_TRACES=yes` is enabled in `.env`.

Behavior with blind traces:
- Fault-identifying fields in OTEL/fault spans are anonymized.
- Real fault names are replaced by aliases such as `F1`, `F2`, etc.

This is expected and desirable for blind-evaluation behavior.

Important distinction:
- Blind aliasing affects fault trace attributes.
- GT metadata remains available for offline evaluation in sidecar-injected metadata (not in prompt input).

---

## 3. What Appears in Traces

### 3.1 Input Prompt (`input.messages`)
You should see the analysis instruction prompt and data summary.

You should not see GT injected into prompt content.

### 3.2 Metadata Location
Depending on export shape, GT is visible in one of:
- `metadata.fault_names` / `metadata.expected_output` (less common in export)
- `metadata.requester_metadata.fault_names` / `metadata.requester_metadata.expected_output` (common)

With the new marker fields, also check for:
- `metadata.requester_metadata.gt_metadata_present`
- `metadata.requester_metadata.gt_metadata_source`
- `metadata.requester_metadata.gt_metadata_version`

### 3.3 Why Some Trace Files Show No `llm_analysis`
Some exported files are trace-level summaries and do not include generation rows.
In those files, `llm_analysis` and GT metadata checks return zero by design.

---

## 4. Practical Verification Checklist

Use these checks during an active run:

1. GT exists in ConfigMap:
   - `kubectl -n sock-shop get cm flash-agent-metadata -o jsonpath='{.data.GROUND_TRUTH_JSON}' | wc -c`
   - Expect non-zero size.

2. GT file is mounted in sidecar:
   - `kubectl -n sock-shop exec <flash-agent-pod> -c agent-sidecar -- sh -lc 'wc -c /etc/agent/metadata/GROUND_TRUTH_JSON'`
   - Expect same non-zero size.

3. Sidecar contains GT injection code:
   - Verify `_load_ground_truth_metadata` exists and `llm_analysis` gate exists in `/app/proxy.py`.

4. Flash-agent is making analysis calls:
   - Check logs for `requesting analysis` and `analysis complete`.

5. In Langfuse generation metadata:
   - Check `requester_metadata.generation_name == llm_analysis`.
   - Check `requester_metadata.fault_names`.
   - Check `requester_metadata.expected_output`.
   - Check `requester_metadata.gt_metadata_present == true`.

---

## 5. Changelog of Fixes

### 5.1 Prompt and Behavior Fixes
- Replaced old experiment/fault-injection prompt with a generic Advanced ITOps prompt.
- Removed hardcoded AKS wording and made prompt infrastructure-agnostic.
- Updated output schema in prompt to generic environment/issues/health model.

### 5.2 Trace/Span Fixes
- Implemented custom LiteLLM callback emitting legacy span names:
  - `Received Proxy Server Request`
  - `auth`
  - `router`
  - `proxy_pre_call`
- Registered callback in LiteLLM config.
- Mounted callback file via deployment subPath.

### 5.3 GT Metadata Fixes
- Sidecar injects GT only for `llm_analysis` calls.
- GT fields injected: `fault_names`, `expected_output`.
- Added GT identifier fields:
  - `gt_metadata_present`
  - `gt_metadata_source`
  - `gt_metadata_version`

### 5.4 Build/Deploy Pipeline Fixes
- Fixed flash-agent image repo drift issues (`agentcert/flash-agent` vs `agentcert/agentcert-flash-agent`).
- Updated build scripts to accept and propagate `--env-file` consistently.
- Added live workload sync in flash-agent build script to set deployment/cronjob image directly.
- Updated agent-charts defaults to use canonical flash-agent image repo.

### 5.5 Argo Workflow Patches (service.go / handler.go)
- Added `applyUninstallAllPatch`: appends a final `uninstall-all` Argo step that runs `helm uninstall` for both the agent release and the app release (sock-shop) after all chaos steps complete. Also deletes ChaosEngine and ChaosResult resources.
- Added `applyDynamicPreCleanupWaitPatch`: injects a configurable sleep step (`PRE_CLEANUP_WAIT_SECONDS`, default 120s) before `cleanup-chaos-resources` to give the flash-agent time to complete its final analysis cycle.
- Added `podGC: OnWorkflowCompletion`: Argo automatically garbage-collects completed executor pods so they don't accumulate in `litmus-exp`.

### 5.6 Per-Experiment Monitoring (Prometheus + Grafana)
- Removed `monitoring.enabled=false` override from `applyInstallApplicationTemplateOverrides` in `service.go`.
- `monitoring.enabled=true` is now the default from `app-charts/charts/sock-shop/values.yaml`.
- Result: every experiment run automatically deploys Prometheus + Grafana in the `monitoring` namespace as part of the sock-shop Helm release.
- Prometheus scrapes sock-shop pods. Grafana reads from Prometheus. Both are torn down by `uninstall-all` when the workflow completes.
- `prometheus-mcp-server` in `litmus-exp` (permanent bridge) points to `http://prometheus.monitoring.svc.cluster.local:9090` — it proxies flash-agent metric queries to the per-experiment Prometheus.
- `mcpTools.kubernetesMcpServer.enabled=false` and `mcpTools.prometheusMcpServer.enabled=false` remain forced on the install-application step because those MCP tools are provided permanently by `litmus-exp`, not per-experiment.

### 5.7 enable-chaos-infra.sh Changes
- Script now applies `kubectl apply` for `prometheus-mcp-server` URL patch after manifest deploy.
- Removed permanent Prometheus + Grafana deployment from the script (they are now per-experiment).
- Added cleanup loop to remove any previously-deployed permanent Prometheus/Grafana from `litmus-exp` on re-run (migration safety).

---

## 6. Current Expected State

If everything is healthy now, you should observe:
- Prompt text in `llm_analysis` input is the new Advanced ITOps version.
- Fault traces use aliases (`F1`, `F2`, ...) because blind mode is on.
- GT exists in metadata for `llm_analysis` generations (usually under `requester_metadata`).
- GT markers (`gt_metadata_present/source/version`) are present after sidecar update rollout.

### 6.1 Cluster State During an Active Experiment
```
litmus-exp    chaos-operator              1/1  Running   permanent
litmus-exp    chaos-exporter              1/1  Running   permanent
litmus-exp    workflow-controller         1/1  Running   permanent
litmus-exp    subscriber                  1/1  Running   permanent
litmus-exp    event-tracker               1/1  Running   permanent
litmus-exp    kubernetes-mcp-server       1/1  Running   permanent
litmus-exp    prometheus-mcp-server       1/1  Running   permanent
sock-shop     flash-agent-<hash>          2/2  Running   per-experiment (agent + sidecar)
monitoring    prometheus-deployment       1/1  Running   per-experiment
monitoring    grafana                     1/1  Running   per-experiment
```

### 6.2 Cluster State After Experiment Completes
```
litmus-exp    <all permanent pods>        1/1  Running   unchanged
sock-shop     <namespace deleted>                        uninstall-all removed it
monitoring    <namespace deleted>                        uninstall-all removed it (owned by sock-shop Helm)
```

---

## 7. Known Caveats

1. Export format variability:
   - Different export endpoints/files may flatten or nest metadata differently.

2. Summary exports:
   - Some files include no generation records; GT checks appear empty there.

3. Search behavior:
   - Searching `Ground Truth` literal text may miss GT metadata. Prefer key-based search.

---

## 8. How to Find GT Data in Trace JSON

### 8.1 Which block contains GT?

Only `GENERATION` blocks with `name: llm_analysis (...)` contain GT. `tool_selection` blocks do NOT.

**Quick visual check:**
```
name: "llm_analysis (9b348e35)"   ← HAS GT
name: "tool_selection (time-15-)" ← NO GT
```

### 8.2 GT identifier fields (after latest sidecar update)

In the block's `metadata.requester_metadata`:

| Field | Value | Meaning |
|-------|-------|---------|
| `is_ground_truth_data` | `true` | **Top-level flag — clearest identifier** |
| `gt_block_type` | `"llm_analysis"` | Block type that carries GT |
| `gt_metadata_present` | `true` | Backward-compatible marker |
| `gt_metadata_source` | `"GROUND_TRUTH_JSON"` | Source of GT data |
| `gt_metadata_version` | `"v1"` | Version tag |
| `fault_names` | `["disk-fill", ...]` | Which faults were injected |
| `expected_output` | `"{...}"` | **This is the actual GT data** |

### 8.3 What is `expected_output`?

`expected_output` is the GT — a JSON string with per-fault entries:

```json
"expected_output": {
  "disk-fill": {
    "fault_description_goal_remediation": {
      "goal": "Fill ephemeral storage to test resilience...",
      "remediation": "Identify evicted pods, clean up files...",
      "symptoms": ["Pod evicted", "DiskPressure on node", ...]
    },
    "ideal_course_of_action": [
      {"step": 1, "action": "List cluster events", ...},
      {"step": 2, "action": "List pods in namespace", ...}
    ],
    "ideal_tool_usage_trajectory": [
      {"step": 1, "tool": "Events: List", "command": "..."},
      ...
    ]
  },
  "pod-cpu-hog": { ... },
  "pod-delete":  { ... }
}
```

This is compared against the agent's actual `output` field for evaluation/scoring.

### 8.4 jq filter to find GT blocks

```bash
# Simplest — top-level flag (new, clearest)
cat trace.json | jq '.[] | select(.metadata.requester_metadata.is_ground_truth_data == true)'

# Check just names and fault list
cat trace.json | jq '.[] | select(.metadata.requester_metadata.is_ground_truth_data == true) | {name, fault_names: .metadata.requester_metadata.fault_names}'

# Count GT blocks
cat trace.json | jq '[.[] | select(.metadata.requester_metadata.is_ground_truth_data == true)] | length'
```

### 8.5 Suggested Key Searches in Trace JSON

- `is_ground_truth_data` — top-level GT flag (new)
- `gt_block_type` — block type marker (new)
- `llm_analysis` — generation name
- `fault_names` — injected fault list
- `expected_output` — the GT payload
- `gt_metadata_present` — backward-compat marker
- `fault.alias` — blind trace alias (F1, F2, ...)

---

## 9. Workflow Cleanup Behavior

### 9.1 What each cleanup step does

| Step | What it cleans | What it leaves |
|------|---------------|----------------|
| `cleanup-chaos-resources` | Deletes ChaosEngine CRDs for the run (`kubectl delete chaosengine -l workflow_run_id=...`) | Everything else |
| `delete-loadtest` | Reverts/removes the target application (e.g. sock-shop) from app namespace | Infrastructure pods |

**`cleanup-chaos-resources` log output (example):**
```
chaosengine.litmuschaos.io 'disk-filljb9dh' deleted
chaosengine.litmuschaos.io 'pod-cpu-hogtvmj9' deleted
chaosengine.litmuschaos.io 'pod-delete8xzbc' deleted
chaosengine.litmuschaos.io 'pod-memory-hogqcwcf' deleted
chaosengine.litmuschaos.io 'pod-network-lossp5krb' deleted
```

### 9.2 What is NOT cleaned up automatically

**Argo workflow step pods (Completed state):**
```
litmus-exp   agent-cert-exp-gt-<workflow-id>-XXXXXXXXXX   0/2   Completed
```
These are left by Argo by design. Options to clean:
```bash
# Manual cleanup
kubectl delete pods -n litmus-exp --field-selector=status.phase==Succeeded

# Or set TTL on the Argo workflow (in workflow spec):
# ttlStrategy:
#   secondsAfterCompletion: 300
```

**Permanent litmus-exp pods (should NOT be deleted — part of installation):**
- `chaos-exporter`
- `chaos-operator-ce`
- `event-tracker`
- `kubernetes-mcp-server`
- `prometheus-mcp-server`
- `subscriber`
- `workflow-controller`

### 9.3 App namespace (e.g. `sock-shop`) behavior

The target app namespace (sock-shop today, could be any app tomorrow) is handled by `delete-loadtest` using `agentcert/agentcert-install-app:latest` with `-folder=<app> -namespace=<app>` in revert mode. This removes the app workloads. The namespace itself may persist.

**The app namespace is parameterized** (`{{workflow.parameters.appNamespace}}`), so the same workflow works for any app — not just sock-shop.
