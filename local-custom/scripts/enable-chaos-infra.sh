#!/bin/bash

# Enable Litmus Chaos Infrastructure Script
# Applies the manifest and restarts components to pick up SERVER_ADDR.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
LOCAL_ENV_FILE="${LOCAL_ENV_FILE:-$REPO_ROOT/local-custom/config/.env}"
if [ -z "${MANIFEST:-}" ]; then
  CANDIDATE_MANIFESTS=(
    "$REPO_ROOT/local-custom/agentcert-framework-litmus-chaos-enable.yml"
    "$REPO_ROOT/local-custom/agentcert-framwork-litmus-chaos-enable.yml"
    "$REPO_ROOT/local-custom/k8s/litmus-installation.yaml"
  )
  for candidate in "${CANDIDATE_MANIFESTS[@]}"; do
    if [ -f "$candidate" ]; then
      MANIFEST="$candidate"
      break
    fi
  done
else
  MANIFEST="$MANIFEST"
fi
NAMESPACE="litmus-exp"

info() { echo "[INFO] $1"; }
warn() { echo "[WARN] $1"; }
err() { echo "[ERROR] $1"; }

get_env_value() {
  local key="$1"
  local file="$2"
  [ -f "$file" ] || return 0
  local raw
  raw=$(grep -E "^${key}[[:space:]]*=" "$file" | tail -1 | cut -d'=' -f2-)
  raw=$(echo "$raw" | sed 's/^ *//;s/ *$//' | tr -d '\r\n')
  raw=${raw#"\""}
  raw=${raw%"\""}
  raw=${raw#"'"}
  raw=${raw%"'"}
  echo "$raw"
}

escape_sed_replacement() {
  printf '%s' "$1" | sed 's/[&|\\]/\\&/g'
}

resolve_mongo_connection() {
  local mongodb_host mongodb_port mongodb_username mongodb_password mongodb_auth_source mongodb_db_name
  local mongodb_project_id mongodb_environment_id

  mongodb_host=$(get_env_value "MONGODB_HOST" "$LOCAL_ENV_FILE")
  mongodb_host="${mongodb_host:-localhost}"
  mongodb_port=$(get_env_value "MONGODB_PORT" "$LOCAL_ENV_FILE")
  mongodb_port="${mongodb_port:-27017}"
  mongodb_username=$(get_env_value "MONGODB_USERNAME" "$LOCAL_ENV_FILE")
  mongodb_username="${mongodb_username:-root}"
  mongodb_password=$(get_env_value "MONGODB_PASSWORD" "$LOCAL_ENV_FILE")
  mongodb_password="${mongodb_password:-1234}"
  mongodb_auth_source=$(get_env_value "MONGODB_AUTH_SOURCE" "$LOCAL_ENV_FILE")
  mongodb_auth_source="${mongodb_auth_source:-admin}"
  mongodb_db_name=$(get_env_value "MONGODB_DB_NAME" "$LOCAL_ENV_FILE")
  mongodb_db_name="${mongodb_db_name:-litmus}"
  mongodb_project_id=$(get_env_value "LITMUS_PROJECT_ID" "$LOCAL_ENV_FILE")
  [ -z "$mongodb_project_id" ] && mongodb_project_id=$(get_env_value "PROJECT_ID" "$LOCAL_ENV_FILE")
  mongodb_environment_id=$(get_env_value "LITMUS_ENVIRONMENT_ID" "$LOCAL_ENV_FILE")
  [ -z "$mongodb_environment_id" ] && mongodb_environment_id=$(get_env_value "ENVIRONMENT_ID" "$LOCAL_ENV_FILE")

  MONGO_HOST="$mongodb_host"
  MONGO_PORT="$mongodb_port"
  MONGO_USERNAME="$mongodb_username"
  MONGO_PASSWORD="$mongodb_password"
  MONGO_AUTH_SOURCE="$mongodb_auth_source"
  MONGO_DB_NAME="$mongodb_db_name"
  MONGO_PROJECT_ID="$mongodb_project_id"
  MONGO_ENVIRONMENT_ID="$mongodb_environment_id"
}

extract_manifest_value() {
  local key="$1"
  local file="$2"
  grep -E "^[[:space:]]*${key}:[[:space:]]*" "$file" | head -n 1 | sed -E "s|^[[:space:]]*${key}:[[:space:]]*||" | tr -d '\r\n' | sed -E 's|^"||; s|"$||'
}

resolve_server_addr() {
  local server_addr
  server_addr=$(get_env_value "SERVER_ADDR" "$LOCAL_ENV_FILE")
  if [ -z "$server_addr" ]; then
    server_addr="http://litmusportal-server-service.litmus-chaos.svc.cluster.local:9004/query"
  fi
  echo "$server_addr"
}

# The GraphQL server always uses the "litmus" database (hardcoded DbName in init.go).
# All Mongo operations below target only that database.
litmus_mongo_uri() {
  echo "mongodb://${MONGO_USERNAME}:${MONGO_PASSWORD}@${MONGO_HOST}:${MONGO_PORT}/litmus?authSource=${MONGO_AUTH_SOURCE}"
}

# Scenario 2 (restart/redeploy): read the confirmed registered infra from litmus DB.
fetch_registered_infra_from_mongo() {
  local uri
  uri="$(litmus_mongo_uri)"
  local out
  out=$(MONGO_PROJECT_ID="$MONGO_PROJECT_ID" MONGO_ENVIRONMENT_ID="$MONGO_ENVIRONMENT_ID" mongosh --quiet "$uri" --eval '
    const projectID = process.env.MONGO_PROJECT_ID || "";
    const environmentID = process.env.MONGO_ENVIRONMENT_ID || "";
    const query = { is_infra_confirmed: true, is_removed: { $ne: true } };
    if (projectID) query.project_id = projectID;
    if (environmentID) query.environment_id = environmentID;

    const doc = db.chaosInfrastructures
      .find(query)
      .sort({ updated_at: -1 })
      .limit(1)
      .toArray()[0];
    if (!doc || !doc.infra_id || !doc.access_key) { quit(2); }
    print(doc.infra_id);
    print(doc.access_key);
  ' 2>/dev/null || true)

  local id; id="$(echo "$out" | sed -n '1p')"
  local key; key="$(echo "$out" | sed -n '2p')"
  if [ -n "$id" ] && [ -n "$key" ]; then
    printf '%s\n%s\n' "$id" "$key"
    return 0
  fi
  return 1
}

# Activate the given infra ID in the litmus DB (never upserts — only updates real records).
# Optional second arg: access_key from the manifest — synced into the DB so the subscriber
# and server always agree on the credential (critical after downloading a fresh UI manifest).
activate_infra_in_mongo() {
  local infra_id="$1"
  local access_key="${2:-}"
  local uri
  uri="$(litmus_mongo_uri)"
  local out
  out=$(MONGO_INFRA_ID="$infra_id" MONGO_ACCESS_KEY="$access_key" MONGO_PROJECT_ID="$MONGO_PROJECT_ID" MONGO_ENVIRONMENT_ID="$MONGO_ENVIRONMENT_ID" mongosh --quiet "$uri" --eval "
    const id  = process.env.MONGO_INFRA_ID;
    const accessKey = process.env.MONGO_ACCESS_KEY || '';
    const projectID = process.env.MONGO_PROJECT_ID || '';
    const environmentID = process.env.MONGO_ENVIRONMENT_ID || '';
    const now = NumberLong(String(Date.now()));
    const targetQuery = { infra_id: id };
    if (projectID) targetQuery.project_id = projectID;
    if (environmentID) targetQuery.environment_id = environmentID;
    const exists = db.chaosInfrastructures.countDocuments(targetQuery);
    if (!exists) { print('skipped=not_registered'); quit(0); }
    const deactivateQuery = { infra_id: { \$ne: id } };
    if (projectID) deactivateQuery.project_id = projectID;
    if (environmentID) deactivateQuery.environment_id = environmentID;
    db.chaosInfrastructures.updateMany(
      deactivateQuery,
      { \$set: { is_active: false, updated_at: now } }
    );
    const setFields = { is_active: true, updated_at: now };
    if (accessKey) setFields.access_key = accessKey;
    db.chaosInfrastructures.updateOne(
      targetQuery,
      { \$set: setFields }
    );
    const updated = db.chaosInfrastructures.findOne(targetQuery);
    print('activated=' + (updated && updated.is_active ? 'true' : 'false'));
    if (accessKey) print('access_key_synced=true');
  " 2>/dev/null || true)
  info "Mongo activate: $out"
  echo "$out" | grep -q 'activated=true'
}

if ! command -v kubectl >/dev/null 2>&1; then
  err "kubectl is not installed or not in PATH"
  exit 1
fi

if [ ! -f "$MANIFEST" ]; then
  err "Manifest not found: $MANIFEST"
  err "Place the UI-generated infra file at D:\\Studies\\AgentCert\\local-custom\\agentcert-framework-litmus-chaos-enable.yml"
  err "Or run with MANIFEST set to the full path of your infra manifest"
  exit 1
fi

if ! command -v mongosh >/dev/null 2>&1; then
  err "mongosh is not installed or not in PATH"
  exit 1
fi

resolve_mongo_connection

# ── SCENARIO 1: UI manifest has real INFRA_ID/ACCESS_KEY ─────────────────────
# When the user creates infra via the UI and copies the downloaded manifest to
# local-custom/agentcert-framework-litmus-chaos-enable.yml, those values are
# the authoritative source. Just activate that infra in Mongo and apply.
#
# ── SCENARIO 2: Manifest has placeholders (restart / redeploy) ───────────────
# No new manifest was provided. Read the confirmed registered infra from the
# litmus DB (the only DB the control-plane reads) and inject those values.

INFRA_ID="$(extract_manifest_value "INFRA_ID" "$MANIFEST")"
ACCESS_KEY="$(extract_manifest_value "ACCESS_KEY" "$MANIFEST")"
SERVER_ADDR="$(resolve_server_addr)"

if [ -z "$INFRA_ID" ] || [ "$INFRA_ID" = "__INFRA_ID__" ] || \
   [ -z "$ACCESS_KEY" ] || [ "$ACCESS_KEY" = "__ACCESS_KEY__" ]; then
  info "Scenario 2 (restart/redeploy): manifest has placeholders. Reading registered infra from litmus DB..."
  infra_data="$(fetch_registered_infra_from_mongo || true)"
  INFRA_ID="$(echo "$infra_data" | sed -n '1p')"
  ACCESS_KEY="$(echo "$infra_data" | sed -n '2p')"
  if [ -z "$INFRA_ID" ] || [ -z "$ACCESS_KEY" ]; then
    err "No confirmed registered infra found in litmus DB. Register the infra via the UI first."
    exit 1
  fi
  info "Scenario 2: resolved INFRA_ID=$INFRA_ID from litmus DB"
else
  info "Scenario 1 (first deploy): using INFRA_ID=$INFRA_ID from UI manifest"
fi

info "Activating infra $INFRA_ID in litmus DB (syncing access_key=$ACCESS_KEY)..."
if ! activate_infra_in_mongo "$INFRA_ID" "$ACCESS_KEY"; then
  warn "Infra not yet registered in litmus DB (this is normal on very first deploy before subscriber connects)."
fi

info "Using SERVER_ADDR: $SERVER_ADDR"

rendered_manifest="$(mktemp -t litmus-infra-XXXXXX.yaml)"
INFRA_ID_ESCAPED="$(escape_sed_replacement "$INFRA_ID")"
ACCESS_KEY_ESCAPED="$(escape_sed_replacement "$ACCESS_KEY")"
SERVER_ADDR_ESCAPED="$(escape_sed_replacement "$SERVER_ADDR")"
sed -E \
  -e "s|(instanceID:[[:space:]]*).*|\\1${INFRA_ID_ESCAPED}|" \
  -e "s|(INFRA_ID:[[:space:]]*).*|\\1${INFRA_ID_ESCAPED}|" \
  -e "s|(ACCESS_KEY:[[:space:]]*).*|\\1${ACCESS_KEY_ESCAPED}|" \
  -e "s|(SERVER_ADDR:[[:space:]]*).*|\\1${SERVER_ADDR_ESCAPED}|" \
  "$MANIFEST" > "$rendered_manifest"

info "Applying chaos infra manifest..."
kubectl apply -f "$rendered_manifest"
rm -f "$rendered_manifest"

# The UI-generated infra manifest always creates an 'argo-chaos' ServiceAccount in the
# infra namespace. Bind it to cluster-admin so Helm installs (which create ClusterRoles,
# ServiceAccounts, nonResourceURL rules, etc.) succeed without RBAC escalation errors.
# This is applied here — not in the manifest — so the manifest stays UI-managed.
info "Binding argo-chaos to cluster-admin in namespace ${NAMESPACE}..."
kubectl create clusterrolebinding argo-chaos-cluster-admin \
    --clusterrole=cluster-admin \
    --serviceaccount="${NAMESPACE}:argo-chaos" \
    --dry-run=client -o yaml | kubectl apply -f -
info "cluster-admin bound to ${NAMESPACE}/argo-chaos"

# Ensure infra-cluster-role always has the permissions required for experiment runs.
# Rules added here:
#   1. rbac.authorization.k8s.io - so argo-chaos can create workload-discovery ClusterRoles
#   2. apps watch verbs - Kubernetes RBAC escalation prevention requires the grantor to already
#      hold every verb it tries to grant; litmus-workload-discoverer grants watch on apps resources
#      so argo-chaos must also have watch on those resources.
info "Ensuring infra-cluster-role has all required experiment runtime permissions..."
if kubectl get clusterrole infra-cluster-role >/dev/null 2>&1; then
  kubectl get clusterrole infra-cluster-role -o json | python3 -c "
import json, sys

cr = json.load(sys.stdin)
rules = cr['rules']

# Rules that must be present (idempotent - only added if not already covered)
required = [
    {
        'apiGroups': ['rbac.authorization.k8s.io'],
        'resources': ['clusterroles', 'clusterrolebindings', 'roles', 'rolebindings'],
        'verbs': ['get', 'list', 'create', 'update', 'patch', 'delete']
    },
    {
        # argo-chaos must hold watch on apps resources to be allowed to grant it
        # to litmus-workload-discoverer (Kubernetes RBAC escalation prevention)
        'apiGroups': ['apps'],
        'resources': ['deployments', 'statefulsets', 'daemonsets', 'replicasets'],
        'verbs': ['get', 'list', 'watch', 'create', 'update', 'patch']
    },
    {
        'apiGroups': [''],
        'resources': ['pods', 'namespaces', 'services', 'configmaps', 'secrets',
                      'serviceaccounts', 'persistentvolumeclaims', 'persistentvolumes',
                      'endpoints', 'events', 'replicationcontrollers'],
        'verbs': ['get', 'list', 'watch', 'create', 'update', 'patch', 'delete']
    },
    {
        # Helm may create batch jobs, cronjobs, etc.
        'apiGroups': ['batch'],
        'resources': ['jobs', 'cronjobs'],
        'verbs': ['get', 'list', 'create', 'update', 'patch', 'delete']
    },
    {
        # Core Helm resource permissions across all common API groups
        'apiGroups': ['*'],
        'resources': ['*'],
        'verbs': ['get', 'list', 'watch']
    },
    {
        # nonResourceURLs required so argo-chaos can grant prometheus /metrics access
        # (Kubernetes escalation prevention: grantor must hold the permission)
        'nonResourceURLs': ['/metrics', '/healthz', '/readyz', '/livez'],
        'verbs': ['get']
    }
]

def covers(existing_rules, needed):
    # nonResourceURLs rule — different structure, no apiGroups/resources
    if 'nonResourceURLs' in needed:
        for rule in existing_rules:
            if (set(needed['nonResourceURLs']).issubset(set(rule.get('nonResourceURLs', []))) and
                set(needed['verbs']).issubset(set(rule.get('verbs', [])))):
                return True
        return False
    for rule in existing_rules:
        if (set(needed['apiGroups']).issubset(set(rule.get('apiGroups', []))) and
            set(needed['resources']).issubset(set(rule.get('resources', []))) and
            set(needed['verbs']).issubset(set(rule.get('verbs', [])))):
            return True
    return False

added = []
for req in required:
    if not covers(rules, req):
        rules.append(req)
        added.append(str(req.get('apiGroups', req.get('nonResourceURLs', '?'))))

for f in ['resourceVersion', 'uid', 'creationTimestamp', 'managedFields']:
    cr.get('metadata', {}).pop(f, None)

if added:
    print('NEEDS_UPDATE:' + ','.join(added), file=sys.stderr)
else:
    print('NO_CHANGE', file=sys.stderr)

print(json.dumps(cr))
" 2>/tmp/rbac_patch_status | kubectl apply -f - >/dev/null
  STATUS=$(cat /tmp/rbac_patch_status)
  if echo "$STATUS" | grep -q 'NEEDS_UPDATE'; then
    info "infra-cluster-role patched: $STATUS"
  else
    info "infra-cluster-role already has all required permissions"
  fi
fi

info "Restarting subscriber/event-tracker/workflow-controller..."
if kubectl get deployment -n "$NAMESPACE" subscriber >/dev/null 2>&1; then
  kubectl rollout restart deployment/subscriber -n "$NAMESPACE"
else
  warn "subscriber deployment not found in $NAMESPACE"
fi

if kubectl get deployment -n "$NAMESPACE" event-tracker >/dev/null 2>&1; then
  kubectl rollout restart deployment/event-tracker -n "$NAMESPACE"
else
  warn "event-tracker deployment not found in $NAMESPACE"
fi

if kubectl get deployment -n "$NAMESPACE" workflow-controller >/dev/null 2>&1; then
  kubectl rollout restart deployment/workflow-controller -n "$NAMESPACE"
else
  warn "workflow-controller deployment not found in $NAMESPACE"
fi

info "Waiting for rollouts..."
kubectl rollout status deployment/subscriber -n "$NAMESPACE" --timeout=120s || true
kubectl rollout status deployment/event-tracker -n "$NAMESPACE" --timeout=120s || true
kubectl rollout status deployment/workflow-controller -n "$NAMESPACE" --timeout=120s || true

# ── Clean up permanent Prometheus + Grafana if they exist (now per-experiment) ─
for res in deployment/prometheus deployment/grafana service/prometheus service/grafana \
           configmap/prometheus-config configmap/grafana-datasources configmap/grafana-dashboard-provider; do
  kubectl delete "$res" -n "$NAMESPACE" --ignore-not-found 2>/dev/null && \
    info "Removed $res from $NAMESPACE (now per-experiment)" || true
done

kubectl get pods -n "$NAMESPACE" -o wide

info "Enable complete."
