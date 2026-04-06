#!/bin/bash
# ============================================================================
# AgentCert Graceful Shutdown Script (Linux / Azure VM)
# ============================================================================
# Stops all AgentCert services started by start-agentcert.sh
#
# Usage:
#   ./stop-agentcert.sh
#   ./stop-agentcert.sh --keep-mongo
# ============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KEEP_MONGO=false

for arg in "$@"; do
    case "$arg" in
        --keep-mongo) KEEP_MONGO=true ;;
    esac
done

status()  { echo -e "\033[36m[STATUS]\033[0m $1"; }
ok()      { echo -e "\033[32m[  OK  ]\033[0m $1"; }
skip()    { echo -e "\033[90m[ SKIP ]\033[0m $1"; }
wait_msg(){ echo -e "\033[33m[WAIT  ]\033[0m $1"; }

echo ""
echo -e "\033[35m============================================\033[0m"
echo -e "\033[35m       AgentCert Shutdown Script (Linux)   \033[0m"
echo -e "\033[35m============================================\033[0m"
echo ""

stop_service() {
    local name="$1"
    local pidfile="$2"

    if [ ! -f "$pidfile" ]; then
        skip "$name: No PID file found"
        return
    fi

    local pid
    pid=$(cat "$pidfile")

    if ! kill -0 "$pid" 2>/dev/null; then
        skip "$name: Process not running (PID: $pid)"
        rm -f "$pidfile"
        return
    fi

    wait_msg "Stopping $name (PID: $pid)..."
    kill "$pid" 2>/dev/null || true
    
    # Wait up to 10 seconds for graceful shutdown
    local waited=0
    while kill -0 "$pid" 2>/dev/null && [ $waited -lt 10 ]; do
        sleep 1
        waited=$((waited + 1))
    done

    if kill -0 "$pid" 2>/dev/null; then
        kill -9 "$pid" 2>/dev/null || true
        sleep 1
    fi

    ok "$name stopped"
    rm -f "$pidfile"
}

# Stop services in reverse order
stop_service "Frontend"       "$SCRIPT_DIR/.agentcert-frontend.pid"
stop_service "GraphQL Server" "$SCRIPT_DIR/.agentcert-graphql.pid"
stop_service "Auth Service"   "$SCRIPT_DIR/.agentcert-auth.pid"

# Clean up log files
rm -f "$SCRIPT_DIR/.auth.log" "$SCRIPT_DIR/.graphql.log" "$SCRIPT_DIR/.frontend.log" 2>/dev/null

# Optionally stop MongoDB
if [ "$KEEP_MONGO" = false ]; then
    status "Checking MongoDB container..."
    if docker ps --filter "name=m3" --format "{{.Names}}" 2>/dev/null | grep -q "m3"; then
        wait_msg "Stopping MongoDB container 'm3'..."
        docker stop m3 > /dev/null 2>&1 || true
        ok "MongoDB stopped"
    else
        skip "MongoDB container 'm3' not running"
    fi
else
    skip "MongoDB: --keep-mongo specified"
fi

echo ""
echo -e "\033[32m============================================\033[0m"
echo -e "\033[32m       AgentCert Stopped                   \033[0m"
echo -e "\033[32m============================================\033[0m"
echo ""
