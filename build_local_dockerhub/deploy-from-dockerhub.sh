#!/bin/bash
set -euo pipefail
# =============================================================================
# build_local_dockerhub/deploy-from-dockerhub.sh
#
# Pulls AgentCert images from Docker Hub and deploys them to local minikube.
# No build step — use this after azure_build/build-all.sh has pushed new images.
#
# Usage:
#   bash deploy-from-dockerhub.sh [--env-file /path/.env]
#
# What it does (mirrors the local build-* scripts, minus the docker build):
#   1. docker pull each image from Docker Hub (tags read from .env)
#   2. Clean old ci-* tags from minikube for each image
#   3. minikube image load each image (latest + ci tag)
#   4. kubectl set env litmusportal-server with all image + config vars
#   5. kubectl rollout status litmusportal-server
#   6. kubectl set image flash-agent deployment + cronjob in sock-shop (if present)
# =============================================================================

ENV_FILE="/mnt/d/Studies/AgentCert/local-custom/config/.env"
SERVER_NAMESPACE="litmus-chaos"
SERVER_DEPLOYMENT="litmusportal-server"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file) ENV_FILE="${2:-}"; shift 2 ;;
    *) echo "[ERROR] Unknown option: $1" >&2; exit 1 ;;
  esac
done

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "[ERROR] .env file not found: ${ENV_FILE}" >&2; exit 1
fi

# Colors
RED='\033[0;31m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'; NC='\033[0m'
log_info()    { echo -e "${CYAN}[INFO]${NC}  $*"; }
log_success() { echo -e "${GREEN}[OK]${NC}    $*"; }
log_error()   { echo -e "${RED}[ERROR]${NC}  $*"; }

