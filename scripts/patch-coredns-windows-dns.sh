#!/usr/bin/env bash
# Patch CoreDNS to use real Windows DNS servers directly.
set -euo pipefail

DNS1="10.50.53.53"
DNS2="10.50.53.54"

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
    forward . ${DNS1} ${DNS2} {
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
echo "CoreDNS patched — DNS: $DNS1 $DNS2"


