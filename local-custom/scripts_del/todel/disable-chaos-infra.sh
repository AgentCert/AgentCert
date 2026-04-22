#!/bin/bash

# Disable Litmus Chaos Infrastructure Cleanup Script
# This script performs the cleanup steps listed in the UI prompt.
# It assumes you already downloaded the manifest for your infra and
# saved it as agent-cert-choas-litmus-chaos-enable.yml in the current directory.

set -euo pipefail

NAMESPACE="litmus-exp"
INFRA_MANIFEST="agent-cert-choas-litmus-chaos-enable.yml"
CRD_URL="https://raw.githubusercontent.com/litmuschaos/litmus/master/mkdocs/docs/3.21.0/litmus-portal-crds.yml"

info() { echo "[INFO] $1"; }
warn() { echo "[WARN] $1"; }
err() { echo "[ERROR] $1"; }

if ! command -v kubectl >/dev/null 2>&1; then
  err "kubectl is not installed or not in PATH"
  exit 1
fi

info "Step 1: Deleting chaos experiments, engines, results in namespace: $NAMESPACE"
if kubectl get ns "$NAMESPACE" >/dev/null 2>&1; then
  kubectl delete chaosexperiment,chaosengine,chaosresult --all -n "$NAMESPACE" || true
else
  warn "Namespace $NAMESPACE not found. Skipping Step 1."
fi

info "Step 2: Deleting Litmus CRDs (skip if you have another infra on this cluster)"
kubectl delete -f "$CRD_URL" || true

info "Step 3: Deleting remaining components from infra manifest"
if [ -f "$INFRA_MANIFEST" ]; then
  kubectl delete -f "$INFRA_MANIFEST" || true
else
  warn "Infra manifest not found: $INFRA_MANIFEST"
  warn "Place the downloaded manifest in the current directory or set INFRA_MANIFEST path."
fi

info "Step 4: Deleting namespace: $NAMESPACE"
if kubectl get ns "$NAMESPACE" >/dev/null 2>&1; then
  kubectl delete ns "$NAMESPACE" || true
else
  warn "Namespace $NAMESPACE not found. Skipping Step 4."
fi

info "Cleanup complete."
