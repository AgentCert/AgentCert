package agent_registry

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/authorization"
	"github.com/sirupsen/logrus"
)

// Handler provides GraphQL resolver handlers for Agent Registry operations.
// It transforms GraphQL requests to service layer calls and handles response formatting.
type Handler struct {
	service Service
}

// NewHandler creates a new Handler instance with the provided Service.
func NewHandler(service Service) *Handler {
	return &Handler{
		service: service,
	}
}

// RegisterAgent handles the GraphQL mutation for registering a new agent.
// It validates user permissions, transforms the input, calls the service layer,
// and returns the appropriate response with sync status.
func (h *Handler) RegisterAgent(ctx context.Context, input model.RegisterAgentInput) (*model.RegisterAgentResponse, error) {
	logFields := logrus.Fields{
		"projectId": input.ProjectID,
		"agentName": input.Name,
	}
	logrus.WithFields(logFields).Info("request received to register agent")

	// Extract JWT token from context
	jwt, ok := ctx.Value(authorization.AuthKey).(string)
	if !ok || jwt == "" {
		logrus.Error("authentication token not found in context")
		return nil, ErrUnauthorized
	}

	// Validate user has PROJECT_OWNER or PROJECT_ADMIN role
	err := authorization.ValidateRole(ctx, input.ProjectID,
		[]string{"Owner", "Editor"}, // PROJECT_OWNER and PROJECT_ADMIN equivalent roles
		model.InvitationAccepted.String())
	if err != nil {
		logrus.WithFields(logFields).WithError(err).Error("authorization failed for register agent")
		return nil, ErrInsufficientPermissions
	}

	// Transform GraphQL input to service request
	request, err := MapRegisterAgentInputToRequest(input)
	if err != nil {
		logrus.WithFields(logFields).WithError(err).Error("failed to map input to request")
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Call service layer to register agent
	agent, err := h.service.RegisterAgent(ctx, request)
	if err != nil {
		logrus.WithFields(logFields).WithError(err).Error("failed to register agent")
		// Map service errors to appropriate GraphQL errors
		if errors.Is(err, ErrDuplicateAgentName) {
			return nil, fmt.Errorf("agent name '%s' already exists in project", input.Name)
		}
		if errors.Is(err, ErrInvalidAgentName) || errors.Is(err, ErrInvalidVersion) || errors.Is(err, ErrInvalidCapabilities) {
			return nil, fmt.Errorf("validation error: %w", err)
		}
		return nil, fmt.Errorf("failed to register agent: %w", err)
	}

	// Attempt Langfuse sync (non-blocking, best effort)
	syncStatus := model.LangfuseSyncStatusSkipped
	
	if agent.LangfuseConfig != nil && agent.LangfuseConfig.SyncEnabled {
		syncErr := h.service.SyncToLangfuse(ctx, agent)
		if syncErr != nil {
			logrus.WithFields(logFields).WithError(syncErr).Warn("langfuse sync failed")
			syncStatus = model.LangfuseSyncStatusFailed
		} else {
			syncStatus = model.LangfuseSyncStatusSuccess
		}
	}

	// Transform internal agent to GraphQL model
	agentModel := MapAgentToModel(agent)

	logrus.WithFields(logFields).WithField("agentId", agent.AgentID).Info("agent registered successfully")

	return &model.RegisterAgentResponse{
		Agent:              agentModel,
		LangfuseSyncStatus: syncStatus,
	}, nil
}

// GetAgent handles the GraphQL query for retrieving an agent by ID.
// It validates user access and returns the agent if found.
func (h *Handler) GetAgent(ctx context.Context, id string) (*model.Agent, error) {
	logFields := logrus.Fields{
		"agentId": id,
	}
	logrus.WithFields(logFields).Info("request received to get agent")

	// Extract JWT token from context for authorization
	jwt, ok := ctx.Value(authorization.AuthKey).(string)
	if !ok || jwt == "" {
		logrus.Error("authentication token not found in context")
		return nil, ErrUnauthorized
	}

	// Call service layer to get agent
	agent, err := h.service.GetAgent(ctx, id)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			logrus.WithFields(logFields).Warn("agent not found")
			return nil, fmt.Errorf("agent with ID '%s' not found", id)
		}
		if errors.Is(err, ErrInsufficientPermissions) {
			logrus.WithFields(logFields).Error("authorization failed for get agent")
			return nil, ErrInsufficientPermissions
		}
		logrus.WithFields(logFields).WithError(err).Error("failed to get agent")
		return nil, fmt.Errorf("failed to retrieve agent: %w", err)
	}

	// Validate user has access to the agent's project
	// This is done implicitly in the service layer through JWT claims
	// Additional validation can be added here if needed
	err = authorization.ValidateRole(ctx, agent.ProjectID,
		[]string{"Owner", "Editor", "Viewer"}, // All roles can view
		model.InvitationAccepted.String())
	if err != nil {
		logrus.WithFields(logFields).WithError(err).Error("authorization failed for get agent")
		return nil, ErrInsufficientPermissions
	}

	// Transform internal agent to GraphQL model
	agentModel := MapAgentToModel(agent)

	logrus.WithFields(logFields).Info("agent retrieved successfully")
	return agentModel, nil
}

