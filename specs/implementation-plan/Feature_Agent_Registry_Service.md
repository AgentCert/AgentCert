# Feature: Agent Registry Service
## Implementation Plan

**Document Version**: 1.0

---

# Introduction

This implementation plan defines the complete development roadmap for the Agent Registry Service, a core backend component of the AgentCert platform that manages the complete lifecycle of AI agents within the chaos engineering ecosystem. The service provides agent registration, metadata management, capability-based querying, health validation, and Langfuse synchronization through a GraphQL API backed by MongoDB.

## 1. Requirements & Constraints

### Functional Requirements

- **REQ-001**: Agent registration with comprehensive metadata (name, version, vendor, capabilities, container image, namespace, endpoint, Langfuse project ID)
- **REQ-002**: Agent name must be unique within project scope
- **REQ-003**: Automatic endpoint discovery from Kubernetes Service using convention: `http://{agent-name}.{namespace}.svc.cluster.local:8080`
- **REQ-004**: Agent containers MUST expose `/health` and `/ready` endpoints
- **REQ-005**: Support CRUD operations for agent metadata with audit trail
- **REQ-006**: Capability-based querying using AND logic (agents must support ALL specified capabilities)
- **REQ-007**: Agent health validation via HTTP endpoints with status transitions: REGISTERED → VALIDATING → ACTIVE → INACTIVE → DELETED
- **REQ-008**: Synchronize agent metadata to Langfuse on registration, update, status change, and deletion
- **REQ-009**: Graceful degradation when Langfuse sync fails (non-blocking)
- **REQ-010**: Support soft delete (mark inactive) and hard delete operations

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
| TASK-001 | Create directory structure `chaoscenter/graphql/server/pkg/agent_registry/` with files: `handler.go`, `service.go`, `operator.go`, `model.go`, `validator.go`, `langfuse_client.go`, `health_scheduler.go`, `constants.go`, `errors.go`, `agent_registry_test.go` | | |
| TASK-002 | Define data models in `model.go`: `Agent` struct with fields (AgentID, ProjectID, Name, Version, Vendor, Capabilities, ContainerImage, Namespace, Endpoint, LangfuseConfig, Status, Metadata, AuditInfo), `ContainerImage` struct, `AgentEndpoint` struct with discovery type enum, `LangfuseConfig` struct, `AgentMetadata` struct with labels/annotations | | |
| TASK-003 | Define constants in `constants.go`: Collection name `agent_registry_collection`, status enums (REGISTERED, VALIDATING, ACTIVE, INACTIVE, DELETED), endpoint discovery types (AUTO, MANUAL), endpoint types (REST, GRPC), health check paths, timeouts | | |
| TASK-004 | Define custom errors in `errors.go`: ErrAgentNotFound, ErrDuplicateAgentName, ErrInvalidAgentName, ErrInvalidVersion, ErrInvalidCapabilities, ErrInvalidContainerImage, ErrHealthCheckFailed, ErrLangfuseSyncFailed, ErrUnauthorized, ErrInsufficientPermissions | | |
| TASK-005 | Implement `Operator` interface in `operator.go` with methods: CreateAgent, GetAgent, GetAgentByProjectAndName, ListAgents, UpdateAgent, DeleteAgent, GetAgentsByCapabilities | | |
| TASK-006 | Implement `CreateAgent` method using `collection.InsertOne` with agent document | | |
| TASK-007 | Implement `GetAgent` method using `collection.FindOne` with filter `{"agentId": id}` and return ErrAgentNotFound if not found | | |
| TASK-008 | Implement `ListAgents` method with filter builder for projectId, status, capabilities, searchTerm; apply pagination with skip/limit; return agents array and total count using `collection.CountDocuments` | | |
| TASK-009 | Implement `UpdateAgent` method using `collection.ReplaceOne` with filter `{"agentId": agent.AgentID}` and update timestamps | | |
| TASK-010 | Implement `DeleteAgent` method using `collection.DeleteOne` with filter `{"agentId": id}` | | |
| TASK-011 | Implement `GetAgentsByCapabilities` method with filter using `$all` operator for capabilities array and projectId filter; use index hint for capabilities index | | |
| TASK-012 | Create MongoDB migration script `migrations/001_create_agent_registry.js` to create collection and indexes: unique index on agentId, compound unique index on (projectId, name), index on (status, auditInfo.createdAt), multikey index on capabilities | | |

