#!/usr/bin/env bash
set -euo pipefail

BODY='{"query":"{getKubernetesNamespaces}"}'

curl -s -H "Content-Type: application/json" -d "$BODY" http://localhost:8080/query | python3 -m json.tool
