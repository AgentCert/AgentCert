#!/usr/bin/env bash
set -euo pipefail

TOKEN='eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3NzAxMjEyNDUsInJvbGUiOiJhZG1pbiIsInVpZCI6IjBkY2QyZjhlLWU2MjgtNDQxOS04ZjZiLTY3MjI2ZjcxMzQ1YyIsInVzZXJuYW1lIjoiYWRtaW4ifQ.ZnQH_-LGhEOiHdlQ9m4RCnVpQnCUCZW0NeBFx1CGC7FRn4TmvYbCnpEa6-AeKMsQr1iNHDUPYv1pkJzG4Ri6uA'

BODY=$(cat << 'EOF'
{
  "operationName": "getKubeNamespace",
  "variables": {"request": {"infraID": "4f1705ba-15b9-4d68-b1c8-ad86c476463c"}},
  "query": "subscription getKubeNamespace($request: KubeNamespaceRequest!) {\n  getKubeNamespace(request: $request) {\n    infraID\n    kubeNamespace {\n      name\n      __typename\n    }\n    __typename\n  }\n}"
}
EOF
)

curl -s -i -k --max-time 10 https://localhost:2001/api/query \
  -H 'Accept-Language: en-GB,en-US;q=0.9,en;q=0.8' \
  -H 'Connection: keep-alive' \
  -H 'Origin: https://localhost:2001' \
  -H 'Referer: https://localhost:2001/' \
  -H 'Sec-Fetch-Dest: empty' \
  -H 'Sec-Fetch-Mode: cors' \
  -H 'Sec-Fetch-Site: same-origin' \
  -H 'User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36' \
  -H 'accept: */*' \
  -H "authorization: Bearer ${TOKEN}" \
  -H 'content-type: application/json' \
  -H 'sec-ch-ua: "Not(A:Brand";v="8", "Chromium";v="144", "Google Chrome";v="144"' \
  -H 'sec-ch-ua-mobile: ?0' \
  -H 'sec-ch-ua-platform: "Windows"' \
  --data-raw "$BODY"
