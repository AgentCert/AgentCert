# Kubernetes MCP Server - HTTP Setup Guide

This document provides step-by-step instructions to deploy the Kubernetes MCP Server in HTTP mode inside a Kubernetes cluster and access it from your application.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Step 1: Create Namespace and ServiceAccount](#step-1-create-namespace-and-serviceaccount)
- [Step 2: Configure RBAC Permissions](#step-2-configure-rbac-permissions)
- [Step 3: Deploy the MCP Server](#step-3-deploy-the-mcp-server)
- [Step 4: Expose the MCP Server via Service](#step-4-expose-the-mcp-server-via-service)
- [Step 5: Verify the Deployment](#step-5-verify-the-deployment)
- [Step 6: Connect Your Application](#step-6-connect-your-application)
- [Optional: Expose Externally via Ingress](#optional-expose-externally-via-ingress)
- [Optional: Enable OAuth/OIDC Authentication](#optional-enable-oauthoidc-authentication)
- [Configuration Options](#configuration-options)
- [Available HTTP Endpoints](#available-http-endpoints)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

- A running Kubernetes cluster (v1.24+)
- `kubectl` configured with cluster admin access
- (Optional) Helm v3 if using the Helm chart deployment method

---

## Step 1: Create Namespace and ServiceAccount

Create a dedicated namespace and ServiceAccount for the MCP server:

```bash
# Create a dedicated namespace
kubectl create namespace mcp

# Create a ServiceAccount
kubectl create serviceaccount mcp-server -n mcp
```

---

## Step 2: Configure RBAC Permissions

Choose one of the following RBAC configurations based on your security requirements.

### Option A: Read-Only Access (Recommended for most use cases)

```bash
# Grant cluster-wide read-only access
kubectl create clusterrolebinding mcp-server-view \
    --clusterrole=view \
    --serviceaccount=mcp:mcp-server
```

### Option B: Full Access (Use with caution)

```yaml
# Save as rbac.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: mcp-server-admin
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: ["*"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: mcp-server-admin-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: mcp-server-admin
subjects:
  - kind: ServiceAccount
    name: mcp-server
    namespace: mcp
```

```bash
kubectl apply -f rbac.yaml
```

### Option C: Namespace-Scoped Access

```bash
# Grant access only to a specific namespace (e.g., "my-app")
kubectl create rolebinding mcp-server-edit \
    --clusterrole=edit \
    --serviceaccount=mcp:mcp-server \
    -n my-app
```

---

## Step 3: Deploy the MCP Server

### Option A: Using Kubernetes Manifests

```yaml
# Save as mcp-server-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kubernetes-mcp-server
  namespace: mcp
  labels:
    app: kubernetes-mcp-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kubernetes-mcp-server
  template:
    metadata:
      labels:
        app: kubernetes-mcp-server
    spec:
      serviceAccountName: mcp-server
      containers:
        - name: mcp-server
          image: quay.io/containers/kubernetes_mcp_server:latest
          args:
            - "--port"
            - "8080"
            - "--stateless"       # Recommended for container deployments
            - "--read-only"       # Remove if write access is needed
          ports:
            - containerPort: 8080
              name: http
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 3
            periodSeconds: 5
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 256Mi
```

```bash
kubectl apply -f mcp-server-deployment.yaml
```

### Option B: Using the Helm Chart

```bash
# Add/install from the project's Helm chart
helm install kubernetes-mcp-server ./charts/kubernetes-mcp-server \
    --namespace mcp \
    --set service.port=8080
```

---

## Step 4: Expose the MCP Server via Service

If you used the Kubernetes manifests approach (Option A above), create a Service:

```yaml
# Save as mcp-server-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: kubernetes-mcp-server
  namespace: mcp
  labels:
    app: kubernetes-mcp-server
spec:
  type: ClusterIP
  selector:
    app: kubernetes-mcp-server
  ports:
    - port: 8080
      targetPort: http
      protocol: TCP
      name: http
```

```bash
kubectl apply -f mcp-server-service.yaml
```

---

## Step 5: Verify the Deployment

### Check the pod is running

```bash
kubectl get pods -n mcp -l app=kubernetes-mcp-server
```

Expected output:

```
NAME                                      READY   STATUS    RESTARTS   AGE
kubernetes-mcp-server-xxxxxxxxxx-xxxxx    1/1     Running   0          30s
```

### Check the health endpoint

```bash
# Port-forward for local testing
kubectl port-forward -n mcp svc/kubernetes-mcp-server 8080:8080

# In another terminal, check health
curl http://localhost:8080/healthz
```

### Check server stats

```bash
curl http://localhost:8080/stats
```

---

## Step 6: Connect Your Application

### Internal Cluster Access (Recommended)

From any pod running in the same Kubernetes cluster, connect to the MCP server using the internal DNS:

```
http://kubernetes-mcp-server.mcp.svc.cluster.local:8080
```

### MCP Client Configuration

#### Streamable HTTP Transport (Recommended)

Use the `/mcp` endpoint for the modern Streamable HTTP transport:

```json
{
  "mcpServers": {
    "kubernetes": {
      "type": "streamable-http",
      "url": "http://kubernetes-mcp-server.mcp.svc.cluster.local:8080/mcp"
    }
  }
}
```

#### SSE Transport (Legacy)

Use the `/sse` endpoint for Server-Sent Events transport:

```json
{
  "mcpServers": {
    "kubernetes": {
      "type": "sse",
      "url": "http://kubernetes-mcp-server.mcp.svc.cluster.local:8080/sse"
    }
  }
}
```

### Example: Connecting with a Go MCP Client

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/mark3labs/mcp-go/client"
    "github.com/mark3labs/mcp-go/mcp"
)

func main() {
    ctx := context.Background()

    // Connect to the MCP server via Streamable HTTP
    mcpClient, err := client.NewStreamableHttpClient(
        "http://kubernetes-mcp-server.mcp.svc.cluster.local:8080/mcp",
    )
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }

    if err := mcpClient.Start(ctx); err != nil {
        log.Fatalf("Failed to start client: %v", err)
    }
    defer mcpClient.Close()

    // Initialize the MCP session
    initResult, err := mcpClient.Initialize(ctx, mcp.InitializeRequest{
        Params: mcp.InitializeParams{
            ClientInfo: mcp.Implementation{
                Name:    "my-app",
                Version: "1.0.0",
            },
            ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
        },
    })
    if err != nil {
        log.Fatalf("Failed to initialize: %v", err)
    }
    fmt.Printf("Connected to: %s %s\n", initResult.ServerInfo.Name, initResult.ServerInfo.Version)

    // List available tools
    toolsResult, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
    if err != nil {
        log.Fatalf("Failed to list tools: %v", err)
    }
    for _, tool := range toolsResult.Tools {
        fmt.Printf("Tool: %s - %s\n", tool.Name, tool.Description)
    }

    // Example: List all namespaces
    callResult, err := mcpClient.CallTool(ctx, mcp.CallToolRequest{
        Params: mcp.CallToolParams{
            Name: "namespaces_list",
        },
    })
    if err != nil {
        log.Fatalf("Failed to call tool: %v", err)
    }
    for _, content := range callResult.Content {
        if textContent, ok := content.(mcp.TextContent); ok {
            fmt.Println(textContent.Text)
        }
    }
}
```

### Example: Connecting with a Python MCP Client

```python
import asyncio
from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client

async def main():
    url = "http://kubernetes-mcp-server.mcp.svc.cluster.local:8080/mcp"

    async with streamablehttp_client(url) as (read_stream, write_stream, _):
        async with ClientSession(read_stream, write_stream) as session:
            # Initialize the session
            await session.initialize()

            # List available tools
            tools = await session.list_tools()
            for tool in tools.tools:
                print(f"Tool: {tool.name} - {tool.description}")

            # Example: List all namespaces
            result = await session.call_tool("namespaces_list", arguments={})
            print(result)

asyncio.run(main())
```

---

## Optional: Expose Externally via Ingress

To access the MCP server from outside the cluster:

```yaml
# Save as mcp-server-ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: kubernetes-mcp-server
  namespace: mcp
  annotations:
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
    # Required for SSE connections
    nginx.ingress.kubernetes.io/proxy-buffering: "off"
spec:
  ingressClassName: nginx
  rules:
    - host: mcp.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: kubernetes-mcp-server
                port:
                  number: 8080
  tls:
    - hosts:
        - mcp.example.com
      secretName: mcp-tls-secret
```

```bash
kubectl apply -f mcp-server-ingress.yaml
```

> **⚠️ Important:** When exposing the MCP server externally, always enable authentication.
> See the [OAuth/OIDC section](#optional-enable-oauthoidc-authentication) below.

---

## Optional: Enable OAuth/OIDC Authentication

For production deployments, secure the HTTP endpoint with OAuth/OIDC:

```yaml
# In the Deployment, update the container args:
args:
  - "--port"
  - "8080"
  - "--stateless"
  - "--require-oauth"
  - "--authorization-url"
  - "https://keycloak.example.com/realms/mcp/protocol/openid-connect"
  - "--server-url"
  - "https://mcp.example.com"
```

For a complete OAuth/OIDC setup using Keycloak, refer to the
[Keycloak OIDC Setup Guide](https://github.com/containers/kubernetes-mcp-server/blob/main/docs/KEYCLOAK_OIDC_SETUP.md).

---

## Configuration Options

The following flags can be passed as container `args` in the Deployment:

| Flag                      | Description                                                                 |
|---------------------------|-----------------------------------------------------------------------------|
| `--port 8080`             | **(Required)** Enables HTTP mode on the specified port                      |
| `--stateless`             | Recommended for container deployments, disables state-dependent features    |
| `--read-only`             | Restricts to read-only operations (no create, update, delete)              |
| `--disable-destructive`   | Allows create/update but blocks delete operations                          |
| `--toolsets core,config`  | Comma-separated list of toolsets to enable                                 |
| `--log-level 2`           | Logging verbosity (0-9, similar to kubectl)                                |
| `--disable-multi-cluster` | Restrict to the in-cluster context only                                    |
| `--require-oauth`         | Require OAuth authentication for all requests                              |
| `--kubeconfig`            | Path to kubeconfig (not needed for in-cluster deployments)                 |

### Available Toolsets

| Toolset    | Description                                          | Default |
|------------|------------------------------------------------------|---------|
| `config`   | Kubeconfig view and cluster/context management       | ✓       |
| `core`     | Pods, Resources, Events, Namespaces, Nodes, Helm     | ✓       |
| `helm`     | Helm chart install, list, uninstall                  | ✓       |
| `kubevirt` | KubeVirt virtual machine management                  |         |
| `kiali`    | Service mesh observability via Kiali                 |         |
| `kcp`      | Multi-tenancy workspace management                   |         |

---

## Available HTTP Endpoints

Once the server is running in HTTP mode, the following endpoints are available:

| Endpoint   | Method       | Description                                    |
|------------|--------------|------------------------------------------------|
| `/mcp`     | POST         | Streamable HTTP MCP transport (recommended)    |
| `/sse`     | GET          | Server-Sent Events MCP transport (legacy)      |
| `/message` | POST         | SSE message submission endpoint                |
| `/healthz` | GET          | Health check (returns 200 if healthy)          |
| `/stats`   | GET          | Server statistics in JSON format               |
| `/metrics` | GET          | Prometheus-compatible metrics                  |

---

## Troubleshooting

### Pod is in CrashLoopBackOff

```bash
# Check pod logs
kubectl logs -n mcp -l app=kubernetes-mcp-server --tail=50
```

Common causes:
- Missing RBAC permissions for the ServiceAccount
- Invalid command-line arguments

### Cannot connect from application pod

```bash
# Verify the service is reachable from within the cluster
kubectl run -n mcp test-curl --rm -it --image=curlimages/curl -- \
    curl -s http://kubernetes-mcp-server.mcp.svc.cluster.local:8080/healthz
```

### Permission denied errors when calling tools

```bash
# Check what permissions the ServiceAccount has
kubectl auth can-i --list --as=system:serviceaccount:mcp:mcp-server
```

### SSE connection drops behind proxy/ingress

Ensure your ingress controller has:
- Proxy buffering disabled
- Adequate read/send timeouts (at least 3600s for long-lived SSE connections)
- WebSocket/HTTP upgrade support enabled

---

## References

- [Kubernetes MCP Server Repository](https://github.com/containers/kubernetes-mcp-server)
- [Model Context Protocol Specification](https://modelcontextprotocol.io/)
- [Kubernetes RBAC Documentation](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [Keycloak OIDC Setup Guide](https://github.com/containers/kubernetes-mcp-server/blob/main/docs/KEYCLOAK_OIDC_SETUP.md)
- [Kubernetes ServiceAccount Setup Guide](https://github.com/containers/kubernetes-mcp-server/blob/main/docs/GETTING_STARTED_KUBERNETES.md)