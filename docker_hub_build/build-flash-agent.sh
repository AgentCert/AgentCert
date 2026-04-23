#!/bin/bash
# Build flash-agent image and push to Docker Hub.
# Usage: bash build-flash-agent.sh [--env-file <path>] [--tag <tag>]
set -e

SERVER_NAMESPACE="litmus-chaos"
SERVER_DEPLOYMENT="litmusportal-server"
ENV_FILE="/mnt/d/Studies/AgentCert/local-custom/config/.env"
PUSH_TAG=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --env-file)  ENV_FILE="${2:-}";  shift 2 ;;
        --tag)       PUSH_TAG="${2:-}";  shift 2 ;;
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

IMAGE_TAG="${PUSH_TAG:-ci-$(date +%Y%m%d%H%M%S)}"
IMAGE="agentcert/agentcert-flash-agent:${IMAGE_TAG}"

echo "[INFO] Building ${IMAGE}"
cd /mnt/d/Studies/flash-agent
docker build -t "${IMAGE}" -f Dockerfile .
docker tag "${IMAGE}" agentcert/agentcert-flash-agent:latest
echo "[OK] Docker build completed"

echo "[INFO] Pushing to Docker Hub..."
docker push "${IMAGE}"
docker push agentcert/agentcert-flash-agent:latest
echo "[OK] Pushed: ${IMAGE} and :latest"

# Sync running server env (LiteLLM keys + MCP URLs + flash-agent image)
if command -v kubectl >/dev/null 2>&1 && \
   kubectl get deployment "${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" >/dev/null 2>&1; then
    echo "[INFO] Syncing live server env..."

    litellm_master_key=$(read_env_value "LITELLM_MASTER_KEY"); litellm_master_key="${litellm_master_key:-sk-litellm-local-dev}"
    openai_base_url=$(read_env_value "OPENAI_BASE_URL");       openai_base_url="${openai_base_url:-http://litellm-proxy.litellm.svc.cluster.local:4000/v1}"
    openai_api_key=$(read_env_value "OPENAI_API_KEY");         openai_api_key="${openai_api_key:-${litellm_master_key}}"
    model_alias=$(read_env_value "AZURE_OPENAI_DEPLOYMENT");   model_alias="${model_alias:-gpt-4}"
    k8s_mcp_url=$(read_env_value "K8S_MCP_URL");               k8s_mcp_url="${k8s_mcp_url:-http://kubernetes-mcp-server.litmus-exp.svc.cluster.local:8081/mcp}"
    prom_mcp_url=$(read_env_value "PROM_MCP_URL");             prom_mcp_url="${prom_mcp_url:-http://prometheus-mcp-server.litmus-exp.svc.cluster.local:9090/mcp}"
    chaos_namespace=$(read_env_value "CHAOS_NAMESPACE");       chaos_namespace="${chaos_namespace:-litmus-exp}"
    pre_cleanup=$(read_env_value "PRE_CLEANUP_WAIT_SECONDS");  pre_cleanup="${pre_cleanup:-0}"

    kubectl set env deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" \
        FLASH_AGENT_IMAGE="agentcert/agentcert-flash-agent:latest" \
        LITELLM_MASTER_KEY="${litellm_master_key}" \
        OPENAI_API_KEY="${openai_api_key}" \
        OPENAI_BASE_URL="${openai_base_url}" \
        MODEL_ALIAS="${model_alias}" \
        K8S_MCP_URL="${k8s_mcp_url}" \
        PROM_MCP_URL="${prom_mcp_url}" \
        CHAOS_NAMESPACE="${chaos_namespace}" \
        PRE_CLEANUP_WAIT_SECONDS="${pre_cleanup}" >/dev/null
    kubectl rollout status deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" --timeout=120s >/dev/null
    echo "[OK] Live server env synced"
else
    echo "[WARN] ${SERVER_NAMESPACE}/${SERVER_DEPLOYMENT} not found; skipping live server env sync"
fi

# Update .env
if grep -q "^FLASH_AGENT_IMAGE=" "${ENV_FILE}"; then
    sed -i "s|^FLASH_AGENT_IMAGE=.*|FLASH_AGENT_IMAGE=agentcert/agentcert-flash-agent:latest|" "${ENV_FILE}"
else
    sed -i "/^INSTALL_AGENT_IMAGE=/a FLASH_AGENT_IMAGE=agentcert/agentcert-flash-agent:latest" "${ENV_FILE}"
fi
echo "[OK] .env updated: FLASH_AGENT_IMAGE=agentcert/agentcert-flash-agent:latest"
