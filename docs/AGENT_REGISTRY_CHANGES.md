# Agent Registry Feature - Implementation Summary

## Overview

This document summarizes the Agent Registry feature implementation, which enables users to register, view, edit, and delete AI agents through the AgentCert platform.

## What Was Implemented

### Backend (GraphQL Server)

#### 1. GraphQL Schema (`chaoscenter/graphql/definitions/shared/agent_registry.graphqls`)
- Added new types: `Agent`, `AgentInput`, `UpdateAgentInput`, `AgentFilterInput`
- Added queries: `listAgents`, `getAgent`
- Added mutations: `registerAgent`, `updateAgent`, `deleteAgent`

#### 2. Resolvers (`chaoscenter/graphql/server/graph/agent_registry.resolvers.go`)
- `RegisterAgent` - Creates a new agent entry
- `UpdateAgent` - Updates an existing agent
- `DeleteAgent` - Removes an agent
- `ListAgents` - Returns paginated list of agents
- `GetAgent` - Retrieves a single agent by ID

#### 3. Service Layer (`chaoscenter/graphql/server/pkg/agent_registry/`)
- `service.go` - Business logic for agent operations
- `handler.go` - Handler functions for database operations
- `mapper.go` - Maps between GraphQL models and database models

#### 4. Database (`chaoscenter/graphql/server/pkg/database/mongodb/init.go`)
- Added `AgentRegistryCollection` constant pointing to `agent_registry` collection

### Frontend (React Web App)

#### 1. API Hooks (`chaoscenter/web/src/api/core/agents/`)
- `listAgents.ts` - Hook to fetch agents with pagination
- `updateAgent.ts` - Hook to update agent details
- `deleteAgent.ts` - Hook to delete an agent
- `index.ts` - Exports all hooks

#### 2. UI Components (`chaoscenter/web/src/views/AgentOnboarding/`)
- `AgentOnboarding.tsx` - Main view with:
  - Agent registration form
  - Tabular display of registered agents
  - Edit modal for modifying agent details
  - Delete confirmation dialog
- `AgentOnboarding.module.scss` - Styling for the view

#### 3. Localization (`chaoscenter/web/src/strings/`)
- Added new strings for agent management UI

---

## Where Agent Data is Stored

### Database: MongoDB

**Collection Name:** `agent_registry`

**Database:** `litmus` (default, configured via `DB_SERVER` environment variable)

### Agent Document Schema

```json
{
  "_id": ObjectId("..."),
  "agent_id": "uuid-string",
  "project_id": "project-uuid",
  "name": "My AI Agent",
  "description": "Agent description",
  "version": "1.0.0",
  "vendor": "OpenAI",
  "capabilities": ["text-generation", "code-completion"],
  "tags": ["production", "gpt-4"],
  "namespace": "default",
  "status": "active",
  "container_image": "myregistry/agent:v1",
  "endpoint": "https://api.example.com/agent",
  "audit_info": {
    "created_at": ISODate("2026-01-24T..."),
    "updated_at": ISODate("2026-01-24T..."),
    "created_by": "admin",
    "updated_by": "admin"
  }
}
```

---

## How to View Agent Data via CLI

### Using MongoDB Shell (mongosh)

```bash
# Connect to MongoDB
mongosh mongodb://localhost:27017

# Switch to litmus database
use litmus

# List all agents
db.agent_registry.find().pretty()

# Count total agents
db.agent_registry.countDocuments()

# Find agents by name
db.agent_registry.find({ name: /MyAgent/i }).pretty()

# Find agents by vendor
db.agent_registry.find({ vendor: "OpenAI" }).pretty()

# Find agents by status
db.agent_registry.find({ status: "active" }).pretty()

# Find agents with specific capability
db.agent_registry.find({ capabilities: "text-generation" }).pretty()

# Find agents by project
db.agent_registry.find({ project_id: "your-project-id" }).pretty()

# Get a specific agent by ID
db.agent_registry.findOne({ agent_id: "agent-uuid-here" })
```

### Using Docker (if MongoDB is in container)

```bash
# If using Docker containers named m1, m2, m3 (replica set)
docker exec -it m1 mongosh

# Then run the MongoDB commands above
```

### Using curl with GraphQL API

```bash
# Get JWT token first (login)
TOKEN=$(curl -s -X POST http://localhost:3030/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Pa$$w0rd"}' | jq -r '.accessToken')

# List all agents
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "query": "query { listAgents(pagination: {page: 1, limit: 50}) { totalCount agents { agentID name vendor version status } } }"
  }'

# Get a specific agent
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "query": "query { getAgent(agentID: \"your-agent-id\") { agentID name description vendor version capabilities } }"
  }'
```

---

## Environment Configuration

The following environment variables control the database connection:

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_SERVER` | MongoDB connection string | `mongodb://localhost:27017` |
| `DB_USER` | MongoDB username (optional) | - |
| `DB_PASSWORD` | MongoDB password (optional) | - |
| `JWT_SECRET` | Secret for JWT tokens | `litmus-portal@123` |

---

## Files Changed

### Backend (19 files)
- `chaoscenter/graphql/definitions/shared/agent_registry.graphqls`
- `chaoscenter/graphql/server/go.mod`
- `chaoscenter/graphql/server/go.sum`
- `chaoscenter/graphql/server/graph/agent_registry.resolvers.go`
- `chaoscenter/graphql/server/graph/generated/generated.go`
- `chaoscenter/graphql/server/graph/model/models_gen.go`
- `chaoscenter/graphql/server/pkg/agent_registry/handler.go`
- `chaoscenter/graphql/server/pkg/agent_registry/mapper.go`
- `chaoscenter/graphql/server/pkg/agent_registry/service.go`
- `chaoscenter/graphql/server/pkg/database/mongodb/init.go`
- `chaoscenter/graphql/server/utils/variables.go`

### Frontend (10 files)
- `chaoscenter/web/config/webpack.dev.js`
- `chaoscenter/web/src/api/core/agents/index.ts`
- `chaoscenter/web/src/api/core/agents/listAgents.ts`
- `chaoscenter/web/src/api/core/agents/deleteAgent.ts` (new)
- `chaoscenter/web/src/api/core/agents/updateAgent.ts` (new)
- `chaoscenter/web/src/strings/strings.en.yaml`
- `chaoscenter/web/src/strings/types.ts`
- `chaoscenter/web/src/views/AgentOnboarding/AgentOnboarding.tsx`
- `chaoscenter/web/src/views/AgentOnboarding/AgentOnboarding.module.scss`
- `chaoscenter/web/src/views/AgentOnboarding/AgentOnboarding.module.scss.d.ts`

---

## Commit Information

- **Branch:** `Agent-Registrations-and-UI-Integrations`
- **Commit:** `f068fb7`
- **Message:** `feat: Add Agent Registry UI with Edit/Delete functionality`

---

## Testing the Feature

1. Start MongoDB: `docker start m1 m2 m3`
2. Start Auth Service: `cd chaoscenter/authentication && go run api/main.go`
3. Start GraphQL Server: `cd chaoscenter/graphql/server && ./server.exe`
4. Start Frontend: `cd chaoscenter/web && yarn dev`
5. Open https://localhost:3000 and login with `admin` / `Pa$$w0rd`
6. Navigate to Agent Onboarding page
7. Register a new agent using the form
8. View, edit, or delete agents in the table