### Implementation Phase 2: Validation Layer

**GOAL-002**: Implement input validation and business rule enforcement

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-013 | Implement `Validator` interface in `validator.go` with methods: ValidateRegistration, ValidateUpdate, ValidateCapabilities, ValidateContainerImage | | |
| TASK-014 | Implement `ValidateRegistration` method: check name uniqueness by calling operator.GetAgentByProjectAndName, validate name format using regex `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` with max 63 chars, validate version is valid semver using regex `^v?(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9.-]+))?(?:\+([a-zA-Z0-9.-]+))?$`, validate capabilities not empty, validate container image format | | |
| TASK-015 | Implement `ValidateCapabilities` method: check each capability exists in taxonomy map loaded from configuration or hardcoded list of supported capabilities | | |
| TASK-016 | Implement `ValidateContainerImage` method: verify registry/repository/tag are non-empty, validate registry contains dot (domain format), validate repository follows standard format, validate tag is non-empty | | |
| TASK-017 | Implement `ValidateUpdate` method: if name provided validate format, if version provided validate semver, if capabilities provided validate against taxonomy, if container image provided validate format | | |
| TASK-018 | Implement helper function `loadCapabilitiesTaxonomy` returning map with predefined capabilities: pod-crash-remediation, pod-delete-remediation, node-drain-remediation, network-latency-remediation, network-partition-remediation, disk-pressure-remediation, memory-stress-remediation, cpu-stress-remediation, container-kill-remediation, service-unavailable-remediation | | |

### Implementation Phase 3: Service Layer - Core Logic

**GOAL-003**: Implement business logic layer with agent lifecycle operations

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-019 | Implement `Service` interface in `service.go` with methods: RegisterAgent, GetAgent, ListAgents, UpdateAgent, DeleteAgent, GetAgentsByCapabilities, ValidateAgentHealth, SyncToLangfuse, GetCapabilitiesTaxonomy | | |
| TASK-020 | Implement `RegisterAgent` method: validate input using validator.ValidateRegistration, generate UUID for agentId, determine endpoint discovery (if not provided, call discoverAgentEndpoint), create Agent struct with status REGISTERED and audit info, call operator.CreateAgent, asynchronously call SyncToLangfuse in goroutine with error logging, initiate health check asynchronously to transition to VALIDATING status, return agent | | |
| TASK-021 | Implement `discoverAgentEndpoint` helper: use k8sClient.CoreV1().Services(namespace).Get to find service matching agent name, if found construct endpoint `http://{serviceName}.{namespace}.svc.cluster.local:{port}`, set discoveryType=AUTO, healthPath="/health", readyPath="/ready", if not found return error with suggestion to provide manual endpoint | | |
| TASK-022 | Implement `GetAgent` method: call operator.GetAgent(id), verify user has access to agent's project via JWT context, return agent or authorization error | | |
| TASK-023 | Implement `ListAgents` method: build AgentFilter from input parameters, verify user project access, call operator.ListAgents with filter and pagination, return AgentListResponse with agents array, totalCount, pageInfo (currentPage, totalPages, hasNextPage) | | |
| TASK-024 | Implement `UpdateAgent` method: validate input using validator.ValidateUpdate, call operator.GetAgent to fetch existing agent, verify user authorization (PROJECT_OWNER or PROJECT_ADMIN), merge updates into agent struct preserving non-updated fields, update auditInfo.updatedAt and updatedBy, call operator.UpdateAgent, asynchronously sync to Langfuse if metadata changed, return updated agent | | |
| TASK-025 | Implement `DeleteAgent` method: call operator.GetAgent to verify exists, verify user authorization (PROJECT_OWNER or PROJECT_ADMIN), check for active benchmarks using agent (future: query benchmark service), if hardDelete=true call operator.DeleteAgent else update status to DELETED and call operator.UpdateAgent, asynchronously sync deletion to Langfuse, return DeleteAgentResponse with success=true | | |
| TASK-026 | Implement `GetAgentsByCapabilities` method: verify user project access, call operator.GetAgentsByCapabilities with projectId and capabilities list, return filtered agents | | |
| TASK-027 | Implement `GetCapabilitiesTaxonomy` method: return list of CapabilityDefinition structs with id, name, description, category fields | | |

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

