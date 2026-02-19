#Requires -Version 5.1
<#
.SYNOPSIS
    AgentCert Complete Build and Deploy Script for Windows PowerShell

.DESCRIPTION
    Production-ready build pipeline with:
    - Prerequisite validation
    - Docker image cleanup (local + minikube)
    - Multi-stage Go builds
    - Image loading into minikube
    - Kubernetes deployment
    - Health verification

.PARAMETER CleanAll
    Full cleanup including K8s namespace

.PARAMETER SkipBuild
    Skip building, only deploy

.PARAMETER SkipDeploy
    Only build, don't deploy

.PARAMETER Debug
    Enable debug logging

.PARAMETER EnvFile
    Path to .env file for build args (default: local-custom/config/.env)

.EXAMPLE
    .\build-and-deploy.ps1
    .\build-and-deploy.ps1 -CleanAll
    .\build-and-deploy.ps1 -EnvFile "C:\path\to\.env"
#>

[CmdletBinding()]
param(
    [switch]$CleanAll,
    [switch]$SkipBuild,
    [switch]$SkipDeploy,
    [switch]$Debug,
    [string]$EnvFile = ""
)

$ErrorActionPreference = "Stop"

# ============================================================================
# CONFIGURATION
# ============================================================================
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = (Resolve-Path "$ScriptDir\..\..").Path
$Namespace = "litmus-chaos"
$DockerRegistry = "litmuschaos"
$LocalEnvFile = Join-Path $ProjectRoot "local-custom\config\.env"

# Use a unique tag by default to avoid stale images
$UniqueTag = $true
if ($UniqueTag) {
    $ImageTag = "ci-$(Get-Date -Format 'yyyyMMddHHmmss')"
} else {
    $ImageTag = "ci"
}

$AuthServerImage = "${DockerRegistry}/litmusportal-auth-server:${ImageTag}"
$GraphQLServerImage = "${DockerRegistry}/litmusportal-server:${ImageTag}"
$BuildLogFile = "C:\Temp\agentcert-build-$(Get-Date -Format 'yyyyMMdd-HHmmss').log"

# Override LOCAL_ENV_FILE if -EnvFile parameter is provided
if ($EnvFile) {
    $LocalEnvFile = $EnvFile
    if (-not (Test-Path $LocalEnvFile)) {
        Write-Host "ERROR: Specified .env file not found: $LocalEnvFile" -ForegroundColor Red
        exit 1
    }
}

if ($Debug) {
    $DebugPreference = "Continue"
}

# Create temp directory if it doesn't exist
if (-not (Test-Path "C:\Temp")) {
    New-Item -ItemType Directory -Path "C:\Temp" | Out-Null
}

# ============================================================================
# OUTPUT FUNCTIONS
# ============================================================================
function Write-Header {
    param([string]$Message)
    Write-Host ""
    Write-Host "╔════════════════════════════════════════════════════════════════╗" -ForegroundColor Blue
    Write-Host "║ $Message" -ForegroundColor Cyan
    Write-Host "╚════════════════════════════════════════════════════════════════╝" -ForegroundColor Blue
    Write-Host ""
}

function Write-Section {
    param([string]$Message)
    Write-Host "→ $Message" -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host "✓ $Message" -ForegroundColor Green
}

function Write-Info {
    param([string]$Message)
    Write-Host "ℹ $Message" -ForegroundColor Blue
}

function Write-Warning {
    param([string]$Message)
    Write-Host "⚠ $Message" -ForegroundColor Yellow
}

function Write-ErrorMsg {
    param([string]$Message)
    Write-Host "✗ $Message" -ForegroundColor Red
}

function Write-LogFile {
    param([string]$Message)
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    "[$timestamp] $Message" | Out-File -FilePath $BuildLogFile -Append
}

