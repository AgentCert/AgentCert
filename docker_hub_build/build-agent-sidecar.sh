#!/bin/bash
# Build agent-sidecar image and push to Docker Hub.
# Usage: bash build-agent-sidecar.sh [--env-file <path>] [--tag <tag>]
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

IMAGE_TAG="${PUSH_TAG:-ci-$(date +%Y%m%d%H%M%S)}"
IMAGE="agentcert/agent-sidecar:${IMAGE_TAG}"

echo "[INFO] Building ${IMAGE}"
cd /mnt/d/Studies/AgentCert/agent-sidecar
DOCKER_BUILDKIT=1 docker build -t "${IMAGE}" -f Dockerfile . || \
    DOCKER_BUILDKIT=0 docker build -t "${IMAGE}" -f Dockerfile .

docker tag "${IMAGE}" agentcert/agent-sidecar:latest
echo "[OK] Docker build completed"

echo "[INFO] Pushing to Docker Hub..."
docker push "${IMAGE}"
docker push agentcert/agent-sidecar:latest
echo "[OK] Pushed: ${IMAGE} and :latest"

# Sync running server so it picks up the new sidecar image on next experiment run
if command -v kubectl >/dev/null 2>&1 && \
   kubectl get deployment "${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" >/dev/null 2>&1; then
    echo "[INFO] Syncing live server env: AGENT_SIDECAR_IMAGE=${IMAGE}"
    kubectl set env deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" \
        AGENT_SIDECAR_IMAGE="${IMAGE}" >/dev/null
    kubectl rollout status deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" --timeout=120s >/dev/null
    echo "[OK] Live server env synced"
else
    echo "[WARN] ${SERVER_NAMESPACE}/${SERVER_DEPLOYMENT} not found; skipping live server env sync"
fi

# Update .env
if grep -q "^AGENT_SIDECAR_IMAGE=" "${ENV_FILE}"; then
    sed -i "s|^AGENT_SIDECAR_IMAGE=.*|AGENT_SIDECAR_IMAGE=agentcert/agent-sidecar:latest|" "${ENV_FILE}"
else
    printf '\nAGENT_SIDECAR_IMAGE=agentcert/agent-sidecar:latest\n' >> "${ENV_FILE}"
fi
echo "[OK] .env updated: AGENT_SIDECAR_IMAGE=agentcert/agent-sidecar:latest"
