# Flash-Agent Data Leakage Analysis

**Date:** April 17, 2026  
**Analyst:** AI Architect Review  
**Trace Source:** Langfuse — `flash-agent-sock-shop` (scan-12, 2026-03-31T07:18–07:22 UTC)  
**Severity:** Critical

---

## 1. Executive Summary

The flash-agent, intended to be a **blind observer** that detects anomalies purely from system signals, is receiving and logging the **full fault-injection specification** (fault types, targets, parameters, probe configs) before it begins analysis. This defeats the fundamental "double-blind" principle that gives AgentCert resilience certifications their credibility.

---

## 2. Evidence from Trace Data

### 2.1 Layer A — Chaos Fault Step Spans (The "Answer Key")

The flash-agent logs step spans whose `input` field contains the complete injection spec:

| Span Name | Leaked Input Fields |
|---|---|
| `Step 7: pod-cpu-hog → deployment/carts` | `fault_type: pod-cpu-hog`, `target: deployment/carts`, `params: {CPU_CORES: 1, CHAOS_KILL_COMMAND: "kill md5sum"}`, probe: `HTTP GET front-end:80 == 200`, probe_mode: `Continuous` |
| `Step 8: pod-delete → deployment/catalogue` | `target: deployment/catalogue`, `CHAOS_INTERVAL: 10`, `FORCE: false`, probe: `HTTP GET front-end:80/catalogue == 200`, probe_mode: `Edge` |
| `Step 9: pod-network-loss → statefulset/user-db` | `target: statefulset/user-db`, `NETWORK_PACKET_LOSS_PERCENTAGE: 100`, `NETWORK_INTERFACE: eth0` |
| `Step 10: pod-memory-hog → deployment/orders` | `target: deployment/orders`, `MEMORY_CONSUMPTION: 500` |
| `Step 11: disk-fill → statefulset/catalogue-db` | `target: statefulset/catalogue-db`, `FILL_PERCENTAGE: 100` |

The agent knows **exactly** what faults are being injected, against which targets, with which parameters — before it even analyses the system.

### 2.2 Layer B — MCP Tool Request (Requesting Chaos Data)

The `mcp-kubernetes-request` span (trace IDs: `1c36e632ef3ae122`, `fde3f72a3e1629cf`) shows the agent explicitly requesting chaos-aware K8s resources:

```json
{
  "tools": [
    "pods_list_in_namespace",
    "events_list",
    "pods_top",
    "chaosengines",       // ← Leaks ChaosEngine names
    "chaosresults",       // ← Leaks fault verdicts
    "argo_workflows"      // ← Leaks workflow execution history
  ]
}
```

The MCP response includes all 10 ChaosEngine names (e.g., `pod-cpu-hog2n9jp`, `pod-deletenf2nz`) and 14 Argo workflow runs with their phases. The agent is **actively pulling** fault-injection artefacts.

### 2.3 Layer C — LLM Analysis Prompt (Feeding Chaos Data to LLM)

The `llm-analysis` GENERATION span (trace ID: `e389aab241bbe742`) shows the data sent to `gpt-4.1-mini` includes:

```
## CHAOSENGINES (10 total)
  Names: ["pod-cpu-hog2n9jp", "pod-cpu-hog6kbft", ..., "pod-network-losskjjnd"]

## ARGO WORKFLOWS (14 total)
  Latest experiment workflow: demochaos1-1774939315515
```

The analysis LLM can trivially infer "CPU hog is being injected" just from reading the engine names. The ChaosEngine naming convention (`pod-cpu-hog*`, `pod-delete*`, `pod-memory-hog*`, `pod-network-loss*`) directly encodes the fault type.

### 2.4 Layer D — Sidecar Metadata Injection

The `agent-sidecar/proxy.py` injects `experiment_id`, `experiment_run_id`, `workflow_name`, and `agent_id` into every LLM API call's metadata. While `agent_id` is needed for billing/routing, the experiment/workflow identifiers create a direct correlation between the observer agent and the experiment being run — visible in every Langfuse generation span.

---

## 3. Root Cause Analysis

| # | Root Cause | Source File / Component | Severity |
|---|---|---|---|
| **RC-1** | MCP tool set **hardcodes** `chaosengines`, `chaosresults`, `argo_workflows` in every scan request | Flash-agent scan orchestrator (server-side) | **Critical** |
| **RC-2** | Post-scan pipeline **parses Argo workflow YAML** to extract fault steps, targets, params, probes — and logs them as Langfuse span inputs | Flash-agent scan pipeline (server-side) | **Critical** |
| **RC-3** | LLM analysis prompt template **includes raw MCP response** (with ChaosEngine names) in "DATA TO ANALYSE" section | Flash-agent prompt template (server-side) | **High** |
| **RC-4** | Sidecar injects `experiment_id`, `experiment_run_id`, `workflow_name` into every LLM call metadata — correlating agent to specific experiment | `agent-sidecar/proxy.py` + Helm chart env vars | **Medium** |

---

## 4. Impact Assessment

