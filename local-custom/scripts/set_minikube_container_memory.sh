#!/usr/bin/env bash
set -euo pipefail

PROFILE="minikube"
MEMORY="10g"
MEMORY_SWAP=""
START_IF_NEEDED=false
PERSIST_CONFIG=false

usage() {
  cat <<'EOF'
Usage:
  ./set_minikube_container_memory.sh [options]

Options:
  -p, --profile <name>       Minikube profile / Docker container name (default: minikube)
  -m, --memory <value>       Docker memory limit to set (default: 10g)
  --memory-swap <value>      Docker memory+swap limit (default: same as --memory)
  --start-if-needed          Start minikube if the profile is not running
  --persist                  Save the memory value into minikube config for future starts
  --status                   Show current Docker memory settings and exit
  -h, --help                 Show this help

Examples:
  ./set_minikube_container_memory.sh
  ./set_minikube_container_memory.sh --memory 10g --persist
  ./set_minikube_container_memory.sh --profile minikube --start-if-needed
  ./set_minikube_container_memory.sh --status

Notes:
  - This updates the Docker container that backs minikube without recreating the cluster.
  - If you use the docker driver, the container name normally matches the minikube profile.
  - Kubernetes node allocatable/capacity may not refresh until kubelet or minikube restarts.
EOF
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Error: required command not found: $cmd" >&2
    exit 1
  fi
}

parse_args() {
  local show_status=false

  while [[ $# -gt 0 ]]; do
    case "$1" in
      -p|--profile)
        PROFILE="${2:-}"
        shift 2
        ;;
      -m|--memory)
        MEMORY="${2:-}"
        shift 2
        ;;
      --memory-swap)
        MEMORY_SWAP="${2:-}"
        shift 2
        ;;
      --start-if-needed)
        START_IF_NEEDED=true
        shift
        ;;
      --persist)
        PERSIST_CONFIG=true
        shift
        ;;
      --status)
        show_status=true
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        echo "Unknown option: $1" >&2
        usage
        exit 1
        ;;
    esac
  done

  if [[ -z "$MEMORY_SWAP" ]]; then
    MEMORY_SWAP="$MEMORY"
  fi

  if [[ "$show_status" == true ]]; then
    print_status
    exit 0
  fi
}

minikube_is_running() {
  local status
  status=$(minikube status -p "$PROFILE" --format '{{.Host}}' 2>/dev/null || true)
  [[ "$status" == "Running" ]]
}

container_exists() {
  docker inspect "$PROFILE" >/dev/null 2>&1
}

start_minikube_if_needed() {
  if minikube_is_running; then
    return
  fi

  if [[ "$START_IF_NEEDED" != true ]]; then
    echo "Error: minikube profile '$PROFILE' is not running." >&2
    echo "Use --start-if-needed or start it manually first." >&2
    exit 1
  fi

  echo "Starting minikube profile '$PROFILE'..."
  minikube start -p "$PROFILE"
}

print_status() {
  if ! container_exists; then
    echo "Minikube Docker container '$PROFILE' does not exist."
    exit 1
  fi

  docker inspect "$PROFILE" \
    --format='Container={{.Name}} Memory={{.HostConfig.Memory}} MemorySwap={{.HostConfig.MemorySwap}}'

  echo "---"
  docker stats --no-stream --format 'table {{.Name}}\t{{.MemUsage}}' "$PROFILE"
}

memory_to_mb() {
  local value
  value=$(printf '%s' "$MEMORY" | tr '[:upper:]' '[:lower:]')

  case "$value" in
    *gi|*gib)
      value="${value%gi}"
      value="${value%gib}"
      printf '%s' "$((value * 1024))"
      ;;
    *g)
      value="${value%g}"
      printf '%s' "$((value * 1024))"
      ;;
    *mi|*mib)
      value="${value%mi}"
      value="${value%mib}"
      printf '%s' "$value"
      ;;
    *m)
      value="${value%m}"
      printf '%s' "$value"
      ;;
    *)
      echo "Error: unsupported --memory format for --persist: $MEMORY" >&2
      echo "Use values like 10g, 10Gi, 10240m, or 10240Mi." >&2
      exit 1
      ;;
  esac
}

persist_config() {
  local memory_mb
  memory_mb=$(memory_to_mb)

  echo "Saving memory setting for future starts..."
  minikube config set memory "$memory_mb"
}

apply_live_update() {
  if ! container_exists; then
    echo "Error: minikube Docker container '$PROFILE' does not exist." >&2
    exit 1
  fi

  echo "Updating Docker limits for '$PROFILE'..."
  docker update --memory "$MEMORY" --memory-swap "$MEMORY_SWAP" "$PROFILE" >/dev/null
}

main() {
  require_cmd docker
  require_cmd minikube

  parse_args "$@"
  start_minikube_if_needed
  apply_live_update

  if [[ "$PERSIST_CONFIG" == true ]]; then
    persist_config
  fi

  echo "Updated minikube container memory successfully."
  print_status
}

main "$@"