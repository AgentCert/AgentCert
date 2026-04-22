# Helm Deployment Integration - Implementation Guide

This document outlines all the changes required to make the Helm-based agent deployment fully functional.

## Overview

The implementation adds the ability to deploy AI agents to Kubernetes using Helm charts directly from the UI. When a user uploads a Helm chart (values.yaml or .tgz) and clicks "Onboard", the system:
1. Parses the chart values to extract agent metadata
2. Deploys the Helm chart to the Kubernetes cluster
3. Registers the agent in the Agent Registry
4. Returns the deployment status to the UI

## Prerequisites

### Required Tools
- **Go 1.24+** - Already installed
- **Helm CLI v3.14+** - For local testing
- **kubectl** - For Kubernetes access
- **Kind cluster** - Already running (kind-control-plane, kind-worker)

### Kubernetes Access
Ensure the GraphQL server has access to the Kubernetes cluster:
- For local development: Uses `~/.kube/config` or `KUBECONFIG` env var
- For in-cluster: Uses ServiceAccount with appropriate RBAC

## Implementation Steps

### Step 1: Install Go Dependencies

Navigate to the GraphQL server directory and install the Helm SDK:

```powershell
cd c:\Users\sharmadeep\AgentCert\chaoscenter\graphql\server
go get helm.sh/helm/v3@v3.14.0
go get k8s.io/cli-runtime@v0.28.0
go mod tidy
```

### Step 2: Regenerate GraphQL Models

After adding the `deployAgentWithHelm` mutation to the schema, regenerate the Go models:

```powershell
cd c:\Users\sharmadeep\AgentCert\chaoscenter\graphql\server
go run github.com/99designs/gqlgen generate
```

This will:
- Generate `HelmDeploymentInput` struct in `graph/model/models_gen.go`
- Generate `HelmDeploymentResponse` struct
- Generate `HelmContentType` enum
- Create resolver stubs in `graph/agent_registry.resolvers.go`

### Step 3: Verify Generated Models

After regeneration, verify that these types exist in `graph/model/models_gen.go`:

```go
type HelmDeploymentInput struct {
    ProjectID      string           `json:"projectID"`
    ReleaseName    string           `json:"releaseName"`
    Namespace      string           `json:"namespace"`
    ChartContent   string           `json:"chartContent"`
    ContentType    HelmContentType  `json:"contentType"`
    ChartPath      *string          `json:"chartPath,omitempty"`
    OverrideValues *string          `json:"overrideValues,omitempty"`
}

type HelmDeploymentResponse struct {
    Success        bool    `json:"success"`
    ReleaseName    string  `json:"releaseName"`
    Namespace      string  `json:"namespace"`
    Agent          *Agent  `json:"agent,omitempty"`
    Message        string  `json:"message"`
    ManifestOutput *string `json:"manifestOutput,omitempty"`
}

type HelmContentType string

const (
    HelmContentTypeArchive    HelmContentType = "ARCHIVE"
    HelmContentTypeValuesOnly HelmContentType = "VALUES_ONLY"
    HelmContentTypeLocalPath  HelmContentType = "LOCAL_PATH"
)
```

### Step 4: Build and Test

```powershell
cd c:\Users\sharmadeep\AgentCert\chaoscenter\graphql\server
go build -o server.exe .
```

### Step 5: Configure RBAC for Helm Deployments

The GraphQL server's ServiceAccount needs permissions to create Kubernetes resources. Create or update the RBAC:

```yaml
# helm-deployer-rbac.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: helm-deployer
rules:
- apiGroups: ["", "apps", "batch", "extensions"]
  resources: ["*"]
  verbs: ["*"]
- apiGroups: ["rbac.authorization.k8s.io"]
  resources: ["roles", "rolebindings", "clusterroles", "clusterrolebindings"]
  verbs: ["*"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: helm-deployer-binding
subjects:
- kind: ServiceAccount
  name: litmus-server-account
  namespace: litmus
roleRef:
  kind: ClusterRole
  name: helm-deployer
  apiGroup: rbac.authorization.k8s.io
```

Apply with:
```powershell
kubectl apply -f helm-deployer-rbac.yaml
```

## Files Changed

### GraphQL Schema
- `chaoscenter/graphql/definitions/shared/agent_registry.graphqls`
  - Added `deployAgentWithHelm` mutation
  - Added `HelmDeploymentInput` input type
  - Added `HelmDeploymentResponse` type
  - Added `HelmContentType` enum

### Backend (Go)
- `chaoscenter/graphql/server/go.mod`
  - Added `helm.sh/helm/v3` dependency
  - Added `k8s.io/cli-runtime` dependency
  - Added `gopkg.in/yaml.v3` dependency

- `chaoscenter/graphql/server/pkg/helm/executor.go` (NEW)
  - Helm SDK wrapper with `Deploy()`, `Uninstall()`, `ListReleases()` methods
  - Chart loading from archive, values, or local path
  - Agent values parsing from YAML

- `chaoscenter/graphql/server/pkg/agent_registry/service.go`
  - Added `helmExecutor` field to `serviceImpl`
  - Implemented `DeployHelmChart()` using Helm SDK
  - Implemented `ExtractAgentFromHelmValues()` for parsing agent config

