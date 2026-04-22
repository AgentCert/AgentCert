# AgentCert Azure Deployment Architecture

## PowerPoint Slide Outline

---

### Slide 1: Title Slide
**Title:** AgentCert Azure Production Deployment  
**Subtitle:** End-to-End Architecture & Component Stack  
**Date:** April 2026  
**Logo/Branding:** AgentCert Logo

---

### Slide 2: Architecture Overview (using zones)
**Title:** Deployment Architecture Zones

Visual: 4 colored zones:
- **User Entry Zone** (blue): Azure DNS → App Gateway WAF → Ingress
- **Compute Zone** (green): AKS cluster with all workloads
- **Azure Services Zone** (purple): ACR, Key Vault, Cosmos DB, Azure OpenAI, Storage, Monitor
- **Observability Zone** (orange): Langfuse, Azure Monitor

**Key point:** Users → Secure Gateway → AKS Cluster → Azure Managed Services

---

### Slide 3: Detailed Azure Deployment Topology
**Title:** Detailed Azure Deployment Topology

Visual: 5-column architecture view:
- **Entry & Security:** Azure DNS, Application Gateway + WAF, TLS, AGIC/NGINX ingress
- **Control Plane:** Frontend, auth server, GraphQL server, Mongo-backed registries
- **Execution Plane:** subscriber, event-tracker, workflow-controller, chaos-operator, chaos-exporter
- **Agent Plane:** flash-agent, install-agent, install-app workloads, agent-sidecar
- **Platform Services:** ACR, Key Vault, Azure OpenAI via LiteLLM, Langfuse, Monitor, Storage

**Key point:** The design isolates user-facing services from experiment controllers and agent workloads while keeping all evidence paths observable.

---

### Slide 4: Edge & Security Layer
**Title:** Edge Layer: DNS, WAF, Ingress

**Components (each as a box/shape):**
1. **Azure DNS** → Resolves production domain
2. **App Gateway with WAF** → TLS termination, security policies
3. **AKS Ingress Controller** → Routes to backend services

**Flow:** User Request → DNS → App Gateway (TLS + WAF) → Ingress → AKS Service

---

### Slide 5: Core Compute: AKS Cluster
**Title:** Azure Kubernetes Service (AKS) Workloads

**Main box: AKS Cluster** containing:

| Pod/Service | Namespace | Tech Stack | Role |
|---|---|---|---|
| Frontend | litmus-chaos | React/TypeScript | Web UI |
| GraphQL API | litmus-chaos | Go/gqlgen/Gin | Core API & orchestration |
| Auth Service | litmus-chaos | Go/REST+gRPC | User auth & tokens |
| Event Tracker | litmus-exp | Go controllers | Event streaming |
| LitmusChaos + Argo Workflows | litmus-exp | Workflow engine | Chaos orchestration |
| Agents (Flash, Install, Custom) | Agent namespaces | Helm + Agent Framework | Remediation execution |
| Agent Sidecar | Agent namespaces | Python | Trace injection & context |
| LiteLLM Proxy | Shared integration namespace | Python/FastAPI | LLM gateway with callbacks |
| OTEL Exporters | Control-plane services | Go/OTEL SDK | Trace bridge to Langfuse |

---

### Slide 6: Data & Secrets Layer
**Title:** Data, Secrets & Storage

**3 main components:**

1. **Azure Cosmos DB for MongoDB** (or Azure VM MongoDB)
   - Stores: Users, runs, experiment metadata, config
   - Connection: Secured via Key Vault, private endpoint optional

2. **Azure Key Vault**
   - Stores: API keys, DB credentials, JWT secret, Langfuse keys
   - Access: AKS pods via managed identity

3. **Azure Storage Account (Blob)**
   - Stores: Workflow artifacts, exports, backups
   - Usage: LitmusChaos workflows and exports

---

### Slide 7: MongoDB Data Model
**Title:** MongoDB Data Model

