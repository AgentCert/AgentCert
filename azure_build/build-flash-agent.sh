#!/bin/bash
set -e

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
