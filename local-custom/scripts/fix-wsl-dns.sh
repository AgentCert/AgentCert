#!/usr/bin/env bash
# fix-wsl-dns — runs at WSL boot and on-demand.
# Reads the actual Windows DNS servers via powershell.exe and uses them
# directly in WSL resolv.conf and Docker daemon.json.
# Falls back to the WSL NAT gateway if powershell.exe is unavailable.
# Usage: sudo bash fix-wsl-dns.sh
set -euo pipefail

GATEWAY=$(ip route show default 2>/dev/null | awk '/default/ {print $3; exit}')

if [[ -z "$GATEWAY" ]]; then
  echo "[fix-wsl-dns] ERROR: no default route found, skipping DNS fix" >&2
  exit 0
fi

echo "[fix-wsl-dns] WSL gateway: $GATEWAY"

# Read real Windows DNS servers — these are what Windows actually uses
# (from DHCP or manual config). On corporate networks this is the corporate
# resolver (e.g. 10.50.53.54), NOT the NAT gateway which is just a virtual NIC.
# Try both the interop-PATH name and the full path (needed under sudo).
_PS_CMD="Get-DnsClientServerAddress -AddressFamily IPv4 | Select-Object -ExpandProperty ServerAddresses | Sort-Object -Unique"
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
  echo "[fix-wsl-dns] Windows DNS servers: $WIN_DNS"
  ALL_DNS="$WIN_DNS"
else
  echo "[fix-wsl-dns] WARN: powershell.exe DNS read failed — using gateway $GATEWAY"
  ALL_DNS="$GATEWAY"
fi

# ── 1. WSL resolv.conf ───────────────────────────────────────────────────────
if [[ -L /etc/resolv.conf ]]; then
  rm -f /etc/resolv.conf
fi
{
  echo "# Managed by fix-wsl-dns — do not edit manually."
  for srv in $ALL_DNS; do
    echo "nameserver $srv"
  done
  echo "options edns0"
} > /etc/resolv.conf
echo "[fix-wsl-dns] /etc/resolv.conf -> nameserver(s): $ALL_DNS"

# ── 2. Docker daemon DNS ─────────────────────────────────────────────────────
mkdir -p /etc/docker
DNS_JSON=$(echo "$ALL_DNS" | tr ' ' '\n' | awk '{printf "%s\"%s\"", (NR>1?", ":"["), $0} END{print "]"}')
cat > /etc/docker/daemon.json <<DOCKER
{
  "dns": $DNS_JSON
}
DOCKER
echo "[fix-wsl-dns] /etc/docker/daemon.json -> dns: $DNS_JSON"

# ── 3. Restart Docker to pick up new daemon.json ─────────────────────────────
if systemctl is-active --quiet docker 2>/dev/null; then
  systemctl restart docker
  echo "[fix-wsl-dns] Docker restarted"
fi

echo "[fix-wsl-dns] Done — DNS now follows Windows network dynamically"
