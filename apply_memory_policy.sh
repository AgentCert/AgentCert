#!/usr/bin/env bash
set -euo pipefail

# Applies/removes a safe per-container memory policy using LimitRange.
# Allows limits up to 10Gi without forcing every pod to request high memory.
# Can apply to single namespaces or all namespaces dynamically.

NAMESPACE=""
POLICY_NAME="container-memory-policy"
MAX_MEMORY="10Gi"
MIN_MEMORY="128Mi"
DEFAULT_MEMORY="1Gi"
DEFAULT_REQUEST="256Mi"
MAX_RATIO="40"
ACTION="apply"
APPLY_ALL_NAMESPACES=false

usage() {
  cat <<'EOF'
Usage:
  ./apply_memory_policy.sh [options]

Actions:
  --apply                 Apply or update the LimitRange policy (default)
  --delete                Delete the LimitRange policy
  --status                Show current LimitRange policy

Options:
  -n, --namespace <ns>    Target a specific namespace (required unless using --all)
  --all                   Apply policy to all existing namespaces
  --name <name>           LimitRange name (default: container-memory-policy)
  --max-memory <value>    Container max memory (default: 10Gi)
  --min-memory <value>    Container min memory (default: 128Mi)
  --default-memory <val>  Default container limit (default: 1Gi)
  --default-request <val> Default container request (default: 256Mi)
  --max-ratio <value>     maxLimitRequestRatio.memory (default: 40)
  -h, --help              Show help

Examples:
  ./apply_memory_policy.sh -n litmus-exp
  ./apply_memory_policy.sh --all
  ./apply_memory_policy.sh --all --status
  ./apply_memory_policy.sh -n litmus-exp --delete
  ./apply_memory_policy.sh --all --max-memory 10Gi
EOF
}

require_kubectl() {
  if ! command -v kubectl >/dev/null 2>&1; then
    echo "Error: kubectl is not installed or not in PATH." >&2
    exit 1
  fi
}

get_all_namespaces() {
  kubectl get ns -o jsonpath='{.items[*].metadata.name}'
}

apply_policy_to_namespace() {
  local ns="$1"
  kubectl apply -f - <<EOF
apiVersion: v1
kind: LimitRange
metadata:
  name: ${POLICY_NAME}
  namespace: ${ns}
spec:
  limits:
  - type: Container
    max:
      memory: ${MAX_MEMORY}
    min:
      memory: ${MIN_MEMORY}
    default:
      memory: ${DEFAULT_MEMORY}
    defaultRequest:
      memory: ${DEFAULT_REQUEST}
    maxLimitRequestRatio:
      memory: "${MAX_RATIO}"
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --apply)
        ACTION="apply"
        shift
        ;;
      --delete)
        ACTION="delete"
        shift
        ;;
      --status)
        ACTION="status"
        shift
        ;;
      --all)
        APPLY_ALL_NAMESPACES=true
        shift
        ;;
      -n|--namespace)
        NAMESPACE="${2:-}"
        shift 2
        ;;
      --name)
        POLICY_NAME="${2:-}"
        shift 2
        ;;
      --max-memory)
        MAX_MEMORY="${2:-}"
        shift 2
        ;;
      --min-memory)
        MIN_MEMORY="${2:-}"
        shift 2
        ;;
      --default-memory)
        DEFAULT_MEMORY="${2:-}"
        shift 2
        ;;
      --default-request)
        DEFAULT_REQUEST="${2:-}"
        shift 2
        ;;
      --max-ratio)
        MAX_RATIO="${2:-}"
        shift 2
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
}

apply_policy() {
  if [[ "$APPLY_ALL_NAMESPACES" == true ]]; then
    echo "Applying policy to all namespaces..."
    local ns_list
    ns_list=$(get_all_namespaces)
    for ns in $ns_list; do
      echo "  → Applying to namespace: $ns"
      apply_policy_to_namespace "$ns"
    done
    echo "Policy applied to all namespaces."
  else
    if [[ -z "$NAMESPACE" ]]; then
      echo "Error: Either specify -n/--namespace or use --all" >&2
      usage
      exit 1
    fi
    echo "Applying policy to namespace: $NAMESPACE"
    apply_policy_to_namespace "$NAMESPACE"
    echo "Applied policy ${POLICY_NAME} in namespace ${NAMESPACE}."
  fi
}

delete_policy() {
  if [[ "$APPLY_ALL_NAMESPACES" == true ]]; then
    echo "Deleting policy from all namespaces..."
    local ns_list
    ns_list=$(get_all_namespaces)
    for ns in $ns_list; do
      echo "  → Deleting from namespace: $ns"
      kubectl delete limitrange "${POLICY_NAME}" -n "$ns" --ignore-not-found
    done
    echo "Policy deleted from all namespaces."
  else
    if [[ -z "$NAMESPACE" ]]; then
      echo "Error: Either specify -n/--namespace or use --all" >&2
      usage
      exit 1
    fi
    kubectl delete limitrange "${POLICY_NAME}" -n "${NAMESPACE}" --ignore-not-found
    echo "Policy deleted from namespace: ${NAMESPACE}"
  fi
}

show_status() {
  if [[ "$APPLY_ALL_NAMESPACES" == true ]]; then
    echo "Showing policy in all namespaces..."
    local ns_list
    ns_list=$(get_all_namespaces)
    for ns in $ns_list; do
      echo "=== Namespace: $ns ==="
      kubectl get limitrange "${POLICY_NAME}" -n "$ns" -o yaml 2>/dev/null || echo "  (not found)"
      echo
    done
  else
    if [[ -z "$NAMESPACE" ]]; then
      echo "Error: Either specify -n/--namespace or use --all" >&2
      usage
      exit 1
    fi
    kubectl get limitrange "${POLICY_NAME}" -n "${NAMESPACE}" -o yaml
  fi
}

main() {
  parse_args "$@"
  require_kubectl

  case "$ACTION" in
    apply)
      apply_policy
      ;;
    delete)
      delete_policy
      ;;
    status)
      show_status
      ;;
    *)
      echo "Unsupported action: $ACTION" >&2
      exit 1
      ;;
  esac
}

main "$@"