# ============================================================================
# ENV HELPERS
# ============================================================================
function Get-EnvValue {
    param(
        [string]$Key,
        [string]$FilePath
    )
    
    if (-not (Test-Path $FilePath)) {
        return ""
    }
    
    $content = Get-Content $FilePath -Raw
    if ($content -match "(?m)^$Key\s*=\s*(.+)$") {
        $value = $Matches[1].Trim()
        # Remove quotes
        $value = $value -replace '^["'']|["'']$', ''
        return $value
    }
    return ""
}

function Get-MinikubeHostIP {
    $hostIp = minikube ssh "getent hosts host.minikube.internal | awk '{print \`$1}'" 2>$null | Select-Object -First 1
    if ($hostIp) {
        return $hostIp.Trim()
    }
    return ""
}

function Sync-AzureEnv {
    if (-not (Test-Path $LocalEnvFile)) {
        Write-Warning ".env not found at $LocalEnvFile; skipping Azure env sync"
        return
    }

    Write-Section "Syncing Azure OpenAI env from .env to cluster"
    
    $endpoint = Get-EnvValue "AZURE_OPENAI_ENDPOINT" $LocalEnvFile
    $deployment = Get-EnvValue "AZURE_OPENAI_DEPLOYMENT" $LocalEnvFile
    $apiVersion = Get-EnvValue "AZURE_OPENAI_API_VERSION" $LocalEnvFile
    $embedding = Get-EnvValue "AZURE_OPENAI_EMBEDDING_DEPLOYMENT" $LocalEnvFile
    $key = Get-EnvValue "AZURE_OPENAI_KEY" $LocalEnvFile

    # Patch ConfigMap
    if ($endpoint -or $deployment -or $apiVersion -or $embedding) {
        $cmData = @{}
        if ($endpoint) { $cmData["AZURE_OPENAI_ENDPOINT"] = $endpoint }
        if ($deployment) { $cmData["AZURE_OPENAI_DEPLOYMENT"] = $deployment }
        if ($apiVersion) { $cmData["AZURE_OPENAI_API_VERSION"] = $apiVersion }
        if ($embedding) { $cmData["AZURE_OPENAI_EMBEDDING_DEPLOYMENT"] = $embedding }
        
        $cmPatch = (@{ data = $cmData } | ConvertTo-Json -Compress -Depth 10)
        kubectl -n $Namespace patch configmap litmus-portal-admin-config --type merge -p $cmPatch 2>$null | Out-Null
    }

    # Patch Secret
    if ($key) {
        $secPatch = (@{ stringData = @{ AZURE_OPENAI_KEY = $key } } | ConvertTo-Json -Compress -Depth 10)
        kubectl -n $Namespace patch secret litmus-portal-admin-secret --type merge -p $secPatch 2>$null | Out-Null
    }
}

function Sync-LangfuseEnv {
    if (-not (Test-Path $LocalEnvFile)) {
        Write-Warning ".env not found at $LocalEnvFile; skipping Langfuse env sync"
        return
    }

    Write-Section "Syncing Langfuse env from .env to cluster"
    
    $host = Get-EnvValue "LANGFUSE_HOST" $LocalEnvFile
    $publicKey = Get-EnvValue "LANGFUSE_PUBLIC_KEY" $LocalEnvFile
    $secretKey = Get-EnvValue "LANGFUSE_SECRET_KEY" $LocalEnvFile
    $orgId = Get-EnvValue "LANGFUSE_ORG_ID" $LocalEnvFile
    $projectId = Get-EnvValue "LANGFUSE_PROJECT_ID" $LocalEnvFile

    # Patch ConfigMap
    if ($host -or $orgId -or $projectId) {
        $cmData = @{}
        if ($host) { $cmData["LANGFUSE_HOST"] = $host }
        if ($orgId) { $cmData["LANGFUSE_ORG_ID"] = $orgId }
        if ($projectId) { $cmData["LANGFUSE_PROJECT_ID"] = $projectId }
        
        $cmPatch = (@{ data = $cmData } | ConvertTo-Json -Compress -Depth 10)
        kubectl -n $Namespace patch configmap litmus-portal-admin-config --type merge -p $cmPatch 2>$null | Out-Null
    }

    # Patch Secret
    if ($secretKey -or $publicKey) {
        $secData = @{}
        if ($secretKey) { $secData["LANGFUSE_SECRET_KEY"] = $secretKey }
        if ($publicKey) { $secData["LANGFUSE_PUBLIC_KEY"] = $publicKey }
        
        $secPatch = (@{ stringData = $secData } | ConvertTo-Json -Compress -Depth 10)
        kubectl -n $Namespace patch secret litmus-portal-admin-secret --type merge -p $secPatch 2>$null | Out-Null
    }
}