// ListAgents handles the GraphQL query for retrieving a paginated list of agents.
// It validates user access, applies filters, and returns agents with pagination metadata.
func (h *Handler) ListAgents(ctx context.Context, filter *model.ListAgentsFilter, pagination model.PaginationInput) (*model.AgentListResponse, error) {
	logFields := logrus.Fields{
		"page":  pagination.Page,
		"limit": pagination.Limit,
	}
	if filter != nil && filter.ProjectID != nil {
		logFields["projectId"] = *filter.ProjectID
	}
	logrus.WithFields(logFields).Info("request received to list agents")

	// Extract JWT token from context for authorization
	jwt, ok := ctx.Value(authorization.AuthKey).(string)
	if !ok || jwt == "" {
		logrus.Error("authentication token not found in context")
		return nil, ErrUnauthorized
	}

	// Transform GraphQL filter to internal filter
	internalFilter := MapAgentFilterInputToFilter(filter)
	
	// Transform GraphQL pagination to internal pagination
	internalPagination := MapPaginationInputToPagination(pagination)

	// Validate user has access to the project if projectId is specified
	if internalFilter.ProjectID != "" {
		err := authorization.ValidateRole(ctx, internalFilter.ProjectID,
			[]string{"Owner", "Editor", "Viewer"}, // All roles can view
			model.InvitationAccepted.String())
		if err != nil {
			logrus.WithFields(logFields).WithError(err).Error("authorization failed for list agents")
			return nil, ErrInsufficientPermissions
		}
	}

	// Call service layer to list agents
	response, err := h.service.ListAgents(ctx, internalFilter, internalPagination)
	if err != nil {
		logrus.WithFields(logFields).WithError(err).Error("failed to list agents")
		return nil, fmt.Errorf("failed to retrieve agents: %w", err)
	}

	// Transform internal response to GraphQL model
	gqlResponse := MapAgentListResponseToModel(response)

	// Calculate pagination fields
	if pagination.Limit > 0 {
		// totalPages = ceil(totalCount / limit)
		gqlResponse.TotalPages = (gqlResponse.TotalCount + pagination.Limit - 1) / pagination.Limit
		
		// hasNextPage = currentPage < totalPages
		gqlResponse.HasNextPage = gqlResponse.CurrentPage < gqlResponse.TotalPages
	} else {
		gqlResponse.TotalPages = 1
		gqlResponse.HasNextPage = false
	}

	logrus.WithFields(logFields).WithField("totalCount", gqlResponse.TotalCount).Info("agents listed successfully")
	return gqlResponse, nil
}

