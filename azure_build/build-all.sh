#!/bin/bash
set -euo pipefail

# Wrapper script to rebuild and deploy all AgentCert components in sequence.
# Runs directly on Linux (no wsl wrapper needed).
#
# Usage:
#   bash build-all.sh [--git] --llm 1 --env-file /path/to/.env --paths-file /path/to/build-paths.env
#
# --llm        1|azure / 2|openai / 3|all
# --env-file   path to your .env (default: <agentcert-root>/local-custom/config/.env)
# --paths-file path to build-paths.env (default: same dir as this script/build-paths.env)
# --git        clone (if missing) or git pull each repo before building

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

ENV_FILE=""
PATHS_FILE="${SCRIPT_DIR}/build-paths.env"
LLM_ARG=""
GIT_SYNC=false

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
    --paths-file)
      PATHS_FILE="${2:-}"
      shift 2
      ;;
    --git)
      GIT_SYNC=true
      shift
      ;;
    *)
      shift
      ;;
  esac
done

# ── Load paths file ────────────────────────────────────────────────────────────
if [[ ! -f "${PATHS_FILE}" ]]; then
  echo "[ERROR] Paths file not found: ${PATHS_FILE}" >&2
  echo "[ERROR] Copy azure_build/build-paths.env and update it for your environment." >&2
  exit 1
fi
# shellcheck source=/dev/null
source "${PATHS_FILE}"

# ── Git sync (clone or pull each repo) ────────────────────────────────────────
sync_repo() {
  local dir="$1"
  local url="$2"
  local branch="${GIT_BRANCH:-main}"

  if [[ -z "${url}" ]]; then
    echo "[ERROR] --git flag used but no git URL configured for ${dir}" >&2
    echo "[ERROR] Set the corresponding *_GIT_URL in ${PATHS_FILE}" >&2
    exit 1
  fi

  if [[ -d "${dir}/.git" ]]; then
    echo "[INFO] git pull  ${dir} (branch: ${branch})"
    git -C "${dir}" fetch origin
    git -C "${dir}" checkout "${branch}" 2>/dev/null || true
    git -C "${dir}" reset --hard "origin/${branch}"
  else
    echo "[INFO] git clone ${url} → ${dir} (branch: ${branch})"
    mkdir -p "$(dirname "${dir}")"
    git clone --branch "${branch}" --depth 1 "${url}" "${dir}"
  fi
}

if [[ "${GIT_SYNC}" == "true" ]]; then
  echo "[INFO] Syncing repos from git..."
  sync_repo "${AGENTCERT_ROOT}"    "${AGENTCERT_GIT_URL:-}"
  sync_repo "${APP_CHARTS_ROOT}"   "${APP_CHARTS_GIT_URL:-}"
  sync_repo "${AGENT_CHARTS_ROOT}" "${AGENT_CHARTS_GIT_URL:-}"
  sync_repo "${FLASH_AGENT_ROOT}"  "${FLASH_AGENT_GIT_URL:-}"
  echo "[OK] All repos synced"
fi

# Validate required path variables
for var in AGENTCERT_ROOT APP_CHARTS_ROOT AGENT_CHARTS_ROOT FLASH_AGENT_ROOT; do
  if [[ -z "${!var:-}" ]]; then
    echo "[ERROR] ${var} is not set in ${PATHS_FILE}" >&2
    exit 1
  fi
  if [[ ! -d "${!var}" ]]; then
    echo "[ERROR] ${var}=${!var} — directory not found" >&2
    exit 1
  fi
done

# Default env file relative to AGENTCERT_ROOT
if [[ -z "${ENV_FILE}" ]]; then
  ENV_FILE="${AGENTCERT_ROOT}/local-custom/config/.env"
fi

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

# ── Colours ────────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()    { echo -e "${CYAN}[INFO]${NC}  $*"; }
log_success() { echo -e "${GREEN}[OK]${NC}    $*"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC}   $*"; }
log_error()   { echo -e "${RED}[ERROR]${NC}  $*"; }

