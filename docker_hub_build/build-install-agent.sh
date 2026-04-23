#!/bin/bash
# Build install-agent image and push to Docker Hub.
# Usage: bash build-install-agent.sh [--env-file <path>] [--tag <tag>]
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
IMAGE="agentcert/agentcert-install-agent:${IMAGE_TAG}"

echo "[INFO] Building ${IMAGE}"
cd /mnt/d/Studies/agent-charts
DOCKER_BUILDKIT=1 docker build -t "${IMAGE}" -f install-agent/Dockerfile . || \
    DOCKER_BUILDKIT=0 docker build -t "${IMAGE}" -f install-agent/Dockerfile .

docker tag "${IMAGE}" agentcert/agentcert-install-agent:latest
echo "[OK] Docker build completed"

echo "[INFO] Pushing to Docker Hub..."
docker push "${IMAGE}"
docker push agentcert/agentcert-install-agent:latest
echo "[OK] Pushed: ${IMAGE} and :latest"

# Sync running server so it picks up the new image on next experiment run
if command -v kubectl >/dev/null 2>&1 && \
   kubectl get deployment "${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" >/dev/null 2>&1; then
    echo "[INFO] Syncing live server env: INSTALL_AGENT_IMAGE=${IMAGE}"
    kubectl set env deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" \
        INSTALL_AGENT_IMAGE="${IMAGE}" >/dev/null
    kubectl rollout status deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" --timeout=120s >/dev/null
    echo "[OK] Live server env synced"
else
    echo "[WARN] ${SERVER_NAMESPACE}/${SERVER_DEPLOYMENT} not found; skipping live server env sync"
fi

# Update .env so other scripts reference the pushed tag
if grep -q "^INSTALL_AGENT_IMAGE=" "${ENV_FILE}"; then
    sed -i "s|^INSTALL_AGENT_IMAGE=.*|INSTALL_AGENT_IMAGE=agentcert/agentcert-install-agent:latest|" "${ENV_FILE}"
else
    printf '\nINSTALL_AGENT_IMAGE=agentcert/agentcert-install-agent:latest\n' >> "${ENV_FILE}"
fi
echo "[OK] .env updated: INSTALL_AGENT_IMAGE=agentcert/agentcert-install-agent:latest"
