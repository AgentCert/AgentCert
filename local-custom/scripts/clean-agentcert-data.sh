#!/usr/bin/env bash
set -euo pipefail

# Clean AgentCert data (agents, experiments, runs, probes) from MongoDB
# Usage:
#   ./clean-agentcert-data.sh [MONGO_URI]
# or set MONGO_URI env var.

DEFAULT_URI="mongodb://root:1234@172.22.174.61:27017/admin"
MONGO_URI="${1:-${MONGO_URI:-$DEFAULT_URI}}"

if ! command -v mongosh >/dev/null 2>&1; then
  echo "mongosh not found. Please install MongoDB Shell (mongosh)." >&2
  exit 1
fi

echo "Using MongoDB URI: $MONGO_URI"

mongosh "$MONGO_URI" --quiet --eval "
  db = db.getSiblingDB('litmus');
  print('Cleaning collections in db: ' + db.getName());
  const results = {
    agentRegistry: db.agentRegistry.deleteMany({}).deletedCount,
    chaosExperiments: db.chaosExperiments.deleteMany({}).deletedCount,
    chaosExperimentRuns: db.chaosExperimentRuns.deleteMany({}).deletedCount,
    chaosProbes: db.chaosProbes.deleteMany({}).deletedCount
  };
  printjson(results);
"