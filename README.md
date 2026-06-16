# AgentCert

### A reliability-certification platform for AI agents that operate Kubernetes infrastructure.

Deploy an AI agent, break the environment it runs in, and produce a defensible report
on how well the agent held up. AgentCert is the **control plane** — UI, GraphQL API,
agent / app / fault registries, in-cluster subscriber, and Langfuse trace pipeline —
that ties the whole AgentCert monorepo together. It is a forked, extended LitmusChaos
ChaosCenter v3.0.0.

[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![React](https://img.shields.io/badge/React-18-61DAFB?style=flat-square&logo=react)](https://reactjs.org)
[![Litmus](https://img.shields.io/badge/forks-LitmusChaos%20v3.0.0-7E47C5?style=flat-square)](https://github.com/litmuschaos/litmus)
[![Argo](https://img.shields.io/badge/Argo-Workflows-EF7B4D?style=flat-square)](https://argoproj.github.io/argo-workflows/)
[![MongoDB](https://img.shields.io/badge/MongoDB-5-47A248?style=flat-square&logo=mongodb)](https://www.mongodb.com)
[![Langfuse](https://img.shields.io/badge/Langfuse-tracing-1C3D5A?style=flat-square)](https://langfuse.com)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue?style=flat-square)](LICENSE)

---

## What it does

The honest question every team shipping an autonomous Kubernetes agent has to answer
is: *when my cluster degrades, does my agent help, or make things worse?* AgentCert answers it by binding four concerns into one reproducible workflow:

| Concern | How AgentCert addresses it |
|---------|---------------------------|
| Controlled fault injection | LitmusChaos experiments — pod-delete, CPU/memory hog, network corruption, DNS spoofing, disk fill, install/uninstall lifecycle — selected from [`chaos-charts`](../../tree/chaos-charts) |
| A real system under test | A microservices target deployed from [`app-charts`](../../tree/app-charts) with Prometheus, Grafana, Kubernetes-MCP, Prometheus-MCP |
| A registered agent | First-class agent registry with versioned container images, capabilities, Langfuse linkage, and a health scheduler. Installed via [`agent-charts`](../../tree/agent-charts) |
| Auditable LLM behaviour | Every agent ↔ LLM call is intercepted by the [`agent-sidecar`](../../tree/agent-sidecar), stamped with experiment identity, routed through LiteLLM, and persisted to Langfuse |
| Statistical certification | Traces are ingested by the [`certifier`](../../tree/certifier) and turned into a 12-section HTML/PDF report |

---

## High-level architecture

```
Web (React) :2001  ──▶  GraphQL API :8080  ──▶  MongoDB :27017
                              │
                              ├── Authentication :3000 REST / :3030 gRPC
                              │   + Dex OIDC :5556
                              │
                              ├── Argo + LitmusChaos in target cluster
                              │      └── Subscriber pod
                              │           └── installs target app + agent + faults
                              │                └── agent → sidecar → LiteLLM
                              │                                       └── Langfuse
                              │
                              └── certifier ── reads Langfuse ── 12-section report
```

Full diagram, service map, image names, and per-namespace install stages live in [`docs/architecture.md`](docs/architecture.md).

---

## Documentation

The reference material is in [`docs/`](docs/) — eight focused files covering one
subsystem each. Read them in the order below for a first pass.

| Doc | Subsystem | What it covers |
|-----|-----------|----------------|
| [Architecture](docs/architecture.md) | Whole platform | Component map, port table, image map, deployment manifests, the four-stage per-namespace install |
| [Agent Registry](docs/agent-registry.md) | `pkg/agent_registry` + `pkg/agenthub` | Data model, status machine, GraphQL surface, the helm bridge that installs registered agents, Langfuse linkage, health scheduler, end-to-end registration flow |
| [App Registry](docs/app-registry.md) | `pkg/apps_registry` + `pkg/apphub` | App data model, status, AppHub chart-source management, how apps get installed via the `install-app` image |
| [Fault Studio](docs/fault-studio.md) | `pkg/fault_studio` | Curated fault collections — schema, injection types, service operations, default ChaosHub binding |
| [Observability](docs/observability.md) | `pkg/observability` | `LangfuseTracer` with deterministic IDs, content-signature dedup, workflow-node state cache, SLA config |
| [MCP Infrastructure](docs/mcp-infrastructure.md) | Stage-4 manifest | `kubernetes-mcp-server` + `prometheus-mcp-server` deployment, placeholders, in-cluster DNS, how agents discover the servers |
| [Experiment Flow](docs/experiment-flow.md) | End-to-end | A single run from UI click to PDF — every step, every code reference, every failure mode |
| [Development](docs/development.md) | Local dev | Prerequisites, monorepo bootstrap vs standalone, Makefile targets, image map, full env-var reference, common workflows |

---

## Quick start

For the fastest path — including LiteLLM, MongoDB, and the agents — use the monorepo
bootstrap from the repo root:

```bash
cd /srv/projects/ace-monorepo
./scripts/start-local-services.sh     # mongo (rs0) + AgentCert + LiteLLM
./scripts/build-and-push.sh           # rebuild custom images
```

To run AgentCert standalone (Go services in `go run` mode, web in `yarn dev`), see [`docs/development.md`](docs/development.md).

UI: https://localhost:2001 · GraphQL playground: http://localhost:8080

---

## Repository layout

```
AgentCert/
├── chaoscenter/                      # Forked & extended LitmusChaos ChaosCenter
│   ├── authentication/               # JWT REST :3000 + gRPC :3030
│   ├── graphql/
│   │   ├── definitions/shared/       # *.graphqls schema (incl. agent_registry, fault_studio)
│   │   └── server/                   # Go GraphQL server :8080
│   │       ├── graph/                # Resolvers + generated model
│   │       └── pkg/                  # Feature packages (see docs/architecture.md)
│   ├── subscriber/                   # Runs in target cluster, dispatches Argo
│   ├── dex-server/                   # OIDC
│   ├── event-tracker/                # K8s events → ChaosEngine
│   ├── upgrade-agents/               # Per-version migration tooling
│   ├── web/                          # React/TypeScript SPA
│   ├── manifests/                    # Top-level install YAML (CRDs, Litmus, Argo)
│   └── Makefile                      # lint / test / docker.buildx / push
│
├── genhash/                          # bcrypt password hash utility
├── docs/                             # ⭐ Subsystem docs — start here
├── setup.md                          # Long-form local-dev guide
├── LICENSE                           # Apache 2.0
├── NOTICE                            # Upstream attribution (required by Apache 2.0)
└── README.md                         # ← you are here
```

---

## Related repositories in the monorepo

| Repository | Role |
|------------|------|
| [`chaos-charts`](../../tree/chaos-charts) | Fault catalogue (ChaosHub source) |
| [`agent-charts`](../../tree/agent-charts) | Helm charts for AI agents + `install-agent` image |
| [`app-charts`](../../tree/app-charts) | Helm charts for target apps + `install-app` image |
| [`agent-sidecar`](../../tree/agent-sidecar) | Transparent HTTP proxy stamping experiment identity onto LLM calls |
| [`flash-agent`](../../tree/flash-agent) | Reference FLASH-style ITOps agent under test |
| [`agentcert-stack`](../../tree/agentcert-stack) | LiteLLM bootstrap (Docker Compose + K8s manifests + model registry) |
| [`certifier`](../../tree/certifier) | Trace-to-certification pipeline (4 phases → 12-section HTML/PDF report) |

---

## License

<<<<<<< HEAD
AgentCert is licensed under the **Apache License, Version 2.0** — see [LICENSE](LICENSE).

This project is a fork of [LitmusChaos ChaosCenter v3.0.0](https://github.com/litmuschaos/litmus)
(Apache 2.0, Copyright 2016-2024 LitmusChaos Authors). Files derived from that upstream
retain their original Apache 2.0 headers. AgentCert additions and modifications are
Copyright 2026 AgentCert Authors, also under Apache 2.0.

See [NOTICE](NOTICE) for full upstream attribution and third-party dependency information
as required by Apache 2.0 Section 4(d).
=======
Apache 2.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE). Upstream LitmusChaos code retains its Apache-2.0
attribution wherever it survives unchanged.
>>>>>>> 66bc0c8 (chore(license): replace MIT with Apache 2.0, add NOTICE)
