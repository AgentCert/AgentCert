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
  echo "[ERROR] --source-dir is required (path to AgentCert/agent-sidecar)" >&2
  exit 1
fi
if [[ ! -d "${SOURCE_DIR}" ]]; then
  echo "[ERROR] --source-dir not found: ${SOURCE_DIR}" >&2
  exit 1
fi

push_to_dockerhub() {
  local dh_user dh_token
  dh_user=$(grep -E "^DOCKERHUB_USERNAME=" "${ENV_FILE}" | tail -1 | cut -d'=' -f2- | tr -d '\r\n"'"'"')
  dh_token=$(grep -E "^DOCKERHUB_TOKEN=" "${ENV_FILE}" | tail -1 | cut -d'=' -f2- | tr -d '\r\n"'"'"')
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

echo "[INFO] Pruning old agent-sidecar images..."
docker images | grep "agentcert/agent-sidecar" | grep -v "latest\|dev" | awk '{print $3}' | xargs -r docker rmi -f 2>/dev/null || true
docker image prune -f 2>/dev/null || true
echo "[OK] Old images pruned"

IMAGE_REPO="agentcert/agent-sidecar"
IMAGE_TAG="ci-$(date +%Y%m%d%H%M%S)"
IMAGE="${IMAGE_REPO}:${IMAGE_TAG}"

echo "[INFO] Building ${IMAGE} from ${SOURCE_DIR}"
cd "${SOURCE_DIR}"

run_docker_build() {
  docker build -t "${IMAGE}" -f Dockerfile .
}

if ! run_docker_build; then
  echo "[WARN] Docker build failed. Retrying with DOCKER_BUILDKIT=0..."
  DOCKER_BUILDKIT=0 run_docker_build
fi

docker tag "${IMAGE}" agentcert/agent-sidecar:latest
docker tag "${IMAGE}" agentcert/agent-sidecar:dev
echo "[OK] Docker build completed"

push_to_dockerhub

LATEST_IMAGE="agentcert/agent-sidecar:latest"
if grep -q "^AGENT_SIDECAR_IMAGE=" "${ENV_FILE}" 2>/dev/null; then
  sed -i "s|^AGENT_SIDECAR_IMAGE=.*|AGENT_SIDECAR_IMAGE=${LATEST_IMAGE}|" "${ENV_FILE}"
else
  if grep -q "^FLASH_AGENT_IMAGE=" "${ENV_FILE}"; then
    sed -i "/^FLASH_AGENT_IMAGE=/a AGENT_SIDECAR_IMAGE=${LATEST_IMAGE}" "${ENV_FILE}"
  elif grep -q "^INSTALL_AGENT_IMAGE=" "${ENV_FILE}"; then
    sed -i "/^INSTALL_AGENT_IMAGE=/a AGENT_SIDECAR_IMAGE=${LATEST_IMAGE}" "${ENV_FILE}"
  else
    printf '\nAGENT_SIDECAR_IMAGE=%s\n' "${LATEST_IMAGE}" >> "${ENV_FILE}"
  fi
fi
echo "[OK] .env updated: AGENT_SIDECAR_IMAGE=${LATEST_IMAGE}"
