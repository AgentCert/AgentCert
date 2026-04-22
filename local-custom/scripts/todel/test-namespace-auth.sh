#!/usr/bin/env bash
set -euo pipefail

TOKEN=$(bash /mnt/c/Users/sanjsingh/Downloads/Studies/AgentCert-Framework/local-custom/scripts/test-auth-login.sh | python3 -c "import sys,json;print(json.load(sys.stdin)['accessToken'])")

BODY='{"query":"{getKubernetesNamespaces}"}'

curl -s -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${TOKEN}" \
  -d "$BODY" \
  http://localhost:8080/query | python3 -m json.tool
