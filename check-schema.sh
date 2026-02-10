#!/bin/bash

# Test if getKubernetesNamespaces exists in GraphQL schema
echo "Checking GraphQL schema for getKubernetesNamespaces..."
echo ""

RESPONSE=$(curl -s -X POST http://localhost:8080/query \
  -H 'Content-Type: application/json' \
  -d '{"query":"{__type(name:\"Query\"){fields{name}}}"}')

echo "All Query fields:"
echo "$RESPONSE" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    fields = [f['name'] for f in data.get('data', {}).get('__type', {}).get('fields', [])]
    for field in sorted(fields):
        if 'namespace' in field.lower() or 'agent' in field.lower():
            print(f'  ✓ {field}')
    if not any('namespace' in f.lower() for f in fields):
        print('  ✗ getKubernetesNamespaces NOT FOUND')
        print('')
        print('First 10 fields:', ', '.join(sorted(fields)[:10]))
except Exception as e:
    print(f'Error: {e}')
    print('Response:', data)
" 2>/dev/null || echo "Connection failed - port-forward may not be active"
