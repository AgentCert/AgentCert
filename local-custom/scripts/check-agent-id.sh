#!/usr/bin/env bash
set -euo pipefail
FPOD=$(kubectl get pod -n litmus-chaos -l app=flash-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [[ -z "$FPOD" ]]; then
  echo "No flash-agent pod found"
  exit 1
fi
echo "Pod: $FPOD"
echo "--- Image ---"
kubectl get pod -n litmus-chaos "$FPOD" -o jsonpath='{.spec.containers[?(@.name=="flash-agent")].image}' && echo ""
echo "--- AGENT_ID env in flash-agent container ---"
kubectl exec -n litmus-chaos "$FPOD" -c flash-agent -- env 2>/dev/null | grep AGENT || echo "(not set)"
echo "--- AGENT_ID file in sidecar ConfigMap mount ---"
kubectl exec -n litmus-chaos "$FPOD" -c agent-sidecar -- cat /etc/agent/metadata/AGENT_ID 2>/dev/null || echo "(file missing)"
echo "--- All files in ConfigMap mount ---"
kubectl exec -n litmus-chaos "$FPOD" -c agent-sidecar -- ls /etc/agent/metadata/ 2>/dev/null
