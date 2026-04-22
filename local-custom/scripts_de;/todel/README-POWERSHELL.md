# PowerShell Build Script - Windows Native Support

## Quick Start - Ubuntu/WSL to PowerShell Conversion

If you're currently using the Ubuntu/WSL command:
```bash
wsl -d Ubuntu bash -c "cd /mnt/c/Users/sanjsingh/Downloads/Studies/AgentCert-Framework/local-custom/scripts && bash build-and-deploy.sh --env-file ../config/.env"
```

**Switch to native PowerShell:**
```powershell
cd C:\Users\sanjsingh\Downloads\Studies\AgentCert-Framework\local-custom\scripts
.\build-and-deploy.ps1 -EnvFile "..\config\.env"
```

**Benefits:**
- ✅ No WSL overhead
- ✅ Native Windows path handling
- ✅ Better PowerShell integration
- ✅ Same functionality

---

## Overview
`build-and-deploy.ps1` is a native Windows PowerShell script that provides the same functionality as `build-and-deploy.sh` without requiring WSL or Git Bash.

## Prerequisites

### Required Tools
- **Docker Desktop for Windows** - Latest version
- **kubectl** - Install via: `choco install kubernetes-cli`
- **minikube** - Install via: `choco install minikube`

### Optional Tools
- **mongosh** (MongoDB Shell) - For replica set management
  - Download from: https://www.mongodb.com/try/download/shell
  - Only needed if managing local MongoDB replica sets

## Usage

### Basic Commands

```powershell
# Run from local-custom\scripts directory
cd C:\Users\sanjsingh\Downloads\Studies\AgentCert-Framework\local-custom\scripts

# Basic build and deploy (uses default .env location)
.\build-and-deploy.ps1

# With explicit .env file path (equivalent to --env-file in bash script)
.\build-and-deploy.ps1 -EnvFile "..\config\.env"

# With absolute path
.\build-and-deploy.ps1 -EnvFile "C:\Users\sanjsingh\Downloads\Studies\AgentCert-Framework\local-custom\config\.env"

# Clean everything and rebuild
.\build-and-deploy.ps1 -CleanAll

# Only build (skip deployment)
.\build-and-deploy.ps1 -SkipDeploy

# Only deploy (skip build)
.\build-and-deploy.ps1 -SkipBuild

# Enable debug output
.\build-and-deploy.ps1 -Debug
```

### Get Help
```powershell
Get-Help .\build-and-deploy.ps1 -Full
```

## Key Fixes Applied

### 1. **Docker Build Context** ✅
- **Issue**: GraphQL build context was wrong
- **Fix**: Changed from `chaoscenter\graphql` to `chaoscenter` (matches bash script)

### 2. **Docker Command Execution** ✅
- **Issue**: `Invoke-Expression` with complex strings can fail with quotes
- **Fix**: Use argument array with `& docker $buildArgs`

### 3. **Path Separators for Docker** ✅
- **Issue**: Docker on Windows expects forward slashes in paths
- **Fix**: Convert all paths: `$path -replace '\\', '/'`

### 4. **Minikube Host IP** ✅
- **Issue**: SSH command backtick escaping
- **Fix**: Proper PowerShell escaping: `` `$1 `` and null checks

### 5. **kubectl JSON Patches** ✅
- **Issue**: JSON depth limit and output noise
- **Fix**: Added `-Depth 10` to `ConvertTo-Json` and pipe to `Out-Null`

## Differences from Bash Script

### Command Comparison

| Bash (Ubuntu/WSL) | PowerShell (Windows) |
|-------------------|---------------------|
| `bash build-and-deploy.sh --env-file ../config/.env` | `.\build-and-deploy.ps1 -EnvFile "..\config\.env"` |
| `bash build-and-deploy.sh --clean-all` | `.\build-and-deploy.ps1 -CleanAll` |
| `bash build-and-deploy.sh --skip-build` | `.\build-and-deploy.ps1 -SkipBuild` |
| `bash build-and-deploy.sh --skip-deploy` | `.\build-and-deploy.ps1 -SkipDeploy` |
| `bash build-and-deploy.sh --debug` | `.\build-and-deploy.ps1 -Debug` |

### Technical Differences

| Feature | Bash Script | PowerShell Script |
|---------|-------------|-------------------|
| Path format | `/mnt/c/...` | `C:\...` |
| Commands | Unix tools | Windows native |
| Error handling | `set -euo pipefail` | `$ErrorActionPreference = "Stop"` |
| MongoDB check | `mongosh` (optional) | `mongosh` (optional) |
| Functions | bash functions | PowerShell functions |

## MongoDB Setup

### Windows MongoDB Options

**Option 1: MongoDB in Docker** (Recommended)
```powershell
docker run -d --name mongodb `
  -p 27017:27017 `
  -e MONGO_INITDB_ROOT_USERNAME=root `
  -e MONGO_INITDB_ROOT_PASSWORD=1234 `
  mongo:5 --replSet rs0
