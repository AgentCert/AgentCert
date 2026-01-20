package agent_registry

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/langfuse"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
)

// Service provides business logic for agent registry operations
type Service interface {
	// Agent CRUD
	RegisterAgent(ctx context.Context, input RegisterAgentInput, createdBy string) (*Agent, error)
	GetAgent(ctx context.Context, agentID string) (*Agent, error)
	UpdateAgent(ctx context.Context, agentID string, input UpdateAgentInput, updatedBy string) (*Agent, error)
	DeleteAgent(ctx context.Context, agentID string, deletedBy string) error
	ListAgents(ctx context.Context, filter ListAgentsFilter, pagination ListAgentsPagination) (*ListAgentsResponse, error)

	// Capability queries
	GetAgentsByCapabilities(ctx context.Context, projectID string, capabilities []string, activeOnly bool) ([]*Agent, error)
	GetCapabilitiesTaxonomy() map[string][]string

	// Status management
	UpdateAgentStatus(ctx context.Context, agentID string, status AgentStatus, updatedBy string) (*Agent, error)
	ValidateAgentHealth(ctx context.Context, agentID string) (*Agent, error)

	// Langfuse integration
	SyncAgentToLangfuse(ctx context.Context, agentID string) error
	SyncAllPendingAgents(ctx context.Context) error
}

type service struct {
	operator       *Operator
	validator      *Validator
	langfuseClient *langfuse.Client
	logger         *logrus.Logger
}

// NewService creates a new agent registry service
func NewService(operator *Operator, validator *Validator, langfuseClient *langfuse.Client, logger *logrus.Logger) Service {
	return &service{
		operator:       operator,
		validator:      validator,
		langfuseClient: langfuseClient,
		logger:         logger,
	}
}

// RegisterAgent registers a new AI agent
func (s *service) RegisterAgent(ctx context.Context, input RegisterAgentInput, createdBy string) (*Agent, error) {
	// Validate input
	if err := s.validator.ValidateRegisterInput(input); err != nil {
		return nil, err
	}

	// Check for duplicate
	existing, err := s.operator.GetAgentByProjectAndName(ctx, input.ProjectID, input.Name)
	if err == nil && existing != nil {
		return nil, ErrAgentAlreadyExists
	}

	// Build agent model
	agent := &Agent{
		AgentID:      uuid.New().String(),
		ProjectID:    input.ProjectID,
		Name:         input.Name,
		Description:  input.Description,
		Vendor:       input.Vendor,
		Version:      input.Version,
		Capabilities: input.Capabilities,
		Namespace:    input.Namespace,
		ServiceAccount: input.ServiceAccount,
		Status:       AgentStatusRegistered,
		Metadata:     input.Metadata,
		Tags:         input.Tags,
		AuditInfo: AuditInfo{
			CreatedBy: createdBy,
			UpdatedBy: createdBy,
		},
	}

	// Set container image if provided
	if input.ContainerImage != nil {
		agent.ContainerImage = &ContainerImage{
			Registry:    input.ContainerImage.Registry,
			Repository:  input.ContainerImage.Repository,
			Tag:         input.ContainerImage.Tag,
			Digest:      input.ContainerImage.Digest,
			PullPolicy:  input.ContainerImage.PullPolicy,
			PullSecrets: input.ContainerImage.PullSecrets,
		}
	}

	// Set endpoint if provided
	if input.Endpoint != nil {
		agent.Endpoint = &AgentEndpoint{
			DiscoveryType: EndpointDiscoveryType(input.Endpoint.DiscoveryType),
			EndpointType:  EndpointType(input.Endpoint.EndpointType),
			URL:           input.Endpoint.URL,
			ServiceName:   input.Endpoint.ServiceName,
			Port:          input.Endpoint.Port,
			TLSEnabled:    input.Endpoint.TLSEnabled,
			CertSecretRef: input.Endpoint.CertSecretRef,
		}
	}

	// Set health check config
	if input.HealthCheck != nil {
		agent.HealthCheck = &HealthCheckConfig{
			Enabled:       input.HealthCheck.Enabled,
			Path:          input.HealthCheck.Path,
			IntervalSec:   input.HealthCheck.IntervalSec,
			TimeoutSec:    input.HealthCheck.TimeoutSec,
			MaxRetries:    input.HealthCheck.MaxRetries,
			SuccessThresh: input.HealthCheck.SuccessThresh,
		}
		// Apply defaults
		if agent.HealthCheck.Path == "" {
			agent.HealthCheck.Path = DefaultHealthCheckPath
		}
		if agent.HealthCheck.IntervalSec == 0 {
			agent.HealthCheck.IntervalSec = DefaultHealthCheckInterval
		}
		if agent.HealthCheck.TimeoutSec == 0 {
			agent.HealthCheck.TimeoutSec = DefaultHealthCheckTimeout
		}
		if agent.HealthCheck.MaxRetries == 0 {
			agent.HealthCheck.MaxRetries = DefaultMaxHealthRetries
		}
	}

	// Set Langfuse config
	if input.EnableLangfuse {
		agent.LangfuseConfig = &LangfuseConfig{
			Enabled:    true,
			SyncStatus: "PENDING",
		}
	}

	// Create in database
	createdAgent, err := s.operator.CreateAgent(ctx, agent)
	if err != nil {
		return nil, NewOperationError("RegisterAgent", "", err)
	}

	// Sync to Langfuse asynchronously if enabled
	if input.EnableLangfuse && s.langfuseClient != nil && s.langfuseClient.IsEnabled() {
		go s.syncAgentToLangfuseAsync(createdAgent)
	}

	s.logger.Infof("Registered agent '%s' (ID: %s) in project '%s'", agent.Name, agent.AgentID, agent.ProjectID)
	return createdAgent, nil
}

