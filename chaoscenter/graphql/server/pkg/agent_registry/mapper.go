package agent_registry

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
)

// MapRegisterAgentInputToRequest converts GraphQL input to service request.
func MapRegisterAgentInputToRequest(input model.RegisterAgentInput) (*RegisterAgentRequest, error) {
	req := &RegisterAgentRequest{
		ProjectID:    input.ProjectID,
		Name:         input.Name,
		Version:      input.Version,
		Vendor:       input.Vendor,
		Capabilities: input.Capabilities,
		Namespace:    input.Namespace,
	}

	// Map ContainerImage
	if input.ContainerImage != nil {
		req.ContainerImage = &ContainerImage{
			Registry:   input.ContainerImage.Registry,
			Repository: input.ContainerImage.Repository,
			Tag:        input.ContainerImage.Tag,
		}
	}

	// Map Endpoint
	if input.Endpoint != nil {
		req.Endpoint = &AgentEndpoint{
			URL:           input.Endpoint.URL,
			Type:          EndpointType(input.Endpoint.Type),
			DiscoveryType: EndpointDiscoveryType(input.Endpoint.DiscoveryType),
			HealthPath:    input.Endpoint.HealthPath,
			ReadyPath:     input.Endpoint.ReadyPath,
		}
	}

	// Map LangfuseConfig
	if input.LangfuseConfig != nil {
		req.LangfuseConfig = &LangfuseConfig{
			ProjectID:   input.LangfuseConfig.ProjectID,
			SyncEnabled: input.LangfuseConfig.SyncEnabled,
		}
	}

	// Map Metadata
	if input.Metadata != nil {
		metadata := &AgentMetadata{}

		// Parse labels from JSON string
		if input.Metadata.Labels != nil {
			labels := make(map[string]string)
			if err := json.Unmarshal([]byte(*input.Metadata.Labels), &labels); err != nil {
				return nil, fmt.Errorf("failed to parse labels: %w", err)
			}
			metadata.Labels = labels
		}

		// Parse annotations from JSON string
		if input.Metadata.Annotations != nil {
			annotations := make(map[string]string)
			if err := json.Unmarshal([]byte(*input.Metadata.Annotations), &annotations); err != nil {
				return nil, fmt.Errorf("failed to parse annotations: %w", err)
			}
			metadata.Annotations = annotations
		}

		req.Metadata = metadata
	}

	return req, nil
}

// MapUpdateAgentInputToRequest converts GraphQL update input to service request.
func MapUpdateAgentInputToRequest(input model.UpdateAgentInput) (*UpdateAgentRequest, error) {
	req := &UpdateAgentRequest{
		Name:         input.Name,
		Version:      input.Version,
		Vendor:       input.Vendor,
		Capabilities: input.Capabilities,
	}

	// Map ContainerImage
	if input.ContainerImage != nil {
		req.ContainerImage = &ContainerImage{
			Registry:   input.ContainerImage.Registry,
			Repository: input.ContainerImage.Repository,
			Tag:        input.ContainerImage.Tag,
		}
	}

	// Map Endpoint
	if input.Endpoint != nil {
		req.Endpoint = &AgentEndpoint{
			URL:           input.Endpoint.URL,
			Type:          EndpointType(input.Endpoint.Type),
			DiscoveryType: EndpointDiscoveryType(input.Endpoint.DiscoveryType),
			HealthPath:    input.Endpoint.HealthPath,
			ReadyPath:     input.Endpoint.ReadyPath,
		}
	}

	// Map LangfuseConfig
	if input.LangfuseConfig != nil {
		req.LangfuseConfig = &LangfuseConfig{
			ProjectID:   input.LangfuseConfig.ProjectID,
			SyncEnabled: input.LangfuseConfig.SyncEnabled,
		}
	}

	// Map Metadata
	if input.Metadata != nil {
		metadata := &AgentMetadata{}

		// Parse labels from JSON string
		if input.Metadata.Labels != nil {
			labels := make(map[string]string)
			if err := json.Unmarshal([]byte(*input.Metadata.Labels), &labels); err != nil {
				return nil, fmt.Errorf("failed to parse labels: %w", err)
			}
			metadata.Labels = labels
		}

		// Parse annotations from JSON string
		if input.Metadata.Annotations != nil {
			annotations := make(map[string]string)
			if err := json.Unmarshal([]byte(*input.Metadata.Annotations), &annotations); err != nil {
				return nil, fmt.Errorf("failed to parse annotations: %w", err)
			}
			metadata.Annotations = annotations
		}

		req.Metadata = metadata
	}

	return req, nil
}

// MapAgentFilterInputToFilter converts GraphQL filter input to service filter.
func MapAgentFilterInputToFilter(input model.AgentFilterInput) *AgentFilter {
	filter := &AgentFilter{
		ProjectID:    input.ProjectID,
		Capabilities: input.Capabilities,
		SearchTerm:   input.SearchTerm,
	}

	// Map Status
	if input.Status != nil {
		status := AgentStatus(*input.Status)
		filter.Status = &status
	}

	// Map Statuses
	if len(input.Statuses) > 0 {
		statuses := make([]AgentStatus, len(input.Statuses))
		for i, s := range input.Statuses {
			statuses[i] = AgentStatus(s)
		}
		filter.Statuses = statuses
	}

	return filter
}

