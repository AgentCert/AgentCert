# AgentHub & AppsHub Design Document

## 1. Overview

AgentCert's **ChaosHub** provides a catalogue of chaos faults with one-liner descriptions, organized by category (Kubernetes, Azure, etc.), backed by a Git repository (`chaos-charts`). Users browse faults, see descriptions, and pick them for experiment construction.

This document designs two analogous hubs:

| Hub | Purpose | Data Source | Analogy |
|-----|---------|-------------|---------|
| **ChaosHub** *(existing)* | Catalogue of chaos faults | `chaos-charts` Git repo | "What faults can I inject?" |
| **AgentHub** *(new)* | Catalogue of AI agents — deployed & available | `agent-charts` Git repo + live cluster state | "What agents exist and what can they do?" |
| **AppsHub** *(new)* | Catalogue of target applications — deployed & available | `app-charts` Git repo + live cluster state | "What applications are deployed and what do they contain?" |

All three hubs share the same UX pattern: **left sidebar with categories → right panel with cards showing name + one-liner description**.

---

## 2. ChaosHub Architecture (Reference)

Understanding the existing pattern is critical because AgentHub and AppsHub must extend it.

### 2.1 Data Flow

```
chaos-charts Git repo
  └── faults/
       └── kubernetes/
            ├── kubernetes.chartserviceversion.yaml   ← category metadata + fault list
            ├── pod-delete/
            │    ├── pod-delete.chartserviceversion.yaml
            │    ├── fault.yaml  (ChaosExperiment CR)
            │    └── engine.yaml (ChaosEngine CR)
            ├── install-agent/
            │    └── ...
            └── ...
```

### 2.2 GraphQL Schema (Key Types)

```graphql
type Chart {
  apiVersion: String!
  kind: String!
  metadata: Metadata!
  spec: Spec!
  packageInfo: PackageInformation!
}

type Spec {
  displayName: String!              # e.g. "Kubernetes"
  categoryDescription: String!      # Category-level description
  faults: [FaultList!]!             # Array of faults in this category
  ...
}

type FaultList {
  name: String!                     # e.g. "pod-delete"
  displayName: String!              # e.g. "Pod Delete"
  description: String!              # One-liner: "It injects pod-delete chaos..."
}
```

### 2.3 Backend Service

- `SyncDefaultChaosHubs()` — goroutine clones/pulls from `DEFAULT_HUB_GIT_URL` into `/tmp/default/<hubName>/`
- `ListChaosHubs()` — merges synthetic default hub + DB hubs, counts faults from filesystem
- `ListChaosFaults()` — reads `chartserviceversion.yaml` files from cloned repo, returns `[Chart]`
- `GetChartsData()` — walks category directories under `faults/`, parses YAML → `[]*model.Chart`

### 2.4 Frontend Components

```
SideNav
  └── "ChaosHubs" link → /chaos-hubs
       └── ChaosHubsView (list of hub cards)
            └── ChaosHubView (inside a hub)
                 ├── Tab: "Chaos Experiments" (PredefinedExperiments)
                 └── Tab: "Chaos Faults" (ChaosFaults)
                      ├── Left sidebar: category tags (Kubernetes, Azure, ...) with counts
                      └── Right panel: paginated fault cards
                           └── FaultCard: icon + displayName + description
```

---

## 3. AgentHub Design

### 3.1 Concept

AgentHub is a read-only catalogue that shows all AI agents that are:
1. **Available** — defined in the `agent-charts` Git repo (chart definitions)
2. **Deployed** — currently running in the Kubernetes cluster (live status from Agent Registry)

Each agent has a one-liner description, capabilities, version, and deployment status.

### 3.2 Data Source: `agent-charts` Repository

Following the ChaosHub pattern, we introduce a **chartserviceversion.yaml** for agents:

```
agent-charts Git repo
  └── charts/
       └── agents/                                          ← category directory
            ├── agents.chartserviceversion.yaml              ← category metadata + agent list
            ├── flash-agent/
            │    ├── Chart.yaml
            │    ├── values.yaml
            │    └── templates/
            └── k8s-agent/
                 ├── Chart.yaml
                 ├── values.yaml
                 └── templates/
```

#### `agents.chartserviceversion.yaml`

