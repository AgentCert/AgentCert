# AgentCert

A platform for automated certification of AI agents operating under Kubernetes fault injection. AgentCert orchestrates chaos engineering experiments, collects agent behavior traces, and generates comprehensive certification reports.

## Overview

AgentCert evaluates how AI agents respond to infrastructure failures by:

1. **Deploying** target applications and AI agents into Kubernetes
2. **Injecting** chaos faults (pod kills, CPU stress, network latency, etc.)
3. **Observing** agent behavior via LLM trace collection
4. **Analyzing** response patterns across multiple experimental runs
5. **Certifying** agent resilience with statistical and qualitative metrics

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        AgentCert Platform                        │
├─────────────────┬─────────────────┬─────────────────────────────┤
│   Frontend      │   GraphQL API   │   Auth Service              │
│   (React)       │   (Go)          │   (Go)                      │
└────────┬────────┴────────┬────────┴────────┬────────────────────┘
         │                 │                 │
         └─────────────────┼─────────────────┘
                           │
         ┌─────────────────┼─────────────────┐
         │                 │                 │
    ┌────▼────┐    ┌───────▼───────┐   ┌────▼────┐
    │ MongoDB │    │   Kubernetes  │   │ Langfuse│
    │         │    │   Cluster     │   │ (Traces)│
    └─────────┘    └───────┬───────┘   └─────────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
         ┌────▼────┐  ┌────▼────┐  ┌────▼────┐
         │ Target  │  │   AI    │  │  Chaos  │
         │  App    │  │  Agent  │  │ Faults  │
         └─────────┘  └─────────┘  └─────────┘
```

## Quick Start

### Prerequisites

- Docker 20.x+
- Go 1.24+
- Node.js 18.x+
- Python 3.10+
- Kubernetes cluster (minikube/kind)
- kubectl

### Setup

```bash
# Clone repository
git clone https://github.com/agentcert/AgentCert.git
cd AgentCert

# Start services (Linux/macOS)
./start-agentcert.sh

# Or use the automated build script
./build-all.sh --llm azure
```

See [setup.md](setup.md) for detailed installation instructions.

## Project Structure

```
AgentCert/
├── agentcert/           # Core application code
│   ├── authentication/  # Auth service (Go)
│   ├── graphql/         # GraphQL API (Go)
│   └── frontend/        # Web UI (React)
├── chaoscenter/         # Litmus Chaos integration
├── scripts/             # Utility scripts
├── local-custom/        # Local configuration
├── build-*.sh           # Build scripts for components
└── setup.md             # Detailed setup guide
```

## Build Components

```bash
# Build all Docker images
./build-all.sh --llm azure

# Or build individually:
./build-flash-agent.sh      # Flash agent image
./build-agent-sidecar.sh    # Sidecar proxy image
./build-install-agent.sh    # Agent installer image
./build-install-app.sh      # App installer image
./build-litellm.sh          # LiteLLM proxy image
```

## Running Experiments

1. **Start AgentCert** — Launch the platform services
2. **Create Environment** — Define Kubernetes cluster connection
3. **Deploy Application** — Install target app (e.g., Sock Shop)
4. **Deploy Agent** — Install AI agent (e.g., Flash Agent)
5. **Create Experiment** — Select chaos faults and run duration
6. **Execute** — Run the experiment and collect traces
7. **Certify** — Generate certification report from traces

## Documentation

- [Setup Guide](setup.md) — Detailed installation instructions
- [Install Agent Flow](INSTALL_AGENT_HELM_FLOW.md) — Agent deployment internals
- [Trace Workflow](TRACE_WORKFLOW_AND_CHANGELOG.md) — Trace collection pipeline

## Related Repositories

| Repository | Description |
|------------|-------------|
| [agent-charts](../agent-charts) | Helm charts for AI agents |
| [app-charts](../app-charts) | Helm charts for target applications |
| [flash-agent](../flash-agent) | ITOps Kubernetes log analysis agent |
| [agent-sidecar](../agent-sidecar) | Metadata injection proxy |
| [certifier](../certifier) | Certification report generator |
| [chaos-charts](../chaos-charts) | Litmus chaos experiment definitions |

## License

MIT License - see [LICENSE](LICENSE)
