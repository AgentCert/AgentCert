#!/usr/bin/env bash
set -euo pipefail

BODY='{"username":"admin","password":"Sanjeev@123"}'

curl -s -X POST http://localhost:3030/login \
  -H 'Content-Type: application/json' \
  -d "$BODY" | python3 -m json.tool
