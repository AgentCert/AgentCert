````markdown
# MongoDB Single-Node Replica Set Setup Instructions

This guide outlines the steps to deploy a MongoDB 5.0 container configured as a single-node replica set with admin authentication.

## 1. Environment Preparation
### Navigate to your project workspace and ensure you have the latest MongoDB 5 image.
```
docker pull mongo:5
```

### Clean up the resources
```
docker stop mongo \
docker system prune -y
```

### Deploy the Container
```
docker run -d \
  --name simple-mongo \
  -p 27017:27017 \
  mongo:5 mongod --replSet rs0 --bind_ip_all
```

### Initialize the Replica Set
### Access the MongoDB shell to initiate the cluster configuration
```
docker exec -it simple-mongo mongosh
```
### Inside the shell, run:
```
rs.initiate({
  _id: "rs0",
  members: [{ _id: 0, host: "localhost:27017" }]
})
```
### Create a root administrative user.
```
db.getSiblingDB("admin").createUser({
  user: "admin",
  pwd: "1234",
  roles: [{ role: "root", db: "admin" }]
})

exit()
```

### Verify Connection
### Test the administrative access using the credentials created in the previous step.
```
docker exec -it simple-mongo mongosh -u admin -p 1234 --authenticationDatabase admin
```

# Start the auth services
```
export DB_USER=admin                               
export DB_PASSWORD=1234
export DB_SERVER="mongodb://$DB_USER:$DB_PASSWORD@localhost:27017/?replicaSet=rs0&authSource=admin"
export JWT_SECRET=litmus-portal@123
export PORTAL_ENDPOINT=http://localhost:8080
export LITMUS_SVC_ENDPOINT=""
export SELF_AGENT=false
export INFRA_SCOPE=cluster
export INFRA_NAMESPACE=litmus
export LITMUS_PORTAL_NAMESPACE=litmus
export PORTAL_SCOPE=namespace
export SUBSCRIBER_IMAGE=agentcert/litmusportal-subscriber:3.0.0
export EVENT_TRACKER_IMAGE=litmuschaos/litmusportal-event-tracker:ci
export CONTAINER_RUNTIME_EXECUTOR=k8sapi
export ARGO_WORKFLOW_CONTROLLER_IMAGE=argoproj/workflow-controller:v2.11.0
export ARGO_WORKFLOW_EXECUTOR_IMAGE=argoproj/argoexec:v2.11.0
export CHAOS_CENTER_SCOPE=cluster
export WORKFLOW_HELPER_IMAGE_VERSION=3.0.0
export LITMUS_CHAOS_OPERATOR_IMAGE=litmuschaos/chaos-operator:3.0.0
export LITMUS_CHAOS_RUNNER_IMAGE=litmuschaos/chaos-runner:3.0.0
export LITMUS_CHAOS_EXPORTER_IMAGE=litmuschaos/chaos-exporter:3.0.0
export ADMIN_USERNAME=admin
export ADMIN_PASSWORD=litmus
export VERSION=ci
export HUB_BRANCH_NAME=v2.0.x
export INFRA_DEPLOYMENTS="[\"app=chaos-exporter\", \"name=chaos-operator\", \"app=event-tracker\",\"app=workflow-controller\"]"
export INFRA_COMPATIBLE_VERSIONS='["0.2.0", "0.1.0","ci"]'
export DEFAULT_HUB_BRANCH_NAME=master
export REST_PORT=3000                              
export GRPC_PORT=3030
export ALLOWED_ORIGINS=".*" 
```

### Run the service
```
cd chaoscenter/authentication/api
go run main.go  
```

# Start the graphql server
```
export DB_USER=admin                               
export DB_PASSWORD=1234
export DB_SERVER="mongodb://$DB_USER:$DB_PASSWORD@localhost:27017/?replicaSet=rs0&authSource=admin"
export JWT_SECRET=litmus-portal@123
export PORTAL_ENDPOINT=http://localhost:8080
export LITMUS_SVC_ENDPOINT=""
export SELF_AGENT=false
export INFRA_SCOPE=cluster
export INFRA_NAMESPACE=litmus
export LITMUS_PORTAL_NAMESPACE=litmus
export PORTAL_SCOPE=namespace
export SUBSCRIBER_IMAGE=agentcert/litmusportal-subscriber:3.0.0
export EVENT_TRACKER_IMAGE=litmuschaos/litmusportal-event-tracker:ci
export CONTAINER_RUNTIME_EXECUTOR=k8sapi
export ARGO_WORKFLOW_CONTROLLER_IMAGE=argoproj/workflow-controller:v2.11.0
export ARGO_WORKFLOW_EXECUTOR_IMAGE=argoproj/argoexec:v2.11.0
export CHAOS_CENTER_SCOPE=cluster
export WORKFLOW_HELPER_IMAGE_VERSION=3.0.0
export LITMUS_CHAOS_OPERATOR_IMAGE=litmuschaos/chaos-operator:3.0.0
export LITMUS_CHAOS_RUNNER_IMAGE=litmuschaos/chaos-runner:3.0.0
export LITMUS_CHAOS_EXPORTER_IMAGE=litmuschaos/chaos-exporter:3.0.0
export ADMIN_USERNAME=admin
export ADMIN_PASSWORD=litmusexport VERSION=ci
export HUB_BRANCH_NAME=v2.0.x
export INFRA_DEPLOYMENTS="["app=chaos-exporter", "name=chaos-operator", "app=event-tracker","app=workflow-controller"]"export INFRA_COMPATIBLE_VERSIONS='["0.2.0", "0.1.0","ci"]'
export DEFAULT_HUB_BRANCH_NAME=master
export ALLOWED_ORIGINS=".*"
```

