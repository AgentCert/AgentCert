# Development

How to build, configure, and run AgentCert locally.

---

## Prerequisites

| Requirement | Version | Notes |
|---|---|---|
| Go | 1.24+ | Backend services + `genhash` utility |
| Node.js | 18.x + Yarn | Web UI |
| Python | 3.10+ | Only needed for LiteLLM bring-up |
| Docker | 20.x+ | Multi-stage image builds |
| Kubernetes | 1.28+ (Minikube / kind / managed) | Experiment plane |
| MongoDB | 5.0 (single-node replica set) | App database |
| Langfuse | SaaS or self-hosted | LLM observability |
| OpenAI / Azure OpenAI / Gemini | — | LLM backend (typically via LiteLLM) |

---

## Two paths

### Option A — Monorepo bootstrap (recommended)

The fastest, most reproducible way. Brings up MongoDB (shared `rs0`), LiteLLM,
AgentCert, the agents, and the certifier in one go.

```bash
# From the monorepo root
cd /srv/projects/ace-monorepo

./scripts/start-local-services.sh             # mongo (rs0) + AgentCert + LiteLLM
./scripts/build-and-push.sh                   # rebuild custom images
```

### Option B — Submodule standalone

Use when you want to iterate on this submodule in isolation, without the rest of
the platform running.

```bash
# 1) MongoDB (single-node replica set with admin auth)
docker run -d --name mongo -p 27017:27017 mongo:5 --replSet rs0
docker exec mongo mongosh --eval "rs.initiate()"

# 2) Generate admin bcrypt hash
cd genhash && go run . litmus       # use the hash when seeding the admin user

# 3) Auth service
cd ../chaoscenter/authentication/api
go run main.go                      # :3000 REST, :3030 gRPC

# 4) GraphQL server (new terminal)
cd ../../graphql/server
go run server.go                    # :8080

# 5) Web UI (new terminal)
cd ../../web
yarn install
yarn dev                            # https://localhost:2001

# 6) (Optional) Minikube + Litmus
kubectl apply -f ../manifests/litmus-installation.yaml
```

UI: <https://localhost:2001> &nbsp;·&nbsp; GraphQL playground: <http://localhost:8080>
&nbsp;·&nbsp; Auth REST: <http://localhost:3000>

---

## Building component images

Top-level Makefile: [`chaoscenter/Makefile`](../chaoscenter/Makefile).

```bash
cd chaoscenter

make deps                       # _build_check_docker — verifies docker is present
make frontend-services-checks   # eslint + format checks on the web SPA
make backend-services-checks    # gofmt + go mod tidy on every Go service
make backend-unit-tests         # go test for graphql-server + authentication
make web-unit-tests             # yarn + jest
make docker.buildx              # multi-arch (amd64 + arm64) build
make buildx.push.image          # push every backend service image
make push-portal-component      # docker.buildx + buildx.push.image
make push-frontend              # docker.buildx + buildx.push.frontend
```

### Image map

| Service | Image | Base |
|---|---|---|
| `graphql/server` | `agentcert/litmusportal-server` | `golang:1.24` → `ubi9-minimal` |
| `authentication` | `agentcert/litmusportal-auth-server` | `golang:1.24` → `ubi9-minimal` |
| `subscriber` | `agentcert/litmusportal-subscriber` | `golang:1.24` → `ubi9-minimal` |
| `web` | `agentcert/litmusportal-frontend` | `node:18` → `ubi8-minimal` + nginx |
| `dex-server` | `agentcert/dex-server` | Go → ubi |

The monorepo root provides convenience build scripts that orchestrate the
agent / sidecar / installer / litellm images together:

```bash
# From the monorepo root
./build-flash-agent.sh        # ../flash-agent
./build-agent-sidecar.sh      # ../agent-sidecar
./build-install-agent.sh      # ../agent-charts/install-agent
./build-install-app.sh        # ../app-charts/install-app
./build-litellm.sh            # ../agentcert-stack/litellm-setup
```

---

## Configuration reference

The most-referenced environment variables. The complete list is in the
per-service `.env.example` files.

### Database & auth