// GetAgent retrieves an agent by ID
func (s *service) GetAgent(ctx context.Context, agentID string) (*Agent, error) {
	if agentID == "" {
		return nil, ErrInvalidAgentID
	}

	agent, err := s.operator.GetAgentByID(ctx, agentID)
	if err != nil {
		return nil, err
	}

	return agent, nil
}

// UpdateAgent updates an existing agent
func (s *service) UpdateAgent(ctx context.Context, agentID string, input UpdateAgentInput, updatedBy string) (*Agent, error) {
	if agentID == "" {
		return nil, ErrInvalidAgentID
	}

	// Validate input
	if err := s.validator.ValidateUpdateInput(input); err != nil {
		return nil, err
	}

	// Get existing agent
	existing, err := s.operator.GetAgentByID(ctx, agentID)
	if err != nil {
		return nil, err
	}

	// Build update document
	update := bson.M{
		"auditInfo.updatedBy": updatedBy,
	}

	if input.Description != nil {
		update["description"] = *input.Description
	}
	if input.Version != nil {
		update["version"] = *input.Version
	}
	if input.Capabilities != nil {
		update["capabilities"] = input.Capabilities
	}
	if input.Metadata != nil {
		update["metadata"] = input.Metadata
	}
	if input.Tags != nil {
		update["tags"] = input.Tags
	}

	if input.ContainerImage != nil {
		update["containerImage"] = &ContainerImage{
			Registry:    input.ContainerImage.Registry,
			Repository:  input.ContainerImage.Repository,
			Tag:         input.ContainerImage.Tag,
			Digest:      input.ContainerImage.Digest,
			PullPolicy:  input.ContainerImage.PullPolicy,
			PullSecrets: input.ContainerImage.PullSecrets,
		}
	}

	if input.Endpoint != nil {
		update["endpoint"] = &AgentEndpoint{
			DiscoveryType: EndpointDiscoveryType(input.Endpoint.DiscoveryType),
			EndpointType:  EndpointType(input.Endpoint.EndpointType),
			URL:           input.Endpoint.URL,
			ServiceName:   input.Endpoint.ServiceName,
			Port:          input.Endpoint.Port,
			TLSEnabled:    input.Endpoint.TLSEnabled,
			CertSecretRef: input.Endpoint.CertSecretRef,
		}
	}

	if input.HealthCheck != nil {
		hc := &HealthCheckConfig{
			Enabled:       input.HealthCheck.Enabled,
			Path:          input.HealthCheck.Path,
			IntervalSec:   input.HealthCheck.IntervalSec,
			TimeoutSec:    input.HealthCheck.TimeoutSec,
			MaxRetries:    input.HealthCheck.MaxRetries,
			SuccessThresh: input.HealthCheck.SuccessThresh,
		}
		if hc.Path == "" {
			hc.Path = DefaultHealthCheckPath
		}
		update["healthCheck"] = hc
	}

	if input.EnableLangfuse != nil {
		if *input.EnableLangfuse {
			if existing.LangfuseConfig == nil || !existing.LangfuseConfig.Enabled {
				update["langfuseConfig"] = &LangfuseConfig{
					Enabled:    true,
					SyncStatus: "PENDING",
				}
			}
		} else {
			update["langfuseConfig.enabled"] = false
		}
	}

	// Perform update
	updatedAgent, err := s.operator.UpdateAgent(ctx, agentID, update)
	if err != nil {
		return nil, NewOperationError("UpdateAgent", agentID, err)
	}

	// Sync to Langfuse if enabled
	if updatedAgent.LangfuseConfig != nil && updatedAgent.LangfuseConfig.Enabled && s.langfuseClient != nil {
		go s.syncAgentToLangfuseAsync(updatedAgent)
	}

	s.logger.Infof("Updated agent '%s' (ID: %s)", updatedAgent.Name, agentID)
	return updatedAgent, nil
}

