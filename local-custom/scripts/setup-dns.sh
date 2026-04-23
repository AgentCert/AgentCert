#!/usr/bin/env bash
# setup-dns.sh
# ─────────────────────────────────────────────────────────────────────────────
# One-shot DNS setup for a fresh WSL Ubuntu + minikube environment.
#
# Fixes the full DNS chain:
#   Windows (current network) → WSL resolv.conf → Docker daemon → CoreDNS
#
# Works on any network (home, office, VPN, hotspot) because it reads the
# Windows NAT gateway from the routing table — the gateway ALWAYS proxies
# to whatever DNS Windows is currently using.
#
# Usage:
#   sudo bash setup-dns.sh              # full setup (WSL + Docker + minikube)
#   sudo bash setup-dns.sh --wsl-only   # skip minikube CoreDNS step
#   sudo bash setup-dns.sh --dry-run    # print what would change, no writes
#
# Re-run any time you change networks and Docker builds start failing.
# The script is also installed as /usr/local/bin/setup-dns and hooked into
# /etc/wsl.conf [boot] so WSL and Docker are fixed automatically on startup.
# (minikube CoreDNS must be re-run manually after each 'minikube start').
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

# ── Argument parsing ──────────────────────────────────────────────────────────
DRY_RUN=0
WSL_ONLY=0
for arg in "$@"; do
  case "$arg" in
    --dry-run)  DRY_RUN=1  ;;
    --wsl-only) WSL_ONLY=1 ;;
  esac
done

# ── Helpers ───────────────────────────────────────────────────────────────────
info()    { echo "[setup-dns] $*"; }
ok()      { echo "[setup-dns] OK: $*"; }
warn()    { echo "[setup-dns] WARN: $*" >&2; }
die()     { echo "[setup-dns] ERROR: $*" >&2; exit 1; }
dry_run() { [[ $DRY_RUN -eq 1 ]]; }

write_file() {
  local path="$1"; local content="$2"
  if dry_run; then
    echo "  [dry-run] would write $path:"
    echo "$content" | sed 's/^/    /'
  else
    # Interpret embedded \n escapes so generated files have real newlines.
    printf '%b\n' "$content" > "$path"
    ok "wrote $path"
  fi
}

dedupe_ipv4_list() {
  # Input: space-delimited IPv4 list. Output: space-delimited unique IPv4 list.
  echo "$1" \
    | tr ' ' '\n' \
    | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' \
    | awk '!seen[$0]++' \
    | tr '\n' ' ' \
    | sed 's/ $//'
}

# ── Step 1: Discover Windows NAT gateway + real Windows DNS servers ─────────────
info "Step 1: Discovering Windows NAT gateway and DNS servers..."
GATEWAY=$(ip route show default 2>/dev/null | awk '/default/ {print $3; exit}')
[[ -z "$GATEWAY" ]] && die "No default route found — is WSL networking up?"
info "  WSL NAT gateway: $GATEWAY"

# Read DNS from active Windows interfaces that currently have a default gateway.
# This avoids stale DNS from disconnected VPN/virtual adapters.
# Try both the interop-PATH name and the full Windows path (needed under sudo
# where WSL interop PATH entries may be stripped).
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

if [[ -n "$WIN_DNS" ]]; then
  info "  Windows DNS servers (via powershell): $WIN_DNS"
  PRIMARY_DNS=$(echo "$WIN_DNS" | awk '{print $1}')
else
  warn "  Could not read Windows DNS via powershell.exe — falling back to gateway"
  PRIMARY_DNS="$GATEWAY"
fi

# WSL resolver should prioritize active Windows DNS and keep WSL gateway fallback.
WSL_DNS_SERVERS=$(dedupe_ipv4_list "$WIN_DNS $GATEWAY")
if [[ -z "$WSL_DNS_SERVERS" ]]; then
  WSL_DNS_SERVERS="$GATEWAY"
fi

# Docker-in-WSL resolver chain should start with WSL gateway (reachable from containers),
# then host DNS, then public resolvers for resilience.
DOCKER_DNS_SERVERS=$(dedupe_ipv4_list "$GATEWAY $WIN_DNS 1.1.1.1 8.8.8.8")
if [[ -z "$DOCKER_DNS_SERVERS" ]]; then
  DOCKER_DNS_SERVERS="$GATEWAY 1.1.1.1 8.8.8.8"
fi

info "  Primary DNS: $PRIMARY_DNS"
info "  WSL DNS chain: $WSL_DNS_SERVERS"
info "  Docker DNS chain: $DOCKER_DNS_SERVERS"

# ── Step 2: WSL resolv.conf ───────────────────────────────────────────────────
info "Step 2: Fixing WSL /etc/resolv.conf..."

if ! dry_run; then
  # Break the systemd-resolved symlink so we can write a static file.
  if [[ -L /etc/resolv.conf ]]; then
    rm -f /etc/resolv.conf
    info "  Removed systemd-resolved symlink"
  fi
fi

# Build resolv.conf with WSL DNS chain (host DNS + gateway fallback).
RESOLV_CONTENT="# Managed by setup-dns — do not edit manually."
for srv in $WSL_DNS_SERVERS; do
  RESOLV_CONTENT="${RESOLV_CONTENT}\nnameserver $srv"
