content = r'''#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/local-custom/config/.env"
LITELLM_DIR_DEFAULT_A="${SCRIPT_DIR}/../agent-charts/litellm"
LITELLM_DIR_DEFAULT_B="${SCRIPT_DIR}/../../agent-charts/litellm"
LITELLM_DIR=""
NAMESPACE="litellm"
DEPLOYMENT="litellm-proxy"
SERVER_NAMESPACE="litmus-chaos"
SERVER_DEPLOYMENT="litmusportal-server"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file)   ENV_FILE="${2:-}"; shift 2 ;;
    --litellm-dir) LITELLM_DIR="${2:-}"; shift 2 ;;
    *) echo "[ERROR] Unknown option: $1" >&2; exit 1 ;;
  esac
done

if [[ -z "${LITELLM_DIR}" ]]; then
  if   [[ -d "${LITELLM_DIR_DEFAULT_A}" ]]; then LITELLM_DIR="${LITELLM_DIR_DEFAULT_A}"
  elif [[ -d "${LITELLM_DIR_DEFAULT_B}" ]]; then LITELLM_DIR="${LITELLM_DIR_DEFAULT_B}"
  else LITELLM_DIR="${LITELLM_DIR_DEFAULT_A}"
  fi
fi

LITELLM_VERSION="v1.82.0"

echo ""
echo "Select LiteLLM provider profile:"
echo "  1) azure   - Azure OpenAI only"
echo "  2) openai  - OpenAI only"
echo "  3) all     - All providers"
echo ""

if [ -n "${LITELLM_PROFILE:-}" ]; then
  PROFILE="${LITELLM_PROFILE}"
  echo "[INFO] Using LITELLM_PROFILE=${PROFILE} from environment"
else
  read -r -p "Enter choice [1/2/3] (default: 1): " PROFILE_CHOICE
  case "${PROFILE_CHOICE:-1}" in
    1|azure)  PROFILE="azure" ;;
    2|openai) PROFILE="openai" ;;
    3|all)    PROFILE="all" ;;
    *) echo "[ERROR] Invalid choice." >&2; exit 1 ;;
  esac
fi

case "${PROFILE}" in azure|openai|all) ;;
  *) echo "[ERROR] Unknown profile: ${PROFILE}" >&2; exit 1 ;;
esac

# Always pull the image from Docker Hub / public registry — no local minikube cache
IMAGE="docker.io/litellm/litellm:${LITELLM_VERSION}-stable"
echo "[INFO] Profile: ${PROFILE} => image: ${IMAGE}"

if [ ! -f "${ENV_FILE}" ]; then
  echo "[ERROR] .env not found at ${ENV_FILE}" >&2; exit 1
fi
if [ ! -d "${LITELLM_DIR}" ]; then
  echo "[ERROR] LiteLLM manifest dir not found at ${LITELLM_DIR}" >&2; exit 1
fi

read_env_value() {
  local key="$1" value
  value=$(grep -E "^${key}=" "${ENV_FILE}" | tail -1 | cut -d'=' -f2- || true)
  value=$(echo "${value}" | tr -d '\r\n')
  value=${value#'"'}; value=${value%'"'}
  value=${value#"'"}; value=${value%"'"}
  echo "${value}"
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { echo "[ERROR] Required command not found: $1" >&2; exit 1; }
}

require_cmd docker
require_cmd kubectl

AZURE_API_KEY=$(read_env_value "AZURE_OPENAI_KEY")
[ -z "${AZURE_API_KEY}" ] && AZURE_API_KEY=$(read_env_value "AZURE_OPENAI_API_KEY")
AZURE_API_BASE=$(read_env_value "AZURE_OPENAI_ENDPOINT")
AZURE_OPENAI_DEPLOYMENT=$(read_env_value "AZURE_OPENAI_DEPLOYMENT")
AZURE_API_VERSION=$(read_env_value "AZURE_OPENAI_API_VERSION")
AZURE_MODEL="azure/${AZURE_OPENAI_DEPLOYMENT}"
OPENAI_API_KEY=$(read_env_value "OPENAI_API_KEY")
LITELLM_MASTER_KEY=$(read_env_value "LITELLM_MASTER_KEY")
LANGFUSE_PUBLIC_KEY=$(read_env_value "LANGFUSE_PUBLIC_KEY")
LANGFUSE_SECRET_KEY=$(read_env_value "LANGFUSE_SECRET_KEY")
LANGFUSE_HOST=$(read_env_value "LANGFUSE_HOST")
PRE_CLEANUP_WAIT_SECONDS=$(read_env_value "PRE_CLEANUP_WAIT_SECONDS")

case "${PROFILE}" in
  azure|all)
    if [ -z "${AZURE_API_KEY}" ] || [ -z "${AZURE_API_BASE}" ]; then
      echo "[ERROR] Profile '${PROFILE}' requires AZURE_OPENAI_KEY and AZURE_OPENAI_ENDPOINT in .env." >&2
      exit 1
    fi ;;
  openai)
    if [ -z "${OPENAI_API_KEY}" ]; then
      echo "[ERROR] Profile 'openai' requires OPENAI_API_KEY in .env." >&2; exit 1
    fi ;;
esac

[ -z "${LITELLM_MASTER_KEY}" ] && LITELLM_MASTER_KEY="sk-litellm-local-dev" && echo "[WARN] LITELLM_MASTER_KEY not set; using default"
[ -z "${PRE_CLEANUP_WAIT_SECONDS}" ] && PRE_CLEANUP_WAIT_SECONDS="0"
{ [ -z "${LANGFUSE_PUBLIC_KEY}" ] || [ -z "${LANGFUSE_SECRET_KEY}" ] || [ -z "${LANGFUSE_HOST}" ]; } && echo "[WARN] Langfuse keys/host missing; traces may not be exported"

echo "[INFO] Applying namespace and configmap"
kubectl apply -f "${LITELLM_DIR}/namespace.yaml"
sed "s/model_name: LITELLM_MODEL_NAME/model_name: ${AZURE_OPENAI_DEPLOYMENT}/g" \
  "${LITELLM_DIR}/configmap.yaml" | kubectl apply -f -

echo "[INFO] Applying litellm secret from .env values"
kubectl -n "${NAMESPACE}" create secret generic litellm-secrets \
  --from-literal=AZURE_API_KEY="${AZURE_API_KEY}" \
  --from-literal=AZURE_API_BASE="${AZURE_API_BASE}" \
  --from-literal=AZURE_API_VERSION="${AZURE_API_VERSION}" \
  --from-literal=AZURE_MODEL="${AZURE_MODEL}" \
  --from-literal=OPENAI_API_KEY="${OPENAI_API_KEY}" \
  --from-literal=LITELLM_MASTER_KEY="${LITELLM_MASTER_KEY}" \
  --from-literal=LANGFUSE_PUBLIC_KEY="${LANGFUSE_PUBLIC_KEY}" \
  --from-literal=LANGFUSE_SECRET_KEY="${LANGFUSE_SECRET_KEY}" \
  --from-literal=LANGFUSE_HOST="${LANGFUSE_HOST}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "[INFO] Applying deployment manifest"
kubectl apply -f "${LITELLM_DIR}/deployment.yaml"
kubectl -n "${NAMESPACE}" set image deployment/"${DEPLOYMENT}" litellm="${IMAGE}" >/dev/null
kubectl -n "${NAMESPACE}" rollout restart deployment/"${DEPLOYMENT}" >/dev/null
kubectl -n "${NAMESPACE}" rollout status deployment/"${DEPLOYMENT}" --timeout=180s

if kubectl get deployment "${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" >/dev/null 2>&1; then
  echo "[INFO] Syncing litmusportal-server LiteLLM env"
  OPENAI_BASE_URL=$(read_env_value "OPENAI_BASE_URL")
  [ -z "${OPENAI_BASE_URL}" ] && OPENAI_BASE_URL="http://litellm-proxy.litellm.svc.cluster.local:4000/v1"
  kubectl set env deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" \
    LITELLM_MASTER_KEY="${LITELLM_MASTER_KEY}" \
    OPENAI_API_KEY="${LITELLM_MASTER_KEY}" \
    OPENAI_BASE_URL="${OPENAI_BASE_URL}" \
    MODEL_ALIAS="${AZURE_OPENAI_DEPLOYMENT}" \
    PRE_CLEANUP_WAIT_SECONDS="${PRE_CLEANUP_WAIT_SECONDS}" >/dev/null
  kubectl rollout status deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" --timeout=180s >/dev/null
  echo "[OK] Synced litmusportal-server LiteLLM env"
else
  echo "[WARN] ${SERVER_NAMESPACE}/${SERVER_DEPLOYMENT} not found; skipping server env sync"
fi

if grep -q "^LITELLM_PROXY_IMAGE=" "${ENV_FILE}"; then
  sed -i "s|^LITELLM_PROXY_IMAGE=.*|LITELLM_PROXY_IMAGE=${IMAGE}|" "${ENV_FILE}"
else
  printf "\nLITELLM_PROXY_IMAGE=%s\n" "${IMAGE}" >> "${ENV_FILE}"
fi
if grep -q "^LITELLM_PROFILE=" "${ENV_FILE}"; then
  sed -i "s|^LITELLM_PROFILE=.*|LITELLM_PROFILE=${PROFILE}|" "${ENV_FILE}"
else
  printf "LITELLM_PROFILE=%s\n" "${PROFILE}" >> "${ENV_FILE}"
fi
if grep -q "^LITELLM_MASTER_KEY=" "${ENV_FILE}"; then
  sed -i "s|^LITELLM_MASTER_KEY=.*|LITELLM_MASTER_KEY=${LITELLM_MASTER_KEY}|" "${ENV_FILE}"
else
  printf "LITELLM_MASTER_KEY=%s\n" "${LITELLM_MASTER_KEY}" >> "${ENV_FILE}"
fi

echo "[OK] .env updated: LITELLM_PROXY_IMAGE=${IMAGE} LITELLM_PROFILE=${PROFILE}"
echo "[DONE] LiteLLM deploy completed (profile: ${PROFILE})"
'''

with open('/mnt/d/Studies/AgentCert/azure_build/build-litellm.sh', 'w', newline='\n') as f:
    f.write(content)
print('Written: build-litellm.sh')
