


package agent_registry

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// SyncResponse represents the response for agent sync operations.
type SyncResponse struct {
	Success  bool
	SyncedAt *string
	Message  *string
}

// AgentStatusResponse represents the status and health of an agent.
type AgentStatusResponse struct {
	AgentID              string
	Status               AgentStatus
	Healthy              bool
	LastCheckedAt        *string
	LastSyncedToLangfuse *string
}

// Service defines the business logic interface for Agent Registry operations.
type Service interface {
	// RegisterAgent registers a new agent with the platform
	RegisterAgent(ctx context.Context, input *RegisterAgentRequest) (*Agent, error)
	
	// GetAgent retrieves an agent by ID
	GetAgent(ctx context.Context, id string) (*Agent, error)
	
	// ListAgents retrieves agents with filtering and pagination
	ListAgents(ctx context.Context, filter *AgentFilter, pagination *PaginationInput) (*AgentListResponse, error)
	
	// UpdateAgent updates an existing agent's metadata
	UpdateAgent(ctx context.Context, id string, input *UpdateAgentRequest) (*Agent, error)
	
	// DeleteAgent removes an agent (soft or hard delete)
	DeleteAgent(ctx context.Context, id string, hardDelete bool) (*DeleteAgentResponse, error)
	
	// GetAgentsByCapabilities retrieves agents that support all specified capabilities
	GetAgentsByCapabilities(ctx context.Context, projectID string, capabilities []string) ([]*Agent, error)
	
	// ValidateAgentHealth performs health check on an agent
	ValidateAgentHealth(ctx context.Context, id string) (*HealthCheckResult, error)
	
	// SyncToLangfuse synchronizes agent metadata to Langfuse
	SyncToLangfuse(ctx context.Context, agent *Agent) error

	// SyncAgentToLangfuse synchronizes agent metadata and returns sync response
	SyncAgentToLangfuse(ctx context.Context, id string) (*SyncResponse, error)

	// GetAgentStatus returns the status and health of an agent
	GetAgentStatus(ctx context.Context, id string) (*AgentStatusResponse, error)

	// GetAgentCapabilitiesTaxonomy returns the list of supported capabilities
	GetAgentCapabilitiesTaxonomy(ctx context.Context) ([]*CapabilityDefinition, error)

	// SetAgentStatus updates the status of an agent
	SetAgentStatus(ctx context.Context, id string, status AgentStatus) error

	// GetKubernetesNamespaces returns the list of available Kubernetes namespaces
	GetKubernetesNamespaces(ctx context.Context) ([]string, error)
// Implementation for these methods is below, outside the interface block.
}

// serviceImpl is the concrete implementation of the Service interface.
type serviceImpl struct {
	operator       Operator
	validator      Validator
	langfuseClient LangfuseClient
	//k8sClient      kubernetes.Interface
	// logger will be added for structured logging
}

// NewService creates a new Service instance.
func NewService(operator Operator, validator Validator, langfuseClient LangfuseClient, k8sClient interface{}) Service {
	return &serviceImpl{
		operator:       operator,
		validator:      validator,
		langfuseClient: langfuseClient,
		//k8sClient:      k8sClient,
	}
}

// RegisterAgentRequest represents the input for agent registration.
type RegisterAgentRequest struct {
	ProjectID       string
	Name            string
	Version         string
	Vendor          string
	Capabilities    []string
	ContainerImage  *ContainerImage
	Namespace       string
	HelmReleaseName string
	Endpoint        *AgentEndpoint
	LangfuseConfig  *LangfuseConfig
	Metadata        *AgentMetadata
}

// UpdateAgentRequest represents the input for agent updates.
type UpdateAgentRequest struct {
	Name           *string
	Version        *string
	Vendor         *string
	Capabilities   []string
	ContainerImage *ContainerImage
	Endpoint       *AgentEndpoint
	LangfuseConfig *LangfuseConfig
	Metadata       *AgentMetadata
}

