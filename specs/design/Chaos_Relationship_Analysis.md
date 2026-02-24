# Chaos Experiments Flow Analysis

## Overview

This document provides a comprehensive analysis of how Chaos Experiments work in the LitmusChaos platform, including the relationships between Experiments, Environments, and Infrastructures.

---

## 1. How Chaos Experiments Flow Works

The Chaos Experiments flow follows a **multi-tier architecture**:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           FRONTEND (Web UI)                                  │
│   NewExperimentButton → ChaosStudio → ExperimentVisualBuilder               │
└──────────────────────────────────┬──────────────────────────────────────────┘
                                   │ GraphQL Mutations
                                   ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         GRAPHQL SERVER (Backend)                             │
│   chaos_experiment.resolvers.go → ChaosExperimentHandler                     │
│   chaos_experiment_run.resolvers.go → ChaosExperimentRunHandler             │
└──────────────────────────────────┬──────────────────────────────────────────┘
                                   │ SendExperimentToSubscriber()
                                   ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                       CHAOS INFRASTRUCTURE (Subscriber)                      │
│   Receives K8s manifests via WebSocket → Deploys to target cluster          │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Key Components

| Component | Location | Purpose |
|-----------|----------|---------|
| **NewExperimentButton** | `chaoscenter/web/src/components/NewExperimentButton/NewExperimentButton.tsx` | Triggers navigation to experiment creation |
| **StudioOverview** | `chaoscenter/web/src/views/StudioOverview/StudioOverview.tsx` | Collects experiment metadata + infrastructure selection |
| **ChaosStudio** | `chaoscenter/web/src/views/ChaosStudio/ChaosStudio.tsx` | Full experiment builder with visual/YAML editor |
| **RunChaosExperiment** | `chaoscenter/graphql/server/graph/chaos_experiment_run.resolvers.go` (Line 24) | Backend resolver that triggers experiment execution |

---

## 2. How "+ New Experiment" Button Ties to Environment and Infrastructure

When you click **"+ New Experiment"**, here's the flow:

### Step 1: Route Navigation

```typescript
// NewExperimentButton.tsx
onClick={() => history.push(paths.toNewExperiment({ experimentKey: getHash() }))}
// Routes to: /experiments/new/{experimentKey}/chaos-studio
```

### Step 2: StudioOverview Form

The **StudioOverview** component renders a form with:

```typescript
// StudioOverview.tsx - Lines 79-91
initialValues={{
  name: '',
  description: '',
  tags: [],
  chaosInfrastructure: {
    id: '',                    // Infrastructure ID (required)
    namespace: undefined,
    environmentID: ''          // Environment ID (linked via infra)
  }
}}
```

### Step 3: Infrastructure Reference Field

The **KubernetesChaosInfrastructureReferenceFieldController** provides infrastructure selection:

```typescript
// KubernetesChaosInfrastructureReferenceField.tsx - Key relationships

// 1. Fetch environments
const { data: listEnvironmentData } = listEnvironment({ ...scope });

// 2. Fetch infrastructures filtered by environment
const { data: listChaosInfraData } = listChaosInfra({
  environmentIDs: envID === AllEnv.AllEnv ? undefined : [envID],  // Filter by env
  ...
});

// 3. When infrastructure is selected, store both IDs
setFieldValue('chaosInfrastructure.id', infrastructure.id, true);
setFieldValue('chaosInfrastructure.namespace', infrastructure.namespace, false);
setFieldValue('chaosInfrastructure.environmentID', infrastructure.environmentID, false);
```

### Data Model Relationship

```
┌─────────────────┐       1:N        ┌─────────────────────┐       1:N        ┌─────────────────┐
│   Environment   │ ───────────────► │  ChaosInfrastructure │ ───────────────► │  ChaosExperiment │
│                 │                  │                     │                  │                 │
│ - environmentID │                  │ - infraID           │                  │ - experimentID  │
│ - name          │                  │ - environmentID     │                  │ - infraID       │
│ - type          │                  │ - infraNamespace    │                  │ - projectID     │
│ - infra_ids[]   │                  │ - isActive          │                  │ - manifest      │
└─────────────────┘                  └─────────────────────┘                  └─────────────────┘
```

