#!/bin/bash
set -euo pipefail

# Wrapper script to rebuild and deploy all AgentCert components in sequence:
# Works from Ubuntu (direct bash execution)
# 1. App chart (Sock Shop) with local-mode
# 2. Cluster deployment sync
# 3. LiteLLM proxy (ensures secrets/keys in sync)
# 4. Install-agent image
# 5. Agent-sidecar image
# 6. Flash-agent image

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/local-custom/config/.env"
APP_CHART_DIR=""
AGENTCERT_ROOT="${SCRIPT_DIR}"
TARGET_CONTEXT=""
ALLOW_NAMESPACE_CREATE="false"

# ---------------------------------------------------------------------------
# Argument parsing
# --llm 1|azure  / 2|openai / 3|all  -> sets LITELLM_PROFILE, skipping prompt
# ---------------------------------------------------------------------------
LLM_ARG=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --llm)
            if [[ -z "${2:-}" ]]; then
                echo "[ERROR] Missing value for --llm" >&2
                exit 1
            fi
      LLM_ARG="${2:-}"
      shift 2
      ;;
        --env-file)
            if [[ -z "${2:-}" ]]; then
                echo "[ERROR] Missing value for --env-file" >&2
                exit 1
            fi
            ENV_FILE="${2:-}"
            shift 2
            ;;
        --app-chart-dir)
            if [[ -z "${2:-}" ]]; then
                echo "[ERROR] Missing value for --app-chart-dir" >&2
                exit 1
            fi
            APP_CHART_DIR="${2:-}"
            shift 2
            ;;
        --agentcert-root)
            if [[ -z "${2:-}" ]]; then
                echo "[ERROR] Missing value for --agentcert-root" >&2
                exit 1
            fi
            AGENTCERT_ROOT="${2:-}"
            shift 2
            ;;
        --context)
            if [[ -z "${2:-}" ]]; then
                echo "[ERROR] Missing value for --context" >&2
                exit 1
            fi
            TARGET_CONTEXT="${2:-}"
            shift 2
            ;;
        --allow-namespace-create)
            ALLOW_NAMESPACE_CREATE="true"
            shift
            ;;
    *)
            echo "[ERROR] Unknown argument: $1" >&2
            exit 1
      ;;
  esac
done

if [[ ! -f "${ENV_FILE}" ]]; then
    echo "[ERROR] Env file not found: ${ENV_FILE}" >&2
    exit 1
fi

if [[ -n "${LLM_ARG}" ]]; then
  case "${LLM_ARG}" in
    1|azure)  export LITELLM_PROFILE="azure" ;;
    2|openai) export LITELLM_PROFILE="openai" ;;
    3|all)    export LITELLM_PROFILE="all" ;;
    *)
      echo "[ERROR] Unknown --llm value '${LLM_ARG}'. Use 1/azure, 2/openai, or 3/all." >&2
      exit 1
      ;;
  esac
fi

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

function log_info() {
    echo -e "${CYAN}[INFO]${NC}  $*"
}

function log_success() {
    echo -e "${GREEN}[OK]${NC}    $*"
}

function log_warn() {
    echo -e "${YELLOW}[WARN]${NC}   $*"
}

function log_error() {
    echo -e "${RED}[ERROR]${NC}  $*"
}

get_env_value() {
    local key="$1"
    local line=""
    line="$(grep -E "^[[:space:]]*${key}=" "${ENV_FILE}" | tail -n1 || true)"
    line="${line#*=}"
    line="${line%$'\r'}"
    line="${line%\"}"
    line="${line#\"}"
    line="${line%\'}"
    line="${line#\'}"
    printf "%s" "${line}"
}

require_command() {
    local cmd="$1"
    if ! command -v "$cmd" >/dev/null 2>&1; then
        log_error "Required command not found: $cmd"
        exit 1
    fi
}

