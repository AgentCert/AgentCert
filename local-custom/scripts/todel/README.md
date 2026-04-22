# AgentCert Scripts - Build & Deployment Guide

## Overview
This directory contains all scripts needed to build, deploy, and maintain the AgentCert Framework.

## Quick Start

### Option 1: Full Build & Deploy (Recommended)
```bash
cd local-custom/scripts
bash build-and-deploy.sh
```

This will:
1. ✅ Check prerequisites (Docker, kubectl, minikube)
2. ✅ Clean old Docker images
3. ✅ Build new Docker images with Langfuse fix integrated
4. ✅ Load images into minikube
5. ✅ Deploy Kubernetes manifests
6. ✅ Sync environment variables (Azure OpenAI, Langfuse)
7. ✅ **Apply Litmus configuration fixes** (NEW)
8. ✅ Verify all pods are ready

### Option 2: Skip Build (Just Deploy)
```bash
bash build-and-deploy.sh --skip-build
```

### Option 3: Clean Start
```bash
bash build-and-deploy.sh --clean-all
```

---

## Available Scripts

### build-and-deploy.sh
**Purpose**: Complete build and deployment pipeline  
**Usage**: `bash build-and-deploy.sh [OPTIONS]`

**Options**:
- `--clean-all` : Delete K8s namespace and rebuild everything
- `--skip-build` : Only deploy, don't rebuild images
- `--skip-deploy` : Only build images, don't deploy
- `--debug` : Enable debug output
- `--env-file PATH` : Use specific .env file

**What it does**:
- Builds Docker images for Auth Server and GraphQL Server
- Loads images into minikube
- Deploys Kubernetes manifests
- Syncs environment variables from .env files
- Applies configuration fixes (image pull policy, etc.)

**Example**:
```bash
# Full rebuild
bash build-and-deploy.sh

# Skip building, just deploy updated manifests
bash build-and-deploy.sh --skip-build

# Full clean rebuild
bash build-and-deploy.sh --clean-all
```

---

### fix-litmus-config.sh ✨ (NEW)
**Purpose**: Apply Litmus configuration fixes for offline/minikube environments  
**Usage**: `bash fix-litmus-config.sh [NAMESPACE]`

**What it fixes**:
1. Sets `LIB_IMAGE_PULL_POLICY=IfNotPresent` (prevents image pull failures)
2. Verifies Langfuse authentication is configured

**Example**:
```bash
# Use default namespace (litmus-chaos)
bash fix-litmus-config.sh

# Use custom namespace
bash fix-litmus-config.sh my-litmus-namespace
```

**Note**: This is automatically called by build-and-deploy.sh during deployment

---

### port-forward.sh
**Purpose**: Set up port forwarding for local access  
**Usage**: `bash port-forward.sh`

