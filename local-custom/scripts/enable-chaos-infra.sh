#!/bin/bash

# Enable Litmus Chaos Infrastructure Script
# Applies the manifest and restarts components to pick up SERVER_ADDR.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
if [ -z "${MANIFEST:-}" ]; then
  CANDIDATE_MANIFESTS=(
    "$REPO_ROOT/local-custom/agentcert-framwork-litmus-chaos-enable.yml"
    "$REPO_ROOT/local-custom/k8s/litmus-installation.yaml"
  )
  for candidate in "${CANDIDATE_MANIFESTS[@]}"; do
    if [ -f "$candidate" ]; then
      MANIFEST="$candidate"
      break
    fi
  done
else
  MANIFEST="$MANIFEST"
fi
NAMESPACE="litmus-exp"

info() { echo "[INFO] $1"; }
warn() { echo "[WARN] $1"; }
err() { echo "[ERROR] $1"; }

if ! command -v kubectl >/dev/null 2>&1; then
  err "kubectl is not installed or not in PATH"
  exit 1
fi

if [ ! -f "$MANIFEST" ]; then
  err "Manifest not found: $MANIFEST"
  exit 1
fi

info "Applying chaos infra manifest..."
kubectl apply -f "$MANIFEST"

info "Restarting subscriber and event-tracker to pick up SERVER_ADDR..."
if kubectl get deployment -n "$NAMESPACE" subscriber >/dev/null 2>&1; then
  kubectl rollout restart deployment/subscriber -n "$NAMESPACE"
else
  warn "subscriber deployment not found in $NAMESPACE"
fi

if kubectl get deployment -n "$NAMESPACE" event-tracker >/dev/null 2>&1; then
  kubectl rollout restart deployment/event-tracker -n "$NAMESPACE"
else
  warn "event-tracker deployment not found in $NAMESPACE"
fi

info "Waiting for pods to be ready..."
sleep 10
kubectl get pods -n "$NAMESPACE"

info "Enable complete."
