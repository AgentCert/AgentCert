#!/usr/bin/env bash
set -euo pipefail

LOGIN_BODY='{"username":"admin","password":"Sanjeev@123"}'
TOKEN=$(curl -s -k https://localhost:2001/auth/login -H 'Content-Type: application/json' -d "$LOGIN_BODY" | python3 -c "import sys,json;print(json.load(sys.stdin).get('accessToken',''))")

if [ -z "$TOKEN" ]; then
  echo "Failed to get token" >&2
  exit 1
fi

BODY='{"operationName":"getKubernetesNamespaces","variables":{},"query":"query getKubernetesNamespaces {\n  getKubernetesNamespaces\n}"}'

curl -s -k https://localhost:2001/api/query \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${TOKEN}" \
  --data-raw "$BODY" | python3 -m json.tool