| Variable | Example | Purpose |
|---|---|---|
| `DB_SERVER` | `mongodb://localhost:27017` | MongoDB endpoint |
| `DB_USER` / `DB_PASSWORD` | `admin` / `1234` | MongoDB credentials |
| `JWT_SECRET` | `litmus-portal@123` | Auth service token signing |
| `VERSION` | `3.0.0` | Image tag plumbed into every service |

### Subscriber & installer images

| Variable | Example | Purpose |
|---|---|---|
| `INFRA_DEPLOYMENTS` | `false` | Self-hosted (single-cluster) mode |
| `SUBSCRIBER_IMAGE` | `agentcert/litmusportal-subscriber:3.0.0` | Image the server installs into target clusters |
| `INSTALL_AGENT_IMAGE` | `agentcert/agentcert-install-agent:latest` | Synced by `agent-charts/install-agent/build-install-agent.sh` |
| `INSTALL_APP_IMAGE` | `agentcert/agentcert-install-app:latest` | Synced by `app-charts/install-app/build-install-app.sh` |
| `KUBERNETES_MCP_SERVER_IMAGE` | `agentcert/kubernetes-mcp-server:latest` | Image used in stage-4 MCP manifest |
| `PROMETHEUS_MCP_SERVER_IMAGE` | `agentcert/prometheus-mcp-server:latest` | Image used in stage-4 MCP manifest |

### Default hubs (cloned into ChaosHub / AgentHub / AppHub on first run)

| Variable | Default |
|---|---|
| `DEFAULT_HUB_GIT_URL` | `https://github.com/agentcert/chaos-charts` |
| `DEFAULT_AGENT_HUB_GIT_URL` | `https://github.com/agentcert/agent-charts` |
| `DEFAULT_APP_HUB_GIT_URL` | `https://github.com/agentcert/app-charts` |

### LLM + observability

| Variable | Purpose |
|---|---|
| `LITELLM_URL` | LLM gateway URL (default `http://localhost:4000`) |
| `OPENAI_API_KEY` | Provider key (when not using LiteLLM) |
| `LANGFUSE_HOST` / `LANGFUSE_PUBLIC_KEY` / `LANGFUSE_SECRET_KEY` | Server-wide trace ingestion. See [`observability.md`](observability.md) |
| `SLA_*` | SLA targets stamped onto traces. See [`observability.md`](observability.md) |

### Per-namespace token substitutions

Used by the subscriber when applying
[`manifests/namespace/*.yaml`](../chaoscenter/graphql/server/manifests/namespace/):

| Token | Meaning |
|---|---|
| `#{INFRA_NAMESPACE}` | Target namespace |
| `#{KUBERNETES_MCP_SERVER_IMAGE}` | `kubernetes-mcp-server` image |
| `#{PROMETHEUS_MCP_SERVER_IMAGE}` | `prometheus-mcp-server` image |
| `#{PROMETHEUS_MCP_URL}` | Upstream Prometheus URL the MCP queries |
| `#{TOLERATIONS}` | Cluster-specific scheduling tolerations |
| `#{NODE_SELECTOR}` | Cluster-specific node selector |

---

## Common workflows

### Reset a local environment

```bash
cd /srv/projects/ace-monorepo
./scripts/start-local-services.sh --reset       # nukes Mongo, restarts everything
```

### Rebuild + push a single image fast

```bash
cd AgentCert/chaoscenter
docker.buildx PLATFORMS=linux/amd64 push-portal-component
```

### Tail GraphQL server logs

```bash
docker logs -f agentcert-graphql-server
```

### Open the GraphQL playground

<http://localhost:8080>

### Re-seed the admin user with a known password

```bash
cd genhash
go run . <new-password>
# paste the resulting bcrypt hash into the Mongo `users` collection
```

---

## Testing

```bash
cd chaoscenter

make backend-unit-tests         # graphql-server + authentication
make web-unit-tests             # yarn + jest

# Single Go package
cd graphql/server/pkg/agent_registry
go test ./...

# Run with race detector
go test -race ./...

# Coverage
go test -cover ./...
```

The frontend test suite lives under `chaoscenter/web` and uses jest +
react-testing-library.

---

## Related docs

- [`architecture.md`](architecture.md) — what each service is and how they connect
- [`agent-registry.md`](agent-registry.md) — agent registration internals
- [`experiment-flow.md`](experiment-flow.md) — what an end-to-end run looks like
- [`observability.md`](observability.md) — Langfuse + SLA configuration
