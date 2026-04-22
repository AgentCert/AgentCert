#!/bin/bash
set -euo pipefail

# Wrapper script to rebuild and deploy all AgentCert components in sequence:
# Works from Ubuntu (direct bash execution)
# 1. App chart (Sock Shop) with local-mode
# 2. Cluster deployment sync
# 3. LiteLLM proxy (ensures secrets/keys in sync)
# 4. Install-agent image
# 5. Agent-sidecar image
# 6. Flash-agent image

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/local-custom/config/.env"

# ---------------------------------------------------------------------------
# Argument parsing
# --llm 1|azure  / 2|openai / 3|all  -> sets LITELLM_PROFILE, skipping prompt
# ---------------------------------------------------------------------------
LLM_ARG=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --llm)
      LLM_ARG="${2:-}"
      shift 2
      ;;
        --env-file)
            ENV_FILE="${2:-}"
            shift 2
            ;;
    *)
      shift
      ;;
  esac
done

if [[ ! -f "${ENV_FILE}" ]]; then
    echo "[ERROR] Env file not found: ${ENV_FILE}" >&2
    exit 1
fi

if [[ -n "${LLM_ARG}" ]]; then
  case "${LLM_ARG}" in
    1|azure)  export LITELLM_PROFILE="azure" ;;
    2|openai) export LITELLM_PROFILE="openai" ;;
    3|all)    export LITELLM_PROFILE="all" ;;
    *)
      echo "[ERROR] Unknown --llm value '${LLM_ARG}'. Use 1/azure, 2/openai, or 3/all." >&2
      exit 1
      ;;
  esac
fi

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

function log_info() {
    echo -e "${CYAN}[INFO]${NC}  $*"
}

function log_success() {
    echo -e "${GREEN}[OK]${NC}    $*"
}

function log_warn() {
    echo -e "${YELLOW}[WARN]${NC}   $*"
}

function log_error() {
    echo -e "${RED}[ERROR]${NC}  $*"
}

choose_cluster_deploy_script() {
    local namespace="litmus-chaos"
    local infra_namespace="litmus-exp"
    local auth_pod=""
    local graphql_pod=""

    if ! command -v kubectl >/dev/null 2>&1; then
        log_warn "kubectl not found, defaulting to scratch deploy script" >&2
        echo "build-and-deploy-scratch.sh"
        return 0
    fi

    if ! kubectl get namespace "$namespace" >/dev/null 2>&1; then
        log_info "Namespace $namespace not found; creating it" >&2
        kubectl create namespace "$namespace" >/dev/null 2>&1 || true
    fi
    if ! kubectl get namespace "$infra_namespace" >/dev/null 2>&1; then
        log_info "Namespace $infra_namespace not found; creating it" >&2
        kubectl create namespace "$infra_namespace" >/dev/null 2>&1 || true
    fi

    if ! kubectl get namespace "$namespace" >/dev/null 2>&1; then
        log_warn "Namespace $namespace still unavailable; selecting scratch deploy script" >&2
        echo "build-and-deploy-scratch.sh"
        return 0
    fi

    auth_pod=$(kubectl get pods -n "$namespace" -l component=litmusportal-auth-server -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
    graphql_pod=$(kubectl get pods -n "$namespace" -l component=litmusportal-server -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

    if [[ -n "$auth_pod" && -n "$graphql_pod" ]]; then
        log_info "Auth and GraphQL pods detected; selecting incremental deploy script" >&2
        echo "build-and-deploy.sh"
    else
        log_info "Auth/GraphQL pod(s) missing; selecting scratch deploy script" >&2
        echo "build-and-deploy-scratch.sh"
    fi
}

echo ""
echo -e "${CYAN}================================${NC}"
echo -e "${CYAN}AgentCert Build+Deploy Pipeline${NC}"
echo -e "${CYAN}================================${NC}"
echo ""

# Step 1: Build and deploy app chart (Sock Shop)
log_info "Starting: Build app chart (Sock Shop)"
if cd /mnt/d/Studies/app-charts/install-app && bash build-and-deploy-app-chart.sh --local-mode; then
    log_success "Completed: Build app chart (Sock Shop)"
else
    log_error "Failed: Build app chart (Sock Shop)"
    exit 1
fi

echo ""

# Step 2: Sync cluster deployment
log_info "Starting: Sync cluster deployment"
DEPLOY_SCRIPT="$(choose_cluster_deploy_script)"
log_info "Using deploy script: ${DEPLOY_SCRIPT}"
if cd "${SCRIPT_DIR}/local-custom/scripts" && bash "$DEPLOY_SCRIPT" --env-file "${ENV_FILE}"; then
    log_success "Completed: Sync cluster deployment"
else
    log_error "Failed: Sync cluster deployment"
    exit 1
fi

echo ""

# Step 3: Build and deploy LiteLLM proxy (ensures secrets and keys are in sync)
log_info "Starting: Build LiteLLM proxy"
if bash "${SCRIPT_DIR}/build-litellm.sh" --env-file "${ENV_FILE}"; then
    log_success "Completed: Build LiteLLM proxy"
else
    log_error "Failed: Build LiteLLM proxy"
    exit 1
fi

echo ""

# Step 4: Build install-agent image
log_info "Starting: Build install-agent image"
if DOCKER_BUILDKIT=1 bash /mnt/d/Studies/AgentCert/build-install-agent.sh; then
    log_success "Completed: Build install-agent image"
else
    log_error "Failed: Build install-agent image"
    exit 1
fi

echo ""

# Step 5: Build agent-sidecar image
log_info "Starting: Build agent-sidecar image"
if bash /mnt/d/Studies/AgentCert/build-agent-sidecar.sh; then
    log_success "Completed: Build agent-sidecar image"
else
    log_error "Failed: Build agent-sidecar image"
    exit 1
fi

echo ""

# Step 6: Build flash-agent image (syncs LITELLM_MASTER_KEY + MCP URLs to server)
log_info "Starting: Build flash-agent image"
if bash /mnt/d/Studies/AgentCert/build-flash-agent.sh; then
    log_success "Completed: Build flash-agent image"
else
    log_error "Failed: Build flash-agent image"
    exit 1
fi

echo ""
echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}All builds completed successfully!${NC}"
echo -e "${GREEN}================================${NC}"
echo ""
