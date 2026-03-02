# ============================================================================
# AgentCert Unified Startup Script
# ============================================================================
# This script starts all AgentCert components in the correct order with
# health checks to ensure stability.
# ============================================================================

param(
    [switch]$SkipMongo,
    [switch]$Verbose
)

$ErrorActionPreference = "Stop"
$ProjectRoot = $PSScriptRoot

# Colors for output
function Write-Status { param($msg) Write-Host "[STATUS] " -ForegroundColor Cyan -NoNewline; Write-Host $msg }
function Write-Success { param($msg) Write-Host "[  OK  ] " -ForegroundColor Green -NoNewline; Write-Host $msg }
function Write-Fail { param($msg) Write-Host "[FAILED] " -ForegroundColor Red -NoNewline; Write-Host $msg }
function Write-Wait { param($msg) Write-Host "[WAIT  ] " -ForegroundColor Yellow -NoNewline; Write-Host $msg }

Write-Host ""
Write-Host "============================================" -ForegroundColor Magenta
Write-Host "       AgentCert Startup Script            " -ForegroundColor Magenta
Write-Host "============================================" -ForegroundColor Magenta
Write-Host ""

# ============================================================================
# Step 1: Check and clean up any zombie processes on required ports
# ============================================================================
Write-Status "Checking for port conflicts..."

$ports = @(2001, 3030, 8080)
$portsInUse = @()

foreach ($port in $ports) {
    $process = Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue | 
               Where-Object { $_.State -eq 'Listen' } | 
               Select-Object -First 1

    if ($process) {
        $proc = Get-Process -Id $process.OwningProcess -ErrorAction SilentlyContinue
        if ($proc) {
            $portsInUse += @{ Port = $port; Process = $proc.ProcessName; PID = $proc.Id }
        }
    }
}

if ($portsInUse.Count -gt 0) {
    Write-Fail "The following ports are already in use:"
    foreach ($p in $portsInUse) {
        Write-Host "         Port $($p.Port): $($p.Process) (PID: $($p.PID))" -ForegroundColor Yellow
    }
    Write-Host ""
    $response = Read-Host "Do you want to terminate these processes? (Y/n)"
    if ($response -eq "" -or $response -match "^[Yy]") {
        foreach ($p in $portsInUse) {
            try {
                Stop-Process -Id $p.PID -Force -ErrorAction Stop
                Write-Success "Terminated $($p.Process) on port $($p.Port)"
            } catch {
                Write-Fail "Could not terminate process on port $($p.Port): $_"
                exit 1
            }
        }
        # Wait for ports to be released
        Start-Sleep -Seconds 2
    } else {
        Write-Fail "Cannot continue with ports in use. Exiting."
        exit 1
    }
} else {
    Write-Success "No port conflicts detected"
}

# ============================================================================
# Step 2: Check MongoDB
# ============================================================================
Write-Status "Checking MongoDB..."

$mongoRunning = $false
try {
    $mongoContainer = docker ps --filter "publish=27017" --format "{{.Names}}" 2>$null
    if ($mongoContainer) {
        Write-Success "MongoDB is running in container: $mongoContainer"
        $mongoRunning = $true
    }
} catch {}

if (-not $mongoRunning -and -not $SkipMongo) {
    Write-Wait "Starting MongoDB container..."
    try {
        # Check if m3 container exists but stopped
        $existingContainer = docker ps -a --filter "name=m3" --format "{{.Names}}" 2>$null
        if ($existingContainer -eq "m3") {
            docker start m3 | Out-Null
            Write-Success "Started existing MongoDB container 'm3'"
        } else {
            # Start a new container
            docker run -d --name agentcert-mongo -p 27017:27017 mongo:4.2 | Out-Null
            Write-Success "Started new MongoDB container 'agentcert-mongo'"
        }
        $mongoRunning = $true
    } catch {
        Write-Fail "Could not start MongoDB: $_"
        exit 1
    }
}

# Wait for MongoDB to be ready
if ($mongoRunning) {
    Write-Wait "Waiting for MongoDB to accept connections..."
    $maxRetries = 10
    $retry = 0
    while ($retry -lt $maxRetries) {
        try {
            $result = docker exec m3 mongo --eval "db.adminCommand('ping')" 2>$null
            if ($LASTEXITCODE -eq 0) {
                Write-Success "MongoDB is ready"
                break
            }
        } catch {}
        $retry++
        Start-Sleep -Seconds 1
    }
    if ($retry -eq $maxRetries) {
        Write-Fail "MongoDB did not become ready in time"
        exit 1
    }
}