preflight_checks() {
    local current_context=""
    local default_app_hub_path=""

    log_info "Running preflight checks"
    require_command bash
    require_command docker
    require_command kubectl

    if ! kubectl cluster-info >/dev/null 2>&1; then
        log_error "kubectl cannot reach a cluster. Check kubeconfig and network."
        exit 1
    fi

    current_context="$(kubectl config current-context 2>/dev/null || true)"
    if [[ -z "${current_context}" ]]; then
        log_error "Unable to determine current kubectl context."
        exit 1
    fi
    log_info "Current kubectl context: ${current_context}"

    if [[ -n "${TARGET_CONTEXT}" && "${TARGET_CONTEXT}" != "${current_context}" ]]; then
        log_error "Context mismatch. Expected '${TARGET_CONTEXT}', got '${current_context}'."
        exit 1
    fi

    if [[ -z "${APP_CHART_DIR}" ]]; then
        default_app_hub_path="$(get_env_value DEFAULT_APP_HUB_PATH)"
        if [[ -n "${default_app_hub_path}" ]]; then
            APP_CHART_DIR="${default_app_hub_path%/}/install-app"
        else
            APP_CHART_DIR="${SCRIPT_DIR}/../app-charts/install-app"
        fi
    fi

    if [[ ! -d "${APP_CHART_DIR}" ]]; then
        log_error "App chart directory not found: ${APP_CHART_DIR}"
        exit 1
    fi
    if [[ ! -f "${APP_CHART_DIR}/build-and-deploy-app-chart.sh" ]]; then
        log_error "Missing script: ${APP_CHART_DIR}/build-and-deploy-app-chart.sh"
        exit 1
    fi

    if [[ ! -d "${SCRIPT_DIR}/local-custom/scripts" ]]; then
        log_error "Missing deploy scripts directory: ${SCRIPT_DIR}/local-custom/scripts"
        exit 1
    fi
    if [[ ! -f "${SCRIPT_DIR}/build-litellm.sh" ]]; then
        log_error "Missing script: ${SCRIPT_DIR}/build-litellm.sh"
        exit 1
    fi
    if [[ ! -f "${AGENTCERT_ROOT}/build-install-agent.sh" ]]; then
        log_error "Missing script: ${AGENTCERT_ROOT}/build-install-agent.sh"
        exit 1
    fi
    if [[ ! -f "${AGENTCERT_ROOT}/build-agent-sidecar.sh" ]]; then
        log_error "Missing script: ${AGENTCERT_ROOT}/build-agent-sidecar.sh"
        exit 1
    fi
    if [[ ! -f "${AGENTCERT_ROOT}/build-flash-agent.sh" ]]; then
        log_error "Missing script: ${AGENTCERT_ROOT}/build-flash-agent.sh"
        exit 1
    fi

    log_info "Resolved app chart directory: ${APP_CHART_DIR}"
    log_info "Resolved AgentCert root: ${AGENTCERT_ROOT}"
}

