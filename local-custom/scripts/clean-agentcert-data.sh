#!/usr/bin/env bash
set -euo pipefail

# Clean AgentCert data (agents, experiments, runs, probes) from MongoDB
# Clean Litmus workflow resources from Kubernetes
# Usage:
#   ./clean-agentcert-data.sh [MONGO_URI]
# or set MONGO_URI env var.

DEFAULT_URI="mongodb://root:1234@localhost:27017/admin"
MONGO_URI="${1:-${MONGO_URI:-$DEFAULT_URI}}"
LITMUS_EXP_NAMESPACE="${LITMUS_EXP_NAMESPACE:-litmus-exp}"
APP_CLEANUP_NAMESPACES="${APP_CLEANUP_NAMESPACES:-sock-shop loadtest monitoring agentcert-system}"

if ! command -v mongosh >/dev/null 2>&1; then
  echo "mongosh not found. Please install MongoDB Shell (mongosh)." >&2
  exit 1
fi

echo "Using MongoDB URI: $MONGO_URI"

mongosh "$MONGO_URI" --quiet --eval "
  db = db.getSiblingDB('litmus');
  print('Cleaning collections in db: ' + db.getName());
  const results = {
    agentRegistry: db.agentRegistry.deleteMany({}).deletedCount,
    chaosExperiments: db.chaosExperiments.deleteMany({}).deletedCount,
    chaosExperimentRuns: db.chaosExperimentRuns.deleteMany({}).deletedCount,
    chaosProbes: db.chaosProbes.deleteMany({}).deletedCount,
    // Cached chaos-hub charts pulled from git. Wiping forces ChaosCenter to
    // re-clone from the configured git URL on next sync, so edits committed
    // to chaos-charts (e.g. STATUS_CHECK_TIMEOUTS in fault.yaml) are picked up.
    chaosHubs: db.chaosHubs.deleteMany({}).deletedCount
  };
  printjson(results);
"

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl not found. Skipping Kubernetes workflow cleanup."
  exit 0
fi

if ! kubectl get namespace "$LITMUS_EXP_NAMESPACE" >/dev/null 2>&1; then
  echo "Namespace '$LITMUS_EXP_NAMESPACE' not found. Skipping Kubernetes workflow cleanup."
  exit 0
fi

echo "Cleaning workflow resources in namespace: $LITMUS_EXP_NAMESPACE"

kubectl delete workflows.argoproj.io --all -n "$LITMUS_EXP_NAMESPACE" --ignore-not-found --wait=false >/dev/null 2>&1 || true
kubectl delete jobs.batch --all -n "$LITMUS_EXP_NAMESPACE" --ignore-not-found --wait=false >/dev/null 2>&1 || true
kubectl delete pods --all -n "$LITMUS_EXP_NAMESPACE" --ignore-not-found --wait=false >/dev/null 2>&1 || true
kubectl delete chaosengines.litmuschaos.io --all -n "$LITMUS_EXP_NAMESPACE" --ignore-not-found --wait=false >/dev/null 2>&1 || true
kubectl delete chaosresults.litmuschaos.io --all -n "$LITMUS_EXP_NAMESPACE" --ignore-not-found --wait=false >/dev/null 2>&1 || true
# ChaosExperiment CRDs themselves carry the fault env (STATUS_CHECK_TIMEOUTS,
# TOTAL_CHAOS_DURATION, etc.). Without deleting them, edits in chaos-charts
# fault.yaml never reach the cluster — ChaosCenter only re-renders them on a
# fresh experiment when the CRD is missing.
kubectl delete chaosexperiments.litmuschaos.io --all -n "$LITMUS_EXP_NAMESPACE" --ignore-not-found --wait=false >/dev/null 2>&1 || true

echo "Kubernetes workflow cleanup triggered for namespace: $LITMUS_EXP_NAMESPACE"

for namespace in $APP_CLEANUP_NAMESPACES; do
  if kubectl get namespace "$namespace" >/dev/null 2>&1; then
    echo "Deleting app namespace: $namespace"
    kubectl delete namespace "$namespace" --ignore-not-found --wait=false >/dev/null 2>&1 || true
  else
    echo "App namespace '$namespace' not found. Skipping."
  fi
done
