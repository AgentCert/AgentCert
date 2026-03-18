# Bank of Anthos Chaos Experiment - Troubleshooting & Fixes

**Experiment:** `argowf-chaos-bank-of-anthos-resiliency`  
**Date:** March 11, 2026  
**Infrastructure:** test-e2e-chaos (minikube on Docker Desktop, Windows)  
**Namespaces:** `litmus` (Argo workflows), `litmus-chaos` (ChaosCenter services), `bank` (Bank of Anthos target app)

---

## Overview

The Bank of Anthos chaos experiment is a multi-step Argo workflow that:

1. **install-application** — Deploys the Bank of Anthos microservices app into the `bank` namespace using `litmuschaos/litmus-app-deployer:latest`
2. **install-chaos-faults** — Installs the `pod-network-loss` ChaosExperiment CR
3. **pod-network-loss** — Runs the chaos experiment (100% packet loss on `balancereader` deployment) with HTTP probes against `frontend.bank.svc.cluster.local:80`
4. **cleanup-chaos-resources** — Deletes ChaosEngines
5. **delete-application** — Tears down the Bank of Anthos app

The experiment repeatedly failed at the `install-application` step with: `"Failed to install bank"`. This document details all the issues found and fixes applied.

---

## Issue 1: RBAC — Missing Permissions for ConfigMaps, Secrets, and Other Resources

### Symptom
The `litmus-app-deployer` container applied the `resilient-bank-of-anthos.yaml` manifest, but ConfigMaps and Secrets were **silently not created**. The deployer reported `"Namespace created successfully"` but then failed with `"Failed to install bank"` within ~13 seconds.

Pods showed `CreateContainerConfigError` because they referenced ConfigMaps/Secrets that didn't exist.

### Root Cause
The `argo-chaos` service account (in `litmus` namespace) is bound to the `infra-cluster-role` ClusterRole via the `argo-chaos-infra-cluster-role-binding` ClusterRoleBinding. This role was **missing permissions** to create:
- `configmaps`
- `secrets`

It also lacked full CRUD on `deployments`, `statefulsets`, `namespaces`, and `services` (only had `get`/`list` for some of these).

When the deployer ran `kubectl apply` internally, the namespace and some pod-level resources were created, but ConfigMaps and Secrets failed silently — causing all dependent pods to fail.

### Fix
Patched both `infra-cluster-role` and `litmus-chaos-infra-role` ClusterRoles to add the missing permissions:

```bash
# Created rbac-patch.json with the following content:
[
  {
    "op": "add",
    "path": "/rules/-",
    "value": {
      "apiGroups": [""],
      "resources": ["configmaps", "secrets"],
      "verbs": ["get", "list", "create", "update", "patch", "delete", "watch"]
    }
  },
  {
    "op": "add",
    "path": "/rules/-",
    "value": {
      "apiGroups": [""],
      "resources": ["namespaces"],
      "verbs": ["get", "list", "create", "update", "patch", "delete", "watch"]
    }
  },
  {
    "op": "add",
    "path": "/rules/-",
    "value": {
      "apiGroups": ["apps"],
      "resources": ["deployments", "statefulsets", "replicasets", "daemonsets"],
      "verbs": ["get", "list", "create", "update", "patch", "delete", "watch"]
    }
  },
  {
    "op": "add",
    "path": "/rules/-",
    "value": {
      "apiGroups": [""],
      "resources": ["services"],
      "verbs": ["get", "list", "create", "update", "patch", "delete", "watch"]
    }
  }
]

# Applied to both cluster roles:
kubectl patch clusterrole infra-cluster-role --type=json --patch-file=rbac-patch.json
kubectl patch clusterrole litmus-chaos-infra-role --type=json --patch-file=rbac-patch.json
```

### Verification
```bash
kubectl get clusterrole infra-cluster-role -o yaml | grep -E "configmaps|secrets"
# Should show both resources in the rules
```

---

## Issue 2: Slow Image Pulls Exceeding Deployer Timeout

### Symptom
Even after RBAC was fixed (in earlier manual attempts before identifying the RBAC root cause), the deployer timed out because Bank of Anthos container images took 15-20 minutes each to pull from `gcr.io/bank-of-anthos-ci/`.

The deployer has a **400-second (6.6 minute) timeout** (`-timeout=400`), but images like `contacts:v0.5.3` (1.23 GB), `frontend:v0.5.3` (1.17 GB), `userservice:v0.5.3` (1.23 GB), and `loadgenerator:v0.5.3` (1.15 GB) are very large.

### Fix
The images only need to be pulled once. After the first (failed) run pulls them, they are cached in minikube's container runtime. Subsequent runs use cached images and start pods within seconds.

### Verification
```bash
# Verify all 9 images are cached:
minikube ssh "crictl images | grep bank-of-anthos"
```

