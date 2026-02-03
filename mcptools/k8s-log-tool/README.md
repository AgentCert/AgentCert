# MCP K8s Log Tool

A minimal MCP-style Kubernetes log fetching tool built in Go. This HTTP service provides an API to fetch Kubernetes pod logs from any cluster accessible via kubeconfig.

## Features

- 🚀 HTTP API for fetching Kubernetes pod logs
- 🔍 Support for filtering by namespace, pod, and container
- 📊 Configurable log line limits
- 🔄 Streaming logs support
- 🌐 CORS-enabled for browser access
- 🏥 Health check endpoint
- 🐳 Docker containerization
- ☸️ Kubernetes deployment ready
- 🔐 RBAC security configuration

## Quick Start

### Local Development

1. **Prerequisites:**
   - Go 1.21+
   - Minikube or any Kubernetes cluster
   - kubectl configured with cluster access

2. **Install dependencies:**
   ```bash
   cd mcptools/k8s-log-tool
   go mod tidy
   ```

3. **Run the service:**
   ```bash
   go run main.go
   ```

4. **Test the API:**
   - Open browser to http://localhost:8080 for documentation
   - Health check: http://localhost:8080/health
   - Fetch logs: http://localhost:8080/logs?namespace=default&pod=my-pod

### Using Browser DevTools

1. Open browser to http://localhost:8080
2. Open DevTools (F12)
3. Go to Console tab
4. Test the API:
   ```javascript
   // Fetch logs for a pod
   fetch('http://localhost:8080/logs?namespace=kube-system&pod=kube-proxy-xxxx&lines=10')
     .then(response => response.text())
     .then(logs => console.log(logs));
   
   // Health check
   fetch('http://localhost:8080/health')
     .then(response => response.json())
     .then(data => console.log(data));
   ```

## API Endpoints

### GET /logs

Fetch pod logs from Kubernetes cluster.

**Query Parameters:**
- `namespace` (required): Kubernetes namespace
- `pod` (required): Pod name
- `container` (optional): Specific container name
- `lines` (optional): Number of log lines to fetch (default: 100)
- `follow` (optional): Stream logs (true/false, default: false)

**Examples:**
```bash
# Basic usage
curl "http://localhost:8080/logs?namespace=default&pod=my-app"

# With specific container and line count
curl "http://localhost:8080/logs?namespace=default&pod=my-app&container=web&lines=50"

# Stream logs
curl "http://localhost:8080/logs?namespace=default&pod=my-app&follow=true"
```

### GET /health

Health check endpoint returning service status.

### GET /

API documentation (HTML interface).

## Deployment

### Docker

1. **Build image:**
   ```bash
   docker build -t k8s-log-tool:latest .
   ```

2. **Run container:**
   ```bash
   # Mount your kubeconfig
   docker run -p 8080:8080 \
     -v ~/.kube/config:/root/.kube/config:ro \
     k8s-log-tool:latest
   ```

### Kubernetes

1. **Load image to Minikube:**
   ```bash
   # Build and load to minikube
   eval $(minikube docker-env)
   docker build -t k8s-log-tool:latest .
   ```

2. **Deploy to cluster:**
   ```bash
   kubectl apply -f deployment.yaml
   ```

3. **Access the service:**
   ```bash
   # Port forward
   kubectl port-forward svc/k8s-log-tool 8080:8080
   
   # Or get service URL (if using NodePort/LoadBalancer)
   minikube service k8s-log-tool --url
   ```

## Security

The deployment includes:
- Service Account with minimal RBAC permissions
- Non-root container execution
- Read-only root filesystem
- Resource limits
- Security context constraints

## Troubleshooting

### Common Issues

1. **"no kubeconfig found"**
   - Ensure kubectl is configured: `kubectl cluster-info`
   - Check KUBECONFIG environment variable
   - Verify ~/.kube/config exists

2. **"Pod not found"**
   - Verify pod exists: `kubectl get pods -n <namespace>`
   - Check namespace and pod name spelling

3. **Permission denied**
   - When running in cluster, ensure RBAC is properly configured
   - Check service account has necessary permissions

### Testing with Minikube

1. **Start Minikube:**
   ```bash
   minikube start
   ```

2. **Create a test pod:**
   ```bash
   kubectl run test-pod --image=nginx --restart=Never
   ```

3. **Test the API:**
   ```bash
   curl "http://localhost:8080/logs?namespace=default&pod=test-pod"
   ```

## Development

### Project Structure
```
k8s-log-tool/
├── main.go          # Main application code
├── go.mod           # Go module definition
├── go.sum           # Dependency checksums
├── Dockerfile       # Docker build configuration
├── deployment.yaml  # Kubernetes deployment manifests
└── README.md        # This file
```

### Contributing

1. Ensure code follows Go conventions
2. Test locally with `go run main.go`
3. Test containerized version
4. Verify Kubernetes deployment works

## License

This project is part of the AgentCert repository and follows the same licensing terms.