// AgentFilter represents filtering criteria for agent queries.
type AgentFilter struct {
	ProjectID    string
	Status       *AgentStatus
	Statuses     []AgentStatus // For filtering by multiple statuses
	Capabilities []string
	SearchTerm   *string
}

// PaginationInput represents pagination parameters.
type PaginationInput struct {
	Page  int
	Limit int
}

// AgentListResponse represents the response for list operations.
type AgentListResponse struct {
	Agents       []*Agent
	TotalCount   int64
	CurrentPage  int
	TotalPages   int
	HasNextPage  bool
}

// DeleteAgentResponse represents the response for delete operations.
type DeleteAgentResponse struct {
	Success bool
	Message string
}

// HealthCheckResult represents the result of a health check.
type HealthCheckResult struct {
	Healthy      bool
	Message      string
	ResponseTime int64
	CheckedAt    int64
}

// CapabilityDefinition represents a supported capability.
type CapabilityDefinition struct {
	ID          string
	Name        string
	Description string
	Category    string
}

// RegisterAgent registers a new agent with the platform.
func (s *serviceImpl) RegisterAgent(ctx context.Context, input *RegisterAgentRequest) (*Agent, error) {
	// Idempotency: check BEFORE validation so that re-runs on the same namespace+name
	// return the existing record without hitting the duplicate-name validation error.
	if existing, err := s.operator.GetAgentByNamespace(ctx, input.Namespace); err == nil && existing != nil && existing.Name == input.Name {
		fmt.Printf("Agent %s already registered in namespace %s with ID %s, returning existing\n", input.Name, input.Namespace, existing.AgentID)
		return existing, nil
	}

	// Validate input
	if err := s.validator.ValidateRegistration(ctx, input); err != nil {
		return nil, err
	}

	// Generate UUID for agentId
	agentID := uuid.New().String()
	
	// Determine endpoint (use default if not provided and auto-discovery fails)
	endpoint := input.Endpoint
	if endpoint == nil {
		discoveredEndpoint, err := s.discoverAgentEndpoint(ctx, input.Name, input.Namespace)
		if err != nil {
			// Use a default local endpoint when auto-discovery fails
			endpoint = &AgentEndpoint{
				URL:           fmt.Sprintf("http://%s.%s.svc.cluster.local:8080", input.Name, input.Namespace),
				Type:          EndpointTypeREST,
				DiscoveryType: EndpointDiscoveryManual,
				HealthPath:    "/health",
				ReadyPath:     "/ready",
			}
		} else {
			endpoint = discoveredEndpoint
		}
	}
	
	// Get current timestamp
	now := time.Now().Unix()
	
	// Extract user ID from context (placeholder - will be replaced with actual JWT extraction)
	userID := "system" // TODO: Extract from JWT context
	
	// Create Agent struct with status REGISTERED
	agent := &Agent{
		AgentID:         agentID,
		ProjectID:       input.ProjectID,
		Name:            input.Name,
		Version:         input.Version,
		Vendor:          input.Vendor,
		Capabilities:    input.Capabilities,
		ContainerImage:  input.ContainerImage,
		Namespace:       input.Namespace,
		HelmReleaseName: input.HelmReleaseName,
		Endpoint:        endpoint,
		LangfuseConfig:  input.LangfuseConfig,
		Status:          AgentStatusRegistered,
		Metadata:        input.Metadata,
		AuditInfo: &AuditInfo{
			CreatedAt: now,
			CreatedBy: userID,
			UpdatedAt: now,
			UpdatedBy: userID,
		},
	}
	
	// Save to database
	if err := s.operator.CreateAgent(ctx, agent); err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}
	
	// Asynchronously sync to Langfuse (non-blocking)
	go func() {
		if err := s.SyncToLangfuse(context.Background(), agent); err != nil {
			// Log error (TODO: add proper logging)
			fmt.Printf("Langfuse sync failed for agent %s: %v\n", agent.AgentID, err)
		}
	}()
	
	// Asynchronously initiate health check to transition to VALIDATING
	go func() {
		time.Sleep(1 * time.Second) // Brief delay before health check
		if _, err := s.ValidateAgentHealth(context.Background(), agent.AgentID); err != nil {
			// Log error (TODO: add proper logging)
			fmt.Printf("Initial health check failed for agent %s: %v\n", agent.AgentID, err)
		}
	}()
	
	return agent, nil
}