**Expected cached images:**
| Image | Tag | Size |
|-------|-----|------|
| `gcr.io/bank-of-anthos-ci/accounts-db` | v0.5.3 | 158MB |
| `gcr.io/bank-of-anthos-ci/balancereader` | v0.5.3 | 208MB |
| `gcr.io/bank-of-anthos-ci/contacts` | v0.5.3 | 1.23GB |
| `gcr.io/bank-of-anthos-ci/frontend` | v0.5.3 | 1.17GB |
| `gcr.io/bank-of-anthos-ci/ledger-db` | v0.5.3 | 160MB |
| `gcr.io/bank-of-anthos-ci/ledgerwriter` | v0.5.3 | 208MB |
| `gcr.io/bank-of-anthos-ci/loadgenerator` | v0.5.3 | 1.15GB |
| `gcr.io/bank-of-anthos-ci/transactionhistory` | v0.5.3 | 208MB |
| `gcr.io/bank-of-anthos-ci/userservice` | v0.5.3 | 1.23GB |

---

## Issue 3: Missing ConfigMaps in Bank Namespace (Manual Fix Before RBAC Discovery)

### Symptom
Before identifying the RBAC root cause, we manually investigated the `bank` namespace and found pods failing with `CreateContainerConfigError`. The deployer's manifest (`resilient-bank-of-anthos.yaml`) includes all required ConfigMaps and Secrets, but they weren't being created (due to RBAC — Issue 1).

### What We Discovered
The Bank of Anthos app requires the following ConfigMaps and Secrets:

| Resource | Type | Required By |
|----------|------|-------------|
| `jwt-key` | Secret | balancereader, contacts, frontend, ledgerwriter, transactionhistory, userservice |
| `environment-config` | ConfigMap | All pods |
| `service-api-config` | ConfigMap | frontend, ledgerwriter |
| `demo-data-config` | ConfigMap | accounts-db, ledger-db, frontend |
| `accounts-db-config` | ConfigMap | accounts-db, contacts, userservice |
| `ledger-db-config` | ConfigMap | ledger-db, balancereader, ledgerwriter, transactionhistory |

### Manual Creation Commands (for reference — no longer needed after RBAC fix)

```bash
# 1. JWT Key Secret (RSA keypair for inter-service auth)
openssl genrsa -out jwtRS256.key 4096
openssl rsa -in jwtRS256.key -outform PEM -pubout -out jwtRS256.key.pub
kubectl create secret generic jwt-key -n bank \
  --from-file=jwtRS256.key=jwtRS256.key \
  --from-file=jwtRS256.key.pub=jwtRS256.key.pub

# 2. Environment Config
kubectl create configmap environment-config -n bank \
  --from-literal=LOCAL_ROUTING_NUM="883745000" \
  --from-literal=PUB_KEY_PATH="/root/.ssh/publickey"

# 3. Service API Config
kubectl create configmap service-api-config -n bank \
  --from-literal=TRANSACTIONS_API_ADDR="ledgerwriter:8080" \
  --from-literal=BALANCES_API_ADDR="balancereader:8080" \
  --from-literal=HISTORY_API_ADDR="transactionhistory:8080" \
  --from-literal=CONTACTS_API_ADDR="contacts:8080" \
  --from-literal=USERSERVICE_API_ADDR="userservice:8080"

# 4. Demo Data Config
kubectl create configmap demo-data-config -n bank \
  --from-literal=USE_DEMO_DATA="True" \
  --from-literal=DEMO_LOGIN_USERNAME="testuser" \
  --from-literal=DEMO_LOGIN_PASSWORD="password"

# 5. Accounts DB Config
kubectl create configmap accounts-db-config -n bank \
  --from-literal=POSTGRES_DB=accounts-db \
  --from-literal=POSTGRES_USER=accounts-admin \
  --from-literal=POSTGRES_PASSWORD=accounts-pwd \
  --from-literal=ACCOUNTS_DB_URI=postgresql://accounts-admin:accounts-pwd@accounts-db:5432/accounts-db

# 6. Ledger DB Config
kubectl create configmap ledger-db-config -n bank \
  --from-literal=POSTGRES_DB=postgresdb \
  --from-literal=POSTGRES_USER=admin \
  --from-literal=POSTGRES_PASSWORD=password \
  --from-literal=SPRING_DATASOURCE_URL=jdbc:postgresql://ledger-db:5432/postgresdb \
  --from-literal=SPRING_DATASOURCE_USERNAME=admin \
  --from-literal=SPRING_DATASOURCE_PASSWORD=password
```

---

## Issue 4: PUB_KEY_PATH Mismatch

### Symptom
The `contacts` pod crashed with:
```
FileNotFoundError: [Errno 2] No such file or directory: '/tmp/.ssh/publickey'
```