```yaml
apiVersion: agentcert.io/v1alpha1
kind: ChartServiceVersion
metadata:
  name: agents
  version: 0.1.0
  annotations:
    categories: AI Agents
    chartDescription: >
      AI agents that observe, diagnose, and remediate faults in Kubernetes environments.
    vendor: AgentCert
spec:
  displayName: AI Agents
  categoryDescription: >
    AI-powered agents that integrate with LitmusChaos to monitor, detect,
    and autonomously remediate faults in Kubernetes clusters. Each agent
    specializes in specific capabilities like log analysis, fault detection,
    or automated remediation.
  keywords:
    - ai-agent
    - kubernetes
    - observability
    - remediation
  maturity: alpha
  maintainers:
    - name: AgentCert Team
      email: agentcert@microsoft.com
  provider:
    name: AgentCert
  agents:
    - name: flash-agent
      displayName: "Flash Agent"
      description: "AI-powered log analysis agent that extracts metrics from Kubernetes pod logs using LLM and reports to Langfuse."
      version: "1.0.0"
      capabilities:
        - log-metrics-extraction
        - langfuse-trace-reporting
    - name: k8s-agent
      displayName: "K8s Agent"
      description: "Kubernetes fault detection and autonomous remediation agent that monitors cluster health and auto-fixes issues."
      version: "1.0.0"
      capabilities:
        - fault-detection
        - auto-remediation
```

### 3.3 GraphQL Schema

```graphql
# ─── New types (following ChaosHub pattern) ───

enum HubKind {
  CHAOS       # Existing
  AGENT       # New
  APP         # New
}

type AgentHubEntry {
  """Agent identifier (folder name in chart repo)"""
  name: String!
  """Human-readable name"""
  displayName: String!
  """One-liner description shown on the card"""
  description: String!
  """Semantic version from chart"""
  version: String!
  """List of capabilities this agent supports"""
  capabilities: [String!]!
  """Whether this agent is currently deployed in the cluster"""
  isDeployed: Boolean!
  """Current status if deployed (ACTIVE, INACTIVE, REGISTERED, etc.)"""
  deploymentStatus: AgentStatus
  """Agent ID in the registry (if deployed)"""
  agentID: ID
  """Namespace where the agent is deployed"""
  namespace: String
  """Helm release name (if deployed via Helm)"""
  helmReleaseName: String
}

type AgentHubCategory {
  """Category name (e.g., "AI Agents")"""
  displayName: String!
  """Category-level description"""
  categoryDescription: String!
  """Agents in this category"""
  agents: [AgentHubEntry!]!
}

type AgentHubStatus {
  """Hub ID"""
  id: ID!
  """Hub name"""
  name: String!
  """Git repo URL"""
  repoURL: String!
  """Git branch"""
  repoBranch: String!
  """Whether the hub is synced and available"""
  isAvailable: Boolean!
  """Total number of agents defined in the hub"""
  totalAgents: String!
  """Number of agents currently deployed"""
  deployedAgents: String!
  """Whether this is the default hub"""
  isDefault: Boolean!
  """Last synced timestamp"""
  lastSyncedAt: String!
}

# ─── Queries ───

extend type Query {
  """List all agent hub categories with their agents and deployment status"""
  listAgentHub(projectID: ID!): [AgentHubCategory!]! @authorized

  """Get a single agent's detailed information from the hub"""
  getAgentHubEntry(projectID: ID!, agentName: String!): AgentHubEntry! @authorized

  """List all configured agent hubs (analogous to listChaosHub)"""
  listAgentHubs(projectID: ID!): [AgentHubStatus]! @authorized
}
```

### 3.4 Backend Service Design

#### New Package: `pkg/agenthub/`

```
pkg/agenthub/
  ├── service.go        # AgentHub service (sync, list, enrich with live status)
  ├── handler/
  │    └── handler.go   # File-system operations (read agent CSV YAMLs)
  └── ops/
       └── gitops.go    # Git clone/pull for agent-charts repo (reuse chaoshub ops pattern)
```

#### Key Service Functions

