// MongoDB Migration Script: Create Agent Registry Collection and Indexes
// Version: 1.0
// Date: 2026-01-16

// Switch to the litmus database (adjust database name as needed)
db = db.getSiblingDB('litmus');

// Create the agent_registry_collection if it doesn't exist
db.createCollection('agent_registry_collection', {
  validator: {
    $jsonSchema: {
      bsonType: 'object',
      required: ['agentId', 'projectId', 'name', 'version', 'capabilities', 'status', 'auditInfo'],
      properties: {
        agentId: {
          bsonType: 'string',
          description: 'Unique agent identifier (UUID)'
        },
        projectId: {
          bsonType: 'string',
          description: 'Project ID this agent belongs to'
        },
        name: {
          bsonType: 'string',
          pattern: '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$',
          maxLength: 63,
          description: 'Agent name (must be unique within project)'
        },
        version: {
          bsonType: 'string',
          description: 'Semantic version of the agent'
        },
        vendor: {
          bsonType: 'string',
          description: 'Agent vendor/organization'
        },
        capabilities: {
          bsonType: 'array',
          minItems: 1,
          items: {
            bsonType: 'string'
          },
          description: 'List of capabilities the agent supports'
        },
        containerImage: {
          bsonType: 'object',
          required: ['registry', 'repository', 'tag'],
          properties: {
            registry: { bsonType: 'string' },
            repository: { bsonType: 'string' },
            tag: { bsonType: 'string' }
          }
        },
        namespace: {
          bsonType: 'string',
          description: 'Kubernetes namespace where agent is deployed'
        },
        endpoint: {
          bsonType: 'object',
          required: ['url', 'type', 'discoveryType'],
          properties: {
            url: { bsonType: 'string' },
            type: { enum: ['REST', 'GRPC'] },
            discoveryType: { enum: ['AUTO', 'MANUAL'] },
            healthPath: { bsonType: 'string' },
            readyPath: { bsonType: 'string' }
          }
        },
        langfuseConfig: {
          bsonType: 'object',
          properties: {
            projectId: { bsonType: 'string' },
            syncEnabled: { bsonType: 'bool' },
            lastSyncedAt: { bsonType: 'long' }
          }
        },
        status: {
          enum: ['REGISTERED', 'VALIDATING', 'ACTIVE', 'INACTIVE', 'DELETED'],
          description: 'Current status of the agent'
        },
        metadata: {
          bsonType: 'object',
          properties: {
            labels: { bsonType: 'object' },
            annotations: { bsonType: 'object' }
          }
        },
        auditInfo: {
          bsonType: 'object',
          required: ['createdAt', 'createdBy', 'updatedAt', 'updatedBy'],
          properties: {
            createdAt: { bsonType: 'long' },
            createdBy: { bsonType: 'string' },
            updatedAt: { bsonType: 'long' },
            updatedBy: { bsonType: 'string' },
            lastHealthCheck: { bsonType: 'long' }
          }
        }
      }
    }
  }
});

print('✓ Created agent_registry_collection');

// Create indexes

// 1. Unique index on agentId (primary identifier)
db.agent_registry_collection.createIndex(
  { agentId: 1 },
  { 
    unique: true,
    name: 'idx_agentId'
  }
);
print('✓ Created unique index on agentId');

// 2. Compound unique index on (projectId, name) to enforce name uniqueness per project
db.agent_registry_collection.createIndex(
  { projectId: 1, name: 1 },
  { 
    unique: true,
    name: 'idx_projectId_name'
  }
);
print('✓ Created compound unique index on (projectId, name)');

// 3. Index on (status, auditInfo.createdAt) for filtering and sorting
db.agent_registry_collection.createIndex(
  { status: 1, 'auditInfo.createdAt': -1 },
  { 
    name: 'idx_status_createdAt'
  }
);
print('✓ Created index on (status, auditInfo.createdAt)');

// 4. Multikey index on capabilities for capability-based queries
db.agent_registry_collection.createIndex(
  { capabilities: 1 },
  { 
    name: 'idx_capabilities'
  }
);
print('✓ Created multikey index on capabilities');

// 5. Index on projectId for project-scoped queries
db.agent_registry_collection.createIndex(
  { projectId: 1 },
  { 
    name: 'idx_projectId'
  }
);
print('✓ Created index on projectId');

// 6. Text index for search functionality on name and vendor
db.agent_registry_collection.createIndex(
  { name: 'text', vendor: 'text' },
  { 
    name: 'idx_text_search',
    weights: {
      name: 10,
      vendor: 5
    }
  }
);
print('✓ Created text index for search');

// Verify indexes
print('\n=== Verification ===');
print('Collection stats:');
printjson(db.agent_registry_collection.stats());

print('\nIndexes created:');
printjson(db.agent_registry_collection.getIndexes());

print('\n=== Migration Complete ===');
print('Collection: agent_registry_collection');
print('Indexes: 6 created successfully');
print('Validation: JSON Schema applied');

// Sample query tests
print('\n=== Running Sample Queries ===');

// Test unique constraint (should fail on duplicate)
print('\nTesting unique constraint on agentId...');
try {
  db.agent_registry_collection.insertOne({
    agentId: 'test-agent-1',
    projectId: 'project-1',
    name: 'test-agent',
    version: '1.0.0',
    vendor: 'Test Vendor',
    capabilities: ['pod-crash-remediation'],
    status: 'REGISTERED',
    auditInfo: {
      createdAt: NumberLong(Date.now()),
      createdBy: 'admin',
      updatedAt: NumberLong(Date.now()),
      updatedBy: 'admin'
    }
  });
  print('✓ Inserted test document');
  
  // Try duplicate - should fail
  try {
    db.agent_registry_collection.insertOne({
      agentId: 'test-agent-1',
      projectId: 'project-2',
      name: 'duplicate-test',
      version: '1.0.0',
      vendor: 'Test Vendor',
      capabilities: ['pod-crash-remediation'],
      status: 'REGISTERED',
      auditInfo: {
        createdAt: NumberLong(Date.now()),
        createdBy: 'admin',
        updatedAt: NumberLong(Date.now()),
        updatedBy: 'admin'
      }
    });
    print('✗ Duplicate insert succeeded (should have failed!)');
  } catch (e) {
    print('✓ Duplicate agentId rejected correctly');
  }
  
  // Clean up test data
  db.agent_registry_collection.deleteOne({ agentId: 'test-agent-1' });
  print('✓ Cleaned up test data');
} catch (e) {
  print('✗ Test failed: ' + e);
}

print('\n=== Migration script completed successfully ===');

// Rollback instructions
/*
ROLLBACK PROCEDURE:
To rollback this migration, run the following commands:

db = db.getSiblingDB('litmus');
db.agent_registry_collection.drop();
print('Agent Registry collection dropped successfully');

This will remove the collection and all its indexes.
WARNING: This will delete all agent data permanently!
*/
