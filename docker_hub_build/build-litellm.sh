#!/bin/bash
# Retag the upstream LiteLLM image as agentcert and push to Docker Hub.
# Also applies k8s secrets and config from .env to the running litellm deployment.
# Usage: bash build-litellm.sh [--env-file <path>] [--llm 1|2|3]
set -e

NAMESPACE="litellm"
DEPLOYMENT="litellm-proxy"
SERVER_NAMESPACE="litmus-chaos"
SERVER_DEPLOYMENT="litmusportal-server"
ENV_FILE="/mnt/d/Studies/AgentCert/local-custom/config/.env"
LITELLM_DIR="/mnt/d/Studies/agent-charts/litellm"
LITELLM_VERSION="v1.82.0"
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

if [[ ! -d "${LITELLM_DIR}" ]]; then
    echo "[ERROR] LiteLLM manifest dir not found: ${LITELLM_DIR}" >&2
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

# Profile selection
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
    echo "Select LiteLLM profile: 1) azure  2) openai  3) all"
    read -r -p "Enter choice [1/2/3] (default: 1): " CHOICE
    case "${CHOICE:-1}" in
        1|azure)  PROFILE="azure" ;;
        2|openai) PROFILE="openai" ;;
        3|all)    PROFILE="all" ;;
        *) echo "[ERROR] Invalid choice." >&2; exit 1 ;;
    esac
fi

SOURCE_IMAGE="docker.io/litellm/litellm:${LITELLM_VERSION}-stable"
PUSH_IMAGE="agentcert/agentcert-litellm-proxy:latest"

echo "[INFO] Profile: ${PROFILE}"
echo "[INFO] Pulling upstream image: ${SOURCE_IMAGE}"
docker pull "${SOURCE_IMAGE}"
docker tag "${SOURCE_IMAGE}" "${PUSH_IMAGE}"

echo "[INFO] Pushing to Docker Hub: ${PUSH_IMAGE}"
docker push "${PUSH_IMAGE}"
echo "[OK] Pushed: ${PUSH_IMAGE}"

# Read secrets from .env
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

if command -v kubectl >/dev/null 2>&1; then
    echo "[INFO] Applying k8s namespace and configmap"
    kubectl apply -f "${LITELLM_DIR}/namespace.yaml"
    sed "s/model_name: LITELLM_MODEL_NAME/model_name: ${AZURE_OPENAI_DEPLOYMENT}/g" \
        "${LITELLM_DIR}/configmap.yaml" | kubectl apply -f -

    echo "[INFO] Applying litellm secret"
    kubectl -n "${NAMESPACE}" create secret generic litellm-secrets \
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
    kubectl -n "${NAMESPACE}" set image deployment/"${DEPLOYMENT}" litellm="${PUSH_IMAGE}" >/dev/null
    kubectl -n "${NAMESPACE}" rollout restart deployment/"${DEPLOYMENT}" >/dev/null
    kubectl -n "${NAMESPACE}" rollout status deployment/"${DEPLOYMENT}" --timeout=180s

    if kubectl get deployment "${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" >/dev/null 2>&1; then
        OPENAI_BASE_URL=$(read_env_value "OPENAI_BASE_URL"); OPENAI_BASE_URL="${OPENAI_BASE_URL:-http://litellm-proxy.litellm.svc.cluster.local:4000/v1}"
        kubectl set env deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" \
            LITELLM_MASTER_KEY="${LITELLM_MASTER_KEY}" \
            OPENAI_API_KEY="${LITELLM_MASTER_KEY}" \
            OPENAI_BASE_URL="${OPENAI_BASE_URL}" \
            MODEL_ALIAS="${AZURE_OPENAI_DEPLOYMENT}" >/dev/null
        kubectl rollout status deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" --timeout=180s >/dev/null
        echo "[OK] Synced litmusportal-server LiteLLM env"
    fi
else
    echo "[WARN] kubectl not found; skipping k8s apply"
fi

# Update .env
if grep -q "^LITELLM_PROXY_IMAGE=" "${ENV_FILE}"; then
    sed -i "s|^LITELLM_PROXY_IMAGE=.*|LITELLM_PROXY_IMAGE=${PUSH_IMAGE}|" "${ENV_FILE}"
else
    printf '\nLITELLM_PROXY_IMAGE=%s\n' "${PUSH_IMAGE}" >> "${ENV_FILE}"
fi
if grep -q "^LITELLM_PROFILE=" "${ENV_FILE}"; then
    sed -i "s|^LITELLM_PROFILE=.*|LITELLM_PROFILE=${PROFILE}|" "${ENV_FILE}"
else
    printf 'LITELLM_PROFILE=%s\n' "${PROFILE}" >> "${ENV_FILE}"
fi
echo "[OK] .env updated: LITELLM_PROXY_IMAGE=${PUSH_IMAGE} LITELLM_PROFILE=${PROFILE}"
echo "[DONE] LiteLLM build+deploy completed (profile: ${PROFILE})"