choose_cluster_deploy_script() {
    local namespace="litmus-chaos"
    local infra_namespace="litmus-exp"
    local auth_pod=""
    local graphql_pod=""

    if ! command -v kubectl >/dev/null 2>&1; then
        log_warn "kubectl not found, defaulting to scratch deploy script" >&2
        echo "build-and-deploy-scratch.sh"
        return 0
    fi

    if ! kubectl get namespace "$namespace" >/dev/null 2>&1; then
        if [[ "${ALLOW_NAMESPACE_CREATE}" == "true" ]]; then
            log_info "Namespace $namespace not found; creating it" >&2
            kubectl create namespace "$namespace" >/dev/null 2>&1 || true
        else
            log_warn "Namespace $namespace not found; run with --allow-namespace-create to create it" >&2
        fi
    fi
    if ! kubectl get namespace "$infra_namespace" >/dev/null 2>&1; then
        if [[ "${ALLOW_NAMESPACE_CREATE}" == "true" ]]; then
            log_info "Namespace $infra_namespace not found; creating it" >&2
            kubectl create namespace "$infra_namespace" >/dev/null 2>&1 || true
        else
            log_warn "Namespace $infra_namespace not found; run with --allow-namespace-create to create it" >&2
        fi
    fi

    if ! kubectl get namespace "$namespace" >/dev/null 2>&1; then
        log_warn "Namespace $namespace still unavailable; selecting scratch deploy script" >&2
        echo "build-and-deploy-scratch.sh"
        return 0
    fi

    auth_pod=$(kubectl get pods -n "$namespace" -l component=litmusportal-auth-server -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
    graphql_pod=$(kubectl get pods -n "$namespace" -l component=litmusportal-server -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

    if [[ -n "$auth_pod" && -n "$graphql_pod" ]]; then
        log_info "Auth and GraphQL pods detected; selecting incremental deploy script" >&2
        echo "build-and-deploy.sh"
    else
        log_info "Auth/GraphQL pod(s) missing; selecting scratch deploy script" >&2
        echo "build-and-deploy-scratch.sh"
    fi
}

echo ""
echo -e "${CYAN}================================${NC}"
echo -e "${CYAN}AgentCert Build+Deploy Pipeline${NC}"
echo -e "${CYAN}================================${NC}"
echo ""

preflight_checks

# Step 1: Build and deploy app chart (Sock Shop)
log_info "Starting: Build app chart (Sock Shop)"
if cd "${APP_CHART_DIR}" && bash build-and-deploy-app-chart.sh --local-mode; then
    log_success "Completed: Build app chart (Sock Shop)"
else
    log_error "Failed: Build app chart (Sock Shop)"
    exit 1
fi

echo ""

# Step 2: Sync cluster deployment
log_info "Starting: Sync cluster deployment"
DEPLOY_SCRIPT="$(choose_cluster_deploy_script)"
log_info "Using deploy script: ${DEPLOY_SCRIPT}"
if cd "${SCRIPT_DIR}/local-custom/scripts" && bash "$DEPLOY_SCRIPT" --env-file "${ENV_FILE}"; then
    log_success "Completed: Sync cluster deployment"
else
    log_error "Failed: Sync cluster deployment"
    exit 1
fi

echo ""

# Step 3: Build and deploy LiteLLM proxy (ensures secrets and keys are in sync)
log_info "Starting: Build LiteLLM proxy"
if bash "${SCRIPT_DIR}/build-litellm.sh" --env-file "${ENV_FILE}"; then
    log_success "Completed: Build LiteLLM proxy"
else
    log_error "Failed: Build LiteLLM proxy"
    exit 1
fi

echo ""

# Step 4: Build install-agent image
log_info "Starting: Build install-agent image"
if DOCKER_BUILDKIT=1 bash "${AGENTCERT_ROOT}/build-install-agent.sh" --env-file "${ENV_FILE}"; then
    log_success "Completed: Build install-agent image"
else
    log_error "Failed: Build install-agent image"
    exit 1
fi

echo ""

# Step 5: Build agent-sidecar image
log_info "Starting: Build agent-sidecar image"
if bash "${AGENTCERT_ROOT}/build-agent-sidecar.sh" --env-file "${ENV_FILE}"; then
    log_success "Completed: Build agent-sidecar image"
else
    log_error "Failed: Build agent-sidecar image"
    exit 1
fi

echo ""

# Step 6: Build flash-agent image (syncs LITELLM_MASTER_KEY + MCP URLs to server)
log_info "Starting: Build flash-agent image"
if bash "${AGENTCERT_ROOT}/build-flash-agent.sh" --env-file "${ENV_FILE}"; then
    log_success "Completed: Build flash-agent image"
else
    log_error "Failed: Build flash-agent image"
    exit 1
fi

echo ""

# Step 7: Post-build cluster sync
#   - Force flash-agent rolling restart so new image is picked up
#     (set image is a no-op when the tag is unchanged but the underlying
#      minikube image was replaced)
#   - If sock-shop helm release exists, upgrade it so the new
#     prometheus-configmap (cadvisor + KSM jobs) and kube-state-metrics
#     Deployment from app-charts reach the cluster immediately rather
#     than waiting for the next experiment to re-trigger install-app
#   - Restart prometheus to reload scrape config
#   - Verify monitoring stack reports the expected scrape jobs
log_info "Starting: Post-build cluster sync"
# Look for any flash-agent Deployment across common install namespaces.
# In prod-grade mode the agent lives in AGENT_INSTALL_NAMESPACE (e.g.
# agentcert-system) and observes sock-shop; in legacy mode it lives in
# sock-shop directly.  Restart whichever exists so the new image is picked
# up by the running pod.
FA_RESTARTED=0
for FA_NS in "${AGENT_INSTALL_NAMESPACE:-agentcert-system}" sock-shop; do
    if kubectl -n "${FA_NS}" get deploy flash-agent >/dev/null 2>&1; then
        log_info "Forcing flash-agent rollout restart in ${FA_NS}..."
        kubectl -n "${FA_NS}" rollout restart deploy/flash-agent >/dev/null 2>&1 || true
        kubectl -n "${FA_NS}" rollout status deploy/flash-agent --timeout=120s >/dev/null 2>&1 \
          && log_success "flash-agent rolled out in ${FA_NS}" \
          || log_warn "flash-agent rollout in ${FA_NS} did not complete in time (non-fatal)"
        FA_RESTARTED=1
    fi
done
if [[ "${FA_RESTARTED}" -eq 0 ]]; then
    log_info "flash-agent Deployment not found in any known ns — skipping rollout restart"
fi

# Helm upgrade sock-shop if a release exists, otherwise just verify monitoring
if kubectl get ns monitoring >/dev/null 2>&1; then
    if command -v helm >/dev/null 2>&1; then
        SOCKSHOP_RELEASE=$(helm list -A 2>/dev/null | awk '$1=="sock-shop"{print $1; exit}')
        SOCKSHOP_NS=$(helm list -A 2>/dev/null | awk '$1=="sock-shop"{print $2; exit}')
        SOCKSHOP_CHART_DIR=""
        for cand in "${APP_CHART_DIR%/install-app}/charts/sock-shop" \
                    /tmp/agentcert-build/app-charts/charts/sock-shop \
                    /mnt/d/Studies/app-charts/charts/sock-shop; do
            [[ -d "$cand" ]] && SOCKSHOP_CHART_DIR="$cand" && break
        done
        if [[ -n "${SOCKSHOP_RELEASE}" && -n "${SOCKSHOP_CHART_DIR}" ]]; then
            log_info "helm upgrade sock-shop in ns ${SOCKSHOP_NS} from ${SOCKSHOP_CHART_DIR}"
            helm upgrade sock-shop "${SOCKSHOP_CHART_DIR}" -n "${SOCKSHOP_NS}" \
                --set monitoring.enabled=true --reuse-values >/dev/null 2>&1 \
                && log_success "sock-shop chart upgraded" \
                || log_warn "helm upgrade failed (non-fatal) — re-run experiment to apply new chart"
        else
            log_info "No sock-shop helm release found — chart will load on next experiment"
        fi
    fi

    # Restart prometheus to reload scrape config
    if kubectl -n monitoring get deploy prometheus-deployment >/dev/null 2>&1; then
        log_info "Restarting prometheus-deployment to reload scrape config..."
        kubectl -n monitoring rollout restart deploy/prometheus-deployment >/dev/null 2>&1 || true
        kubectl -n monitoring rollout status deploy/prometheus-deployment --timeout=120s >/dev/null 2>&1 || true
    fi

    # Verify expected artifacts present
    if kubectl -n monitoring get deploy kube-state-metrics >/dev/null 2>&1; then
        log_success "kube-state-metrics Deployment exists"
    else
        log_warn "kube-state-metrics Deployment NOT found — re-run experiment so install-app applies new chart"
    fi
    if kubectl -n monitoring get cm prometheus-configmap -o jsonpath='{.data.prometheus\.yml}' 2>/dev/null | grep -q 'kubernetes-cadvisor'; then
        log_success "prometheus-configmap contains 'kubernetes-cadvisor' job"
    else
        log_warn "prometheus-configmap MISSING 'kubernetes-cadvisor' job"
    fi
    if kubectl -n monitoring get cm prometheus-configmap -o jsonpath='{.data.prometheus\.yml}' 2>/dev/null | grep -q 'kube-state-metrics'; then
        log_success "prometheus-configmap contains 'kube-state-metrics' job"
    else
        log_warn "prometheus-configmap MISSING 'kube-state-metrics' job"
    fi
else
    log_info "monitoring ns not present — skipping monitoring verify"
fi
log_success "Completed: Post-build cluster sync"

echo ""
echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}All builds completed successfully!${NC}"
echo -e "${GREEN}================================${NC}"
echo ""