### Implementation Phase 5: Langfuse Integration

**GOAL-005**: Implement Langfuse API client with retry logic for metadata synchronization

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-035 | Implement `LangfuseClient` interface in `langfuse_client.go` with methods: CreateOrUpdateUser, DeleteUser | | |
| TASK-036 | Define `LangfuseUserPayload` struct with fields: ID (agentId), Name (agent name), Metadata (map with version, capabilities, containerImage, status, namespace, registeredAt, updatedAt) | | |
| TASK-037 | Implement `NewLangfuseClient` function: accept baseURL and apiKey from environment variables, create HTTP client with 10s timeout, return langfuseClientImpl | | |
| TASK-038 | Implement `CreateOrUpdateUser` method: marshal payload to JSON, create HTTP POST request to `{baseURL}/api/public/users`, set headers (Authorization: Bearer {apiKey}, Content-Type: application/json), set projectId in request context or headers as required by Langfuse API, implement retry logic with exponential backoff (3 retries, delays 1s, 2s, 4s), check response status 200/201, log errors on failure, return error if all retries fail | | |
| TASK-039 | Implement `DeleteUser` method: create payload with Metadata containing deleted:true flag and deletedAt timestamp, call CreateOrUpdateUser with updated payload (Langfuse doesn't support hard delete) | | |
| TASK-040 | Implement `SyncToLangfuse` method in service layer: check if agent.LangfuseConfig is nil or syncEnabled=false and return early if disabled, check if LANGFUSE_API_KEY environment variable is set, build LangfuseUserPayload from agent fields, call langfuseClient.CreateOrUpdateUser, update agent.LangfuseConfig.lastSyncedAt timestamp, call operator.UpdateAgent to persist sync timestamp, log sync success or failure, return error only if sync is critical (currently non-blocking) | | |

### Implementation Phase 6: GraphQL API Layer

**GOAL-006**: Implement GraphQL schema and resolver handlers

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-041 | Define GraphQL schema in `chaoscenter/graphql/server/graph/schema/agent_registry.graphqls`: add Agent type with all fields from model, add ContainerImage type, add AgentEndpoint type with EndpointDiscoveryType and EndpointType enums, add LangfuseConfig type, add AgentStatus enum, add AgentMetadata type with KeyValuePair, add AuditInfo type | | |
| TASK-042 | Define input types in schema: RegisterAgentInput, ContainerImageInput, AgentEndpointInput, LangfuseConfigInput, AgentMetadataInput, KeyValuePairInput, UpdateAgentInput, ListAgentsFilter, PaginationInput | | |
| TASK-043 | Define response types in schema: RegisterAgentResponse with agent and langfuseSyncStatus fields, AgentListResponse with agents array and pagination fields (totalCount, currentPage, totalPages, hasNextPage), AgentStatusResponse with agentId, status, healthy, lastCheckedAt, lastSyncedToLangfuse, HealthCheckResult with healthy, message, responseTime, checkedAt, CapabilityDefinition with id, name, description, category, DeleteAgentResponse with success and message, SyncResponse with success, syncedAt, message | | |
| TASK-044 | Define Query operations in schema: getAgent(id: ID!), listAgents(filter: ListAgentsFilter, pagination: PaginationInput!), getAgentsByCapabilities(projectId: String!, capabilities: [String!]!), getAgentStatus(id: ID!), getAgentCapabilitiesTaxonomy | | |
| TASK-045 | Define Mutation operations in schema: registerAgent(input: RegisterAgentInput!), updateAgent(id: ID!, input: UpdateAgentInput!), deleteAgent(id: ID!, hardDelete: Boolean), validateAgentHealth(id: ID!), syncAgentToLangfuse(id: ID!) | | |
| TASK-046 | Run gqlgen code generation: execute `go run github.com/99designs/gqlgen generate` to generate resolver stubs in `chaoscenter/graphql/server/graph/generated/` | | |
| TASK-047 | Implement `Handler` struct in `handler.go` with service field of type Service interface | | |
| TASK-048 | Implement `NewHandler` function accepting Service and returning *Handler | | |
| TASK-049 | Implement `RegisterAgent` resolver in handler: extract JWT claims from context using existing auth middleware helper, verify user has PROJECT_OWNER or PROJECT_ADMIN role for input.ProjectId, transform model.RegisterAgentInput to internal RegisterAgentRequest struct, call service.RegisterAgent, transform result to model.RegisterAgentResponse with agent and SyncStatus (SUCCESS/FAILED/SKIPPED based on Langfuse sync result), handle errors and return appropriate GraphQL error codes | | |
| TASK-050 | Implement `GetAgent` resolver in handler: extract user context, call service.GetAgent(id), transform internal Agent to model.Agent, handle ErrAgentNotFound with NOT_FOUND error code, handle authorization errors with FORBIDDEN code | | |
| TASK-051 | Implement `ListAgents` resolver in handler: extract user context, transform model.ListAgentsFilter and PaginationInput to internal types, call service.ListAgents, transform results to model.AgentListResponse with calculated pagination fields (totalPages = ceil(totalCount / limit), hasNextPage = currentPage < totalPages) | | |
| TASK-052 | Implement `UpdateAgent` resolver in handler: extract user context, verify authorization, transform model.UpdateAgentInput to internal type, call service.UpdateAgent, transform result to model.Agent, handle errors | | |
| TASK-053 | Implement `DeleteAgent` resolver in handler: extract user context, verify authorization, call service.DeleteAgent with hardDelete flag, return model.DeleteAgentResponse | | |
| TASK-054 | Implement `GetAgentsByCapabilities` resolver in handler: extract user context, verify user has access to projectId, call service.GetAgentsByCapabilities, transform results to model.Agent array | | |
| TASK-055 | Implement `GetAgentStatus` resolver in handler: extract user context, call service.GetAgent to verify access, call service.ValidateAgentHealth to get current status, return model.AgentStatusResponse with health check results and Langfuse sync status | | |
| TASK-056 | Implement `ValidateAgentHealth` resolver in handler: extract user context, call service.ValidateAgentHealth, return model.AgentStatusResponse | | |
| TASK-057 | Implement `SyncAgentToLangfuse` resolver in handler: extract user context, verify authorization, call service.GetAgent, call service.SyncToLangfuse, return model.SyncResponse with success status and timestamp | | |
| TASK-058 | Implement `GetAgentCapabilitiesTaxonomy` resolver in handler: call service.GetCapabilitiesTaxonomy, transform to model.CapabilityDefinition array, return results | | |

### Implementation Phase 7: Integration and Initialization

**GOAL-007**: Integrate Agent Registry into GraphQL server and configure startup

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-059 | Modify `chaoscenter/graphql/server/main.go`: import agent_registry package, after MongoDB initialization create agent_registry.Operator using NewOperator(mongoDatabase), create agent_registry.Validator using NewValidator(operator), create Kubernetes clientset using k8s.io/client-go/kubernetes and in-cluster config, read LANGFUSE_BASE_URL and LANGFUSE_API_KEY from environment variables, create agent_registry.LangfuseClient using NewLangfuseClient(baseURL, apiKey) if configured, create agent_registry.Service using NewService(operator, validator, langfuseClient, k8sClient, logger), create agent_registry.Handler using NewHandler(service) | | |
| TASK-060 | Initialize HealthCheckScheduler in main.go: read AGENT_HEALTH_CHECK_INTERVAL from environment with default "5m", parse interval using time.ParseDuration, create scheduler using NewHealthCheckScheduler(service, interval, logger), start scheduler in goroutine using scheduler.Start(context.Background()), register graceful shutdown to call scheduler.Stop() on SIGTERM/SIGINT | | |
| TASK-061 | Wire Agent Registry handlers to GraphQL resolvers: in resolver.go mutation resolver, add agentRegistryHandler field, implement RegisterAgent mutation by calling handler.RegisterAgent, implement UpdateAgent mutation by calling handler.UpdateAgent, implement DeleteAgent mutation by calling handler.DeleteAgent, implement ValidateAgentHealth mutation by calling handler.ValidateAgentHealth, implement SyncAgentToLangfuse mutation by calling handler.SyncAgentToLangfuse | | |
| TASK-062 | Wire Agent Registry handlers to GraphQL queries: in resolver.go query resolver, implement getAgent query by calling handler.GetAgent, implement listAgents query by calling handler.ListAgents, implement getAgentsByCapabilities query by calling handler.GetAgentsByCapabilities, implement getAgentStatus query by calling handler.GetAgentStatus, implement getAgentCapabilitiesTaxonomy query by calling handler.GetAgentCapabilitiesTaxonomy | | |
| TASK-063 | Create Kubernetes Secret manifest `chaoscenter/manifests/langfuse-secret.yaml` with template for LANGFUSE_API_KEY, document in deployment guide | | |
| TASK-064 | Update GraphQL server Deployment manifest `chaoscenter/graphql/server/manifests/deployment.yaml`: add environment variables LANGFUSE_BASE_URL (from ConfigMap), LANGFUSE_API_KEY (from Secret), AGENT_HEALTH_CHECK_INTERVAL (default "5m"), AGENT_HEALTH_CHECK_TIMEOUT (default "5s") | | |

### Implementation Phase 8: Testing

**GOAL-008**: Implement comprehensive unit and integration tests

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-065 | Create test file `operator_test.go`: implement TestCreateAgent_Success with mock MongoDB using testcontainers, insert agent and verify, implement TestGetAgent_NotFound verifying ErrAgentNotFound returned, implement TestGetAgentByProjectAndName_Success for uniqueness check, implement TestListAgents_WithPagination verifying skip/limit logic and count accuracy, implement TestListAgents_WithFilters testing status filter, capabilities filter, searchTerm filter, implement TestGetAgentsByCapabilities_ANDLogic with 3 agents having different capabilities verifying only agents with ALL capabilities returned, implement TestUpdateAgent_Success verifying updated fields and timestamps, implement TestDeleteAgent_Success verifying document removal | | |
| TASK-066 | Create test file `validator_test.go`: implement TestValidateName_Valid with valid names (agent-1, my-agent, a, agent-name-123), implement TestValidateName_Invalid with invalid names (Agent-1 uppercase, agent_name underscore, -agent leading hyphen, agent- trailing hyphen, agent name with space, 64+ char name), implement TestValidateSemver_Valid with versions (1.0.0, v2.1.3, 1.0.0-alpha, 1.0.0+build), implement TestValidateSemver_Invalid with versions (1.0, v1, 1.0.0.0), implement TestValidateCapabilities_Valid with taxonomy match, implement TestValidateCapabilities_Invalid with unknown capability, implement TestValidateCapabilities_Empty verifying error, implement TestValidateContainerImage_Valid with proper registry/repo/tag, implement TestValidateContainerImage_Invalid with missing fields | | |
| TASK-067 | Create test file `service_test.go`: implement TestRegisterAgent_Success mocking operator.CreateAgent and verifying UUID generation, status REGISTERED, audit timestamps, implement TestRegisterAgent_DuplicateName mocking operator.GetAgentByProjectAndName returning existing agent and verifying ErrDuplicateAgentName, implement TestRegisterAgent_InvalidCapabilities with unknown capability verifying validation error, implement TestRegisterAgent_EndpointDiscovery mocking k8sClient to return service and verifying auto-discovered endpoint, implement TestUpdateAgent_Success mocking operator methods and verifying merge logic, implement TestDeleteAgent_SoftDelete verifying status set to DELETED, implement TestDeleteAgent_HardDelete verifying operator.DeleteAgent called, implement TestListAgents_WithFilters mocking operator and verifying filter passthrough, implement TestGetAgentsByCapabilities_Success verifying capability filtering, implement TestValidateAgentHealth_Success mocking HTTP server with /health returning 200 and verifying status transition to ACTIVE, implement TestValidateAgentHealth_Timeout mocking slow server and verifying status INACTIVE with error message, implement TestValidateAgentHealth_Unhealthy mocking /health returning 503 and verifying INACTIVE status | | |
| TASK-068 | Create test file `langfuse_client_test.go`: implement TestCreateOrUpdateUser_Success mocking HTTP server with POST /api/public/users returning 201 and verifying request payload contains correct agentId, name, metadata, implement TestCreateOrUpdateUser_Retry mocking server to fail twice then succeed and verifying 3 attempts made with exponential backoff delays, implement TestCreateOrUpdateUser_AllRetriesFail mocking server to always return 500 and verifying error returned after 3 retries, implement TestDeleteUser_Success verifying payload contains deleted:true in metadata | | |
| TASK-069 | Create test file `health_scheduler_test.go`: implement TestHealthCheckScheduler_Start creating scheduler with 100ms interval, starting in goroutine, sleeping 350ms, stopping scheduler, verifying health checks ran approximately 3 times by checking logs or metrics, implement TestHealthCheckScheduler_Stop verifying graceful shutdown by checking stopChan closed and no panics | | |
| TASK-070 | Create integration test file `integration_test.go`: setup testcontainers for MongoDB, create real operator/validator/service instances, implement TestEndToEndRegistrationFlow: register agent via service, verify in MongoDB, query by ID, verify returned, update agent, verify changes persisted, delete agent, verify removed, implement TestConcurrentRegistration: launch 10 goroutines each registering agent with unique name, wait for all to complete, verify all 10 agents in DB with no errors, implement TestCapabilityQueryWithMultipleAgents: register 5 agents with different capability combinations, query with 2 capabilities, verify only agents with BOTH capabilities returned, implement TestHealthCheckCycle: register agent with mock HTTP server endpoint, start health scheduler, wait for health check, verify status transitioned to ACTIVE, stop mock server, wait for health check, verify status transitioned to INACTIVE | | |
| TASK-071 | Run all tests with coverage: execute `go test ./pkg/agent_registry/... -cover -coverprofile=coverage.out`, verify coverage >= 85%, generate HTML report using `go tool cover -html=coverage.out`, review uncovered lines and add tests if critical paths uncovered | | |

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
- **FILE-004**: `chaoscenter/graphql/server/pkg/agent_registry/model.go` - Data structures: Agent, ContainerImage, AgentEndpoint, LangfuseConfig, AgentMetadata, AuditInfo structs with BSON and JSON tags
- **FILE-005**: `chaoscenter/graphql/server/pkg/agent_registry/validator.go` - Input validation with Validator interface, 4 public methods, regex validators
- **FILE-006**: `chaoscenter/graphql/server/pkg/agent_registry/langfuse_client.go` - Langfuse API client with LangfuseClient interface, 2 public methods, retry logic
- **FILE-007**: `chaoscenter/graphql/server/pkg/agent_registry/health_scheduler.go` - Background health check scheduler with Start/Stop methods
- **FILE-008**: `chaoscenter/graphql/server/pkg/agent_registry/constants.go` - Constants: collection name, status enums, discovery type enums, endpoint type enums, timeouts
- **FILE-009**: `chaoscenter/graphql/server/pkg/agent_registry/errors.go` - Custom error types: 10 error variables for different failure scenarios
- **FILE-010**: `chaoscenter/graphql/server/pkg/agent_registry/operator_test.go` - Unit tests for operator layer with 8 test functions
- **FILE-011**: `chaoscenter/graphql/server/pkg/agent_registry/validator_test.go` - Unit tests for validator layer with 8 test functions
- **FILE-012**: `chaoscenter/graphql/server/pkg/agent_registry/service_test.go` - Unit tests for service layer with 11 test functions
- **FILE-013**: `chaoscenter/graphql/server/pkg/agent_registry/langfuse_client_test.go` - Unit tests for Langfuse client with 4 test functions
- **FILE-014**: `chaoscenter/graphql/server/pkg/agent_registry/health_scheduler_test.go` - Unit tests for health scheduler with 2 test functions
- **FILE-015**: `chaoscenter/graphql/server/pkg/agent_registry/integration_test.go` - Integration tests with 4 test scenarios using testcontainers
- **FILE-016**: `chaoscenter/graphql/server/pkg/agent_registry/README.md` - Package documentation with architecture, configuration, usage examples
- **FILE-017**: `chaoscenter/graphql/server/graph/schema/agent_registry.graphqls` - GraphQL schema definitions: 8 types, 8 input types, 5 queries, 5 mutations
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
- **TEST-016**: TestValidateContainerImage_Valid - Test with properly formatted image {registry: "docker.io", repository: "chaos/agent", tag: "v1.0"}, verify pass
- **TEST-017**: TestValidateContainerImage_MissingFields - Test with missing registry, repository, or tag; verify ErrInvalidContainerImage
- **TEST-018**: TestRegisterAgent_Success - Mock operator.CreateAgent, validator.ValidateRegistration passing, call service.RegisterAgent, verify UUID generated, status=REGISTERED, auditInfo populated, returns Agent without error
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
- **TEST-040**: TestHealthCheckStatusTransitions - Start MongoDB and mock HTTP server for agent endpoint, register agent with mock endpoint, start health scheduler with 1s interval, verify status transitions REGISTERED → VALIDATING → ACTIVE, stop mock server, wait for health check, verify status transitions ACTIVE → INACTIVE, restart mock server, wait for health check, verify status transitions INACTIVE → ACTIVE

### Performance Tests

- **TEST-041**: LoadTest_RegistrationThroughput - Use k6 to execute 100 registerAgent mutations within 60 seconds with unique agent names, measure p95 latency, verify p95 < 500ms per REQ-001
- **TEST-042**: LoadTest_ListAgentsQuery - Prepopulate DB with 100 agents, use k6 to execute 500 listAgents queries with pagination over 60 seconds, measure p95 latency, verify p95 < 200ms per NFR-002
- **TEST-043**: LoadTest_CapabilityQuery - Prepopulate DB with 1000 agents with varied capabilities, execute getAgentsByCapabilities queries with different capability combinations, measure p95 latency, verify p95 < 300ms
- **TEST-044**: LoadTest_ConcurrentHealthChecks - Register 50 agents with mock endpoints, trigger health checks simultaneously via scheduler, measure completion time and resource usage, verify no goroutine leaks or memory spikes

### End-to-End Tests

- **TEST-045**: E2E_GraphQLToMongoDB - Deploy GraphQL server with Agent Registry in test cluster, use GraphQL client to execute registerAgent mutation, verify response contains agent ID, query MongoDB directly to verify document structure matches model, execute getAgent query and verify response matches DB document
- **TEST-046**: E2E_LangfuseIntegration - Setup test Langfuse project, configure API key in GraphQL server, register agent with Langfuse sync enabled, verify Langfuse API receives CreateOrUpdateUser request with correct payload, check Langfuse dashboard for user entry, update agent, verify Langfuse user metadata updated
- **TEST-047**: E2E_KubernetesDiscovery - Deploy test agent pod with Service in Kubernetes cluster, register agent without providing endpoint, verify GraphQL server discovers endpoint from K8s Service API, execute health check, verify GraphQL server successfully calls discovered endpoint
- **TEST-048**: E2E_AuthorizationEnforcement - Create two test users with different project memberships, authenticate User1 (PROJECT_MEMBER), attempt registerAgent mutation, verify fails with FORBIDDEN error, authenticate User2 (PROJECT_OWNER), attempt same mutation, verify succeeds, authenticate User1, attempt getAgent for User2's agent, verify fails with FORBIDDEN