| Function | Pattern Source | Description |
|----------|---------------|-------------|
| `SyncDefaultAgentHub()` | `SyncDefaultChaosHubs()` | Goroutine that periodically clones/pulls `agent-charts` to `/tmp/default-agents/<hubName>/` |
| `ListAgentHubCategories()` | `ListChaosFaults()` | Reads `chartserviceversion.yaml` from cloned repo, returns `[AgentHubCategory]` |
| `EnrichWithDeploymentStatus()` | *new* | Cross-references chart entries with Agent Registry DB to add `isDeployed`, `deploymentStatus`, `agentID` |
| `listDefaultAgentHubs()` | `listDefaultHubs()` | Returns synthetic default hub entry with `IsDefault=true` |

#### New Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DEFAULT_AGENT_HUB_GIT_URL` | `https://github.com/agentcert/agent-charts` | Git repo URL for default agent hub |
| `DEFAULT_AGENT_HUB_BRANCH_NAME` | `main` | Branch to sync |
| `DEFAULT_AGENT_HUB_PATH` | `/tmp/default-agents/` | Local clone path |

#### Enrichment Logic (Pseudocode)

```go
func (s *agentHubService) ListAgentHubCategories(ctx context.Context, projectID string) ([]*model.AgentHubCategory, error) {
    // 1. Read chart data from filesystem (like GetChartsData)
    categories := handler.GetAgentChartsData(agentChartsPath)

    // 2. Fetch all deployed agents from Agent Registry DB
    deployedAgents := s.agentRegistryOperator.ListAgents(ctx, projectID)

    // 3. Build name→agent lookup map
    deployedMap := make(map[string]*Agent)
    for _, agent := range deployedAgents {
        deployedMap[agent.Name] = agent
    }

    // 4. Enrich each chart entry with live status
    for _, category := range categories {
        for _, entry := range category.Agents {
            if deployed, ok := deployedMap[entry.Name]; ok {
                entry.IsDeployed = true
                entry.DeploymentStatus = deployed.Status
                entry.AgentID = deployed.AgentID
                entry.Namespace = deployed.Namespace
                entry.HelmReleaseName = deployed.HelmReleaseName
            }
        }
    }

    return categories, nil
}
```

### 3.5 Frontend Design

#### Route

```typescript
// RouteDefinitions.ts
toAgentHub: () => '/agent-hub',
```

#### SideNav Addition

```tsx
// SideNav.tsx — add below the ChaosHubs link
<SidebarLink label={getString('chaoshubs')} to={paths.toChaosHubs()} />
<SidebarLink label={getString('agentHub')} to={paths.toAgentHub()} />     {/* NEW */}
<SidebarLink label={getString('appsHub')} to={paths.toAppsHub()} />       {/* NEW */}
```

#### Component Tree

```
AgentHubView (mirrors ChaosHubView)
  ├── Left sidebar: category tags ("AI Agents") with agent count
  └── Right panel: paginated agent cards
       └── AgentCard
            ├── Icon (robot/agent icon)
            ├── displayName (e.g., "Flash Agent")
            ├── description (one-liner)
            ├── Status badge: DEPLOYED (green) / AVAILABLE (grey)
            ├── Capabilities tags
            └── Version badge
```

#### AgentCard Wireframe

```
┌───────────────────────────────────────────┐
│ 🤖  Flash Agent                    v1.0.0 │
│                                           │
│ AI-powered log analysis agent that        │
│ extracts metrics from Kubernetes pod      │
│ logs using LLM and reports to Langfuse.   │
│                                           │
│ ● DEPLOYED  │  ns: flash-agent            │
│                                           │
│ [log-metrics-extraction] [langfuse-trace] │
└───────────────────────────────────────────┘
```

---

## 4. AppsHub Design

### 4.1 Concept

AppsHub is a read-only catalogue that shows all target applications that are:
1. **Available** — defined in the `app-charts` Git repo (Helm chart definitions)
2. **Deployed** — currently running in the Kubernetes cluster (live status from cluster)

Each application entry shows its name, description, the microservices it contains, and deployment status.

### 4.2 Data Source: `app-charts` Repository

```
app-charts Git repo
  └── charts/
       └── applications/                                     ← category directory
            ├── applications.chartserviceversion.yaml         ← category metadata + app list
            └── sock-shop/
                 ├── Chart.yaml
                 ├── values.yaml
                 └── templates/
                      └── sock-shop/
                           ├── carts-deployment.yaml
                           ├── catalogue-deployment.yaml
                           ├── front-end-deployment.yaml
                           ├── orders-deployment.yaml
                           ├── payment-deployment.yaml
                           ├── shipping-deployment.yaml
                           └── ...
```