function Sync-MongoDBConnection {
    $hostIp = Get-MinikubeHostIP
    if (-not $hostIp) {
        Write-Warning "Unable to resolve minikube host IP. Skipping MongoDB connection sync"
        return
    }

    Write-Section "Syncing MongoDB connection to cluster"
    $dbServer = "mongodb://root:1234@${hostIp}:27017/admin"
    $cmPatch = (@{ data = @{ DB_SERVER = $dbServer } } | ConvertTo-Json -Compress -Depth 10)
    kubectl -n $Namespace patch configmap litmus-portal-admin-config --type merge -p $cmPatch 2>$null | Out-Null
    Write-Success "MongoDB connection updated to use host IP: $hostIp"
}

# ============================================================================
# PREREQUISITE CHECK
# ============================================================================
function Test-Prerequisites {
    Write-Header "Checking Prerequisites"
    
    $ok = $true
    
    # Docker
    Write-Section "Checking Docker..."
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        Write-ErrorMsg "Docker not found"
        $ok = $false
    } else {
        try {
            docker ps 2>$null | Out-Null
            Write-Success "Docker: $(docker --version)"
        } catch {
            Write-ErrorMsg "Docker daemon not running"
            $ok = $false
        }
    }
    
    # kubectl
    Write-Section "Checking kubectl..."
    if (-not (Get-Command kubectl -ErrorAction SilentlyContinue)) {
        Write-ErrorMsg "kubectl not found"
        $ok = $false
    } else {
        Write-Success "kubectl ready"
    }
    
    # minikube
    Write-Section "Checking minikube..."
    if (-not (Get-Command minikube -ErrorAction SilentlyContinue)) {
        Write-ErrorMsg "minikube not found"
        $ok = $false
    } else {
        $status = minikube status 2>$null
        if ($LASTEXITCODE -ne 0) {
            Write-Warning "minikube not running. Starting..."
            minikube start
            if ($LASTEXITCODE -ne 0) {
                Write-ErrorMsg "Failed to start minikube"
                $ok = $false
            }
        }
        Write-Success "minikube running"
    }
    
    if (-not $ok) {
        Write-Host ""
        Write-ErrorMsg "Prerequisites missing"
        exit 1
    }
    Write-Success "All prerequisites OK"
}

# ============================================================================
# MONGODB REPLICA SET FIX
# ============================================================================
function Ensure-MongoReplSetHost {
    Write-Header "Ensuring MongoDB Replica Set Host"

    if (-not (Get-Command mongosh -ErrorAction SilentlyContinue)) {
        Write-Warning "mongosh not found. Skipping MongoDB replica set check."
        return
    }

    $hostIp = Get-MinikubeHostIP
    if (-not $hostIp) {
        Write-Warning "Unable to resolve host.minikube.internal. Skipping replica set update."
        return
    }

    Write-Info "Minikube host IP: $hostIp"

    # Check current replica set host
    $currentHost = mongosh --quiet --username root --password 1234 --authenticationDatabase admin --eval "try { cfg=rs.conf(); print(cfg.members[0].host); } catch(e) { print('NOT_INIT'); }" 2>$null | Select-Object -Last 1
    $currentHost = $currentHost.Trim()

    if ($currentHost -eq "NOT_INIT") {
        Write-Warning "Replica set not initialized. Initializing with host ${hostIp}:27017..."
        mongosh --username root --password 1234 --authenticationDatabase admin --eval "rs.initiate({ _id: 'rs0', members: [ { _id: 0, host: '${hostIp}:27017' } ] })" 2>$null
        Write-Success "Replica set initialized"
        return
    }

    if ($currentHost -ne "${hostIp}:27017") {
        Write-Warning "Replica set host is '$currentHost'. Updating to '${hostIp}:27017'..."
        mongosh --username root --password 1234 --authenticationDatabase admin --eval "cfg=rs.conf(); cfg.members[0].host='${hostIp}:27017'; rs.reconfig(cfg, {force:true})" 2>$null
        Write-Success "Replica set host updated"
    } else {
        Write-Success "Replica set host already correct"
    }
}

