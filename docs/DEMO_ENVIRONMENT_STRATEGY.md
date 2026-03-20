# Demo Environment Strategy

## Overview

The `demo-branch` provides an isolated, always-ready environment for customer demos on the Azure VM. Daily code changes on `main` do not impact the demo — redeploy from `demo-branch` whenever a customer demo is planned.

## Why Demo Branch

- **Isolation** from daily `main` churn
- **Deterministic** — same code deploys every time
- **Stubbing** — can mock/stub things without polluting `main`
- **Cherry-pick** critical fixes from `main` when needed
- **Simple** for anyone on the team to redeploy

## Branch Strategy

```
main ─────────────────────────► (daily development)
  │
  └── demo-branch ────────────► (frozen for demos, cherry-pick only)
```

**Rules for demo-branch:**
- Only cherry-pick from `main` — never merge `main` directly
- Tag each demo-ready state: `demo-v1`, `demo-v2`, etc.
- Keep a `demo-values.yaml` override file for stubbed/demo-specific config

## Namespace Isolation

Deploy the demo stack in dedicated namespaces so dev and demo can coexist:

```
Azure VM / AKS Cluster
├── litmus          (dev — deployed from main)
├── sock-shop       (dev workload)
├── litmus-demo     (demo — deployed from demo-branch)
├── sock-shop-demo  (demo workload)
└── litellm         (shared — both use same LiteLLM proxy)
```

## What to Stub for Demo

| Component | Stub Approach |
|-----------|---------------|
| Flash Agent scan interval | Reduce to 60s for faster demo cycles |
| LiteLLM | Point to a reliable Azure OpenAI deployment (not dev quota) |
| Langfuse | Pre-populate with a few good traces so the dashboard isn't empty |
| Chaos experiments | Pre-tested experiments that reliably complete (pod-delete is safest) |
| Sock-shop app | Pin image tags — don't use `:latest` |

## Demo Deploy Script

One command to bring up the full demo environment:

```bash
#!/bin/bash
# deploy-demo.sh — one command to bring up the full demo environment
DEMO_NS="sock-shop-demo"
LITMUS_NS="litmus-demo"

# 1. Deploy sock-shop in demo namespace
kubectl apply -f app-charts/ -n $DEMO_NS

# 2. Deploy ChaosCenter pointing to demo namespace
# (use build-and-deploy.sh with demo overrides)
bash local-custom/scripts/build-and-deploy.sh --namespace $LITMUS_NS --env demo

# 3. Deploy flash-agent targeting demo namespace
helm upgrade --install flash-agent-demo agent-charts/charts/flash-agent \
  -n $DEMO_NS \
  -f demo-values.yaml

# 4. Verify everything is up
kubectl get pods -n $DEMO_NS
kubectl get pods -n $LITMUS_NS
```

## Demo Reset Script

For between demos — reset to clean state:

```bash
#!/bin/bash
# reset-demo.sh — clean slate for next demo
kubectl delete chaosengines --all -n litmus-demo
kubectl rollout restart deployment -n sock-shop-demo
# Optionally restart flash-agent to get fresh traces
kubectl rollout restart deployment/flash-agent -n sock-shop-demo
```

## Checklist Before Every Demo

1. **Pin all image tags** on `demo-branch` — no `:latest` anywhere
2. **Pre-bake Langfuse data** — export a few good traces and have a restore script
3. **Tag demo-ready states** — `git tag demo-v1` so you can roll back to a known-good version
4. **Separate `.env.demo`** file with demo-specific credentials (in case you want a dedicated Azure OpenAI deployment with guaranteed quota)
5. **Health-check script** — run before every demo to verify all pods are up, LiteLLM responds, Langfuse has data

## Pre-Demo Health Check

```bash
#!/bin/bash
# health-check-demo.sh — run before every demo
echo "=== Pod Status ==="
kubectl get pods -n sock-shop-demo
kubectl get pods -n litmus-demo

echo "=== LiteLLM Health ==="
kubectl exec -n sock-shop-demo deploy/flash-agent -- \
  curl -s http://litellm-proxy.litellm.svc.cluster.local:4000/health

echo "=== Langfuse Health ==="
kubectl exec -n sock-shop-demo deploy/flash-agent -- \
  curl -s http://langfuse-web.langfuse.svc.cluster.local:3000/api/public/health

echo "=== Flash Agent Logs (last 10 lines) ==="
kubectl logs -n sock-shop-demo deploy/flash-agent --tail=10
```
