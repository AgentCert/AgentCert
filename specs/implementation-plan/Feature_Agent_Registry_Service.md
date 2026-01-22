# Feature: Agent Registry Service
## Implementation Plan

**Document Version**: 2.0

---

# Introduction

This implementation plan defines the complete development roadmap for the Agent Registry Service, a core backend component of the AgentCert platform that manages the complete lifecycle of AI agents within the chaos engineering ecosystem. The service provides **Helm-based agent onboarding** through Kubernetes event-driven discovery, agent metadata management, capability-based querying, health validation, and Langfuse synchronization through a GraphQL API backed by MongoDB.

## Agent Onboarding Approach

Agents are onboarded via **Helm charts** rather than manual registration:
1. User deploys agent using Helm chart containing agent workload and metadata ConfigMap
2. Kubernetes Watcher detects new resources with agent labels (`agentcert.io/agent=true`)
3. Watcher extracts metadata, validates, and automatically registers agent in database
4. Health checks validate agent availability
5. Metadata synced to Langfuse for observability
6. UI displays registered agents

## 1. Requirements & Constraints

### Functional Requirements

- **REQ-001**: Helm-based agent onboarding with automatic discovery via Kubernetes Watcher monitoring ConfigMaps/Deployments with label `agentcert.io/agent=true`
- **REQ-002**: Agent metadata extracted from ConfigMap data (agent-metadata.yaml or agent-metadata.json)
- **REQ-003**: Required metadata fields: name, version, projectId, capabilities, langfuseConfig; Optional: vendor, description, category
- **REQ-004**: Agent name must be unique within project/namespace scope
- **REQ-005**: Automatic endpoint discovery from Kubernetes Service using convention: `http://{agent-name}.{namespace}.svc.cluster.local:8080`
- **REQ-006**: Agent containers MUST expose `/health` and `/ready` endpoints
- **REQ-007**: Support CRUD operations for agent metadata with audit trail
- **REQ-008**: Capability-based querying using AND logic (agents must support ALL specified capabilities)
- **REQ-009**: Agent health validation via HTTP endpoints with status transitions: DISCOVERED → VALIDATING → ACTIVE → INACTIVE → DELETED
- **REQ-010**: Synchronize agent metadata to Langfuse on registration, update, status change, and deletion
- **REQ-011**: Graceful degradation when Langfuse sync fails (non-blocking)
- **REQ-012**: Support soft delete (mark inactive) and hard delete operations
- **REQ-013**: Idempotent registration: Helm upgrades trigger updates, not duplicate registrations (use Kubernetes resource UID as idempotency key)
- **REQ-014**: Watch events for ConfigMaps, Deployments, and Services in specified namespaces

### Non-Functional Requirements

- **NFR-001**: Registration latency < 500ms (excluding Langfuse sync)
- **NFR-002**: Query response time < 200ms for list operations (100 agents)
- **NFR-003**: Support 50+ concurrent GraphQL operations
- **NFR-004**: Support 1000+ registered agents per installation
- **NFR-005**: 99.9% uptime (follows GraphQL server availability)
- **NFR-006**: All database queries must use indexed fields

### Security Requirements

- **SEC-001**: JWT-based authentication via existing Auth Service
- **SEC-002**: Project-level RBAC (PROJECT_MEMBER for view, PROJECT_OWNER/PROJECT_ADMIN for mutate)
- **SEC-003**: Input sanitization to prevent injection attacks
- **SEC-004**: Langfuse API keys stored in Kubernetes Secrets
- **SEC-005**: Validate URLs against SSRF attacks (no localhost, private IPs)

### Constraints

- **CON-001**: Must leverage existing LitmusChaos patterns (MongoDB operators, GraphQL resolvers, gRPC clients)
- **CON-002**: Cannot modify existing GraphQL schema; only extend it
- **CON-003**: Must use Go 1.22+ following existing codebase conventions
- **CON-004**: Must use MongoDB 5.0+ with existing connection pool
- **CON-005**: GraphQL-first API maintaining consistency with existing LitmusChaos APIs
- **CON-006**: No separate service deployment; integrate as Go package in chaoscenter/graphql/server

### Guidelines

- **GUD-001**: Use structured logging with logrus and consistent field naming
- **GUD-002**: Expose Prometheus metrics for monitoring
- **GUD-003**: Follow idiomatic Go error handling patterns
- **GUD-004**: Write unit tests with 85%+ code coverage
- **GUD-005**: Document all public APIs and complex business logic

### Patterns

- **PAT-001**: Follow existing LitmusChaos three-layer architecture: Handler → Service → Operator
- **PAT-002**: Use MongoDB ObjectID for internal IDs, UUID for external agentId
- **PAT-003**: Implement exponential backoff for external API retries (Langfuse, K8s)
- **PAT-004**: Use context.Context for cancellation and timeout propagation
- **PAT-005**: Implement background Go routine for periodic health checks

## 2. Implementation Steps

### Implementation Phase 1: Foundation and Data Layer

**GOAL-001**: Establish project structure, data models, and MongoDB operator layer

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-001 | Create directory structure `chaoscenter/graphql/server/pkg/agent_registry/` with files: `handler.go`, `service.go`, `operator.go`, `model.go`, `validator.go`, `langfuse_client.go`, `watcher.go`, `metadata_extractor.go`, `health_scheduler.go`, `constants.go`, `errors.go`, `agent_registry_test.go` | | |
| TASK-002 | Define data models in `model.go`: `Agent` struct with fields (AgentID, ProjectID, Name, Version, Vendor, Description, Category, Capabilities, HelmRelease, KubernetesResources, Namespace, Endpoint, LangfuseConfig, Status, Metadata, AuditInfo), `HelmReleaseInfo` struct (ReleaseName, Namespace, ChartName, ChartVersion), `KubernetesResourceInfo` struct (DeploymentName, ServiceName, ConfigMapName, ResourceUID, Labels), `AgentEndpoint` struct with discovery type enum, `LangfuseConfig` struct, `AgentMetadata` struct for ConfigMap-based metadata extraction | | |
| TASK-003 | Define constants in `constants.go`: Collection name `agent_registry_collection`, status enums (DISCOVERED, VALIDATING, ACTIVE, INACTIVE, DELETED), endpoint discovery types (AUTO, MANUAL), endpoint types (REST, GRPC), Kubernetes label keys (`agentcert.io/agent`, `agentcert.io/agent-metadata`, `agentcert.io/project-id`), health check paths, timeouts | | |
| TASK-004 | Define custom errors in `errors.go`: ErrAgentNotFound, ErrDuplicateAgentName, ErrInvalidAgentName, ErrInvalidVersion, ErrInvalidCapabilities, ErrInvalidMetadata, ErrHealthCheckFailed, ErrLangfuseSyncFailed, ErrUnauthorized, ErrInsufficientPermissions, ErrK8sResourceNotFound | | |
| TASK-005 | Implement `Operator` interface in `operator.go` with methods: CreateAgent, GetAgent, GetAgentByProjectAndName, GetAgentByResourceUID, ListAgents, UpdateAgent, DeleteAgent, GetAgentsByCapabilities | | |
| TASK-006 | Implement `CreateAgent` method using `collection.InsertOne` with agent document | | |
| TASK-007 | Implement `GetAgent` method using `collection.FindOne` with filter `{"agentId": id}` and return ErrAgentNotFound if not found | | |
| TASK-008 | Implement `ListAgents` method with filter builder for projectId, status, capabilities, searchTerm; apply pagination with skip/limit; return agents array and total count using `collection.CountDocuments` | | |
| TASK-009 | Implement `UpdateAgent` method using `collection.ReplaceOne` with filter `{"agentId": agent.AgentID}` and update timestamps | | |
| TASK-010 | Implement `DeleteAgent` method using `collection.DeleteOne` with filter `{"agentId": id}` | | |
| TASK-011 | Implement `GetAgentsByCapabilities` method with filter using `$all` operator for capabilities array and projectId filter; use index hint for capabilities index | | |
| TASK-012 | Create MongoDB migration script `migrations/001_create_agent_registry.js` to create collection and indexes: unique index on agentId, compound unique index on (projectId, name), unique index on kubernetesResources.resourceUID, index on (status, auditInfo.discoveredAt), multikey index on capabilities | | |
| TASK-012A | Implement `AgentWatcher` in `watcher.go`: Define struct with k8sClient, service, metadataExtractor, logger fields; implement NewAgentWatcher constructor | | |
| TASK-012B | Implement `Start` method on AgentWatcher: Launch goroutines for watchConfigMaps, watchDeployments, watchServices with label selector `agentcert.io/agent=true` | | |
| TASK-012C | Implement `watchConfigMaps` method: Use k8sClient.CoreV1().ConfigMaps(namespace).Watch with label selector `agentcert.io/agent-metadata=true`, handle ADDED/MODIFIED/DELETED events | | |
| TASK-012D | Implement `handleAgentDiscovery` method: Extract metadata from ConfigMap, validate, check for existing agent by resourceUID (idempotency), auto-discover endpoint from Service, register agent with status DISCOVERED, trigger health check | | |
| TASK-012E | Implement `handleAgentUpdate` method: Find existing agent by resourceUID, extract updated metadata, call service.UpdateAgent | | |
| TASK-012F | Implement `handleAgentDeletion` method: Find agent by resourceUID, soft delete (mark as DELETED) | | |
| TASK-012G | Implement `MetadataExtractor` in `metadata_extractor.go`: Define interface with ExtractFromConfigMap | | |
| TASK-012H | Implement `ExtractFromConfigMap` method: Parse ConfigMap data with key `agent-metadata.yaml` or `agent-metadata.json` using yaml/json unmarshaling, validate required fields (name, version, projectId, capabilities, langfuseConfig), return AgentMetadata struct | | |

#### **CHECKPOINT-PHASE-1: Foundation and Data Layer Validation**

**Objective**: Verify that data models, MongoDB operator layer, and Kubernetes watcher components are correctly implemented before proceeding to validation logic.

**Prerequisites**: 
- All tasks TASK-001 through TASK-012H must be completed
- MongoDB instance must be accessible
- Go build must succeed without errors

**Validation Steps**:

1. **Code Compilation Check**
   ```powershell
   cd chaoscenter/graphql/server
   go build ./pkg/agent_registry/...
   ```
   - **Pass Criteria**: Build completes with exit code 0, no compilation errors
   - **Fail Action**: Fix syntax errors, missing imports, or type mismatches

2. **Data Model Validation**
   ```powershell
   go test ./pkg/agent_registry -run TestAgentModelSerialization -v
   ```
   Create test `TestAgentModelSerialization` to:
   - Create Agent struct with all required fields including helmRelease, kubernetesResources
   - Marshal to BSON and JSON
   - Unmarshal and verify all fields preserved
   - **Pass Criteria**: Test passes, all fields serialize/deserialize correctly
   - **Fail Action**: Fix BSON/JSON tags in model.go

3. **MongoDB Connection and Collection Creation**
   ```powershell
   go test ./pkg/agent_registry -run TestOperatorConnection -v
   ```
   Create test `TestOperatorConnection` to:
   - Create operator instance with real MongoDB (testcontainers)
   - Verify collection exists or gets created
   - **Pass Criteria**: Connection established, collection accessible
   - **Fail Action**: Check MongoDB connection string, network access, credentials

4. **MongoDB Index Verification**
   ```powershell
   # Run migration script
   mongosh --eval "load('migrations/001_create_agent_registry.js')"
   
   # Verify indexes
   mongosh --eval "db.agent_registry_collection.getIndexes()" | jq
   ```
   - **Pass Criteria**: Should show 5 indexes:
     - `_id` (default)
     - `agentId` (unique)
     - `projectId_1_name_1` (unique compound)
     - `kubernetesResources.resourceUID_1` (unique)
     - `status_1_auditInfo.discoveredAt_1` (compound)
     - `capabilities` (multikey)
   - **Fail Action**: Re-run migration script, check for errors

5. **Operator CRUD Operations**
   ```powershell
   go test ./pkg/agent_registry -run TestOperatorCRUD -v
   ```
   Create test `TestOperatorCRUD` to:
   - Call CreateAgent with complete agent object including helmRelease
   - Call GetAgent with agentId, verify returned
   - Call GetAgentByProjectAndName, verify found
   - Call GetAgentByResourceUID with kubernetesResources.resourceUID, verify found
   - Call UpdateAgent with modified fields, verify changes persisted
   - Call DeleteAgent, verify removed
   - **Pass Criteria**: All operations succeed, data integrity maintained
   - **Fail Action**: Debug operator methods, check MongoDB queries

6. **Watcher Component Initialization**
   ```powershell
   go test ./pkg/agent_registry -run TestWatcherInitialization -v
   ```
   Create test `TestWatcherInitialization` to:
   - Create mock k8s client
   - Initialize AgentWatcher
   - Verify watcher fields populated correctly
   - **Pass Criteria**: Watcher initializes without errors
   - **Fail Action**: Fix constructor, check dependencies

7. **Metadata Extractor Functionality**
   ```powershell
   go test ./pkg/agent_registry -run TestMetadataExtractorBasic -v
   ```
   Create test `TestMetadataExtractorBasic` to:
   - Create ConfigMap with valid agent-metadata.yaml
   - Call ExtractFromConfigMap
   - Verify AgentMetadata struct has all required fields
   - Test both YAML and JSON formats
   - **Pass Criteria**: Extraction succeeds for both formats, all fields populated
   - **Fail Action**: Fix parsing logic in metadata_extractor.go

8. **Constants and Errors Defined**
   ```powershell
   go test ./pkg/agent_registry -run TestConstantsAndErrors -v
   ```
   Create test `TestConstantsAndErrors` to:
   - Verify all status enums defined (DISCOVERED, VALIDATING, ACTIVE, INACTIVE, DELETED)
   - Verify all custom errors defined (11 error types)
   - Verify Kubernetes label keys defined
   - **Pass Criteria**: All constants accessible, errors instantiable
   - **Fail Action**: Add missing constants/errors to constants.go and errors.go

**Overall Phase 1 Pass Criteria**:
- ✅ All 8 validation steps pass
- ✅ Code coverage for operator.go, model.go, watcher.go, metadata_extractor.go >= 80%
- ✅ No critical linting errors: `golangci-lint run ./pkg/agent_registry/...`
- ✅ MongoDB indexes verified via `getIndexes()` command

**Rollback Actions** (if checkpoint fails):
1. Review failed test output
2. Fix identified issues in corresponding files
3. Re-run specific failed validation step
4. Once all validations pass, proceed to Phase 2

---

### Implementation Phase 2: Validation Layer

**GOAL-002**: Implement input validation and business rule enforcement

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-013 | Implement `Validator` interface in `validator.go` with methods: ValidateRegistration, ValidateUpdate, ValidateCapabilities, ValidateMetadata | | |
| TASK-014 | Implement `ValidateRegistration` method: check name uniqueness by calling operator.GetAgentByProjectAndName, validate name format using regex `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` with max 63 chars, validate version is valid semver using regex `^v?(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9.-]+))?(?:\+([a-zA-Z0-9.-]+))?$`, validate capabilities not empty | | |
| TASK-015 | Implement `ValidateCapabilities` method: check each capability exists in taxonomy map loaded from configuration or hardcoded list of supported capabilities | | |
| TASK-016 | Implement `ValidateMetadata` method: check required fields (name, version, projectId, capabilities, langfuseConfig) are non-empty, validate projectId exists in project collection, validate langfuseConfig has projectId and syncEnabled fields, validate Helm release info if provided | | |
| TASK-017 | Implement `ValidateUpdate` method: if name provided validate format, if version provided validate semver, if capabilities provided validate against taxonomy | | |
| TASK-018 | Implement helper function `loadCapabilitiesTaxonomy` returning map with predefined capabilities: pod-crash-remediation, pod-delete-remediation, node-drain-remediation, network-latency-remediation, network-partition-remediation, disk-pressure-remediation, memory-stress-remediation, cpu-stress-remediation, container-kill-remediation, service-unavailable-remediation | | |

#### **CHECKPOINT-PHASE-2: Validation Layer Verification**

**Objective**: Ensure all input validation and business rules are correctly enforced before implementing service logic.

**Prerequisites**:
- Phase 1 checkpoint passed
- All tasks TASK-013 through TASK-018 completed
- validator.go implements all validation methods

**Validation Steps**:

