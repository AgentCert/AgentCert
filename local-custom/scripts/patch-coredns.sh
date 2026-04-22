#!/usr/bin/env bash
# Patches minikube CoreDNS to use the Windows NAT gateway as upstream DNS.
# Run as: bash patch-coredns.sh
set -euo pipefail

GATEWAY=$(ip route show default | awk '/default/ {print $3; exit}')
echo "[coredns] Gateway: $GATEWAY"

COREFILE=$(kubectl get configmap coredns -n kube-system -o jsonpath='{.data.Corefile}')

NEW_COREFILE=$(echo "$COREFILE" | python3 -c "
import sys, re
content = sys.stdin.read()
content = re.sub(r'forward \.[^{\n]*', 'forward . $GATEWAY ', content)
print(content, end='')
")

echo "[coredns] New forward line: $(echo "$NEW_COREFILE" | grep 'forward \.' | head -1 | sed 's/^[[:space:]]*//')"

PATCH_JSON=$(python3 -c "
import json, sys
print(json.dumps({'data': {'Corefile': sys.stdin.read()}}))
" <<< "$NEW_COREFILE")

kubectl patch configmap coredns -n kube-system --type merge -p "$PATCH_JSON"
kubectl rollout restart deployment/coredns -n kube-system
kubectl rollout status deployment/coredns -n kube-system --timeout=60s
echo "[coredns] Done — CoreDNS now uses: $GATEWAY"