### Run the service
```
cd chaoscenter/graphql/server
go run server.go  
```

# Start the UI
```
cd chaoscenter/web
yarn
yarn dev
```

# Start Minikube
```
minikube start --cpus=2 --memory=4096 --driver=docker
```

# Initialise the environment, setup chaos with cluster level access, download the yaml file
## alter the SERVER_ADDRESS: http://<System IP address>:8080/query
```
kubectl apply -f <yaml file>
```

## Verify if all pods are running
```
kubectl get po -A
```
## Issue: RBAC Permission Denied (Namespace Creation)
### The Litmus Chaos workflow failed because the argo-chaos ServiceAccount lacked sufficient permissions to create the sock-shop namespace at the cluster scope.
### Grant the ServiceAccount the necessary cluster-wide permissions by creating a ClusterRoleBinding using the kubectl command-line tool:
```
kubectl create clusterrolebinding argo-chaos-cluster-admin \
  --clusterrole=cluster-admin \
  --serviceaccount=litmus:argo-chaos
```

## Onboard the chaos to the choashub from https://github.com/AgentCert/chaos-charts.git



## LiteLLM Setup

### Postgres Container: This container will use port 5433 on your host.
```
docker run --name litellm-postgres \
  -e POSTGRES_USER=litellm_user \
  -e POSTGRES_PASSWORD=litellm_pass \
  -e POSTGRES_DB=litellm_db \
  -p 5433:5432 \
  -d postgres:17
```

### Update your litellm-config.yaml
```
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

LiteLLM provides a unified Python SDK and AI Gateway (proxy) to call 100+ LLMs (OpenAI, Anthropic, Azure, etc.) with an OpenAI-compatible API. Complete docs: https://docs.litellm.ai/

### 1. Install LiteLLM

```bash
pip install 'litellm[proxy]'
pip install prisma
pip install opentelemetry-api opentelemetry-sdk opentelemetry-exporter-otlp
pip install langfuse
prisma generate --schema <PATH>/.venv/lib/python3.10/site-packages/litellm/proxy/schema.prismaa
```

### 2. Start LiteLLM Proxy (OpenAI-compatible endpoint on port 4000) 

```bash
# Start proxy with a default model
litellm --model gpt-4o --port 4000
```

### 3. Verify LiteLLM is Running

```bash
curl http://localhost:4000/v1/models
```

### 4. Test with Python

```python
from litellm import completion
import os
os.environ['OPENAI_API_KEY'] = 'your-openai-key'
response = completion(
    model='gpt-4o',
    messages=[{'role': 'user', 'content': 'Hello!'}],
    api_base='http://localhost:4000'
)
print(response['choices'][0]['message']['content'])
```

---

## Langfuse Setup

Langfuse is an open-source LLM observability platform. Docs: https://docs.langfuse.com/

### Option 1: Langfuse Cloud (Quick Start)

1. Go to https://cloud.langfuse.com
2. Sign up and create a project
3. Get API keys from Settings > API Keys

```bash
export LANGFUSE_PUBLIC_KEY=pk-lf-xxxxxxxx
export LANGFUSE_SECRET_KEY=sk-lf-xxxxxxxx
export LANGFUSE_HOST=https://cloud.langfuse.com
```


## Complete Environment Variables

```bash
# LiteLLM
export LITELLM_URL=http://localhost:4000
export MODEL_ALIAS=gpt-4o
export OPENAI_API_KEY=your-openai-key

# Langfuse
export LANGFUSE_PUBLIC_KEY=pk-lf-xxxxxxxx
export LANGFUSE_SECRET_KEY=sk-lf-xxxxxxxx
export LANGFUSE_HOST=https://cloud.langfuse.com
```

````
