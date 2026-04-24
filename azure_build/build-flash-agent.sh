#!/bin/bash
set -e

SERVER_NAMESPACE="litmus-chaos"
SERVER_DEPLOYMENT="litmusportal-server"
ENV_FILE=""
SOURCE_DIR=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file)
      ENV_FILE="${2:-}"
      shift 2
      ;;
    --source-dir)
      SOURCE_DIR="${2:-}"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

if [[ -z "${ENV_FILE}" ]]; then
  echo "[ERROR] --env-file is required" >&2
  exit 1
fi
if [[ ! -f "${ENV_FILE}" ]]; then
  echo "[ERROR] Env file not found: ${ENV_FILE}" >&2
  exit 1
fi
if [[ -z "${SOURCE_DIR}" ]]; then
  echo "[ERROR] --source-dir is required (path to flash-agent repo)" >&2
  exit 1
fi
if [[ ! -d "${SOURCE_DIR}" ]]; then
  echo "[ERROR] --source-dir not found: ${SOURCE_DIR}" >&2
  exit 1
fi

read_env_value() {
  local key="$1"
  local value
  value=$(grep -E "^${key}=" "${ENV_FILE}" | tail -1 | cut -d'=' -f2- || true)
  value=$(echo "${value}" | tr -d '\r\n')
  value=${value#"\""}
  value=${value%"\""}
  value=${value#"'"}
  value=${value%"'"}
  echo "${value}"
}

sync_live_server_env() {
  if ! command -v kubectl >/dev/null 2>&1; then
    echo "[WARN] kubectl not found; skipping live server env sync"
    return 0
  fi
  if ! kubectl get deployment "${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" >/dev/null 2>&1; then
    echo "[WARN] ${SERVER_NAMESPACE}/${SERVER_DEPLOYMENT} not found; skipping live server env sync"
    return 0
  fi

  echo "[INFO] Syncing live server env..."
  local litellm_master_key openai_base_url openai_api_key model_alias
  local k8s_mcp_url prom_mcp_url chaos_namespace pre_cleanup_wait_seconds

  litellm_master_key=$(read_env_value "LITELLM_MASTER_KEY")
  openai_base_url=$(read_env_value "OPENAI_BASE_URL")
  openai_api_key=$(read_env_value "OPENAI_API_KEY")
  model_alias=$(read_env_value "AZURE_OPENAI_DEPLOYMENT")
  k8s_mcp_url=$(read_env_value "K8S_MCP_URL")
  prom_mcp_url=$(read_env_value "PROM_MCP_URL")
  chaos_namespace=$(read_env_value "CHAOS_NAMESPACE")
  pre_cleanup_wait_seconds=$(read_env_value "PRE_CLEANUP_WAIT_SECONDS")

  [[ -z "${litellm_master_key}" ]]        && litellm_master_key="sk-litellm-local-dev"
  [[ -z "${model_alias}" ]]               && model_alias="gpt-4"
  [[ -z "${openai_base_url}" ]]           && openai_base_url="http://litellm-proxy.litellm.svc.cluster.local:4000/v1"
  [[ -z "${openai_api_key}" ]]            && openai_api_key="${litellm_master_key}"
  [[ -z "${k8s_mcp_url}" ]]              && k8s_mcp_url="http://kubernetes-mcp-server.litmus-exp.svc.cluster.local:8081/mcp"
  [[ -z "${prom_mcp_url}" ]]             && prom_mcp_url="http://prometheus-mcp-server.litmus-exp.svc.cluster.local:9090/mcp"
  [[ -z "${chaos_namespace}" ]]           && chaos_namespace="litmus-exp"
  [[ -z "${pre_cleanup_wait_seconds}" ]] && pre_cleanup_wait_seconds="0"

  kubectl set env deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" \
    FLASH_AGENT_IMAGE="${IMAGE}" \
    LITELLM_MASTER_KEY="${litellm_master_key}" \
    OPENAI_API_KEY="${openai_api_key}" \
    OPENAI_BASE_URL="${openai_base_url}" \
    MODEL_ALIAS="${model_alias}" \
    K8S_MCP_URL="${k8s_mcp_url}" \
    PROM_MCP_URL="${prom_mcp_url}" \
    CHAOS_NAMESPACE="${chaos_namespace}" \
    PRE_CLEANUP_WAIT_SECONDS="${pre_cleanup_wait_seconds}" >/dev/null
  kubectl rollout status deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" --timeout=120s >/dev/null
  echo "[OK] Live server env synced: FLASH_AGENT_IMAGE=${IMAGE} LITELLM_MASTER_KEY=<set>"
}

sync_live_flash_agent_workloads() {
  local namespace="sock-shop"
  local deployment="flash-agent"
  local cronjob="flash-agent-cronjob"

  if ! command -v kubectl >/dev/null 2>&1; then
    echo "[WARN] kubectl not found; skipping flash-agent workload image sync"
    return 0
  fi

  if kubectl -n "${namespace}" get deployment "${deployment}" >/dev/null 2>&1; then
    echo "[INFO] Updating ${namespace}/${deployment} image to ${IMAGE}"
    kubectl -n "${namespace}" set image deployment/"${deployment}" agent="${IMAGE}" >/dev/null || true
    kubectl -n "${namespace}" rollout status deployment/"${deployment}" --timeout=120s >/dev/null || true
  else
    echo "[WARN] ${namespace}/${deployment} not found; skipping deployment image sync"
  fi

  if kubectl -n "${namespace}" get cronjob "${cronjob}" >/dev/null 2>&1; then
    echo "[INFO] Updating ${namespace}/${cronjob} image to ${IMAGE}"
    kubectl -n "${namespace}" set image cronjob/"${cronjob}" agent="${IMAGE}" >/dev/null || true
  else
    echo "[WARN] ${namespace}/${cronjob} not found; skipping cronjob image sync"
  fi

  echo "[OK] Flash-agent workloads synced to ${IMAGE}"
}

push_to_dockerhub() {
  local dh_user dh_token
  dh_user=$(read_env_value "DOCKERHUB_USERNAME")
  dh_token=$(read_env_value "DOCKERHUB_TOKEN")
  if [[ -z "${dh_user}" || -z "${dh_token}" ]]; then
    echo "[WARN] DOCKERHUB_USERNAME or DOCKERHUB_TOKEN not set in .env; skipping Docker Hub push"
    return 0
  fi
  echo "[INFO] Pushing to Docker Hub as ${dh_user}..."
  echo "${dh_token}" | docker login -u "${dh_user}" --password-stdin
  docker push "${IMAGE}"
  docker push "${IMAGE_REPO}:latest"
  docker logout >/dev/null 2>&1 || true
  echo "[OK] Pushed to Docker Hub: ${IMAGE} and ${IMAGE_REPO}:latest"
}

echo "[INFO] Pruning old agentcert-flash-agent images..."
docker images | grep "agentcert-flash-agent" | grep -v "latest\|dev" | awk '{print $3}' | xargs -r docker rmi -f 2>/dev/null || true
docker image prune -f 2>/dev/null || true
echo "[OK] Old images pruned"

IMAGE_REPO="agentcert/agentcert-flash-agent"
IMAGE_TAG="ci-$(date +%Y%m%d%H%M%S)"
IMAGE="${IMAGE_REPO}:${IMAGE_TAG}"

echo "[INFO] Building ${IMAGE} from ${SOURCE_DIR}"
cd "${SOURCE_DIR}"
docker build -t "${IMAGE}" -f Dockerfile .
docker tag "${IMAGE}" agentcert/agentcert-flash-agent:latest
docker tag "${IMAGE}" agentcert/agentcert-flash-agent:dev
echo "[OK] Docker build completed"

push_to_dockerhub

LATEST_IMAGE="agentcert/agentcert-flash-agent:latest"
if grep -q "^FLASH_AGENT_IMAGE=" "${ENV_FILE}"; then
  sed -i "s|^FLASH_AGENT_IMAGE=.*|FLASH_AGENT_IMAGE=${LATEST_IMAGE}|" "${ENV_FILE}"
else
  sed -i "/^INSTALL_AGENT_IMAGE=/a FLASH_AGENT_IMAGE=${LATEST_IMAGE}" "${ENV_FILE}"
fi
echo "[OK] .env updated: FLASH_AGENT_IMAGE=${LATEST_IMAGE}"
