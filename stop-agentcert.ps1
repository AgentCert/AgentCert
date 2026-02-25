# ============================================================================
# AgentCert Graceful Shutdown Script
# ============================================================================
# This script gracefully terminates all AgentCert processes and cleans up
# resources to ensure clean restarts.
# ============================================================================

param(
    [switch]$Force,
    [switch]$KeepMongo,
    [switch]$Verbose
)

$ErrorActionPreference = "Continue"
$ProjectRoot = $PSScriptRoot

# Colors for output
function Write-Status { param($msg) Write-Host "[STATUS] " -ForegroundColor Cyan -NoNewline; Write-Host $msg }
function Write-Success { param($msg) Write-Host "[  OK  ] " -ForegroundColor Green -NoNewline; Write-Host $msg }
function Write-Fail { param($msg) Write-Host "[FAILED] " -ForegroundColor Red -NoNewline; Write-Host $msg }
function Write-Wait { param($msg) Write-Host "[WAIT  ] " -ForegroundColor Yellow -NoNewline; Write-Host $msg }
function Write-Skip { param($msg) Write-Host "[ SKIP ] " -ForegroundColor DarkGray -NoNewline; Write-Host $msg }

Write-Host ""
Write-Host "============================================" -ForegroundColor Magenta
Write-Host "       AgentCert Shutdown Script           " -ForegroundColor Magenta
Write-Host "============================================" -ForegroundColor Magenta
Write-Host ""

# ============================================================================
# Function to gracefully stop a process
# ============================================================================
function Stop-ServiceGracefully {
    param(
        [int]$ProcessId,
        [string]$ServiceName,
        [int]$TimeoutSeconds = 10
    )

    if ($ProcessId -eq 0) {
        Write-Skip "${ServiceName}: No PID found"
        return $true
    }

    $process = Get-Process -Id $ProcessId -ErrorAction SilentlyContinue
    if (-not $process) {
        Write-Skip "${ServiceName}: Process not running (PID: $ProcessId)"
        return $true
    }

    Write-Wait "Stopping $ServiceName (PID: $ProcessId)..."

    # Try graceful shutdown first
    try {
        # Send close signal
        $process.CloseMainWindow() | Out-Null
        
        # Wait for graceful exit
        $waited = 0
        while (-not $process.HasExited -and $waited -lt $TimeoutSeconds) {
            Start-Sleep -Milliseconds 500
            $waited += 0.5
        }

        if ($process.HasExited) {
            Write-Success "$ServiceName stopped gracefully"
            return $true
        }

        # Force kill if graceful shutdown failed
        Write-Wait "$ServiceName did not stop gracefully, forcing termination..."
        Stop-Process -Id $ProcessId -Force -ErrorAction Stop
        
        # Wait a bit more
        Start-Sleep -Seconds 1
        
        $process = Get-Process -Id $ProcessId -ErrorAction SilentlyContinue
        if (-not $process -or $process.HasExited) {
            Write-Success "$ServiceName terminated"
            return $true
        } else {
            Write-Fail "$ServiceName could not be terminated"
            return $false
        }
    } catch {
        Write-Fail "$ServiceName shutdown error: $_"
        return $false
    }
}

# ============================================================================
# Function to stop process by port
# ============================================================================
function Stop-ProcessByPort {
    param(
        [int]$Port,
        [string]$ServiceName
    )

    $connection = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($connection) {
        $procId = $connection.OwningProcess
        $process = Get-Process -Id $procId -ErrorAction SilentlyContinue
        if ($process) {
            Write-Wait "Found $ServiceName on port $Port (PID: $procId, Process: $($process.ProcessName))"
            return Stop-ServiceGracefully -ProcessId $procId -ServiceName $ServiceName
        }
    }
    Write-Skip "${ServiceName}: Port $Port not in use"
    return $true
}

# ============================================================================
# Step 1: Load saved PIDs (if available)
# ============================================================================
$pidFile = Join-Path $ProjectRoot ".agentcert-pids.json"
$savedPids = $null

if (Test-Path $pidFile) {
    try {
        $savedPids = Get-Content $pidFile -Raw | ConvertFrom-Json
        Write-Status "Found saved process information from $($savedPids.StartTime)"
    } catch {
        Write-Status "Could not read saved PIDs, will detect processes by port"
    }
}

# ============================================================================
# Step 2: Stop Frontend (Port 3001)
# ============================================================================
Write-Status "Stopping Frontend..."

$frontendStopped = $false

# Try saved PID first
if ($savedPids -and $savedPids.Frontend) {
    $frontendStopped = Stop-ServiceGracefully -ProcessId $savedPids.Frontend -ServiceName "Frontend"
}

# Also check for any node processes on port 3001
if (-not $frontendStopped) {
    $frontendStopped = Stop-ProcessByPort -Port 3001 -ServiceName "Frontend"
}

