#!/bin/bash
# Build install-app (sock-shop) image and push to Docker Hub.
# Usage: bash build-install-app.sh [--env-file <path>] [--tag <tag>]
set -e

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
IMAGE="agentcert/agentcert-install-app:${IMAGE_TAG}"

echo "[INFO] Building ${IMAGE}"
cd /mnt/d/Studies/app-charts
DOCKER_BUILDKIT=1 docker build -t "${IMAGE}" -f install-app/Dockerfile . || \
    DOCKER_BUILDKIT=0 docker build -t "${IMAGE}" -f install-app/Dockerfile .

docker tag "${IMAGE}" agentcert/agentcert-install-app:latest
echo "[OK] Docker build completed"

echo "[INFO] Pushing to Docker Hub..."
docker push "${IMAGE}"
docker push agentcert/agentcert-install-app:latest
echo "[OK] Pushed: ${IMAGE} and :latest"

# Update .env
if grep -q "^INSTALL_APPLICATION_IMAGE=" "${ENV_FILE}"; then
    sed -i "s|^INSTALL_APPLICATION_IMAGE=.*|INSTALL_APPLICATION_IMAGE=agentcert/agentcert-install-app:latest|" "${ENV_FILE}"
else
    printf '\nINSTALL_APPLICATION_IMAGE=agentcert/agentcert-install-app:latest\n' >> "${ENV_FILE}"
fi
echo "[OK] .env updated: INSTALL_APPLICATION_IMAGE=agentcert/agentcert-install-app:latest"