done
RESOLV_CONTENT="${RESOLV_CONTENT}\noptions edns0"
write_file /etc/resolv.conf "$RESOLV_CONTENT"

# ── Step 3: Docker daemon DNS ─────────────────────────────────────────────────
info "Step 3: Fixing Docker daemon DNS..."
mkdir -p /etc/docker

# Build daemon.json with Docker DNS chain as a JSON array.
DNS_JSON_ARRAY=$(echo "$DOCKER_DNS_SERVERS" | tr ' ' '\n' | awk '{printf "%s\"%s\"", (NR>1?", ":"["), $0} END{print "]"}')
write_file /etc/docker/daemon.json "{
  \"dns\": $DNS_JSON_ARRAY
}"

if ! dry_run; then
  if systemctl is-active --quiet docker 2>/dev/null; then
    systemctl restart docker
    ok "Docker restarted"
  else
    warn "Docker not running — daemon.json written, will take effect on next start"
  fi
fi

# ── Step 4: wsl.conf boot hook ────────────────────────────────────────────────
info "Step 4: Installing boot hook in /etc/wsl.conf..."

# Copy this script to a stable system path so wsl.conf can call it by name.
SCRIPT_PATH="/usr/local/bin/setup-dns"
if ! dry_run; then
  cp "$0" "$SCRIPT_PATH"
  chmod +x "$SCRIPT_PATH"
  ok "Installed $SCRIPT_PATH"
fi

# Add [boot] command only if not already present.
if grep -q 'setup-dns' /etc/wsl.conf 2>/dev/null; then
  info "  /etc/wsl.conf already has setup-dns boot hook — skipping"
else
  if dry_run; then
    echo "  [dry-run] would add 'command = setup-dns --wsl-only' under [boot] in /etc/wsl.conf"
  else
    # Insert 'command = setup-dns --wsl-only' on the line after [boot].
    # --wsl-only because minikube may not be running at boot time.
    if grep -q '^\[boot\]' /etc/wsl.conf; then
      sed -i '/^\[boot\]/a command = setup-dns --wsl-only' /etc/wsl.conf
    else
      printf '\n[boot]\nsystemd=true\ncommand = setup-dns --wsl-only\n' >> /etc/wsl.conf
    fi
    ok "Added boot hook to /etc/wsl.conf"
  fi
fi

# ── Step 5: minikube CoreDNS ──────────────────────────────────────────────────
if [[ $WSL_ONLY -eq 1 ]]; then
  info "Step 5: Skipping minikube CoreDNS (--wsl-only)"
else
  info "Step 5: Patching minikube CoreDNS to use $WSL_DNS_SERVERS..."

  if ! command -v kubectl &>/dev/null; then
    warn "kubectl not found — skipping CoreDNS patch (run again after minikube start)"
  else
    CM_CHECK=$(kubectl get configmap coredns -n kube-system --ignore-not-found 2>/dev/null)
    if [[ -z "$CM_CHECK" ]]; then
      warn "CoreDNS configmap not found — is minikube running? Skipping."
    else
      # Build a clean Corefile using block format for the forward directive.
      # The minikube default uses the old inline style which CoreDNS v1.12 rejects.
      MINIKUBE_HOST=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || echo "192.168.49.1")
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
    forward . ${WSL_DNS_SERVERS} {
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

      info "  New CoreDNS forward: forward . $WSL_DNS_SERVERS"

      if dry_run; then
        echo "  [dry-run] would patch CoreDNS configmap and restart coredns deployment"
      else
        PATCH_JSON=$(python3 -c "
import json, sys
print(json.dumps({'data': {'Corefile': sys.argv[1]}}))
" "$NEW_COREFILE")
        kubectl patch configmap coredns -n kube-system --type merge -p "$PATCH_JSON"
        kubectl rollout restart deployment/coredns -n kube-system
        kubectl rollout status deployment/coredns -n kube-system --timeout=60s
        ok "CoreDNS patched and restarted"
      fi
    fi
  fi
fi

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "─────────────────────────────────────────────────────"
echo "[setup-dns] DNS chain configured:"
echo "  Windows DNS      : $WIN_DNS"
echo "  WSL gateway      : $GATEWAY"
echo "  WSL DNS chain    : $WSL_DNS_SERVERS"
echo "  Docker DNS chain : $DOCKER_DNS_SERVERS"
echo "  /etc/resolv.conf : nameserver(s) $WSL_DNS_SERVERS"
echo "  /etc/docker/daemon.json : dns $DOCKER_DNS_SERVERS"
if [[ $WSL_ONLY -eq 0 ]]; then
echo "  CoreDNS forward  : $WSL_DNS_SERVERS"
fi
echo "  Boot hook        : /etc/wsl.conf [boot] command = setup-dns --wsl-only"
echo ""
echo "  To re-apply after a network change:"
echo "    sudo setup-dns             # WSL + Docker + minikube CoreDNS"
echo "    sudo setup-dns --wsl-only  # WSL + Docker only (minikube not needed)"
echo "─────────────────────────────────────────────────────"