### Database Schema References

From `chaoscenter/graphql/server/pkg/database/mongodb/chaos_infrastructure/schema.go`:

```go
type ChaosInfra struct {
    InfraID       string `bson:"infra_id"`
    EnvironmentID string `bson:"environment_id"`  // Links infra to environment
    ProjectID     string `bson:"project_id"`
    InfraNamespace *string `bson:"infra_namespace"`
    IsActive      bool   `bson:"is_active"`
    // ...
}
```

From `chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment/schema.go`:

```go
type ChaosExperimentRequest struct {
    ProjectID    string `bson:"project_id"`
    ExperimentID string `bson:"experiment_id"`
    InfraID      string `bson:"infra_id"`  // Links experiment to infrastructure
    // ...
}
```

---

## 3. How Faults Run on the Selected Environment

When you run an experiment, here's the execution flow:

### Step 1: Trigger Run

```typescript
// ChaosStudio.tsx - runExperimentHandler
runChaosExperimentMutation({
  variables: {
    projectID: scope.projectID,
    experimentID: experimentKey  // Contains infraID in its manifest
  }
});
```

### Step 2: GraphQL Resolver

```go
// chaos_experiment_run.resolvers.go - RunChaosExperiment
func (r *mutationResolver) RunChaosExperiment(ctx context.Context, experimentID string, projectID string) {
    // 1. Fetch the experiment from DB (contains infraID)
    experiment, err := r.chaosExperimentHandler.GetDBExperiment(query)
    
    // 2. Execute the workflow on the infra
    uiResponse, err = r.chaosExperimentRunHandler.RunChaosWorkFlow(ctx, projectID, experiment, data_store.Store)
}
```

### Step 3: Validate Infrastructure

```go
// handler.go - RunChaosWorkFlow (Line 671-677)
func (c *ChaosExperimentRunHandler) RunChaosWorkFlow(...) {
    // Get the infrastructure details using infraID from experiment
    infra, err := dbChaosInfra.NewInfrastructureOperator(c.mongodbOperator).GetInfra(workflow.InfraID)
    
    // Check if infrastructure is active
    if !infra.IsActive {
        return nil, errors.New("experiment re-run failed due to inactive infra")
    }
    // ...
}
```

### Step 4: Send to Subscriber (Target Environment)

```go
// handler.go - Lines 937-947
// After creating the experiment run in DB, send to the infrastructure's subscriber
chaos_infrastructure.SendExperimentToSubscriber(projectID, &model.ChaosExperimentRequest{
    ExperimentID:       &workflow.ExperimentID,
    ExperimentManifest: string(manifest),   // K8s workflow manifest
    InfraID:            workflow.InfraID,   // Target infrastructure
}, &username, nil, "create", r)
```

### Step 5: Subscriber Receives and Executes

```go
// infra_utils.go - SendRequestToSubscriber
func SendRequestToSubscriber(subscriberRequest SubscriberRequests, r store.StateData) {
    newAction := &model.InfraActionResponse{
        ProjectID: subscriberRequest.ProjectID,
        Action: &model.ActionPayload{
            K8sManifest:  subscriberRequest.K8sManifest,  // Workflow YAML
            Namespace:    subscriberRequest.Namespace,
            RequestType:  subscriberRequest.RequestType,  // "create"
        },
    }

    // Send to the connected subscriber via WebSocket channel
    if observer, ok := r.ConnectedInfra[subscriberRequest.InfraID]; ok {
        observer <- newAction  // Subscriber applies manifest to cluster
    }
}
```

---

## 4. Complete Execution Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                                    EXECUTION FLOW                                        │
└─────────────────────────────────────────────────────────────────────────────────────────┘

User clicks "Run"
       │
       ▼
┌──────────────────┐
│  runChaosExperiment  │  GraphQL Mutation
│  (experimentID)      │
└─────────┬────────────┘
          │
          ▼
┌──────────────────────────────────────────┐
│  GetDBExperiment(experimentID)           │  Fetch experiment with infraID
│  ├── experiment.InfraID = "infra-xyz"    │
│  └── experiment.Manifest = YAML          │
└─────────┬────────────────────────────────┘
          │
          ▼