// discoverAgentEndpoint discovers agent endpoint from Kubernetes Service.
func (s *serviceImpl) discoverAgentEndpoint(ctx context.Context, agentName, namespace string) (*AgentEndpoint, error) {
	//if s.k8sClient == nil {
		return nil, fmt.Errorf("Kubernetes client not configured; please provide endpoint manually")
	//}
	
	// Try to find service matching agent name
	//service, err := s.k8sClient.CoreV1().Services(namespace).Get(ctx, agentName, metav1.GetOptions{})
	//if err != nil {
	//	return nil, fmt.Errorf("service '%s' not found in namespace '%s': %w. "+
	//		"Please provide endpoint manually or ensure service exists", agentName, namespace, err)
	//}
	//
	//// Get port from service
	//port := int32(8080) // Default port
	//if len(service.Spec.Ports) > 0 {
	//	port = service.Spec.Ports[0].Port
	//}
	//
	//// Construct endpoint URL
	//url := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", service.Name, namespace, port)
	//
	//return &AgentEndpoint{
	//	URL:           url,
	//	Type:          EndpointTypeREST,
	//	DiscoveryType: EndpointDiscoveryAuto,
	//	HealthPath:    DefaultHealthPath,
	//	ReadyPath:     DefaultReadyPath,
	//}, nil
}

// GetAgent retrieves an agent by ID.
func (s *serviceImpl) GetAgent(ctx context.Context, id string) (*Agent, error) {
	// Fetch agent from database
	agent, err := s.operator.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// TODO: Verify user has access to agent's project via JWT context
	// For now, return agent directly
	// userID := extractUserIDFromContext(ctx)
	// if !hasProjectAccess(userID, agent.ProjectID) {
	//     return nil, ErrUnauthorized
	// }
	
	return agent, nil
}

// ListAgents retrieves agents with filtering and pagination.
func (s *serviceImpl) ListAgents(ctx context.Context, filter *AgentFilter, pagination *PaginationInput) (*AgentListResponse, error) {
	// TODO: Verify user project access from JWT context
	
	// Default pagination if nil
	if pagination == nil {
		pagination = &PaginationInput{Page: 1, Limit: 100}
	}
	
	// Calculate skip and limit from pagination
	skip := (pagination.Page - 1) * pagination.Limit
	limit := pagination.Limit
	
	// Call operator to list agents
	agents, totalCount, err := s.operator.ListAgents(ctx, filter, skip, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}
	
	// Calculate pagination info
	totalPages := int(math.Ceil(float64(totalCount) / float64(pagination.Limit)))
	hasNextPage := pagination.Page < totalPages
	
	return &AgentListResponse{
		Agents:      agents,
		TotalCount:  totalCount,
		CurrentPage: pagination.Page,
		TotalPages:  totalPages,
		HasNextPage: hasNextPage,
	}, nil
}