# ============================================================================
# Step 3: Create launcher scripts with embedded environment variables
# ============================================================================
Write-Status "Preparing service launchers..."

# Common environment variables
$commonEnv = @"
`$env:VERSION = "3.0.0"
`$env:INFRA_DEPLOYMENTS = "false"
`$env:DB_SERVER = "mongodb://localhost:27017"
`$env:JWT_SECRET = "litmus-portal@123"
`$env:DB_USER = "admin"
`$env:DB_PASSWORD = "1234"
`$env:SELF_AGENT = "false"
`$env:INFRA_COMPATIBLE_VERSIONS = '["3.0.0"]'
`$env:ALLOWED_ORIGINS = '^(http://|https://|)(localhost|host\.docker\.internal|host\.minikube\.internal)(:[0-9]+|)'
`$env:SKIP_SSL_VERIFY = "true"
`$env:ENABLE_GQL_INTROSPECTION = "true"
`$env:INFRA_SCOPE = "cluster"
`$env:ENABLE_INTERNAL_TLS = "false"
`$env:DEFAULT_HUB_GIT_URL = "https://github.com/sharmadeep2/chaos-charts"
`$env:DEFAULT_HUB_BRANCH_NAME = "master"
`$env:SUBSCRIBER_IMAGE = "litmuschaos/litmusportal-subscriber:3.0.0"
`$env:EVENT_TRACKER_IMAGE = "litmuschaos/litmusportal-event-tracker:3.0.0"
`$env:ARGO_WORKFLOW_CONTROLLER_IMAGE = "litmuschaos/workflow-controller:v3.3.1"
`$env:ARGO_WORKFLOW_EXECUTOR_IMAGE = "litmuschaos/argoexec:v3.3.1"
`$env:LITMUS_CHAOS_OPERATOR_IMAGE = "litmuschaos/chaos-operator:3.0.0"
`$env:LITMUS_CHAOS_RUNNER_IMAGE = "litmuschaos/chaos-runner:3.0.0"
`$env:LITMUS_CHAOS_EXPORTER_IMAGE = "litmuschaos/chaos-exporter:3.0.0"
`$env:CONTAINER_RUNTIME_EXECUTOR = "k8sapi"
`$env:WORKFLOW_HELPER_IMAGE_VERSION = "3.0.0"
"@

# Auth launcher script
$authLauncherPath = Join-Path $ProjectRoot ".auth-launcher.ps1"
$authPath = Join-Path $ProjectRoot "chaoscenter\authentication"
$authLauncher = @"
$commonEnv
`$env:ADMIN_USERNAME = "admin"
`$env:ADMIN_PASSWORD = "litmus"
`$env:REST_PORT = "3000"
`$env:GRPC_PORT = "3030"
Set-Location "$authPath"
.\auth.exe
"@
$authLauncher | Out-File -FilePath $authLauncherPath -Encoding UTF8 -Force

# GraphQL launcher script
$gqlLauncherPath = Join-Path $ProjectRoot ".graphql-launcher.ps1"
$gqlPath = Join-Path $ProjectRoot "chaoscenter\graphql\server"
$gqlLauncher = @"
$commonEnv
`$env:LITMUS_AUTH_GRPC_ENDPOINT = "localhost"
`$env:LITMUS_AUTH_GRPC_PORT = "3030"
Set-Location "$gqlPath"
.\server.exe
"@
$gqlLauncher | Out-File -FilePath $gqlLauncherPath -Encoding UTF8 -Force

Write-Success "Service launchers prepared"

# ============================================================================
# Step 4: Start Authentication Service
# ============================================================================
Write-Status "Starting Authentication Service..."

$authExe = Join-Path $authPath "auth.exe"

if (-not (Test-Path $authExe)) {
    Write-Fail "auth.exe not found at $authExe"
    exit 1
}

$authProcess = Start-Process -FilePath "powershell" -ArgumentList "-ExecutionPolicy", "Bypass", "-File", $authLauncherPath -PassThru -WindowStyle Minimized
$script:AuthPID = $authProcess.Id

# Save PID for shutdown script
$authProcess.Id | Out-File -FilePath (Join-Path $ProjectRoot ".agentcert-auth.pid") -Force