#### `applications.chartserviceversion.yaml`

```yaml
apiVersion: agentcert.io/v1alpha1
kind: ChartServiceVersion
metadata:
  name: applications
  version: 0.1.0
  annotations:
    categories: Target Applications
    chartDescription: >
      Target applications that can be deployed into the cluster for
      chaos engineering and AI agent benchmarking.
    vendor: AgentCert
spec:
  displayName: Target Applications
  categoryDescription: >
    Pre-configured applications designed to be deployed as chaos experiment
    targets. Each application is a realistic microservices stack with known
    failure modes, suitable for benchmarking AI agent remediation capabilities.
  keywords:
    - target-app
    - microservices
    - benchmark
  maturity: stable
  maintainers:
    - name: AgentCert Team
      email: agentcert@microsoft.com
  provider:
    name: AgentCert
  applications:
    - name: sock-shop
      displayName: "Sock Shop"
      description: "Weaveworks Sock Shop — a cloud-native microservices demo with 13 services (carts, catalogue, orders, payment, shipping, user, queue-master, front-end + DBs + RabbitMQ)."
      version: "1.0.0"
      namespace: sock-shop
      microservices:
        - name: carts
          description: "Shopping cart service (Java/Spring)"
        - name: carts-db
          description: "Cart database (MongoDB)"
        - name: catalogue
          description: "Product catalogue service (Go)"
        - name: catalogue-db
          description: "Catalogue database (MySQL)"
        - name: front-end
          description: "Web frontend (Node.js)"
        - name: orders
          description: "Order processing service (Java/Spring)"
        - name: orders-db
          description: "Orders database (MongoDB)"
        - name: payment
          description: "Payment service (Go)"
        - name: queue-master
          description: "Queue consumer for order fulfillment (Java/Spring)"
        - name: rabbitmq
          description: "Message broker (RabbitMQ)"
        - name: shipping
          description: "Shipping service (Java/Spring)"
        - name: user
          description: "User account service (Go)"
        - name: user-db
          description: "User database (MongoDB)"
```

### 4.3 GraphQL Schema

```graphql
type Microservice {
  """Microservice name (e.g., "carts")"""
  name: String!
  """One-liner description"""
  description: String!
  """Whether this microservice pod is currently running"""
  isRunning: Boolean
  """Number of ready replicas"""
  readyReplicas: Int
  """Number of desired replicas"""
  desiredReplicas: Int
}

type AppHubEntry {
  """Application identifier (folder name in chart repo)"""
  name: String!
  """Human-readable name"""
  displayName: String!
  """One-liner description shown on the card"""
  description: String!
  """Semantic version from chart"""
  version: String!
  """Target namespace for deployment"""
  namespace: String!
  """Microservices that comprise this application"""
  microservices: [Microservice!]!
  """Whether this application is currently deployed in the cluster"""
  isDeployed: Boolean!
  """Number of microservices running vs total"""
  runningServices: String
  """Helm release name (if deployed via Helm)"""
  helmReleaseName: String
}

type AppHubCategory {
  """Category name (e.g., "Target Applications")"""
  displayName: String!
  """Category-level description"""
  categoryDescription: String!
  """Applications in this category"""
  applications: [AppHubEntry!]!
}

type AppHubStatus {
  """Hub ID"""
  id: ID!
  """Hub name"""
  name: String!
  """Git repo URL"""
  repoURL: String!
  """Git branch"""
  repoBranch: String!
  """Whether the hub is synced and available"""
  isAvailable: Boolean!
  """Total number of applications defined in the hub"""
  totalApps: String!
  """Number of applications currently deployed"""
  deployedApps: String!
  """Whether this is the default hub"""
  isDefault: Boolean!
  """Last synced timestamp"""
  lastSyncedAt: String!
}

# ─── Queries ───

extend type Query {
  """List all app hub categories with their applications and deployment status"""
  listAppHub(projectID: ID!): [AppHubCategory!]! @authorized

  """Get a single application's detailed information from the hub"""
  getAppHubEntry(projectID: ID!, appName: String!): AppHubEntry! @authorized

  """List all configured app hubs (analogous to listChaosHub)"""
  listAppHubs(projectID: ID!): [AppHubStatus]! @authorized
}
```

