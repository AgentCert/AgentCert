#!/usr/bin/env bash
set -euo pipefail

# Tune readiness probes dynamically for any app namespace.
# It lowers only overly-large readiness initial delays.
# Usage:
#   ./tune-sockshop-readiness.sh [namespace]
# Environment overrides:
#   READINESS_MAX_INITIAL_DELAY=120  # patch only when current delay is above this
#   READINESS_TARGET_INITIAL_DELAY=45
#   READINESS_PERIOD_SECONDS=5
#   READINESS_TIMEOUT_SECONDS=2

NS="${1:-sock-shop}"
READINESS_MAX_INITIAL_DELAY="${READINESS_MAX_INITIAL_DELAY:-120}"
READINESS_TARGET_INITIAL_DELAY="${READINESS_TARGET_INITIAL_DELAY:-45}"
READINESS_PERIOD_SECONDS="${READINESS_PERIOD_SECONDS:-5}"
READINESS_TIMEOUT_SECONDS="${READINESS_TIMEOUT_SECONDS:-2}"

if ! kubectl get ns "$NS" >/dev/null 2>&1; then
  echo "Namespace '$NS' does not exist"
  exit 1
fi

patched=0
skipped=0

for deploy in $(kubectl get deploy -n "$NS" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}'); do
  container_name="$(kubectl get deploy -n "$NS" "$deploy" -o jsonpath='{.spec.template.spec.containers[0].name}')"
  current_delay="$(kubectl get deploy -n "$NS" "$deploy" -o jsonpath='{.spec.template.spec.containers[0].readinessProbe.initialDelaySeconds}')"

  if [[ -z "$current_delay" || "$current_delay" == "" ]]; then
    skipped=$((skipped + 1))
    continue
  fi

  if [[ "$current_delay" =~ ^[0-9]+$ ]] && (( current_delay > READINESS_MAX_INITIAL_DELAY )); then
    kubectl patch deployment "$deploy" -n "$NS" --type='merge' -p "{
      \"spec\": {
        \"template\": {
          \"spec\": {
            \"containers\": [
              {
                \"name\": \"$container_name\",
                \"readinessProbe\": {
                  \"initialDelaySeconds\": $READINESS_TARGET_INITIAL_DELAY,
                  \"periodSeconds\": $READINESS_PERIOD_SECONDS,
                  \"timeoutSeconds\": $READINESS_TIMEOUT_SECONDS
                }
              }
            ]
          }
        }
      }
    }" >/dev/null
    echo "Patched deployment/$deploy readiness initialDelay ${current_delay}s -> ${READINESS_TARGET_INITIAL_DELAY}s"
    patched=$((patched + 1))
  else
    skipped=$((skipped + 1))
  fi
done

kubectl get deploy -n "$NS" -o custom-columns=NAME:.metadata.name,READY:.status.readyReplicas,AVAILABLE:.status.availableReplicas
echo "Readiness tuning complete for namespace '$NS' (patched=${patched}, skipped=${skipped})."
