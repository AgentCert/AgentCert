#!/bin/bash

################################################################################
# AgentCert Complete Build and Deploy Script
# ============================================
# Production-ready build pipeline with:
# - Prerequisite validation
# - Docker image cleanup (local + minikube)
# - Multi-stage Go builds
# - Image loading into minikube
# - Kubernetes deployment
# - Health verification
#
# Usage: bash build-and-deploy.sh [OPTIONS]
# Options:
#   --clean-all     : Full cleanup including K8s namespace
#   --skip-build    : Skip building, only deploy
#   --skip-deploy   : Only build, don't deploy
#   --debug         : Enable debug logging
#   --env-file PATH : Path to .env file for build args (default: local-custom/config/.env)
################################################################################

set -euo pipefail

# ============================================================================
# WSL PATH HANDLING
# ============================================================================
# Convert Windows paths to WSL paths if running in WSL
if grep -qi microsoft /proc/version 2>/dev/null; then
    WSL_MODE=true
else
    WSL_MODE=false
fi

# ============================================================================
# CONFIGURATION
# ============================================================================
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="litmus-chaos"
DOCKER_REGISTRY="litmuschaos"
AI_OPS_ENV_FILE="${AI_OPS_ENV_FILE:-$PROJECT_ROOT/../AI_Ops/config/.env}"
LOCAL_ENV_FILE="$PROJECT_ROOT/local-custom/config/.env"

# Use a unique tag by default to avoid stale images with the same tag
UNIQUE_TAG=${UNIQUE_TAG:-true}
if [ "$UNIQUE_TAG" = true ]; then
    IMAGE_TAG="ci-$(date +%Y%m%d%H%M%S)"
else
    IMAGE_TAG="${IMAGE_TAG:-ci}"
fi

AUTH_SERVER_IMAGE="${DOCKER_REGISTRY}/litmusportal-auth-server:${IMAGE_TAG}"
GRAPHQL_SERVER_IMAGE="${DOCKER_REGISTRY}/litmusportal-server:${IMAGE_TAG}"
BUILD_LOG_FILE="/tmp/agentcert-build-$(date +%Y%m%d-%H%M%S).log"

# Command line arguments
CLEAN_ALL=false
SKIP_BUILD=false
SKIP_DEPLOY=false
DEBUG=false
CUSTOM_ENV_FILE=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --clean-all) CLEAN_ALL=true; shift ;;
        --skip-build) SKIP_BUILD=true; shift ;;
        --skip-deploy) SKIP_DEPLOY=true; shift ;;
        --debug) DEBUG=true; shift ;;
        --env-file) CUSTOM_ENV_FILE="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

# Override LOCAL_ENV_FILE if --env-file parameter is provided
if [ -n "$CUSTOM_ENV_FILE" ]; then
    LOCAL_ENV_FILE="$CUSTOM_ENV_FILE"
    if [ ! -f "$LOCAL_ENV_FILE" ]; then
        echo "ERROR: Specified .env file not found: $LOCAL_ENV_FILE"
        exit 1
    fi
fi

[ "$DEBUG" = true ] && set -x

# ============================================================================
# COLOR CODES AND OUTPUT FUNCTIONS
# ============================================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

