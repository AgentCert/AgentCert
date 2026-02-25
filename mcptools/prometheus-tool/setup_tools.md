# Prometheus MCP Server - HTTP Setup Guide

This document provides step-by-step instructions to deploy the Prometheus MCP Server in HTTP mode inside a Kubernetes cluster and access it from your application or AI tools.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Step 1: Create Namespace and ServiceAccount](#step-1-create-namespace-and-serviceaccount)
- [Step 2: Configure Prometheus Connection](#step-2-configure-prometheus-connection)
- [Step 3: Configure RBAC Permissions](#step-3-configure-rbac-permissions)
- [Step 4: Deploy the MCP Server](#step-4-deploy-the-mcp-server)
- [Step 5: Expose the MCP Server via Service](#step-5-expose-the-mcp-server-via-service)
- [Step 6: Verify the Deployment](#step-6-verify-the-deployment)
- [Step 7: Connect Your Application](#step-7-connect-your-application)
- [Optional: Expose Externally via Ingress](#optional-expose-externally-via-ingress)
- [Configuration Options](#configuration-options)
- [Available MCP Tools](#available-mcp-tools)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

- A running Kubernetes cluster (v1.24+)
- `kubectl` configured with cluster admin access
- A Prometheus server accessible from within the cluster
- (Optional) MCP Inspector for testing: `npx @modelcontextprotocol/inspector`

---

## Step 1: Create Namespace and ServiceAccount

The deployment uses the `litmus` namespace (shared with Litmus Chaos infrastructure). If deploying standalone, create a dedicated namespace:

```bash
# Using the litmus namespace (already exists if Litmus is deployed)
kubectl get namespace litmus || kubectl create namespace litmus

# The ServiceAccount is created automatically by mcp-server-deploy.yml
```

---

## Step 2: Configure Prometheus Connection

Before deploying, update the `PROMETHEUS_URL` in the ConfigMap section of `mcp-server-deploy.yml` to point to your Prometheus instance.

### Finding Your Prometheus URL

```bash
# Check if Prometheus is running in the cluster
kubectl get svc -A | grep -i prometheus

# Common in-cluster URLs:
# - Prometheus Operator:   http://prometheus-operated.monitoring.svc.cluster.local:9090
# - Helm kube-prometheus:  http://prometheus-server.monitoring.svc.cluster.local:9090
# - Standalone:            http://prometheus.monitoring.svc.cluster.local:9090
```

### Common Prometheus URL patterns

| Deployment Type | URL |
|----------------|-----|
| In-cluster (monitoring ns) | `http://prometheus-server.monitoring.svc.cluster.local:9090` |
| In-cluster (default ns) | `http://prometheus-server.default.svc.cluster.local:9090` |
| Minikube host | `http://host.minikube.internal:9090` |
| External | `https://prometheus.example.com` |
| Thanos/Cortex/Mimir | `http://thanos-query.monitoring.svc.cluster.local:9090` |

### Update the ConfigMap

Edit the `PROMETHEUS_URL` field in `mcp-server-deploy.yml`:

```yaml
data:
  PROMETHEUS_URL: "http://prometheus-server.monitoring.svc.cluster.local:9090"  # ← Update this
```

### (Optional) Configure Authentication

If your Prometheus requires authentication, uncomment the relevant fields in the Secret section:

```yaml
# For basic auth:
stringData:
  PROMETHEUS_USERNAME: "admin"
  PROMETHEUS_PASSWORD: "your-password"

# For bearer token auth:
stringData:
  PROMETHEUS_TOKEN: "your-bearer-token"

# For multi-tenant setups (Cortex/Mimir/Thanos):
stringData:
  ORG_ID: "your-org-id"
```

---

## Step 3: Configure RBAC Permissions

The `mcp-server-deploy.yml` includes a ClusterRole with read-only access to Kubernetes resources and Litmus CRDs. This allows the MCP server to:

- Resolve service endpoints for Prometheus discovery
- Observe Litmus chaos experiment resources

The permissions are intentionally minimal. The Prometheus MCP server primarily communicates with the Prometheus HTTP API — the RBAC is supplementary.

### Included Permissions

| API Group | Resources | Verbs |
|-----------|-----------|-------|
| `""` (core) | pods, services, endpoints, namespaces, nodes | get, list, watch |
| `apps` | deployments, statefulsets, daemonsets | get, list, watch |
| `litmuschaos.io` | chaosengines, chaosexperiments, chaosresults | get, list, watch |
| `argoproj.io` | workflows, cronworkflows | get, list, watch |

---

## Step 4: Deploy the MCP Server

### One-command deployment

```bash
kubectl apply -f mcp-server-deploy.yml
```

This creates all resources:
- ServiceAccount (`prometheus-mcp-server`)
- ConfigMap (`prometheus-mcp-config`)
- Secret (`prometheus-mcp-secret`)
- ClusterRole + ClusterRoleBinding
- Deployment (`prometheus-mcp-server`)
- Service (`prometheus-mcp-server`)

---

## Step 5: Expose the MCP Server via Service

The Service is included in `mcp-server-deploy.yml` and created automatically. It exposes the MCP server on port 9090 within the cluster (using a different port from the k8s-log-tool which runs on 8080).

```bash
# Verify the service
kubectl get svc prometheus-mcp-server -n litmus
```

---

## Step 6: Verify the Deployment

### Check the pod is running

```bash
kubectl get pods -n litmus -l app=prometheus-mcp-server
```

Expected output:

```
NAME                                      READY   STATUS    RESTARTS   AGE
prometheus-mcp-server-xxxxxxxxxx-xxxxx    1/1     Running   0          30s
```

### Check pod logs

```bash
kubectl logs -n litmus -l app=prometheus-mcp-server --tail=20
```

### Check the health endpoint

```bash
# Port-forward for local testing
kubectl port-forward -n litmus svc/prometheus-mcp-server 9090:9090

# In another terminal, check health
curl http://localhost:9090/healthz
```

### Test a PromQL query via MCP

Using the MCP Inspector:

```bash
npx @modelcontextprotocol/inspector
```

In the Inspector UI:
1. Set server URL to `http://localhost:9090`
2. Choose transport type: **Streamable HTTP**
3. Click **Connect**
4. Use the `execute_query` tool with query: `up`

---

## Step 7: Connect Your Application

### Internal Cluster Access (Recommended)

From any pod in the cluster:

```
http://prometheus-mcp-server.litmus.svc.cluster.local:9090
```

### MCP Client Configuration

#### Streamable HTTP Transport (Recommended)

```json
{
  "mcpServers": {
    "prometheus": {
      "type": "streamable-http",
      "url": "http://prometheus-mcp-server.litmus.svc.cluster.local:9090/mcp"
    }
  }
}
```

#### SSE Transport (Legacy)

```json
{
  "mcpServers": {
    "prometheus": {
      "type": "sse",
      "url": "http://prometheus-mcp-server.litmus.svc.cluster.local:9090/sse"
    }
  }
}
```

#### Docker Desktop (Local Development)

```json
{
  "mcpServers": {
    "prometheus": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "PROMETHEUS_URL",
        "ghcr.io/pab1it0/prometheus-mcp-server:latest"
      ],
      "env": {
        "PROMETHEUS_URL": "http://host.docker.internal:9090"
      }
    }
  }
}
```

### Example: Python MCP Client

```python
import asyncio
from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client

async def main():
    url = "http://prometheus-mcp-server.litmus.svc.cluster.local:9090/mcp"

    async with streamablehttp_client(url) as (read_stream, write_stream, _):
        async with ClientSession(read_stream, write_stream) as session:
            await session.initialize()

            # List available tools
            tools = await session.list_tools()
            for tool in tools.tools:
                print(f"Tool: {tool.name} - {tool.description}")

            # Execute a PromQL query
            result = await session.call_tool("execute_query", arguments={
                "query": "up"
            })
            print(result)

            # Execute a range query
            result = await session.call_tool("execute_range_query", arguments={
                "query": "rate(http_requests_total[5m])",
                "start": "2026-02-24T00:00:00Z",
                "end": "2026-02-24T12:00:00Z",
                "step": "60s"
            })
            print(result)

asyncio.run(main())
```

---

## Optional: Expose Externally via Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: prometheus-mcp-server
  namespace: litmus
  annotations:
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-buffering: "off"
spec:
  ingressClassName: nginx
  rules:
    - host: prometheus-mcp.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: prometheus-mcp-server
                port:
                  number: 9090
  tls:
    - hosts:
        - prometheus-mcp.example.com
      secretName: prometheus-mcp-tls-secret
```

> **⚠️ Important:** When exposing externally, consider adding authentication via an OAuth proxy or API gateway.

---

## Configuration Options

### Environment Variables (ConfigMap)

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `PROMETHEUS_URL` | URL of your Prometheus server | **Yes** | — |
| `PROMETHEUS_MCP_SERVER_TRANSPORT` | Transport mode: `stdio`, `http`, `sse` | No | `stdio` |
| `PROMETHEUS_MCP_BIND_HOST` | Host to bind for HTTP transport | No | `127.0.0.1` |
| `PROMETHEUS_MCP_BIND_PORT` | Port for HTTP transport | No | `8080` (set to `9090` in our deploy) |
| `PROMETHEUS_REQUEST_TIMEOUT` | Request timeout in seconds | No | `30` |
| `PROMETHEUS_URL_SSL_VERIFY` | Set `false` to disable SSL verification | No | `true` |
| `PROMETHEUS_DISABLE_LINKS` | Set `true` to disable UI links (saves tokens) | No | `false` |
| `PROMETHEUS_CUSTOM_HEADERS` | Custom headers as JSON string | No | — |
| `TOOL_PREFIX` | Prefix for tool names (e.g., `staging_execute_query`) | No | — |

### Environment Variables (Secret)

| Variable | Description | Required |
|----------|-------------|----------|
| `PROMETHEUS_USERNAME` | Basic auth username | No |
| `PROMETHEUS_PASSWORD` | Basic auth password | No |
| `PROMETHEUS_TOKEN` | Bearer token authentication | No |
| `ORG_ID` | Organization ID for multi-tenant setups | No |

---

## Available MCP Tools

The Prometheus MCP server exposes 6 tools:

| Tool | Category | Description |
|------|----------|-------------|
| `health_check` | System | Health check endpoint for container monitoring and status verification |
| `execute_query` | Query | Execute a PromQL instant query against Prometheus |
| `execute_range_query` | Query | Execute a PromQL range query with start time, end time, and step interval |
| `list_metrics` | Discovery | List all available metrics with optional pagination and filtering |
| `get_metric_metadata` | Discovery | Get metadata for a specific metric or bulk metadata with filtering |
| `get_targets` | Discovery | Get information about all scrape targets |

### Tool Usage Examples

#### execute_query
```json
{
  "query": "up",
  "time": "2026-02-24T10:00:00Z"
}
```

#### execute_range_query
```json
{
  "query": "rate(http_requests_total[5m])",
  "start": "2026-02-24T00:00:00Z",
  "end": "2026-02-24T12:00:00Z",
  "step": "60s"
}
```

#### list_metrics
```json
{
  "filter_pattern": "http_",
  "limit": "50",
  "offset": 0
}
```

#### get_metric_metadata
```json
{
  "metric": "http_requests_total"
}
```

#### get_targets
No arguments required.

---

## Troubleshooting

### Pod is in CrashLoopBackOff

```bash
# Check pod logs
kubectl logs -n litmus -l app=prometheus-mcp-server --tail=50
```

Common causes:
- **`PROMETHEUS_URL` not set or invalid** — The server needs a valid Prometheus URL
- **Wrong transport mode** — Ensure `PROMETHEUS_MCP_SERVER_TRANSPORT=http` for Kubernetes deployment
- **Bind host issue** — Must be `0.0.0.0` (not `127.0.0.1`) when running in a pod

### Cannot connect to Prometheus

```bash
# Test from within the cluster
kubectl run -n litmus test-curl --rm -it --image=curlimages/curl -- \
    curl -s http://prometheus-server.monitoring.svc.cluster.local:9090/-/healthy

# If using minikube host
kubectl run -n litmus test-curl --rm -it --image=curlimages/curl -- \
    curl -s http://host.minikube.internal:9090/-/healthy
```

### Health endpoint returns error

```bash
# Port-forward and test
kubectl port-forward -n litmus svc/prometheus-mcp-server 9090:9090
curl -v http://localhost:9090/healthz
```

### MCP Inspector cannot connect

1. Ensure port-forward is running: `kubectl port-forward -n litmus svc/prometheus-mcp-server 9090:9090`
2. In Inspector, use URL: `http://localhost:9090`
3. Try both **Streamable HTTP** and **SSE** transport types
4. Check for CORS issues in browser console

### Permission denied errors

```bash
# Check ServiceAccount permissions
kubectl auth can-i --list --as=system:serviceaccount:litmus:prometheus-mcp-server
```

### SSL verification errors

If Prometheus uses self-signed certificates, set in the ConfigMap:
```yaml
PROMETHEUS_URL_SSL_VERIFY: "false"
```

---

## Cleanup

Remove all Prometheus MCP Server resources:

```bash
kubectl delete -f mcp-server-deploy.yml
```

Or remove individually:

```bash
kubectl delete svc prometheus-mcp-server -n litmus
kubectl delete deployment prometheus-mcp-server -n litmus
kubectl delete configmap prometheus-mcp-config -n litmus
kubectl delete secret prometheus-mcp-secret -n litmus
kubectl delete clusterrolebinding prometheus-mcp-server-role-binding
kubectl delete clusterrole prometheus-mcp-server-role
kubectl delete serviceaccount prometheus-mcp-server -n litmus
```

---

## References

- [Prometheus MCP Server Repository](https://github.com/pab1it0/prometheus-mcp-server)
- [Docker Hub — Prometheus MCP](https://hub.docker.com/mcp/server/prometheus/overview)
- [GHCR Image](https://github.com/users/pab1it0/packages/container/package/prometheus-mcp-server)
- [Model Context Protocol Specification](https://modelcontextprotocol.io/)
- [PromQL Documentation](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Kubernetes MCP Server (k8s-log-tool)](../k8s-log-tool/setup_tools.md)
