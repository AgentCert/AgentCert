# Install-Agent to Flash-Agent Helm Deployment Flow

## Executive Summary
The install-agent (running as the litmusportal-server GraphQL backend) deploys flash-agent via Helm by reading environment variables from the server pod's environment and passing them through the Helm chart using `--set` and `--set-string` parameters.

---

## 1. Environment Variable Sources: Where Values Originate

### Source: Server Pod Environment Variables
File: [build-flash-agent.sh](build-flash-agent.sh#L30-L78)

The build-flash-agent.sh script syncs these values from `.env` file to the litmusportal-server deployment:

```bash
kubectl set env deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" \
  FLASH_AGENT_IMAGE="${IMAGE}" \
  LITELLM_MASTER_KEY="${litellm_master_key}" \
  OPENAI_API_KEY="${openai_api_key}" \
  OPENAI_BASE_URL="${openai_base_url}" \
  K8S_MCP_URL="${k8s_mcp_url}" \
  PROM_MCP_URL="${prom_mcp_url}" \
  CHAOS_NAMESPACE="${chaos_namespace}"
```

**Default Values** (from [build-flash-agent.sh](build-flash-agent.sh#L45-L63)):
- `LITELLM_MASTER_KEY`: `sk-litellm-local-dev` (if not set)
- `OPENAI_API_KEY`: Falls back to `LITELLM_MASTER_KEY` value
- `OPENAI_BASE_URL`: `http://litellm-proxy.litellm.svc.cluster.local:4000/v1`
- `K8S_MCP_URL`: `http://kubernetes-mcp-server.litmus-exp.svc.cluster.local:8081/mcp`
- `PROM_MCP_URL`: `http://prometheus-mcp-server.litmus-exp.svc.cluster.local:9090/mcp`
- `CHAOS_NAMESPACE`: `litmus-exp`

### Source: GraphQL Server Configuration (.env file)
File: [local-custom/config/.env](local-custom/config/.env)

Example configuration keys:
```
LITELLM_MASTER_KEY=sk-xxx...
OPENAI_API_KEY=sk-proj-xxx...
K8S_MCP_URL=http://kubernetes-mcp-server.litmus-exp.svc.cluster.local:8081/mcp
PROM_MCP_URL=http://prometheus-mcp-server.litmus-exp.svc.cluster.local:9090/mcp
CHAOS_NAMESPACE=litmus-exp
OPENAI_BASE_URL=http://litellm-proxy.litellm.svc.cluster.local:4000/v1
```

---

## 2. How Flash-Agent Helm Chart Receives These Values

### Helm Deployment Mechanism
File: [chaoscenter/graphql/server/pkg/agent_registry/helm.go](chaoscenter/graphql/server/pkg/agent_registry/helm.go)

#### Helm Install/Upgrade Command Line
```bash
helm upgrade --install <releaseName> <chartPath> \
  --namespace <namespace> \
  --create-namespace \
  --wait \
  --timeout <timeout> \
  --atomic \
  --cleanup-on-fail \
  --set agentId=<agentID> \
  --set image.tag=<imageTag> \
  --set-string secrets.azureOpenaiKey=<value> \
  --set configMap.AZURE_OPENAI_ENDPOINT=<value> \
  --set configMap.AZURE_OPENAI_DEPLOYMENT=<value> \
  --set configMap.AZURE_OPENAI_API_VERSION=<value> \
  --set configMap.AZURE_OPENAI_EMBEDDING_DEPLOYMENT=<value>
```

### Where Helm Parameters Are Constructed
File: [chaoscenter/graphql/server/pkg/agent_registry/helm.go#L220-L260](chaoscenter/graphql/server/pkg/agent_registry/helm.go#L220-L260)

**Key Implementation Pattern:**

```go
// Lines 220-260 in helm.go
args := []string{
    "upgrade", "--install",
    req.ReleaseName,
    chartPath,
    "--namespace", req.Namespace,
    "--create-namespace",
    "--wait",
    "--timeout", timeout,
    "--atomic",
    "--cleanup-on-fail",
}

// Set agentId and optional image tag
args = append(args, "--set", fmt.Sprintf("agentId=%s", req.AgentID))
if req.ImageTag != nil && strings.TrimSpace(*req.ImageTag) != "" {
    args = append(args, "--set", fmt.Sprintf("image.tag=%s", *req.ImageTag))
}

// Pass Azure OpenAI values using --set or --set-string parameters
if req.AzureOpenAIKey != nil && strings.TrimSpace(*req.AzureOpenAIKey) != "" {
    args = append(args, "--set-string", fmt.Sprintf("secrets.azureOpenaiKey=%s", *req.AzureOpenAIKey))
}
if req.AzureOpenAIEndpoint != nil && strings.TrimSpace(*req.AzureOpenAIEndpoint) != "" {
    args = append(args, "--set", fmt.Sprintf("configMap.AZURE_OPENAI_ENDPOINT=%s", *req.AzureOpenAIEndpoint))
}
// ... additional environment variables ...
```

---

## 3. GraphQL API Flow: DeployAgentWithHelm Mutation

### Mutation Input
File: [chaoscenter/graphql/server/graph/agent_registry.resolvers.go#L120-L230](chaoscenter/graphql/server/graph/agent_registry.resolvers.go#L120-L230)

The `DeployAgentWithHelm` mutation:
1. **Accepts request from UI** with optional Azure OpenAI credentials
2. **Falls back to server environment variables** if not provided:

```go
// Lines 164-184 in agent_registry.resolvers.go
if request.AzureOpenAIKey != nil && !isMasked(*request.AzureOpenAIKey) {
    deployReq.AzureOpenAIKey = request.AzureOpenAIKey
} else if envVal := os.Getenv("AZURE_OPENAI_KEY"); envVal != "" {
    deployReq.AzureOpenAIKey = &envVal
}

if request.AzureOpenAIEndpoint != nil {
    deployReq.AzureOpenAIEndpoint = request.AzureOpenAIEndpoint
} else if envVal := os.Getenv("AZURE_OPENAI_ENDPOINT"); envVal != "" {
    deployReq.AzureOpenAIEndpoint = &envVal
}
// ... similar pattern for other OpenAI configs ...
```

3. **Registers agent** with the platform
4. **Calls DeployWithHelm()** with the constructed request
5. **Passes Helm values** via `--set` parameters

### Data Flow Diagram
```
┌─────────────────────────────────────────────────────────────┐
│ Client / UI                                                 │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       │ GraphQL: deployAgentWithHelm(request {
                       │   azureOpenAIKey?, azureOpenAIEndpoint?, ...
                       │ })
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│ GraphQL Server (litmusportal-server)                        │
│ [chaoscenter/graphql/server/graph/agent_registry.resolvers] │
│                                                             │
│ 1. Read environment variables (os.Getenv):                │
│    - AZURE_OPENAI_KEY                                      │
│    - AZURE_OPENAI_ENDPOINT                                 │
│    - AZURE_OPENAI_DEPLOYMENT                               │
│    - AZURE_OPENAI_API_VERSION                              │
│    - AZURE_OPENAI_EMBEDDING_DEPLOYMENT                     │
│                                                             │
│ 2. Use request values if provided, else use env vars       │
│                                                             │
│ 3. Build HelmDeployRequest struct                          │
│    - AzureOpenAIKey                                        │
│    - AzureOpenAIEndpoint                                   │
│    - AzureOpenAIDeployment                                 │
│    - AzureOpenAIAPIVersion                                 │
│    - AzureOpenAIEmbeddingDeployment                        │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       │ Pass HelmDeployRequest
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│ Helm Deployment Handler                                    │
│ [agent_registry/helm.go:DeployWithHelm()]                 │
│                                                             │
│ Builds helm command with --set parameters:                │
│  helm upgrade --install <release> <chart> \               │
│    --namespace <ns> \                                      │
│    --set agentId=<agentID> \                              │
│    --set configMap.AZURE_OPENAI_KEY=<value> \            │
│    --set-string secrets.azureOpenaiKey=<value> \         │
│    --set configMap.AZURE_OPENAI_ENDPOINT=<value> \       │
│    ... (etc)                                               │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       │ Execute helm command
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│ Kubernetes Cluster / Flash-Agent Deployment                │
│ [agent-chart/templates/configmap.yaml]                    │
│ [agent-chart/templates/deployment.yaml]                   │
│                                                             │
│ ConfigMap (agent-metadata):                               │
│   agent.config.AZURE_OPENAI_*=<helm set values>          │
│                                                             │
│ Secret (agent-secret):                                     │
│   AZURE_OPENAI_KEY=<from --set-string>                   │
│                                                             │
│ Deployment references these via valueFrom:                │
│   env:                                                      │
│   - name: AZURE_OPENAI_ENDPOINT                           │
│     valueFrom:                                             │
│       configMapKeyRef:                                     │
│         name: agent-metadata                               │
│         key: AZURE_OPENAI_ENDPOINT                        │
└─────────────────────────────────────────────────────────────┘
```

---

## 4. Agent Helm Chart Configuration

### Values Template
File: [agent-chart/values.yaml](agent-chart/values.yaml#L30-L70)

```yaml
agent:
  config:
    # LiteLLM Proxy
    LITELLM_URL: ""
    MODEL_ALIAS: ""
    OPENAI_BASE_URL: ""
    OPENAI_API_KEY: ""
    # Experiment context (injected by install-agent via Argo template variables)
    EXPERIMENT_ID: ""
    EXPERIMENT_RUN_ID: ""
    WORKFLOW_NAME: ""

  secret:
    LITELLM_MASTER_KEY: ""

  deploymentEnv:
    - name: OPENAI_BASE_URL
      key: OPENAI_BASE_URL
    - name: OPENAI_API_KEY
      key: OPENAI_API_KEY
    - name: EXPERIMENT_ID
      key: EXPERIMENT_ID
    - name: EXPERIMENT_RUN_ID
      key: EXPERIMENT_RUN_ID
    - name: WORKFLOW_NAME
      key: WORKFLOW_NAME
```

### ConfigMap Template
File: [agent-chart/templates/configmap.yaml](agent-chart/templates/configmap.yaml)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Values.agent.name }}-metadata
data:
  agent-metadata.json: |
    {
      "name": "{{ .Values.agent.name }}",
      "version": "{{ .Values.agent.version }}",
      ...
    }
{{ toYaml .Values.agent.config | nindent 2 }}
```

### Deployment Template
File: [agent-chart/templates/deployment.yaml](agent-chart/templates/deployment.yaml#L55-L95)

```yaml
env:
  # ... other env vars ...
  
  {{- if .Values.agent.config.LITELLM_URL }}
  - name: LITELLM_URL
    valueFrom:
      configMapKeyRef:
        name: {{ .Values.agent.name }}-metadata
        key: LITELLM_URL
  {{- end }}

  {{- if .Values.agent.secret.LITELLM_MASTER_KEY }}
  - name: LITELLM_MASTER_KEY
    valueFrom:
      secretKeyRef:
        name: {{ .Values.agent.name }}-secret
        key: LITELLM_MASTER_KEY
  {{- end }}

  {{- range $env := .Values.agent.deploymentEnv }}
  - name: {{ $env.name }}
    valueFrom:
      configMapKeyRef:
        name: {{ $.Values.agent.name }}-metadata
        key: {{ $env.key }}
  {{- end }}
```

---

## 5. Summary: Parameter Passing Methods

### Method 1: Via ConfigMap (Non-Sensitive)
- **Used for**: OPENAI_API_KEY, OPENAI_BASE_URL, K8S_MCP_URL, PROM_MCP_URL, CHAOS_NAMESPACE, OPENAI_MODEL, etc.
- **Helm Set**: `--set agent.config.<VAR_NAME>=<value>`
- **Chart Storage**: ConfigMap named `<releaseName>-metadata`
- **Pod Reference**: `valueFrom.configMapKeyRef`
- **Pattern**: `{{ .Values.agent.config.<VAR_NAME> }}`

### Method 2: Via Secret (Sensitive)
- **Used for**: LITELLM_MASTER_KEY, AZURE_OPENAI_KEY, etc.
- **Helm Set**: `--set-string agent.secret.<VAR_NAME>=<value>`
- **Chart Storage**: Secret named `<releaseName>-secret`
- **Pod Reference**: `valueFrom.secretKeyRef`
- **Pattern**: `{{ .Values.agent.secret.<VAR_NAME> }}`

### Method 3: Environment-Injected (Server Pod Defaults)
- **Source**: Server deployment environment variables (set via `kubectl set env`)
- **Read by**: GraphQL resolver (`os.Getenv()`)
- **Available in**: `GetEnvironmentVariables` query resolver
- **Passed to Helm**: As `--set` or `--set-string` parameters

---

## 6. Example: Complete Flow for LITELLM_MASTER_KEY

```
1. Local Oper Setup Phase
   └─ Edit .env file
      LITELLM_MASTER_KEY=sk-my-key-123

2. Sync to Server Pod
   └─ Run: build-flash-agent.sh
      └─ Reads LITELLM_MASTER_KEY from .env
      └─ kubectl set env deployment/litmusportal-server ... LITELLM_MASTER_KEY=sk-my-key-123

3. Server Pod Environment
   └─ litmusportal-server container now has env var:
      LITELLM_MASTER_KEY=sk-my-key-123

4. GraphQL Mutation (DeployAgentWithHelm)
   └─ Client calls: deployAgentWithHelm(request { ... })
   └─ Resolver reads: os.Getenv("LITELLM_MASTER_KEY") → sk-my-key-123
   └─ Builds HelmDeployRequest with this value

5. Helm Execution
   └─ helm upgrade --install myagent ./agent-chart \
        --namespace myns \
        --set-string agent.secret.LITELLM_MASTER_KEY=sk-my-key-123

6. Kubernetes Resources Created
   └─ Secret: myagent-secret
      data:
        LITELLM_MASTER_KEY: c2stbXkta2V5LTEyMyA=  (base64)
   
   └─ Deployment: myagent
      containers:
      - env:
        - name: LITELLM_MASTER_KEY
          valueFrom:
            secretKeyRef:
              name: myagent-secret
              key: LITELLM_MASTER_KEY
```

---

## 7. Related Files

**Helm Chart Templates:**
- [agent-chart/Chart.yaml](agent-chart/Chart.yaml) - Chart metadata
- [agent-chart/values.yaml](agent-chart/values.yaml) - Default values
- [agent-chart/templates/configmap.yaml](agent-chart/templates/configmap.yaml) - ConfigMap template
- [agent-chart/templates/deployment.yaml](agent-chart/templates/deployment.yaml) - Deployment template
- [agent-chart/templates/role.yaml](agent-chart/templates/role.yaml) - RBAC role
- [agent-chart/templates/rolebinding.yaml](agent-chart/templates/rolebinding.yaml) - Role binding
- [agent-chart/templates/serviceaccount.yaml](agent-chart/templates/serviceaccount.yaml) - Service account

**Build & Sync Scripts:**
- [build-flash-agent.sh](build-flash-agent.sh) - Builds flash-agent image & syncs env to server
- [build-litellm.sh](build-litellm.sh) - Deploys LiteLLM proxy & syncs env to server
- [build-install-agent.sh](build-install-agent.sh) - Builds install-agent (server) image

**GraphQL Server Code:**
- [chaoscenter/graphql/server/graph/agent_registry.resolvers.go](chaoscenter/graphql/server/graph/agent_registry.resolvers.go) - GraphQL resolvers for Helm deployment
- [chaoscenter/graphql/server/pkg/agent_registry/helm.go](chaoscenter/graphql/server/pkg/agent_registry/helm.go) - Helm deployment logic
- [chaoscenter/graphql/server/pkg/agent_registry/service.go](chaoscenter/graphql/server/pkg/agent_registry/service.go) - Agent registry service

**Configuration:**
- [local-custom/config/.env](local-custom/config/.env) - Environment configuration
