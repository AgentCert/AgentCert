# MongoDB Setup Guide for AgentCert

This guide covers installing MongoDB with Vector Search capability and configuring it for storing `LLMQuantitativeExtraction` and `LLMQualitativeExtraction` metrics data.

## Table of Contents

- [Overview](#overview)
- [Installation Options](#installation-options)
  - [Option 1: MongoDB Atlas (Recommended)](#option-1-mongodb-atlas-recommended)
  - [Option 2: Local MongoDB with Docker](#option-2-local-mongodb-with-docker)
  - [Option 3: Local MongoDB Installation (Windows)](#option-3-local-mongodb-installation-windows)
- [Configuration](#configuration)
- [Collection Schema](#collection-schema)
- [Vector Search Setup](#vector-search-setup)
- [Python Dependencies](#python-dependencies)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)

---

## Overview

AgentCert uses MongoDB to store extracted metrics from IT-Ops agent runs:

| Collection | Purpose | Data Model |
|------------|---------|------------|
| `llm_quantitative_extractions` | Stores quantitative metrics (TTD, tokens, accuracy) | `LLMQuantitativeExtraction` |
| `llm_qualitative_extractions` | Stores qualitative metrics (RAI, security, reasoning) | `LLMQualitativeExtraction` |
| `agent_run_metrics` | Combined metrics with vector embeddings | Combined document |

**Vector Search** enables semantic similarity queries over reasoning judgements and recommendations.

---

## Installation Options

### Option 1: MongoDB Atlas (Recommended)

MongoDB Atlas provides managed MongoDB with built-in Vector Search capability.

#### Step 1: Create Atlas Account

1. Go to [MongoDB Atlas](https://www.mongodb.com/cloud/atlas)
2. Sign up for a free account
3. Create a new project (e.g., "AgentCert")

#### Step 2: Create a Cluster

1. Click **"Build a Database"**
2. Select **M0 Free Tier** (or higher for production)
3. Choose a cloud provider and region close to your location
4. Name your cluster (e.g., "agentcert-cluster")
5. Click **"Create Cluster"**

#### Step 3: Configure Network Access

1. Go to **Security > Network Access**
2. Click **"Add IP Address"**
3. For development: Click **"Allow Access from Anywhere"** (0.0.0.0/0)
4. For production: Add specific IP addresses

#### Step 4: Create Database User

1. Go to **Security > Database Access**
2. Click **"Add New Database User"**
3. Choose **Password** authentication
4. Set username and password (save these!)
5. Set privileges to **"Read and write to any database"**
6. Click **"Add User"**

#### Step 5: Get Connection String

1. Go to **Database > Connect**
2. Select **"Connect your application"**
3. Choose **Python** and version **3.12 or later**
4. Copy the connection string:
   ```
   mongodb+srv://<username>:<password>@<cluster>.mongodb.net/?retryWrites=true&w=majority
   ```
5. Replace `<username>` and `<password>` with your credentials

#### Step 6: Enable Vector Search

1. Go to **Database > Browse Collections**
2. Create database `agentcert` if not exists
3. Go to **Atlas Search** tab
4. Click **"Create Search Index"**
5. Select **"JSON Editor"**
6. Use this index definition:

```json
{
  "name": "metrics_vector_index",
  "type": "vectorSearch",
  "definition": {
    "fields": [
      {
        "type": "vector",
        "path": "embedding",
        "numDimensions": 1536,
        "similarity": "cosine"
      },
      {
        "type": "filter",
        "path": "namespace"
      },
      {
        "type": "filter",
        "path": "quantitative.detection_accuracy"
      }
    ]
  }
}
```

7. Select collection `agent_run_metrics`
8. Click **"Create Search Index"**

---

### Option 2: Local MongoDB with Docker

Best for local development. Vector Search requires Atlas, but basic storage works locally.

#### Step 1: Install Docker

Download and install [Docker Desktop](https://www.docker.com/products/docker-desktop/)

#### Step 2: Run MongoDB Container

```powershell
# Pull MongoDB image
docker pull mongo:7.0

# Run MongoDB container
docker run -d `
  --name mongodb-agentcert `
  -p 27017:27017 `
  -e MONGO_INITDB_ROOT_USERNAME=admin `
  -e MONGO_INITDB_ROOT_PASSWORD=password123 `
  -v mongodb_data:/data/db `
  mongo:7.0
```

#### Step 3: Verify Container

```powershell
# Check container is running
docker ps

# View logs
docker logs mongodb-agentcert
```

#### Step 4: Connection String

```
mongodb://admin:password123@localhost:27017/?authSource=admin
```

> ⚠️ **Note**: Local MongoDB does not support Vector Search. For semantic search, use MongoDB Atlas.

---

### Option 3: Local MongoDB Installation (Windows)

#### Step 1: Download MongoDB

1. Go to [MongoDB Download Center](https://www.mongodb.com/try/download/community)
2. Select:
   - Version: **7.0.x** (latest)
   - Platform: **Windows**
   - Package: **MSI**
3. Click **Download**

#### Step 2: Run Installer

1. Run the downloaded `.msi` file
2. Choose **Complete** installation
3. ✅ Check **"Install MongoDB as a Service"**
4. ✅ Check **"Install MongoDB Compass"** (GUI tool)
5. Complete installation

#### Step 3: Verify Installation

```powershell
# Check MongoDB service status
Get-Service MongoDB

# Connect via mongosh
mongosh
```

#### Step 4: Create Database and User

```javascript
// In mongosh
use admin

// Create admin user
db.createUser({
  user: "agentcert_admin",
  pwd: "your_secure_password",
  roles: [
    { role: "readWrite", db: "agentcert" },
    { role: "dbAdmin", db: "agentcert" }
  ]
})

// Switch to agentcert database
use agentcert

// Create collections
db.createCollection("llm_quantitative_extractions")
db.createCollection("llm_qualitative_extractions")
db.createCollection("agent_run_metrics")
```

#### Step 5: Connection String

```
mongodb://agentcert_admin:your_secure_password@localhost:27017/agentcert?authSource=admin
```

---

## Configuration

### Environment Variable

Set the MongoDB connection string as an environment variable:

**Windows (PowerShell):**
```powershell
$env:MONGODB_CONNECTION_STRING = "mongodb+srv://user:pass@cluster.mongodb.net/?retryWrites=true&w=majority"
```

**Windows (Permanent):**
```powershell
[Environment]::SetEnvironmentVariable("MONGODB_CONNECTION_STRING", "your_connection_string", "User")
```

**Or create a `.env` file** in `agentcert/`:
```env
MONGODB_CONNECTION_STRING=mongodb+srv://user:pass@cluster.mongodb.net/?retryWrites=true&w=majority
```

### Configuration File

The MongoDB settings are in `agentcert/configs/configs.json`:

```json
{
  "mongodb": {
    "connection_string_env": "MONGODB_CONNECTION_STRING",
    "database": "agentcert",
    "collections": {
      "metrics": "agent_run_metrics",
      "quantitative": "llm_quantitative_extractions",
      "qualitative": "llm_qualitative_extractions"
    },
    "vector_search": {
      "index_name": "metrics_vector_index",
      "embedding_field": "embedding",
      "dimensions": 1536,
      "similarity": "cosine",
      "num_candidates": 100,
      "limit": 10
    }
  }
}
```

---

## Collection Schema

### LLMQuantitativeExtraction Collection

```javascript
// Collection: llm_quantitative_extractions
{
  "_id": ObjectId("..."),
  "session_id": "uuid-string",                    // Unique session identifier
  "namespace": "kubernetes-namespace",            // K8s namespace
  "deployment_name": "app-name",                  // Deployment name
  "time_to_detection_seconds": 45.5,              // TTD in seconds
  "time_to_mitigation_seconds": 120.0,            // TTM in seconds
  "framework_overhead_seconds": 5.2,              // Framework overhead
  "detection_accuracy": "Correct",                // Correct/Incorrect/Unknown
  "submission_status": "VALID_SUBMISSION",        // Submission result
  "trajectory_steps": 12,                         // Agent steps count
  "input_tokens": 15000,                          // LLM input tokens
  "output_tokens": 2500,                          // LLM output tokens
  "fault_type": "Misconfig",                      // Fault category
  "fault_target_service": "service-name",         // Affected service
  "fault_namespace": "fault-namespace",           // Fault location
  "tool_calls": [                                 // Agent tool invocations
    {
      "name": "get_logs",
      "arguments": {"namespace": "default"},
      "success": true
    }
  ],
  "extraction_timestamp": ISODate("2026-01-21T10:00:00Z"),
  "embedding": [0.1, 0.2, ...]                    // Optional: 1536-dim vector
}
```

**Indexes:**
- `session_id` (unique, sparse)
- `extraction_timestamp` (descending)
- `namespace, deployment_name` (compound)
- `detection_accuracy`
- `submission_status`
- `fault_target_service`

### LLMQualitativeExtraction Collection

```javascript
// Collection: llm_qualitative_extractions
{
  "_id": ObjectId("..."),
  "session_id": "uuid-string",
  "rai_check_status": "Passed",                   // Passed/Failed/Not Evaluated
  "rai_check_notes": "No harmful content detected",
  "trajectory_efficiency_score": 8.5,             // 0-10 score
  "trajectory_efficiency_notes": "Efficient path taken",
  "security_compliance_status": "Compliant",      // Compliant/Non-Compliant/Partial
  "security_compliance_notes": "No sensitive data exposed",
  "acceptance_criteria_met": true,
  "acceptance_criteria_notes": "Anomaly correctly identified",
  "response_quality_score": 9.0,                  // 0-10 score
  "response_quality_notes": "Clear reasoning provided",
  "reasoning_judgement": "Agent showed strong diagnostic skills...",
  "reasoning_score": 8,                           // 0-10 score
  "known_limitations": [
    "Did not check all log sources",
    "Could improve error handling"
  ],
  "recommendations": [
    "Add retry logic for failed tool calls",
    "Include more context in submissions"
  ],
  "extraction_timestamp": ISODate("2026-01-21T10:00:00Z"),
  "embedding": [0.1, 0.2, ...]                    // Optional: 1536-dim vector
}
```

**Indexes:**
- `session_id` (unique, sparse)
- `extraction_timestamp` (descending)
- `rai_check_status`
- `security_compliance_status`
- `reasoning_score` (descending)
- `trajectory_efficiency_score` (descending)

### Combined Metrics Collection

```javascript
// Collection: agent_run_metrics
{
  "_id": ObjectId("..."),
  "session_id": "uuid-string",
  "quantitative": { /* LLMQuantitativeExtraction fields */ },
  "qualitative": { /* LLMQualitativeExtraction fields */ },
  "metadata": {
    "run_file_path": "/path/to/run.txt",
    "agent_version": "1.0.0"
  },
  "extraction_timestamp": ISODate("2026-01-21T10:00:00Z"),
  "embedding": [0.1, 0.2, ...]                    // 1536-dim vector for semantic search
}
```

---

## Vector Search Setup

Vector Search enables semantic similarity queries over metrics data.

### Prerequisites

- MongoDB Atlas M10+ cluster (Vector Search requires paid tier for production)
- Or M0 free tier with Atlas Search enabled

### Creating Vector Search Index via Atlas UI

1. Navigate to **Atlas Search** in your cluster
2. Click **"Create Search Index"**
3. Select **"JSON Editor"**
4. Paste the index definition:

```json
{
  "name": "metrics_vector_index",
  "type": "vectorSearch",
  "definition": {
    "fields": [
      {
        "type": "vector",
        "path": "embedding",
        "numDimensions": 1536,
        "similarity": "cosine"
      },
      {
        "type": "filter",
        "path": "session_id"
      },
      {
        "type": "filter",
        "path": "quantitative.detection_accuracy"
      },
      {
        "type": "filter",
        "path": "qualitative.rai_check_status"
      }
    ]
  }
}
```

5. Select the `agent_run_metrics` collection
6. Click **"Create Search Index"**

### Creating Index Programmatically

```python
from utils.mongodb_util import MongoDBClient

client = MongoDBClient()
success = client.create_vector_search_index("agent_run_metrics")
print(f"Vector index created: {success}")
```

---

## Python Dependencies

### Install Dependencies

```powershell
cd agentcert
pip install -r requirements.txt
```

The following packages are required:

```
pymongo[srv]==4.10.1    # MongoDB driver with SRV support
motor==3.6.0            # Async MongoDB driver
```

### Verify Installation

```python
import pymongo
import motor

print(f"pymongo version: {pymongo.version}")
print(f"motor version: {motor.version}")
```

---

## Verification

### Test Connection

```python
from utils.mongodb_util import MongoDBClient

# Create client
client = MongoDBClient()

# Health check
if client.health_check():
    print("✅ MongoDB connection successful")
else:
    print("❌ Connection failed")

# Initialize collections
results = client.initialize_collections()
for collection, success in results.items():
    print(f"  {'✅' if success else '❌'} {collection}")

# Get summary
summary = client.get_metrics_summary()
print(f"\nMetrics summary: {summary}")

client.close()
```

### Test Insertion

```python
from utils.mongodb_util import MongoDBClient

client = MongoDBClient()

# Insert quantitative data
quant_data = {
    "session_id": "test-session-001",
    "namespace": "default",
    "deployment_name": "test-app",
    "time_to_detection_seconds": 45.5,
    "detection_accuracy": "Correct",
    "submission_status": "VALID_SUBMISSION",
    "trajectory_steps": 10,
    "input_tokens": 15000,
    "output_tokens": 2500,
    "tool_calls": []
}

doc_id = client.insert_quantitative(quant_data)
print(f"Inserted document: {doc_id}")

# Retrieve by session
result = client.find_quantitative_by_session("test-session-001")
print(f"Retrieved: {result}")

client.close()
```

### Run Full Test

```powershell
cd agentcert
python -m utils.mongodb_util
```

---

## Troubleshooting

### Connection Issues

**Error: "Connection refused"**
```
pymongo.errors.ServerSelectionTimeoutError: localhost:27017
```

**Solution:**
- Check MongoDB service is running: `Get-Service MongoDB`
- Verify port 27017 is not blocked
- For Atlas: Check IP whitelist includes your IP

**Error: "Authentication failed"**
```
pymongo.errors.OperationFailure: Authentication failed
```

**Solution:**
- Verify username/password in connection string
- Check `authSource` parameter matches user's database
- For Atlas: Ensure user has correct roles

### Vector Search Issues

**Error: "Index not found"**
```
pymongo.errors.OperationFailure: PlanExecutor error: $vectorSearch
```

**Solution:**
- Vector Search requires Atlas (not available on local MongoDB)
- Verify index was created and is in "Active" status
- Check index name matches configuration

**Error: "Invalid vector dimensions"**

**Solution:**
- Ensure embeddings are exactly 1536 dimensions (OpenAI text-embedding-3-small)
- Verify `numDimensions` in index matches your embeddings

### Performance Tips

1. **Connection Pooling**: Reuse `MongoDBClient` instances
2. **Batch Inserts**: Use `insert_many()` for bulk operations
3. **Projection**: Exclude large fields (embeddings) when not needed
4. **Indexes**: Ensure indexes exist for frequently queried fields

---

## Next Steps

1. ✅ MongoDB installed and configured
2. ✅ Collections created with indexes
3. ⬜ Integrate with `MetricsExtractor` for automatic storage
4. ⬜ Add embedding generation for vector search
5. ⬜ Build analytics dashboard for metrics visualization

---

## References

- [MongoDB Atlas Documentation](https://www.mongodb.com/docs/atlas/)
- [MongoDB Atlas Vector Search](https://www.mongodb.com/docs/atlas/atlas-vector-search/vector-search-overview/)
- [PyMongo Documentation](https://pymongo.readthedocs.io/)
- [Motor (Async Driver) Documentation](https://motor.readthedocs.io/)