# Wait for auth service to be ready
Write-Wait "Waiting for Auth Service to be ready on port 3030..."
$maxRetries = 30
$retry = 0
while ($retry -lt $maxRetries) {
    $listening = Get-NetTCPConnection -LocalPort 3030 -State Listen -ErrorAction SilentlyContinue
    if ($listening) {
        Write-Success "Authentication Service is ready (PID: $($authProcess.Id))"
        break
    }
    $retry++
    Start-Sleep -Seconds 1
}
if ($retry -eq $maxRetries) {
    Write-Fail "Authentication Service did not start in time"
    exit 1
}

# ============================================================================
# Step 5: Start GraphQL Server
# ============================================================================
Write-Status "Starting GraphQL Server..."

$gqlExe = Join-Path $gqlPath "server.exe"

if (-not (Test-Path $gqlExe)) {
    Write-Fail "server.exe not found at $gqlExe"
    exit 1
}

$gqlProcess = Start-Process -FilePath "powershell" -ArgumentList "-ExecutionPolicy", "Bypass", "-File", $gqlLauncherPath -PassThru -WindowStyle Minimized
$script:GqlPID = $gqlProcess.Id

# Save PID for shutdown script
$gqlProcess.Id | Out-File -FilePath (Join-Path $ProjectRoot ".agentcert-graphql.pid") -Force

# Wait for GraphQL service to be ready
Write-Wait "Waiting for GraphQL Server to be ready on port 8080..."
$maxRetries = 30
$retry = 0
while ($retry -lt $maxRetries) {
    $listening = Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue
    if ($listening) {
        Write-Success "GraphQL Server is ready (PID: $($gqlProcess.Id))"
        break
    }
    $retry++
    Start-Sleep -Seconds 1
}
if ($retry -eq $maxRetries) {
    Write-Fail "GraphQL Server did not start in time"
    Stop-Process -Id $authProcess.Id -Force -ErrorAction SilentlyContinue
    exit 1
}

# ============================================================================
# Step 6: Start Frontend
# ============================================================================
Write-Status "Starting Frontend..."

$webPath = Join-Path $ProjectRoot "chaoscenter\web"

if (-not (Test-Path (Join-Path $webPath "package.json"))) {
    Write-Fail "package.json not found in $webPath"
    exit 1
}

# Start yarn dev in a new PowerShell window
$frontendProcess = Start-Process -FilePath "powershell" -ArgumentList "-NoExit", "-Command", "Set-Location '$webPath'; yarn dev" -PassThru -WindowStyle Normal

# Save PID for shutdown script
$frontendProcess.Id | Out-File -FilePath (Join-Path $ProjectRoot ".agentcert-frontend.pid") -Force

# Wait for frontend to be ready
Write-Wait "Waiting for Frontend to be ready on port 2001..."
$maxRetries = 60
$retry = 0
while ($retry -lt $maxRetries) {
    $listening = Get-NetTCPConnection -LocalPort 2001 -State Listen -ErrorAction SilentlyContinue
    if ($listening) {
        Write-Success "Frontend is ready (PID: $($frontendProcess.Id))"
        break
    }
    $retry++
    Start-Sleep -Seconds 1
}
if ($retry -eq $maxRetries) {
    Write-Fail "Frontend did not start in time"
    # Don't exit, frontend may still be building
    Write-Host "         Frontend may still be building. Check the terminal window." -ForegroundColor Yellow
}

# ============================================================================
# Summary
# ============================================================================
Write-Host ""
Write-Host "============================================" -ForegroundColor Green
Write-Host "       AgentCert Started Successfully!     " -ForegroundColor Green
Write-Host "============================================" -ForegroundColor Green
Write-Host ""
Write-Host "Services:" -ForegroundColor White
Write-Host "  - MongoDB:        localhost:27017" -ForegroundColor Gray
Write-Host "  - Auth Service:   localhost:3030 (gRPC)" -ForegroundColor Gray
Write-Host "  - GraphQL Server: http://localhost:8080" -ForegroundColor Gray
Write-Host "  - Frontend:       https://localhost:2001" -ForegroundColor Gray
Write-Host ""
Write-Host "Default Credentials:" -ForegroundColor White
Write-Host "  - Username: admin" -ForegroundColor Gray
Write-Host "  - Password: litmus" -ForegroundColor Gray
Write-Host ""
Write-Host "To stop all services, run: .\stop-agentcert.ps1" -ForegroundColor Yellow
Write-Host ""

# Save all PIDs to a combined file for easy shutdown
@{
    Auth = $authProcess.Id
    GraphQL = $gqlProcess.Id
    Frontend = $frontendProcess.Id
    StartTime = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
} | ConvertTo-Json | Out-File -FilePath (Join-Path $ProjectRoot ".agentcert-pids.json") -Force
