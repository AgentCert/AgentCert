#!/usr/bin/env bash
POD=$(kubectl get pods -n litellm -o jsonpath='{.items[0].metadata.name}')
echo "Testing DNS from pod: $POD"
kubectl cp /mnt/d/Studies/AgentCert/scripts/test_dns.py litellm/${POD}:/tmp/test_dns.py
kubectl exec -n litellm ${POD} -- python3 /tmp/test_dns.py