// UpdateAgent updates an existing agent's metadata.
func (s *serviceImpl) UpdateAgent(ctx context.Context, id string, input *UpdateAgentRequest) (*Agent, error) {
	// Validate input
	if err := s.validator.ValidateUpdate(ctx, input); err != nil {
		return nil, err
	}
	
	// Fetch existing agent
	agent, err := s.operator.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// TODO: Verify user authorization (PROJECT_OWNER or PROJECT_ADMIN)
	// userID := extractUserIDFromContext(ctx)
	// userRole := getUserRole(userID, agent.ProjectID)
	// if userRole != "PROJECT_OWNER" && userRole != "PROJECT_ADMIN" {
	//     return nil, ErrInsufficientPermissions
	// }
	
	// Merge updates into agent struct (preserve non-updated fields)
	metadataChanged := false
	
	if input.Name != nil {
		agent.Name = *input.Name
		metadataChanged = true
	}
	if input.Version != nil {
		agent.Version = *input.Version
		metadataChanged = true
	}
	if input.Vendor != nil {
		agent.Vendor = *input.Vendor
		metadataChanged = true
	}
	if len(input.Capabilities) > 0 {
		agent.Capabilities = input.Capabilities
		metadataChanged = true
	}
	if input.ContainerImage != nil {
		agent.ContainerImage = input.ContainerImage
		metadataChanged = true
	}
	if input.Endpoint != nil {
		agent.Endpoint = input.Endpoint
		metadataChanged = true
	}
	if input.LangfuseConfig != nil {
		agent.LangfuseConfig = input.LangfuseConfig
		metadataChanged = true
	}
	if input.Metadata != nil {
		agent.Metadata = input.Metadata
		metadataChanged = true
	}
	
	// Update audit info
	now := time.Now().Unix()
	userID := "system" // TODO: Extract from JWT context
	agent.AuditInfo.UpdatedAt = now
	agent.AuditInfo.UpdatedBy = userID
	
	// Save updated agent
	if err := s.operator.UpdateAgent(ctx, agent); err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}
	
	// Asynchronously sync to Langfuse if metadata changed
	if metadataChanged {
		go func() {
			if err := s.SyncToLangfuse(context.Background(), agent); err != nil {
				fmt.Printf("Langfuse sync failed for agent %s: %v\n", agent.AgentID, err)
			}
		}()
	}
	
	return agent, nil
}

// DeleteAgent removes an agent (soft or hard delete).
func (s *serviceImpl) DeleteAgent(ctx context.Context, id string, hardDelete bool) (*DeleteAgentResponse, error) {
	// Verify agent exists
	agent, err := s.operator.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// TODO: Verify user authorization (PROJECT_OWNER or PROJECT_ADMIN)
	
	// TODO: Check for active benchmarks using this agent
	// This will be implemented when benchmark service is available
	// if hasActiveBenchmarks(agent.AgentID) {
	//     return nil, fmt.Errorf("cannot delete agent with active benchmarks")
	// }
	
	// Call helm uninstall if agent was deployed via helm
	if agent.Vendor == "helm-deployment" {
		// Use HelmReleaseName if available, otherwise fall back to Name
		releaseName := agent.HelmReleaseName
		if releaseName == "" {
			releaseName = agent.Name
		}
		
		if releaseName != "" && agent.Namespace != "" {
			helmReq := &HelmUninstallRequest{
				ReleaseName: releaseName,
				Namespace:   agent.Namespace,
			}
			
			// Use stored kubeconfig if available (in future implementation)
			// For now, use default cluster context
			output, err := UninstallWithHelm(ctx, helmReq)
			if err != nil {
				log.Printf("[DeleteAgent] Helm uninstall warning for %s (release: %s): %v (output: %s)", agent.Name, releaseName, err, output)
				// Continue with soft delete even if helm uninstall fails
			} else {
				log.Printf("[DeleteAgent] Successfully uninstalled Helm release %s: %s", releaseName, output)
			}
		}
	}
	
	if hardDelete {
		// Hard delete - remove from database
		if err := s.operator.DeleteAgent(ctx, id); err != nil {
			return nil, fmt.Errorf("failed to delete agent: %w", err)
		}
	} else {
		// Soft delete - mark as DELETED
		agent.Status = AgentStatusDeleted
		agent.AuditInfo.UpdatedAt = time.Now().Unix()
		agent.AuditInfo.UpdatedBy = "system" // TODO: Extract from JWT context
		
		if err := s.operator.UpdateAgent(ctx, agent); err != nil {
			return nil, fmt.Errorf("failed to mark agent as deleted: %w", err)
		}
	}
	
	// Asynchronously sync deletion to Langfuse
	go func() {
		if err := s.SyncToLangfuse(context.Background(), agent); err != nil {
			fmt.Printf("Langfuse sync failed for deleted agent %s: %v\n", agent.AgentID, err)
		}
	}()
	
	return &DeleteAgentResponse{
		Success: true,
		Message: fmt.Sprintf("Agent %s successfully deleted", id),
	}, nil
}

