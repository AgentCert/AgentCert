# AgentCert — Local Development Setup Guide

A step-by-step guide to clone, configure, and run the **AgentCert** platform on your local machine.

---

## Table of Contents

1. [Prerequisites](#1-prerequisites)
2. [Clone the Repository](#2-clone-the-repository)
3. [Set Up MongoDB](#3-set-up-mongodb)
4. [Start the Authentication Service](#4-start-the-authentication-service)
5. [Start the GraphQL Server](#5-start-the-graphql-server)
6. [Start the Frontend](#6-start-the-frontend)
7. [Set Up Minikube (Kubernetes)](#7-set-up-minikube-kubernetes)
8. [Set Up LiteLLM (LLM Gateway)](#8-set-up-litellm-llm-gateway)
9. [Set Up Langfuse (LLM Observability)](#9-set-up-langfuse-llm-observability)
10. [Quick Start (Automated)](#10-quick-start-automated)
11. [Verify Everything Works](#11-verify-everything-works)
12. [Stopping Services](#12-stopping-services)
13. [Troubleshooting](#13-troubleshooting)
14. [Port Reference](#14-port-reference)

---

## 1. Prerequisites

Install the following tools before proceeding:

| Tool       | Version      | Installation                                                       |
| ---------- | ------------ | ------------------------------------------------------------------ |
| **Git**    | 2.x+         | `sudo apt install git` (Ubuntu) / `brew install git` (macOS)      |
| **Docker** | 20.x+        | [Install Docker](https://docs.docker.com/engine/install/)          |
| **Go**     | 1.24.0+      | [Install Go](https://go.dev/doc/install)                           |
| **Node.js**| 18.x (LTS)   | [Install Node.js](https://nodejs.org/) or use `nvm install 18`    |
| **Yarn**   | 1.22+        | `npm install -g yarn`                                              |
| **Python** | 3.10+        | [Install Python](https://www.python.org/downloads/)                |
| **Minikube** | latest     | [Install Minikube](https://minikube.sigs.k8s.io/docs/start/)      |
| **kubectl** | latest      | [Install kubectl](https://kubernetes.io/docs/tasks/tools/)         |

**Verify installations:**

```bash
git --version
docker --version
go version
node --version
yarn --version
python3 --version
minikube version
kubectl version --client
```

---

## 2. Clone the Repository

```bash
git clone https://github.com/AgentCert/AgentCert.git
cd AgentCert
```

---

## 3. Set Up MongoDB

AgentCert uses MongoDB 5.0 as a single-node replica set with admin authentication.

### 3.1 Pull the MongoDB image

```bash
docker pull mongo:5
```

### 3.2 Clean up any previous MongoDB containers (optional)

```bash
docker stop mongo \
docker system prune -y
```

### 3.3 Start the MongoDB container

```bash
docker run -d \
  --name simple-mongo \
  -p 27017:27017 \
  mongo:5 mongod --replSet rs0 --bind_ip_all
```

### 3.4 Initialize the replica set

```bash
docker exec -it simple-mongo mongosh
```

Inside the MongoDB shell, run:

```javascript
rs.initiate({
  _id: "rs0",
  members: [{ _id: 0, host: "localhost:27017" }]
})
```

### 3.5 Create the admin user

Still in the MongoDB shell:

```javascript
db.getSiblingDB("admin").createUser({
  user: "admin",
  pwd: "1234",
  roles: [{ role: "root", db: "admin" }]
})
```

Type `exit` to leave the shell.

### 3.6 Verify the connection

```bash
docker exec -it simple-mongo mongosh -u admin -p 1234 --authenticationDatabase admin
```

If you see the MongoDB prompt, your database is ready. Type `exit` to leave.

---

## 4. Start the Authentication Service

The Auth Service is a Go-based service that handles user authentication, JWT tokens, and gRPC.

### 4.1 Set environment variables

Open a new terminal and run:

```bash
export VERSION="3.0.0"
export INFRA_DEPLOYMENTS="false"
export DB_SERVER="mongodb://localhost:27017"
export JWT_SECRET="litmus-portal@123"
export DB_USER="admin"
export DB_PASSWORD="1234"
export SELF_AGENT="false"
export INFRA_COMPATIBLE_VERSIONS='["3.0.0"]'
export ALLOWED_ORIGINS=".*"
export SKIP_SSL_VERIFY="true"
export ENABLE_GQL_INTROSPECTION="true"
export INFRA_SCOPE="cluster"
export ENABLE_INTERNAL_TLS="false"
export DEFAULT_HUB_GIT_URL="https://github.com/agentcert/chaos-charts"
export DEFAULT_HUB_BRANCH_NAME="master"
export SUBSCRIBER_IMAGE="agentcert/litmusportal-subscriber:3.0.0"
export EVENT_TRACKER_IMAGE="litmuschaos/litmusportal-event-tracker:3.0.0"
export ARGO_WORKFLOW_CONTROLLER_IMAGE="litmuschaos/workflow-controller:v3.3.1"
export ARGO_WORKFLOW_EXECUTOR_IMAGE="litmuschaos/argoexec:v3.3.1"
export LITMUS_CHAOS_OPERATOR_IMAGE="litmuschaos/chaos-operator:3.0.0"
export LITMUS_CHAOS_RUNNER_IMAGE="litmuschaos/chaos-runner:3.0.0"
export LITMUS_CHAOS_EXPORTER_IMAGE="litmuschaos/chaos-exporter:3.0.0"
export CONTAINER_RUNTIME_EXECUTOR="k8sapi"
export WORKFLOW_HELPER_IMAGE_VERSION="3.0.0"
export DEFAULT_AGENT_HUB_GIT_URL="https://github.com/agentcert/agent-charts"
export DEFAULT_AGENT_HUB_BRANCH_NAME="main"
export DEFAULT_AGENT_HUB_PATH='C:/temp/default/'
export DEFAULT_APP_HUB_GIT_URL="https://github.com/agentcert/app-charts"
export DEFAULT_APP_HUB_BRANCH_NAME="main"
export DEFAULT_APP_HUB_PATH='C:/temp/default/'
export ADMIN_USERNAME="admin"
export ADMIN_PASSWORD="litmus"
export REST_PORT="3000"
export GRPC_PORT="3030"
```

### 4.2 Run the Auth Service

```bash
cd chaoscenter/authentication/api
go run main.go
```

> **Keep this terminal open.** The Auth Service runs on:
> - **REST API:** `http://localhost:3000`
> - **gRPC:** `localhost:3030`

Wait until you see logs confirming the service has started before proceeding to the next step.

---

## 5. Start the GraphQL Server

The GraphQL Server is the core API layer for AgentCert.

### 5.1 Set environment variables

Open a **new terminal** and run:

```bash
export VERSION="3.0.0"
export INFRA_DEPLOYMENTS="false"
export DB_SERVER="mongodb://localhost:27017"
export JWT_SECRET="litmus-portal@123"
export DB_USER="admin"
export DB_PASSWORD="1234"
export SELF_AGENT="false"
export INFRA_COMPATIBLE_VERSIONS='["3.0.0"]'
export ALLOWED_ORIGINS=".*"
export SKIP_SSL_VERIFY="true"
export ENABLE_GQL_INTROSPECTION="true"
export INFRA_SCOPE="cluster"
export ENABLE_INTERNAL_TLS="false"
export DEFAULT_HUB_GIT_URL="https://github.com/agentcert/chaos-charts"
export DEFAULT_HUB_BRANCH_NAME="master"
export SUBSCRIBER_IMAGE="agentcert/litmusportal-subscriber:3.0.0"
export EVENT_TRACKER_IMAGE="litmuschaos/litmusportal-event-tracker:3.0.0"
export ARGO_WORKFLOW_CONTROLLER_IMAGE="litmuschaos/workflow-controller:v3.3.1"
export ARGO_WORKFLOW_EXECUTOR_IMAGE="litmuschaos/argoexec:v3.3.1"
export LITMUS_CHAOS_OPERATOR_IMAGE="litmuschaos/chaos-operator:3.0.0"
export LITMUS_CHAOS_RUNNER_IMAGE="litmuschaos/chaos-runner:3.0.0"
export LITMUS_CHAOS_EXPORTER_IMAGE="litmuschaos/chaos-exporter:3.0.0"
export CONTAINER_RUNTIME_EXECUTOR="k8sapi"
export WORKFLOW_HELPER_IMAGE_VERSION="3.0.0"
export DEFAULT_AGENT_HUB_GIT_URL="https://github.com/agentcert/agent-charts"
export DEFAULT_AGENT_HUB_BRANCH_NAME="main"
export DEFAULT_AGENT_HUB_PATH='C:/temp/default/'
export DEFAULT_APP_HUB_GIT_URL="https://github.com/agentcert/app-charts"
export DEFAULT_APP_HUB_BRANCH_NAME="main"
export DEFAULT_AGENT_HUB_PATH='C:/temp/default/'
export LITMUS_AUTH_GRPC_ENDPOINT="localhost"
export LITMUS_AUTH_GRPC_PORT="3030"
```

### 5.2 Run the GraphQL Server

```bash
cd chaoscenter/graphql/server
go run server.go
```

> **Keep this terminal open.** The GraphQL Server runs on:
> - **REST/GraphQL:** `http://localhost:8080`
> - **gRPC:** `localhost:8082`

---

## 6. Start the Frontend

The Frontend is a React/TypeScript web application.

### 6.1 Install dependencies

Open a **new terminal** and run:

```bash
cd chaoscenter/web
yarn install
```

> First-time install may take a few minutes.

### 6.2 Start the dev server

```bash
export AUTH_PROXY_PORT=3000
yarn dev
```

> **Keep this terminal open.** The Frontend runs on:
> - **UI:** `https://localhost:2001`

### 6.3 Login

Open your browser and go to `https://localhost:2001`. Use these default credentials:

| Field    | Value    |
| -------- | -------- |
| Username | `admin`  |
| Password | `litmus` |

---

## 7. Set Up Minikube (Kubernetes)

Minikube is required for running chaos experiments on a local Kubernetes cluster.

### 7.1 Start Minikube

```bash
minikube start --cpus=2 --memory=4096 --driver=docker
```

### 7.2 Connect the infrastructure

1. Log into the AgentCert UI (`https://localhost:2001`).
2. Navigate to **Environments → Infrastructure** and create a new environment.
3. Download the generated YAML file.
4. **Find your machine's local IP** (you'll need this in the next steps):
   ```bash
   MY_IP=$(ipconfig | awk '/IPv4/ && $2!="127.0.0.1" {print $NF; exit}')
   echo "$MY_IP"
   ```

5. **Edit the YAML** — update the `SERVER_ADDRESS` field to your machine's IP:
   ```
   SERVER_ADDRESS: "http://<YOUR_LOCAL_IP>:8080/query"
   ```
   Replace `<YOUR_LOCAL_IP>` with the IP from step 4.

   > **Why is this needed?** The subscriber pod runs inside the Minikube cluster (a separate Docker network), so it cannot reach the GraphQL server via `localhost`. You must use your host machine's actual IP address so the subscriber can communicate back to the AgentCert control plane running on the host.

6. **Apply the YAML:**
   ```bash
   kubectl apply -f <downloaded-yaml-file>.yaml
   ```

7. **Patch the subscriber config** — update the server address and required components:
   ```bash
   kubectl patch configmap subscriber-config -n litmus \
     --type merge \
     -p "{\"data\":{\"SERVER_ADDR\":\"http://$MY_IP:8080/query\"}}"

   kubectl patch configmap subscriber-config -n litmus \
     --type merge \
     -p '{"data":{"COMPONENTS":"DEPLOYMENTS: \n    - app=subscriber\n    - app=event-tracker\n    - name=chaos-operator\n    - app=workflow-controller\n    - app=chaos-exporter\n"}}'
   ```

8. **Restart the subscriber** to pick up the config changes:
   ```bash
   kubectl rollout restart deployment/subscriber -n litmus
   ```

9. **Check subscriber logs** to confirm it connected successfully:
   ```bash
   kubectl logs -n litmus -l app=subscriber
   ```

### 7.3 Verify pods are running

```bash
kubectl get po -A
```

All pods in the `litmus` namespace should be in `Running` state.

### 7.4 Fix RBAC permissions (if needed)

If chaos workflows fail with permission errors, grant cluster-admin access:

```bash
kubectl create clusterrolebinding argo-chaos-cluster-admin \
  --clusterrole=cluster-admin \
  --serviceaccount=litmus:argo-chaos
```

### 7.5 Onboard ChaosHub

In the AgentCert UI, go to **ChaosHubs** and connect the chaos chart repository:

```
https://github.com/AgentCert/chaos-charts.git
```

---

## 8. Set Up LiteLLM (LLM Gateway)

LiteLLM provides a unified OpenAI-compatible API gateway for calling LLMs.

### 8.1 Start a Postgres container (LiteLLM backend)

```bash
docker run --name litellm-postgres \
  -e POSTGRES_USER=litellm_user \
  -e POSTGRES_PASSWORD=litellm_pass \
  -e POSTGRES_DB=litellm_db \
  -p 5433:5432 \
  -d postgres:17
```

### 8.2 Create a LiteLLM config file

Create a file named `litellm-config.yaml` in the project root:

```yaml
model_list:
  - model_name: gpt-4o
    litellm_params:
      model: gpt-4o

litellm_settings:
  success_callback: ["langfuse"]
  failure_callback: ["langfuse"]

general_settings:
  port: 4000
  database_url: "postgresql://litellm_user:litellm_pass@localhost:5433/litellm_db"
  master_key: sk-1234
  ui_username: admin
```

### 8.3 Install LiteLLM

```bash
pip install 'litellm[proxy]'
pip install prisma
pip install opentelemetry-api opentelemetry-sdk opentelemetry-exporter-otlp
pip install langfuse
```

### 8.4 Generate Prisma client

```bash
prisma generate --schema $(python3 -c "import litellm; print(litellm.__path__[0])")/proxy/schema.prisma
```

### 8.5 Start LiteLLM proxy

```bash
export OPENAI_API_KEY="your-openai-api-key"
litellm --config litellm-config.yaml --port 4000
```

### 8.6 Verify LiteLLM is running

```bash
curl http://localhost:4000/v1/models
```

---

## 9. Set Up Langfuse (LLM Observability)

Langfuse tracks all AI agent behavior with traces, metrics, and scoring.

### 9.1 Sign up for Langfuse Cloud

1. Go to [https://cloud.langfuse.com](https://cloud.langfuse.com)
2. Create a free account and a new project
3. Navigate to **Settings → API Keys** and copy your keys

### 9.2 Set environment variables

```bash
export LANGFUSE_PUBLIC_KEY="pk-lf-xxxxxxxx"
export LANGFUSE_SECRET_KEY="sk-lf-xxxxxxxx"
export LANGFUSE_HOST="https://cloud.langfuse.com"
```

### 9.3 Set LiteLLM + Langfuse variables together

For convenience, export all AI-related variables in one block:

```bash
export LITELLM_URL="http://localhost:4000"
export MODEL_ALIAS="gpt-4o"
export OPENAI_API_KEY="your-openai-api-key"
export LANGFUSE_PUBLIC_KEY="pk-lf-xxxxxxxx"
export LANGFUSE_SECRET_KEY="sk-lf-xxxxxxxx"
export LANGFUSE_HOST="https://cloud.langfuse.com"
```

---

## 10. Quick Start (Automated)

Instead of starting each service manually (Steps 3–6), you can use the automated startup script:

```bash
chmod +x start-agentcert.sh
./start-agentcert.sh
```

This script will:

1. Check for port conflicts (3030, 3000, 8080, 8082, 2001)
2. Start or reuse a MongoDB Docker container
3. Set all required environment variables
4. Start the Authentication Service (Go)
5. Build and start the GraphQL Server (Go)
6. Install dependencies and start the Frontend (React)
7. Wait for each service with health checks

**Options:**

```bash
./start-agentcert.sh --skip-mongo      # Skip MongoDB startup (if already running)
./start-agentcert.sh --skip-frontend   # Skip Frontend startup (backend only)
```

---

## 11. Verify Everything Works

Run these checks to confirm all services are healthy:

### Auth Service (port 3000)

```bash
curl -s http://localhost:3000/status
```

### GraphQL Server (port 8080)

```bash
curl -s http://localhost:8080
```

### Frontend (port 2001)

Open `https://localhost:2001` in your browser.

### MongoDB (port 27017)

```bash
docker exec -it simple-mongo mongosh -u admin -p 1234 --authenticationDatabase admin --eval "rs.status()"
```

### LiteLLM (port 4000)

```bash
curl -s http://localhost:4000/v1/models
```

### Kubernetes (Minikube)

```bash
kubectl get po -A
```

---

## 12. Stopping Services

### If you used the automated script:

```bash
chmod +x stop-agentcert.sh
./stop-agentcert.sh
```

> Use `./stop-agentcert.sh --keep-mongo` to keep MongoDB running.

### If you started services manually:

Press `Ctrl+C` in each terminal running a service, then stop Docker containers:

```bash
docker stop simple-mongo litellm-postgres
```

To stop Minikube:

```bash
minikube stop
```

---

## 13. Troubleshooting

### Port already in use

```bash
# Find what's using a port (e.g., 3000)
lsof -i :3000

# Kill the process
kill -9 <PID>
```

### MongoDB connection refused

```bash
# Check if the container is running
docker ps | grep mongo

# Restart it
docker start simple-mongo
```

### Go build errors

```bash
# Make sure you're using Go 1.24+
go version

# Clean module cache and retry
go clean -modcache
go mod tidy
```

### Frontend `yarn install` fails

```bash
# Make sure you're using Node 18
node --version

# Clear cache and retry
rm -rf node_modules
yarn cache clean
yarn install
```

### GraphQL Server can't connect to Auth Service

Make sure the Auth Service is running on port 3030 **before** starting the GraphQL Server:

```bash
ss -tlnp | grep 3030
```

### Minikube pod CrashLoopBackOff

```bash
# Check pod logs
kubectl logs <pod-name> -n litmus

# Describe the pod for events
kubectl describe pod <pod-name> -n litmus
```

---

## 14. Port Reference

| Service              | Port  | Protocol      |
| -------------------- | ----- | ------------- |
| MongoDB              | 27017 | TCP           |
| Auth Service (REST)  | 3000  | HTTP          |
| Auth Service (gRPC)  | 3030  | gRPC          |
| GraphQL Server       | 8080  | HTTP/GraphQL  |
| GraphQL gRPC         | 8082  | gRPC          |
| Frontend (UI)        | 2001  | HTTPS         |
| LiteLLM Proxy        | 4000  | HTTP          |
| PostgreSQL (LiteLLM) | 5433  | TCP           |

---

## Architecture Overview

```
┌──────────────┐     ┌──────────────────┐     ┌─────────────┐
│   Frontend   │────▶│  GraphQL Server  │────▶│  MongoDB    │
│  (React UI)  │     │   (Go :8080)     │     │  (:27017)   │
│  :2001       │     └──────┬───────────┘     └─────────────┘
└──────────────┘            │
                            │ gRPC
                     ┌──────▼───────────┐
                     │  Auth Service    │
                     │  (Go :3000/3030) │
                     └──────────────────┘

┌──────────────┐     ┌──────────────────┐
│  LiteLLM     │────▶│  Langfuse        │
│  (Proxy :4000)│    │  (Observability) │
└──────┬───────┘     └──────────────────┘
       │
       ▼
  LLM Provider
  (OpenAI, etc.)

┌──────────────────────────────────────────┐
│           Minikube (Kubernetes)           │
│  - Subscriber Pod                        │
│  - Chaos Operator / Runner               │
│  - Argo Workflow Controller              │
└──────────────────────────────────────────┘
```

---

**Happy Chaos Engineering!**
