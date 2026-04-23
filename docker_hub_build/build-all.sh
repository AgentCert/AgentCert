#!/bin/bash
# Build all AgentCert images and push to Docker Hub.
# Run this after pushing code changes; the team then runs deploy-from-dockerhub.sh.
#
# Prerequisites: docker login (docker.io) done before running this script.
#
# Usage:
#   bash build-all.sh --env-file /path/to/.env [--llm 1|azure] [--skip-app] [--skip-litellm]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="/mnt/d/Studies/AgentCert/local-custom/config/.env"
LLM_ARG=""
SKIP_APP=false
SKIP_LITELLM=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --env-file)     ENV_FILE="${2:-}";     shift 2 ;;
        --llm)          LLM_ARG="${2:-}";      shift 2 ;;
        --skip-app)     SKIP_APP=true;         shift ;;
        --skip-litellm) SKIP_LITELLM=true;     shift ;;
        *) shift ;;
    esac
done

if [[ ! -f "${ENV_FILE}" ]]; then
    echo "[ERROR] Env file not found: ${ENV_FILE}" >&2
    exit 1
fi

[[ -n "${LLM_ARG}" ]] && export LITELLM_PROFILE="${LLM_ARG}"

RED='\033[0;31m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'; NC='\033[0m'
log_info()    { echo -e "${CYAN}[INFO]${NC}  $*"; }
log_success() { echo -e "${GREEN}[OK]${NC}    $*"; }
log_error()   { echo -e "${RED}[ERROR]${NC}  $*"; }

# Verify docker is logged in to Docker Hub
if ! docker info --format '{{.RegistryConfig.IndexConfigs}}' 2>/dev/null | grep -q "docker.io"; then
    echo "[WARN] Not logged in to docker.io. Run 'docker login' first if pushes fail."
fi

echo ""
echo -e "${CYAN}============================================${NC}"
echo -e "${CYAN} AgentCert Build + Push to Docker Hub${NC}"
echo -e "${CYAN}============================================${NC}"
echo ""

# ── install-app ──────────────────────────────────────────────────────────────
if [[ "${SKIP_APP}" == "false" ]]; then
    log_info "Building install-app..."
    if bash "${SCRIPT_DIR}/build-install-app.sh" --env-file "${ENV_FILE}"; then
        log_success "install-app pushed"
    else
        log_error "install-app failed"; exit 1
    fi
    echo ""
fi

# ── LiteLLM ──────────────────────────────────────────────────────────────────
if [[ "${SKIP_LITELLM}" == "false" ]]; then
    log_info "Building LiteLLM proxy..."
    LLM_EXTRA=""
    [[ -n "${LLM_ARG}" ]] && LLM_EXTRA="--llm ${LLM_ARG}"
    if bash "${SCRIPT_DIR}/build-litellm.sh" --env-file "${ENV_FILE}" ${LLM_EXTRA}; then
        log_success "LiteLLM pushed"
    else
        log_error "LiteLLM failed"; exit 1
    fi
    echo ""
fi

# ── install-agent ─────────────────────────────────────────────────────────────
log_info "Building install-agent..."
if bash "${SCRIPT_DIR}/build-install-agent.sh" --env-file "${ENV_FILE}"; then
    log_success "install-agent pushed"
else
    log_error "install-agent failed"; exit 1
fi
echo ""

# ── agent-sidecar ─────────────────────────────────────────────────────────────
log_info "Building agent-sidecar..."
if bash "${SCRIPT_DIR}/build-agent-sidecar.sh" --env-file "${ENV_FILE}"; then
    log_success "agent-sidecar pushed"
else
    log_error "agent-sidecar failed"; exit 1
fi
echo ""

# ── flash-agent ───────────────────────────────────────────────────────────────
log_info "Building flash-agent..."
if bash "${SCRIPT_DIR}/build-flash-agent.sh" --env-file "${ENV_FILE}"; then
    log_success "flash-agent pushed"
else
    log_error "flash-agent failed"; exit 1
fi
echo ""

echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN} All images built and pushed to Docker Hub!${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""
echo "Team can now run:"
echo "  bash docker_hub_build/deploy-from-dockerhub.sh --env-file <their-.env> --llm 1"
echo ""