// UpdateAgent handles the GraphQL mutation for updating an agent
func (h *Handler) UpdateAgent(ctx context.Context, id string, input *model.UpdateAgentInput) (*model.Agent, error) {
	// Extract JWT token from context
	token, ok := ctx.Value(authorization.AuthKey).(string)
	if !ok || token == "" {
		logrus.Error("failed to extract JWT token from context")
		return nil, ErrUnauthorized
	}

	logFields := logrus.Fields{
		"agentId": id,
	}

	// Get existing agent to verify it exists and get projectId for authorization
	existingAgent, err := h.service.GetAgent(ctx, id)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			logrus.WithFields(logFields).Warn("agent not found for update")
			return nil, fmt.Errorf("agent with ID '%s' not found", id)
		}
		logrus.WithFields(logFields).WithError(err).Error("failed to get agent for update")
		return nil, err
	}

	logFields["projectId"] = existingAgent.ProjectID

	// Verify user has authorization to update (Owner or Editor)
	if err := authorization.ValidateRole(ctx, existingAgent.ProjectID, []string{"Owner", "Editor"}, ""); err != nil {
		logrus.WithFields(logFields).WithError(err).Warn("insufficient permissions to update agent")
		return nil, ErrInsufficientPermissions
	}

	// Transform GraphQL input to internal request type
	updateReq, err := MapUpdateAgentInputToRequest(*input)
	if err != nil {
		logrus.WithFields(logFields).WithError(err).Error("failed to transform update input")
		return nil, err
	}

	// Call service layer to update agent
	updatedAgent, err := h.service.UpdateAgent(ctx, id, updateReq)
	if err != nil {
		if errors.Is(err, ErrInvalidAgentName) {
			logrus.WithFields(logFields).WithError(err).Warn("invalid agent name provided for update")
			return nil, err
		}
		if errors.Is(err, ErrInvalidVersion) {
			logrus.WithFields(logFields).WithError(err).Warn("invalid version provided for update")
			return nil, err
		}
		if errors.Is(err, ErrInvalidCapabilities) {
			logrus.WithFields(logFields).WithError(err).Warn("invalid capabilities provided for update")
			return nil, err
		}
		if errors.Is(err, ErrInvalidContainerImage) {
			logrus.WithFields(logFields).WithError(err).Warn("invalid container image provided for update")
			return nil, err
		}
		logrus.WithFields(logFields).WithError(err).Error("failed to update agent")
		return nil, err
	}

	// Transform internal agent to GraphQL model
	gqlAgent := MapAgentToModel(updatedAgent)

	logrus.WithFields(logFields).Info("agent updated successfully")
	return gqlAgent, nil
}

// DeleteAgent handles the GraphQL mutation for deleting an agent
func (h *Handler) DeleteAgent(ctx context.Context, id string, hardDelete *bool) (*model.DeleteAgentResponse, error) {
	// Extract JWT token from context
	token, ok := ctx.Value(authorization.AuthKey).(string)
	if !ok || token == "" {
		logrus.Error("failed to extract JWT token from context")
		return nil, ErrUnauthorized
	}

	logFields := logrus.Fields{
		"agentId": id,
	}

	// Get existing agent to verify it exists and get projectId for authorization
	existingAgent, err := h.service.GetAgent(ctx, id)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			logrus.WithFields(logFields).Warn("agent not found for deletion")
			return nil, fmt.Errorf("agent with ID '%s' not found", id)
		}
		logrus.WithFields(logFields).WithError(err).Error("failed to get agent for deletion")
		return nil, err
	}

	logFields["projectId"] = existingAgent.ProjectID

	// Verify user has authorization to delete (Owner or Editor)
	if err := authorization.ValidateRole(ctx, existingAgent.ProjectID, []string{"Owner", "Editor"}, ""); err != nil {
		logrus.WithFields(logFields).WithError(err).Warn("insufficient permissions to delete agent")
		return nil, ErrInsufficientPermissions
	}

	// Determine hard delete flag (default to false for soft delete)
	isHardDelete := false
	if hardDelete != nil {
		isHardDelete = *hardDelete
	}

	logFields["hardDelete"] = isHardDelete

	// Call service layer to delete agent
	deleteResp, err := h.service.DeleteAgent(ctx, id, isHardDelete)
	if err != nil {
		logrus.WithFields(logFields).WithError(err).Error("failed to delete agent")
		return nil, err
	}

	// Transform internal response to GraphQL model
	gqlResponse := MapDeleteAgentResponseToModel(deleteResp)

	logrus.WithFields(logFields).Info("agent deleted successfully")
	return gqlResponse, nil
}