- `chaoscenter/graphql/server/pkg/agent_registry/handler.go`
  - Added `DeployAgentWithHelm()` handler method

- `chaoscenter/graphql/server/graph/agent_registry.resolvers.go`
  - Added `DeployAgentWithHelm()` resolver

### Frontend (TypeScript/React)
- `chaoscenter/web/src/api/core/agents/registerAgent.ts` (NEW)
  - GraphQL mutation hook for agent registration

- `chaoscenter/web/src/api/core/agents/deployAgentWithHelm.ts` (NEW)
  - GraphQL mutation hook for Helm deployment
  - File reading utilities (`fileToBase64`, `fileToText`)
  - YAML parsing for agent values

- `chaoscenter/web/src/api/core/agents/index.ts` (NEW)
  - Export aggregation

- `chaoscenter/web/src/api/core/index.ts`
  - Added agents module export

- `chaoscenter/web/src/views/AgentOnboarding/AgentOnboarding.tsx`
  - Added `useDeployAgentWithHelm` hook
  - Updated `handleFileChange` to read file content
  - Updated `handleOnboard` to call deployment API
  - Added loading state during deployment

- `chaoscenter/web/src/strings/strings.en.yaml`
  - Added deployment-related strings

## Testing

### 1. Unit Tests
```powershell
cd c:\Users\sharmadeep\AgentCert\chaoscenter\graphql\server
go test ./pkg/helm/... -v
go test ./pkg/agent_registry/... -v
```

### 2. Integration Test via GraphQL Playground

1. Open http://localhost:8080/ (GraphQL Playground)
2. Execute the mutation:

```graphql
mutation {
  deployAgentWithHelm(input: {
    projectID: "default-project"
    releaseName: "test-agent"
    namespace: "default"
    chartContent: "YWdlbnQ6CiAgbmFtZTogdGVzdC1hZ2VudAogIHZlcnNpb246IHYxLjAuMAo="
    contentType: VALUES_ONLY
    chartPath: "./agent-chart"
  }) {
    success
    releaseName
    namespace
    message
    agent {
      agentID
      name
      status
    }
  }
}
```

### 3. End-to-End UI Test

1. Navigate to https://localhost:3000
2. Login with admin/Pa$$w0rd
3. Go to Agent Onboarding
4. Click "New Agent"
5. Select "Onboard agent using Helm Chart"
6. Upload the values.yaml from agent-chart
7. Click "Onboard"
8. Verify:
   - Loading state shows "Deploying..."
   - Success toast appears
   - Agent appears in the table
   - `kubectl get pods -n default` shows the deployed agent

### 4. Verify Helm Release

```powershell
helm list -n default
kubectl get all -n default -l app=ai-agent
```

## Troubleshooting

### "failed to initialize helm action config"
- Ensure KUBECONFIG is set or ~/.kube/config exists
- Check kubectl can access the cluster: `kubectl cluster-info`

### "chart path does not exist"
- Ensure agent-chart directory exists at the specified path
- For local dev, use absolute path or ensure working directory is correct

### "authentication required"
- Ensure JWT token is passed in the Authorization header
- Check that the user has Owner/Editor role on the project

### GraphQL schema mismatch
- Regenerate models: `go run github.com/99designs/gqlgen generate`
- Restart the GraphQL server

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Frontend (React)                         │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │              AgentOnboarding.tsx                        │    │
│  │  - Upload helm chart (values.yaml or .tgz)              │    │
│  │  - Call deployAgentWithHelm mutation                    │    │
│  │  - Show deployment status                               │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    GraphQL Server (Go)                           │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │              agent_registry.resolvers.go                │    │
│  │  - DeployAgentWithHelm resolver                         │    │
│  └─────────────────────────────────────────────────────────┘    │
│                              │                                   │
│                              ▼                                   │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │              agent_registry/handler.go                  │    │
│  │  - Authorization validation                             │    │
│  │  - Call service layer                                   │    │
│  └─────────────────────────────────────────────────────────┘    │
│                              │                                   │
│                              ▼                                   │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │              agent_registry/service.go                  │    │
│  │  - DeployHelmChart: Execute helm deployment             │    │
│  │  - ExtractAgentFromHelmValues: Parse agent config       │    │
│  │  - RegisterAgent: Create agent in DB                    │    │
│  └─────────────────────────────────────────────────────────┘    │
│                              │                                   │
│                              ▼                                   │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    helm/executor.go                     │    │
│  │  - Deploy(): Install/upgrade Helm release               │    │
│  │  - LoadChart(): Load from archive, values, or path      │    │
│  │  - ParseAgentValues(): Extract agent metadata           │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                            │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Helm Release: test-agent                               │    │
│  │  - Deployment                                           │    │
│  │  - Service                                              │    │
│  │  - ConfigMap                                            │    │
│  │  - ServiceAccount, Role, RoleBinding                    │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

## Next Steps

1. **Add Helm chart validation** - Validate chart structure before deployment
2. **Add rollback support** - Allow rolling back failed deployments
3. **Add deployment progress tracking** - Stream deployment logs to UI
4. **Add chart repository support** - Pull charts from Helm repositories
5. **Add secrets management** - Handle sensitive values securely
