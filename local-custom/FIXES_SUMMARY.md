# AgentCert Framework - Fixes Summary & Build Integration

## Overview

All critical fixes have been **integrated into the build pipeline** and will be automatically applied for future deployments.

---

## Fixes Applied

### 1. ✅ Langfuse Authentication (CRITICAL)
**Status**: Permanently fixed in source code  
**Location**: 
- `chaoscenter/graphql/server/pkg/observability/langfuse_tracer.go` (Line 49)
- `chaoscenter/graphql/server/pkg/agent_registry/langfuse_client.go` (Lines 61, 106, 193)
- `chaoscenter/graphql/server/graph/resolver.go` (Line 76)

**What was changed**:
- Changed from Bearer token to Basic Auth (publicKey:secretKey)
- Now passes both publicKey AND secretKey to LangfuseClient
- All HTTP requests to Langfuse API now use `req.SetBasicAuth(publicKey, secretKey)`

**Build Integration**: 
- ✅ Automatically included when Docker image is built
- ✅ Langfuse credentials are synced during deployment via `sync_langfuse_env_from_dotenv()`

**Verification**:
```bash
# Check logs for successful trace uploads
kubectl -n litmus-chaos logs -f deployment/litmusportal-server | grep "Successfully logged experiment trace"
# Output: Successfully logged experiment trace to Langfuse (status: 200)
```

---

### 2. ✅ Chaos Experiment Image Pull Policy (CRITICAL)
**Status**: Fixed via helper script + build integration  
**Location**: `local-custom/scripts/fix-litmus-config.sh`

**What it does**:
- Sets `LIB_IMAGE_PULL_POLICY=IfNotPresent` for chaos experiments
- Prevents image pull failures in offline/minikube environments
- Creates ConfigMap with default pull policy settings

**Build Integration**:
- ✅ Now automatically called from `build-and-deploy.sh` during deployment
- ✅ Helper script included in local-custom/scripts folder

**How to use**:
```bash
# Automatic (via build-and-deploy.sh):
bash local-custom/scripts/build-and-deploy.sh

# Manual (if needed):
bash local-custom/scripts/fix-litmus-config.sh litmus-chaos
```

---

## Files Modified/Created

### Modified Files (Source Code)
```
chaoscenter/graphql/server/pkg/observability/langfuse_tracer.go
chaoscenter/graphql/server/pkg/agent_registry/langfuse_client.go
chaoscenter/graphql/server/graph/resolver.go
local-custom/scripts/build-and-deploy.sh
```

### New Files Created
```
local-custom/scripts/fix-litmus-config.sh          (Helper script)
docs/FIXES_APPLIED.md                              (Detailed documentation)
local-custom/FIXES_SUMMARY.md                      (This file)
```

---

## Build Pipeline Integration

### Current Build Process
```
build-and-deploy.sh
├── Prerequisite checks
├── MongoDB replica set fix
├── Sync environment variables
│   ├── Azure OpenAI (azure_env_from_dotenv)
│   └── Langfuse (langfuse_env_from_dotenv)
├── Docker image cleanup
├── Build Docker images
│   └── Langfuse credentials embedded in image
├── Load images to minikube
├── Deploy Kubernetes manifests
├── Apply Litmus config fixes ← NEW STEP
│   └── Calls fix-litmus-config.sh
└── Verify pods are ready
```

### What Happens Automatically

1. **Langfuse Fix**:
   - Docker image is built with fixed source code (Basic Auth)
   - Deployment syncs LANGFUSE_PUBLIC_KEY and LANGFUSE_SECRET_KEY to cluster Secret
   - GraphQL server uses both keys for authentication

2. **Chaos Experiment Image Pull Fix**:
   - fix-litmus-config.sh is called during deployment
   - ConfigMap is created with `LIB_IMAGE_PULL_POLICY=IfNotPresent`
   - All future chaos experiments use IfNotPresent policy

---

## Deployment Checklist

### For Fresh Deployments
```bash
cd local-custom/scripts
bash build-and-deploy.sh
# Everything is automatic!
```

### For Existing Deployments
If you already have a running cluster and want to apply the fixes:

```bash
# 1. Update source code (git pull or manual update)
# 2. Rebuild and redeploy
bash local-custom/scripts/build-and-deploy.sh --skip-build=false

# OR just update configs without rebuilding
bash local-custom/scripts/build-and-deploy.sh --skip-build=true

# OR manually run the config fix
bash local-custom/scripts/fix-litmus-config.sh litmus-chaos
```

---

## Verification Steps

### 1. Verify Langfuse Authentication
```bash
# Check if tracer initialized with correct auth
kubectl -n litmus-chaos logs deployment/litmusportal-server | grep "Langfuse tracer initialized"

# Run an experiment and check for successful uploads
kubectl -n litmus-chaos logs deployment/litmusportal-server | grep "Successfully logged experiment trace"

# Check Langfuse UI for uploaded traces
# Traces should have:
# - Input: (nil)
# - Output: (nil) 
# - Metadata: {phase: "injection", priority: "high", etc.}
```

### 2. Verify Image Pull Policy
```bash
# Create and run a chaos experiment
# Check the pod status
kubectl -n litmus-exp get pods

# Verify the policy setting
kubectl -n litmus-exp get configmap chaosengine-defaults -o yaml
# Should show: LIB_IMAGE_PULL_POLICY: "IfNotPresent"
```

### 3. End-to-End Test
```bash
# 1. Start port forwarding
bash local-custom/scripts/port-forward.sh

# 2. Start web UI (new terminal)
bash local-custom/scripts/start-web.sh

# 3. Access UI: https://localhost:2001

# 4. Create and run a chaos experiment
# 5. Check Langfuse for uploaded traces
# 6. Verify experiment completes successfully
```

---

## Important Notes

### For Offline/Airgapped Environments
1. Ensure Docker images are pre-loaded into your minikube
2. The `fix-litmus-config.sh` script will automatically set `IfNotPresent` policy
3. No external network access is required after images are loaded

### For Online Environments
1. All fixes work seamlessly
2. Image pull policy will be set to IfNotPresent (safe default)
3. Langfuse authentication is now robust and uses Basic Auth

### Known Limitations
- Probe validation has a cosmetic issue (doesn't prevent chaos injection)
- This is a separate issue and doesn't affect experiment execution

---

## Troubleshooting

### Langfuse Traces Not Uploading
```bash
# Check environment variables are set
kubectl -n litmus-chaos get secret litmus-portal-admin-secret -o yaml

# Check pod has the credentials
kubectl -n litmus-chaos exec deployment/litmusportal-server -- env | grep LANGFUSE

# Check logs for 401 errors (should not see them now)
kubectl -n litmus-chaos logs deployment/litmusportal-server | grep -i "401\|auth"
```

### Image Pull Failures
```bash
# Check if IfNotPresent is set
kubectl -n litmus-exp get configmap chaosengine-defaults

# Verify images are available locally
minikube image ls | grep go-runner

# Force re-run of fix script
bash local-custom/scripts/fix-litmus-config.sh litmus-chaos
```

---

## Summary

| Component | Status | Integration | Next Run |
|-----------|--------|-------------|----------|
| Langfuse Auth | ✅ Fixed | Built into Docker image | Auto applied |
| Image Pull Policy | ✅ Fixed | Integrated in build script | Auto applied |
| Build Pipeline | ✅ Updated | Calls fix-litmus-config.sh | Every deployment |
| Documentation | ✅ Complete | docs/FIXES_APPLIED.md | Reference |

All fixes are **production-ready** and automatically applied during deployment.