┌──────────────────────────────────────────┐
│  GetInfra(workflow.InfraID)              │  Validate infra is active
│  ├── infra.EnvironmentID = "prod"        │
│  └── infra.IsActive = true               │
└─────────┬────────────────────────────────┘
          │
          ▼
┌──────────────────────────────────────────┐
│  CreateExperimentRun() in MongoDB        │  Record the run
└─────────┬────────────────────────────────┘
          │
          ▼
┌──────────────────────────────────────────┐
│  SendExperimentToSubscriber()            │  Send to target infra
│  ├── InfraID: "infra-xyz"                │
│  └── Manifest: Argo Workflow + ChaosEngine │
└─────────┬────────────────────────────────┘
          │
          ▼ (WebSocket)
┌──────────────────────────────────────────┐
│  SUBSCRIBER (in target K8s cluster)      │
│  ├── Receives action via WebSocket       │
│  ├── kubectl apply -f manifest.yaml      │
│  └── Reports status back to ChaosCenter  │
└──────────────────────────────────────────┘
          │
          ▼
┌──────────────────────────────────────────┐
│  Argo Workflow Controller                │
│  ├── Executes workflow steps             │
│  └── Runs ChaosEngine (fault injection)  │
└─────────┬────────────────────────────────┘
          │
          ▼
┌──────────────────────────────────────────┐
│  TARGET PODS (in environment namespace)  │
│  └── Fault applied: pod-delete, cpu-hog  │
└──────────────────────────────────────────┘
```

---

## 5. Key File References

### Frontend Components

| File | Purpose |
|------|---------|
| `chaoscenter/web/src/components/NewExperimentButton/NewExperimentButton.tsx` | New experiment button |
| `chaoscenter/web/src/views/StudioOverview/StudioOverview.tsx` | Experiment overview form |
| `chaoscenter/web/src/views/ChaosStudio/ChaosStudio.tsx` | Chaos studio (visual/YAML builder) |
| `chaoscenter/web/src/controllers/KubernetesChaosInfrastructureReferenceField/KubernetesChaosInfrastructureReferenceField.tsx` | Infrastructure selection |
| `chaoscenter/web/src/api/core/experiments/runChaosExperiment.ts` | Run experiment GraphQL mutation |

### Backend Resolvers & Handlers

| File | Purpose |
|------|---------|
| `chaoscenter/graphql/server/graph/chaos_experiment.resolvers.go` | Experiment CRUD resolvers |
| `chaoscenter/graphql/server/graph/chaos_experiment_run.resolvers.go` | Experiment run resolvers |
| `chaoscenter/graphql/server/pkg/chaos_experiment_run/handler/handler.go` | Run workflow handler |
| `chaoscenter/graphql/server/pkg/chaos_infrastructure/infra_utils.go` | Send to subscriber |

### Database Schemas

| File | Purpose |
|------|---------|
| `chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment/schema.go` | Experiment schema |
| `chaoscenter/graphql/server/pkg/database/mongodb/chaos_infrastructure/schema.go` | Infrastructure schema |
| `chaoscenter/graphql/server/pkg/database/mongodb/environment/schema.go` | Environment schema |

---

## 6. Summary

| Question | Answer |
|----------|--------|
| **How experiments work** | Experiments are stored with an `infraID`. When run, the manifest is sent via WebSocket to the subscriber agent running in the target cluster. |
| **Environment-Infrastructure tie** | Infrastructures belong to Environments (via `environmentID` field). When selecting infra in the UI, you first filter by environment, then pick an infrastructure. |
| **Running faults on environment** | The experiment's `infraID` determines which subscriber receives the workflow manifest. The subscriber applies the Argo Workflow + ChaosEngine to execute faults in that cluster/namespace. |

---

## 7. Entity Relationship Summary

```
Project
   │
   ├── Environment (1:N)
   │      │
   │      └── ChaosInfrastructure (1:N)
   │             │
   │             └── ChaosExperiment (1:N)
   │                    │
   │                    └── ChaosExperimentRun (1:N)
   │
   └── ChaosHub (1:N)
          │
          └── Faults/Charts (predefined chaos experiments)
```

---

*Document created: January 30, 2026*
*Last updated: January 30, 2026*
