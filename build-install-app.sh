#!/bin/bash
set -e

SERVER_NAMESPACE="litmus-chaos"
SERVER_DEPLOYMENT="litmusportal-server"

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
	kubectl set env deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" INSTALL_APPLICATION_IMAGE="${IMAGE}" >/dev/null
	kubectl rollout status deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" --timeout=120s >/dev/null
	echo "[OK] Live server env synced: INSTALL_APPLICATION_IMAGE=${IMAGE}"
}

# Prune old agentcert-install-app images before building new one
echo "[INFO] Pruning old agentcert-install-app images..."
docker images | grep "agentcert-install-app" | grep -v "latest\|dev" | awk '{print $3}' | xargs -r docker rmi -f 2>/dev/null || true
docker image prune -f 2>/dev/null || true
echo "[OK] Old images pruned"

IMAGE_TAG="ci-$(date +%Y%m%d%H%M%S)"
IMAGE="agentcert/agentcert-install-app:${IMAGE_TAG}"

echo "[INFO] Building ${IMAGE}"
cd /mnt/d/Studies/app-charts
docker build -t "${IMAGE}" -f install-app/Dockerfile .
docker tag "${IMAGE}" agentcert/agentcert-install-app:latest
docker tag "${IMAGE}" agentcert/agentcert-install-app:dev
echo "[OK] Docker build completed"

echo "[INFO] Cleaning up old images from minikube..."
# Remove old ci-* tags from minikube (keep only latest, dev, and the new one)
minikube image ls | grep "agentcert-install-app:ci-" | grep -v "${IMAGE_TAG}" | awk '{print $1}' | xargs -r minikube image rm 2>/dev/null || true
echo "[OK] Old minikube images cleaned"

echo "[INFO] Loading into minikube..."
minikube image load "${IMAGE}"
minikube image load agentcert/agentcert-install-app:latest
minikube image load agentcert/agentcert-install-app:dev
echo "[OK] Images loaded into minikube"

# Update .env
ENV_FILE="/mnt/d/Studies/AgentCert/local-custom/config/.env"
sed -i "s|^INSTALL_APPLICATION_IMAGE=.*|INSTALL_APPLICATION_IMAGE=${IMAGE}|" "${ENV_FILE}"
echo "[OK] .env updated: INSTALL_APPLICATION_IMAGE=${IMAGE}"

sync_live_server_env
