#!/usr/bin/env bash
# Patch CoreDNS to use real Windows DNS servers directly.
# DNS servers are read dynamically from the active Windows network adapter —
# no hardcoded IPs so this works on any machine/network.
set -euo pipefail

# Read DNS from active Windows interfaces (the ones with a default gateway).
_PS_CMD="Get-NetIPConfiguration | Where-Object { \$_.NetAdapter.Status -eq 'Up' -and \$_.IPv4DefaultGateway -ne \$null } | ForEach-Object { \$_.DNSServer.ServerAddresses } | Sort-Object -Unique"
_PS_FULL="/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
_ps_bin=""
command -v powershell.exe &>/dev/null && _ps_bin="powershell.exe"
[[ -z "$_ps_bin" && -x "$_PS_FULL" ]] && _ps_bin="$_PS_FULL"

WIN_DNS=""
if [[ -n "$_ps_bin" ]]; then
  WIN_DNS=$("$_ps_bin" -NoProfile -Command "$_PS_CMD" 2>/dev/null \
    | tr -d '\r' \
    | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' \
    | grep -vE '^(127\.|169\.254\.|0\.0\.0\.0)' \
    | head -4 \
    | tr '\n' ' ' | sed 's/ $//' || true)
fi

GATEWAY=$(ip route show default 2>/dev/null | awk '/default/ {print $3; exit}')
DNS_SERVERS="${WIN_DNS:-${GATEWAY}}"
[[ -z "$DNS_SERVERS" ]] && { echo "[ERROR] Could not determine DNS servers"; exit 1; }
echo "Using DNS servers: $DNS_SERVERS"

# Read the minikube host IP for the hosts block.
MINIKUBE_HOST=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || echo "192.168.49.1")

# Write a clean Corefile using block format for the forward directive.
# The annotation-stored version used inline format which CoreDNS v1.12 rejects.
NEW_COREFILE=".:53 {
    log
    errors
    health {
       lameduck 5s
    }
    ready
    kubernetes cluster.local in-addr.arpa ip6.arpa {
       pods insecure
       fallthrough in-addr.arpa ip6.arpa
       ttl 30
    }
    prometheus :9153
    hosts {
       ${MINIKUBE_HOST} host.minikube.internal
       fallthrough
    }
    forward . ${DNS_SERVERS} {
       max_concurrent 1000
    }
    cache 30 {
       disable success cluster.local
       disable denial cluster.local
    }
    loop
    reload
    loadbalance
}"

echo "New Corefile forward line: $(echo "$NEW_COREFILE" | grep 'forward \.')"

PATCH=$(python3 -c "
import json, sys
corefile = sys.argv[1]
print(json.dumps({'data': {'Corefile': corefile}}))
" "$NEW_COREFILE")

kubectl patch configmap coredns -n kube-system --type merge -p "$PATCH"
kubectl rollout restart deployment/coredns -n kube-system
kubectl rollout status deployment/coredns -n kube-system --timeout=60s
echo "CoreDNS patched — DNS: $DNS_SERVERS"