### Root Cause
The `environment-config` ConfigMap was initially created with `PUB_KEY_PATH="/tmp/.ssh/publickey"`, but the `jwt-key` secret is mounted at `/root/.ssh/publickey` (as defined in the pod spec's volume mounts).

### Fix
The correct value (matching the deployer's manifest) is:
```
PUB_KEY_PATH="/root/.ssh/publickey"
```

> **Note:** This was only an issue during manual ConfigMap creation. The deployer's built-in manifest has the correct value.

---

## Issue 5: Cleaning Up Between Runs

### Symptom
Re-running the experiment while the `bank` namespace still existed from a previous failed run caused conflicts. The deployer logged `"Namespace already exist"` and then failed.

### Fix
Before each re-run, clean up:
```bash
# Delete the bank namespace (removes all resources)
kubectl delete ns bank

# Delete failed Argo workflows
kubectl delete workflow --all -n litmus
```

Then re-trigger the experiment from the ChaosCenter UI.

---

## Deployer Manifest Reference

The `litmuschaos/litmus-app-deployer:latest` image contains the Bank of Anthos manifest at:
```
/run/resilient-bank-of-anthos.yaml
```

It is invoked with these args:
```
-namespace=bank -typeName=resilient -operation=apply -timeout=400 -app=bank-of-anthos -scope=cluster
```

To inspect the manifest:
```bash
kubectl run temp-deployer -n litmus \
  --image=litmuschaos/litmus-app-deployer:latest \
  --restart=Never --command -- sh -c "cat /run/resilient-bank-of-anthos.yaml"

# Wait for completion, then:
kubectl logs temp-deployer -n litmus

# Clean up:
kubectl delete pod temp-deployer -n litmus
```

---

## Summary of All Fixes Applied

| # | Issue | Root Cause | Fix | Permanent? |
|---|-------|------------|-----|------------|
| 1 | RBAC permissions missing | `infra-cluster-role` lacked `configmaps` and `secrets` permissions | Patched ClusterRole with `rbac-patch.json` | Yes (until cluster reset) |
| 2 | Image pull timeout | Large images (1+ GB each) exceeded 400s deployer timeout | First-run pulls cache images; subsequent runs use cache | Yes (until minikube delete) |
| 3 | Missing ConfigMaps/Secrets | Consequence of Issue 1 | Resolved by fixing RBAC (Issue 1) | N/A |
| 4 | PUB_KEY_PATH mismatch | Manual ConfigMap had wrong path | Use `/root/.ssh/publickey` (matches pod volume mount) | N/A (only during manual fix) |
| 5 | Stale namespace conflicts | Previous failed run left resources | Delete `bank` ns and workflows before re-run | Per-run cleanup |

---

## Pre-Run Checklist

Before running the Bank of Anthos experiment, ensure:

- [ ] **RBAC patched**: `infra-cluster-role` and `litmus-chaos-infra-role` include `configmaps` and `secrets` permissions
- [ ] **Images cached**: All 9 `gcr.io/bank-of-anthos-ci/*:v0.5.3` images are present in minikube (`minikube ssh "crictl images | grep bank-of-anthos"`)
- [ ] **Bank namespace clean**: No leftover `bank` namespace from previous runs (`kubectl get ns bank` should return NotFound)
- [ ] **No stale workflows**: `kubectl get workflows -n litmus` should be empty
- [ ] **ChaosCenter services running**: GraphQL server and Auth server pods are healthy in `litmus-chaos` namespace
- [ ] **Port-forwards active**: GraphQL (8080) and Auth (3030) are forwarded
- [ ] **Argo workflow controller**: Running in `litmus` namespace with correct instance ID (`de38c4cc-15b9-49e0-8e8c-1917c1134735`)

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────┐
│                   Argo Workflow                      │
│              (litmus namespace)                      │
│                                                      │
│  Step 1: install-application                         │
│    └─ litmus-app-deployer:latest                     │
│       └─ Applies resilient-bank-of-anthos.yaml       │
│          └─ Creates: namespace, secrets, configmaps, │
│             deployments, statefulsets, services       │
│             in "bank" namespace                      │
│                                                      │
│  Step 2: install-chaos-faults                        │
│    └─ kubectl apply pod-network-loss experiment      │
│                                                      │
│  Step 3: pod-network-loss                            │
│    └─ ChaosEngine targets: deployment/balancereader  │
│       Probes: HTTP GET frontend.bank.svc:80          │
│       Duration: 90s, 100% packet loss                │
│                                                      │
│  Step 4: cleanup-chaos-resources + delete-application│
│    └─ Deletes ChaosEngines and bank namespace        │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│              Bank of Anthos (bank namespace)         │
│                                                      │
│  ┌──────────┐  ┌──────────┐  ┌───────────────────┐  │
│  │accounts- │  │ ledger-  │  │                   │  │
│  │  db      │  │   db     │  │    frontend       │  │
│  │(PG:5432) │  │(PG:5432) │  │   (HTTP:8080)     │  │
│  └────┬─────┘  └────┬─────┘  └───────┬───────────┘  │
│       │             │                │               │
│  ┌────┴─────┐  ┌────┴──────────┐  ┌──┴────────────┐ │
│  │contacts  │  │balancereader  │  │ ledgerwriter   │ │
│  │userservice│  │txn-history   │  │                │ │
│  └──────────┘  └───────────────┘  └────────────────┘ │
│                                                      │
│  Required Resources:                                 │
│  • Secret: jwt-key (RSA keypair)                     │
│  • CM: environment-config, service-api-config        │
│  • CM: accounts-db-config, ledger-db-config          │
│  • CM: demo-data-config                              │
└─────────────────────────────────────────────────────┘
```
