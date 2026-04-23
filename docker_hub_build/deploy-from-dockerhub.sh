#!/bin/bash
# Team install script: pull all AgentCert images from Docker Hub,
# load into minikube, and sync the running litmusportal-server env vars.
#
# Run this instead of build-all.sh when you don't have source repos.
# Prerequisites: docker, kubectl, minikube running, .env file configured.
#
# Usage:
#   bash deploy-from-dockerhub.sh --env-file /path/to/.env [--llm 1|azure]
set -e

SERVER_NAMESPACE="litmus-chaos"
SERVER_DEPLOYMENT="litmusportal-server"
LITELLM_NAMESPACE="litellm"
LITELLM_DEPLOYMENT="litellm-proxy"
LITELLM_DIR="/mnt/d/Studies/agent-charts/litellm"
ENV_FILE="/mnt/d/Studies/AgentCert/local-custom/config/.env"
LLM_ARG=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --env-file)  ENV_FILE="${2:-}";  shift 2 ;;
        --llm)       LLM_ARG="${2:-}";   shift 2 ;;
        *)           shift ;;
    esac
done

if [[ ! -f "${ENV_FILE}" ]]; then
    echo "[ERROR] Env file not found: ${ENV_FILE}" >&2
    exit 1
fi

read_env_value() {
    local key="$1"
    local value
    value=$(grep -E "^${key}=" "${ENV_FILE}" | tail -1 | cut -d'=' -f2- || true)
    value=$(echo "${value}" | tr -d '\r\n')
    value=${value#"\""}; value=${value%"\""}
    value=${value#"'"}; value=${value%"'"}
    echo "${value}"
}

require_cmd() { command -v "$1" >/dev/null 2>&1 || { echo "[ERROR] Required command not found: $1" >&2; exit 1; }; }
require_cmd docker
require_cmd kubectl
require_cmd minikube

echo ""
echo "========================================"
echo " AgentCert Deploy from Docker Hub"
echo "========================================"
echo ""

# ── 1. Pull + load all custom images ────────────────────────────────────────
IMAGES=(
    "agentcert/agentcert-install-agent:latest"
    "agentcert/agentcert-install-app:latest"
    "agentcert/agent-sidecar:latest"
    "agentcert/agentcert-flash-agent:latest"
    "agentcert/agentcert-litellm-proxy:latest"
)

for img in "${IMAGES[@]}"; do
    echo "[INFO] Pulling ${img}"
    docker pull "${img}"
    echo "[INFO] Loading ${img} into minikube"
    minikube image load "${img}"
    echo "[OK] ${img}"
done

# ── 2. Apply LiteLLM k8s resources (secrets, configmap, deployment) ─────────
if [[ -n "${LLM_ARG}" ]]; then
    case "${LLM_ARG}" in
        1|azure)  PROFILE="azure" ;;
        2|openai) PROFILE="openai" ;;
        3|all)    PROFILE="all" ;;
        *) echo "[ERROR] Unknown --llm value '${LLM_ARG}'. Use 1/azure, 2/openai, or 3/all." >&2; exit 1 ;;
    esac
elif [[ -n "${LITELLM_PROFILE:-}" ]]; then
    PROFILE="${LITELLM_PROFILE}"
else
    PROFILE=$(read_env_value "LITELLM_PROFILE"); PROFILE="${PROFILE:-azure}"
fi

AZURE_API_KEY=$(read_env_value "AZURE_OPENAI_KEY")
[[ -z "${AZURE_API_KEY}" ]] && AZURE_API_KEY=$(read_env_value "AZURE_OPENAI_API_KEY")
AZURE_API_BASE=$(read_env_value "AZURE_OPENAI_ENDPOINT")
AZURE_OPENAI_DEPLOYMENT=$(read_env_value "AZURE_OPENAI_DEPLOYMENT")
AZURE_API_VERSION=$(read_env_value "AZURE_OPENAI_API_VERSION")
AZURE_MODEL="azure/${AZURE_OPENAI_DEPLOYMENT}"
OPENAI_API_KEY=$(read_env_value "OPENAI_API_KEY")
LITELLM_MASTER_KEY=$(read_env_value "LITELLM_MASTER_KEY"); LITELLM_MASTER_KEY="${LITELLM_MASTER_KEY:-sk-litellm-local-dev}"
LANGFUSE_PUBLIC_KEY=$(read_env_value "LANGFUSE_PUBLIC_KEY")
LANGFUSE_SECRET_KEY=$(read_env_value "LANGFUSE_SECRET_KEY")
LANGFUSE_HOST=$(read_env_value "LANGFUSE_HOST")

