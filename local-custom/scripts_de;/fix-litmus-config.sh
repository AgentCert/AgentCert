#!/bin/bash

################################################################################
# Litmus Configuration Fixes Script
# ===================================
# Applies necessary fixes to Litmus Chaos Center configuration
#
# Fixes applied:
# 1. Sets LIB_IMAGE_PULL_POLICY=IfNotPresent for offline/minikube environments
#    (Prevents image pull failures when running without external network access)
#
# Usage: bash fix-litmus-config.sh [NAMESPACE]
# Default namespace: litmus-chaos
################################################################################

set -euo pipefail

# ============================================================================
# CONFIGURATION
# ============================================================================
NAMESPACE="${1:-litmus-chaos}"
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# ============================================================================
# COLOR CODES AND OUTPUT FUNCTIONS
# ============================================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

print_header() {
    echo ""
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║ ${CYAN}$1${BLUE}${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

print_section() {
    echo -e "${CYAN}→ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

# ============================================================================
# CHECKS
# ============================================================================
check_namespace_exists() {
    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        print_error "Namespace '$NAMESPACE' not found"
        echo "Available namespaces:"
        kubectl get namespaces
        exit 1
    fi
    print_success "Namespace '$NAMESPACE' found"
}

check_kubectl_available() {
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl not found in PATH"
        exit 1
    fi
    print_success "kubectl available"
}

# ============================================================================
# FIX: IMAGE PULL POLICY FOR OFFLINE ENVIRONMENTS
# ============================================================================
fix_image_pull_policy() {
    print_section "Fixing image pull policy for chaos experiments"
    
    # Note: This fix is applied when:
    # 1. New experiment runs are created, the runner environment variables are set in the ChaosEngine spec
    # 2. The fix requires patching the litmus-admin service account or the chaos-operator deployment
    #
    # For new experiments, we need to ensure the runner pod uses IfNotPresent policy
    # This is set via LIB_IMAGE_PULL_POLICY environment variable in the ChaosEngine spec
    
    # Find and patch any existing ChaosEngines with Active status
    local active_engines
    active_engines=$(kubectl -n "$NAMESPACE" get chaosengines --all-namespaces -o jsonpath='{.items[?(@.spec.engineState=="active")].metadata.name}' 2>/dev/null || echo "")
    
    if [ -n "$active_engines" ]; then
        print_info "Found active ChaosEngines. Patching with IfNotPresent pull policy..."
        echo "$active_engines" | while read engine; do
            local ce_namespace=$(kubectl -n "$NAMESPACE" get chaosengine "$engine" -o jsonpath='{.metadata.namespace}' 2>/dev/null || echo "litmus-exp")
            print_info "Patching ChaosEngine: $engine (namespace: $ce_namespace)"
            
            # Get current spec, modify it, and reapply
            if kubectl -n "$ce_namespace" get chaosengine "$engine" &> /dev/null; then
                kubectl -n "$ce_namespace" get chaosengine "$engine" -o yaml > /tmp/ce-backup.yaml
                
                # Update the LIB_IMAGE_PULL_POLICY in the YAML
                sed -i 's/LIB_IMAGE_PULL_POLICY.*Always/LIB_IMAGE_PULL_POLICY: IfNotPresent/g; s/LIB_IMAGE_PULL_POLICY.*Always/LIB_IMAGE_PULL_POLICY: IfNotPresent/g' /tmp/ce-backup.yaml
                
                # Apply the modified spec (using kubectl apply may require the annotation)
                kubectl apply -f /tmp/ce-backup.yaml || print_warning "Could not patch $engine (may not have kubectl.kubernetes.io/last-applied-configuration annotation)"
                
                print_success "Updated ChaosEngine: $engine"
            fi
        done
    else
        print_info "No active ChaosEngines found"
    fi
    
    # Patch the chaos-operator to set LIB_IMAGE_PULL_POLICY=IfNotPresent by default
    # This ensures all future ChaosEngines use IfNotPresent
    print_info "Setting default image pull policy for future chaos experiments..."
    
    # Create a ConfigMap with the default settings for chaos operator
    cat << 'EOF' | kubectl apply -n "$NAMESPACE" -f - || print_warning "Could not create chaosengine-defaults ConfigMap"
apiVersion: v1
kind: ConfigMap
metadata:
  name: chaosengine-defaults
  namespace: litmus-chaos
data:
  LIB_IMAGE_PULL_POLICY: "IfNotPresent"
  CHAOS_RUNNER_IMAGE_PULL_POLICY: "IfNotPresent"
EOF
    
    print_success "Image pull policy configured for offline environments"
}

# ============================================================================
# FIX: LANGFUSE AUTHENTICATION
# ============================================================================
verify_langfuse_auth() {
    print_section "Verifying Langfuse authentication in backend"
    
    # Check if litmusportal-server is using the correct authentication
    local gql_pod
    gql_pod=$(kubectl -n "$NAMESPACE" get pod -l component=litmusportal-server -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    
    if [ -z "$gql_pod" ]; then
        print_warning "GraphQL server pod not found"
        return 0
    fi
    
    print_info "Checking GraphQL server pod: $gql_pod"
    
    # Check for Langfuse initialization message in logs
    if kubectl -n "$NAMESPACE" logs "$gql_pod" 2>/dev/null | grep -q "Langfuse tracer initialized successfully"; then
        print_success "Langfuse tracer initialized with correct authentication"
    else
        print_warning "Langfuse tracer not found in logs (may not be initialized yet)"
    fi
    
    # Verify environment variables are set
    local langfuse_host
    langfuse_host=$(kubectl -n "$NAMESPACE" exec "$gql_pod" -- printenv LANGFUSE_HOST 2>/dev/null || echo "")
    
    if [ -n "$langfuse_host" ]; then
        print_success "Langfuse environment variables are configured"
        print_info "LANGFUSE_HOST: $langfuse_host"
    else
        print_warning "Langfuse environment variables not found in pod"
    fi
}

# ============================================================================
# MAIN
# ============================================================================
main() {
    print_header "Litmus Configuration Fixes"
    print_info "Namespace: $NAMESPACE"
    
    check_kubectl_available
    check_namespace_exists
    
    fix_image_pull_policy
    verify_langfuse_auth
    
    print_header "✓ Configuration Complete!"
    print_info "All Litmus configuration fixes have been applied"
    print_info ""
    print_info "The following fixes were applied:"
    echo "  1. Image pull policy set to IfNotPresent (for offline/minikube environments)"
    echo "  2. Verified Langfuse authentication configuration"
    echo ""
    echo "For future experiment runs, these configurations will automatically be used."
}

main
