#!/usr/bin/env python3
import json, subprocess, sys

cr = json.loads(subprocess.check_output(
    ["kubectl", "get", "clusterrole", "infra-cluster-role", "-o", "json"]
))

new_rule = {
    "apiGroups": ["rbac.authorization.k8s.io"],
    "resources": ["clusterroles", "clusterrolebindings", "roles", "rolebindings"],
    "verbs": ["get", "list", "create", "update", "patch", "delete"]
}

# Avoid duplicates
already = any(
    "rbac.authorization.k8s.io" in r.get("apiGroups", [])
    for r in cr["rules"]
)

if already:
    print("RBAC rule already present, no change needed.")
    sys.exit(0)

cr["rules"].append(new_rule)

# Remove server-managed fields
for f in ["resourceVersion", "uid", "creationTimestamp", "managedFields", "annotations"]:
    cr.get("metadata", {}).pop(f, None)

proc = subprocess.run(
    ["kubectl", "apply", "-f", "-"],
    input=json.dumps(cr).encode(),
    capture_output=True
)
print(proc.stdout.decode())
if proc.returncode != 0:
    print(proc.stderr.decode(), file=sys.stderr)
    sys.exit(proc.returncode)