# ============================================================================
# CLEANUP
# ============================================================================
function Remove-DockerImages {
    Write-Header "Cleaning Docker Images"
    
    docker rmi -f $AuthServerImage 2>$null
    docker rmi -f $GraphQLServerImage 2>$null
    
    docker image prune -f --filter "dangling=true" 2>$null | Out-Null
    
    Write-Section "Removing old image versions (keeping latest 2)..."
    
    # Clean old auth server images
    $authImages = docker images "litmuschaos/litmusportal-auth-server" --format "{{.Tag}}" | Where-Object { $_ -match "^ci-" } | Sort-Object
    if ($authImages -and $authImages.Count -gt 2) {
        $authImages | Select-Object -First ($authImages.Count - 2) | ForEach-Object {
            Write-Info "Removing: litmuschaos/litmusportal-auth-server:$_"
            docker rmi -f "litmuschaos/litmusportal-auth-server:$_" 2>$null
        }
    }
    
    # Clean old graphql server images
    $gqlImages = docker images "litmuschaos/litmusportal-server" --format "{{.Tag}}" | Where-Object { $_ -match "^ci-" } | Sort-Object
    if ($gqlImages -and $gqlImages.Count -gt 2) {
        $gqlImages | Select-Object -First ($gqlImages.Count - 2) | ForEach-Object {
            Write-Info "Removing: litmuschaos/litmusportal-server:$_"
            docker rmi -f "litmuschaos/litmusportal-server:$_" 2>$null
        }
    }
    
    Write-Success "Docker cleanup done"
}

function Remove-MinikubeImages {
    Write-Header "Cleaning Minikube Images"
    
    minikube image rm $AuthServerImage 2>$null
    minikube image rm $GraphQLServerImage 2>$null
    
    Write-Success "Minikube cleanup done"
}

function Remove-K8sNamespace {
    Write-Header "Cleaning Kubernetes"
    $ns = kubectl get namespace $Namespace 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Info "Deleting namespace $Namespace..."
        kubectl delete namespace $Namespace
        Write-Info "Waiting for deletion..."
        Start-Sleep -Seconds 30
        Write-Success "Namespace deleted"
    }
}

# ============================================================================
# BUILD
# ============================================================================
function Build-DockerImage {
    param(
        [string]$Dockerfile,
        [string]$Context,
        [string]$Image,
        [string]$Name
    )
    
    Write-Section "Building $Name..."
    Write-Info "Context: $Context"
    
    if (-not (Test-Path $Context)) {
        Write-ErrorMsg "Context not found: $Context"
        return $false
    }
    
    # Extract Langfuse credentials
    $langfuseHost = Get-EnvValue "LANGFUSE_HOST" $LocalEnvFile
    $langfusePublicKey = Get-EnvValue "LANGFUSE_PUBLIC_KEY" $LocalEnvFile
    $langfuseSecretKey = Get-EnvValue "LANGFUSE_SECRET_KEY" $LocalEnvFile
    $langfuseOrgId = Get-EnvValue "LANGFUSE_ORG_ID" $LocalEnvFile
    $langfuseProjectId = Get-EnvValue "LANGFUSE_PROJECT_ID" $LocalEnvFile
    
    # Convert Windows paths to forward slashes for Docker
    $dockerfilePath = $Dockerfile -replace '\\', '/'
    $contextPath = $Context -replace '\\', '/'
    
    $start = Get-Date
    
    # Build arguments array
    $buildArgs = @(
        'build',
        '-t', $Image,
        '-f', $dockerfilePath,
        '--build-arg', 'TARGETOS=linux',
        '--build-arg', 'TARGETARCH=amd64',
        '--build-arg', "LANGFUSE_HOST=$langfuseHost",
        '--build-arg', "LANGFUSE_PUBLIC_KEY=$langfusePublicKey",
        '--build-arg', "LANGFUSE_SECRET_KEY=$langfuseSecretKey",
        '--build-arg', "LANGFUSE_ORG_ID=$langfuseOrgId",
        '--build-arg', "LANGFUSE_PROJECT_ID=$langfuseProjectId",
        $contextPath
    )
    
    try {
        & docker $buildArgs 2>&1 | Tee-Object -FilePath $BuildLogFile -Append
        if ($LASTEXITCODE -ne 0) {
            Write-ErrorMsg "Build failed"
            return $false
        }
        $elapsed = ((Get-Date) - $start).TotalSeconds
        Write-Success "$Name built ($([math]::Round($elapsed))s)"
        return $true
    } catch {
        Write-ErrorMsg "Build failed: $_"
        return $false
    }
}

