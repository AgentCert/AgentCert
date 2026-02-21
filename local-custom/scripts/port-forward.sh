#!/bin/bash

# AgentCert Port-Forward Helper Script
# Forwards frontend service to localhost for UI access
# Internal pod-to-pod communication uses Kubernetes service DNS (no port-forwards needed)

NAMESPACE="litmus-chaos"
GRAPHQL_PORT=${GRAPHQL_PORT:-8080}   # local port for /api proxy
AUTH_PORT=${AUTH_PORT:-3030}         # local port for /auth proxy

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper function to print colored output
print_info() {
  echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
  echo -e "${GREEN}[OK]${NC} $1"
}

print_warning() {
  echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

# Check if kubectl is available
# Try multiple common locations for kubectl
KUBECTL_CMD=""
for kubectl_path in /usr/local/bin/kubectl /usr/bin/kubectl kubectl; do
  if [ -x "$kubectl_path" ] || command -v "$kubectl_path" &> /dev/null 2>&1; then
    KUBECTL_CMD="$kubectl_path"
    break
  fi
done

if [ -z "$KUBECTL_CMD" ]; then
  print_error "kubectl is not installed or not in PATH"
  print_error "Checked: /usr/local/bin/kubectl, /usr/bin/kubectl, or kubectl in PATH"
  exit 1
fi

# Check if namespace exists
if ! $KUBECTL_CMD get namespace "$NAMESPACE" &> /dev/null; then
  print_error "Namespace '$NAMESPACE' not found"
  exit 1
fi

# Auto-detect backend services
print_info "Auto-detecting services in namespace '$NAMESPACE'..."

# Find GraphQL/Server service (look for 'server' in name, but NOT 'auth')
GRAPHQL_SERVICE=$($KUBECTL_CMD get svc -n "$NAMESPACE" --no-headers -o custom-columns=NAME:.metadata.name | grep -E "server|graphql" | grep -v "auth" | head -1)
if [ -z "$GRAPHQL_SERVICE" ]; then
  print_error "Could not find GraphQL/Server service in namespace '$NAMESPACE'"
  print_error "Available services:"
  $KUBECTL_CMD get svc -n "$NAMESPACE" --no-headers
  exit 1
fi

# Find Auth service (look for 'auth' in name)
AUTH_SERVICE=$($KUBECTL_CMD get svc -n "$NAMESPACE" --no-headers -o custom-columns=NAME:.metadata.name | grep -i "auth" | head -1)
if [ -z "$AUTH_SERVICE" ]; then
  print_error "Could not find Auth service in namespace '$NAMESPACE'"
  print_error "Available services:"
  $KUBECTL_CMD get svc -n "$NAMESPACE" --no-headers
  exit 1
fi

# Get actual service ports - use first port for each service
GRAPHQL_SERVICE_PORT=$($KUBECTL_CMD get svc -n "$NAMESPACE" "$GRAPHQL_SERVICE" -o jsonpath='{.spec.ports[0].port}')
AUTH_SERVICE_PORT=$($KUBECTL_CMD get svc -n "$NAMESPACE" "$AUTH_SERVICE" -o jsonpath='{.spec.ports[0].port}')

print_success "Auto-detected services:"
echo "  - GraphQL Service: $GRAPHQL_SERVICE (port $GRAPHQL_SERVICE_PORT)"
echo "  - Auth Service:    $AUTH_SERVICE (port $AUTH_SERVICE_PORT)"
echo ""

# Kill any existing port-forward processes
print_info "Stopping any existing port-forward processes..."
#pkill -f "kubectl port-forward" 2>/dev/null || true
sleep 1

print_success "Backend port-forwards:"
echo "  - GraphQL: will map localhost:$GRAPHQL_PORT -> svc/$GRAPHQL_SERVICE:$GRAPHQL_SERVICE_PORT"
echo "  - Auth:    will map localhost:$AUTH_PORT -> svc/$AUTH_SERVICE:$AUTH_SERVICE_PORT"
echo ""

print_success "Starting port-forward for GraphQL server..."
echo ""
$KUBECTL_CMD port-forward -n "$NAMESPACE" "svc/$GRAPHQL_SERVICE" "$GRAPHQL_PORT:$GRAPHQL_SERVICE_PORT" &
GRAPHQL_PID=$!

print_success "Starting port-forward for Auth server..."
echo ""
$KUBECTL_CMD port-forward -n "$NAMESPACE" "svc/$AUTH_SERVICE" "$AUTH_PORT:$AUTH_SERVICE_PORT" &
AUTH_PID=$!

echo ""
print_success "Port-forward started!"
echo "  GraphQL PID: $GRAPHQL_PID"
echo "  Auth PID:    $AUTH_PID"
echo ""

print_warning "Press Ctrl+C to stop port-forward"
echo ""

# Cleanup on exit
trap "print_info 'Stopping port-forwards...'; kill $GRAPHQL_PID $AUTH_PID 2>/dev/null; print_success 'Port-forwards stopped'" EXIT

# Keep the process running
wait $GRAPHQL_PID $AUTH_PID