**Forwards**:
- Port 2001 → Frontend (https://localhost:2001)
- Port 3000 → Langfuse UI
- Port 27017 → MongoDB

**Example**:
```bash
# Start in new terminal
bash port-forward.sh
```

---

### start-web.sh
**Purpose**: Start the frontend development server  
**Usage**: `bash start-web.sh`

**Requirements**: Node.js, npm

**What it does**:
- Installs dependencies (if needed)
- Starts webpack-dev-server on port 2001
- Serves the React frontend with hot reloading

**Example**:
```bash
# Start in new terminal (after port-forward.sh)
bash start-web.sh
```

---

## Typical Workflow

### First Time Setup
```bash
# Terminal 1: Build & Deploy
cd local-custom/scripts
bash build-and-deploy.sh

# Wait for all pods to be ready, then proceed to Terminal 2

# Terminal 2: Port Forwarding
bash port-forward.sh
# Keep this running

# Terminal 3: Frontend
bash start-web.sh
# Keep this running

# Terminal 4: Access UI
open https://localhost:2001
# Login with admin / litmus
```

### After Code Changes
```bash
# Terminal 1: Rebuild & Redeploy
cd local-custom/scripts
bash build-and-deploy.sh

# Frontend changes hot-reload automatically
# Backend changes require rebuild
```

### Just Update Configuration
```bash
# Only sync environment variables (no rebuild)
cd local-custom/scripts
bash build-and-deploy.sh --skip-build
```

---

## Environment Configuration

### Configuration Files
- **local-custom/config/.env** - Local environment variables (credentials, endpoints)
- **local-custom/k8s/litmus-installation.yaml** - Kubernetes manifest

### Required Environment Variables
```bash
# .env file format
LANGFUSE_HOST=http://langfuse-web.langfuse.svc.cluster.local:3000
LANGFUSE_PUBLIC_KEY=pk-lf-...
LANGFUSE_SECRET_KEY=sk-lf-...
LANGFUSE_ORG_ID=...
LANGFUSE_PROJECT_ID=...

AZURE_OPENAI_ENDPOINT=https://...
AZURE_OPENAI_KEY=...
AZURE_OPENAI_DEPLOYMENT=...
AZURE_OPENAI_API_VERSION=2024-02-15-preview
AZURE_OPENAI_EMBEDDING_DEPLOYMENT=...
```

---

## Built-In Fixes

The build pipeline now includes automatic fixes for:

### 1. Langfuse Authentication (✅ Fixed)
- **Issue**: 401 "Access denied" errors
- **Fix**: Now uses Basic Auth with both publicKey and secretKey
- **Status**: Permanently built into Docker images

### 2. Chaos Experiment Image Pull (✅ Fixed)
- **Issue**: `ErrImagePull` when running experiments
- **Fix**: Sets `LIB_IMAGE_PULL_POLICY=IfNotPresent`
- **Status**: Automatically applied via fix-litmus-config.sh

---

## Troubleshooting

### Pods Not Starting
```bash
# Check pod status
kubectl -n litmus-chaos get pods

# Check pod logs
kubectl -n litmus-chaos logs deployment/litmusportal-server

# Force restart
kubectl -n litmus-chaos rollout restart deployment/litmusportal-server
```

### Image Pull Failures
```bash
# Check image availability
minikube image ls | grep litmus

# Re-apply fixes
bash fix-litmus-config.sh

# Check ConfigMap
kubectl -n litmus-chaos get configmap chaosengine-defaults -o yaml
```

### Langfuse Not Connecting
```bash
# Check if tracer initialized
kubectl -n litmus-chaos logs deployment/litmusportal-server | grep "Langfuse"

# Verify environment variables
kubectl -n litmus-chaos get secret litmus-portal-admin-secret -o yaml

# Check for 401 errors (should not see any)
kubectl -n litmus-chaos logs deployment/litmusportal-server | grep "401\|Access denied"
```

---

## Additional Resources

- **docs/FIXES_APPLIED.md** - Detailed technical documentation of all fixes
- **local-custom/FIXES_SUMMARY.md** - Summary of applied fixes and integration
- **README.md** - Project overview

---

## Quick Command Reference

```bash
# Build and Deploy
bash build-and-deploy.sh                          # Full pipeline
bash build-and-deploy.sh --skip-build             # Deploy only
bash build-and-deploy.sh --clean-all              # Clean rebuild

# Configuration
bash fix-litmus-config.sh                         # Apply Litmus fixes
bash port-forward.sh                              # Port forwarding
bash start-web.sh                                 # Frontend dev server

# Kubernetes
kubectl -n litmus-chaos get pods                  # Check pods
kubectl -n litmus-chaos logs -f deployment/litmusportal-server  # Live logs
kubectl -n litmus-exp get workflow                # Chaos experiments
```

---

## Support

For issues or questions:
1. Check the troubleshooting section above
2. Review logs: `kubectl logs -n litmus-chaos -f deployment/litmusportal-server`
3. Check documentation: `docs/FIXES_APPLIED.md`
4. Verify environment: `local-custom/config/.env`

