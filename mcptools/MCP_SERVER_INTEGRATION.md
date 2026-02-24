# MCP Server Integration in Litmus Chaos Infrastructure

## Overview

The `test-mcp-litmus-chaos-enable.yml` manifest deploys the full **Litmus Chaos Infrastructure** along with a **Kubernetes MCP (Model Context Protocol) Server**. The MCP server enables AI/LLM agents (such as VS Code Copilot, MCP Inspector, or custom AI tools) to query cluster state, read pod logs, and inspect chaos experiment results via a standardized protocol.

---

## What Was Added

The following **5 Kubernetes resources** were appended to the end of `test-mcp-litmus-chaos-enable.yml`, after the existing Litmus chaos components:

### 1. ServiceAccount — `mcp-server`

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: mcp-server
  namespace: litmus
```

**Purpose:** Creates a dedicated identity for the MCP server pod. This keeps its permissions isolated from the Litmus `litmus` ServiceAccount.

---

### 2. ClusterRole — `mcp-server-role`

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: mcp-server-role
rules:
  # Standard K8s resources (read-only)
  - apiGroups: [""]
    resources: ["pods", "pods/log", "services", "configmaps", ...]
    verbs: ["get", "list", "watch"]
  # Litmus CRDs (full access for chaos operations)
  - apiGroups: ["litmuschaos.io"]
    resources: ["chaosengines", "chaosexperiments", "chaosresults"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  # Argo Workflow CRDs
  - apiGroups: ["argoproj.io"]
    resources: ["workflows", "cronworkflows", ...]
    verbs: ["get", "list", "watch"]
  # ... plus EventTracker, CRD discovery, networking, OpenShift
```

**Purpose:** Grants the MCP server the permissions it needs to:
- **Read** standard Kubernetes resources (pods, deployments, services, logs, etc.)
- **Read + Write** Litmus Chaos CRDs (so AI agents can trigger/manage experiments)
- **Read** Argo Workflows (Litmus uses these to orchestrate chaos)
- **Discover** CRDs via `apiextensions.k8s.io`

> **Security note:** Standard K8s resources are read-only. Litmus CRDs have full CRUD to allow AI-driven experiment management. Adjust verbs if you want stricter access.

---

### 3. ClusterRoleBinding — `mcp-server-role-binding`

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: mcp-server-role-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: mcp-server-role
subjects:
  - kind: ServiceAccount
    name: mcp-server
    namespace: litmus
```

**Purpose:** Binds `mcp-server-role` to the `mcp-server` ServiceAccount in the `litmus` namespace, giving the pod cluster-wide access as defined by the ClusterRole.

---

### 4. Deployment — `kubernetes-mcp-server`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kubernetes-mcp-server
  namespace: litmus
spec:
  replicas: 1
  template:
    spec:
      serviceAccountName: mcp-server
      containers:
        - name: mcp-server
          image: quay.io/containers/kubernetes_mcp_server:latest
          args: ["--port", "8080", "--stateless"]
          ports:
            - containerPort: 8080
```

**Purpose:** Runs the MCP server container with:
- **Image:** `quay.io/containers/kubernetes_mcp_server:latest`
- **Port:** 8080 (HTTP)
- **Mode:** `--stateless` (no session persistence needed)
- **Health checks:** Liveness and readiness probes on `/healthz`
- **Security:** Runs as non-root user (UID 65532), no privilege escalation
- **Resources:** 128Mi–256Mi memory, 100m–500m CPU

---

### 5. Service — `kubernetes-mcp-server`

```yaml
apiVersion: v1
kind: Service
metadata:
  name: kubernetes-mcp-server
  namespace: litmus
spec:
  type: ClusterIP
  selector:
    app: kubernetes-mcp-server
  ports:
    - port: 8080
      targetPort: http
```

**Purpose:** Exposes the MCP server within the cluster on port 8080. Other pods or port-forward commands can reach it at `kubernetes-mcp-server.litmus.svc.cluster.local:8080`.

---

## File Structure

The `test-mcp-litmus-chaos-enable.yml` is organized in this order:

| Section | Lines (approx) | Resources |
|---------|----------------|-----------|
| **Namespace & Identity** | 1–13 | Namespace `litmus`, ServiceAccount `litmus` |
| **Litmus CRDs** | 14–2980 | ChaosEngine, ChaosExperiment, ChaosResult, EventTrackerPolicy |
| **Argo Workflow Engine** | 2980–3900 | Workflow Controller (Deployment, ConfigMap, Service), Argo CRDs, RBAC |
| **Subscriber & Event Tracker** | 3900–4020 | subscriber-config, subscriber-secret, Subscriber Deployment, Event Tracker Deployment |
| **Chaos Operator & Exporter** | 4020–4289 | litmus-admin SA, ClusterRole, chaos-operator-ce Deployment, chaos-exporter Deployment + Service |
| **🆕 MCP Server** | 4290+ | mcp-server SA, ClusterRole, ClusterRoleBinding, Deployment, Service |

---

## How to Deploy

Deploy everything (Litmus infrastructure + MCP server) in one command:

```bash
kubectl apply -f test-mcp-litmus-chaos-enable.yml
```

This creates the `litmus` namespace and deploys all components into it.

## How to Access the MCP Server

### From within the cluster
```
http://kubernetes-mcp-server.litmus.svc.cluster.local:8080
```

### From your local machine (port-forward)
```bash
kubectl port-forward -n litmus svc/kubernetes-mcp-server 8089:8080
```
Then connect at `http://localhost:8089`.

### Using MCP Inspector
```bash
npx @modelcontextprotocol/inspector
```
In the Inspector UI, set the server URL to `http://localhost:8089` and transport type to **Streamable HTTP**.

---

## Customization Guide

### Change the MCP server image
Find the Deployment named `kubernetes-mcp-server` and update the `image` field:
```yaml
image: quay.io/containers/kubernetes_mcp_server:latest  # Change tag or registry
```

### Restrict MCP permissions
Edit the `mcp-server-role` ClusterRole. For example, to make Litmus CRDs read-only:
```yaml
  - apiGroups: ["litmuschaos.io"]
    resources: ["chaosengines", "chaosexperiments", "chaosresults"]
    verbs: ["get", "list", "watch"]  # Removed create, update, patch, delete
```

### Change the MCP server port
Update these three places:
1. Deployment `args`: `"--port", "9090"`
2. Deployment `containerPort`: `9090`
3. Service `ports.port`: `9090`

### Add NodeSelector or tolerations
Add under the Deployment's `spec.template.spec`:
```yaml
      nodeSelector:
        kubernetes.io/os: linux
      tolerations:
        - key: "dedicated"
          operator: "Equal"
          value: "chaos"
          effect: "NoSchedule"
```

---

## Cleanup

To remove everything:
```bash
kubectl delete -f test-mcp-litmus-chaos-enable.yml
```

To remove only the MCP server (keep Litmus):
```bash
kubectl delete svc kubernetes-mcp-server -n litmus
kubectl delete deployment kubernetes-mcp-server -n litmus
kubectl delete clusterrolebinding mcp-server-role-binding
kubectl delete clusterrole mcp-server-role
kubectl delete serviceaccount mcp-server -n litmus
```