| Aspect | Impact |
|---|---|
| **False confidence in results** | LLM marks all 10 faults "Pass" and system "resilient" — but had no independent evidence (`probe_success_percentage: null` for all, all `impact_observed: "insufficient data"`) |
| **Certification validity** | An AgentCert resilience certificate based on this trace is not trustworthy — the observing agent was not blind to the injection |
| **Prompt contamination** | ChaosEngine names like `pod-cpu-hog2n9jp` directly encode fault type in the name, making it impossible for the LLM to be unbiased |
| **Security exposure** | Full injection params (kill commands, network interface names, fill percentages) visible in Langfuse to anyone with trace access |

### Analogy

This is equivalent to giving a medical trial patient the exact drug/placebo assignment before measuring outcomes. The observation is contaminated, and no credible certification can be issued from this data.

---

## 5. Remediation Plan

### 5.1 Sidecar: Limit Metadata Injection (This PR)

**File:** `agent-sidecar/proxy.py`

The sidecar should only inject `agent_id` (needed for routing/billing). Remove `experiment_id`, `experiment_run_id`, and `workflow_name` from the metadata injection — the correlation should happen server-side in the GraphQL layer (which already has this data).

```python
# BEFORE: Injects all 4 keys
EXPERIMENT_CONTEXT = {}
for _key in ("EXPERIMENT_ID", "EXPERIMENT_RUN_ID", "WORKFLOW_NAME", "AGENT_ID"):
    ...

# AFTER: Only inject agent_id
_SAFE_KEYS = frozenset(("AGENT_ID",))
EXPERIMENT_CONTEXT = {}
for _key in ("AGENT_ID",):
    ...
```

**File:** `agent-chart/values.yaml` + `agent-chart/templates/deployment.yaml`

Remove `EXPERIMENT_ID`, `EXPERIMENT_RUN_ID`, `WORKFLOW_NAME` env vars from the sidecar container spec. Keep only `AGENT_ID`.

### 5.2 MCP Tool Set: Remove Chaos-Specific Tools (Separate PR — Server-Side)

The agent should request **only workload-health tools**:

```python
# BEFORE (leaks chaos internals)
tools = ["pods_list_in_namespace", "events_list", "pods_top",
         "chaosengines", "chaosresults", "argo_workflows"]

# AFTER (pure observability)
tools = ["pods_list_in_namespace", "events_list", "pods_top"]
```

### 5.3 LLM Analysis Prompt: Strip Chaos Artefacts (Separate PR — Server-Side)

The "DATA TO ANALYSE" section should exclude:
- `## CHAOSENGINES` section entirely
- `## ARGO WORKFLOWS` section (or redact workflow names that encode experiment identity)

### 5.4 Step Span Inputs: Redact Fault Specs (Separate PR — Server-Side)

The step spans (`Step 7: pod-cpu-hog → deployment/carts`) should NOT include `fault_type`, `target`, `params`, `probe`, `probe_mode`, `duration_config` in the `input` field. If needed for debugging, use a **separate admin-only Langfuse project** with restricted access.

### 5.5 Validate Blind Detection Capability (Post-Fix)

After remediation:
1. Inject a fault the agent has never seen (e.g., `node-drain`)
2. Confirm the agent detects anomalies (pod evictions, restart spikes) without knowing the fault type
3. Compare the agent's analysis accuracy with vs. without chaos metadata

---

## 6. Target Architecture

```
┌─────────────┐     ┌──────────────┐     ┌──────────────────┐
│ AgentCert    │────→│ Argo Workflow │────→│ ChaosEngine      │
│ GraphQL      │     │ (injects     │     │ (executes fault)  │
│ Server       │     │  faults)     │     │                   │
└──────┬───────┘     └──────────────┘     └──────────────────┘
       │                                           │
       │ ┌─────────── INFORMATION BARRIER ─────────┤
       │ │                                         │
       ▼ ▼                                         ▼
┌──────────────┐                          ┌──────────────┐
│ Flash-Agent  │──── K8s API ────────────→│ Kubernetes   │
│ (BLIND)      │     (pods, events,       │ Cluster      │
│              │      metrics ONLY)       │              │
└──────┬───────┘                          └──────────────┘
       │
       ▼
┌──────────────┐
│ Langfuse     │  ← Only: health scores, detected anomalies
│ (clean       │     NO: fault names, targets, params
│  traces)     │
└──────────────┘
```

**Principle:** The agent that certifies resilience must not know what is being tested — same as a double-blind clinical trial.

---

## 7. Files Changed in This PR

| File | Change | Addresses |
|---|---|---|
| `agent-sidecar/proxy.py` | Restrict injection to `AGENT_ID` only; add env-driven allowlist | RC-4 |
| `agent-chart/values.yaml` | Remove experiment env vars from sidecar config | RC-4 |
| `agent-chart/templates/deployment.yaml` | Remove experiment env vars from sidecar container spec | RC-4 |
| `docs/Flash-agent-data-leakage-analysis.md` | This analysis document | — |

---

## 8. Out of Scope (Requires Server-Side Changes)

- RC-1: MCP tool set reduction (flash-agent Python code, deployed separately)
- RC-2: Step span input redaction (flash-agent scan pipeline)
- RC-3: LLM prompt template cleanup (flash-agent prompt builder)

These require changes to the flash-agent codebase deployed on the Kubernetes cluster and should be tracked as separate issues.