# ── Choose deploy script based on cluster state ────────────────────────────────
choose_cluster_deploy_script() {
  local namespace="litmus-chaos"
  local infra_namespace="litmus-exp"

  if ! command -v kubectl >/dev/null 2>&1; then
    log_warn "kubectl not found, defaulting to scratch deploy script" >&2
    echo "build-and-deploy-scratch.sh"
    return 0
  fi

  kubectl get namespace "$namespace" >/dev/null 2>&1 || \
    kubectl create namespace "$namespace" >/dev/null 2>&1 || true
  kubectl get namespace "$infra_namespace" >/dev/null 2>&1 || \
    kubectl create namespace "$infra_namespace" >/dev/null 2>&1 || true

  if ! kubectl get namespace "$namespace" >/dev/null 2>&1; then
    log_warn "Namespace $namespace still unavailable; selecting scratch deploy script" >&2
    echo "build-and-deploy-scratch.sh"
    return 0
  fi

  local auth_pod graphql_pod
  auth_pod=$(kubectl get pods -n "$namespace" -l component=litmusportal-auth-server \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
  graphql_pod=$(kubectl get pods -n "$namespace" -l component=litmusportal-server \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

  if [[ -n "$auth_pod" && -n "$graphql_pod" ]]; then
    log_info "Auth and GraphQL pods detected; selecting incremental deploy script" >&2
    echo "build-and-deploy.sh"
  else
    log_info "Auth/GraphQL pod(s) missing; selecting scratch deploy script" >&2
    echo "build-and-deploy-scratch.sh"
  fi
}

# ── Build scripts (in this azure_build folder) ─────────────────────────────────
BUILD_INSTALL_APP="${SCRIPT_DIR}/build-and-deploy-app-chart.sh"
BUILD_INSTALL_AGENT="${SCRIPT_DIR}/build-install-agent.sh"
BUILD_AGENT_SIDECAR="${SCRIPT_DIR}/build-agent-sidecar.sh"
BUILD_FLASH_AGENT="${SCRIPT_DIR}/build-flash-agent.sh"
BUILD_LITELLM="${SCRIPT_DIR}/build-litellm.sh"

for s in "$BUILD_INSTALL_APP" "$BUILD_INSTALL_AGENT" "$BUILD_AGENT_SIDECAR" "$BUILD_FLASH_AGENT" "$BUILD_LITELLM"; do
  if [[ ! -f "$s" ]]; then
    echo "[ERROR] Build script not found: $s" >&2
    exit 1
  fi
done

echo ""
echo -e "${CYAN}================================${NC}"
echo -e "${CYAN}AgentCert Build+Deploy Pipeline${NC}"
echo -e "${CYAN}================================${NC}"
echo ""
log_info "AGENTCERT_ROOT  = ${AGENTCERT_ROOT}"
log_info "APP_CHARTS_ROOT = ${APP_CHARTS_ROOT}"
log_info "AGENT_CHARTS_ROOT = ${AGENT_CHARTS_ROOT}"
log_info "FLASH_AGENT_ROOT  = ${FLASH_AGENT_ROOT}"
log_info "ENV_FILE        = ${ENV_FILE}"
echo ""

# Step 1: Build and deploy app chart (Sock Shop)
log_info "Starting: Build app chart (Sock Shop)"
if bash "${BUILD_INSTALL_APP}" \
    --local-mode \
    --env-file "${ENV_FILE}" \
    --source-dir "${APP_CHARTS_ROOT}/install-app"; then
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
if cd "${AGENTCERT_ROOT}/local-custom/scripts" && bash "$DEPLOY_SCRIPT" --env-file "${ENV_FILE}"; then
  log_success "Completed: Sync cluster deployment"
else
  log_error "Failed: Sync cluster deployment"
  exit 1
fi
echo ""

# Step 3: Build and deploy LiteLLM proxy
log_info "Starting: Build LiteLLM proxy"
if bash "${BUILD_LITELLM}" --env-file "${ENV_FILE}"; then
  log_success "Completed: Build LiteLLM proxy"
else
  log_error "Failed: Build LiteLLM proxy"
  exit 1
fi
echo ""

# Step 4: Build install-agent image
log_info "Starting: Build install-agent image"
if DOCKER_BUILDKIT=1 bash "${BUILD_INSTALL_AGENT}" \
    --env-file "${ENV_FILE}" \
    --source-dir "${AGENT_CHARTS_ROOT}"; then
  log_success "Completed: Build install-agent image"
else
  log_error "Failed: Build install-agent image"
  exit 1
fi
echo ""

# Step 5: Build agent-sidecar image
log_info "Starting: Build agent-sidecar image"
if bash "${BUILD_AGENT_SIDECAR}" \
    --env-file "${ENV_FILE}" \
    --source-dir "${AGENTCERT_ROOT}/agent-sidecar"; then
  log_success "Completed: Build agent-sidecar image"
else
  log_error "Failed: Build agent-sidecar image"
  exit 1
fi
echo ""

# Step 6: Build flash-agent image
log_info "Starting: Build flash-agent image"
if bash "${BUILD_FLASH_AGENT}" \
    --env-file "${ENV_FILE}" \
    --source-dir "${FLASH_AGENT_ROOT}"; then
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