// GetAgentsByCapabilities handles the GraphQL query for retrieving agents by capabilities
func (h *Handler) GetAgentsByCapabilities(ctx context.Context, projectID string, capabilities []string) ([]*model.Agent, error) {
	// Extract JWT token from context
	token, ok := ctx.Value(authorization.AuthKey).(string)
	if !ok || token == "" {
		logrus.Error("failed to extract JWT token from context")
		return nil, ErrUnauthorized
	}

	logFields := logrus.Fields{
		"projectId":    projectID,
		"capabilities": capabilities,
	}

	// Verify user has access to the project (at least Viewer role)
	if err := authorization.ValidateRole(ctx, projectID, []string{"Owner", "Editor", "Viewer"}, ""); err != nil {
		logrus.WithFields(logFields).WithError(err).Warn("insufficient permissions to query agents by capabilities")
		return nil, ErrInsufficientPermissions
	}

	// Validate capabilities array is not empty
	if len(capabilities) == 0 {
		logrus.WithFields(logFields).Warn("capabilities array is empty")
		return nil, fmt.Errorf("capabilities array cannot be empty")
	}

	// Call service layer to get agents by capabilities
	agents, err := h.service.GetAgentsByCapabilities(ctx, projectID, capabilities)
	if err != nil {
		logrus.WithFields(logFields).WithError(err).Error("failed to get agents by capabilities")
		return nil, err
	}

	// Transform internal agents to GraphQL models
	gqlAgents := make([]*model.Agent, 0, len(agents))
	for _, agent := range agents {
		gqlAgent := MapAgentToModel(agent)
		gqlAgents = append(gqlAgents, gqlAgent)
	}

	logrus.WithFields(logFields).WithField("resultCount", len(gqlAgents)).Info("agents retrieved by capabilities successfully")
	return gqlAgents, nil
}

// GetAgentStatus handles the GraphQL query for retrieving agent status with health check
func (h *Handler) GetAgentStatus(ctx context.Context, id string) (*model.AgentStatusResponse, error) {
	// Extract JWT token from context
	token, ok := ctx.Value(authorization.AuthKey).(string)
	if !ok || token == "" {
		logrus.Error("failed to extract JWT token from context")
		return nil, ErrUnauthorized
	}

	logFields := logrus.Fields{
		"agentId": id,
	}

	// Get agent to verify access and existence
	agent, err := h.service.GetAgent(ctx, id)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			logrus.WithFields(logFields).Warn("agent not found for status check")
			return nil, fmt.Errorf("agent with ID '%s' not found", id)
		}
		logrus.WithFields(logFields).WithError(err).Error("failed to get agent for status check")
		return nil, err
	}

	logFields["projectId"] = agent.ProjectID

	// Validate agent health to get current status
	healthResult, err := h.service.ValidateAgentHealth(ctx, id)
	if err != nil {
		logrus.WithFields(logFields).WithError(err).Error("failed to validate agent health")
		return nil, err
	}

	// Build AgentStatusResponse
	response := &model.AgentStatusResponse{
		AgentID: agent.AgentID,
		Status:  model.AgentStatus(agent.Status),
		Healthy: healthResult.Healthy,
	}

	// Add lastCheckedAt timestamp if available
	if healthResult.CheckedAt > 0 {
		checkedAtStr := strconv.FormatInt(healthResult.CheckedAt, 10)
		response.LastCheckedAt = &checkedAtStr
	}

	// Add lastSyncedToLangfuse timestamp if available
	if agent.LangfuseConfig != nil && agent.LangfuseConfig.LastSyncedAt != nil {
		syncedAtStr := strconv.FormatInt(*agent.LangfuseConfig.LastSyncedAt, 10)
		response.LastSyncedToLangfuse = &syncedAtStr
	}

	logrus.WithFields(logFields).WithFields(logrus.Fields{
		"healthy": healthResult.Healthy,
		"status":  agent.Status,
	}).Info("agent status retrieved successfully")

	return response, nil
}