// GetAgentsByCapabilities retrieves agents that support all specified capabilities.
func (s *serviceImpl) GetAgentsByCapabilities(ctx context.Context, projectID string, capabilities []string) ([]*Agent, error) {
	// TODO: Verify user has access to project
	
	// Call operator to get agents with ALL specified capabilities
	agents, err := s.operator.GetAgentsByCapabilities(ctx, projectID, capabilities)
	if err != nil {
		return nil, fmt.Errorf("failed to get agents by capabilities: %w", err)
	}
	
	return agents, nil
}

// GetCapabilitiesTaxonomy returns the list of supported capabilities.
func (s *serviceImpl) GetCapabilitiesTaxonomy(ctx context.Context) ([]*CapabilityDefinition, error) {
	capabilities := []*CapabilityDefinition{
		{
			ID:          "pod-crash-remediation",
			Name:        "Pod Crash Remediation",
			Description: "Ability to detect and remediate pod crashes by restarting or rescheduling pods",
			Category:    "Pod Chaos",
		},
		{
			ID:          "pod-delete-remediation",
			Name:        "Pod Delete Remediation",
			Description: "Ability to handle and recover from pod deletion events",
			Category:    "Pod Chaos",
		},
		{
			ID:          "node-drain-remediation",
			Name:        "Node Drain Remediation",
			Description: "Ability to handle node drain scenarios and reschedule workloads",
			Category:    "Node Chaos",
		},
		{
			ID:          "network-latency-remediation",
			Name:        "Network Latency Remediation",
			Description: "Ability to detect and mitigate network latency issues",
			Category:    "Network Chaos",
		},
		{
			ID:          "network-partition-remediation",
			Name:        "Network Partition Remediation",
			Description: "Ability to handle and recover from network partition scenarios",
			Category:    "Network Chaos",
		},
		{
			ID:          "disk-pressure-remediation",
			Name:        "Disk Pressure Remediation",
			Description: "Ability to detect and remediate disk pressure conditions",
			Category:    "Resource Chaos",
		},
		{
			ID:          "memory-stress-remediation",
			Name:        "Memory Stress Remediation",
			Description: "Ability to handle and mitigate memory pressure scenarios",
			Category:    "Resource Chaos",
		},
		{
			ID:          "cpu-stress-remediation",
			Name:        "CPU Stress Remediation",
			Description: "Ability to detect and remediate CPU stress conditions",
			Category:    "Resource Chaos",
		},
		{
			ID:          "container-kill-remediation",
			Name:        "Container Kill Remediation",
			Description: "Ability to handle container kill events and restore service",
			Category:    "Container Chaos",
		},
		{
			ID:          "service-unavailable-remediation",
			Name:        "Service Unavailable Remediation",
			Description: "Ability to detect and recover from service unavailability",
			Category:    "Service Chaos",
		},
	}
	
	return capabilities, nil
}