### 4.4 Backend Service Design

#### New Package: `pkg/apphub/`

```
pkg/apphub/
  ├── service.go        # AppHub service (sync, list, enrich with live status)
  ├── handler/
  │    └── handler.go   # File-system operations (read app CSV YAMLs)
  └── ops/
       └── gitops.go    # Git clone/pull for app-charts repo
```

#### Key Service Functions

| Function | Pattern Source | Description |
|----------|---------------|-------------|
| `SyncDefaultAppHub()` | `SyncDefaultChaosHubs()` | Goroutine that periodically clones/pulls `app-charts` to `/tmp/default-apps/<hubName>/` |
| `ListAppHubCategories()` | `ListChaosFaults()` | Reads `chartserviceversion.yaml` from cloned repo, returns `[AppHubCategory]` |
| `EnrichWithDeploymentStatus()` | *new* | Queries Kubernetes API for namespaces/deployments to determine `isDeployed`, `runningServices` |
| `listDefaultAppHubs()` | `listDefaultHubs()` | Returns synthetic default hub entry with `IsDefault=true` |

#### New Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DEFAULT_APP_HUB_GIT_URL` | `https://github.com/agentcert/app-charts` | Git repo URL for default app hub |
| `DEFAULT_APP_HUB_BRANCH_NAME` | `main` | Branch to sync |
| `DEFAULT_APP_HUB_PATH` | `/tmp/default-apps/` | Local clone path |

#### Enrichment Logic (Pseudocode)

```go
func (s *appHubService) ListAppHubCategories(ctx context.Context, projectID string) ([]*model.AppHubCategory, error) {
    // 1. Read chart data from filesystem
    categories := handler.GetAppChartsData(appChartsPath)

    // 2. For each app, check Kubernetes cluster for deployment status
    for _, category := range categories {
        for _, app := range category.Applications {
            ns := app.Namespace
            deployments, err := k8sClient.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
            if err != nil || len(deployments.Items) == 0 {
                app.IsDeployed = false
                continue
            }

            app.IsDeployed = true
            running := 0
            for _, ms := range app.Microservices {
                for _, dep := range deployments.Items {
                    if dep.Name == ms.Name {
                        ms.ReadyReplicas = int(dep.Status.ReadyReplicas)
                        ms.DesiredReplicas = int(*dep.Spec.Replicas)
                        ms.IsRunning = dep.Status.ReadyReplicas > 0
                        if ms.IsRunning { running++ }
                    }
                }
            }
            app.RunningServices = fmt.Sprintf("%d/%d", running, len(app.Microservices))
        }
    }

    return categories, nil
}
```

### 4.5 Frontend Design

#### Route

```typescript
// RouteDefinitions.ts
toAppsHub: () => '/apps-hub',
```

#### Component Tree

```
AppsHubView (mirrors ChaosHubView)
  ├── Left sidebar: category tags ("Target Applications") with app count
  └── Right panel: paginated app cards
       └── AppCard
            ├── Icon (app/cluster icon)
            ├── displayName (e.g., "Sock Shop")
            ├── description (one-liner)
            ├── Status badge: DEPLOYED (green) / AVAILABLE (grey)
            ├── Running services: "11/13 running"
            └── Namespace badge: "sock-shop"
```

#### AppCard Wireframe

```
┌───────────────────────────────────────────┐
│ 📦  Sock Shop                      v1.0.0 │
│                                           │
│ Weaveworks Sock Shop — a cloud-native     │
│ microservices demo with 13 services.      │
│                                           │
│ ● DEPLOYED  │  ns: sock-shop              │
│ Services: 11/13 running                   │
│                                           │
│ Expand ▼ (click to see microservices)     │
│  ├── ● carts (1/1)                        │
│  ├── ● catalogue (1/1)                    │
│  ├── ○ orders (0/1)                       │
│  └── ... 10 more                          │
└───────────────────────────────────────────┘
```

---

## 5. Shared Infrastructure

### 5.1 Generic Hub Sync Framework

All three hubs (ChaosHub, AgentHub, AppsHub) share the same Git sync pattern. We can extract a shared interface:

