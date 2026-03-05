# MCP Tools Integration in Chaos Infrastructure

## Overview

This document describes the changes made to automatically deploy **MCP (Model Context Protocol) tool servers** alongside Litmus Chaos infrastructure components. When a user creates (registers) a new chaos infrastructure through the LitmusChaos control plane, the generated Kubernetes YAML manifest now includes two MCP tool deployments:

| MCP Tool | Image | Service Port | Purpose |
|----------|-------|-------------|---------|
| **kubernetes-mcp-server** | `quay.io/containers/kubernetes_mcp_server:latest` | `8081` | Provides Kubernetes cluster read access (pods, deployments, CRDs, logs) via the MCP protocol |
| **prometheus-mcp-server** | `ghcr.io/pab1it0/prometheus-mcp-server:latest` | `9090` | Exposes Prometheus metrics via the MCP protocol for observability |

These tools enable AI agents (e.g., GitHub Copilot, Claude) to query the chaos infrastructure cluster state and metrics through the MCP protocol, supporting automated chaos experiment analysis and certification workflows.

---

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│  LitmusChaos Control Plane (litmus-chaos namespace)      │
│  ┌─────────────────────┐  ┌──────────────────────────┐   │
│  │ GraphQL Server       │  │ Auth Server              │   │
│  │ (litmusportal-server)│  │ (litmusportal-auth-server│   │
│  └──────┬──────────────┘  └──────────────────────────┘   │
│         │ registerInfra mutation                          │
│         │ generates K8s YAML via ManifestParser()         │
│         ▼                                                 │
│  ┌─────────────────────────────────────────────────────┐  │
│  │ Manifest Templates (manifests/cluster/ or namespace/)│  │
│  │  1a_argo_crds.yaml          3a_agents_rbac.yaml     │  │
│  │  1b_argo_rbac.yaml          3b_agents_deployment.yaml│  │
│  │  1c_argo_deployment.yaml    4a_mcp_tools_rbac.yaml  │  │
│  │  2a_litmus_crds.yaml        4b_mcp_tools_deployment │  │
│  │  2b_litmus_admin_rbac.yaml                          │  │
│  │  2c_litmus_deployment.yaml                          │  │
│  └─────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────┐
│  Chaos Infrastructure (target namespace, e.g. "litmus")  │
│                                                          │
│  Existing components:            NEW MCP tools:          │
│  ┌──────────────────┐   ┌──────────────────────────────┐ │
│  │ subscriber        │   │ kubernetes-mcp-server        │ │
│  │ chaos-operator    │   │   Port: 8081 (HTTP/SSE)      │ │
│  │ chaos-exporter    │   │   SA: mcp-server             │ │
│  │ event-tracker     │   │   ClusterRole: read K8s+CRDs │ │
│  │ workflow-controller│  ├──────────────────────────────┤ │
│  └──────────────────┘   │ prometheus-mcp-server         │ │
│                          │   Port: 9090 (HTTP/SSE)      │ │
│                          │   SA: prometheus-mcp-server   │ │
│                          │   ConfigMap: prom URL config  │ │
│                          └──────────────────────────────┘ │
└──────────────────────────────────────────────────────────┘
```

---

## Files Changed

### 1. New Template Files (Manifest Templates)

These templates are read alphabetically by `ManifestParser()` and included in the generated chaos infra YAML.

#### `chaoscenter/graphql/server/manifests/cluster/4a_mcp_tools_rbac.yaml`

RBAC resources for **cluster-scoped** infrastructure:

- **ServiceAccount** `mcp-server` — used by the kubernetes-mcp-server deployment
- **ClusterRole** `mcp-server-role` — read-only access to:
  - Standard K8s resources: pods, logs, services, deployments, jobs, etc.
  - LitmusChaos CRDs: chaosengines, chaosexperiments, chaosresults (read/write)
  - Argo Workflow CRDs: workflows, cronworkflows (read-only)
  - CRD discovery via apiextensions.k8s.io
- **ClusterRoleBinding** `mcp-server-role-binding`
- **ServiceAccount** `prometheus-mcp-server`
- **ClusterRole** `prometheus-mcp-server-role` — read-only access to pods, services, deployments, chaos CRDs, and Argo workflows
- **ClusterRoleBinding** `prometheus-mcp-server-role-binding`

#### `chaoscenter/graphql/server/manifests/cluster/4b_mcp_tools_deployment.yaml`

Deployment resources for **cluster-scoped** infrastructure:

- **ConfigMap** `prometheus-mcp-config` — Prometheus URL and transport settings
- **Secret** `prometheus-mcp-secret` — placeholder for future Prometheus auth
- **Deployment** `kubernetes-mcp-server` — runs with `--port 8081 --stateless` args
- **Service** `kubernetes-mcp-server` — ClusterIP on port 8081
- **Deployment** `prometheus-mcp-server` — configured via envFrom (ConfigMap + Secret)
- **Service** `prometheus-mcp-server` — ClusterIP on port 9090

#### `chaoscenter/graphql/server/manifests/namespace/4a_mcp_tools_rbac.yaml`

Same as the cluster version but uses **Role/RoleBinding** instead of ClusterRole/ClusterRoleBinding (scoped to the infra namespace).

#### `chaoscenter/graphql/server/manifests/namespace/4b_mcp_tools_deployment.yaml`

Identical to the cluster version (deployments are namespace-scoped by nature).

### 2. Go Source Changes

#### `chaoscenter/graphql/server/utils/variables.go`

Added three new fields to the `Configuration` struct:

```go
KubernetesMcpServerImage    string `split_words:"true" default:"quay.io/containers/kubernetes_mcp_server:latest"`
PrometheusMcpServerImage    string `split_words:"true" default:"ghcr.io/pab1it0/prometheus-mcp-server:latest"`
PrometheusMcpUrl            string `split_words:"true" default:"http://prometheus.monitoring.svc.cluster.local:9090"`
```

These map to environment variables via the `envconfig` library with `split_words:"true"`:
- `KUBERNETES_MCP_SERVER_IMAGE`
- `PROMETHEUS_MCP_SERVER_IMAGE`
- `PROMETHEUS_MCP_URL`

#### `chaoscenter/graphql/server/pkg/chaos_infrastructure/infra_utils.go`

Added three `strings.Replace` calls in the `ManifestParser()` function (after the existing `#{CUSTOM_TLS_CERT}` replacement):

```go
newContent = strings.Replace(newContent, "#{KUBERNETES_MCP_SERVER_IMAGE}", utils.Config.KubernetesMcpServerImage, -1)
newContent = strings.Replace(newContent, "#{PROMETHEUS_MCP_SERVER_IMAGE}", utils.Config.PrometheusMcpServerImage, -1)
newContent = strings.Replace(newContent, "#{PROMETHEUS_MCP_URL}", utils.Config.PrometheusMcpUrl, -1)
```

### 3. Configuration Changes

#### `local-custom/k8s/litmus-installation.yaml`

Added environment variables to the `litmusportal-server` (GraphQL server) container:

```yaml
- name: KUBERNETES_MCP_SERVER_IMAGE
  value: "quay.io/containers/kubernetes_mcp_server:latest"
- name: PROMETHEUS_MCP_SERVER_IMAGE
  value: "ghcr.io/pab1it0/prometheus-mcp-server:latest"
- name: PROMETHEUS_MCP_URL
  value: "http://prometheus.monitoring.svc.cluster.local:9090"
```

Updated `INFRA_DEPLOYMENTS` to include MCP tool health monitoring:

```yaml
- name: INFRA_DEPLOYMENTS
  value: '["app=chaos-exporter", "name=chaos-operator", "app=workflow-controller", "app=event-tracker", "app=kubernetes-mcp-server", "app=prometheus-mcp-server"]'
```

#### `local-custom/config/.env`

Added matching environment variables:

```
KUBERNETES_MCP_SERVER_IMAGE=quay.io/containers/kubernetes_mcp_server:latest
PROMETHEUS_MCP_SERVER_IMAGE=ghcr.io/pab1it0/prometheus-mcp-server:latest
PROMETHEUS_MCP_URL=http://prometheus.monitoring.svc.cluster.local:9090
INFRA_DEPLOYMENTS='["app=chaos-exporter", "name=chaos-operator", "app=event-tracker","app=workflow-controller","app=kubernetes-mcp-server","app=prometheus-mcp-server"]'
```

---

## How It Works

### Manifest Generation Flow

1. User calls the `registerInfra` GraphQL mutation (via UI or API)
2. The GraphQL server invokes `GetK8sInfraYaml()` in `infra_utils.go`
3. Based on the infra scope (cluster or namespace), `ManifestParser()` reads all YAML template files from the corresponding `manifests/cluster/` or `manifests/namespace/` directory
4. Files are read **alphabetically**, so `4a_mcp_tools_rbac.yaml` and `4b_mcp_tools_deployment.yaml` are processed after all existing templates (1a-3b)
5. Template placeholders (`#{PLACEHOLDER}`) are replaced with actual values from the server's `Configuration` struct
6. The complete YAML (all templates concatenated) is returned to the user for applying to their target cluster

### Key Placeholders Used in MCP Templates

| Placeholder | Source | Example Value |
|-------------|--------|---------------|
| `#{INFRA_NAMESPACE}` | User-provided or default `litmus` | `litmus` |
| `#{KUBERNETES_MCP_SERVER_IMAGE}` | `utils.Config.KubernetesMcpServerImage` | `quay.io/containers/kubernetes_mcp_server:latest` |
| `#{PROMETHEUS_MCP_SERVER_IMAGE}` | `utils.Config.PrometheusMcpServerImage` | `ghcr.io/pab1it0/prometheus-mcp-server:latest` |
| `#{PROMETHEUS_MCP_URL}` | `utils.Config.PrometheusMcpUrl` | `http://prometheus.monitoring.svc.cluster.local:9090` |
| `#{TOLERATIONS}` | User-provided infra tolerations | (YAML block or empty) |
| `#{NODE_SELECTOR}` | User-provided node selector | (YAML block or empty) |

---

## Deploying Chaos Infrastructure with MCP Tools

### Option 1: Via the LitmusChaos UI

1. Navigate to **Environments** → select an environment → **Enable Chaos Infrastructure**
2. Configure the infrastructure name, namespace, scope, etc.
3. Download and apply the generated YAML — it will automatically include MCP tool deployments

### Option 2: Via the GraphQL API

```graphql
mutation {
  registerInfra(
    projectID: "your-project-id"
    request: {
      name: "my-infra"
      environmentID: "your-env-id"
      infraScope: "cluster"
      infraNamespace: "litmus"
      platformName: "Kubernetes"
      infraNsExists: false
      infraSaExists: false
    }
  ) {
    infraID
    name
    manifest
  }
}
```

The `manifest` field will contain the full YAML including MCP tool resources.

### Option 3: Standalone MCP Tools Deployment

If you need to deploy MCP tools independently (without creating a full chaos infrastructure), you can use the standalone manifest:

```bash
kubectl apply -f mcptools/test-mcp-litmus-chaos-enable.yml
```

This creates the `litmus` namespace with both MCP tool deployments and their RBAC.

---

## Accessing MCP Tools

After the chaos infrastructure YAML is applied to the target cluster, the MCP tools are accessible within the cluster.

### Port-Forwarding for Local Access

```bash
# kubernetes-mcp-server → localhost:8081
kubectl port-forward -n <infra-namespace> svc/kubernetes-mcp-server 8081:8081

# prometheus-mcp-server → localhost:9091 (avoiding conflict with cluster Prometheus on 9090)
kubectl port-forward -n <infra-namespace> svc/prometheus-mcp-server 9091:9090
```

### Testing with MCP Inspector

```bash
npx @modelcontextprotocol/inspector

# Then in the inspector UI:
# - kubernetes-mcp-server: http://localhost:8081/sse
# - prometheus-mcp-server: http://localhost:9091/sse
```

### In-Cluster URLs

Other pods within the cluster can reach the MCP tools at:
- `http://kubernetes-mcp-server.<namespace>.svc.cluster.local:8081`
- `http://prometheus-mcp-server.<namespace>.svc.cluster.local:9090`

---

## Configuration Reference

### Environment Variables (GraphQL Server)

| Variable | Default | Description |
|----------|---------|-------------|
| `KUBERNETES_MCP_SERVER_IMAGE` | `quay.io/containers/kubernetes_mcp_server:latest` | Docker image for the Kubernetes MCP server |
| `PROMETHEUS_MCP_SERVER_IMAGE` | `ghcr.io/pab1it0/prometheus-mcp-server:latest` | Docker image for the Prometheus MCP server |
| `PROMETHEUS_MCP_URL` | `http://prometheus.monitoring.svc.cluster.local:9090` | Prometheus endpoint URL that the Prometheus MCP server queries |

### Customizing Images

To use custom or private registry images, set the environment variables on the GraphQL server deployment before registering infrastructure:

```yaml
# In litmus-installation.yaml or via kubectl set env
- name: KUBERNETES_MCP_SERVER_IMAGE
  value: "my-registry.example.com/kubernetes-mcp-server:v1.0"
- name: PROMETHEUS_MCP_SERVER_IMAGE
  value: "my-registry.example.com/prometheus-mcp-server:v1.0"
- name: PROMETHEUS_MCP_URL
  value: "http://my-prometheus:9090"
```

### Health Monitoring

The MCP tools are included in `INFRA_DEPLOYMENTS`, which means the Litmus subscriber monitors their health alongside other chaos infrastructure components:

```
INFRA_DEPLOYMENTS=["app=chaos-exporter", "name=chaos-operator", "app=workflow-controller", "app=event-tracker", "app=kubernetes-mcp-server", "app=prometheus-mcp-server"]
```

---

## Security

### RBAC Permissions

- **kubernetes-mcp-server**: Has **read-only** access to standard K8s resources and **read/write** access to LitmusChaos CRDs (needed for chaos experiment management)
- **prometheus-mcp-server**: Has **read-only** access to pods, services, deployments, and chaos/workflow CRDs
- Both run as **non-root** (`runAsUser: 65532`) with `allowPrivilegeEscalation: false`
- The prometheus-mcp-server additionally uses `readOnlyRootFilesystem: true`

### Resource Limits

Both deployments have conservative resource limits:

```yaml
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "256Mi"
    cpu: "500m"
```

---

## Build and Deploy

After making changes to the MCP templates or Go source code, rebuild and deploy:

```bash
# From Git Bash on Windows
cd local-custom/scripts
bash build-and-deploy.sh --env-file ../config/.env
```

This rebuilds the GraphQL server Docker image (which includes the manifest templates via `COPY server/manifests/. /litmus/manifests` in the Dockerfile), loads it into minikube, and redeploys.
