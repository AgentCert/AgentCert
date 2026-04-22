#!/usr/bin/env bash
# fix-minikube-dns.sh
# Patches CoreDNS in minikube to use Windows/WSL-provided upstream DNS servers.
# Reads DNS servers dynamically from WSL -- no hardcoded values.
# Usage: bash fix-minikube-dns.sh [--dry-run]
set -euo pipefail

DRY_RUN=0
for arg in "$@"; do
  [[ "$arg" == "--dry-run" ]] && DRY_RUN=1
done

# ── Discover upstream DNS servers from WSL ────────────────────────────────────
discover_dns() {
  local dns_ips=""

  # Prefer the real upstream resolv.conf (bypasses the 127.0.0.53 stub)
  if [[ -f /run/systemd/resolve/resolv.conf ]]; then
    dns_ips=$(grep -E '^nameserver' /run/systemd/resolve/resolv.conf \
              | awk '{print $2}' \
              | grep -v '^127\.' \
              | tr '\n' ' ' \
              | sed 's/ $//')
  fi

  # Fallback: resolvectl status
  if [[ -z "$dns_ips" ]] && command -v resolvectl &>/dev/null; then
    dns_ips=$(resolvectl status 2>/dev/null \
              | grep -E 'DNS Servers:' \
              | awk '{for(i=3;i<=NF;i++) printf $i" "}' \
              | grep -v '^127\.' \
              | sed 's/ $//')
  fi

  if [[ -z "$dns_ips" ]]; then
    echo "ERROR: Could not discover upstream DNS servers." >&2
    exit 1
  fi

  echo "$dns_ips"
}

# ── Main ──────────────────────────────────────────────────────────────────────
echo "==> Discovering DNS servers from WSL..."
DNS_SERVERS=$(discover_dns)
echo "    Found: $DNS_SERVERS"

echo "==> Reading current CoreDNS Corefile..."
COREFILE=$(kubectl get configmap coredns -n kube-system \
           -o jsonpath='{.data.Corefile}' 2>/dev/null)
if [[ -z "$COREFILE" ]]; then
  echo "ERROR: Could not read CoreDNS configmap." >&2
  exit 1
fi

echo "--- Current Corefile ---"
echo "$COREFILE"
echo "------------------------"

# Build replacement forward line preserving any existing block options
OLD_FWD=$(echo "$COREFILE" | grep 'forward \.' | head -1 | sed 's/[[:space:]]*$//')
NEW_FWD="forward . ${DNS_SERVERS}"

# Reconstruct: replace the opening 'forward .' token; keep { ... } block intact
# Strategy: replace everything up to (but not including) a '{' or end-of-line
NEW_COREFILE=$(echo "$COREFILE" | python3 -c "
import sys, re
content = sys.stdin.read()
# Replace 'forward . <anything_not_{>' with new servers
content = re.sub(r'forward \.[^{\n]*', 'forward . ${DNS_SERVERS} ', content)
print(content, end='')
")

echo "--- New Corefile ---"
echo "$NEW_COREFILE"
echo "--------------------"

if [[ $DRY_RUN -eq 1 ]]; then
  echo "==> [DRY-RUN] Would patch CoreDNS with the above Corefile. Exiting."
  exit 0
fi

echo "==> Patching CoreDNS configmap..."
PATCH_JSON=$(python3 -c "
import json, sys
print(json.dumps({'data': {'Corefile': sys.stdin.read()}}))
" <<< "$NEW_COREFILE")

kubectl patch configmap coredns -n kube-system --type merge -p "$PATCH_JSON"

echo "==> Restarting CoreDNS pods..."
kubectl rollout restart deployment/coredns -n kube-system
kubectl rollout status deployment/coredns -n kube-system --timeout=60s

echo "==> Verifying DNS resolution from inside cluster..."
kubectl run dns-test --image=busybox:1.28 --restart=Never --rm -it \
  --command -- nslookup google.com 2>/dev/null || \
  echo "WARN: nslookup test pod failed (may already be clean)"

echo "==> Done. CoreDNS is now using: $DNS_SERVERS"
