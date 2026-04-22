#!/bin/bash
echo "=== Collections in litmus database ==="
mongosh 'mongodb://root:1234@localhost:27017/litmus?authSource=admin&replicaSet=rs0' --eval 'db.getCollectionNames()'

echo ""
echo "=== Looking for probe collections ==="
mongosh 'mongodb://root:1234@localhost:27017/litmus?authSource=admin&replicaSet=rs0' --eval 'db.getCollectionNames().filter(name => name.includes("probe"))'

echo ""
echo "=== All probes ==="
mongosh 'mongodb://root:1234@localhost:27017/litmus?authSource=admin&replicaSet=rs0' --eval 'db.getCollection("probe-collection").find().limit(5).pretty()'
