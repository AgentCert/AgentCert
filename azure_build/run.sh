#!/bin/bash
# ============================================================================
# AgentCert Remote / Azure VM — Single entrypoint
# ============================================================================
# 1. Builds all Docker images and pushes them to Docker Hub (build-all.sh)
# 2. Starts the AgentCert Go services using the freshly-built images (start-agentcert.sh)
#
# Usage:
#   bash run.sh [OPTIONS]
#
# Options:
#   --git           Clone / pull repos from GitHub before building
#   --llm 1|2|3     LiteLLM profile: 1=azure  2=openai  3=all
#   --env-file PATH Path to .env  (default: <agentcert-root>/local-custom/config/.env)
#   --paths-file PATH Path to build-paths.env (default: <script-dir>/build-paths.env)
#   --skip-build    Skip build-all step (use already-pushed images)
#   --skip-start    Skip start-agentcert step (only build + push)
#   --skip-mongo    Pass through to start-agentcert.sh
#   --skip-frontend Pass through to start-agentcert.sh
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

ENV_FILE=""
PATHS_FILE="${SCRIPT_DIR}/build-paths.env"
LLM_ARG=""
GIT_FLAG=""
SKIP_BUILD=false
SKIP_START=false
SKIP_MONGO=""
SKIP_FRONTEND=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --git)           GIT_FLAG="--git";    shift ;;
    --llm)           LLM_ARG="${2:-}";    shift 2 ;;
    --env-file)      ENV_FILE="${2:-}";   shift 2 ;;
    --paths-file)    PATHS_FILE="${2:-}"; shift 2 ;;
    --skip-build)    SKIP_BUILD=true;     shift ;;
    --skip-start)    SKIP_START=true;     shift ;;
    --skip-mongo)    SKIP_MONGO="--skip-mongo";       shift ;;
    --skip-frontend) SKIP_FRONTEND="--skip-frontend"; shift ;;
    *) echo "[ERROR] Unknown option: $1" >&2; exit 1 ;;
  esac
done

# ── Resolve defaults after parsing ────────────────────────────────────────────
if [[ ! -f "${PATHS_FILE}" ]]; then
  echo "[ERROR] Paths file not found: ${PATHS_FILE}" >&2
  echo "[ERROR] Update azure_build/build-paths.env with your local paths." >&2
  exit 1
fi
# shellcheck source=/dev/null
source "${PATHS_FILE}"

if [[ -z "${ENV_FILE}" ]]; then
  ENV_FILE="${AGENTCERT_ROOT}/local-custom/config/.env"
fi

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "[ERROR] .env file not found: ${ENV_FILE}" >&2
  exit 1
fi

# ── Colors ────────────────────────────────────────────────────────────────────
CYAN='\033[0;36m'; GREEN='\033[0;32m'; NC='\033[0m'

echo ""
echo -e "${CYAN}════════════════════════════════════════════${NC}"
echo -e "${CYAN}   AgentCert Remote Deployment Pipeline     ${NC}"
echo -e "${CYAN}════════════════════════════════════════════${NC}"
echo -e "  env-file:   ${ENV_FILE}"
echo -e "  paths-file: ${PATHS_FILE}"
echo ""

# ── Step 1: Build images + push to Docker Hub ─────────────────────────────────
if [[ "${SKIP_BUILD}" == "false" ]]; then
  echo -e "${CYAN}[1/2] Building images and pushing to Docker Hub...${NC}"

  BUILD_ARGS=(
    ${GIT_FLAG}
    --env-file  "${ENV_FILE}"
    --paths-file "${PATHS_FILE}"
  )
  [[ -n "${LLM_ARG}" ]] && BUILD_ARGS+=(--llm "${LLM_ARG}")

  bash "${SCRIPT_DIR}/build-all.sh" "${BUILD_ARGS[@]}"
  echo -e "${GREEN}[1/2] Build + push complete.${NC}"
else
  echo -e "${CYAN}[1/2] --skip-build: using existing Docker Hub images.${NC}"
fi

echo ""

# ── Step 2: Start AgentCert services ─────────────────────────────────────────
if [[ "${SKIP_START}" == "false" ]]; then
  echo -e "${CYAN}[2/2] Starting AgentCert services...${NC}"

  START_ARGS=(
    --env-file   "${ENV_FILE}"
    --paths-file "${PATHS_FILE}"
  )
  [[ -n "${SKIP_MONGO}" ]]    && START_ARGS+=("${SKIP_MONGO}")
  [[ -n "${SKIP_FRONTEND}" ]] && START_ARGS+=("${SKIP_FRONTEND}")

  bash "${SCRIPT_DIR}/start-agentcert.sh" "${START_ARGS[@]}"
else
  echo -e "${CYAN}[2/2] --skip-start: services not started.${NC}"
fi
