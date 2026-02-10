#!/bin/bash
# Test GraphQL schema for getKubernetesNamespaces query

QUERY='{
  __type(name: "Query") {
    fields(includeDeprecated: false) {
      name
    }
  }
}'

curl -s -X POST http://localhost:8081/query \
  -H 'Content-Type: application/json' \
  -d "{\"query\": \"$(echo $QUERY | tr '\n' ' ')\"}" | \
  python3 -m json.tool | grep -E "getKubernetesNamespaces|getAgents|listAgents" || echo "Query not found"