# Read a value from .env (strips quotes + CR)
read_env_value() {
  local key="$1" default="${2:-}"
  local val
  val=$(grep -E "^${key}=" "${ENV_FILE}" | tail -1 | cut -d'=' -f2- | tr -d '\r\n' || true)
  val=${val#'"'}; val=${val%'"'}; val=${val#"'"}; val=${val%"'"}
  echo "${val:-${default}}"
}

# Read image tags from .env
INSTALL_AGENT_IMAGE="$(read_env_value INSTALL_AGENT_IMAGE agentcert/agentcert-install-agent:latest)"
INSTALL_APP_IMAGE="$(read_env_value INSTALL_APPLICATION_IMAGE agentcert/agentcert-install-app:latest)"
FLASH_AGENT_IMAGE="$(read_env_value FLASH_AGENT_IMAGE agentcert/agentcert-flash-agent:latest)"
AGENT_SIDECAR_IMAGE="$(read_env_value AGENT_SIDECAR_IMAGE agentcert/agent-sidecar:latest)"

# Config vars for litmusportal-server (same as build-flash-agent.sh sync)
LITELLM_MASTER_KEY="$(read_env_value LITELLM_MASTER_KEY sk-litellm-local-dev)"
OPENAI_BASE_URL="$(read_env_value OPENAI_BASE_URL http://litellm-proxy.litellm.svc.cluster.local:4000/v1)"
OPENAI_API_KEY="$(read_env_value OPENAI_API_KEY "${LITELLM_MASTER_KEY}")"
MODEL_ALIAS="$(read_env_value AZURE_OPENAI_DEPLOYMENT gpt-4)"
K8S_MCP_URL="$(read_env_value K8S_MCP_URL http://kubernetes-mcp-server.litmus-exp.svc.cluster.local:8081/mcp)"
PROM_MCP_URL="$(read_env_value PROM_MCP_URL http://prometheus-mcp-server.litmus-exp.svc.cluster.local:9090/mcp)"
CHAOS_NAMESPACE="$(read_env_value CHAOS_NAMESPACE litmus-exp)"
PRE_CLEANUP_WAIT="$(read_env_value PRE_CLEANUP_WAIT_SECONDS 0)"

echo ""
echo -e "${CYAN}============================================${NC}"
echo -e "${CYAN}AgentCert: Pull from Docker Hub + Deploy to Minikube${NC}"
echo -e "${CYAN}============================================${NC}"
echo ""
log_info "INSTALL_AGENT_IMAGE:  ${INSTALL_AGENT_IMAGE}"
log_info "INSTALL_APP_IMAGE:    ${INSTALL_APP_IMAGE}"
log_info "FLASH_AGENT_IMAGE:    ${FLASH_AGENT_IMAGE}"
log_info "AGENT_SIDECAR_IMAGE:  ${AGENT_SIDECAR_IMAGE}"
echo ""

# ── Step 1: Docker Hub login ────────────────────────────────────────────────
DOCKERHUB_USERNAME="$(read_env_value DOCKERHUB_USERNAME)"
DOCKERHUB_TOKEN="$(read_env_value DOCKERHUB_TOKEN)"
if [[ -n "${DOCKERHUB_USERNAME}" && -n "${DOCKERHUB_TOKEN}" ]]; then
  log_info "Logging into Docker Hub as ${DOCKERHUB_USERNAME}..."
  echo "${DOCKERHUB_TOKEN}" | docker login -u "${DOCKERHUB_USERNAME}" --password-stdin
  log_success "Docker Hub login OK"
else
  log_info "DOCKERHUB_USERNAME/TOKEN not in .env — assuming already logged in"
fi
echo ""

# ── Step 2: Pull + clean + load each image ─────────────────────────────────
# Mirrors what the local build-*.sh scripts do after docker build:
#   - prune old ci-* from local docker
#   - clean old ci-* from minikube
#   - load latest + ci tag into minikube

load_image() {
  local image="$1"        # e.g. agentcert/agentcert-flash-agent:ci-20260424214206
  local grep_name="$2"    # e.g. agentcert-flash-agent  (for grep in minikube image ls)

  local tag
  tag=$(echo "${image}" | cut -d':' -f2)  # ci-* or latest

  log_info "Pulling ${image} from Docker Hub..."
  docker pull "${image}"
  log_success "Pulled: ${image}"

  # Tag as :latest locally so minikube load uses consistent name
  local repo
  repo=$(echo "${image}" | cut -d':' -f1)
  docker tag "${image}" "${repo}:latest"

  log_info "Cleaning old ${grep_name} ci-* images from minikube..."
  minikube image ls 2>/dev/null \
    | grep "${grep_name}:ci-" \
    | grep -v "${tag}" \
    | awk '{print $1}' \
    | xargs -r minikube image rm 2>/dev/null || true
  log_success "Old minikube images cleaned"

  log_info "Loading ${image} into minikube..."
  minikube image load "${image}"
  minikube image load "${repo}:latest"
  log_success "Loaded into minikube: ${image} + ${repo}:latest"
  echo ""
}

load_image "${INSTALL_AGENT_IMAGE}"  "agentcert-install-agent"
load_image "${INSTALL_APP_IMAGE}"    "agentcert-install-app"
load_image "${FLASH_AGENT_IMAGE}"    "agentcert-flash-agent"
load_image "${AGENT_SIDECAR_IMAGE}"  "agent-sidecar"

# ── Step 3: Pull + load all sock-shop images into minikube ─────────────────
# These are fixed third-party images used when an experiment deploys sock-shop.
# Pre-loading ensures imagePullPolicy:IfNotPresent works offline / fast.
load_static_image() {
  local image="$1"
  log_info "Pulling ${image}..."
  docker pull "${image}" || { log_info "Skipping ${image} (pull failed)"; return 0; }
  minikube image load "${image}" || { log_info "Skipping minikube load for ${image}"; return 0; }
  log_success "Loaded: ${image}"
}

log_info "Loading sock-shop images into minikube..."
echo ""
# Sock Shop microservices
load_static_image "weaveworksdemos/front-end:0.3.12"
load_static_image "weaveworksdemos/catalogue:0.3.5"
load_static_image "weaveworksdemos/catalogue-db:0.3.0"
load_static_image "weaveworksdemos/carts:0.4.8"
load_static_image "weaveworksdemos/orders:0.4.7"
load_static_image "weaveworksdemos/payment:0.4.3"
load_static_image "weaveworksdemos/shipping:0.4.8"
load_static_image "weaveworksdemos/user:0.4.7"
load_static_image "weaveworksdemos/user-db:0.4.0"
load_static_image "weaveworksdemos/queue-master:0.3.1"
load_static_image "mongo:latest"
load_static_image "rabbitmq:3.6.8"
# Observability
load_static_image "litmuschaos/chaos-exporter:1.13.3"
load_static_image "prom/prometheus:v2.25.0"
load_static_image "grafana/grafana:latest"
# MCP tools
load_static_image "quay.io/containers/kubernetes_mcp_server:latest"
load_static_image "agentcert/prometheus-mcp-server:latest"
log_success "All sock-shop images loaded into minikube"
echo ""

# ── Step 4: Pull + load LiteLLM proxy image + restart pod ─────────────────
LITELLM_IMAGE="$(read_env_value LITELLM_PROXY_IMAGE docker.io/litellm/litellm:v1.82.0-stable)"
log_info "Pulling LiteLLM proxy image: ${LITELLM_IMAGE}..."
docker pull "${LITELLM_IMAGE}" || log_info "LiteLLM pull failed — skipping"
minikube image load "${LITELLM_IMAGE}" || log_info "LiteLLM minikube load failed — skipping"
log_success "Loaded: ${LITELLM_IMAGE}"

if kubectl get deployment litellm-proxy -n litellm >/dev/null 2>&1; then
  log_info "Restarting litellm-proxy deployment..."
  kubectl rollout restart deployment/litellm-proxy -n litellm
  kubectl rollout status deployment/litellm-proxy -n litellm --timeout=120s
  log_success "litellm-proxy restarted"
else
  log_info "litellm-proxy deployment not found — skipping restart"
fi
echo ""

# ── Step 5: kubectl set env litmusportal-server + rollout ──────────────────
if ! command -v kubectl >/dev/null 2>&1; then
  log_error "kubectl not found — skipping deployment sync"; exit 1
fi
if ! kubectl get deployment "${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" >/dev/null 2>&1; then
  log_error "${SERVER_NAMESPACE}/${SERVER_DEPLOYMENT} not found — is minikube running with litmus deployed?" >&2
  exit 1
fi

log_info "Syncing litmusportal-server env vars..."
kubectl set env deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" \
  INSTALL_AGENT_IMAGE="${INSTALL_AGENT_IMAGE}" \
  INSTALL_APPLICATION_IMAGE="${INSTALL_APP_IMAGE}" \
  FLASH_AGENT_IMAGE="${FLASH_AGENT_IMAGE}" \
  AGENT_SIDECAR_IMAGE="${AGENT_SIDECAR_IMAGE}" \
  LITELLM_MASTER_KEY="${LITELLM_MASTER_KEY}" \
  OPENAI_API_KEY="${OPENAI_API_KEY}" \
  OPENAI_BASE_URL="${OPENAI_BASE_URL}" \
  MODEL_ALIAS="${MODEL_ALIAS}" \
  K8S_MCP_URL="${K8S_MCP_URL}" \
  PROM_MCP_URL="${PROM_MCP_URL}" \
  CHAOS_NAMESPACE="${CHAOS_NAMESPACE}" \
  PRE_CLEANUP_WAIT_SECONDS="${PRE_CLEANUP_WAIT}" >/dev/null

log_info "Rolling out ${SERVER_DEPLOYMENT}..."
kubectl rollout status deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" --timeout=120s
log_success "litmusportal-server updated and healthy"
echo ""

# ── Step 6: Sync flash-agent deployment + cronjob in sock-shop ────────────
FA_NAMESPACE="sock-shop"
for workload_type in deployment cronjob; do
  workload_name="flash-agent"
  [[ "${workload_type}" == "cronjob" ]] && workload_name="flash-agent-cronjob"
  if kubectl -n "${FA_NAMESPACE}" get "${workload_type}" "${workload_name}" >/dev/null 2>&1; then
    log_info "Updating ${FA_NAMESPACE}/${workload_name} image -> ${FLASH_AGENT_IMAGE}"
    kubectl -n "${FA_NAMESPACE}" set image "${workload_type}/${workload_name}" \
      agent="${FLASH_AGENT_IMAGE}" >/dev/null || true
    if [[ "${workload_type}" == "deployment" ]]; then
      kubectl -n "${FA_NAMESPACE}" rollout status "deployment/${workload_name}" \
        --timeout=120s >/dev/null || true
    fi
    log_success "${workload_name} updated"
  else
    log_info "${FA_NAMESPACE}/${workload_name} not found — skipping"
  fi
done
echo ""

docker logout >/dev/null 2>&1 || true

echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}Done! All images loaded into minikube and${NC}"
echo -e "${GREEN}litmusportal-server synced with new tags.${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""