1. **Name Format Validation**
   ```powershell
   go test ./pkg/agent_registry -run TestValidateName -v
   ```
   Create test `TestValidateName` to verify:
   - Valid names pass: `agent-1`, `my-agent`, `a`, `agent-name-123`
   - Invalid names fail: `Agent-1`, `agent_name`, `-agent`, `agent-`, `agent name`, 64+ chars
   - **Pass Criteria**: All valid names accepted, all invalid rejected with ErrInvalidAgentName
   - **Fail Action**: Fix regex in ValidateRegistration method

2. **Semver Validation**
   ```powershell
   go test ./pkg/agent_registry -run TestValidateSemver -v
   ```
   Create test `TestValidateSemver` to verify:
   - Valid versions pass: `1.0.0`, `v2.1.3`, `1.0.0-alpha`, `1.0.0+build`
   - Invalid versions fail: `1.0`, `v1`, `1.0.0.0`
   - **Pass Criteria**: All valid semver accepted, invalid rejected with ErrInvalidVersion
   - **Fail Action**: Fix semver regex in validator

3. **Capabilities Taxonomy Validation**
   ```powershell
   go test ./pkg/agent_registry -run TestValidateCapabilities -v
   ```
   Create test `TestValidateCapabilities` to:
   - Load capabilities taxonomy map
   - Verify all 10 predefined capabilities present
   - Test validation with valid capability: `pod-crash-remediation` (should pass)
   - Test with invalid capability: `unknown-capability` (should fail)
   - Test with empty capabilities array (should fail)
   - **Pass Criteria**: Known capabilities accepted, unknown rejected with ErrInvalidCapabilities
   - **Fail Action**: Update loadCapabilitiesTaxonomy or validation logic

4. **Metadata Validation**
   ```powershell
   go test ./pkg/agent_registry -run TestValidateMetadata -v
   ```
   Create test `TestValidateMetadata` to:
   - Test with all required fields present (name, version, projectId, capabilities, langfuseConfig) - should pass
   - Test with missing name - should fail
   - Test with missing version - should fail
   - Test with missing projectId - should fail
   - Test with missing capabilities - should fail
   - Test with missing langfuseConfig - should fail
   - Test with invalid langfuseConfig (missing projectId or syncEnabled) - should fail
   - **Pass Criteria**: Complete metadata passes, any missing required field fails with ErrInvalidMetadata
   - **Fail Action**: Fix required field checks in ValidateMetadata

5. **Registration Validation Integration**
   ```powershell
   go test ./pkg/agent_registry -run TestValidateRegistration -v
   ```
   Create test `TestValidateRegistration` to:
   - Mock operator.GetAgentByProjectAndName returning nil (name available)
   - Call ValidateRegistration with valid agent metadata
   - Verify passes with no errors
   - Mock operator returning existing agent (name taken)
   - Call ValidateRegistration with duplicate name
   - Verify fails with ErrDuplicateAgentName
   - **Pass Criteria**: Unique name accepted, duplicate rejected
   - **Fail Action**: Fix uniqueness check in ValidateRegistration

6. **Update Validation**
   ```powershell
   go test ./pkg/agent_registry -run TestValidateUpdate -v
   ```
   Create test `TestValidateUpdate` to:
   - Call with valid name change - should pass
   - Call with invalid name format - should fail
   - Call with valid version change - should pass
   - Call with invalid semver - should fail
   - Call with valid capabilities - should pass
   - Call with invalid capability - should fail
   - **Pass Criteria**: Valid updates pass, invalid formats rejected
   - **Fail Action**: Fix ValidateUpdate logic

7. **End-to-End Validation Flow**
   ```powershell
   go test ./pkg/agent_registry -run TestValidatorE2E -v
   ```
   Create test `TestValidatorE2E` to:
   - Create complete agent metadata with all fields
   - Call all validator methods in sequence
   - Verify comprehensive validation works correctly
   - **Pass Criteria**: Complete validation flow succeeds for valid input
   - **Fail Action**: Debug validator integration

**Overall Phase 2 Pass Criteria**:
- ✅ All 7 validation steps pass
- ✅ Code coverage for validator.go >= 85%
- ✅ All edge cases covered (empty strings, nil values, boundary values)
- ✅ Error messages are descriptive and include field names

**Rollback Actions** (if checkpoint fails):
1. Identify which validation test failed
2. Review validation logic in validator.go
3. Fix regex patterns, required field checks, or taxonomy lookup
4. Re-run failed validation test
5. Once all validations pass, proceed to Phase 3

---

### Implementation Phase 3: Service Layer - Core Logic

**GOAL-003**: Implement business logic layer with agent lifecycle operations

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-019 | Implement `Service` interface in `service.go` with methods: RegisterAgentFromWatcher, GetAgent, GetAgentByResourceUID, ListAgents, UpdateAgent, DeleteAgent, GetAgentsByCapabilities, ValidateAgentHealth, ValidateAgentMetadata, SyncToLangfuse, GetCapabilitiesTaxonomy | | |
| TASK-020 | Implement `RegisterAgentFromWatcher` method: check for duplicate by resourceUID using operator.GetAgentByResourceUID, if exists log warning and return, validate agent metadata, create Agent struct with status DISCOVERED and auditInfo (discoveredAt=now), call operator.CreateAgent, asynchronously call SyncToLangfuse in goroutine with error logging, initiate health check asynchronously to transition to VALIDATING status, return agent | | |
| TASK-021 | Implement `discoverAgentEndpoint` helper: use k8sClient.CoreV1().Services(namespace).Get to find service matching agent name, if found construct endpoint `http://{serviceName}.{namespace}.svc.cluster.local:{port}`, set discoveryType=AUTO, healthPath="/health", readyPath="/ready", if not found return error with suggestion to provide manual endpoint | | |
| TASK-022 | Implement `GetAgent` method: call operator.GetAgent(id), verify user has access to agent's project via JWT context, return agent or authorization error | | |
| TASK-023 | Implement `ListAgents` method: build AgentFilter from input parameters, verify user project access, call operator.ListAgents with filter and pagination, return AgentListResponse with agents array, totalCount, pageInfo (currentPage, totalPages, hasNextPage) | | |
| TASK-024 | Implement `UpdateAgent` method: validate input using validator.ValidateUpdate, call operator.GetAgent to fetch existing agent, verify user authorization (PROJECT_OWNER or PROJECT_ADMIN), merge updates into agent struct preserving non-updated fields, update auditInfo.updatedAt and updatedBy, call operator.UpdateAgent, asynchronously sync to Langfuse if metadata changed, return updated agent | | |
| TASK-025 | Implement `DeleteAgent` method: call operator.GetAgent to verify exists, verify user authorization (PROJECT_OWNER or PROJECT_ADMIN), check for active benchmarks using agent (future: query benchmark service), if hardDelete=true call operator.DeleteAgent else update status to DELETED and call operator.UpdateAgent, asynchronously sync deletion to Langfuse, return DeleteAgentResponse with success=true | | |
| TASK-026 | Implement `GetAgentsByCapabilities` method: verify user project access, call operator.GetAgentsByCapabilities with projectId and capabilities list, return filtered agents | | |
| TASK-027 | Implement `GetCapabilitiesTaxonomy` method: return list of CapabilityDefinition structs with id, name, description, category fields | | |
| TASK-027A | Implement `ValidateAgentMetadata` method: check required fields (name, version, projectId, capabilities), validate name format, validate semver, validate capabilities against taxonomy | | |
| TASK-027B | Implement `GetAgentByResourceUID` method: call operator.GetAgentByResourceUID(resourceUID), return agent or ErrAgentNotFound | | |

#### **CHECKPOINT-PHASE-3: Service Layer Core Logic Validation**

**Objective**: Verify core business logic including agent registration, retrieval, updates, and capability queries work correctly.

**Prerequisites**:
- Phase 2 checkpoint passed
- All tasks TASK-019 through TASK-027B completed
- service.go implements all core methods

**Validation Steps**:

1. **Agent Registration from Watcher**
   ```powershell
   go test ./pkg/agent_registry -run TestRegisterAgentFromWatcher -v
   ```
   Create test `TestRegisterAgentFromWatcher` to:
   - Mock operator.GetAgentByResourceUID returning nil (no duplicate)
   - Mock operator.CreateAgent succeeding
   - Mock k8sClient returning service for endpoint discovery
   - Call RegisterAgentFromWatcher with complete metadata
   - Verify agent created with status DISCOVERED
   - Verify UUID generated for agentId
   - Verify auditInfo.discoveredAt timestamp set
   - Verify helmRelease and kubernetesResources populated
   - **Pass Criteria**: Agent registered successfully, all fields populated
   - **Fail Action**: Debug RegisterAgentFromWatcher method, check field mappings

2. **Idempotency Check (Duplicate ResourceUID)**
   ```powershell
   go test ./pkg/agent_registry -run TestRegisterAgentIdempotency -v
   ```
   Create test `TestRegisterAgentIdempotency` to:
   - Mock operator.GetAgentByResourceUID returning existing agent
   - Call RegisterAgentFromWatcher with same resourceUID
   - Verify returns early with log warning, no duplicate created
   - **Pass Criteria**: No duplicate registration, existing agent returned/logged
   - **Fail Action**: Fix duplicate check in RegisterAgentFromWatcher

3. **Endpoint Auto-Discovery**
   ```powershell
   go test ./pkg/agent_registry -run TestEndpointDiscovery -v
   ```
   Create test `TestEndpointDiscovery` to:
   - Mock k8sClient.CoreV1().Services().Get() returning Service with ClusterIP
   - Call discoverAgentEndpoint helper
   - Verify endpoint constructed as `http://{serviceName}.{namespace}.svc.cluster.local:8080`
   - Verify discoveryType set to AUTO
   - Verify healthPath and readyPath set
   - **Pass Criteria**: Endpoint correctly discovered and formatted
   - **Fail Action**: Fix discoverAgentEndpoint helper logic

4. **Get Agent Operations**
   ```powershell
   go test ./pkg/agent_registry -run TestGetAgentOperations -v
   ```
   Create test `TestGetAgentOperations` to:
   - Test GetAgent: mock operator returning agent, verify returned correctly
   - Test GetAgent with invalid ID: verify ErrAgentNotFound returned
   - Test GetAgentByResourceUID: mock operator, verify agent found by resourceUID
   - **Pass Criteria**: All get operations return correct data or appropriate errors
   - **Fail Action**: Fix GetAgent or GetAgentByResourceUID methods

5. **List Agents with Filters and Pagination**
   ```powershell
   go test ./pkg/agent_registry -run TestListAgentsWithFilters -v
   ```
   Create test `TestListAgentsWithFilters` to:
   - Mock operator.ListAgents to return 25 agents total
   - Call with pagination (page=2, limit=10)
   - Verify returns agents 11-20
   - Verify totalCount=25, totalPages=3, currentPage=2, hasNextPage=true
   - Call with status filter (ACTIVE only)
   - Verify filter passed to operator correctly
   - **Pass Criteria**: Pagination math correct, filters applied properly
   - **Fail Action**: Fix pagination logic or filter building in ListAgents

6. **Update Agent**
   ```powershell
   go test ./pkg/agent_registry -run TestUpdateAgent -v
   ```
   Create test `TestUpdateAgent` to:
   - Mock operator.GetAgent returning existing agent
   - Mock validator.ValidateUpdate passing
   - Mock operator.UpdateAgent succeeding
   - Call UpdateAgent with partial update (version only)
   - Verify only version changed, other fields preserved
   - Verify auditInfo.updatedAt timestamp updated
   - **Pass Criteria**: Partial updates work, timestamps updated, other fields unchanged
   - **Fail Action**: Fix field merging logic in UpdateAgent

7. **Delete Agent (Soft and Hard)**
   ```powershell
   go test ./pkg/agent_registry -run TestDeleteAgent -v
   ```
   Create test `TestDeleteAgent` to:
   - Test soft delete: Call with hardDelete=false, verify status set to DELETED, UpdateAgent called
   - Test hard delete: Call with hardDelete=true, verify operator.DeleteAgent called
   - **Pass Criteria**: Both delete modes work correctly
   - **Fail Action**: Fix DeleteAgent method logic

8. **Capability-Based Query**
   ```powershell
   go test ./pkg/agent_registry -run TestGetAgentsByCapabilities -v
   ```
   Create test `TestGetAgentsByCapabilities` to:
   - Mock operator.GetAgentsByCapabilities returning filtered agents
   - Call with capabilities [cap1, cap2]
   - Verify correct agents returned (must have ALL capabilities)
   - **Pass Criteria**: Capability filtering works with AND logic
   - **Fail Action**: Verify operator query uses $all operator correctly

9. **Capabilities Taxonomy Retrieval**
   ```powershell
   go test ./pkg/agent_registry -run TestGetCapabilitiesTaxonomy -v
   ```
   Create test `TestGetCapabilitiesTaxonomy` to:
   - Call GetCapabilitiesTaxonomy
   - Verify returns list of CapabilityDefinition structs
   - Verify all 10 predefined capabilities present with descriptions
   - **Pass Criteria**: Taxonomy complete and formatted correctly
   - **Fail Action**: Fix taxonomy data structure or retrieval logic

10. **Integration Test: Full Registration Flow**
    ```powershell
    go test ./pkg/agent_registry -run TestServiceRegistrationFlow -v
    ```
    Create test `TestServiceRegistrationFlow` to:
    - Use testcontainers for real MongoDB
    - Create real operator, validator, service instances (no mocks)
    - Call RegisterAgentFromWatcher with complete metadata
    - Call GetAgent to retrieve
    - Call UpdateAgent to modify version
    - Call ListAgents to verify appears in list
    - Call DeleteAgent (soft)
    - Verify status DELETED
    - **Pass Criteria**: Complete flow works end-to-end with real database
    - **Fail Action**: Debug integration issues, check database state

**Overall Phase 3 Pass Criteria**:
- ✅ All 10 validation steps pass
- ✅ Code coverage for service.go >= 85%
- ✅ All CRUD operations functional
- ✅ Endpoint discovery working
- ✅ Idempotency enforced
- ✅ Pagination and filtering correct

**Rollback Actions** (if checkpoint fails):
1. Identify which service test failed
2. Check operator and validator mocks are correct
3. Debug service method implementation
4. Verify database operations succeed
5. Re-run failed test
6. Once all validations pass, proceed to Phase 4

---

### Implementation Phase 4: Service Layer - Health Checks

**GOAL-004**: Implement agent health validation with status transitions

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-028 | Implement `ValidateAgentHealth` method: call operator.GetAgent to fetch agent, verify user authorization, create HTTP client with 5s timeout, perform GET request to agent.Endpoint.URL + agent.Endpoint.HealthPath, check status code 200, parse response if JSON health format expected, perform GET request to agent.Endpoint.URL + agent.Endpoint.ReadyPath, check status code 200, create HealthCheckResult with healthy boolean, message, responseTime, checkedAt timestamp, update agent status based on health result (healthy → ACTIVE, unhealthy → INACTIVE), update agent.AuditInfo.lastHealthCheck, call operator.UpdateAgent, return HealthCheckResult | | |
| TASK-029 | Implement `updateAgentStatus` helper: accept agent and newStatus, perform status validation (valid transitions only), update agent.Status and agent.AuditInfo.updatedAt, call operator.UpdateAgent, log status transition, asynchronously sync to Langfuse | | |
| TASK-030 | Implement health check scheduler in `health_scheduler.go`: define HealthCheckScheduler struct with service, interval (default 5m), stopChan, logger fields | | |
| TASK-031 | Implement `NewHealthCheckScheduler` function returning initialized scheduler with provided interval | | |
| TASK-032 | Implement `Start` method on HealthCheckScheduler: run infinite loop with time.NewTicker(interval), on each tick call runHealthChecks, listen on stopChan for graceful shutdown | | |
| TASK-033 | Implement `runHealthChecks` method: fetch all agents with status ACTIVE or VALIDATING using service.ListAgents with appropriate filter, iterate over agents, for each agent call service.ValidateAgentHealth, log results (successful checks, failed checks, errors), use semaphore or worker pool to limit concurrent checks to 10 | | |
| TASK-034 | Implement `Stop` method on HealthCheckScheduler: close stopChan to signal shutdown, wait for current health check cycle to complete | | |

#### **CHECKPOINT-PHASE-4: Health Check System Validation**

