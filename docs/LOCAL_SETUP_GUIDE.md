# AgentCert Local Development Setup Guide

This document provides step-by-step instructions for setting up, running, and managing the AgentCert/ChaosCenter development environment on **Windows**.

---

## Table of Contents
- [Prerequisites](#prerequisites)
- [Initial Setup (First Time Only)](#initial-setup-first-time-only)
- [Starting the Project](#starting-the-project)
- [Everyday Quick Start](#everyday-quick-start)
- [Stopping the Project](#stopping-the-project)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

Ensure the following are installed on your system:

| Tool | Version | Installation |
|------|---------|--------------|
| Docker Desktop | Latest | [Download](https://www.docker.com/products/docker-desktop/) |
| Go | 1.20+ | `winget install GoLang.Go` |
| Node.js | 18+ | `winget install OpenJS.NodeJS.LTS` |
| Yarn | 1.22+ | `npm install -g yarn` |
| OpenSSL | 3.x | `winget install ShiningLight.OpenSSL.Light` |

---

## Initial Setup (First Time Only)

### 1. Add Hosts Entry (Requires Admin)

Open PowerShell as Administrator and run:
```powershell
Add-Content -Path "C:\Windows\System32\drivers\etc\hosts" -Value "`n127.0.0.1       m1 m2 m3"
```

Or manually edit `C:\Windows\System32\drivers\etc\hosts` and add:
```
127.0.0.1       m1 m2 m3
```

### 2. Reserve Port 3030 (Optional - If Port Conflicts)

Run as Administrator:
```powershell
net stop winnat
netsh int ipv4 add excludedportrange protocol=tcp startport=3030 numberofports=1
net start winnat
```

### 3. Create MongoDB Network and Containers

```powershell
# Create Docker network
docker network create mongo-cluster

# Run MongoDB replica set containers
docker run -d --net mongo-cluster -p 27015:27015 --name m1 mongo:4.2 mongod --replSet rs0 --port 27015
docker run -d --net mongo-cluster -p 27016:27016 --name m2 mongo:4.2 mongod --replSet rs0 --port 27016
docker run -d --net mongo-cluster -p 27017:27017 --name m3 mongo:4.2 mongod --replSet rs0 --port 27017
```

### 4. Configure MongoDB Replica Set

```powershell
# Initialize replica set
docker exec m1 mongo --port 27015 --eval "rs.initiate({_id:'rs0',members:[{_id:0,host:'m1:27015'},{_id:1,host:'m2:27016'},{_id:2,host:'m3:27017'}]})"

# Wait 5 seconds for election
Start-Sleep -Seconds 5

# Create admin user
docker exec m1 mongo --port 27015 --eval "db.getSiblingDB('admin').createUser({user:'admin',pwd:'1234',roles:[{role:'root',db:'admin'}]})"
```

### 5. Generate SSL Certificates for Frontend

```powershell
cd chaoscenter\web

# Create certificates directory if not exists
mkdir -p certificates

# Generate self-signed certificate
& "C:\Program Files\OpenSSL-Win64\bin\openssl.exe" req -x509 -newkey rsa:4096 -keyout certificates/localhost-key.pem -out certificates/localhost.pem -days 365 -nodes -subj "//C=US"
```

### 6. Install Frontend Dependencies

```powershell
cd chaoscenter\web
yarn
```

---

## Starting the Project

### Quick Start (All Services)

Run each command in a **separate PowerShell terminal**:

#### Terminal 1: Start MongoDB Containers
```powershell
docker start m1 m2 m3
```

#### Terminal 2: Start Authentication Server (Port 3000)
```powershell
cd c:\Users\sharmadeep\AgentCert\chaoscenter\authentication

# Set environment variables
$env:ADMIN_USERNAME="admin"
$env:ADMIN_PASSWORD="litmus"
$env:DB_SERVER="mongodb://m1:27015,m2:27016,m3:27017/?replicaSet=rs0"
$env:DB_USER="admin"
$env:DB_PASSWORD="1234"
$env:JWT_SECRET="litmus-portal@123"
$env:PORTAL_ENDPOINT="http://localhost:8080"
$env:LITMUS_SVC_ENDPOINT=""
$env:SELF_AGENT="false"
$env:INFRA_SCOPE="cluster"
$env:INFRA_NAMESPACE="litmus"
$env:LITMUS_PORTAL_NAMESPACE="litmus"
$env:PORTAL_SCOPE="namespace"
$env:ENABLE_INTERNAL_TLS="false"
$env:REST_PORT="3000"
$env:GRPC_PORT="3030"

# Run the server
go run api/main.go
```

#### Terminal 3: Start GraphQL Server (Port 8080)
```powershell
cd c:\Users\sharmadeep\AgentCert\chaoscenter\graphql\server

# Set environment variables
$env:DB_SERVER="mongodb://m1:27015,m2:27016,m3:27017/?replicaSet=rs0"
$env:DB_USER="admin"
$env:DB_PASSWORD="1234"
$env:JWT_SECRET="litmus-portal@123"
$env:PORTAL_ENDPOINT="http://localhost:8080"
$env:LITMUS_SVC_ENDPOINT=""
$env:SELF_AGENT="false"
$env:INFRA_SCOPE="cluster"
$env:INFRA_NAMESPACE="litmus"
$env:LITMUS_PORTAL_NAMESPACE="litmus"
$env:PORTAL_SCOPE="namespace"
$env:SUBSCRIBER_IMAGE="litmuschaos/litmusportal-subscriber:ci"
$env:EVENT_TRACKER_IMAGE="litmuschaos/litmusportal-event-tracker:ci"
$env:CONTAINER_RUNTIME_EXECUTOR="k8sapi"
$env:ARGO_WORKFLOW_CONTROLLER_IMAGE="argoproj/workflow-controller:v2.11.0"
$env:ARGO_WORKFLOW_EXECUTOR_IMAGE="argoproj/argoexec:v2.11.0"
$env:CHAOS_CENTER_SCOPE="cluster"
$env:WORKFLOW_HELPER_IMAGE_VERSION="3.0.0"
$env:LITMUS_CHAOS_OPERATOR_IMAGE="litmuschaos/chaos-operator:3.0.0"
$env:LITMUS_CHAOS_RUNNER_IMAGE="litmuschaos/chaos-runner:3.0.0"
$env:LITMUS_CHAOS_EXPORTER_IMAGE="litmuschaos/chaos-exporter:3.0.0"
$env:ADMIN_USERNAME="admin"
$env:ADMIN_PASSWORD="litmus"
$env:VERSION="ci"
$env:HUB_BRANCH_NAME="v2.0.x"
$env:INFRA_DEPLOYMENTS='["app=chaos-exporter", "name=chaos-operator", "app=event-tracker","app=workflow-controller"]'
$env:INFRA_COMPATIBLE_VERSIONS='["0.2.0", "0.1.0","ci"]'
$env:DEFAULT_HUB_BRANCH_NAME="master"

# Run the server
go run server.go
```

#### Terminal 4: Start Frontend (Port 8185)
```powershell
cd c:\Users\sharmadeep\AgentCert\chaoscenter\web
yarn dev
```

### Access the Application

| Service | URL | Notes |
|---------|-----|-------|
| **Frontend UI** | https://localhost:8185 | Accept self-signed certificate warning |
| GraphQL API | http://localhost:8080 | Backend API |
| Auth API | http://localhost:3000 | Authentication service |

### Login Credentials
```
Username: admin
Password: litmus
```

---

## Everyday Quick Start

> **Use this section for subsequent runs after the initial setup is complete.**

After the initial setup, starting the project is straightforward. Follow these steps each time you want to run the project:

### Step 1: Start MongoDB (Terminal 1)

```powershell
docker start m1 m2 m3
```

Wait 2-3 seconds for MongoDB to be ready.

### Step 2: Start Authentication Server (Terminal 2)

Open a **new PowerShell terminal** and run:

```powershell
cd c:\Users\sharmadeep\AgentCert\chaoscenter\authentication

$env:ADMIN_USERNAME="admin"; $env:ADMIN_PASSWORD="litmus"; $env:DB_SERVER="mongodb://m1:27015,m2:27016,m3:27017/?replicaSet=rs0"; $env:DB_USER="admin"; $env:DB_PASSWORD="1234"; $env:JWT_SECRET="litmus-portal@123"; $env:PORTAL_ENDPOINT="http://localhost:8080"; $env:LITMUS_SVC_ENDPOINT=""; $env:SELF_AGENT="false"; $env:INFRA_SCOPE="cluster"; $env:INFRA_NAMESPACE="litmus"; $env:LITMUS_PORTAL_NAMESPACE="litmus"; $env:PORTAL_SCOPE="namespace"; $env:ENABLE_INTERNAL_TLS="false"; $env:REST_PORT="3000"; $env:GRPC_PORT="3030"

go run api/main.go
```

Wait until you see: `Listening and serving HTTP on :3000`

### Step 3: Start GraphQL Server (Terminal 3)

Open a **new PowerShell terminal** and run:

```powershell
cd c:\Users\sharmadeep\AgentCert\chaoscenter\graphql\server

$env:DB_SERVER="mongodb://m1:27015,m2:27016,m3:27017/?replicaSet=rs0"; $env:DB_USER="admin"; $env:DB_PASSWORD="1234"; $env:JWT_SECRET="litmus-portal@123"; $env:PORTAL_ENDPOINT="http://localhost:8080"; $env:LITMUS_SVC_ENDPOINT=""; $env:SELF_AGENT="false"; $env:INFRA_SCOPE="cluster"; $env:INFRA_NAMESPACE="litmus"; $env:LITMUS_PORTAL_NAMESPACE="litmus"; $env:PORTAL_SCOPE="namespace"; $env:SUBSCRIBER_IMAGE="litmuschaos/litmusportal-subscriber:ci"; $env:EVENT_TRACKER_IMAGE="litmuschaos/litmusportal-event-tracker:ci"; $env:CONTAINER_RUNTIME_EXECUTOR="k8sapi"; $env:ARGO_WORKFLOW_CONTROLLER_IMAGE="argoproj/workflow-controller:v2.11.0"; $env:ARGO_WORKFLOW_EXECUTOR_IMAGE="argoproj/argoexec:v2.11.0"; $env:CHAOS_CENTER_SCOPE="cluster"; $env:WORKFLOW_HELPER_IMAGE_VERSION="3.0.0"; $env:LITMUS_CHAOS_OPERATOR_IMAGE="litmuschaos/chaos-operator:3.0.0"; $env:LITMUS_CHAOS_RUNNER_IMAGE="litmuschaos/chaos-runner:3.0.0"; $env:LITMUS_CHAOS_EXPORTER_IMAGE="litmuschaos/chaos-exporter:3.0.0"; $env:ADMIN_USERNAME="admin"; $env:ADMIN_PASSWORD="litmus"; $env:VERSION="ci"; $env:HUB_BRANCH_NAME="v2.0.x"; $env:INFRA_DEPLOYMENTS='["app=chaos-exporter", "name=chaos-operator", "app=event-tracker","app=workflow-controller"]'; $env:INFRA_COMPATIBLE_VERSIONS='["0.2.0", "0.1.0","ci"]'; $env:DEFAULT_HUB_BRANCH_NAME="master"

go run server.go
```

Wait until the server is ready.

### Step 4: Start Frontend (Terminal 4)

Open a **new PowerShell terminal** and run:

```powershell
cd c:\Users\sharmadeep\AgentCert\chaoscenter\web
yarn dev
```

Wait until you see: `compiled successfully` or webpack output.

### Step 5: Access the Application

Open your browser and navigate to: **https://localhost:8185**

Login with:
- **Username:** `admin`
- **Password:** `litmus`

---

### Quick Reference: All-in-One Script

For convenience, you can create a batch file `start-litmus.ps1` in the project root:

```powershell
# start-litmus.ps1 - Run this file to start all services
# Each command opens a new terminal window

# Start MongoDB
docker start m1 m2 m3

Start-Sleep -Seconds 3

# Start Auth Server in new window
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd 'c:\Users\sharmadeep\AgentCert\chaoscenter\authentication'; `$env:ADMIN_USERNAME='admin'; `$env:ADMIN_PASSWORD='litmus'; `$env:DB_SERVER='mongodb://m1:27015,m2:27016,m3:27017/?replicaSet=rs0'; `$env:DB_USER='admin'; `$env:DB_PASSWORD='1234'; `$env:JWT_SECRET='litmus-portal@123'; `$env:PORTAL_ENDPOINT='http://localhost:8080'; `$env:LITMUS_SVC_ENDPOINT=''; `$env:SELF_AGENT='false'; `$env:INFRA_SCOPE='cluster'; `$env:INFRA_NAMESPACE='litmus'; `$env:LITMUS_PORTAL_NAMESPACE='litmus'; `$env:PORTAL_SCOPE='namespace'; `$env:ENABLE_INTERNAL_TLS='false'; `$env:REST_PORT='3000'; `$env:GRPC_PORT='3030'; go run api/main.go"

Start-Sleep -Seconds 10

# Start GraphQL Server in new window
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd 'c:\Users\sharmadeep\AgentCert\chaoscenter\graphql\server'; `$env:DB_SERVER='mongodb://m1:27015,m2:27016,m3:27017/?replicaSet=rs0'; `$env:DB_USER='admin'; `$env:DB_PASSWORD='1234'; `$env:JWT_SECRET='litmus-portal@123'; `$env:PORTAL_ENDPOINT='http://localhost:8080'; `$env:ADMIN_USERNAME='admin'; `$env:ADMIN_PASSWORD='litmus'; `$env:VERSION='ci'; go run server.go"

Start-Sleep -Seconds 10

# Start Frontend in new window
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd 'c:\Users\sharmadeep\AgentCert\chaoscenter\web'; yarn dev"

Write-Host "`nAll services starting! Wait 30 seconds then open: https://localhost:8185"
Write-Host "Login: admin / litmus"
```

To use: `powershell -ExecutionPolicy Bypass -File start-litmus.ps1`

---

## Stopping the Project

### Option 1: Stop Individual Services

Press `Ctrl+C` in each terminal running a service.

### Option 2: Stop All Services

```powershell
# Stop MongoDB containers
docker stop m1 m2 m3

# Find and kill Go processes (Auth & GraphQL servers)
Get-Process -Name "go" -ErrorAction SilentlyContinue | Stop-Process -Force

# Find and kill Node processes (Frontend)
Get-Process -Name "node" -ErrorAction SilentlyContinue | Stop-Process -Force
```

### Option 3: Stop Only MongoDB

```powershell
docker stop m1 m2 m3
```

### Option 4: Full Cleanup (Remove Everything)

```powershell
# Stop and remove MongoDB containers
docker stop m1 m2 m3
docker rm m1 m2 m3

# Remove Docker network
docker network rm mongo-cluster

# Kill all related processes
Get-Process -Name "go" -ErrorAction SilentlyContinue | Stop-Process -Force
Get-Process -Name "node" -ErrorAction SilentlyContinue | Stop-Process -Force
```

---

## Service Status Check

### Check if Services are Running

```powershell
# Check MongoDB containers
docker ps --filter "name=m1" --filter "name=m2" --filter "name=m3" --format "table {{.Names}}\t{{.Status}}"

# Check ports in use
netstat -an | Select-String ":3000|:8080|:8185"
```

Expected output when all services are running:
```
TCP    0.0.0.0:3000    LISTENING    # Auth Server
TCP    0.0.0.0:8080    LISTENING    # GraphQL Server  
TCP    0.0.0.0:8185    LISTENING    # Frontend
```

---

## Troubleshooting

### MongoDB Connection Issues

**Error**: `lookup m1: no such host`

**Solution**: Ensure hosts file entry exists:
```powershell
Get-Content "C:\Windows\System32\drivers\etc\hosts" | Select-String "m1"
```

If missing, add it (requires admin):
```powershell
Add-Content -Path "C:\Windows\System32\drivers\etc\hosts" -Value "127.0.0.1       m1 m2 m3"
```

### Port Already in Use

**Error**: `listen tcp :3000: bind: address already in use`

**Solution**: Find and kill the process:
```powershell
netstat -ano | Select-String ":3000"
# Note the PID, then:
Stop-Process -Id <PID> -Force
```

### Go Not Found

**Error**: `go: The term 'go' is not recognized`

**Solution**: Refresh PATH:
```powershell
$env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")
```

### OpenSSL Not Found

**Solution**: Use full path:
```powershell
& "C:\Program Files\OpenSSL-Win64\bin\openssl.exe" version
```

### MongoDB Replica Set Not Initialized

**Solution**: Re-initialize:
```powershell
docker exec m1 mongo --port 27015 --eval "rs.status()"
# If not initialized:
docker exec m1 mongo --port 27015 --eval "rs.initiate({_id:'rs0',members:[{_id:0,host:'m1:27015'},{_id:1,host:'m2:27016'},{_id:2,host:'m3:27017'}]})"
```

---
## Tomorrow: Quick Restart

> **Use this section to quickly restart all services after a graceful shutdown.**

### Terminal 1: Start MongoDB
```powershell
docker start m1 m2 m3
```

Wait 2-3 seconds for MongoDB to be ready.

### Terminal 2: Start Auth Server
```powershell
cd c:\Users\sharmadeep\AgentCert\chaoscenter\authentication

$env:ADMIN_USERNAME="admin"; $env:ADMIN_PASSWORD="litmus"; $env:DB_SERVER="mongodb://m1:27015,m2:27016,m3:27017/?replicaSet=rs0"; $env:DB_USER="admin"; $env:DB_PASSWORD="1234"; $env:JWT_SECRET="litmus-portal@123"; $env:PORTAL_ENDPOINT="http://localhost:8080"; $env:LITMUS_SVC_ENDPOINT=""; $env:SELF_AGENT="false"; $env:INFRA_SCOPE="cluster"; $env:INFRA_NAMESPACE="litmus"; $env:LITMUS_PORTAL_NAMESPACE="litmus"; $env:PORTAL_SCOPE="namespace"; $env:ENABLE_INTERNAL_TLS="false"; $env:REST_PORT="3000"; $env:GRPC_PORT="3030"

go run api/main.go
```

Wait until you see: `Listening and serving HTTP on :3000`

### Terminal 3: Start GraphQL Server (with ALLOWED_ORIGINS for Kubernetes)
```powershell
cd c:\Users\sharmadeep\AgentCert\chaoscenter\graphql\server

$env:DB_SERVER="mongodb://m1:27015,m2:27016,m3:27017/?replicaSet=rs0"; $env:DB_USER="admin"; $env:DB_PASSWORD="1234"; $env:JWT_SECRET="litmus-portal@123"; $env:PORTAL_ENDPOINT="http://localhost:8080"; $env:LITMUS_SVC_ENDPOINT=""; $env:SELF_AGENT="false"; $env:INFRA_SCOPE="cluster"; $env:INFRA_NAMESPACE="litmus"; $env:LITMUS_PORTAL_NAMESPACE="litmus"; $env:PORTAL_SCOPE="namespace"; $env:SUBSCRIBER_IMAGE="litmuschaos/litmusportal-subscriber:ci"; $env:EVENT_TRACKER_IMAGE="litmuschaos/litmusportal-event-tracker:ci"; $env:CONTAINER_RUNTIME_EXECUTOR="k8sapi"; $env:ARGO_WORKFLOW_CONTROLLER_IMAGE="argoproj/workflow-controller:v2.11.0"; $env:ARGO_WORKFLOW_EXECUTOR_IMAGE="argoproj/argoexec:v2.11.0"; $env:CHAOS_CENTER_SCOPE="cluster"; $env:WORKFLOW_HELPER_IMAGE_VERSION="3.0.0"; $env:LITMUS_CHAOS_OPERATOR_IMAGE="litmuschaos/chaos-operator:3.0.0"; $env:LITMUS_CHAOS_RUNNER_IMAGE="litmuschaos/chaos-runner:3.0.0"; $env:LITMUS_CHAOS_EXPORTER_IMAGE="litmuschaos/chaos-exporter:3.0.0"; $env:ADMIN_USERNAME="admin"; $env:ADMIN_PASSWORD="litmus"; $env:VERSION="ci"; $env:HUB_BRANCH_NAME="v2.0.x"; $env:INFRA_DEPLOYMENTS='["app=chaos-exporter", "name=chaos-operator", "app=event-tracker","app=workflow-controller"]'; $env:INFRA_COMPATIBLE_VERSIONS='["0.2.0", "0.1.0","ci"]'; $env:DEFAULT_HUB_BRANCH_NAME="master"; $env:ALLOWED_ORIGINS="^(http://|https://|)litmuschaos.io(:[0-9]+|)?,^(http://|https://|)localhost(:[0-9]+|),^(http://|https://|)host.docker.internal(:[0-9]+|)"

go run server.go
```

Wait until the server is ready.

### Terminal 4: Start Frontend
```powershell
cd c:\Users\sharmadeep\AgentCert\chaoscenter\web
yarn dev
```

Wait until you see: `compiled successfully`

### (If you scaled down) Restart Kubernetes Pods
```powershell
kubectl scale deployment subscriber event-tracker chaos-exporter chaos-operator-ce workflow-controller -n litmus --replicas=1
```

### Verify Everything is Running
```powershell
# Check services are listening
netstat -an | Select-String ":3000|:8080|:8185"

# Check Kubernetes pods
kubectl get pods -n litmus
```

### Access the Application
Open **https://localhost:8185** and login with:
- **Username:** `admin`
- **Password:** `litmus`

> **Note:** The Kubernetes ConfigMap changes (`SERVER_ADDR: http://host.docker.internal:8080/query`) are persistent and don't need to be redone after restart.

---

## Service Status Check

// ...existing code...
## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                        Frontend                              │
│                   https://localhost:8185                     │
│                      (React + Webpack)                       │
└─────────────────────┬───────────────────┬───────────────────┘
                      │                   │
                      │ /api              │ /auth
                      ▼                   ▼
┌─────────────────────────────┐  ┌─────────────────────────────┐
│      GraphQL Server         │  │    Authentication Server    │
│   http://localhost:8080     │  │    http://localhost:3000    │
│         (Go)                │  │          (Go)               │
└─────────────────────────────┘  └─────────────────────────────┘
                      │                   │
                      └─────────┬─────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────┐
│                    MongoDB Replica Set                       │
│         m1:27015  |  m2:27016  |  m3:27017                  │
│                    (Docker Containers)                       │
└─────────────────────────────────────────────────────────────┘
```

---

## Quick Reference Commands

| Action | Command |
|--------|---------|
| Start MongoDB | `docker start m1 m2 m3` |
| Stop MongoDB | `docker stop m1 m2 m3` |
| Check MongoDB Status | `docker ps --filter "name=m"` |
| Check Ports | `netstat -an \| Select-String ":3000\|:8080\|:8185"` |
| Kill Go Processes | `Get-Process go \| Stop-Process -Force` |
| Kill Node Processes | `Get-Process node \| Stop-Process -Force` |

---

*Document created: January 16, 2026*
*Based on successful local deployment of AgentCert/ChaosCenter*