if [[ -d "${LITELLM_DIR}" ]]; then
    echo "[INFO] Applying LiteLLM k8s resources (profile: ${PROFILE})"
    kubectl apply -f "${LITELLM_DIR}/namespace.yaml"
    sed "s/model_name: LITELLM_MODEL_NAME/model_name: ${AZURE_OPENAI_DEPLOYMENT}/g" \
        "${LITELLM_DIR}/configmap.yaml" | kubectl apply -f -
    kubectl -n "${LITELLM_NAMESPACE}" create secret generic litellm-secrets \
        --from-literal=AZURE_API_KEY="${AZURE_API_KEY}" \
        --from-literal=AZURE_API_BASE="${AZURE_API_BASE}" \
        --from-literal=AZURE_MODEL="${AZURE_MODEL}" \
        --from-literal=AZURE_API_VERSION="${AZURE_API_VERSION}" \
        --from-literal=OPENAI_API_KEY="${OPENAI_API_KEY}" \
        --from-literal=LITELLM_MASTER_KEY="${LITELLM_MASTER_KEY}" \
        --from-literal=LANGFUSE_PUBLIC_KEY="${LANGFUSE_PUBLIC_KEY}" \
        --from-literal=LANGFUSE_SECRET_KEY="${LANGFUSE_SECRET_KEY}" \
        --from-literal=LANGFUSE_HOST="${LANGFUSE_HOST}" \
        --dry-run=client -o yaml | kubectl apply -f -
    kubectl apply -f "${LITELLM_DIR}/deployment.yaml"
    kubectl -n "${LITELLM_NAMESPACE}" set image deployment/"${LITELLM_DEPLOYMENT}" \
        litellm="agentcert/agentcert-litellm-proxy:latest" >/dev/null
    kubectl -n "${LITELLM_NAMESPACE}" rollout restart deployment/"${LITELLM_DEPLOYMENT}" >/dev/null
    kubectl -n "${LITELLM_NAMESPACE}" rollout status deployment/"${LITELLM_DEPLOYMENT}" --timeout=180s
    echo "[OK] LiteLLM deployed"
else
    echo "[WARN] LiteLLM manifest dir not found at ${LITELLM_DIR}; skipping LiteLLM k8s apply"
fi

# ── 3. Sync litmusportal-server env vars ─────────────────────────────────────
if kubectl get deployment "${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" >/dev/null 2>&1; then
    echo "[INFO] Syncing litmusportal-server env vars..."

    litellm_master_key="${LITELLM_MASTER_KEY}"
    openai_base_url=$(read_env_value "OPENAI_BASE_URL"); openai_base_url="${openai_base_url:-http://litellm-proxy.litellm.svc.cluster.local:4000/v1}"
    model_alias="${AZURE_OPENAI_DEPLOYMENT:-gpt-4}"
    k8s_mcp_url=$(read_env_value "K8S_MCP_URL");         k8s_mcp_url="${k8s_mcp_url:-http://kubernetes-mcp-server.litmus-exp.svc.cluster.local:8081/mcp}"
    prom_mcp_url=$(read_env_value "PROM_MCP_URL");        prom_mcp_url="${prom_mcp_url:-http://prometheus-mcp-server.litmus-exp.svc.cluster.local:9090/mcp}"
    chaos_ns=$(read_env_value "CHAOS_NAMESPACE");          chaos_ns="${chaos_ns:-litmus-exp}"
    pre_cleanup=$(read_env_value "PRE_CLEANUP_WAIT_SECONDS"); pre_cleanup="${pre_cleanup:-0}"
    install_agent_img=$(read_env_value "INSTALL_AGENT_IMAGE"); install_agent_img="${install_agent_img:-agentcert/agentcert-install-agent:latest}"
    flash_agent_img=$(read_env_value "FLASH_AGENT_IMAGE");     flash_agent_img="${flash_agent_img:-agentcert/agentcert-flash-agent:latest}"
    sidecar_img=$(read_env_value "AGENT_SIDECAR_IMAGE");       sidecar_img="${sidecar_img:-agentcert/agent-sidecar:latest}"

    kubectl set env deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" \
        INSTALL_AGENT_IMAGE="${install_agent_img}" \
        FLASH_AGENT_IMAGE="${flash_agent_img}" \
        AGENT_SIDECAR_IMAGE="${sidecar_img}" \
        LITELLM_MASTER_KEY="${litellm_master_key}" \
        OPENAI_API_KEY="${litellm_master_key}" \
        OPENAI_BASE_URL="${openai_base_url}" \
        MODEL_ALIAS="${model_alias}" \
        K8S_MCP_URL="${k8s_mcp_url}" \
        PROM_MCP_URL="${prom_mcp_url}" \
        CHAOS_NAMESPACE="${chaos_ns}" \
        PRE_CLEANUP_WAIT_SECONDS="${pre_cleanup}" >/dev/null
    kubectl rollout status deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" --timeout=180s >/dev/null
    echo "[OK] litmusportal-server env synced"
else
    echo "[WARN] ${SERVER_NAMESPACE}/${SERVER_DEPLOYMENT} not found; skipping server env sync"
fi

echo ""
echo "========================================"
echo " Deploy from Docker Hub: DONE"
echo "========================================"
echo ""