**Objective**: Verify health check functionality, status transitions, and scheduler operations work correctly.

**Prerequisites**:
- Phase 3 checkpoint passed
- All tasks TASK-028 through TASK-034 completed
- health_scheduler.go implemented

**Validation Steps**:

1. **Health Check Success**
   ```powershell
   go test ./pkg/agent_registry -run TestValidateAgentHealthSuccess -v
   ```
   Create test `TestValidateAgentHealthSuccess` to:
   - Create mock HTTP server responding 200 OK to /health and /ready endpoints
   - Register agent with mock server endpoint
   - Call ValidateAgentHealth
   - Verify HealthCheckResult.healthy = true
   - Verify agent status transitions to ACTIVE
   - Verify auditInfo.lastHealthCheck timestamp updated
   - Verify responseTime recorded
   - **Pass Criteria**: Health check succeeds, status transitions correctly
   - **Fail Action**: Debug ValidateAgentHealth HTTP client logic

2. **Health Check Failure**
   ```powershell
   go test ./pkg/agent_registry -run TestValidateAgentHealthFailure -v
   ```
   Create test `TestValidateAgentHealthFailure` to:
   - Create mock HTTP server responding 503 Service Unavailable to /health
   - Call ValidateAgentHealth
   - Verify HealthCheckResult.healthy = false
   - Verify agent status transitions to INACTIVE
   - Verify error message captured in HealthCheckResult
   - **Pass Criteria**: Unhealthy agent marked INACTIVE with error details
   - **Fail Action**: Fix health check error handling

3. **Health Check Timeout**
   ```powershell
   go test ./pkg/agent_registry -run TestValidateAgentHealthTimeout -v
   ```
   Create test `TestValidateAgentHealthTimeout` to:
   - Create mock HTTP server with 10-second delay
   - Configure health check with 2-second timeout
   - Call ValidateAgentHealth
   - Verify returns timeout error
   - Verify agent status transitions to INACTIVE
   - Verify message indicates timeout
   - **Pass Criteria**: Timeout handled, agent marked INACTIVE
   - **Fail Action**: Fix HTTP client timeout configuration

4. **Status Transition Validation**
   ```powershell
   go test ./pkg/agent_registry -run TestUpdateAgentStatus -v
   ```
   Create test `TestUpdateAgentStatus` to:
   - Test valid transitions:
     - DISCOVERED → VALIDATING → ACTIVE (should succeed)
     - ACTIVE → INACTIVE (should succeed)
     - INACTIVE → ACTIVE (should succeed)
   - Verify updateAgentStatus helper updates status and timestamp
   - Verify operator.UpdateAgent called
   - **Pass Criteria**: Valid transitions succeed, invalid ones rejected if applicable
   - **Fail Action**: Fix updateAgentStatus helper logic

5. **Health Check Scheduler Initialization**
   ```powershell
   go test ./pkg/agent_registry -run TestHealthCheckSchedulerInit -v
   ```
   Create test `TestHealthCheckSchedulerInit` to:
   - Create scheduler with 5-minute interval
   - Verify fields populated correctly
   - Verify no goroutines leaked before Start() called
   - **Pass Criteria**: Scheduler initializes correctly
   - **Fail Action**: Fix NewHealthCheckScheduler constructor

6. **Health Check Scheduler Periodic Execution**
   ```powershell
   go test ./pkg/agent_registry -run TestHealthCheckSchedulerPeriodicExecution -v
   ```
   Create test `TestHealthCheckSchedulerPeriodicExecution` to:
   - Create service mock tracking ValidateAgentHealth call count
   - Create scheduler with 200ms interval
   - Start scheduler in goroutine
   - Sleep for 650ms (should complete ~3 cycles)
   - Stop scheduler
   - Verify ValidateAgentHealth called approximately 3 times
   - Verify no goroutine leaks after Stop()
   - **Pass Criteria**: Scheduler runs periodically at correct interval
   - **Fail Action**: Fix ticker logic in Start() method

7. **Health Check Batch Processing**
   ```powershell
   go test ./pkg/agent_registry -run TestRunHealthChecksBatch -v
   ```
   Create test `TestRunHealthChecksBatch` to:
   - Mock service.ListAgents to return 15 agents (status ACTIVE or VALIDATING)
   - Mock health check servers for all agents
   - Call runHealthChecks
   - Verify all 15 agents checked
   - Verify concurrency limited to 10 (use semaphore/worker pool)
   - Verify execution completes in reasonable time
   - **Pass Criteria**: All agents checked, concurrency controlled
   - **Fail Action**: Fix runHealthChecks concurrency logic

8. **Health Check Scheduler Graceful Shutdown**
   ```powershell
   go test ./pkg/agent_registry -run TestHealthCheckSchedulerShutdown -v
   ```
   Create test `TestHealthCheckSchedulerShutdown` to:
   - Start scheduler
   - Immediately call Stop()
   - Verify stopChan closed
   - Verify Start() goroutine exits without panic
   - Wait 1 second and verify no further health checks executed
   - **Pass Criteria**: Clean shutdown, no panics, no hanging goroutines
   - **Fail Action**: Fix Stop() method and shutdown signal handling

9. **Integration Test: Status Lifecycle**
   ```powershell
   go test ./pkg/agent_registry -run TestHealthCheckStatusLifecycle -v
   ```
   Create test `TestHealthCheckStatusLifecycle` to:
   - Use testcontainers for MongoDB
   - Create real service and operator instances
   - Create mock HTTP server for agent endpoint
   - Register agent (status DISCOVERED)
   - Call ValidateAgentHealth (should transition to ACTIVE)
   - Stop mock server
   - Call ValidateAgentHealth (should transition to INACTIVE)
   - Restart mock server
   - Call ValidateAgentHealth (should transition back to ACTIVE)
   - Verify all transitions persisted in MongoDB
   - **Pass Criteria**: Complete status lifecycle works end-to-end
   - **Fail Action**: Debug status transition persistence

**Overall Phase 4 Pass Criteria**:
- ✅ All 9 validation steps pass
- ✅ Code coverage for health check methods >= 85%
- ✅ Health checks complete within configured timeout
- ✅ Status transitions correctly persisted
- ✅ Scheduler runs reliably at configured interval
- ✅ Graceful shutdown works without leaks

**Rollback Actions** (if checkpoint fails):
1. Identify which health check test failed
2. Review HTTP client configuration and timeout settings
3. Debug status transition logic
4. Check scheduler ticker and goroutine management
5. Re-run failed test
6. Once all validations pass, proceed to Phase 5

---

### Implementation Phase 5: Langfuse Integration