// ValidateAgentHealth handles the GraphQL mutation for validating agent health
func (h *Handler) ValidateAgentHealth(ctx context.Context, id string) (*model.HealthCheckResult, error) {
	// Extract JWT token from context
	token, ok := ctx.Value(authorization.AuthKey).(string)
	if !ok || token == "" {
		logrus.Error("failed to extract JWT token from context")
		return nil, ErrUnauthorized
	}

	logFields := logrus.Fields{
		"agentId": id,
	}

	// Call service to validate agent health
	healthResult, err := h.service.ValidateAgentHealth(ctx, id)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			logrus.WithFields(logFields).Warn("agent not found for health validation")
			return nil, fmt.Errorf("agent with ID '%s' not found", id)
		}
		logrus.WithFields(logFields).WithError(err).Error("failed to validate agent health")
		return nil, err
	}

	// Transform internal HealthCheckResult to GraphQL model
	gqlResult := MapHealthCheckResultToModel(healthResult)

	logrus.WithFields(logFields).WithField("healthy", healthResult.Healthy).Info("agent health validated successfully")
	return gqlResult, nil
}

// SyncAgentToLangfuse handles the GraphQL mutation for synchronizing an agent to Langfuse
func (h *Handler) SyncAgentToLangfuse(ctx context.Context, id string) (*model.SyncResponse, error) {
	// Extract JWT token from context
	token, ok := ctx.Value(authorization.AuthKey).(string)
	if !ok || token == "" {
		logrus.Error("failed to extract JWT token from context")
		return nil, ErrUnauthorized
	}

	logFields := logrus.Fields{
		"agentId": id,
	}

	// Get agent to verify it exists and get projectId for authorization
	agent, err := h.service.GetAgent(ctx, id)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			logrus.WithFields(logFields).Warn("agent not found for Langfuse sync")
			return nil, fmt.Errorf("agent with ID '%s' not found", id)
		}
		logrus.WithFields(logFields).WithError(err).Error("failed to get agent for Langfuse sync")
		return nil, err
	}

	logFields["projectId"] = agent.ProjectID

	// Verify user has authorization to sync (Owner or Editor)
	if err := authorization.ValidateRole(ctx, agent.ProjectID, []string{"Owner", "Editor"}, ""); err != nil {
		logrus.WithFields(logFields).WithError(err).Warn("insufficient permissions to sync agent to Langfuse")
		return nil, ErrInsufficientPermissions
	}

	// Call service to sync agent to Langfuse
	err = h.service.SyncToLangfuse(ctx, agent)
	
	// Build response
	response := &model.SyncResponse{
		Success: err == nil,
	}

	if err != nil {
		logrus.WithFields(logFields).WithError(err).Warn("failed to sync agent to Langfuse")
		errMsg := err.Error()
		response.Message = &errMsg
	} else {
		// Add sync timestamp if successful
		if agent.LangfuseConfig != nil && agent.LangfuseConfig.LastSyncedAt != nil {
			syncedAtStr := strconv.FormatInt(*agent.LangfuseConfig.LastSyncedAt, 10)
			response.SyncedAt = &syncedAtStr
		} else {
			// Use current time if lastSyncedAt not set
			now := strconv.FormatInt(time.Now().Unix(), 10)
			response.SyncedAt = &now
		}
		successMsg := "agent synced to Langfuse successfully"
		response.Message = &successMsg
		logrus.WithFields(logFields).Info("agent synced to Langfuse successfully")
	}

	return response, nil
}

// GetAgentCapabilitiesTaxonomy handles the GraphQL query for retrieving capabilities taxonomy
func (h *Handler) GetAgentCapabilitiesTaxonomy(ctx context.Context) ([]*model.CapabilityDefinition, error) {
	logrus.Debug("retrieving agent capabilities taxonomy")

	// Call service to get capabilities taxonomy
	capabilities, err := h.service.GetCapabilitiesTaxonomy(ctx)
	if err != nil {
		logrus.WithError(err).Error("failed to get capabilities taxonomy")
		return nil, err
	}

	// Transform internal capabilities to GraphQL model
	gqlCapabilities := make([]*model.CapabilityDefinition, 0, len(capabilities))
	for _, cap := range capabilities {
		gqlCap := &model.CapabilityDefinition{
			ID:          cap.ID,
			Name:        cap.Name,
			Description: cap.Description,
			Category:    cap.Category,
		}
		gqlCapabilities = append(gqlCapabilities, gqlCap)
	}

	logrus.WithField("count", len(gqlCapabilities)).Info("capabilities taxonomy retrieved successfully")
	return gqlCapabilities, nil
}