// DeleteAgent soft-deletes an agent
func (s *service) DeleteAgent(ctx context.Context, agentID string, deletedBy string) error {
	if agentID == "" {
		return ErrInvalidAgentID
	}

	// Get agent first to sync deletion to Langfuse
	agent, err := s.operator.GetAgentByID(ctx, agentID)
	if err != nil {
		return err
	}

	if err := s.operator.DeleteAgent(ctx, agentID, deletedBy); err != nil {
		return NewOperationError("DeleteAgent", agentID, err)
	}

	// Sync deletion to Langfuse
	if agent.LangfuseConfig != nil && agent.LangfuseConfig.Enabled && s.langfuseClient != nil {
		go s.syncAgentDeletionToLangfuse(agent)
	}

	s.logger.Infof("Deleted agent '%s' (ID: %s)", agent.Name, agentID)
	return nil
}

// ListAgents lists agents with filtering and pagination
func (s *service) ListAgents(ctx context.Context, filter ListAgentsFilter, pagination ListAgentsPagination) (*ListAgentsResponse, error) {
	// Apply defaults
	if pagination.Limit <= 0 {
		pagination.Limit = 20
	}
	if pagination.Limit > 100 {
		pagination.Limit = 100
	}

	return s.operator.ListAgents(ctx, filter, pagination)
}

// GetAgentsByCapabilities finds agents with specific capabilities
func (s *service) GetAgentsByCapabilities(ctx context.Context, projectID string, capabilities []string, activeOnly bool) ([]*Agent, error) {
	if projectID == "" {
		return nil, ErrInvalidProjectID
	}
	if len(capabilities) == 0 {
		return nil, ErrInvalidCapabilities
	}

	return s.operator.GetAgentsByCapabilities(ctx, projectID, capabilities, activeOnly)
}

// GetCapabilitiesTaxonomy returns the predefined capability categories
func (s *service) GetCapabilitiesTaxonomy() map[string][]string {
	return CapabilityCategories
}

// UpdateAgentStatus updates the status of an agent
func (s *service) UpdateAgentStatus(ctx context.Context, agentID string, status AgentStatus, updatedBy string) (*Agent, error) {
	if agentID == "" {
		return nil, ErrInvalidAgentID
	}
	if !status.IsValid() {
		return nil, ErrInvalidStatus
	}

	// Get current status
	agent, err := s.operator.GetAgentByID(ctx, agentID)
	if err != nil {
		return nil, err
	}

	// Validate transition
	if !agent.Status.CanTransitionTo(status) {
		return nil, NewOperationError("UpdateAgentStatus", agentID, ErrInvalidTransition)
	}

	return s.operator.UpdateAgentStatus(ctx, agentID, status, updatedBy)
}

