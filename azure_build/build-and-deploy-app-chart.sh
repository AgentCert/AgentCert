#!/usr/bin/env bash

set -euo pipefail

# Portable version of app-charts/install-app/build-and-deploy-app-chart.sh
# Added params vs original:
#   --source-dir PATH  path to install-app directory (sets SCRIPT_DIR + BUILD_CONTEXT)
#   --env-file   PATH  path to AgentCert .env file (overrides AGENTCERT_ENV_FILE)

# Defaults derived from script location — overridden by --source-dir / --env-file
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_CONTEXT="$(cd "$SCRIPT_DIR/.." && pwd)"

IMAGE_REGISTRY="${IMAGE_REGISTRY:-agentcert}"
IMAGE_NAME="${IMAGE_NAME:-agentcert-install-app}"
IMAGE_TAG="${IMAGE_TAG:-ci-$(date +%Y%m%d%H%M%S)}"
TAG_LATEST="${TAG_LATEST:-true}"
NO_CACHE="${NO_CACHE:-false}"
AGENTCERT_ENV_FILE="${AGENTCERT_ENV_FILE:-$BUILD_CONTEXT/../AgentCert/local-custom/config/.env}"
LOCAL_MODE=false

IMAGE_REPO="${IMAGE_REGISTRY}/${IMAGE_NAME}"
PRIMARY_IMAGE="${IMAGE_REPO}:${IMAGE_TAG}"
LATEST_IMAGE="${IMAGE_REPO}:latest"

SOCKSHOP_VALUES_FILE=""
SOCKSHOP_VALUES_BACKUP=""

usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Build and load install-app image into minikube, then update AgentCert .env.

Options:
  --source-dir PATH  Path to the install-app directory (app-charts/install-app)
  --env-file   PATH  Path to AgentCert .env file
  --local-mode       Build with local-friendly sock-shop tracing settings
                     (temporary override: tracing.disableSleuth=true, tracing.zipkinHost="")
  --help             Show this help
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --source-dir)
            SCRIPT_DIR="${2:-}"
            BUILD_CONTEXT="$(cd "${SCRIPT_DIR}/.." && pwd)"
            shift 2
            ;;
        --env-file)
            AGENTCERT_ENV_FILE="${2:-}"
            shift 2
            ;;
        --local-mode)
            LOCAL_MODE=true
            shift
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        *)
            printf '[ERROR] Unknown option: %s\n' "$1" >&2
            usage
            exit 1
            ;;
    esac
done

SOCKSHOP_VALUES_FILE="$BUILD_CONTEXT/charts/sock-shop/values.yaml"

info() {
    printf '\n[INFO] %s\n' "$1"
}

success() {
    printf '[OK] %s\n' "$1"
}

warn() {
    printf '[WARN] %s\n' "$1"
}

require_cmd() {
    command -v "$1" >/dev/null 2>&1 || {
        printf '[ERROR] Required command not found: %s\n' "$1" >&2
        exit 1
    }
}

restore_sockshop_values() {
    if [[ -n "$SOCKSHOP_VALUES_BACKUP" && -f "$SOCKSHOP_VALUES_BACKUP" ]]; then
        mv "$SOCKSHOP_VALUES_BACKUP" "$SOCKSHOP_VALUES_FILE"
        success "Restored sock-shop values.yaml defaults"
    fi
}

prepare_local_mode_overrides() {
    if [[ "$LOCAL_MODE" != "true" ]]; then
        return 0
    fi

    if [[ ! -f "$SOCKSHOP_VALUES_FILE" ]]; then
        warn "sock-shop values not found at $SOCKSHOP_VALUES_FILE; skipping local-mode chart overrides"
        return 0
    fi

    info "Applying local-mode tracing overrides to sock-shop chart (temporary)"
    SOCKSHOP_VALUES_BACKUP="${SOCKSHOP_VALUES_FILE}.bak.$(date +%Y%m%d%H%M%S)"
    cp "$SOCKSHOP_VALUES_FILE" "$SOCKSHOP_VALUES_BACKUP"

    sed -i 's|^\([[:space:]]*zipkinHost:[[:space:]]*\).*|\1""|' "$SOCKSHOP_VALUES_FILE"
    sed -i 's|^\([[:space:]]*disableSleuth:[[:space:]]*\).*|\1true|' "$SOCKSHOP_VALUES_FILE"

    trap restore_sockshop_values EXIT
    success "Local-mode overrides applied: tracing.disableSleuth=true, tracing.zipkinHost=\"\""
}