print_header() {
    echo ""
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║ ${CYAN}$1${BLUE}${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

print_section() {
    echo -e "${CYAN}→ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

log_to_file() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$BUILD_LOG_FILE"
}

# ============================================================================
# ENV SYNC HELPERS
# ============================================================================
json_escape() {
    printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

wsl_path_convert() {
    # Convert Windows paths to WSL paths if needed
    local path="$1"
    if [ "$WSL_MODE" = true ]; then
        # If path starts with /mnt/, it's already in WSL format
        if [[ "$path" == /mnt/* ]]; then
            echo "$path"
        else
            # Convert C:\... to /mnt/c/...
            echo "$path" | sed 's|^[A-Z]:\\|/mnt/\L&|;s|\\|/|g' | sed 's|:/|/|'
        fi
    else
        echo "$path"
    fi
}

get_env_value() {
    local key="$1"
    local file="$2"
    local raw
    raw=$(grep -E "^${key}[[:space:]]*=" "$file" | tail -1 | cut -d'=' -f2-)
    raw=$(echo "$raw" | sed 's/^ *//;s/ *$//')
    # Remove all line ending characters (CR and LF)
    raw=$(echo "$raw" | tr -d '\r\n')
    # Remove quotes
    raw=${raw#"\""}
    raw=${raw%"\""}
    raw=${raw#"'"}
    raw=${raw%"'"}
    echo "$raw"
}

sync_azure_env_from_dotenv() {
    local env_file="$AI_OPS_ENV_FILE"
    if [ ! -f "$env_file" ]; then
        env_file="$LOCAL_ENV_FILE"
    fi
    if [ ! -f "$env_file" ]; then
        print_warning ".env not found at $AI_OPS_ENV_FILE (or fallback $LOCAL_ENV_FILE); skipping Azure env sync"
        return 0
    fi

    print_section "Syncing Azure OpenAI env from .env to cluster"
    local endpoint deployment api_version embedding key
    endpoint=$(get_env_value "AZURE_OPENAI_ENDPOINT" "$env_file")
    deployment=$(get_env_value "AZURE_OPENAI_DEPLOYMENT" "$env_file")
    api_version=$(get_env_value "AZURE_OPENAI_API_VERSION" "$env_file")
    embedding=$(get_env_value "AZURE_OPENAI_EMBEDDING_DEPLOYMENT" "$env_file")
    key=$(get_env_value "AZURE_OPENAI_KEY" "$env_file")

    # Patch ConfigMap with non-sensitive values
    if [ -n "$endpoint" ] || [ -n "$deployment" ] || [ -n "$api_version" ] || [ -n "$embedding" ]; then
        local cm_patch="{\"data\":{"
        [ -n "$endpoint" ] && cm_patch+="\"AZURE_OPENAI_ENDPOINT\":\"$(json_escape "$endpoint")\","
        [ -n "$deployment" ] && cm_patch+="\"AZURE_OPENAI_DEPLOYMENT\":\"$(json_escape "$deployment")\","
        [ -n "$api_version" ] && cm_patch+="\"AZURE_OPENAI_API_VERSION\":\"$(json_escape "$api_version")\","
        [ -n "$embedding" ] && cm_patch+="\"AZURE_OPENAI_EMBEDDING_DEPLOYMENT\":\"$(json_escape "$embedding")\","
        cm_patch=${cm_patch%,}
        cm_patch+="}}"

        kubectl -n "$NAMESPACE" patch configmap litmus-portal-admin-config --type merge -p "$cm_patch" || true
    fi

    # Patch Secret with sensitive value
    if [ -n "$key" ]; then
        local sec_patch="{\"stringData\":{\"AZURE_OPENAI_KEY\":\"$(json_escape "$key")\"}}"
        kubectl -n "$NAMESPACE" patch secret litmus-portal-admin-secret --type merge -p "$sec_patch" || true
    fi
}

sync_langfuse_env_from_dotenv() {
    # Use local .env for Langfuse values
    local env_file="$LOCAL_ENV_FILE"
    if [ ! -f "$env_file" ]; then
        print_warning ".env not found at $LOCAL_ENV_FILE; skipping Langfuse env sync"
        return 0
    fi

    print_section "Syncing Langfuse env from .env to cluster"
    local public_key secret_key org_id project_id host
    host=$(get_env_value "LANGFUSE_HOST" "$env_file")
    public_key=$(get_env_value "LANGFUSE_PUBLIC_KEY" "$env_file")
    secret_key=$(get_env_value "LANGFUSE_SECRET_KEY" "$env_file")
    org_id=$(get_env_value "LANGFUSE_ORG_ID" "$env_file")
    project_id=$(get_env_value "LANGFUSE_PROJECT_ID" "$env_file")

    # Derive OTEL endpoint + headers from Langfuse credentials for the GraphQL server.
    # otel_tracer.go reads OTEL_EXPORTER_OTLP_ENDPOINT and OTEL_EXPORTER_OTLP_HEADERS.
    local otel_endpoint otel_headers
    if [ -n "$host" ] && [ -n "$public_key" ] && [ -n "$secret_key" ]; then
        otel_endpoint="${host}/api/public/otel"
        otel_headers="Authorization=Basic $(printf '%s' "$public_key:$secret_key" | base64 -w0)"
    fi

    # Patch ConfigMap with non-sensitive values
    if [ -n "$org_id" ] || [ -n "$project_id" ] || [ -n "$host" ] || [ -n "$otel_endpoint" ]; then
        local cm_patch="{\"data\":{"
        [ -n "$host" ] && cm_patch+="\"LANGFUSE_HOST\":\"$(json_escape "$host")\","
        [ -n "$org_id" ] && cm_patch+="\"LANGFUSE_ORG_ID\":\"$(json_escape "$org_id")\","
        [ -n "$project_id" ] && cm_patch+="\"LANGFUSE_PROJECT_ID\":\"$(json_escape "$project_id")\","
        [ -n "$otel_endpoint" ] && cm_patch+="\"OTEL_EXPORTER_OTLP_ENDPOINT\":\"$(json_escape "$otel_endpoint")\","
        cm_patch=${cm_patch%,}
        cm_patch+="}}"

        kubectl -n "$NAMESPACE" patch configmap litmus-portal-admin-config --type merge -p "$cm_patch" || true
    fi

    # Patch Secret with sensitive values
    if [ -n "$secret_key" ] || [ -n "$public_key" ] || [ -n "$otel_headers" ]; then
        local sec_patch="{\"stringData\":{"
        [ -n "$secret_key" ] && sec_patch+="\"LANGFUSE_SECRET_KEY\":\"$(json_escape "$secret_key")\","
        [ -n "$public_key" ] && sec_patch+="\"LANGFUSE_PUBLIC_KEY\":\"$(json_escape "$public_key")\","
        [ -n "$otel_headers" ] && sec_patch+="\"OTEL_EXPORTER_OTLP_HEADERS\":\"$(json_escape "$otel_headers")\","
        sec_patch=${sec_patch%,}
        sec_patch+="}}"

        kubectl -n "$NAMESPACE" patch secret litmus-portal-admin-secret --type merge -p "$sec_patch" || true
    fi
}

sync_install_agent_env_from_dotenv() {
    # Use local .env for install-agent image overrides consumed by GraphQL server.
    local env_file="$LOCAL_ENV_FILE"
    if [ ! -f "$env_file" ]; then
        print_warning ".env not found at $LOCAL_ENV_FILE; skipping install-agent env sync"
        return 0
    fi

    print_section "Syncing install-agent env from .env to GraphQL deployment"
    local install_agent_image install_agent_pull_policy pre_cleanup_wait_seconds agent_sidecar_image blind_traces
    install_agent_image=$(get_env_value "INSTALL_AGENT_IMAGE" "$env_file")
    install_agent_pull_policy=$(get_env_value "INSTALL_AGENT_IMAGE_PULL_POLICY" "$env_file")
    pre_cleanup_wait_seconds=$(get_env_value "PRE_CLEANUP_WAIT_SECONDS" "$env_file")
    agent_sidecar_image=$(get_env_value "AGENT_SIDECAR_IMAGE" "$env_file")
    blind_traces=$(get_env_value "BLIND_TRACES" "$env_file")

    if [ -z "$install_agent_image" ] && [ -z "$install_agent_pull_policy" ] && [ -z "$pre_cleanup_wait_seconds" ] && [ -z "$agent_sidecar_image" ]; then
        print_info "INSTALL_AGENT_IMAGE* not set in .env; skipping deployment env update"
        return 0
    fi

    if [ -z "$pre_cleanup_wait_seconds" ]; then
        pre_cleanup_wait_seconds="0"
    fi

    # Validate pull policy when provided to avoid pushing invalid values.
    if [ -n "$install_agent_pull_policy" ]; then
        case "$install_agent_pull_policy" in
            Always|IfNotPresent|Never) ;;
            *)
                print_warning "Invalid INSTALL_AGENT_IMAGE_PULL_POLICY='$install_agent_pull_policy'. Using IfNotPresent"
                install_agent_pull_policy="IfNotPresent"
                ;;
        esac
    fi

    local set_env_args=("deploy/litmusportal-server" "-n" "$NAMESPACE")
    [ -n "$install_agent_image" ] && set_env_args+=("INSTALL_AGENT_IMAGE=$install_agent_image")
    [ -n "$install_agent_pull_policy" ] && set_env_args+=("INSTALL_AGENT_IMAGE_PULL_POLICY=$install_agent_pull_policy")
    [ -n "$agent_sidecar_image" ] && set_env_args+=("AGENT_SIDECAR_IMAGE=$agent_sidecar_image")
    set_env_args+=("PRE_CLEANUP_WAIT_SECONDS=$pre_cleanup_wait_seconds")
    [ -n "$blind_traces" ] && set_env_args+=("BLIND_TRACES=$blind_traces")

    kubectl set env "${set_env_args[@]}" >/dev/null
    print_success "Updated litmusportal-server env: INSTALL_AGENT_IMAGE* AGENT_SIDECAR_IMAGE PRE_CLEANUP_WAIT_SECONDS BLIND_TRACES"
}

sync_mongo_env_from_wsl() {
    print_section "Syncing MongoDB env from local WSL"

    local mongo_user mongo_pass mongo_auth_source mongo_host db_server
    mongo_user=$(get_env_value "MONGODB_USERNAME" "$LOCAL_ENV_FILE")
    mongo_pass=$(get_env_value "MONGODB_PASSWORD" "$LOCAL_ENV_FILE")
    mongo_auth_source=$(get_env_value "MONGODB_AUTH_SOURCE" "$LOCAL_ENV_FILE")
    mongo_host=$(get_minikube_host_ip)

    [ -z "$mongo_user" ] && mongo_user="root"
    [ -z "$mongo_pass" ] && mongo_pass="1234"
    [ -z "$mongo_auth_source" ] && mongo_auth_source="admin"

    if [ -z "$mongo_host" ]; then
        mongo_host=$(get_env_value "MONGODB_HOST" "$LOCAL_ENV_FILE")
    fi

    if [ -z "$mongo_host" ]; then
        print_warning "Unable to resolve MongoDB host from minikube or .env; skipping MongoDB env sync"
        return 0
    fi

    db_server="mongodb://${mongo_user}:${mongo_pass}@${mongo_host}:27017/${mongo_auth_source}"

    if kubectl -n "$NAMESPACE" get configmap litmus-portal-admin-config &>/dev/null; then
        kubectl -n "$NAMESPACE" patch configmap litmus-portal-admin-config --type merge \
            -p "{\"data\":{\"DB_SERVER\":\"$(json_escape "$db_server")\"}}" >/dev/null || true
    fi

    if kubectl -n "$NAMESPACE" get secret litmus-portal-admin-secret &>/dev/null; then
        kubectl -n "$NAMESPACE" patch secret litmus-portal-admin-secret --type merge \
            -p "{\"stringData\":{\"DB_USER\":\"$(json_escape "$mongo_user")\",\"DB_PASSWORD\":\"$(json_escape "$mongo_pass")\"}}" >/dev/null || true
    fi

    print_success "MongoDB runtime env synced (DB_SERVER host: $mongo_host)"
}

# ============================================================================
# RUNTIME RBAC BOOTSTRAP (LITMUS-EXP)
# ============================================================================
ensure_litmus_exp_runtime_rbac() {
        print_header "Ensuring Runtime RBAC in litmus-exp"

        local infra_namespace="litmus-exp"
    local infra_sa=""

        if ! kubectl get namespace "$infra_namespace" &> /dev/null; then
                print_info "Namespace $infra_namespace not found yet. Skipping runtime RBAC bootstrap."
                return 0
        fi

        # Prefer subscriber deployment SA when present, fall back to common SA names.
        if kubectl -n "$infra_namespace" get deployment subscriber &> /dev/null; then
            infra_sa=$(kubectl -n "$infra_namespace" get deployment subscriber -o jsonpath='{.spec.template.spec.serviceAccountName}' 2>/dev/null || true)
        fi

        if [ -z "$infra_sa" ]; then
            for candidate_sa in litmus-exp argo-chaos default; do
                if kubectl -n "$infra_namespace" get serviceaccount "$candidate_sa" &> /dev/null; then
                    infra_sa="$candidate_sa"
                    break
                fi
            done
        fi

        if [ -z "$infra_sa" ]; then
            print_info "No suitable ServiceAccount found in $infra_namespace yet. Skipping runtime RBAC bootstrap."
            return 0
        fi

        print_info "Using ServiceAccount $infra_namespace/$infra_sa"

        print_section "Applying cluster-scope watcher permissions for subscriber"
        kubectl create clusterrolebinding infra-cluster-role-binding-${infra_namespace}-${infra_sa} \
                --clusterrole=infra-cluster-role \
                --serviceaccount="${infra_namespace}:${infra_sa}" \
                --dry-run=client -o yaml | kubectl apply -f -

        print_section "Applying namespace pod-read permissions for subscriber"
        kubectl -n "$infra_namespace" create role subscriber-pod-reader \
            --verb=get,list,watch \
            --resource=pods \
            --dry-run=client -o yaml | kubectl apply -f -

        kubectl -n "$infra_namespace" create rolebinding subscriber-pod-reader-binding \
            --role=subscriber-pod-reader \
            --serviceaccount="${infra_namespace}:${infra_sa}" \
            --dry-run=client -o yaml | kubectl apply -f -

        if kubectl -n "$infra_namespace" get deployment subscriber &> /dev/null; then
                print_section "Restarting subscriber to pick updated RBAC"
                kubectl -n "$infra_namespace" rollout restart deployment/subscriber || true
        else
                print_info "Subscriber deployment not found in $infra_namespace (yet)."
        fi

        print_success "Runtime RBAC bootstrap check complete"
}

# ============================================================================
# PREREQUISITE CHECK
# ============================================================================
check_prerequisites() {
    print_header "Checking Prerequisites"
    
    local ok=true
    
    # Docker
    print_section "Checking Docker..."
    if ! command -v docker &> /dev/null; then
        print_error "Docker not found"; ok=false
    elif ! docker ps &> /dev/null; then
        print_error "Docker daemon not running"; ok=false
    else
        print_success "Docker: $(docker --version)"
    fi
    
    # kubectl
    print_section "Checking kubectl..."
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl not found"; ok=false
    else
        print_success "kubectl ready"
    fi
    
    # minikube
    print_section "Checking minikube..."
    if ! command -v minikube &> /dev/null; then
        print_error "minikube not found"; ok=false
    else
        if ! minikube status &> /dev/null; then
            print_warning "minikube not running. Starting..."
            minikube start || { print_error "Failed to start minikube"; ok=false; }
        fi
        print_success "minikube running"
    fi
    
    [ "$ok" = false ] && { echo ""; print_error "Prerequisites missing"; exit 1; }
    print_success "All prerequisites OK"
}

# ============================================================================
# MONGODB REPLICA SET FIX (MINIKUBE CONNECTIVITY)
# ============================================================================
get_minikube_host_ip() {
    minikube ssh "getent hosts host.minikube.internal | awk '{print \$1}'" 2>/dev/null | head -n 1 | tr -d '\r\n'
}

ensure_mongo_replset_host() {
    print_header "Ensuring MongoDB Replica Set Host"

    if ! command -v mongosh &> /dev/null; then
        print_warning "mongosh not found. Skipping MongoDB replica set check."
        return 0
    fi

    local host_ip
    host_ip=$(get_minikube_host_ip)
    if [ -z "$host_ip" ]; then
        print_warning "Unable to resolve host.minikube.internal inside minikube. Skipping replica set update."
        return 0
    fi

    print_info "Minikube host IP: $host_ip"

    # Check current replica set host
    local current_host
    current_host=$(mongosh --quiet --username root --password 1234 --authenticationDatabase admin \
        --eval "try { cfg=rs.conf(); print(cfg.members[0].host); } catch(e) { print('NOT_INIT'); }" 2>/dev/null | tail -n 1 | tr -d '\r\n')

    if [ "$current_host" = "NOT_INIT" ]; then
        print_warning "Replica set not initialized. Initializing with host ${host_ip}:27017..."
        mongosh --username root --password 1234 --authenticationDatabase admin \
            --eval "rs.initiate({ _id: 'rs0', members: [ { _id: 0, host: '${host_ip}:27017' } ] })" \
            && print_success "Replica set initialized" || print_warning "Replica set init failed"
        return 0
    fi

    if [ "$current_host" != "${host_ip}:27017" ]; then
        print_warning "Replica set host is '$current_host'. Updating to '${host_ip}:27017'..."
        mongosh --username root --password 1234 --authenticationDatabase admin \
            --eval "cfg=rs.conf(); cfg.members[0].host='${host_ip}:27017'; rs.reconfig(cfg, {force:true})" \
            && print_success "Replica set host updated" || print_warning "Replica set update failed"
    else
        print_success "Replica set host already correct"
    fi
}

# ============================================================================
# CLEANUP
# ============================================================================
cleanup_docker() {
    print_header "Cleaning Docker Images"
    
    # Delete the specific current images if they exist
    docker rmi -f "$AUTH_SERVER_IMAGE" 2>/dev/null || print_info "Auth Server current image not found locally"
    docker rmi -f "$GRAPHQL_SERVER_IMAGE" 2>/dev/null || print_info "GraphQL Server current image not found locally"
    
    # Clean dangling images
    docker image prune -f --filter "dangling=true" > /dev/null 2>&1 || true
    
    # Remove old versions - keep only latest 2 for each service
    print_section "Removing old image versions (keeping latest 2)..."
    
    # Clean old auth server images
    local auth_images=$(docker images "litmuschaos/litmusportal-auth-server" --format "{{.Tag}}" | grep "^ci-" | sort -V)
    local auth_count=$(echo "$auth_images" | wc -l)
    if [ "$auth_count" -gt 2 ]; then
        echo "$auth_images" | head -n -2 | while read tag; do
            print_info "Removing: litmuschaos/litmusportal-auth-server:$tag"
            docker rmi -f "litmuschaos/litmusportal-auth-server:$tag" 2>/dev/null || true
        done
    fi
    
    # Clean old graphql server images
    local gql_images=$(docker images "litmuschaos/litmusportal-server" --format "{{.Tag}}" | grep "^ci-" | sort -V)
    local gql_count=$(echo "$gql_images" | wc -l)
    if [ "$gql_count" -gt 2 ]; then
        echo "$gql_images" | head -n -2 | while read tag; do
            print_info "Removing: litmuschaos/litmusportal-server:$tag"
            docker rmi -f "litmuschaos/litmusportal-server:$tag" 2>/dev/null || true
        done
    fi
    
    print_success "Docker cleanup done"
}

cleanup_minikube() {
    print_header "Cleaning Minikube Images"
    
    # Remove current images from minikube
    minikube image rm "$AUTH_SERVER_IMAGE" 2>/dev/null || print_info "Auth Server not in minikube"
    minikube image rm "$GRAPHQL_SERVER_IMAGE" 2>/dev/null || print_info "GraphQL Server not in minikube"
    
    # Remove old versions - keep only latest 2 for each service
    print_section "Removing old versions from minikube (keeping latest 2)..."
    
    # Clean old auth server images from minikube
    local auth_images=$(minikube image ls 2>/dev/null | grep "litmuschaos/litmusportal-auth-server:ci-" | sed 's/.*litmusportal-auth-server://' | sort -V)
    local auth_count=$(echo "$auth_images" | wc -l)
    if [ "$auth_count" -gt 2 ]; then
        echo "$auth_images" | head -n -2 | while read tag; do
            print_info "Removing from minikube: litmuschaos/litmusportal-auth-server:$tag"
            minikube image rm "litmuschaos/litmusportal-auth-server:$tag" 2>/dev/null || true
        done
    fi
    
    # Clean old graphql server images from minikube
    local gql_images=$(minikube image ls 2>/dev/null | grep "litmuschaos/litmusportal-server:ci-" | sed 's/.*litmusportal-server://' | sort -V)
    local gql_count=$(echo "$gql_images" | wc -l)
    if [ "$gql_count" -gt 2 ]; then
        echo "$gql_images" | head -n -2 | while read tag; do
            print_info "Removing from minikube: litmuschaos/litmusportal-server:$tag"
            minikube image rm "litmuschaos/litmusportal-server:$tag" 2>/dev/null || true
        done
    fi
    
    print_success "Minikube cleanup done"
}

cleanup_generated_code() {
    print_header "Cleaning Generated Code"

    # Optional: delete generated GraphQL files only when FORCE_REGEN=true
    if [ "${FORCE_REGEN:-false}" = true ]; then
        rm -f "$PROJECT_ROOT/chaoscenter/graphql/server/graph/generated/generated.go" 2>/dev/null && \
            print_success "Deleted GraphQL generated.go - will regenerate from schema" || \
            print_info "GraphQL generated.go not found (first build)"
    else
        print_info "Skipping generated.go delete (set FORCE_REGEN=true to force)"
    fi

    print_success "Generated code cleanup done"
}

cleanup_k8s() {
    print_header "Cleaning Kubernetes"
    if kubectl get namespace "$NAMESPACE" &> /dev/null; then
        print_info "Deleting namespace $NAMESPACE..."
        kubectl delete namespace "$NAMESPACE"
        print_info "Waiting for deletion..."
        sleep 30
        print_success "Namespace deleted"
    fi
}

# ============================================================================
# BUILD
# ============================================================================
build_image() {
    local dockerfile=$1
    local context=$2
    local image=$3
    local name=$4
    
    print_section "Building $name..."
    print_info "Context: $context"
    
    # Convert paths to WSL format if running in WSL
    local dockerfile_path=$(wsl_path_convert "$dockerfile")
    local context_path=$(wsl_path_convert "$context")
    
    [ ! -d "$context_path" ] && { print_error "Context not found: $context_path"; return 1; }
    
    # Extract Langfuse credentials from local .env for docker build args
    local langfuse_host=""
    local langfuse_public_key=""
    local langfuse_secret_key=""
    local langfuse_org_id=""
    local langfuse_project_id=""
    
    if [ -f "$LOCAL_ENV_FILE" ]; then
        langfuse_host=$(get_env_value "LANGFUSE_HOST" "$LOCAL_ENV_FILE")
        langfuse_public_key=$(get_env_value "LANGFUSE_PUBLIC_KEY" "$LOCAL_ENV_FILE")
        langfuse_secret_key=$(get_env_value "LANGFUSE_SECRET_KEY" "$LOCAL_ENV_FILE")
        langfuse_org_id=$(get_env_value "LANGFUSE_ORG_ID" "$LOCAL_ENV_FILE")
        langfuse_project_id=$(get_env_value "LANGFUSE_PROJECT_ID" "$LOCAL_ENV_FILE")
    fi
    
    local start=$(date +%s)
    if docker build -t "$image" -f "$dockerfile_path" "$context_path" \
        --build-arg TARGETOS=linux --build-arg TARGETARCH=amd64 \
        --build-arg LANGFUSE_HOST="$langfuse_host" \
        --build-arg LANGFUSE_PUBLIC_KEY="$langfuse_public_key" \
        --build-arg LANGFUSE_SECRET_KEY="$langfuse_secret_key" \
        --build-arg LANGFUSE_ORG_ID="$langfuse_org_id" \
        --build-arg LANGFUSE_PROJECT_ID="$langfuse_project_id" \
        2>&1 | tee -a "$BUILD_LOG_FILE"; then
        local elapsed=$(($(date +%s) - start))
        print_success "$name built (${elapsed}s)"
        return 0
    else
        print_error "Build failed"; return 1
    fi
}

build_all_images() {
    print_header "Building Docker Images"
    echo ""
    
    build_image \
        "$PROJECT_ROOT/chaoscenter/authentication/Dockerfile" \
        "$PROJECT_ROOT/chaoscenter/authentication" \
        "$AUTH_SERVER_IMAGE" \
        "Auth Server" || exit 1
    
    echo ""

    # GraphQL server is built with --no-cache because source files live on the
    # Windows NTFS filesystem (accessed via WSL 9P mount).  Docker cannot
    # reliably detect mtime changes across that boundary, so the COPY layer
    # gets cache-hit even when Go source files have changed.
    local dockerfile_path=$(wsl_path_convert "$PROJECT_ROOT/chaoscenter/graphql/server/Dockerfile")
    local context_path=$(wsl_path_convert "$PROJECT_ROOT/chaoscenter/graphql")
    local langfuse_host="" langfuse_public_key="" langfuse_secret_key="" langfuse_org_id="" langfuse_project_id=""
    if [ -f "$LOCAL_ENV_FILE" ]; then
        langfuse_host=$(get_env_value "LANGFUSE_HOST" "$LOCAL_ENV_FILE")
        langfuse_public_key=$(get_env_value "LANGFUSE_PUBLIC_KEY" "$LOCAL_ENV_FILE")
        langfuse_secret_key=$(get_env_value "LANGFUSE_SECRET_KEY" "$LOCAL_ENV_FILE")
        langfuse_org_id=$(get_env_value "LANGFUSE_ORG_ID" "$LOCAL_ENV_FILE")
        langfuse_project_id=$(get_env_value "LANGFUSE_PROJECT_ID" "$LOCAL_ENV_FILE")
    fi
    print_section "Building GraphQL Server (--no-cache)..."
    local start=$(date +%s)
    if docker build --no-cache -t "$GRAPHQL_SERVER_IMAGE" \
        -f "$dockerfile_path" "$context_path" \
        --build-arg TARGETOS=linux --build-arg TARGETARCH=amd64 \
        --build-arg LANGFUSE_HOST="$langfuse_host" \
        --build-arg LANGFUSE_PUBLIC_KEY="$langfuse_public_key" \
        --build-arg LANGFUSE_SECRET_KEY="$langfuse_secret_key" \
        --build-arg LANGFUSE_ORG_ID="$langfuse_org_id" \
        --build-arg LANGFUSE_PROJECT_ID="$langfuse_project_id" \
        2>&1 | tee -a "$BUILD_LOG_FILE"; then
        local elapsed=$(($(date +%s) - start))
        print_success "GraphQL Server built (${elapsed}s)"
    else
        print_error "GraphQL Server build failed"; exit 1
    fi

    print_success "All images built"
}

load_to_minikube() {
    print_header "Loading Images to Minikube"
    
    # Clean old images from minikube to avoid confusion
    print_info "Cleaning old images from minikube..."
    minikube image rm "$AUTH_SERVER_IMAGE" 2>/dev/null || true
    minikube image rm "$GRAPHQL_SERVER_IMAGE" 2>/dev/null || true
    # Also clean any old :ci tagged images (except the current one)
    minikube image ls 2>/dev/null | grep "litmusportal-server:ci-" | while read img; do
        [ "$img" != "$GRAPHQL_SERVER_IMAGE" ] && minikube image rm "$img" 2>/dev/null || true
    done
    minikube image ls 2>/dev/null | grep "litmusportal-auth-server:ci-" | while read img; do
        [ "$img" != "$AUTH_SERVER_IMAGE" ] && minikube image rm "$img" 2>/dev/null || true
    done
    
    # Load new images
    print_info "Loading new Auth Server image: $AUTH_SERVER_IMAGE"
    minikube image load "$AUTH_SERVER_IMAGE" || { print_error "Load failed"; return 1; }
    print_success "Auth Server loaded"
    
    print_info "Loading new GraphQL Server image: $GRAPHQL_SERVER_IMAGE"
    minikube image load "$GRAPHQL_SERVER_IMAGE" || { print_error "Load failed"; return 1; }
    print_success "GraphQL Server loaded"
}

wait_for_deployment() {
    local deployment_name="$1"
    local namespace="$2"
    local timeout_seconds="${3:-120}"
    local elapsed=0

    while [ "$elapsed" -lt "$timeout_seconds" ]; do
        if kubectl get deployment "$deployment_name" -n "$namespace" >/dev/null 2>&1; then
            if kubectl rollout status deployment/"$deployment_name" -n "$namespace" --timeout=20s >/dev/null 2>&1; then
                print_success "Deployment ready: $deployment_name"
                return 0
            fi
        fi

        sleep 2
        elapsed=$((elapsed + 2))
    done

    print_warning "Deployment not ready within ${timeout_seconds}s: $deployment_name"
    return 1
}

# ============================================================================
# DEPLOY
# ============================================================================
create_namespace() {
    print_header "Creating Kubernetes Namespace"
    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        kubectl create namespace "$NAMESPACE"
        sleep 2
    fi
    print_success "Namespace ready"
}

    ensure_required_namespaces() {
        print_header "Ensuring Required Namespaces"

        local required_namespaces=("$NAMESPACE" "litmus-exp")
        local ns
        for ns in "${required_namespaces[@]}"; do
            if ! kubectl get namespace "$ns" &> /dev/null; then
                print_info "Creating namespace: $ns"
                kubectl create namespace "$ns" >/dev/null
            fi

            if kubectl wait --for=condition=Established namespace/"$ns" --timeout=20s &>/dev/null; then
                print_success "Namespace ready: $ns"
            else
                # Fallback for clusters that do not report Established on Namespace.
                if kubectl get namespace "$ns" &>/dev/null; then
                    print_success "Namespace present: $ns"
                else
                    print_warning "Namespace $ns not confirmed yet"
                fi
            fi
        done
    }

deploy_manifest() {
    print_header "Deploying Manifest"
    local manifest="$PROJECT_ROOT/local-custom/k8s/litmus-installation.yaml"
    [ ! -f "$manifest" ] && { print_error "Manifest not found"; return 1; }
    
    print_info "Applying manifest..."
    kubectl apply -f "$manifest"
    sleep 3

    # On first install, deployments may take time to appear after manifest apply.
    print_info "Waiting for core deployments to be created..."
    wait_for_deployment "litmusportal-auth-server" "$NAMESPACE" 120 || true
    wait_for_deployment "litmusportal-server" "$NAMESPACE" 120 || true

    # Sync MongoDB connection details from local WSL/minikube host mapping
    sync_mongo_env_from_wsl

    # Sync Azure OpenAI values from local .env into cluster config/secret
    sync_azure_env_from_dotenv

    # Sync Langfuse values from AI_Ops .env into cluster config/secret
    sync_langfuse_env_from_dotenv

    # Sync install-agent image overrides consumed by GraphQL workflow processing
    sync_install_agent_env_from_dotenv
    
    # Apply Litmus configuration fixes for offline/minikube environments
    print_info "Applying Litmus configuration fixes..."
    bash "$SCRIPT_DIR/fix-litmus-config.sh" "$NAMESPACE" || print_warning "Litmus config fixes encountered an issue, but deployment continues"
    
    # Force new images (unique tag) onto the deployments with explicit image pull policy
    print_info "Updating Auth Server deployment with image: $AUTH_SERVER_IMAGE"
    if kubectl get deployment litmusportal-auth-server -n "$NAMESPACE" &>/dev/null; then
        kubectl set image deployment/litmusportal-auth-server auth-server="$AUTH_SERVER_IMAGE" -n "$NAMESPACE" --record
    else
        print_warning "Auth server deployment not found yet, skipping image update"
    fi
    
    print_info "Updating GraphQL Server deployment with image: $GRAPHQL_SERVER_IMAGE"
    if kubectl get deployment litmusportal-server -n "$NAMESPACE" &>/dev/null; then
        kubectl set image deployment/litmusportal-server graphql-server="$GRAPHQL_SERVER_IMAGE" -n "$NAMESPACE" --record
    else
        print_warning "GraphQL server deployment not found yet, skipping image update"
    fi
    
    # Restart deployments to pick up the new image tag immediately
    print_info "Restarting deployments..."
    kubectl rollout restart deployment/litmusportal-auth-server -n "$NAMESPACE"
    kubectl rollout restart deployment/litmusportal-server -n "$NAMESPACE"
    
    print_info "Waiting 20 seconds for pods to start..."
    sleep 20
    
    print_info "Current pods status:"
    kubectl get pods -n "$NAMESPACE" -o wide || true
}

verify_pods() {
    print_header "Verifying Deployment"
    local max=30
    local attempt=0
    
    while [ $attempt -lt $max ]; do
        attempt=$((attempt + 1))
        local auth=$(kubectl get pods -n "$NAMESPACE" -l component=litmusportal-auth-server -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "")
        local gql=$(kubectl get pods -n "$NAMESPACE" -l component=litmusportal-server -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "")
        
        if [ "$auth" = "True" ] && [ "$gql" = "True" ]; then
            print_success "All pods ready!"
            kubectl get pods -n "$NAMESPACE" -o wide
            return 0
        fi
        print_info "Waiting... ($attempt/$max)"
        sleep 5
    done
    
    print_warning "Timeout waiting for pods. Current status:"
    kubectl get pods -n "$NAMESPACE" -o wide || true
}

# ==========================================================================
# ENV SYNC (RUNNING CLUSTER)
# ==========================================================================
sync_envs_if_namespace_exists() {
    if kubectl get namespace "$NAMESPACE" &> /dev/null; then
        print_header "Syncing Env to Running Cluster"
        sync_mongo_env_from_wsl
        sync_azure_env_from_dotenv
        sync_langfuse_env_from_dotenv
        sync_install_agent_env_from_dotenv
        ensure_litmus_exp_runtime_rbac
        print_info "Restarting GraphQL Server to pick env changes"
        kubectl rollout restart deployment/litmusportal-server -n "$NAMESPACE" || true
    fi
}

# ============================================================================
# SUMMARY
# ============================================================================
display_info() {
    print_header "Deployment Summary"
    echo -e "${CYAN}Build log:${NC} $BUILD_LOG_FILE"
    echo ""
    kubectl get all -n "$NAMESPACE" 2>/dev/null || true
}

display_next() {
    print_header "Next Steps"
    echo -e "${YELLOW}1. Port Forward (new terminal):${NC}"
    echo "   bash $PROJECT_ROOT/local-custom/scripts/port-forward.sh"
    echo ""
    echo -e "${YELLOW}2. Frontend (new terminal):${NC}"
    echo "   bash $PROJECT_ROOT/local-custom/scripts/start-web.sh"
    echo "   # or: cd $PROJECT_ROOT/chaoscenter/web && npm run dev"
    echo ""
    echo -e "${YELLOW}3. Access:${NC}"
    echo "   https://localhost:2001"
    echo "   Username: admin / Password: litmus"
    echo ""
    echo -e "${YELLOW}4. Logs:${NC}"
    echo "   kubectl logs -n $NAMESPACE -f deployment/litmusportal-auth-server"
    echo "   kubectl logs -n $NAMESPACE -f deployment/litmusportal-server"
    echo ""
}

# ============================================================================
# MAIN
# ============================================================================
main() {
    print_header "AgentCert Build & Deploy Pipeline"
    print_info "Using .env file: $LOCAL_ENV_FILE"
    log_to_file "========== Build Started =========="
    
    check_prerequisites
        ensure_required_namespaces
    ensure_mongo_replset_host

    # Always sync envs to running cluster (if present)
    sync_envs_if_namespace_exists
    
    [ "$CLEAN_ALL" = true ] && cleanup_k8s
    cleanup_docker
    cleanup_minikube
    cleanup_generated_code
    
    [ "$SKIP_BUILD" = false ] && { build_all_images; load_to_minikube; } || print_warning "Skipping build"
    [ "$SKIP_DEPLOY" = false ] && { create_namespace; deploy_manifest; sync_langfuse_env_from_dotenv; sync_azure_env_from_dotenv; sync_install_agent_env_from_dotenv; ensure_litmus_exp_runtime_rbac; kubectl rollout restart deployment/litmusportal-server -n "$NAMESPACE"; verify_pods; } || print_warning "Skipping deploy"
    
    display_info
    display_next
    
    print_header "✓ Pipeline Complete!"
    log_to_file "========== Build Completed =========="
}

main