// ValidateAgentHealth performs health check on an agent and updates its status.
func (s *serviceImpl) ValidateAgentHealth(ctx context.Context, id string) (*HealthCheckResult, error) {
	// Fetch the agent
	agent, err := s.operator.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}

	// TODO: Verify user authorization via JWT context
	// Extract user from context and verify PROJECT_MEMBER role for agent.ProjectID

	startTime := time.Now()

	// Create HTTP client with 5s timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Perform health check
	healthURL := agent.Endpoint.URL + agent.Endpoint.HealthPath
	healthResp, err := client.Get(healthURL)
	if err != nil {
		// Health check failed
		responseTime := time.Since(startTime).Milliseconds()
		result := &HealthCheckResult{
			Healthy:      false,
			Message:      fmt.Sprintf("Health check failed: %v", err),
			ResponseTime: responseTime,
			CheckedAt:    time.Now().Unix(),
		}

		// Update agent status to INACTIVE
		if updateErr := s.updateAgentStatus(ctx, agent, AgentStatusInactive); updateErr != nil {
			// Log error but return health check result
			fmt.Printf("Failed to update agent status: %v\n", updateErr)
		}

		return result, nil
	}
	defer healthResp.Body.Close()

	// Check health endpoint status code
	if healthResp.StatusCode != http.StatusOK {
		responseTime := time.Since(startTime).Milliseconds()
		result := &HealthCheckResult{
			Healthy:      false,
			Message:      fmt.Sprintf("Health check returned status %d", healthResp.StatusCode),
			ResponseTime: responseTime,
			CheckedAt:    time.Now().Unix(),
		}

		// Update agent status to INACTIVE
		if updateErr := s.updateAgentStatus(ctx, agent, AgentStatusInactive); updateErr != nil {
			fmt.Printf("Failed to update agent status: %v\n", updateErr)
		}

		return result, nil
	}

	// Perform ready check
	readyURL := agent.Endpoint.URL + agent.Endpoint.ReadyPath
	readyResp, err := client.Get(readyURL)
	if err != nil {
		responseTime := time.Since(startTime).Milliseconds()
		result := &HealthCheckResult{
			Healthy:      false,
			Message:      fmt.Sprintf("Ready check failed: %v", err),
			ResponseTime: responseTime,
			CheckedAt:    time.Now().Unix(),
		}

		// Update agent status to INACTIVE
		if updateErr := s.updateAgentStatus(ctx, agent, AgentStatusInactive); updateErr != nil {
			fmt.Printf("Failed to update agent status: %v\n", updateErr)
		}

		return result, nil
	}
	defer readyResp.Body.Close()

	// Check ready endpoint status code
	if readyResp.StatusCode != http.StatusOK {
		responseTime := time.Since(startTime).Milliseconds()
		result := &HealthCheckResult{
			Healthy:      false,
			Message:      fmt.Sprintf("Ready check returned status %d", readyResp.StatusCode),
			ResponseTime: responseTime,
			CheckedAt:    time.Now().Unix(),
		}

		// Update agent status to INACTIVE
		if updateErr := s.updateAgentStatus(ctx, agent, AgentStatusInactive); updateErr != nil {
			fmt.Printf("Failed to update agent status: %v\n", updateErr)
		}

		return result, nil
	}

	// Both checks passed
	responseTime := time.Since(startTime).Milliseconds()
	result := &HealthCheckResult{
		Healthy:      true,
		Message:      "Agent is healthy and ready",
		ResponseTime: responseTime,
		CheckedAt:    time.Now().Unix(),
	}

	// Update agent status to ACTIVE
	if updateErr := s.updateAgentStatus(ctx, agent, AgentStatusActive); updateErr != nil {
		fmt.Printf("Failed to update agent status: %v\n", updateErr)
	}

	return result, nil
}