upsert_env_value() {
    local key="$1"
    local value="$2"
    local escaped
    escaped=$(printf '%s' "$value" | sed 's/[&/\\]/\\&/g')

    if grep -q -E "^${key}=" "$AGENTCERT_ENV_FILE"; then
        sed -i "s/^${key}=.*/${key}=${escaped}/" "$AGENTCERT_ENV_FILE"
    else
        printf '\n%s=%s\n' "$key" "$value" >> "$AGENTCERT_ENV_FILE"
    fi
}

build_image() {
    local build_args=(docker build -t "$PRIMARY_IMAGE" -f "$SCRIPT_DIR/Dockerfile")
    if [[ "$NO_CACHE" == "true" ]]; then
        build_args+=(--no-cache)
    fi
    build_args+=("$BUILD_CONTEXT")

    info "Building ${PRIMARY_IMAGE}"
    "${build_args[@]}"

    if [[ "$TAG_LATEST" == "true" ]]; then
        info "Tagging ${PRIMARY_IMAGE} as ${LATEST_IMAGE}"
        docker tag "$PRIMARY_IMAGE" "$LATEST_IMAGE"
    fi

    success "Docker build completed"
}

prune_local_images() {
    local keep_refs=("$PRIMARY_IMAGE")
    if [[ "$TAG_LATEST" == "true" ]]; then
        keep_refs+=("$LATEST_IMAGE")
    fi

    info "Pruning older local Docker images for ${IMAGE_REPO}"
    while IFS= read -r ref; do
        [[ -z "$ref" || "$ref" == "<none>:<none>" ]] && continue

        local keep=false
        for wanted in "${keep_refs[@]}"; do
            if [[ "$ref" == "$wanted" ]]; then
                keep=true
                break
            fi
        done

        if [[ "$keep" == "false" ]]; then
            docker rmi -f "$ref" >/dev/null 2>&1 || warn "Failed to remove local image ${ref}"
        fi
    done < <(docker images "$IMAGE_REPO" --format '{{.Repository}}:{{.Tag}}' | sort -u)

    docker image prune -f >/dev/null 2>&1 || true
    success "Local Docker image prune complete"
}

update_agentcert_env() {
    if [[ ! -f "$AGENTCERT_ENV_FILE" ]]; then
        warn "AgentCert .env not found at ${AGENTCERT_ENV_FILE}; skipping env update"
        return 0
    fi

    info "Updating AgentCert .env with ${PRIMARY_IMAGE}"
    upsert_env_value "INSTALL_APPLICATION_IMAGE" "$PRIMARY_IMAGE"
    success "AgentCert .env updated: INSTALL_APPLICATION_IMAGE=${PRIMARY_IMAGE}"
}

show_result() {
    printf '\nBuilt image: %s\n' "$PRIMARY_IMAGE"
    if [[ "$TAG_LATEST" == "true" ]]; then
        printf 'Alias image: %s\n' "$LATEST_IMAGE"
    fi
    printf 'Updated .env: %s\n' "$AGENTCERT_ENV_FILE"
}

push_to_dockerhub() {
    local dh_user dh_token
    dh_user=$(get_env_value "DOCKERHUB_USERNAME" "${AGENTCERT_ENV_FILE}")
    dh_token=$(get_env_value "DOCKERHUB_TOKEN" "${AGENTCERT_ENV_FILE}")
    if [[ -z "${dh_user}" || -z "${dh_token}" ]]; then
        warn "DOCKERHUB_USERNAME or DOCKERHUB_TOKEN not set in .env; skipping Docker Hub push"
        return 0
    fi
    info "Pushing to Docker Hub as ${dh_user}..."
    echo "${dh_token}" | docker login -u "${dh_user}" --password-stdin
    docker push "${PRIMARY_IMAGE}"
    if [[ "${TAG_LATEST}" == "true" ]]; then
        docker push "${LATEST_IMAGE}"
    fi
    docker logout >/dev/null 2>&1 || true
    success "Pushed to Docker Hub: ${PRIMARY_IMAGE}"
}

main() {
    require_cmd docker

    prepare_local_mode_overrides

    build_image
    push_to_dockerhub
    prune_local_images
    update_agentcert_env
    show_result
}

main "$@"