// MapPaginationInputToPagination converts GraphQL pagination input to service pagination.
func MapPaginationInputToPagination(input model.PaginationInput) *PaginationInput {
	return &PaginationInput{
		Page:  input.Page,
		Limit: input.Limit,
	}
}

// MapAgentToModel converts service Agent to GraphQL model.
func MapAgentToModel(agent *Agent) *model.Agent {
	gqlAgent := &model.Agent{
		AgentID:      agent.AgentID,
		ProjectID:    agent.ProjectID,
		Name:         agent.Name,
		Version:      agent.Version,
		Vendor:       agent.Vendor,
		Capabilities: agent.Capabilities,
		Namespace:    agent.Namespace,
		Status:       model.AgentStatus(agent.Status),
		CreatedAt:    strconv.FormatInt(agent.AuditInfo.CreatedAt, 10),
		UpdatedAt:    strconv.FormatInt(agent.AuditInfo.UpdatedAt, 10),
	}

	// Map ContainerImage
	if agent.ContainerImage != nil {
		gqlAgent.ContainerImage = &model.ContainerImage{
			Registry:   agent.ContainerImage.Registry,
			Repository: agent.ContainerImage.Repository,
			Tag:        agent.ContainerImage.Tag,
		}
	}

	// Map Endpoint
	if agent.Endpoint != nil {
		gqlAgent.Endpoint = &model.AgentEndpoint{
			URL:           agent.Endpoint.URL,
			Type:          model.EndpointType(agent.Endpoint.Type),
			DiscoveryType: model.EndpointDiscoveryType(agent.Endpoint.DiscoveryType),
			HealthPath:    agent.Endpoint.HealthPath,
			ReadyPath:     agent.Endpoint.ReadyPath,
		}
	}

	// Map LangfuseConfig
	if agent.LangfuseConfig != nil {
		langfuseConfig := &model.LangfuseConfig{
			ProjectID:   agent.LangfuseConfig.ProjectID,
			SyncEnabled: agent.LangfuseConfig.SyncEnabled,
		}
		if agent.LangfuseConfig.LastSyncedAt != nil {
			lastSynced := strconv.FormatInt(*agent.LangfuseConfig.LastSyncedAt, 10)
			langfuseConfig.LastSyncedAt = &lastSynced
		}
		gqlAgent.LangfuseConfig = langfuseConfig
	}

	// Map Metadata
	if agent.Metadata != nil {
		metadata := &model.AgentMetadata{}

		// Convert labels map to JSON string
		if len(agent.Metadata.Labels) > 0 {
			labelsJSON, _ := json.Marshal(agent.Metadata.Labels)
			labelsStr := string(labelsJSON)
			metadata.Labels = &labelsStr
		}

		// Convert annotations map to JSON string
		if len(agent.Metadata.Annotations) > 0 {
			annotationsJSON, _ := json.Marshal(agent.Metadata.Annotations)
			annotationsStr := string(annotationsJSON)
			metadata.Annotations = &annotationsStr
		}

		gqlAgent.Metadata = metadata
	}

	// Map CreatedBy and UpdatedBy
	gqlAgent.CreatedBy = &model.UserDetails{
		UserID:   agent.AuditInfo.CreatedBy,
		Username: agent.AuditInfo.CreatedBy,
		Email:    agent.AuditInfo.CreatedBy + "@example.com", // Placeholder
	}

	gqlAgent.UpdatedBy = &model.UserDetails{
		UserID:   agent.AuditInfo.UpdatedBy,
		Username: agent.AuditInfo.UpdatedBy,
		Email:    agent.AuditInfo.UpdatedBy + "@example.com", // Placeholder
	}

	return gqlAgent
}

// MapAgentListResponseToModel converts service response to GraphQL model.
func MapAgentListResponseToModel(response *AgentListResponse) *model.AgentListResponse {
	agents := make([]*model.Agent, len(response.Agents))
	for i, agent := range response.Agents {
		agents[i] = MapAgentToModel(agent)
	}

	return &model.AgentListResponse{
		Agents:      agents,
		TotalCount:  int(response.TotalCount),
		CurrentPage: response.CurrentPage,
		TotalPages:  response.TotalPages,
		HasNextPage: response.HasNextPage,
	}
}

// MapDeleteAgentResponseToModel converts service response to GraphQL model.
func MapDeleteAgentResponseToModel(response *DeleteAgentResponse) *model.DeleteAgentResponse {
	return &model.DeleteAgentResponse{
		Success: response.Success,
		Message: response.Message,
	}
}

// MapHealthCheckResultToModel converts service result to GraphQL model.
func MapHealthCheckResultToModel(result *HealthCheckResult) *model.HealthCheckResult {
	return &model.HealthCheckResult{
		Healthy:      result.Healthy,
		Message:      result.Message,
		ResponseTime: strconv.FormatInt(result.ResponseTime, 10),
		CheckedAt:    strconv.FormatInt(result.CheckedAt, 10),
	}
}

// MapCapabilityDefinitionsToModel converts service definitions to GraphQL model.
func MapCapabilityDefinitionsToModel(defs []*CapabilityDefinition) []*model.CapabilityDefinition {
	models := make([]*model.CapabilityDefinition, len(defs))
	for i, def := range defs {
		models[i] = &model.CapabilityDefinition{
			ID:          def.ID,
			Name:        def.Name,
			Description: def.Description,
			Category:    def.Category,
		}
	}
	return models
}