function Build-AllImages {
    Write-Header "Building Docker Images"
    Write-Host ""
    
    $success = Build-DockerImage `
        -Dockerfile "$ProjectRoot\chaoscenter\authentication\Dockerfile" `
        -Context "$ProjectRoot\chaoscenter\authentication" `
        -Image $AuthServerImage `
        -Name "Auth Server"
    
    if (-not $success) { exit 1 }
    
    Write-Host ""
    
    $success = Build-DockerImage `
        -Dockerfile "$ProjectRoot\chaoscenter\graphql\server\Dockerfile" `
        -Context "$ProjectRoot\chaoscenter" `
        -Image $GraphQLServerImage `
        -Name "GraphQL Server"
    
    if (-not $success) { exit 1 }
    
    Write-Success "All images built"
}

function Load-ToMinikube {
    Write-Header "Loading Images to Minikube"
    
    Write-Info "Loading Auth Server image: $AuthServerImage"
    minikube image load $AuthServerImage
    if ($LASTEXITCODE -ne 0) {
        Write-ErrorMsg "Load failed"
        return $false
    }
    Write-Success "Auth Server loaded"
    
    Write-Info "Loading GraphQL Server image: $GraphQLServerImage"
    minikube image load $GraphQLServerImage
    if ($LASTEXITCODE -ne 0) {
        Write-ErrorMsg "Load failed"
        return $false
    }
    Write-Success "GraphQL Server loaded"
    return $true
}

# ============================================================================
# DEPLOY
# ============================================================================
function New-K8sNamespace {
    Write-Header "Creating Kubernetes Namespace"
    $ns = kubectl get namespace $Namespace 2>$null
    if ($LASTEXITCODE -ne 0) {
        kubectl create namespace $Namespace
        Start-Sleep -Seconds 2
    }
    Write-Success "Namespace ready"
}

function Deploy-Manifest {
    Write-Header "Deploying Manifest"
    $manifest = Join-Path $ProjectRoot "local-custom\k8s\litmus-installation.yaml"
    
    if (-not (Test-Path $manifest)) {
        Write-ErrorMsg "Manifest not found"
        return $false
    }
    
    Write-Info "Applying manifest..."
    kubectl apply -f $manifest
    Start-Sleep -Seconds 3

    Sync-AzureEnv
    Sync-LangfuseEnv
    Sync-MongoDBConnection
    
    # Apply Litmus configuration fixes
    Write-Info "Applying Litmus configuration fixes..."
    $fixScript = Join-Path $ScriptDir "fix-litmus-config.sh"
    if (Test-Path $fixScript) {
        if (Get-Command wsl -ErrorAction SilentlyContinue) {
            wsl bash $fixScript $Namespace 2>$null
        } elseif (Get-Command bash -ErrorAction SilentlyContinue) {
            bash $fixScript $Namespace 2>$null
        }
    }
    
    Write-Info "Updating Auth Server deployment with image: $AuthServerImage"
    kubectl set image deployment/litmusportal-auth-server auth-server=$AuthServerImage -n $Namespace --record
    
    Write-Info "Updating GraphQL Server deployment with image: $GraphQLServerImage"
    kubectl set image deployment/litmusportal-server graphql-server=$GraphQLServerImage -n $Namespace --record
    
    Write-Info "Restarting deployments..."
    kubectl rollout restart deployment/litmusportal-auth-server -n $Namespace
    kubectl rollout restart deployment/litmusportal-server -n $Namespace
    
    Write-Info "Waiting 20 seconds for pods to start..."
    Start-Sleep -Seconds 20
    
    Write-Info "Current pods status:"
    kubectl get pods -n $Namespace -o wide
    
    return $true
}