**GOAL-005**: Implement Langfuse API client with retry logic for metadata synchronization

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-035 | Implement `LangfuseClient` interface in `langfuse_client.go` with methods: CreateOrUpdateUser, DeleteUser | | |
| TASK-036 | Define `LangfuseUserPayload` struct with fields: ID (agentId), Name (agent name), Metadata (map with version, vendor, category, capabilities, helmRelease, langfuseProjectId, status, namespace, discoveredAt, updatedAt) | | |
| TASK-037 | Implement `NewLangfuseClient` function: accept baseURL and apiKey from environment variables, create HTTP client with 10s timeout, return langfuseClientImpl | | |
| TASK-038 | Implement `CreateOrUpdateUser` method: marshal payload to JSON, create HTTP POST request to `{baseURL}/api/public/users`, set headers (Authorization: Bearer {apiKey}, Content-Type: application/json), set projectId in request context or headers as required by Langfuse API, implement retry logic with exponential backoff (3 retries, delays 1s, 2s, 4s), check response status 200/201, log errors on failure, return error if all retries fail | | |
| TASK-039 | Implement `DeleteUser` method: create payload with Metadata containing deleted:true flag and deletedAt timestamp, call CreateOrUpdateUser with updated payload (Langfuse doesn't support hard delete) | | |
| TASK-040 | Implement `SyncToLangfuse` method in service layer: check if agent.LangfuseConfig is nil or syncEnabled=false and return early if disabled, check if LANGFUSE_API_KEY environment variable is set, build LangfuseUserPayload from agent fields, call langfuseClient.CreateOrUpdateUser, update agent.LangfuseConfig.lastSyncedAt timestamp, call operator.UpdateAgent to persist sync timestamp, log sync success or failure, return error only if sync is critical (currently non-blocking) | | |

#### **CHECKPOINT-PHASE-5: Langfuse Integration Validation**

**Objective**: Verify Langfuse API client correctly synchronizes agent metadata with retry logic and handles failures gracefully.

**Prerequisites**:
- Phase 4 checkpoint passed
- All tasks TASK-035 through TASK-040 completed
- langfuse_client.go implemented

**Validation Steps**:

1. **Langfuse Client Initialization**
   ```powershell
   go test ./pkg/agent_registry -run TestLangfuseClientInit -v
   ```
   Create test `TestLangfuseClientInit` to:
   - Set environment variables LANGFUSE_BASE_URL and LANGFUSE_API_KEY
   - Call NewLangfuseClient
   - Verify client initialized with correct baseURL
   - Verify HTTP client has 10s timeout
   - **Pass Criteria**: Client initializes successfully
   - **Fail Action**: Fix NewLangfuseClient constructor

2. **Create/Update User Success**
   ```powershell
   go test ./pkg/agent_registry -run TestLangfuseCreateUserSuccess -v
   ```
   Create test `TestLangfuseCreateUserSuccess` to:
   - Create mock HTTP server responding 201 to POST /api/public/users
   - Build LangfuseUserPayload with complete agent data
   - Call CreateOrUpdateUser
   - Verify request has Authorization header with Bearer token
   - Verify Content-Type is application/json
   - Verify payload contains ID (agentId), Name, Metadata fields
   - Verify Metadata includes version, vendor, capabilities, etc.
   - **Pass Criteria**: Request formatted correctly, success status returned
   - **Fail Action**: Fix CreateOrUpdateUser request building

3. **Langfuse Retry Logic**
   ```powershell
   go test ./pkg/agent_registry -run TestLangfuseRetryLogic -v
   ```
   Create test `TestLangfuseRetryLogic` to:
   - Create mock HTTP server that:
     - Returns 500 on first request
     - Returns 500 on second request
     - Returns 201 on third request
   - Call CreateOrUpdateUser
   - Verify exactly 3 HTTP requests made
   - Verify delays between requests approximately 1s, 2s (exponential backoff)
   - Verify final success returned
   - **Pass Criteria**: Retry mechanism works with exponential backoff
   - **Fail Action**: Fix retry loop and backoff timing

4. **Langfuse All Retries Fail**
   ```powershell
   go test ./pkg/agent_registry -run TestLangfuseAllRetriesFail -v
   ```
   Create test `TestLangfuseAllRetriesFail` to:
   - Create mock HTTP server always returning 500
   - Call CreateOrUpdateUser
   - Verify exactly 3 attempts made
   - Verify error returned after final retry
   - Verify error message descriptive
   - **Pass Criteria**: Gives up after 3 retries, returns error
   - **Fail Action**: Fix retry limit and error handling

5. **Delete User (Soft Delete)**
   ```powershell
   go test ./pkg/agent_registry -run TestLangfuseDeleteUser -v
   ```
   Create test `TestLangfuseDeleteUser` to:
   - Create mock HTTP server capturing request payload
   - Call DeleteUser
   - Verify CreateOrUpdateUser called with payload containing:
     - Metadata.deleted = true
     - Metadata.deletedAt timestamp
   - **Pass Criteria**: Soft delete payload correct
   - **Fail Action**: Fix DeleteUser method

6. **Service Layer Sync to Langfuse Success**
   ```powershell
   go test ./pkg/agent_registry -run TestSyncToLangfuseSuccess -v
   ```
   Create test `TestSyncToLangfuseSuccess` to:
   - Mock langfuseClient.CreateOrUpdateUser returning nil (success)
   - Mock operator.UpdateAgent succeeding
   - Create agent with langfuseConfig.syncEnabled = true
   - Call SyncToLangfuse
   - Verify CreateOrUpdateUser called with correct payload
   - Verify agent.LangfuseConfig.lastSyncedAt timestamp updated
   - Verify operator.UpdateAgent called to persist timestamp
   - **Pass Criteria**: Sync succeeds, timestamp persisted
   - **Fail Action**: Debug SyncToLangfuse method

7. **Sync Disabled (Early Return)**
   ```powershell
   go test ./pkg/agent_registry -run TestSyncToLangfuseDisabled -v
   ```
   Create test `TestSyncToLangfuseDisabled` to:
   - Test Case 1: agent.LangfuseConfig = nil
   - Test Case 2: agent.LangfuseConfig.syncEnabled = false
   - For both cases:
     - Call SyncToLangfuse
     - Verify langfuseClient NOT called
     - Verify returns immediately with nil error
   - **Pass Criteria**: Sync skipped when disabled, no API calls made
   - **Fail Action**: Fix early return logic in SyncToLangfuse

8. **Sync Non-Blocking on Failure**
   ```powershell
   go test ./pkg/agent_registry -run TestSyncToLangfuseNonBlocking -v
   ```
   Create test `TestSyncToLangfuseNonBlocking` to:
   - Mock langfuseClient.CreateOrUpdateUser returning error
   - Call SyncToLangfuse
   - Verify error logged but returned as nil (non-blocking)
   - Verify agent registration/update not rolled back
   - **Pass Criteria**: Langfuse failure doesn't block agent operations
   - **Fail Action**: Ensure graceful degradation implemented

9. **Integration Test: Agent Registration with Langfuse Sync**
   ```powershell
   go test ./pkg/agent_registry -run TestAgentRegistrationWithLangfuseSync -v
   ```
   Create test `TestAgentRegistrationWithLangfuseSync` to:
   - Use testcontainers for MongoDB
   - Create mock Langfuse HTTP server
   - Create real service with real Langfuse client
   - Call RegisterAgentFromWatcher with syncEnabled=true
   - Verify agent created in MongoDB
   - Verify Langfuse API received CreateOrUpdateUser request
   - Verify agent.LangfuseConfig.lastSyncedAt populated
   - Update agent
   - Verify Langfuse API received update request
   - Delete agent
   - Verify Langfuse API received soft delete payload
   - **Pass Criteria**: Complete sync flow works end-to-end
   - **Fail Action**: Debug integration between service and Langfuse client

**Overall Phase 5 Pass Criteria**:
- ✅ All 9 validation steps pass
- ✅ Code coverage for langfuse_client.go >= 85%
- ✅ Retry logic works with exponential backoff
- ✅ Graceful degradation on Langfuse failures
- ✅ Sync timestamps correctly persisted
- ✅ Soft delete mechanism working

**Rollback Actions** (if checkpoint fails):
1. Identify which Langfuse test failed
2. Review HTTP client request building
3. Debug retry logic and backoff timing
4. Check error handling and logging
5. Re-run failed test
6. Once all validations pass, proceed to Phase 6

---

### Implementation Phase 6: GraphQL API Layer

**GOAL-006**: Implement GraphQL schema and resolver handlers

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-041 | Define GraphQL schema in `chaoscenter/graphql/server/graph/schema/agent_registry.graphqls`: add Agent type with all fields from model including helmRelease and kubernetesResources, add HelmReleaseInfo type, add KubernetesResourceInfo type, add AgentEndpoint type with EndpointDiscoveryType and EndpointType enums, add LangfuseConfig type, add AgentStatus enum (DISCOVERED, VALIDATING, ACTIVE, INACTIVE, DELETED), add AgentMetadata type with KeyValuePair, add AuditInfo type | | |
| TASK-042 | Define input types in schema: UpdateAgentInput, ListAgentsFilter, PaginationInput (RegisterAgentInput not needed - agents registered automatically via Helm/Watcher) | | |
| TASK-043 | Define response types in schema: AgentListResponse with agents array and pagination fields (totalCount, currentPage, totalPages, hasNextPage), AgentStatusResponse with agentId, status, healthy, lastCheckedAt, lastSyncedToLangfuse, HealthCheckResult with healthy, message, responseTime, checkedAt, CapabilityDefinition with id, name, description, category, DeleteAgentResponse with success and message, SyncResponse with success, syncedAt, message, DiscoveryResponse with success, discoveredAgents, message | | |
| TASK-044 | Define Query operations in schema: getAgent(id: ID!), listAgents(filter: ListAgentsFilter, pagination: PaginationInput!), getAgentsByCapabilities(projectId: String!, capabilities: [String!]!), getAgentStatus(id: ID!), getAgentCapabilitiesTaxonomy | | |
| TASK-045 | Define Mutation operations in schema: triggerAgentDiscovery(namespace: String, projectId: String!), updateAgent(id: ID!, input: UpdateAgentInput!), deleteAgent(id: ID!, hardDelete: Boolean), validateAgentHealth(id: ID!), syncAgentToLangfuse(id: ID!) (removed registerAgent - automatic via Watcher) | | |
| TASK-046 | Run gqlgen code generation: execute `go run github.com/99designs/gqlgen generate` to generate resolver stubs in `chaoscenter/graphql/server/graph/generated/` | | |
| TASK-047 | Implement `Handler` struct in `handler.go` with service field of type Service interface | | |
| TASK-048 | Implement `NewHandler` function accepting Service and returning *Handler | | |
| TASK-049A | Implement `TriggerAgentDiscovery` resolver in handler: extract user context, verify authorization, call watcher to scan namespace for agents with label `agentcert.io/agent=true`, return DiscoveryResponse with count of discovered agents | | |
| TASK-050 | Implement `GetAgent` resolver in handler: extract user context, call service.GetAgent(id), transform internal Agent to model.Agent, handle ErrAgentNotFound with NOT_FOUND error code, handle authorization errors with FORBIDDEN code | | |
| TASK-051 | Implement `ListAgents` resolver in handler: extract user context, transform model.ListAgentsFilter and PaginationInput to internal types, call service.ListAgents, transform results to model.AgentListResponse with calculated pagination fields (totalPages = ceil(totalCount / limit), hasNextPage = currentPage < totalPages) | | |
| TASK-052 | Implement `UpdateAgent` resolver in handler: extract user context, verify authorization, transform model.UpdateAgentInput to internal type, call service.UpdateAgent, transform result to model.Agent, handle errors | | |
| TASK-053 | Implement `DeleteAgent` resolver in handler: extract user context, verify authorization, call service.DeleteAgent with hardDelete flag, return model.DeleteAgentResponse | | |
| TASK-054 | Implement `GetAgentsByCapabilities` resolver in handler: extract user context, verify user has access to projectId, call service.GetAgentsByCapabilities, transform results to model.Agent array | | |
| TASK-055 | Implement `GetAgentStatus` resolver in handler: extract user context, call service.GetAgent to verify access, call service.ValidateAgentHealth to get current status, return model.AgentStatusResponse with health check results and Langfuse sync status | | |
| TASK-056 | Implement `ValidateAgentHealth` resolver in handler: extract user context, call service.ValidateAgentHealth, return model.AgentStatusResponse | | |
| TASK-057 | Implement `SyncAgentToLangfuse` resolver in handler: extract user context, verify authorization, call service.GetAgent, call service.SyncToLangfuse, return model.SyncResponse with success status and timestamp | | |
| TASK-058 | Implement `GetAgentCapabilitiesTaxonomy` resolver in handler: call service.GetCapabilitiesTaxonomy, transform to model.CapabilityDefinition array, return results | | |

#### **CHECKPOINT-PHASE-6: GraphQL API Layer Validation**

**Objective**: Verify GraphQL schema, resolvers, and API endpoints work correctly with proper error handling and authorization.

**Prerequisites**:
- Phase 5 checkpoint passed
- All tasks TASK-041 through TASK-058 completed
- GraphQL schema defined, resolvers implemented

**Validation Steps**:

1. **GraphQL Schema Generation**
   ```powershell
   cd chaoscenter/graphql/server
   go run github.com/99designs/gqlgen generate
   ```
   - **Pass Criteria**: Code generation succeeds, no errors
   - Verify generated files in `graph/generated/` directory
   - Verify resolver stubs created
   - **Fail Action**: Fix schema syntax errors in agent_registry.graphqls

2. **GraphQL Schema Validation**
   ```powershell
   go test ./graph -run TestSchemaTypes -v
   ```
   Create test `TestSchemaTypes` to verify schema contains:
   - Agent type with all fields (agentId, projectId, name, version, helmRelease, kubernetesResources, endpoint, status, etc.)
   - HelmReleaseInfo type
   - KubernetesResourceInfo type
   - AgentEndpoint type with enums
   - LangfuseConfig type
   - Input types: UpdateAgentInput, ListAgentsFilter, PaginationInput
   - Response types: AgentListResponse, HealthCheckResult, etc.
   - 5 Queries: getAgent, listAgents, getAgentsByCapabilities, getAgentStatus, getAgentCapabilitiesTaxonomy
   - 5 Mutations: triggerAgentDiscovery, updateAgent, deleteAgent, validateAgentHealth, syncAgentToLangfuse
   - **Pass Criteria**: All types, queries, mutations present in schema
   - **Fail Action**: Add missing types/operations to schema

3. **Trigger Agent Discovery Mutation**
   ```powershell
   # Start GraphQL server in test mode
   go test ./graph -run TestTriggerAgentDiscovery -v
   ```
   Create test `TestTriggerAgentDiscovery` to:
   - Mock watcher to scan namespace
   - Execute GraphQL mutation:
     ```graphql
     mutation {
       triggerAgentDiscovery(namespace: "test-ns", projectId: "proj-123") {
         success
         discoveredAgents
         message
       }
     }
     ```
   - Verify DiscoveryResponse returned with success=true
   - Verify discoveredAgents count correct
   - **Pass Criteria**: Mutation executes successfully, discovery triggered
   - **Fail Action**: Debug TriggerAgentDiscovery resolver

4. **Get Agent Query**
   ```powershell
   go test ./graph -run TestGetAgentQuery -v
   ```
   Create test `TestGetAgentQuery` to:
   - Mock service.GetAgent returning agent
   - Execute GraphQL query:
     ```graphql
     query {
       getAgent(id: "agent-123") {
         agentId
         name
         version
         status
         helmRelease { releaseName chartName }
         kubernetesResources { deploymentName serviceNamed configMapName resourceUID }
       }
     }
     ```
   - Verify agent data returned correctly
   - Test with non-existent ID, verify error response with NOT_FOUND code
   - **Pass Criteria**: Query returns correct data, errors handled
   - **Fail Action**: Fix GetAgent resolver data transformation

5. **List Agents Query with Pagination**
   ```powershell
   go test ./graph -run TestListAgentsQuery -v
   ```
   Create test `TestListAgentsQuery` to:
   - Mock service.ListAgents returning 25 agents
   - Execute GraphQL query:
     ```graphql
     query {
       listAgents(
         filter: { projectId: "proj-123", status: ACTIVE }
         pagination: { page: 2, limit: 10 }
       ) {
         agents { agentId name status }
         totalCount
         currentPage
         totalPages
         hasNextPage
       }
     }
     ```
   - Verify pagination fields calculated correctly
   - Verify agents array contains correct items
   - **Pass Criteria**: Pagination and filtering work correctly
   - **Fail Action**: Fix pagination calculation in ListAgents resolver

6. **Update Agent Mutation**
   ```powershell
   go test ./graph -run TestUpdateAgentMutation -v
   ```
   Create test `TestUpdateAgentMutation` to:
   - Mock service.UpdateAgent succeeding
   - Execute GraphQL mutation:
     ```graphql
     mutation {
       updateAgent(
         id: "agent-123"
         input: { version: "2.0.0" }
       ) {
         agentId
         version
         auditInfo { updatedAt }
       }
     }
     ```
   - Verify updated agent returned
   - Verify version changed, updatedAt timestamp updated
   - **Pass Criteria**: Mutation succeeds, changes persisted
   - **Fail Action**: Debug UpdateAgent resolver

7. **Delete Agent Mutation (Soft and Hard)**
   ```powershell
   go test ./graph -run TestDeleteAgentMutation -v
   ```
   Create test `TestDeleteAgentMutation` to:
   - Test soft delete:
     ```graphql
     mutation {
       deleteAgent(id: "agent-123", hardDelete: false) {
         success
         message
       }
     }
     ```
   - Verify success=true
   - Test hard delete with hardDelete: true
   - Verify appropriate service method called
   - **Pass Criteria**: Both delete modes work
   - **Fail Action**: Fix DeleteAgent resolver

8. **Get Agents By Capabilities Query**
   ```powershell
   go test ./graph -run TestGetAgentsByCapabilitiesQuery -v
   ```
   Create test `TestGetAgentsByCapabilitiesQuery` to:
   - Mock service.GetAgentsByCapabilities returning filtered agents
   - Execute GraphQL query:
     ```graphql
     query {
       getAgentsByCapabilities(
         projectId: "proj-123"
         capabilities: ["pod-crash-remediation", "pod-delete-remediation"]
       ) {
         agentId
         name
         capabilities
       }
     }
     ```
   - Verify only agents with ALL specified capabilities returned
   - **Pass Criteria**: Capability filtering correct
   - **Fail Action**: Debug GetAgentsByCapabilities resolver

9. **Validate Agent Health Mutation**
   ```powershell
   go test ./graph -run TestValidateAgentHealthMutation -v
   ```
   Create test `TestValidateAgentHealthMutation` to:
   - Mock service.ValidateAgentHealth returning health result
   - Execute GraphQL mutation:
     ```graphql
     mutation {
       validateAgentHealth(id: "agent-123") {
         agentId
         status
         healthy
         lastCheckedAt
       }
     }
     ```
   - Verify health status returned correctly
   - **Pass Criteria**: Health check triggered via GraphQL
   - **Fail Action**: Fix ValidateAgentHealth resolver

10. **Authorization Enforcement**
    ```powershell
    go test ./graph -run TestGraphQLAuthorization -v
    ```
    Create test `TestGraphQLAuthorization` to:
    - Test with PROJECT_MEMBER role:
      - getAgent, listAgents queries should succeed
      - updateAgent, deleteAgent mutations should fail with FORBIDDEN error
    - Test with PROJECT_OWNER role:
      - All operations should succeed
    - Test accessing agent from different project:
      - Should fail with NOT_FOUND or FORBIDDEN error
    - **Pass Criteria**: RBAC correctly enforced at resolver level
    - **Fail Action**: Add authorization checks to resolvers

11. **Error Handling**
    ```powershell
    go test ./graph -run TestGraphQLErrorHandling -v
    ```
    Create test `TestGraphQLErrorHandling` to:
    - Test ErrAgentNotFound → NOT_FOUND error code
    - Test ErrDuplicateAgentName → CONFLICT error code
    - Test ErrInvalidAgentName → BAD_REQUEST error code
    - Test ErrUnauthorized → FORBIDDEN error code
    - Verify error messages descriptive
    - **Pass Criteria**: All errors mapped to appropriate GraphQL error codes
    - **Fail Action**: Implement error transformation in resolvers

12. **Integration Test: Complete GraphQL Flow**
    ```powershell
    # Run integration test with real GraphQL server
    go test ./graph -run TestGraphQLIntegrationFlow -v
    ```
    Create test `TestGraphQLIntegrationFlow` to:
    - Start GraphQL server with test configuration
    - Use testcontainers for MongoDB
    - Execute sequence of GraphQL operations:
      1. triggerAgentDiscovery (register agent via watcher)
      2. getAgent (verify registered)
      3. listAgents (verify appears in list)
      4. updateAgent (modify version)
      5. getAgent (verify updated)
      6. validateAgentHealth (check status)
      7. syncAgentToLangfuse (manual sync)
      8. deleteAgent (soft delete)
      9. listAgents (verify status DELETED)
    - **Pass Criteria**: Complete flow works end-to-end via GraphQL API
    - **Fail Action**: Debug resolver integration with service layer

**Overall Phase 6 Pass Criteria**:
- ✅ All 12 validation steps pass
- ✅ GraphQL schema valid and complete
- ✅ All queries and mutations functional
- ✅ Authorization enforced correctly
- ✅ Error handling consistent
- ✅ Data transformation between internal and GraphQL models correct
- ✅ Integration test passes

**Rollback Actions** (if checkpoint fails):
1. Identify which GraphQL test failed
2. Review schema definitions for syntax errors
3. Debug resolver implementations
4. Check data transformation logic
5. Verify authorization checks present
6. Re-run failed test
7. Once all validations pass, proceed to Phase 7

---

### Implementation Phase 7: Integration and Initialization

**GOAL-007**: Integrate Agent Registry into GraphQL server and configure startup

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-059 | Modify `chaoscenter/graphql/server/main.go`: import agent_registry package, after MongoDB initialization create agent_registry.Operator using NewOperator(mongoDatabase), create agent_registry.Validator using NewValidator(operator), create Kubernetes clientset using k8s.io/client-go/kubernetes and in-cluster config, read LANGFUSE_BASE_URL and LANGFUSE_API_KEY from environment variables, create agent_registry.LangfuseClient using NewLangfuseClient(baseURL, apiKey) if configured, create agent_registry.Service using NewService(operator, validator, langfuseClient, k8sClient, logger), create agent_registry.Handler using NewHandler(service), create agent_registry.AgentWatcher using NewAgentWatcher(k8sClient, service, logger) | | |
| TASK-060 | Initialize HealthCheckScheduler in main.go: read AGENT_HEALTH_CHECK_INTERVAL from environment with default "5m", parse interval using time.ParseDuration, create scheduler using NewHealthCheckScheduler(service, interval, logger), start scheduler in goroutine using scheduler.Start(context.Background()), register graceful shutdown to call scheduler.Stop() on SIGTERM/SIGINT | | |
| TASK-060A | Initialize and start AgentWatcher in main.go: read AGENT_WATCH_NAMESPACES from environment (comma-separated list or "" for all namespaces), start watcher in goroutine using watcher.Start(context.Background(), namespaces), register graceful shutdown to call watcher.Stop() on SIGTERM/SIGINT | | |
| TASK-061 | Wire Agent Registry handlers to GraphQL resolvers: in resolver.go mutation resolver, add agentRegistryHandler field, implement TriggerAgentDiscovery mutation by calling handler.TriggerAgentDiscovery, implement UpdateAgent mutation by calling handler.UpdateAgent, implement DeleteAgent mutation by calling handler.DeleteAgent, implement ValidateAgentHealth mutation by calling handler.ValidateAgentHealth, implement SyncAgentToLangfuse mutation by calling handler.SyncAgentToLangfuse | | |
| TASK-062 | Wire Agent Registry handlers to GraphQL queries: in resolver.go query resolver, implement getAgent query by calling handler.GetAgent, implement listAgents query by calling handler.ListAgents, implement getAgentsByCapabilities query by calling handler.GetAgentsByCapabilities, implement getAgentStatus query by calling handler.GetAgentStatus, implement getAgentCapabilitiesTaxonomy query by calling handler.GetAgentCapabilitiesTaxonomy | | |
| TASK-063 | Create Kubernetes Secret manifest `chaoscenter/manifests/langfuse-secret.yaml` with template for LANGFUSE_API_KEY, document in deployment guide | | |
| TASK-064 | Update GraphQL server Deployment manifest `chaoscenter/graphql/server/manifests/deployment.yaml`: add environment variables LANGFUSE_BASE_URL (from ConfigMap), LANGFUSE_API_KEY (from Secret), AGENT_HEALTH_CHECK_INTERVAL (default "5m"), AGENT_HEALTH_CHECK_TIMEOUT (default "5s"), AGENT_WATCH_NAMESPACES (comma-separated list or empty for all), ensure ServiceAccount has RBAC permissions for watching ConfigMaps, Deployments, Services (verbs: list, watch, get) | | |

#### **CHECKPOINT-PHASE-7: Integration and Initialization Validation**

**Objective**: Verify Agent Registry service correctly integrates into GraphQL server with proper initialization, wiring, and configuration.

**Prerequisites**:
- Phase 6 checkpoint passed
- All tasks TASK-059 through TASK-064 completed
- main.go modified with initialization code
- Kubernetes manifests created/updated

**Validation Steps**:

1. **Build Verification**
   ```powershell
   cd chaoscenter/graphql/server
   go build -o graphql-server.exe ./...
   ```
   - **Pass Criteria**: Build succeeds with exit code 0
   - Executable created
   - No linker errors
   - **Fail Action**: Fix import paths, missing dependencies, or compilation errors

2. **Dependency Check**
   ```powershell
   go mod verify
   go mod tidy
   git diff go.mod go.sum
   ```
   - Verify new dependencies added:
     - github.com/google/uuid
     - k8s.io/client-go
     - k8s.io/api
     - k8s.io/apimachinery
   - **Pass Criteria**: go.mod and go.sum valid, dependencies resolved
   - **Fail Action**: Run `go get` for missing packages, resolve conflicts

3. **Initialization Sequence Test**
   ```powershell
   go test ./... -run TestMainInitialization -v
   ```
   Create test `TestMainInitialization` to:
   - Mock MongoDB connection
   - Mock Kubernetes client
   - Call initialization functions from main.go in sequence:
     - Create operator
     - Create validator
     - Create Langfuse client
     - Create service
     - Create handler
     - Create watcher
     - Create health scheduler
   - Verify each component initialized without nil values
   - Verify dependencies injected correctly
   - **Pass Criteria**: All components initialize successfully
   - **Fail Action**: Fix initialization order or constructor calls in main.go

4. **Environment Variable Configuration**
   ```powershell
   # Create test script to verify environment variable handling
   go test ./... -run TestEnvironmentConfiguration -v
   ```
   Create test `TestEnvironmentConfiguration` to:
   - Set environment variables:
     - LANGFUSE_BASE_URL=http://langfuse-test:3000
     - LANGFUSE_API_KEY=test-key-123
     - AGENT_HEALTH_CHECK_INTERVAL=2m
     - AGENT_WATCH_NAMESPACES=default,agents
   - Call initialization code
   - Verify values read correctly
   - Verify defaults applied when vars not set
   - **Pass Criteria**: Environment variables parsed correctly, defaults work
   - **Fail Action**: Fix environment variable reading logic in main.go

5. **GraphQL Resolver Wiring**
   ```powershell
   go test ./graph -run TestResolverWiring -v
   ```
   Create test `TestResolverWiring` to:
   - Initialize resolver with handler
   - Verify resolver.agentRegistryHandler field not nil
   - Verify all query resolvers callable
   - Verify all mutation resolvers callable
   - Test calling one query (getAgent) and one mutation (updateAgent)
   - **Pass Criteria**: Resolvers wired correctly to handler
   - **Fail Action**: Fix resolver.go initialization or field assignments

6. **Health Check Scheduler Startup**
   ```powershell
   # Run with test configuration
   go test ./... -run TestHealthSchedulerStartup -v
   ```
   Create test `TestHealthSchedulerStartup` to:
   - Initialize service and scheduler
   - Start scheduler in goroutine
   - Sleep 1 second
   - Verify scheduler running (check logs or internal state)
   - Send shutdown signal
   - Verify graceful shutdown completes within 5 seconds
   - **Pass Criteria**: Scheduler starts and stops cleanly
   - **Fail Action**: Fix scheduler Start/Stop logic or signal handling in main.go

7. **Agent Watcher Startup**
   ```powershell
   go test ./... -run TestAgentWatcherStartup -v
   ```
   Create test `TestAgentWatcherStartup` to:
   - Mock Kubernetes client
   - Initialize watcher with namespaces: ["default", "agents"]
   - Start watcher in goroutine
   - Verify watcher.Start() called
   - Verify watching correct namespaces
   - Send shutdown signal
   - Verify watcher stops gracefully
   - **Pass Criteria**: Watcher starts and monitors configured namespaces
   - **Fail Action**: Fix watcher initialization or namespace configuration

8. **Kubernetes RBAC Manifest Validation**
   ```powershell
   # Validate Kubernetes manifests
   kubectl apply --dry-run=client -f chaoscenter/graphql/server/manifests/deployment.yaml
   kubectl apply --dry-run=client -f chaoscenter/manifests/langfuse-secret.yaml
   ```
   - Verify deployment.yaml has:
     - Environment variables defined
     - Secret mounted for LANGFUSE_API_KEY
     - ServiceAccount with RBAC permissions
   - Verify ServiceAccount has ClusterRole/Role with:
     - Resources: configmaps, deployments, services
     - Verbs: list, watch, get
   - **Pass Criteria**: Manifests valid, RBAC permissions correct
   - **Fail Action**: Fix deployment.yaml, add missing RBAC rules

9. **MongoDB Collection and Indexes Verification**
   ```powershell
   # Run migration script and verify
   mongosh --eval "load('migrations/001_create_agent_registry.js')"
   mongosh --eval "db.agent_registry_collection.getIndexes()" | ConvertFrom-Json | Select-Object name, key
   ```
   - Verify 6 indexes exist (including default _id)
   - Verify unique indexes on agentId, (projectId, name), resourceUID
   - **Pass Criteria**: All required indexes created
   - **Fail Action**: Fix migration script, re-run

10. **Integration Test: Server Startup**
    ```powershell
    # Run full server startup test
    go test ./... -run TestGraphQLServerStartup -v -timeout 30s
    ```
    Create test `TestGraphQLServerStartup` to:
    - Start testcontainers for MongoDB
    - Set all required environment variables
    - Initialize all Agent Registry components
    - Start GraphQL server
    - Wait for server ready (health endpoint or /graphql available)
    - Execute simple GraphQL query: `{ __schema { types { name } } }`
    - Verify Agent Registry types present in schema
    - Execute getAgentCapabilitiesTaxonomy query
    - Verify capabilities returned
    - Shutdown server gracefully
    - **Pass Criteria**: Server starts completely, GraphQL endpoint responsive, Agent Registry functional
    - **Fail Action**: Debug startup errors, check logs for initialization failures

11. **Graceful Shutdown Test**
    ```powershell
    go test ./... -run TestGracefulShutdown -v
    ```
    Create test `TestGracefulShutdown` to:
    - Start server with Agent Registry initialized
    - Start health check scheduler
    - Start agent watcher
    - Send SIGTERM signal
    - Verify:
      - Health scheduler stops
      - Agent watcher stops
      - No goroutine leaks (use runtime.NumGoroutine before/after)
      - MongoDB connections closed
      - HTTP server stops
    - **Pass Criteria**: Clean shutdown within 10 seconds, no leaks
    - **Fail Action**: Fix shutdown signal handling, add missing cleanup code

**Overall Phase 7 Pass Criteria**:
- ✅ All 11 validation steps pass
- ✅ Server builds and starts successfully
- ✅ All components initialized correctly
- ✅ Environment variables handled properly
- ✅ Kubernetes manifests valid
- ✅ RBAC permissions correct
- ✅ MongoDB indexes created
- ✅ Graceful shutdown works
- ✅ No goroutine or connection leaks

**Rollback Actions** (if checkpoint fails):
1. Identify which initialization test failed
2. Review main.go for initialization sequence errors
3. Check environment variable handling
4. Verify Kubernetes manifest syntax
5. Debug component wiring issues
6. Re-run failed test
7. Once all validations pass, proceed to Phase 8

---

### Implementation Phase 8: Testing

**GOAL-008**: Implement comprehensive unit and integration tests

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-065 | Create test file `operator_test.go`: implement TestCreateAgent_Success with mock MongoDB using testcontainers, insert agent and verify, implement TestGetAgent_NotFound verifying ErrAgentNotFound returned, implement TestGetAgentByProjectAndName_Success for uniqueness check, implement TestListAgents_WithPagination verifying skip/limit logic and count accuracy, implement TestListAgents_WithFilters testing status filter, capabilities filter, searchTerm filter, implement TestGetAgentsByCapabilities_ANDLogic with 3 agents having different capabilities verifying only agents with ALL capabilities returned, implement TestUpdateAgent_Success verifying updated fields and timestamps, implement TestDeleteAgent_Success verifying document removal | | |
| TASK-066 | Create test file `validator_test.go`: implement TestValidateName_Valid with valid names (agent-1, my-agent, a, agent-name-123), implement TestValidateName_Invalid with invalid names (Agent-1 uppercase, agent_name underscore, -agent leading hyphen, agent- trailing hyphen, agent name with space, 64+ char name), implement TestValidateSemver_Valid with versions (1.0.0, v2.1.3, 1.0.0-alpha, 1.0.0+build), implement TestValidateSemver_Invalid with versions (1.0, v1, 1.0.0.0), implement TestValidateCapabilities_Valid with taxonomy match, implement TestValidateCapabilities_Invalid with unknown capability, implement TestValidateCapabilities_Empty verifying error, implement TestValidateMetadata_RequiredFields verifying name, version, projectId, capabilities, langfuseConfig are required, implement TestValidateLangfuseConfig_Valid verifying projectId and syncEnabled present, implement TestValidateLangfuseConfig_MissingFields verifying error when projectId or syncEnabled missing | | |
| TASK-067 | Create test file `service_test.go`: implement TestRegisterAgentFromWatcher_Success mocking operator.CreateAgent and verifying UUID generation, status DISCOVERED, audit timestamps with discoveredAt, implement TestRegisterAgentFromWatcher_DuplicateResourceUID mocking operator.GetAgentByResourceUID returning existing agent and verifying no duplicate registration, implement TestRegisterAgentFromWatcher_InvalidMetadata with missing required fields (including langfuseConfig) verifying validation error, implement TestRegisterAgentFromWatcher_EndpointDiscovery mocking k8sClient to return service and verifying auto-discovered endpoint, implement TestUpdateAgent_Success mocking operator methods and verifying merge logic, implement TestDeleteAgent_SoftDelete verifying status set to DELETED, implement TestDeleteAgent_HardDelete verifying operator.DeleteAgent called, implement TestListAgents_WithFilters mocking operator and verifying filter passthrough, implement TestGetAgentsByCapabilities_Success verifying capability filtering, implement TestValidateAgentHealth_Success mocking HTTP server with /health returning 200 and verifying status transition to ACTIVE, implement TestValidateAgentHealth_Timeout mocking slow server and verifying status INACTIVE with error message, implement TestValidateAgentHealth_Unhealthy mocking /health returning 503 and verifying INACTIVE status | | |
| TASK-068 | Create test file `langfuse_client_test.go`: implement TestCreateOrUpdateUser_Success mocking HTTP server with POST /api/public/users returning 201 and verifying request payload contains correct agentId, name, metadata, implement TestCreateOrUpdateUser_Retry mocking server to fail twice then succeed and verifying 3 attempts made with exponential backoff delays, implement TestCreateOrUpdateUser_AllRetriesFail mocking server to always return 500 and verifying error returned after 3 retries, implement TestDeleteUser_Success verifying payload contains deleted:true in metadata | | |
| TASK-069 | Create test file `health_scheduler_test.go`: implement TestHealthCheckScheduler_Start creating scheduler with 100ms interval, starting in goroutine, sleeping 350ms, stopping scheduler, verifying health checks ran approximately 3 times by checking logs or metrics, implement TestHealthCheckScheduler_Stop verifying graceful shutdown by checking stopChan closed and no panics | | |
| TASK-069A | Create test file `watcher_test.go`: implement TestWatchConfigMaps_AddEvent creating mock k8s client, triggering ConfigMap ADDED event with valid metadata, verifying handleAgentDiscovery called and agent registered, implement TestWatchConfigMaps_ModifyEvent triggering ConfigMap MODIFIED event, verifying handleAgentUpdate called, implement TestWatchConfigMaps_DeleteEvent triggering ConfigMap DELETED event, verifying handleAgentDeletion called and agent marked DELETED, implement TestHandleAgentDiscovery_Idempotency calling handleAgentDiscovery twice with same resourceUID, verifying only one agent created | | |
| TASK-069B | Create test file `metadata_extractor_test.go`: implement TestExtractFromConfigMap_YAML creating ConfigMap with agent-metadata.yaml data including langfuseConfig, verifying correct AgentMetadata struct returned with all required fields, implement TestExtractFromConfigMap_JSON creating ConfigMap with agent-metadata.json data, verifying parsing succeeds, implement TestExtractFromConfigMap_MissingData verifying error returned when no metadata key found, implement TestExtractFromConfigMap_MissingRequiredFields verifying error when name, version, projectId, capabilities, or langfuseConfig missing, implement TestExtractFromConfigMap_InvalidLangfuseConfig verifying error when langfuseConfig missing projectId or syncEnabled | | |
| TASK-070 | Create integration test file `integration_test.go`: setup testcontainers for MongoDB, create real operator/validator/service/watcher instances, implement TestEndToEndHelmOnboarding: create mock k8s client, deploy ConfigMap with agent metadata, trigger watcher event, verify agent registered in MongoDB with status DISCOVERED, query by ID, verify returned, update ConfigMap, trigger watcher, verify agent updated, delete ConfigMap, trigger watcher, verify agent marked DELETED, implement TestConcurrentDiscovery: launch 10 goroutines each creating ConfigMap with unique agent name, trigger watcher events, wait for all to complete, verify all 10 agents in DB with no errors, implement TestCapabilityQueryWithMultipleAgents: register 5 agents with different capability combinations, query with 2 capabilities, verify only agents with BOTH capabilities returned, implement TestHealthCheckCycle: register agent via watcher with mock HTTP server endpoint, wait for health check, verify status transitioned to ACTIVE, stop mock server, wait for health check, verify status transitioned to INACTIVE | | |
| TASK-071 | Run all tests with coverage: execute `go test ./pkg/agent_registry/... -cover -coverprofile=coverage.out`, verify coverage >= 85%, generate HTML report using `go tool cover -html=coverage.out`, review uncovered lines and add tests if critical paths uncovered | | |

#### **CHECKPOINT-PHASE-8: Comprehensive Testing Validation**

**Objective**: Verify all unit tests, integration tests, and coverage requirements are met before moving to observability.

**Prerequisites**:
- Phase 7 checkpoint passed
- All tasks TASK-065 through TASK-071 completed
- All test files created

**Validation Steps**:

1. **Unit Tests - Operator Layer**
   ```powershell
   go test ./pkg/agent_registry -run TestOperator -v -count=1
   ```
   - Verify tests pass:
     - TestCreateAgent_Success
     - TestGetAgent_NotFound
     - TestGetAgentByProjectAndName_Success
     - TestListAgents_WithPagination
     - TestListAgents_WithFilters
     - TestGetAgentsByCapabilities_ANDLogic
     - TestUpdateAgent_Success
     - TestDeleteAgent_Success
   - **Pass Criteria**: All 8+ operator tests pass
   - **Fail Action**: Debug failed tests, fix operator.go implementation

2. **Unit Tests - Validator Layer**
   ```powershell
   go test ./pkg/agent_registry -run TestValidator -v -count=1
   ```
   - Verify tests pass:
     - TestValidateName_Valid
     - TestValidateName_Invalid
     - TestValidateSemver_Valid
     - TestValidateSemver_Invalid
     - TestValidateCapabilities_Valid
     - TestValidateCapabilities_Invalid
     - TestValidateCapabilities_Empty
     - TestValidateMetadata_RequiredFields
     - TestValidateLangfuseConfig_Valid
     - TestValidateLangfuseConfig_MissingFields
   - **Pass Criteria**: All 10+ validator tests pass
   - **Fail Action**: Fix validation logic, update regex patterns

3. **Unit Tests - Service Layer**
   ```powershell
   go test ./pkg/agent_registry -run TestService -v -count=1
   ```
   - Verify tests pass:
     - TestRegisterAgentFromWatcher_Success
     - TestRegisterAgentFromWatcher_DuplicateResourceUID
     - TestRegisterAgentFromWatcher_InvalidMetadata
     - TestRegisterAgentFromWatcher_EndpointDiscovery
     - TestUpdateAgent_Success
     - TestDeleteAgent_SoftDelete
     - TestDeleteAgent_HardDelete
     - TestListAgents_WithFilters
     - TestGetAgentsByCapabilities_Success
     - TestValidateAgentHealth_Success
     - TestValidateAgentHealth_Timeout
     - TestValidateAgentHealth_Unhealthy
   - **Pass Criteria**: All 12+ service tests pass
   - **Fail Action**: Debug service methods, check mocking setup

4. **Unit Tests - Langfuse Client**
   ```powershell
   go test ./pkg/agent_registry -run TestLangfuse -v -count=1
   ```
   - Verify tests pass:
     - TestCreateOrUpdateUser_Success
     - TestCreateOrUpdateUser_Retry
     - TestCreateOrUpdateUser_AllRetriesFail
     - TestDeleteUser_Success
   - **Pass Criteria**: All 4 Langfuse tests pass
   - **Fail Action**: Fix HTTP client logic, retry mechanism

5. **Unit Tests - Health Scheduler**
   ```powershell
   go test ./pkg/agent_registry -run TestHealthCheckScheduler -v -count=1
   ```
   - Verify tests pass:
     - TestHealthCheckScheduler_Start
     - TestHealthCheckScheduler_Stop
   - **Pass Criteria**: Both scheduler tests pass
   - **Fail Action**: Fix scheduler timing, goroutine management

6. **Unit Tests - Watcher**
   ```powershell
   go test ./pkg/agent_registry -run TestWatcher -v -count=1
   ```
   - Verify tests pass:
     - TestWatchConfigMaps_AddEvent
     - TestWatchConfigMaps_ModifyEvent
     - TestWatchConfigMaps_DeleteEvent
     - TestHandleAgentDiscovery_Idempotency
   - **Pass Criteria**: All 4 watcher tests pass
   - **Fail Action**: Fix event handling logic

7. **Unit Tests - Metadata Extractor**
   ```powershell
   go test ./pkg/agent_registry -run TestMetadataExtractor -v -count=1
   ```
   - Verify tests pass:
     - TestExtractFromConfigMap_YAML
     - TestExtractFromConfigMap_JSON
     - TestExtractFromConfigMap_MissingData
     - TestExtractFromConfigMap_MissingRequiredFields
     - TestExtractFromConfigMap_InvalidLangfuseConfig
   - **Pass Criteria**: All 5 extractor tests pass
   - **Fail Action**: Fix YAML/JSON parsing, validation

8. **Integration Tests**
   ```powershell
   go test ./pkg/agent_registry -run TestIntegration -v -count=1 -timeout=5m
   ```
   - Verify tests pass:
     - TestEndToEndHelmOnboarding
     - TestConcurrentDiscovery
     - TestCapabilityQueryWithMultipleAgents
     - TestHealthCheckCycle
   - **Pass Criteria**: All 4 integration tests pass with real MongoDB
   - **Fail Action**: Debug integration issues, check testcontainers setup

9. **Code Coverage Check**
   ```powershell
   go test ./pkg/agent_registry/... -cover -coverprofile=coverage.out
   go tool cover -func=coverage.out | Select-String "total:" 
   ```
   - Extract total coverage percentage
   - **Pass Criteria**: Total coverage >= 85%
   - Generate HTML report: `go tool cover -html=coverage.out -o coverage.html`
   - Review uncovered lines
   - **Fail Action**: Add tests for uncovered critical paths

10. **Coverage by File**
    ```powershell
    go tool cover -func=coverage.out | Select-String "operator.go|service.go|validator.go|handler.go|langfuse_client.go|health_scheduler.go|watcher.go|metadata_extractor.go"
    ```
    Verify each file has:
    - operator.go: >= 85%
    - service.go: >= 85%
    - validator.go: >= 85%
    - handler.go: >= 80%
    - langfuse_client.go: >= 85%
    - health_scheduler.go: >= 80%
    - watcher.go: >= 85%
    - metadata_extractor.go: >= 85%
    - **Pass Criteria**: All critical files meet coverage targets
    - **Fail Action**: Add missing test cases

11. **Race Condition Check**
    ```powershell
    go test ./pkg/agent_registry/... -race -count=2
    ```
    - Run with race detector enabled
    - Run tests twice to catch intermittent races
    - **Pass Criteria**: No data races detected
    - **Fail Action**: Fix race conditions with proper locking or channels

12. **Linting and Code Quality**
    ```powershell
    golangci-lint run ./pkg/agent_registry/... --timeout=5m
    ```
    - Check for:
      - Unused variables
      - Error handling issues
      - Inefficient code
      - Style violations
    - **Pass Criteria**: No critical or high-severity issues
    - Allow minor warnings if justified
    - **Fail Action**: Fix linting issues

**Overall Phase 8 Pass Criteria**:
- ✅ All unit tests pass (50+ tests across all components)
- ✅ All integration tests pass (4 tests)
- ✅ Total code coverage >= 85%
- ✅ Per-file coverage meets targets
- ✅ No race conditions detected
- ✅ No critical linting issues
- ✅ All tests run consistently (no flaky tests)

**Rollback Actions** (if checkpoint fails):
1. Identify which test category failed
2. Review test output for specific failures
3. Fix implementation bugs or test setup issues
4. Add missing test cases for uncovered code
5. Eliminate race conditions
6. Address linting issues
7. Re-run failed tests
8. Once all validations pass, proceed to Phase 9

---

### Implementation Phase 9: Observability and Monitoring

**GOAL-009**: Implement metrics, logging, and monitoring dashboards

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-072 | Add Prometheus metrics in `service.go`: define counters agentRegistrationsTotal, agentQueriesTotal with operation label (list/get/getByCapabilities), agentHealthChecksTotal with status label (success/failed), agentLangfuseSyncTotal with status label, agentErrorsTotal with errorType label; define histograms agentRegistrationDuration, agentQueryDuration with operation label, agentHealthCheckDuration, agentLangfuseSyncDuration; define gauges agentActiveAgents, agentInactiveAgents | | |
| TASK-073 | Register metrics with Prometheus: in service constructor call prometheus.MustRegister for all metrics, ensure /metrics endpoint exposed by GraphQL server (already exists in LitmusChaos) | | |
| TASK-074 | Instrument RegisterAgent method: record start time, defer metric recording, increment agentRegistrationsTotal on success, observe duration in agentRegistrationDuration, increment agentErrorsTotal on error with error type label, update agentActiveAgents gauge | | |
| TASK-075 | Instrument ListAgents method: increment agentQueriesTotal with operation=list, observe duration in agentQueryDuration | | |
| TASK-076 | Instrument GetAgentsByCapabilities method: increment agentQueriesTotal with operation=getByCapabilities, observe duration | | |
| TASK-077 | Instrument ValidateAgentHealth method: increment agentHealthChecksTotal with status label based on result, observe duration in agentHealthCheckDuration, update agentActiveAgents and agentInactiveAgents gauges based on status transition | | |
| TASK-078 | Instrument SyncToLangfuse method: increment agentLangfuseSyncTotal with status label (success/failed/skipped), observe duration in agentLangfuseSyncDuration | | |
| TASK-079 | Enhance logging in service layer: add structured log fields (agentId, projectId, operation, userId, duration, error) to all log statements, use log levels appropriately (INFO for successful operations, WARN for retries/degradation, ERROR for failures, DEBUG for detailed traces), ensure sensitive data (API keys) never logged | | |
| TASK-080 | Create Grafana dashboard JSON `chaoscenter/manifests/grafana-agent-registry-dashboard.json`: add panels for total agents gauge, registrations rate graph, query latency heatmap, health check success rate, Langfuse sync success rate, error rate by type, active/inactive agents over time, save dashboard with tags [agent-registry, agentcert] | | |

#### **CHECKPOINT-PHASE-9: Observability and Monitoring Validation**

**Objective**: Verify metrics, logging, and monitoring capabilities are correctly implemented and functional.

**Prerequisites**:
- Phase 8 checkpoint passed
- All tasks TASK-072 through TASK-080 completed
- Metrics instrumented, Grafana dashboard created

**Validation Steps**:

1. **Prometheus Metrics Registration**
   ```powershell
   go test ./pkg/agent_registry -run TestMetricsRegistration -v
   ```
   Create test `TestMetricsRegistration` to:
   - Create service instance
   - Verify all metrics registered with Prometheus:
     - Counters: agentRegistrationsTotal, agentQueriesTotal, agentHealthChecksTotal, agentLangfuseSyncTotal, agentErrorsTotal
     - Histograms: agentRegistrationDuration, agentQueryDuration, agentHealthCheckDuration, agentLangfuseSyncDuration
     - Gauges: agentActiveAgents, agentInactiveAgents
   - **Pass Criteria**: All 11 metrics registered successfully
   - **Fail Action**: Fix prometheus.MustRegister calls in service constructor

2. **Metrics Instrumentation - Registration**
   ```powershell
   go test ./pkg/agent_registry -run TestMetricsRegistration -v
   ```
   Create test `TestMetricsInstrumentationRegistration` to:
   - Create service with real Prometheus registry
   - Call RegisterAgentFromWatcher
   - Verify agentRegistrationsTotal counter incremented
   - Verify agentRegistrationDuration histogram recorded
   - Verify agentActiveAgents gauge updated
   - Query Prometheus metrics endpoint
   - **Pass Criteria**: Metrics updated correctly on registration
   - **Fail Action**: Fix metric recording in RegisterAgentFromWatcher

3. **Metrics Instrumentation - Queries**
   ```powershell
   go test ./pkg/agent_registry -run TestMetricsInstrumentationQueries -v
   ```
   Create test `TestMetricsInstrumentationQueries` to:
   - Call ListAgents
   - Verify agentQueriesTotal{operation="list"} incremented
   - Verify agentQueryDuration{operation="list"} recorded
   - Call GetAgentsByCapabilities
   - Verify agentQueriesTotal{operation="getByCapabilities"} incremented
   - **Pass Criteria**: Query metrics recorded with correct labels
   - **Fail Action**: Fix metric instrumentation in query methods

4. **Metrics Instrumentation - Health Checks**
   ```powershell
   go test ./pkg/agent_registry -run TestMetricsInstrumentationHealthChecks -v
   ```
   Create test `TestMetricsInstrumentationHealthChecks` to:
   - Create mock HTTP server (healthy)
   - Call ValidateAgentHealth
   - Verify agentHealthChecksTotal{status="success"} incremented
   - Verify agentHealthCheckDuration recorded
   - Create unhealthy server
   - Call ValidateAgentHealth
   - Verify agentHealthChecksTotal{status="failed"} incremented
   - Verify agentActiveAgents and agentInactiveAgents gauges updated
   - **Pass Criteria**: Health check metrics accurate
   - **Fail Action**: Fix metric recording in ValidateAgentHealth

5. **Metrics Instrumentation - Langfuse Sync**
   ```powershell
   go test ./pkg/agent_registry -run TestMetricsInstrumentationLangfuseSync -v
   ```
   Create test `TestMetricsInstrumentationLangfuseSync` to:
   - Mock successful Langfuse sync
   - Call SyncToLangfuse
   - Verify agentLangfuseSyncTotal{status="success"} incremented
   - Verify agentLangfuseSyncDuration recorded
   - Mock failed sync
   - Verify agentLangfuseSyncTotal{status="failed"} incremented
   - Test sync disabled
   - Verify agentLangfuseSyncTotal{status="skipped"} incremented
   - **Pass Criteria**: Sync metrics cover all scenarios
   - **Fail Action**: Fix metric recording in SyncToLangfuse

6. **Metrics Instrumentation - Errors**
   ```powershell
   go test ./pkg/agent_registry -run TestMetricsInstrumentationErrors -v
   ```
   Create test `TestMetricsInstrumentationErrors` to:
   - Trigger various error scenarios:
     - ErrAgentNotFound
     - ErrDuplicateAgentName
     - ErrInvalidCapabilities
   - Verify agentErrorsTotal{errorType="..."} incremented for each
   - **Pass Criteria**: Error metrics categorized correctly
   - **Fail Action**: Add error metric recording in error paths

7. **Metrics Endpoint Verification**
   ```powershell
   # Start server and query metrics
   go test ./... -run TestMetricsEndpoint -v
   ```
   Create test `TestMetricsEndpoint` to:
   - Start GraphQL server with Agent Registry
   - Perform agent operations (register, query, health check)
   - Query /metrics endpoint via HTTP
   - Parse Prometheus text format
   - Verify Agent Registry metrics present:
     ```
     agent_registry_registrations_total
     agent_registry_queries_total
     agent_registry_health_checks_total
     agent_registry_active_agents
     ```
   - **Pass Criteria**: Metrics exposed and parseable
   - **Fail Action**: Verify /metrics endpoint enabled, metrics exported

8. **Structured Logging Verification**
   ```powershell
   go test ./pkg/agent_registry -run TestStructuredLogging -v
   ```
   Create test `TestStructuredLogging` to:
   - Configure logrus with JSON formatter
   - Capture log output
   - Perform operations (register, update, health check)
   - Parse JSON logs
   - Verify structured fields present:
     - agentId
     - projectId
     - operation
     - userId
     - duration
     - error (when applicable)
   - Verify log levels appropriate:
     - INFO for successful operations
     - WARN for retries
     - ERROR for failures
   - Verify NO sensitive data (API keys) in logs
   - **Pass Criteria**: Logs structured and complete
   - **Fail Action**: Add missing structured fields, fix log levels

9. **Grafana Dashboard Validation**
   ```powershell
   # Validate JSON structure
   $dashboard = Get-Content chaoscenter/manifests/grafana-agent-registry-dashboard.json | ConvertFrom-Json
   $dashboard.panels.Count -ge 7
   ```
   Verify dashboard contains minimum 7 panels:
   - Total agents gauge
   - Registrations rate (rate(agent_registry_registrations_total[5m]))
   - Query latency heatmap (histogram_quantile from agentQueryDuration)
   - Health check success rate
   - Langfuse sync success rate
   - Error rate by type
   - Active/Inactive agents over time
   - **Pass Criteria**: Dashboard JSON valid, all panels present
   - **Fail Action**: Fix dashboard JSON, add missing panels

10. **Grafana Dashboard Import Test** (optional if Grafana available)
    ```powershell
    # Import dashboard via Grafana API
    curl -X POST http://localhost:3000/api/dashboards/db `
      -H "Content-Type: application/json" `
      -H "Authorization: Bearer $GRAFANA_API_KEY" `
      -d "@chaoscenter/manifests/grafana-agent-registry-dashboard.json"
    ```
    - Verify import succeeds
    - Verify panels render (may need sample data)
    - **Pass Criteria**: Dashboard imports without errors
    - **Fail Action**: Fix dashboard schema version or panel configurations

11. **Integration Test: End-to-End Observability**
    ```powershell
    go test ./... -run TestObservabilityIntegration -v
    ```
    Create test `TestObservabilityIntegration` to:
    - Start GraphQL server with metrics enabled
    - Perform sequence of operations:
      1. Register 5 agents
      2. Query agents
      3. Run health checks
      4. Sync to Langfuse
      5. Trigger some errors
    - Query /metrics endpoint
    - Verify metric values match operations:
      - agentRegistrationsTotal = 5
      - agentQueriesTotal >= 1
      - agentHealthChecksTotal = 5
      - agentActiveAgents > 0
    - Capture and parse logs
    - Verify log entries correspond to operations
    - **Pass Criteria**: Metrics and logs accurately reflect operations
    - **Fail Action**: Debug metric/logging instrumentation

**Overall Phase 9 Pass Criteria**:
- ✅ All 11 validation steps pass
- ✅ All Prometheus metrics correctly registered
- ✅ Metrics instrumented in all key operations
- ✅ /metrics endpoint exposes Agent Registry metrics
- ✅ Structured logging implemented with appropriate fields
- ✅ No sensitive data in logs
- ✅ Grafana dashboard valid and complete
- ✅ End-to-end observability working

**Rollback Actions** (if checkpoint fails):
1. Identify which observability test failed
2. Review metric registration and instrumentation
3. Fix missing metric recordings or incorrect labels
4. Add missing structured logging fields
5. Correct Grafana dashboard JSON
6. Re-run failed test
7. Once all validations pass, proceed to Phase 10

---

### Implementation Phase 10: Documentation and Deployment

**GOAL-010**: Create deployment documentation and finalize production readiness

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-081 | Create deployment guide `chaoscenter/graphql/server/pkg/agent_registry/README.md`: document architecture overview, dependencies (MongoDB, Langfuse, Kubernetes), configuration environment variables with defaults and descriptions, MongoDB index creation steps, Kubernetes Secret creation for Langfuse API key, GraphQL API usage examples for all operations, capability taxonomy reference table, status transition diagram, troubleshooting guide for common errors | | |
| TASK-082 | Document GraphQL API in `docs/api/agent-registry-api.md`: document all queries with parameters, sample requests, sample responses; document all mutations with input types, validation rules, error codes; document enums and their meanings; include curl examples for testing via HTTP | | |
| TASK-083 | Create MongoDB migration guide `chaoscenter/graphql/server/pkg/agent_registry/MIGRATION.md`: document migration script execution steps, rollback procedure, index verification commands, data validation queries to ensure migration success | | |
| TASK-084 | Update main README.md: add Agent Registry Service to feature list, link to detailed documentation, update architecture diagram to include Agent Registry component | | |
| TASK-085 | Create Kubernetes ConfigMap `chaoscenter/manifests/agent-registry-config.yaml`: define LANGFUSE_BASE_URL, AGENT_HEALTH_CHECK_INTERVAL, AGENT_HEALTH_CHECK_TIMEOUT, CAPABILITY_TAXONOMY (JSON encoded list) | | |
| TASK-086 | Update GraphQL server Dockerfile: ensure Go modules downloaded including new dependencies (uuid, client-go), verify build succeeds with new package | | |
| TASK-087 | Create Helm chart values for Agent Registry configuration: in `chaoscenter/graphql/server/chart/values.yaml` add section for agentRegistry with enabled flag (default true), langfuse.baseUrl, langfuse.apiKeySecret, healthCheck.interval, healthCheck.timeout, capabilities array | | |
| TASK-088 | Test deployment in development environment: apply MongoDB migration, create Langfuse secret with test API key, deploy updated GraphQL server, verify health check scheduler starts, verify metrics exposed at /metrics, execute sample GraphQL mutations/queries, verify MongoDB documents created, verify Langfuse receives metadata (check Langfuse dashboard for users), verify health checks execute every 5 minutes, review logs for errors | | |
| TASK-089 | Perform load testing: use k6 or Apache Bench to simulate 50 concurrent users executing registerAgent, listAgents, getAgentsByCapabilities operations, measure p95 latency for each operation, verify metrics align with NFR targets (registration < 500ms, queries < 200ms), identify bottlenecks if targets not met, optimize queries or add caching as needed | | |
| TASK-090 | Security review: verify JWT extraction and validation works correctly, test RBAC enforcement by attempting operations with insufficient permissions, attempt SQL injection in searchTerm field, attempt SSRF by providing localhost endpoint URL, verify Langfuse API key not exposed in logs or responses, scan dependencies for CVEs using `go list -json -m all | nancy sleuth` | | |
| TASK-091 | Create production deployment checklist: MongoDB indexes created, Langfuse API key configured in Secret, ConfigMap deployed with production URLs, resource limits set on GraphQL server (CPU, memory), health check scheduler interval appropriate for scale, metrics scraping configured in Prometheus, Grafana dashboard imported, log aggregation configured, backup strategy for MongoDB agent_registry_collection, disaster recovery tested | | |

#### **CHECKPOINT-PHASE-10: Documentation and Deployment Validation**

**Objective**: Verify all documentation is complete, deployment artifacts are ready, and system is production-ready.

**Prerequisites**:
- Phase 9 checkpoint passed
- All tasks TASK-081 through TASK-091 completed
- All documentation files created

**Validation Steps**:

1. **Package README Completeness**
   ```powershell
   Test-Path chaoscenter/graphql/server/pkg/agent_registry/README.md
   Get-Content chaoscenter/graphql/server/pkg/agent_registry/README.md | Measure-Object -Line
   ```
   Verify README.md contains:
   - Architecture overview section
   - Dependencies list (MongoDB, Langfuse, Kubernetes)
   - Configuration section with environment variables table
   - MongoDB index creation steps
   - Kubernetes Secret creation guide
   - GraphQL API usage examples (at least 5 operations)
   - Capability taxonomy reference table
   - Status transition diagram or explanation
   - Troubleshooting guide with common errors
   - Minimum 200 lines of content
   - **Pass Criteria**: README comprehensive and well-structured
   - **Fail Action**: Add missing sections to README.md

2. **API Documentation Completeness**
   ```powershell
   Test-Path docs/api/agent-registry-api.md
   Get-Content docs/api/agent-registry-api.md | Select-String "query|mutation" | Measure-Object -Line
   ```
   Verify API documentation contains:
   - All 5 queries documented with:
     - Parameters
     - Sample GraphQL request
     - Sample response
   - All 5 mutations documented with:
     - Input types
     - Validation rules
     - Error codes
     - Sample request/response
   - Enum definitions (AgentStatus, EndpointDiscoveryType, etc.)
   - At least 3 curl examples for HTTP testing
   - **Pass Criteria**: All API operations fully documented
   - **Fail Action**: Add missing API documentation

3. **Migration Guide Completeness**
   ```powershell
   Test-Path chaoscenter/graphql/server/pkg/agent_registry/MIGRATION.md
   ```
   Verify MIGRATION.md contains:
   - Migration script execution steps
   - Rollback procedure
   - Index verification commands
   - Data validation queries
   - **Pass Criteria**: Migration guide clear and actionable
   - **Fail Action**: Complete migration documentation

4. **Main README Update**
   ```powershell
   Get-Content README.md | Select-String "Agent Registry"
   ```
   Verify README.md updated with:
   - Agent Registry in feature list
   - Link to detailed documentation
   - Architecture diagram mentions Agent Registry component
   - **Pass Criteria**: Main README references Agent Registry
   - **Fail Action**: Update main README.md

5. **Kubernetes Manifests Validation**
   ```powershell
   kubectl apply --dry-run=client -f chaoscenter/manifests/langfuse-secret.yaml
   kubectl apply --dry-run=client -f chaoscenter/manifests/agent-registry-config.yaml
   kubectl apply --dry-run=client -f chaoscenter/graphql/server/manifests/deployment.yaml
   ```
   Verify:
   - langfuse-secret.yaml has template for LANGFUSE_API_KEY
   - agent-registry-config.yaml has all required config values
   - deployment.yaml references ConfigMap and Secret
   - deployment.yaml has environment variables set
   - ServiceAccount with RBAC permissions defined
   - **Pass Criteria**: All manifests valid YAML
   - **Fail Action**: Fix YAML syntax or structure

6. **Docker Build Test**
   ```powershell
   cd chaoscenter/graphql/server
   docker build -t agent-registry-test:latest .
   ```
   - Verify build succeeds
   - Verify Go modules downloaded including new dependencies
   - Check image size reasonable (< 500MB for Go binary)
   - **Pass Criteria**: Docker image builds successfully
   - **Fail Action**: Fix Dockerfile, ensure dependencies included

7. **Security Review - JWT and RBAC**
   ```powershell
   go test ./pkg/agent_registry -run TestSecurityJWTandRBAC -v
   ```
   Create test `TestSecurityJWTandRBAC` to:
   - Test operations with valid JWT token
   - Test operations with missing/invalid JWT (should fail)
   - Test PROJECT_MEMBER role:
     - Can perform read operations (getAgent, listAgents)
     - Cannot perform write operations (updateAgent, deleteAgent)
   - Test PROJECT_OWNER role:
     - Can perform all operations
   - Test cross-project access:
     - User cannot access agents from different project
   - **Pass Criteria**: Authorization properly enforced
   - **Fail Action**: Add authorization checks to resolvers/service

8. **Security Review - Input Validation**
   ```powershell
   go test ./pkg/agent_registry -run TestSecurityInputValidation -v
   ```
   Create test `TestSecurityInputValidation` to:
   - Test SQL injection attempts in searchTerm:
     - `'; DROP TABLE agents; --`
     - Verify sanitized or escaped
   - Test NoSQL injection in filters:
     - `{ "$ne": null }`
     - Verify rejected or escaped
   - Test SSRF protection in endpoint URL:
     - `http://localhost:8080/admin`
     - `http://169.254.169.254/latest/meta-data`
     - Verify rejected
   - Test XSS in agent name/description:
     - `<script>alert('xss')</script>`
     - Verify sanitized
   - **Pass Criteria**: All injection attacks prevented
   - **Fail Action**: Add input sanitization, URL validation

9. **Security Review - Secrets Protection**
   ```powershell
   go test ./pkg/agent_registry -run TestSecuritySecretsProtection -v
   ```
   Create test `TestSecuritySecretsProtection` to:
   - Perform agent operations
   - Capture all log output
   - Verify LANGFUSE_API_KEY never appears in logs
   - Query GraphQL /graphql endpoint
   - Verify API keys not exposed in responses
   - Query /metrics endpoint
   - Verify no sensitive data in metrics
   - **Pass Criteria**: No secrets leaked
   - **Fail Action**: Remove API keys from logs, sanitize responses

10. **Dependency Vulnerability Scan**
    ```powershell
    go list -json -m all | docker run --rm -i sonatypecommunity/nancy:latest sleuth
    ```
    Or use alternative:
    ```powershell
    go install golang.org/x/vuln/cmd/govulncheck@latest
    govulncheck ./pkg/agent_registry/...
    ```
    - Scan for known CVEs in dependencies
    - **Pass Criteria**: No high or critical vulnerabilities
    - Minor/low vulnerabilities acceptable if no fix available
    - **Fail Action**: Update vulnerable dependencies, apply patches

11. **Development Environment Deployment Test**
    ```powershell
    # Deploy to local/dev Kubernetes cluster
    kubectl apply -f migrations/001_create_agent_registry.js # Run via mongosh
    kubectl create secret generic langfuse-api-key --from-literal=LANGFUSE_API_KEY=test-key-123
    kubectl apply -f chaoscenter/manifests/agent-registry-config.yaml
    kubectl apply -f chaoscenter/graphql/server/manifests/deployment.yaml
    kubectl wait --for=condition=Ready pod -l app=graphql-server --timeout=120s
    ```
    - Verify deployment succeeds
    - Verify pods running
    - Check pod logs for startup errors:
      ```powershell
      kubectl logs -l app=graphql-server --tail=50
      ```
    - Verify health check scheduler starts
    - Verify watcher starts
    - Execute sample GraphQL query:
      ```powershell
      kubectl port-forward svc/graphql-server 8080:8080
      curl -X POST http://localhost:8080/graphql -H "Content-Type: application/json" -d '{"query": "{ getAgentCapabilitiesTaxonomy { id name } }"}'
      ```
    - Verify response received
    - **Pass Criteria**: Complete deployment successful, API responsive
    - **Fail Action**: Debug deployment issues, check logs, fix manifests

12. **Production Readiness Checklist Verification**
    ```powershell
    # Verify checklist items
    $checklist = @(
        "MongoDB indexes created",
        "Langfuse API key configured",
        "ConfigMap deployed",
        "Resource limits set",
        "Metrics scraping configured",
        "Grafana dashboard imported",
        "Log aggregation configured",
        "Backup strategy documented"
    )
    # Manually verify each item or automate checks
    ```
    Review production deployment checklist (TASK-091):
    - ✅ MongoDB indexes created (verify with getIndexes)
    - ✅ Langfuse API key configured in Secret
    - ✅ ConfigMap deployed with production URLs
    - ✅ Resource limits set (CPU, memory) in deployment.yaml
    - ✅ Health check interval appropriate
    - ✅ Prometheus scraping /metrics endpoint
    - ✅ Grafana dashboard imported
    - ✅ Log aggregation configured (e.g., Fluent Bit, Loki)
    - ✅ Backup strategy for MongoDB documented
    - ✅ Disaster recovery tested
    - **Pass Criteria**: All checklist items verified
    - **Fail Action**: Complete missing checklist items

**Overall Phase 10 Pass Criteria**:
- ✅ All 12 validation steps pass
- ✅ All documentation complete and comprehensive
- ✅ Kubernetes manifests valid and tested
- ✅ Docker image builds successfully
- ✅ Security review passed (JWT, RBAC, input validation, secrets)
- ✅ No critical vulnerabilities in dependencies
- ✅ Dev environment deployment successful
- ✅ Production readiness checklist complete
- ✅ System ready for production deployment

**Rollback Actions** (if checkpoint fails):
1. Identify which documentation or deployment test failed
2. Complete missing documentation sections
3. Fix Kubernetes manifest errors
4. Address security vulnerabilities
5. Resolve deployment issues
6. Update production checklist
7. Re-run failed validation
8. Once all validations pass, implementation is COMPLETE

---

## **FINAL IMPLEMENTATION VALIDATION**

After completing all 10 implementation phases and passing all checkpoints, perform this final validation:

```powershell
# Final Comprehensive Test Suite
cd chaoscenter/graphql/server

# 1. Build
go build -o graphql-server.exe ./...

# 2. Run all tests
go test ./pkg/agent_registry/... -v -cover -race -count=1

# 3. Check coverage
go test ./pkg/agent_registry/... -coverprofile=coverage.out
go tool cover -func=coverage.out | Select-String "total:"

# 4. Security scan
govulncheck ./pkg/agent_registry/...

# 5. Lint
golangci-lint run ./pkg/agent_registry/...

# 6. Deploy to test environment
kubectl apply -f migrations/001_create_agent_registry.js
kubectl apply -f chaoscenter/manifests/
kubectl apply -f chaoscenter/graphql/server/manifests/deployment.yaml
kubectl wait --for=condition=Ready pod -l app=graphql-server --timeout=300s

# 7. Execute end-to-end GraphQL test
# (Port forward and run GraphQL queries)

# 8. Verify metrics endpoint
curl http://localhost:8080/metrics | Select-String "agent_registry"

# 9. Verify logs structured
kubectl logs -l app=graphql-server | Select-String "agentId"

# 10. Run load test (optional)
# k6 run load-test.js
```

**Final Pass Criteria**:
- ✅ Build succeeds
- ✅ All tests pass (100+ tests total)
- ✅ Coverage >= 85%
- ✅ No race conditions
- ✅ No critical vulnerabilities
- ✅ No critical linting issues
- ✅ Deployment successful
- ✅ GraphQL API functional
- ✅ Metrics exposed
- ✅ Logs structured

**If all checkpoints and final validation pass**: 
✅ **Implementation is COMPLETE and ready for production deployment**

**If any checkpoint fails**:
❌ **STOP and rollback to the last successful checkpoint, fix issues, and retry**

---

## 3. Dependencies

### Internal Dependencies

- **DEP-001**: chaoscenter/graphql/server (Go 1.22+) - Host application for Agent Registry package
- **DEP-002**: chaoscenter/authentication gRPC service - User authentication and project membership validation
- **DEP-003**: MongoDB 5.0+ - Data persistence for agent documents with existing connection pool
- **DEP-004**: Existing GraphQL middleware - JWT extraction, context propagation, error handling
- **DEP-005**: logrus v1.9+ - Structured logging library used throughout LitmusChaos
- **DEP-006**: gqlgen v0.17+ - GraphQL schema code generation

### External Dependencies

- **DEP-007**: Langfuse SaaS or self-hosted instance - Agent observability and metadata correlation platform; API must be accessible from GraphQL server pods; API key with project access required
- **DEP-008**: Kubernetes API server - Service discovery for agent endpoint auto-discovery; requires in-cluster access or kubeconfig; ServiceAccount must have permissions to list/get Services in agent namespaces
- **DEP-009**: Prometheus - Metrics collection and alerting; must be configured to scrape GraphQL server /metrics endpoint
- **DEP-010**: Grafana - Metrics visualization; dashboard import requires admin access

### Go Module Dependencies

- **DEP-011**: github.com/google/uuid v1.6.0 - UUID generation for agent IDs
- **DEP-012**: go.mongodb.org/mongo-driver v1.13.0 - MongoDB client library
- **DEP-013**: github.com/99designs/gqlgen v0.17.40 - GraphQL server framework
- **DEP-014**: k8s.io/client-go v0.28.0 - Kubernetes API client for service discovery
- **DEP-015**: k8s.io/api v0.28.0 - Kubernetes API types
- **DEP-016**: k8s.io/apimachinery v0.28.0 - Kubernetes API machinery
- **DEP-017**: github.com/stretchr/testify v1.8.4 - Testing assertions and mocks
- **DEP-018**: github.com/testcontainers/testcontainers-go v0.26.0 - Integration testing with containers

### Dependency Installation

- **DEP-019**: Execute `go get github.com/google/uuid@v1.6.0` in chaoscenter/graphql/server directory
- **DEP-020**: Execute `go get k8s.io/client-go@v0.28.0 k8s.io/api@v0.28.0 k8s.io/apimachinery@v0.28.0`
- **DEP-021**: Execute `go mod tidy` to resolve transitive dependencies
- **DEP-022**: Update go.mod and go.sum files in version control

## 4. Files

### New Files to Create

- **FILE-001**: `chaoscenter/graphql/server/pkg/agent_registry/handler.go` - GraphQL resolver handlers with 10 public methods for mutations/queries
- **FILE-002**: `chaoscenter/graphql/server/pkg/agent_registry/service.go` - Business logic layer with Service interface and serviceImpl struct, 9 public methods
- **FILE-003**: `chaoscenter/graphql/server/pkg/agent_registry/operator.go` - MongoDB data access layer with Operator interface and operatorImpl struct, 7 public methods
- **FILE-004**: `chaoscenter/graphql/server/pkg/agent_registry/model.go` - Data structures: Agent, HelmReleaseInfo, KubernetesResourceInfo, AgentEndpoint, LangfuseConfig, AgentMetadata, AuditInfo structs with BSON and JSON tags
- **FILE-005**: `chaoscenter/graphql/server/pkg/agent_registry/validator.go` - Input validation with Validator interface, 4 public methods, regex validators
- **FILE-006**: `chaoscenter/graphql/server/pkg/agent_registry/langfuse_client.go` - Langfuse API client with LangfuseClient interface, 2 public methods, retry logic
- **FILE-007**: `chaoscenter/graphql/server/pkg/agent_registry/watcher.go` - Kubernetes resource watcher with AgentWatcher struct, Start/Stop/watch methods for ConfigMaps/Deployments/Services, event handlers
- **FILE-008**: `chaoscenter/graphql/server/pkg/agent_registry/metadata_extractor.go` - Metadata extraction from ConfigMaps with MetadataExtractor interface, ExtractFromConfigMap method
- **FILE-009**: `chaoscenter/graphql/server/pkg/agent_registry/health_scheduler.go` - Background health check scheduler with Start/Stop methods
- **FILE-009**: `chaoscenter/graphql/server/pkg/agent_registry/health_scheduler.go` - Background health check scheduler with Start/Stop methods
- **FILE-010**: `chaoscenter/graphql/server/pkg/agent_registry/constants.go` - Constants: collection name, status enums (DISCOVERED, VALIDATING, ACTIVE, INACTIVE, DELETED), discovery type enums, endpoint type enums, Kubernetes label keys, timeouts
- **FILE-011**: `chaoscenter/graphql/server/pkg/agent_registry/errors.go` - Custom error types: 11 error variables for different failure scenarios including ErrK8sResourceNotFound
- **FILE-012**: `chaoscenter/graphql/server/pkg/agent_registry/operator_test.go` - Unit tests for operator layer with 9 test functions including GetAgentByResourceUID
- **FILE-013**: `chaoscenter/graphql/server/pkg/agent_registry/validator_test.go` - Unit tests for validator layer with 8 test functions (removed container image validation tests)
- **FILE-014**: `chaoscenter/graphql/server/pkg/agent_registry/service_test.go` - Unit tests for service layer with 11 test functions using RegisterAgentFromWatcher instead of RegisterAgent
- **FILE-015**: `chaoscenter/graphql/server/pkg/agent_registry/langfuse_client_test.go` - Unit tests for Langfuse client with 4 test functions
- **FILE-016**: `chaoscenter/graphql/server/pkg/agent_registry/health_scheduler_test.go` - Unit tests for health scheduler with 2 test functions
- **FILE-017**: `chaoscenter/graphql/server/pkg/agent_registry/watcher_test.go` - Unit tests for Kubernetes watcher with 4 test functions
- **FILE-018**: `chaoscenter/graphql/server/pkg/agent_registry/metadata_extractor_test.go` - Unit tests for metadata extractor with 5 test functions
- **FILE-019**: `chaoscenter/graphql/server/pkg/agent_registry/integration_test.go` - Integration tests with 4 test scenarios using testcontainers and mock k8s client
- **FILE-020**: `chaoscenter/graphql/server/pkg/agent_registry/README.md` - Package documentation with architecture, configuration, Helm chart requirements (ConfigMap-based metadata), usage examples
- **FILE-021**: `chaoscenter/graphql/server/graph/schema/agent_registry.graphqls` - GraphQL schema definitions: 10 types (including HelmReleaseInfo, KubernetesResourceInfo), 8 input types, 5 queries, 5 mutations (added triggerAgentDiscovery)
- **FILE-018**: `chaoscenter/manifests/langfuse-secret.yaml` - Kubernetes Secret template for Langfuse API key
- **FILE-019**: `chaoscenter/manifests/agent-registry-config.yaml` - Kubernetes ConfigMap with Agent Registry configuration
- **FILE-020**: `chaoscenter/manifests/grafana-agent-registry-dashboard.json` - Grafana dashboard for Agent Registry metrics
- **FILE-021**: `migrations/001_create_agent_registry.js` - MongoDB migration script to create collection and indexes
- **FILE-022**: `docs/api/agent-registry-api.md` - API documentation with request/response examples for all operations
- **FILE-023**: `chaoscenter/graphql/server/pkg/agent_registry/MIGRATION.md` - Database migration guide with execution and rollback steps

### Files to Modify

- **FILE-024**: `chaoscenter/graphql/server/main.go` - Add Agent Registry initialization code (operator, validator, service, handler, scheduler) after MongoDB setup, wire to GraphQL resolvers, add graceful shutdown for scheduler (~50 lines added)
- **FILE-025**: `chaoscenter/graphql/server/graph/resolver.go` - Add agentRegistryHandler field to Resolver struct, initialize in NewResolver constructor
- **FILE-026**: `chaoscenter/graphql/server/graph/mutation.resolver.go` - Implement 5 mutation resolvers by calling handler methods
- **FILE-027**: `chaoscenter/graphql/server/graph/query.resolver.go` - Implement 5 query resolvers by calling handler methods
- **FILE-028**: `chaoscenter/graphql/server/manifests/deployment.yaml` - Add environment variables for Langfuse and health check configuration, mount Langfuse secret
- **FILE-029**: `chaoscenter/graphql/server/chart/values.yaml` - Add Agent Registry configuration section with Langfuse and health check settings
- **FILE-030**: `chaoscenter/graphql/server/go.mod` - Add new dependencies (uuid, k8s client-go, testcontainers)
- **FILE-031**: `chaoscenter/graphql/server/go.sum` - Auto-generated checksums for new dependencies
- **FILE-032**: `README.md` - Update feature list to include Agent Registry Service, add link to documentation

## 5. Testing

### Unit Tests

- **TEST-001**: TestCreateAgent_Success - Verify operator.CreateAgent inserts document into MongoDB with correct structure and returns no error; use testcontainers for real MongoDB
- **TEST-002**: TestGetAgent_NotFound - Verify operator.GetAgent returns ErrAgentNotFound when agentId doesn't exist in collection
- **TEST-003**: TestGetAgentByProjectAndName_Uniqueness - Verify compound index enforces uniqueness by attempting to insert duplicate (projectId, name) and expecting error
- **TEST-004**: TestListAgents_Pagination - Insert 25 agents, query with page=2 limit=10, verify returns agents 11-20 and totalCount=25
- **TEST-005**: TestListAgents_StatusFilter - Insert 5 ACTIVE and 3 INACTIVE agents, query with status=ACTIVE filter, verify returns only 5 agents
- **TEST-006**: TestListAgents_CapabilitiesFilter - Insert agents with various capabilities, query with capabilities filter, verify returned agents contain specified capabilities
- **TEST-007**: TestGetAgentsByCapabilities_ANDLogic - Insert Agent-A with [cap1, cap2], Agent-B with [cap1, cap3], Agent-C with [cap1, cap2, cap3]; query with [cap1, cap2]; verify only Agent-A and Agent-C returned
- **TEST-008**: TestUpdateAgent_Success - Create agent, modify version and status fields, call UpdateAgent, verify changes persisted and updatedAt timestamp updated
- **TEST-009**: TestDeleteAgent_HardDelete - Create agent, call DeleteAgent, verify document removed from collection
- **TEST-010**: TestValidateName_ValidFormats - Test validator.ValidateRegistration with valid names: "agent-1", "my-agent", "a", "agent-name-123"; all should pass
- **TEST-011**: TestValidateName_InvalidFormats - Test with invalid names: "Agent-1" (uppercase), "agent_name" (underscore), "-agent" (leading hyphen), "agent-" (trailing hyphen), "agent name" (space), 64-char name (too long); all should return ErrInvalidAgentName
- **TEST-012**: TestValidateSemver_ValidVersions - Test with "1.0.0", "v2.1.3", "1.0.0-alpha", "1.0.0+build"; all should pass
- **TEST-013**: TestValidateSemver_InvalidVersions - Test with "1.0", "v1", "1.0.0.0"; all should return ErrInvalidVersion
- **TEST-014**: TestValidateCapabilities_Taxonomy - Test with valid capabilities from taxonomy, verify pass; test with "unknown-capability", verify ErrInvalidCapabilities
- **TEST-015**: TestValidateCapabilities_Empty - Test with empty capabilities array, verify ErrInvalidCapabilities
- **TEST-018**: TestRegisterAgentFromWatcher_Success - Mock operator.CreateAgent, validator.ValidateMetadata passing, call service.RegisterAgentFromWatcher, verify UUID generated, status=DISCOVERED, auditInfo populated with discoveredAt timestamp, returns no error
- **TEST-019**: TestRegisterAgent_DuplicateName - Mock operator.GetAgentByProjectAndName returning existing agent, call service.RegisterAgent, verify returns ErrDuplicateAgentName
- **TEST-020**: TestRegisterAgent_InvalidCapabilities - Call service.RegisterAgent with unknown capability, verify validator.ValidateRegistration fails, returns ErrInvalidCapabilities
- **TEST-021**: TestRegisterAgent_EndpointAutoDiscovery - Mock k8sClient to return Service with ClusterIP and port 8080, call service.RegisterAgent without endpoint, verify endpoint auto-discovered as "http://agent-name.namespace.svc.cluster.local:8080", discoveryType=AUTO
- **TEST-022**: TestUpdateAgent_FieldMerge - Create agent with version "1.0.0" and capabilities [cap1], call UpdateAgent with version "2.0.0" only, verify version updated but capabilities unchanged
- **TEST-023**: TestDeleteAgent_SoftDelete - Call service.DeleteAgent with hardDelete=false, verify operator.UpdateAgent called with status=DELETED, document remains in DB
- **TEST-024**: TestDeleteAgent_HardDelete - Call service.DeleteAgent with hardDelete=true, verify operator.DeleteAgent called, document removed from DB
- **TEST-025**: TestGetAgentsByCapabilities_FilterLogic - Mock operator.GetAgentsByCapabilities, call service method, verify filter passed correctly to operator
- **TEST-026**: TestValidateAgentHealth_Success - Create mock HTTP server responding 200 to /health and /ready, register agent with mock endpoint, call service.ValidateAgentHealth, verify returns healthy=true, status transitions to ACTIVE, lastHealthCheck timestamp updated
- **TEST-027**: TestValidateAgentHealth_Timeout - Create mock HTTP server with 10s delay, call service.ValidateAgentHealth with 5s timeout, verify returns healthy=false with timeout error message, status transitions to INACTIVE
- **TEST-028**: TestValidateAgentHealth_Unhealthy - Create mock HTTP server responding 503 to /health, call service.ValidateAgentHealth, verify returns healthy=false, status=INACTIVE
- **TEST-029**: TestSyncToLangfuse_Success - Mock langfuseClient.CreateOrUpdateUser returning nil, call service.SyncToLangfuse, verify payload contains correct agentId, name, metadata, lastSyncedAt updated
- **TEST-030**: TestSyncToLangfuse_Disabled - Create agent with LangfuseConfig.syncEnabled=false, call service.SyncToLangfuse, verify langfuseClient not called, returns nil immediately
- **TEST-031**: TestLangfuseClient_CreateOrUpdateUser_Success - Create mock HTTP server responding 201 to POST /api/public/users, call client.CreateOrUpdateUser, verify request includes Authorization header with Bearer token, Content-Type application/json, payload with ID and metadata
- **TEST-032**: TestLangfuseClient_RetryLogic - Create mock HTTP server responding 500 twice then 201, call client.CreateOrUpdateUser, verify 3 HTTP requests made with delays approximately 1s, 2s between retries
- **TEST-033**: TestLangfuseClient_AllRetriesFail - Create mock HTTP server always responding 500, call client.CreateOrUpdateUser, verify returns error after 3 retries
- **TEST-034**: TestLangfuseClient_DeleteUser - Call client.DeleteUser, verify creates payload with metadata.deleted=true and calls CreateOrUpdateUser
- **TEST-035**: TestHealthCheckScheduler_PeriodicExecution - Create service mock tracking health check calls, create scheduler with 100ms interval, start in goroutine, sleep 350ms, stop scheduler, verify approximately 3 health check cycles executed
- **TEST-036**: TestHealthCheckScheduler_GracefulShutdown - Create scheduler, start, immediately call Stop, verify stopChan closed and no goroutine leaks

### Integration Tests

- **TEST-037**: TestEndToEndRegistrationFlow - Start real MongoDB container, create operator/validator/service instances, register agent with all fields, verify inserted in DB, call GetAgent and verify returned correctly, call UpdateAgent with version change, verify updated in DB, call DeleteAgent with hardDelete=true, verify removed from DB
- **TEST-038**: TestConcurrentRegistration - Start MongoDB container, launch 10 goroutines each registering agent with unique name in same project, use WaitGroup to wait for completion, query DB for project agents, verify 10 agents exist with no duplicate names
- **TEST-039**: TestCapabilityQueryMultipleAgents - Register 5 agents: Agent1[cap1,cap2], Agent2[cap1,cap3], Agent3[cap2,cap3], Agent4[cap1,cap2,cap3], Agent5[cap4]; query with capabilities[cap1,cap2]; verify only Agent1 and Agent4 returned
- **TEST-040**: TestHealthCheckStatusTransitions - Start MongoDB and mock HTTP server for agent endpoint, register agent via watcher with mock endpoint, start health scheduler with 1s interval, verify status transitions DISCOVERED → VALIDATING → ACTIVE, stop mock server, wait for health check, verify status transitions ACTIVE → INACTIVE, restart mock server, wait for health check, verify status transitions INACTIVE → ACTIVE

### Performance Tests

- **TEST-041**: LoadTest_RegistrationThroughput - Use k6 to execute 100 registerAgent mutations within 60 seconds with unique agent names, measure p95 latency, verify p95 < 500ms per REQ-001
- **TEST-042**: LoadTest_ListAgentsQuery - Prepopulate DB with 100 agents, use k6 to execute 500 listAgents queries with pagination over 60 seconds, measure p95 latency, verify p95 < 200ms per NFR-002
- **TEST-043**: LoadTest_CapabilityQuery - Prepopulate DB with 1000 agents with varied capabilities, execute getAgentsByCapabilities queries with different capability combinations, measure p95 latency, verify p95 < 300ms
- **TEST-044**: LoadTest_ConcurrentHealthChecks - Register 50 agents with mock endpoints, trigger health checks simultaneously via scheduler, measure completion time and resource usage, verify no goroutine leaks or memory spikes

### End-to-End Tests

- **TEST-045**: E2E_HelmToDatabase - Deploy GraphQL server with Agent Registry Watcher in test cluster, create agent Helm chart with ConfigMap containing metadata, deploy via `helm install`, verify watcher detects ConfigMap ADDED event, verify agent registered in MongoDB with status DISCOVERED, query MongoDB directly to verify document structure matches model with helmRelease and kubernetesResources fields, execute getAgent query via GraphQL and verify response matches DB document
- **TEST-046**: E2E_LangfuseIntegration - Setup test Langfuse project, configure API key in GraphQL server, register agent with Langfuse sync enabled, verify Langfuse API receives CreateOrUpdateUser request with correct payload, check Langfuse dashboard for user entry, update agent, verify Langfuse user metadata updated
- **TEST-047**: E2E_KubernetesDiscoveryViaHelm - Deploy test agent via Helm chart in Kubernetes cluster with Service, ConfigMap with metadata not specifying endpoint, verify watcher discovers agent, verify GraphQL server auto-discovers endpoint from K8s Service API, execute health check, verify GraphQL server successfully calls discovered endpoint, verify agent status transitions to ACTIVE
- **TEST-048**: E2E_AuthorizationEnforcement - Create two test users with different project memberships, authenticate User1 (PROJECT_MEMBER), attempt updateAgent mutation, verify allowed for view operations but fails with FORBIDDEN error for update, deploy agent via Helm with projectId for User2's project, verify User1 cannot access agent via getAgent, authenticate User2 (PROJECT_OWNER), verify getAgent succeeds for their project's agent