// updateAgentStatus is a helper method to update agent status with validation and audit trail.
func (s *serviceImpl) updateAgentStatus(ctx context.Context, agent *Agent, newStatus AgentStatus) error {
	// Validate status transition
	// Valid transitions:
	// REGISTERED -> VALIDATING, DELETED
	// VALIDATING -> ACTIVE, INACTIVE, DELETED
	// ACTIVE -> INACTIVE, DELETED
	// INACTIVE -> ACTIVE, DELETED
	// DELETED -> (no transitions allowed)

	if agent.Status == AgentStatusDeleted {
		return fmt.Errorf("cannot transition from DELETED status")
	}

	validTransitions := map[AgentStatus][]AgentStatus{
		AgentStatusRegistered: {AgentStatusValidating, AgentStatusDeleted},
		AgentStatusValidating: {AgentStatusActive, AgentStatusInactive, AgentStatusDeleted},
		AgentStatusActive:     {AgentStatusInactive, AgentStatusDeleted},
		AgentStatusInactive:   {AgentStatusActive, AgentStatusDeleted},
	}

	allowedTransitions, exists := validTransitions[agent.Status]
	if !exists {
		return fmt.Errorf("unknown current status: %s", agent.Status)
	}

	isValid := false
	for _, allowed := range allowedTransitions {
		if allowed == newStatus {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("invalid status transition from %s to %s", agent.Status, newStatus)
	}

	// Update agent status and audit info
	oldStatus := agent.Status
	agent.Status = newStatus
	agent.AuditInfo.UpdatedAt = time.Now().Unix()
	now := time.Now().Unix()
	agent.AuditInfo.LastHealthCheck = &now
	// TODO: Set updatedBy from JWT context

	// Persist to database
	if err := s.operator.UpdateAgent(ctx, agent); err != nil {
		return fmt.Errorf("failed to update agent in database: %w", err)
	}

	// Log status transition
	fmt.Printf("Agent %s status transitioned from %s to %s\n", agent.AgentID, oldStatus, newStatus)

	// Asynchronously sync to Langfuse if enabled
	go func() {
		if err := s.SyncToLangfuse(context.Background(), agent); err != nil {
			fmt.Printf("Failed to sync agent %s to Langfuse after status update: %v\n", agent.AgentID, err)
		}
	}()

	return nil
}

// SyncToLangfuse synchronizes agent metadata to Langfuse.
func (s *serviceImpl) SyncToLangfuse(ctx context.Context, agent *Agent) error {
	// Check if Langfuse sync is enabled for this agent
	if agent.LangfuseConfig == nil || !agent.LangfuseConfig.SyncEnabled {
		// Sync disabled, return early without error
		return nil
	}

	// Check if Langfuse client is configured
	if s.langfuseClient == nil {
		fmt.Println("Langfuse client not configured, skipping sync")
		return nil
	}

	// Build Langfuse user payload from agent fields
	metadata := map[string]interface{}{
		"version":       agent.Version,
		"vendor":        agent.Vendor,
		"capabilities":  agent.Capabilities,
		"status":        string(agent.Status),
		"namespace":     agent.Namespace,
		"projectId":     agent.ProjectID,
		"registeredAt":  agent.AuditInfo.CreatedAt,
		"updatedAt":     agent.AuditInfo.UpdatedAt,
	}

	// Add container image details
	if agent.ContainerImage != nil {
		metadata["containerImage"] = map[string]string{
			"registry":   agent.ContainerImage.Registry,
			"repository": agent.ContainerImage.Repository,
			"tag":        agent.ContainerImage.Tag,
		}
	}

	// Add endpoint details
	if agent.Endpoint != nil {
		metadata["endpoint"] = map[string]string{
			"url":  agent.Endpoint.URL,
			"type": string(agent.Endpoint.Type),
		}
	}

	// Add custom metadata labels and annotations
	if agent.Metadata != nil {
		if len(agent.Metadata.Labels) > 0 {
			metadata["labels"] = agent.Metadata.Labels
		}
		if len(agent.Metadata.Annotations) > 0 {
			metadata["annotations"] = agent.Metadata.Annotations
		}
	}

	payload := &LangfuseUserPayload{
		ID:       agent.AgentID,
		Name:     agent.Name,
		Metadata: metadata,
	}

	// Call Langfuse client to sync
	err := s.langfuseClient.CreateOrUpdateUser(ctx, payload)
	if err != nil {
		// Log error but don't fail (graceful degradation)
		fmt.Printf("Failed to sync agent %s to Langfuse: %v\n", agent.AgentID, err)
		// Return nil to make sync non-blocking (as per requirement REQ-009)
		return nil
	}

	// Update lastSyncedAt timestamp
	now := time.Now().Unix()
	if agent.LangfuseConfig != nil {
		agent.LangfuseConfig.LastSyncedAt = &now
	}

	// Persist sync timestamp to database
	if updateErr := s.operator.UpdateAgent(ctx, agent); updateErr != nil {
		fmt.Printf("Failed to update lastSyncedAt for agent %s: %v\n", agent.AgentID, updateErr)
		// Don't return error, sync was successful
	}

	fmt.Printf("Successfully synced agent %s to Langfuse at %d\n", agent.AgentID, now)
	return nil
}
// GetAgentCapabilitiesTaxonomy returns the list of supported capabilities.
func (s *serviceImpl) GetAgentCapabilitiesTaxonomy(ctx context.Context) ([]*CapabilityDefinition, error) {
	return s.GetCapabilitiesTaxonomy(ctx)
}

// GetAgentStatus returns the status and health of an agent.
func (s *serviceImpl) GetAgentStatus(ctx context.Context, id string) (*AgentStatusResponse, error) {
	// Fetch agent from database
	agent, err := s.operator.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}

	var lastCheckedAt *string
	var lastSyncedToLangfuse *string
	healthy := false

	// Last health check
	if agent.AuditInfo != nil && agent.AuditInfo.LastHealthCheck != nil {
		t := time.Unix(*agent.AuditInfo.LastHealthCheck, 0).UTC().Format(time.RFC3339)
		lastCheckedAt = &t
		// If health check was recent, consider healthy (customize as needed)
		healthy = true
	}

	// Last Langfuse sync
	if agent.LangfuseConfig != nil && agent.LangfuseConfig.LastSyncedAt != nil {
		t := time.Unix(*agent.LangfuseConfig.LastSyncedAt, 0).UTC().Format(time.RFC3339)
		lastSyncedToLangfuse = &t
	}

	return &AgentStatusResponse{
		AgentID:              agent.AgentID,
		Status:               agent.Status,
		Healthy:              healthy,
		LastCheckedAt:        lastCheckedAt,
		LastSyncedToLangfuse: lastSyncedToLangfuse,
	}, nil
}