```

Then initialize replica set:
```powershell
docker exec mongodb mongosh --username root --password 1234 --authenticationDatabase admin --eval "rs.initiate()"
```

**Option 2: MongoDB Windows Service**
- Download: https://www.mongodb.com/try/download/community
- Install as Windows Service
- Configure replica set manually

## Verification Checklist

After running the script, verify:

- [ ] **Docker images built**
  ```powershell
  docker images | Select-String "litmusportal"
  ```

- [ ] **Images loaded to minikube**
  ```powershell
  minikube image ls | Select-String "litmusportal"
  ```

- [ ] **Pods running**
  ```powershell
  kubectl get pods -n litmus-chaos
  ```

- [ ] **ConfigMap updated**
  ```powershell
  kubectl get configmap litmus-portal-admin-config -n litmus-chaos -o yaml
  ```

- [ ] **Logs clean**
  ```powershell
  kubectl logs -n litmus-chaos deployment/litmusportal-server --tail=50
  ```

## Troubleshooting

### Issue: Docker build fails with path errors
**Solution**: Ensure Docker Desktop is using WSL2 backend (Settings → General → Use WSL 2)

### Issue: minikube image load hangs
**Solution**: Check Docker daemon is running and accessible from minikube:
```powershell
minikube docker-env | Invoke-Expression
docker ps
```

### Issue: kubectl patch fails
**Solution**: Ensure namespace exists:
```powershell
kubectl get namespace litmus-chaos
```

### Issue: MongoDB connection fails from pods
**Solution**: Check minikube host IP is reachable:
```powershell
# Get host IP
$hostIp = minikube ssh "getent hosts host.minikube.internal | awk '{print \`$1}'"
Write-Host "Minikube Host IP: $hostIp"

# Test from pod
kubectl exec -n litmus-chaos deployment/litmusportal-server -- curl -v telnet://${hostIp}:27017
```

## Build Log

Build output is logged to: `C:\Temp\agentcert-build-YYYYMMDD-HHMMSS.log`

View recent build log:
```powershell
Get-ChildItem C:\Temp\agentcert-build-*.log | Sort-Object LastWriteTime -Descending | Select-Object -First 1 | Get-Content -Tail 50
```

## Next Steps After Deployment

1. **Port Forward** (new PowerShell window):
   ```powershell
   .\port-forward.ps1
   ```

2. **Start Frontend** (new PowerShell window):
   ```powershell
   cd ..\..\chaoscenter\web
   npm run dev
   ```

3. **Access Application**:
   - URL: https://localhost:2001
   - Username: `admin`
   - Password: `litmus`

4. **View Logs**:
   ```powershell
   kubectl logs -n litmus-chaos -f deployment/litmusportal-server
   kubectl logs -n litmus-chaos -f deployment/litmusportal-auth-server
   ```

## Support

For issues specific to the PowerShell script, check:
1. PowerShell version: `$PSVersionTable.PSVersion` (requires 5.1+)
2. Execution policy: `Get-ExecutionPolicy` (should allow script execution)
3. Docker Desktop version and settings
4. Minikube driver: `minikube config get driver`