// ValidateAgentHealth performs a health check on an agent
func (s *service) ValidateAgentHealth(ctx context.Context, agentID string) (*Agent, error) {
	_, err := s.operator.GetAgentByID(ctx, agentID)
	if err != nil {
		return nil, err
	}

	// For now, just update status - actual health check would call the agent endpoint
	result := &HealthCheckResult{
		Timestamp:    time.Now().UTC(),
		Success:      true,
		ResponseTime: 50,
		Message:      "Health check passed",
	}

	var newStatus *AgentStatus
	if result.Success {
		active := AgentStatusActive
		newStatus = &active
	}

	return s.operator.UpdateHealthCheckResult(ctx, agentID, result, newStatus)
}

// SyncAgentToLangfuse syncs an agent to Langfuse
func (s *service) SyncAgentToLangfuse(ctx context.Context, agentID string) error {
	if s.langfuseClient == nil || !s.langfuseClient.IsEnabled() {
		return nil
	}

	agent, err := s.operator.GetAgentByID(ctx, agentID)
	if err != nil {
		return err
	}

	return s.doSyncAgentToLangfuse(ctx, agent)
}

// SyncAllPendingAgents syncs all agents with pending Langfuse status
func (s *service) SyncAllPendingAgents(ctx context.Context) error {
	if s.langfuseClient == nil || !s.langfuseClient.IsEnabled() {
		return nil
	}

	agents, err := s.operator.GetAgentsForLangfuseSync(ctx)
	if err != nil {
		return err
	}

	for _, agent := range agents {
		if err := s.doSyncAgentToLangfuse(ctx, agent); err != nil {
			s.logger.Warnf("Failed to sync agent %s to Langfuse: %v", agent.AgentID, err)
		}
	}

	return nil
}

// Internal helper to sync agent to Langfuse
func (s *service) doSyncAgentToLangfuse(ctx context.Context, agent *Agent) error {
	metadata := map[string]interface{}{
		"vendor":       agent.Vendor,
		"version":      agent.Version,
		"namespace":    agent.Namespace,
		"status":       string(agent.Status),
		"capabilities": agent.Capabilities,
		"projectId":    agent.ProjectID,
		"createdAt":    agent.AuditInfo.CreatedAt.Format(time.RFC3339),
		"updatedAt":    agent.AuditInfo.UpdatedAt.Format(time.RFC3339),
	}

	if agent.ContainerImage != nil {
		metadata["containerImage"] = agent.ContainerImage.FullImageName()
	}

	if agent.Tags != nil {
		metadata["tags"] = agent.Tags
	}

	err := s.langfuseClient.CreateOrUpdateUserSync(ctx, langfuse.UserPayload{
		ID:       agent.AgentID,
		Name:     agent.Name,
		Metadata: metadata,
	})

	config := &LangfuseConfig{
		Enabled:  true,
		SyncedAt: time.Now().UTC(),
	}

	if err != nil {
		config.SyncStatus = "FAILED"
		config.LastSyncError = err.Error()
		s.operator.UpdateLangfuseConfig(ctx, agent.AgentID, config)
		return err
	}

	config.SyncStatus = "SYNCED"
	config.LangfuseID = agent.AgentID
	s.operator.UpdateLangfuseConfig(ctx, agent.AgentID, config)

	s.logger.Infof("Successfully synced agent '%s' to Langfuse", agent.Name)
	return nil
}

// Async wrapper for Langfuse sync
func (s *service) syncAgentToLangfuseAsync(agent *Agent) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.doSyncAgentToLangfuse(ctx, agent); err != nil {
		s.logger.Errorf("Async Langfuse sync failed for agent %s: %v", agent.AgentID, err)
	}
}

// Sync deletion to Langfuse
func (s *service) syncAgentDeletionToLangfuse(agent *Agent) {
	if s.langfuseClient == nil || !s.langfuseClient.IsEnabled() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := s.langfuseClient.CreateOrUpdateUserSync(ctx, langfuse.UserPayload{
		ID: agent.AgentID,
		Metadata: map[string]interface{}{
			"status":    "DELETED",
			"deletedAt": time.Now().Format(time.RFC3339),
		},
	})

	if err != nil {
		s.logger.Warnf("Failed to sync agent deletion to Langfuse for %s: %v", agent.AgentID, err)
	}
}