# Kill any remaining node processes that might be webpack-dev-server
$nodeProcesses = Get-Process -Name "node" -ErrorAction SilentlyContinue | Where-Object {
    $_.MainWindowTitle -match "webpack" -or $_.CommandLine -match "webpack"
}
foreach ($proc in $nodeProcesses) {
    Write-Wait "Stopping webpack process (PID: $($proc.Id))..."
    Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
}

# ============================================================================
# Step 3: Stop GraphQL Server (Port 8080)
# ============================================================================
Write-Status "Stopping GraphQL Server..."

$gqlStopped = $false

# Try saved PID first
if ($savedPids -and $savedPids.GraphQL) {
    $gqlStopped = Stop-ServiceGracefully -ProcessId $savedPids.GraphQL -ServiceName "GraphQL Server"
}

# Also stop by port
if (-not $gqlStopped) {
    $gqlStopped = Stop-ProcessByPort -Port 8080 -ServiceName "GraphQL Server"
}

# Also stop any server.exe processes
$serverProcesses = Get-Process -Name "server" -ErrorAction SilentlyContinue
foreach ($proc in $serverProcesses) {
    if ($proc.Path -match "graphql") {
        Write-Wait "Stopping server.exe (PID: $($proc.Id))..."
        Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
    }
}

# ============================================================================
# Step 4: Stop Authentication Service (Port 3030)
# ============================================================================
Write-Status "Stopping Authentication Service..."

$authStopped = $false

# Try saved PID first
if ($savedPids -and $savedPids.Auth) {
    $authStopped = Stop-ServiceGracefully -ProcessId $savedPids.Auth -ServiceName "Auth Service"
}

# Also stop by port
if (-not $authStopped) {
    $authStopped = Stop-ProcessByPort -Port 3030 -ServiceName "Auth Service"
}

# Also stop any auth.exe processes
$authProcesses = Get-Process -Name "auth" -ErrorAction SilentlyContinue
foreach ($proc in $authProcesses) {
    Write-Wait "Stopping auth.exe (PID: $($proc.Id))..."
    Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
}

# ============================================================================
# Step 5: Clean up port states (wait for TIME_WAIT to clear)
# ============================================================================
Write-Status "Waiting for ports to be released..."

$ports = @(3001, 3030, 8080)
$maxWait = 10
$waited = 0

while ($waited -lt $maxWait) {
    $busyPorts = @()
    foreach ($port in $ports) {
        $conn = Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue | 
                Where-Object { $_.State -in @('Listen', 'Established', 'TimeWait', 'CloseWait', 'FinWait1', 'FinWait2') }
        if ($conn) {
            $busyPorts += $port
        }
    }
    
    if ($busyPorts.Count -eq 0) {
        Write-Success "All ports are now free"
        break
    }
    
    if ($waited -eq 0) {
        Write-Wait "Ports still releasing: $($busyPorts -join ', ')..."
    }
    
    Start-Sleep -Seconds 1
    $waited++
}

if ($waited -eq $maxWait) {
    Write-Host "         Some ports may still be in TIME_WAIT state. This is normal." -ForegroundColor Yellow
    Write-Host "         They will be released automatically within 30-60 seconds." -ForegroundColor Yellow
}

# ============================================================================
# Step 6: Clean up PID files
# ============================================================================
Write-Status "Cleaning up..."

$pidFiles = @(
    ".agentcert-pids.json",
    ".agentcert-auth.pid",
    ".agentcert-graphql.pid",
    ".agentcert-frontend.pid"
)

foreach ($file in $pidFiles) {
    $filePath = Join-Path $ProjectRoot $file
    if (Test-Path $filePath) {
        Remove-Item $filePath -Force -ErrorAction SilentlyContinue
    }
}

Write-Success "PID files cleaned up"

# ============================================================================
# Summary
# ============================================================================
Write-Host ""
Write-Host "============================================" -ForegroundColor Green
Write-Host "       AgentCert Stopped Successfully!     " -ForegroundColor Green
Write-Host "============================================" -ForegroundColor Green
Write-Host ""

# Final port status
Write-Host "Port Status:" -ForegroundColor White
foreach ($port in $ports) {
    $conn = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
    if ($conn) {
        $proc = Get-Process -Id $conn.OwningProcess -ErrorAction SilentlyContinue
        Write-Host "  - Port ${port}: " -NoNewline -ForegroundColor Gray
        Write-Host "IN USE by $($proc.ProcessName)" -ForegroundColor Red
    } else {
        Write-Host "  - Port ${port}: " -NoNewline -ForegroundColor Gray
        Write-Host "FREE" -ForegroundColor Green
    }
}

Write-Host ""
Write-Host "MongoDB Status:" -ForegroundColor White
if (-not $KeepMongo) {
    Write-Host "  - MongoDB container is still running (use -KeepMongo:$false to stop)" -ForegroundColor Gray
} else {
    Write-Host "  - MongoDB container was kept running" -ForegroundColor Gray
}

Write-Host ""
Write-Host "To restart AgentCert, run: .\start-agentcert.ps1" -ForegroundColor Yellow
Write-Host ""