**Core collections shown in the diagram:**
- `project` with members and tenant ownership
- `environment` with `project_id` and `infra_ids`
- `chaosInfrastructures` for registered execution targets
- `apps_registrations` for benchmark target applications
- `agentRegistry` for onboarded AI agents and endpoint metadata
- `chaosExperiments` for benchmark definitions and revisions
- `chaosExperimentRuns` for per-run evidence and scores
- Supporting collections such as `chaosProbes`, `chaosHubs`, `imageRegistry`, `serverConfig`, `gitops`, and `faultStudios`

**Key relationships:**
- `project -> environment -> apps_registrations`
- `project -> agentRegistry`
- `project -> chaosExperiments -> chaosExperimentRuns`
- `chaosInfrastructures -> chaosExperiments` and `chaosExperimentRuns`

**Key point:** MongoDB holds the platform state and linkage keys, while detailed LLM trace payloads remain in Langfuse.

---

### Slide 8: LLM & Observability Integration
**Title:** LLM Integration & Observability

**Flow:**
1. Agent/LLM Request → LiteLLM Proxy
2. LiteLLM checks AZURE_API_KEY, calls Azure OpenAI
3. Success/failure callbacks → Langfuse
4. Traces collected → Langfuse dashboard

**Key services:**
- **Azure OpenAI**: Inference for agents and evaluators
- **LiteLLM Proxy**: Centralized gateway, retry logic, auth
- **Langfuse**: Trace storage, evaluation, dashboards
- **OTEL Bridge**: Links Langfuse REST traces to OTEL spans

---

### Slide 9: Detailed Deployment & Runtime Flow
**Title:** Detailed Deployment & Runtime Flow

**Runtime sequence:**
1. Build and publish images to ACR
2. Bootstrap namespaces, RBAC, CRDs, and Litmus controllers
3. Deploy frontend, auth, and GraphQL control plane
4. Activate execution-plane controllers and event services
5. Run benchmarks against target applications and registered agents
6. Correlate OTEL, Langfuse, LiteLLM, and Kubernetes telemetry into verdict evidence

**Key point:** Separating bootstrap, control-plane rollout, execution-plane activation, and evidence capture makes production failures easier to isolate.

---

### Slide 10: Container Registry & Images
**Title:** Image Management: Azure Container Registry

**Components stored in ACR:**
- `agentcert/litmusportal-server:3.0.0`
- `agentcert/litmusportal-subscriber:3.0.0`
- `agentcert/graphql-server:latest`
- `agentcert/auth-service:latest`
- `agentcert/agentcert-flash-agent:latest`
- `agentcert/agentcert-install-agent:latest`
- `agentcert/agent-sidecar:latest`
- `agentcert/agentcert-litellm-proxy:latest`

**AKS pulls from ACR** using credentials from Key Vault

---

### Slide 11: Observability & Monitoring
**Title:** Azure Monitor & Logging

**Azure Monitor Integration:**
- Pod-level metrics (CPU, memory, disk)
- Container logs → Log Analytics
- Custom dashboards for API latency, error rates
- Alerts on pod crashes, high latency

**Langfuse Observability:**
- LLM request/response traces
- Agent decision logs
- Evaluation scores (RAGAS, Trajectory)
- Performance trends

---

### Slide 12: CI/CD & Deployment
**Title:** Build & Release Pipeline

**Pipeline stages:**
1. **Code push to GitHub**
2. **GitHub Actions**: Build, test, lint
3. **Build images** → Push to ACR
4. **Deploy to AKS** → Helm charts + kubectl
5. **Secrets injected** from Key Vault
6. **Health checks** on new deployments

**Tools:** GitHub Actions + Azure DevOps + Helm

---

### Slide 13: Security & Optional Hardening
**Title:** Security Layers (Optional)

**Implemented:**
- Key Vault for secrets management
- ACR image scanning
- AKS network policies

**Optional hardening:**
- **VNet private endpoints** for Cosmos DB, Key Vault, Azure OpenAI
- **Network Security Groups (NSGs)** to restrict pod-to-service traffic
- **Microsoft Entra ID** for operator SSO
- **Pod security policies / standards** for container isolation