```go
type HubSyncer interface {
    SyncDefaultHub() error
    GetChartsPath(isDefault bool, hubName string, projectID string) string
    GetChartsData(chartsPath string) (interface{}, error)
}
```

### 5.2 Environment Variable Summary

| Variable | Service | Default Value |
|----------|---------|---------------|
| `DEFAULT_HUB_GIT_URL` | ChaosHub | `https://github.com/agentcert/chaos-charts` |
| `DEFAULT_HUB_BRANCH_NAME` | ChaosHub | `master` |
| `DEFAULT_AGENT_HUB_GIT_URL` | AgentHub | `https://github.com/agentcert/agent-charts` |
| `DEFAULT_AGENT_HUB_BRANCH_NAME` | AgentHub | `main` |
| `DEFAULT_APP_HUB_GIT_URL` | AppsHub | `https://github.com/agentcert/app-charts` |
| `DEFAULT_APP_HUB_BRANCH_NAME` | AppsHub | `main` |

### 5.3 SideNav Layout

```
┌─────────────────────┐
│  Project Selector    │
│  ─────────────────── │
│  Overview            │
│  Chaos Experiments   │
│  Environments        │
│  Resilience Probes   │
│  ─────────────────── │
│  ChaosHub            │   ← existing
│  AgentHub            │   ← NEW
│  AppsHub             │   ← NEW
│  ─────────────────── │
│  Agent Onboarding    │
│  Apps Onboarding     │
│  Project Setup ▸     │
└─────────────────────┘
```

---

## 6. Comparison Matrix

| Aspect | ChaosHub | AgentHub | AppsHub |
|--------|----------|----------|---------|
| **Git Repo** | `chaos-charts` | `agent-charts` | `app-charts` |
| **CSV YAML** | `<category>.chartserviceversion.yaml` | `agents.chartserviceversion.yaml` | `applications.chartserviceversion.yaml` |
| **Entry type** | `FaultList` (name, displayName, description) | `AgentHubEntry` (+ version, capabilities, deploymentStatus) | `AppHubEntry` (+ version, namespace, microservices, runningServices) |
| **Categories** | Kubernetes, Azure, GCP, AWS, etc. | AI Agents (extensible) | Target Applications (extensible) |
| **Live enrichment** | None (static from YAML) | Agent Registry DB → isDeployed, status | K8s API → isDeployed, running counts |
| **Card action** | Navigate to fault details page | Navigate to agent details / deploy | Navigate to app details / deploy |
| **Sidebar categories** | Fault category names | Agent category names | App category names |
| **Count display** | "Kubernetes (35)" | "AI Agents (2)" | "Target Applications (1)" |

---

## 7. Implementation Plan

### Phase 1: Repository Structure (agent-charts & app-charts)
1. Add `charts/agents/agents.chartserviceversion.yaml` to `agent-charts` repo
2. Add `charts/applications/applications.chartserviceversion.yaml` to `app-charts` repo
3. Restructure existing chart directories under the category folders

### Phase 2: Backend — AgentHub Service
1. Add env vars (`DEFAULT_AGENT_HUB_GIT_URL`, etc.) to `utils/variables.go` and `start-agentcert.ps1`
2. Create `pkg/agenthub/` package following `pkg/chaoshub/` patterns
3. Implement `SyncDefaultAgentHub()` goroutine
4. Implement `ListAgentHubCategories()` with Agent Registry enrichment
5. Add GraphQL schema definitions and resolvers

### Phase 3: Backend — AppsHub Service
1. Add env vars (`DEFAULT_APP_HUB_GIT_URL`, etc.)
2. Create `pkg/apphub/` package
3. Implement `SyncDefaultAppHub()` goroutine
4. Implement `ListAppHubCategories()` with K8s API enrichment
5. Add GraphQL schema definitions and resolvers

### Phase 4: Frontend
1. Add routes (`/agent-hub`, `/apps-hub`) to `RouteDefinitions.ts`
2. Add SideNav links
3. Create `AgentHub` view + controller (clone from ChaosHub, adapt types)
4. Create `AppsHub` view + controller
5. Add strings/translations

### Phase 5: Integration & Polish
1. Wire hubs into Overview dashboard (total counts cards)
2. Link agent/app cards to deploy actions (reuse existing onboarding flows)
3. Add search and filtering
4. Test with real data from agent-charts and app-charts repos
