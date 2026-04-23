#!/bin/bash
set -e

SERVER_NAMESPACE="litmus-chaos"
SERVER_DEPLOYMENT="litmusportal-server"
ENV_FILE="/mnt/d/Studies/AgentCert/local-custom/config/.env"

while [[ $# -gt 0 ]]; do
	case "$1" in
		--env-file)
			ENV_FILE="${2:-}"
			shift 2
			;;
		*)
			shift
			;;
	esac
done

if [[ ! -f "${ENV_FILE}" ]]; then
	echo "[ERROR] Env file not found: ${ENV_FILE}" >&2
	exit 1
fi

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
	kubectl set env deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" INSTALL_AGENT_IMAGE="${IMAGE}" >/dev/null
	kubectl rollout status deployment/"${SERVER_DEPLOYMENT}" -n "${SERVER_NAMESPACE}" --timeout=120s >/dev/null
	echo "[OK] Live server env synced: INSTALL_AGENT_IMAGE=${IMAGE}"
}

# Prune old agentcert-install-agent images before building new one
echo "[INFO] Pruning old agentcert-install-agent images..."
docker images | grep "agentcert-install-agent" | grep -v "latest\|dev" | awk '{print $3}' | xargs -r docker rmi -f 2>/dev/null || true
docker image prune -f 2>/dev/null || true
echo "[OK] Old images pruned"

IMAGE_TAG="ci-$(date +%Y%m%d%H%M%S)"
IMAGE="agentcert/agentcert-install-agent:${IMAGE_TAG}"

echo "[INFO] Building ${IMAGE}"
cd /mnt/d/Studies/agent-charts

run_docker_build() {
	docker build -t "${IMAGE}" -f install-agent/Dockerfile .
}

# If BuildKit/buildx is unavailable, retry automatically with legacy builder.
if ! run_docker_build; then
	echo "[WARN] Docker build failed. Retrying with DOCKER_BUILDKIT=0..."
	DOCKER_BUILDKIT=0 run_docker_build
fi

docker tag "${IMAGE}" agentcert/agentcert-install-agent:latest
docker tag "${IMAGE}" agentcert/agentcert-install-agent:dev
echo "[OK] Docker build completed"

echo "[INFO] Cleaning up old images from minikube..."
# Remove old ci-* tags from minikube (keep only latest, dev, and the new one)
minikube image ls | grep "agentcert-install-agent:ci-" | grep -v "${IMAGE_TAG}" | awk '{print $1}' | xargs -r minikube image rm 2>/dev/null || true
echo "[OK] Old minikube images cleaned"

echo "[INFO] Loading into minikube..."
minikube image load "${IMAGE}"
minikube image load agentcert/agentcert-install-agent:latest
minikube image load agentcert/agentcert-install-agent:dev
echo "[OK] Images loaded into minikube"

# Update .env with :latest tag instead of timestamped version
# This ensures consistent deployment across restarts and scales
LATEST_IMAGE="agentcert/agentcert-install-agent:latest"
sed -i "s|^INSTALL_AGENT_IMAGE=.*|INSTALL_AGENT_IMAGE=${LATEST_IMAGE}|" "${ENV_FILE}"
echo "[OK] .env updated: INSTALL_AGENT_IMAGE=${LATEST_IMAGE}"

# Update global variable for sync function
IMAGE="${LATEST_IMAGE}"
sync_live_server_env