---

### Slide 14: Operations, Scaling & Recovery
**Title:** Operations, Scaling & Recovery

**Focus areas:**
- Independent scaling for control plane, experiment controllers, and agent workloads
- Backups for MongoDB state, Langfuse data, Helm values, and exported benchmark evidence
- Runbooks for namespace bootstrap, rollout failures, controller health, ACR pull issues, and secret rotation
- SLOs for API latency, rollout success, benchmark completion, and trace-correlation quality

---

### Slide 15: Deployment Checklist
**Title:** Production Deployment Readiness

**Pre-deployment:**
- [ ] Azure subscription & resource group created
- [ ] AKS cluster provisioned (1.27+, multi-zone)
- [ ] Cosmos DB or MongoDB provisioned
- [ ] Container Registry (ACR) set up
- [ ] Key Vault created with secrets
- [ ] Azure OpenAI resource provisioned

**Deployment:**
- [ ] Docker images built & pushed to ACR
- [ ] Helm charts configured with prod values
- [ ] Secrets synced to Key Vault
- [ ] Helm install/upgrade executed
- [ ] Health checks & smoke tests pass

**Post-deployment:**
- [ ] DNS record points to App Gateway
- [ ] TLS certificate installed
- [ ] Monitoring dashboards created
- [ ] Backup strategy enabled
- [ ] Disaster recovery plan documented

---

### Slide 16: Resource Sizing & Cost (Optional)
**Title:** Production Resource Estimates

| Component | SKU / Size | Approx. Monthly Cost |
|---|---|---|
| AKS Cluster | 3 nodes, Standard_D2s_v3 | $300–400 |
| Cosmos DB MongoDB | 400 RU/s, multi-region | $200–500 |
| Azure OpenAI | gpt-4.1-mini deployments | $50–500 (usage-based) |
| ACR | Premium tier | $100–200 |
| Key Vault | Standard tier | $10 |
| Azure Monitor | Ingestion + retention | $50–200 |
| App Gateway | Standard tier | $30–50 |
| Storage Account | Blob, geo-redundant | $20–100 |
| **Total (baseline)** | | **~$750–2000/month** |

---

### Slide 17: Summary & Next Steps
**Title:** Deployment Summary & Next Steps

**What we've covered:**
✅ Edge security (DNS, WAF, Ingress)  
✅ Core compute (AKS workloads)  
✅ MongoDB data model for platform state and run evidence  
✅ Data layer (Cosmos DB, Key Vault)  
✅ LLM integration (Azure OpenAI + LiteLLM + Langfuse)  
✅ Observability (Azure Monitor + Langfuse)  
✅ CI/CD pipeline and runtime evidence flow  
✅ Operational scaling and recovery guidance  

**Next steps:**
1. Provision Azure resources
2. Configure Helm values for prod environment
3. Set up monitoring & alerting
4. Run dry-run deployment in staging
5. Plan cutover and rollback strategy
6. Train ops team on runbooks

**Questions?**

---

## How to Use This in PowerPoint

1. **Create a new presentation** in PowerPoint
2. **Use the slide outlines above** as your content structure
3. **For each slide:**
   - Add the title as shown
   - Use **Shapes** (rectangles, circles, arrows) to draw the architecture
   - Color-code zones (blue for edge, green for compute, etc.)
   - Add bullet points from the outline
4. **For Slide 4 (AKS Workloads)**, create a **large rectangle** with smaller boxes inside representing each service
5. **For Slide 11 (Checklist)**, use **checkboxes** and bullet points
6. **Export as PDF** once finalized

---

## Design Tips

- **Use consistent colors** across all slides
- **Large boxes/shapes** for major components
- **Arrows** for data flow and dependencies
- **Icons** for Azure services (download from Azure icon library)
- **Keep text minimal** — use speaker notes for detail
- **Use animations** to reveal layers progressively (optional)

