#!/bin/bash

# Test if getKubernetesNamespaces query works
echo "Testing getKubernetesNamespaces query..."
echo ""

RESPONSE=$(curl -s -X POST http://localhost:8080/query \
  -H 'Content-Type: application/json' \
  -d '{"query":"{getKubernetesNamespaces}"}')

echo "Response:"
echo "$RESPONSE" | python3 -m json.tool

if echo "$RESPONSE" | grep -q "Cannot query field"; then
  echo ""
  echo "✗ ERROR: getKubernetesNamespaces query is NOT in schema"
  echo "  Schema generation may not have included it yet"
elif echo "$RESPONSE" | grep -q "errors"; then
  echo ""
  echo "⚠ Query returned error (may need auth or K8s access)"
else
  echo ""
  echo "✓ Query executed successfully!"
fi
