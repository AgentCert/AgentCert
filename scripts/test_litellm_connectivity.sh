#!/usr/bin/env bash
set -euo pipefail

POD=$(kubectl get pods -n litellm -o jsonpath='{.items[0].metadata.name}')
echo "Testing Azure OpenAI endpoint from LiteLLM pod: $POD"

echo "=== DNS resolution ==="
kubectl exec -n litellm "$POD" -- python3 << 'PYEOF'
import socket
try:
    ip = socket.gethostbyname("azureft.openai.azure.com")
    print(f"✓ Azure OpenAI resolved to: {ip}")
except Exception as e:
    print(f"✗ DNS error: {e}")
PYEOF

echo ""
echo "=== HTTPS connectivity to Azure OpenAI ==="
kubectl exec -n litellm "$POD" -- python3 << 'PYEOF'
import urllib.request, urllib.error, ssl, certifi
ctx = ssl.create_default_context(cafile=certifi.where())
try:
    req = urllib.request.Request(
        "https://azureft.openai.azure.com/openai/deployments?api-version=2024-06-01",
        headers={"api-key": "dummy-test-key"}
    )
    resp = urllib.request.urlopen(req, context=ctx, timeout=5)
    print(f"✓ HTTPS status: {resp.status}")
except urllib.error.HTTPError as e:
    print(f"✓ HTTP {e.code} (expected 401 Unauthorized with dummy key)")
except Exception as e:
    print(f"✗ Connection error: {type(e).__name__}: {e}")
PYEOF

echo ""
echo "=== LiteLLM health endpoint ==="
kubectl exec -n litellm "$POD" -- python3 << 'PYEOF'
import urllib.request
try:
    resp = urllib.request.urlopen("http://localhost:4000/health", timeout=5)
    print(f"✓ LiteLLM health status: {resp.status}")
except Exception as e:
    print(f"✗ Health check error: {e}")
PYEOF

echo ""
echo "=== All tests complete ==="