function Test-Pods {
    Write-Header "Verifying Deployment"
    $maxAttempts = 30
    $attempt = 0
    
    while ($attempt -lt $maxAttempts) {
        $attempt++
        
        $auth = kubectl get pods -n $Namespace -l component=litmusportal-auth-server -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>$null
        $gql = kubectl get pods -n $Namespace -l component=litmusportal-server -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>$null
        
        if ($auth -eq "True" -and $gql -eq "True") {
            Write-Success "All pods ready!"
            kubectl get pods -n $Namespace -o wide
            return $true
        }
        
        Write-Info "Waiting... ($attempt/$maxAttempts)"
        Start-Sleep -Seconds 5
    }
    
    Write-Warning "Timeout waiting for pods. Current status:"
    kubectl get pods -n $Namespace -o wide
    return $false
}

# ============================================================================
# SUMMARY
# ============================================================================
function Show-Info {
    Write-Header "Deployment Summary"
    Write-Host "Build log: $BuildLogFile" -ForegroundColor Cyan
    Write-Host ""
    kubectl get all -n $Namespace 2>$null
}

function Show-NextSteps {
    Write-Header "Next Steps"
    Write-Host "1. Port Forward (new terminal):" -ForegroundColor Yellow
    Write-Host "   .\port-forward.ps1"
    Write-Host "   # or bash: bash $ProjectRoot\local-custom\scripts\port-forward.sh"
    Write-Host ""
    Write-Host "2. Frontend (new terminal):" -ForegroundColor Yellow
    Write-Host "   .\start-web.ps1"
    Write-Host "   # or: cd $ProjectRoot\chaoscenter\web && npm run dev"
    Write-Host ""
    Write-Host "3. Access:" -ForegroundColor Yellow
    Write-Host "   https://localhost:2001"
    Write-Host "   Username: admin / Password: litmus"
    Write-Host ""
    Write-Host "4. Logs:" -ForegroundColor Yellow
    Write-Host "   kubectl logs -n $Namespace -f deployment/litmusportal-auth-server"
    Write-Host "   kubectl logs -n $Namespace -f deployment/litmusportal-server"
    Write-Host ""
}

# ============================================================================
# MAIN
# ============================================================================
function Main {
    Write-Header "AgentCert Build & Deploy Pipeline"
    Write-Info "Using .env file: $LocalEnvFile"
    Write-LogFile "========== Build Started =========="
    
    Test-Prerequisites
    Ensure-MongoReplSetHost
    
    if ($CleanAll) {
        Remove-K8sNamespace
    }
    
    Remove-DockerImages
    Remove-MinikubeImages
    
    if (-not $SkipBuild) {
        Build-AllImages
        Load-ToMinikube
    } else {
        Write-Warning "Skipping build"
    }
    
    if (-not $SkipDeploy) {
        New-K8sNamespace
        Deploy-Manifest
        Sync-LangfuseEnv
        Sync-AzureEnv
        kubectl rollout restart deployment/litmusportal-server -n $Namespace
        Test-Pods
    } else {
        Write-Warning "Skipping deploy"
    }
    
    Show-Info
    Show-NextSteps
    
    Write-Header "✓ Pipeline Complete!"
    Write-LogFile "========== Build Completed =========="
}

# Run main
Main