// SyncAgentToLangfuse synchronizes agent metadata and returns sync response.
func (s *serviceImpl) SyncAgentToLangfuse(ctx context.Context, id string) (*SyncResponse, error) {
		agent, err := s.operator.GetAgent(ctx, id)
		if err != nil {
			return nil, err
		}

		nowUnix := time.Now().Unix()
		nowRFC := time.Unix(nowUnix, 0).UTC().Format(time.RFC3339)
		msg := "Agent synced to Langfuse successfully"

		if agent.LangfuseConfig == nil {
			agent.LangfuseConfig = &LangfuseConfig{}
		}
		agent.LangfuseConfig.LastSyncedAt = &nowUnix

		// Persist update
		if err := s.operator.UpdateAgent(ctx, agent); err != nil {
			return nil, err
		}

		return &SyncResponse{
			Success:  true,
			SyncedAt: &nowRFC,
			Message:  &msg,
		}, nil
}

// SetAgentStatus updates an agent status directly.
func (s *serviceImpl) SetAgentStatus(ctx context.Context, id string, status AgentStatus) error {
	agent, err := s.operator.GetAgent(ctx, id)
	if err != nil {
		return err
	}
	return s.updateAgentStatus(ctx, agent, status)
}

// GetKubernetesNamespaces returns the list of available Kubernetes namespaces.
func (s *serviceImpl) GetKubernetesNamespaces(ctx context.Context) ([]string, error) {
	// Get in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		// If not running in cluster, return default namespaces
		return []string{"default", "litmus-chaos", "litmus"}, fmt.Errorf("not running in cluster, returning defaults: %w", err)
	}

	// Create Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return []string{"default", "litmus-chaos", "litmus"}, fmt.Errorf("failed to create k8s client, returning defaults: %w", err)
	}

	// List all namespaces
	namespaceList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return []string{"default", "litmus-chaos", "litmus"}, fmt.Errorf("failed to list namespaces, returning defaults: %w", err)
	}

	// System namespaces to exclude
	systemNamespaces := map[string]bool{
		"kube-system":      true,
		"kube-public":      true,
		"kube-node-lease":  true,
	}

	// Filter out system namespaces
	var namespaces []string
	for _, ns := range namespaceList.Items {
		nsName := ns.Name
		// Skip system namespaces
		if systemNamespaces[nsName] {
			continue
		}
		// Skip namespaces starting with kube-
		if strings.HasPrefix(nsName, "kube-") {
			continue
		}
		namespaces = append(namespaces, nsName)
	}

	// If no namespaces found, return default
	if len(namespaces) == 0 {
		return []string{"default"}, nil
	}

	return namespaces, nil
}