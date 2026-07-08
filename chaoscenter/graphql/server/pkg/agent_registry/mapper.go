
package agent_registry

import (
	"strconv"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
)

// MapSyncResponseToModel converts service SyncResponse to GraphQL model.
func MapSyncResponseToModel(resp *SyncResponse) *model.SyncResponse {
	if resp == nil {
		return nil
	}
	message := ""
	if resp.Message != nil {
		message = *resp.Message
	}
	return &model.SyncResponse{
		Success:  resp.Success,
		SyncedAt: resp.SyncedAt,
		Message:  message,
	}
}

// MapAgentStatusResponseToModel converts service AgentStatusResponse to GraphQL model.
func MapAgentStatusResponseToModel(resp *AgentStatusResponse) *model.AgentStatusResponse {
	if resp == nil {
		return nil
	}
	return &model.AgentStatusResponse{
		AgentID:              resp.AgentID,
		Status:               model.AgentStatus(resp.Status),
		Healthy:              resp.Healthy,
		LastCheckedAt:        resp.LastCheckedAt,
		LastSyncedToLangfuse: resp.LastSyncedToLangfuse,
	}
}

// MapRegisterAgentInputToRequest converts GraphQL input to service request.
func MapRegisterAgentInputToRequest(input model.RegisterAgentInput) (*RegisterAgentRequest, error) {
	helmReleaseName := ""
	if input.HelmReleaseName != nil {
		helmReleaseName = *input.HelmReleaseName
	}
	
	req := &RegisterAgentRequest{
		ProjectID:       input.ProjectID,
		Name:            input.Name,
		Version:         input.Version,
		Vendor:          input.Vendor,
		Capabilities:    input.Capabilities,
		Namespace:       input.Namespace,
		HelmReleaseName: helmReleaseName,
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
		healthPath := "/health"
		readyPath := "/ready"
		
		if input.Endpoint.HealthPath != nil {
			healthPath = *input.Endpoint.HealthPath
		}
		if input.Endpoint.ReadyPath != nil {
			readyPath = *input.Endpoint.ReadyPath
		}
		
		req.Endpoint = &AgentEndpoint{
			URL:           input.Endpoint.URL,
			Type:          EndpointType(input.Endpoint.EndpointType),
			DiscoveryType: EndpointDiscoveryManual,
			HealthPath:    healthPath,
			ReadyPath:     readyPath,
		}
	}

	// Map LangfuseConfig
	if input.LangfuseConfig != nil {
		syncEnabled := true
		if input.LangfuseConfig.SyncEnabled != nil {
			syncEnabled = *input.LangfuseConfig.SyncEnabled
		}
		req.LangfuseConfig = &LangfuseConfig{
			ProjectID:   input.LangfuseConfig.ProjectID,
			SyncEnabled: syncEnabled,
		}
	}

	// Map Metadata
	if input.Metadata != nil {
		metadata := &AgentMetadata{}

		// Map labels from KeyValuePair array
		if len(input.Metadata.Labels) > 0 {
			labels := make(map[string]string)
			for _, kv := range input.Metadata.Labels {
				labels[kv.Key] = kv.Value
			}
			metadata.Labels = labels
		}

		// Map annotations from KeyValuePair array
		if len(input.Metadata.Annotations) > 0 {
			annotations := make(map[string]string)
			for _, kv := range input.Metadata.Annotations {
				annotations[kv.Key] = kv.Value
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
			Repository:input.ContainerImage.Repository,
			Tag:        input.ContainerImage.Tag,
		}
	}

	// Map Endpoint
	if input.Endpoint != nil {
		healthPath := "/health"
		readyPath := "/ready"
		
		if input.Endpoint.HealthPath != nil {
			healthPath = *input.Endpoint.HealthPath
		}
		if input.Endpoint.ReadyPath != nil {
			readyPath = *input.Endpoint.ReadyPath
		}
		
		req.Endpoint = &AgentEndpoint{
			URL:           input.Endpoint.URL,
			Type:          EndpointType(input.Endpoint.EndpointType),
			DiscoveryType: EndpointDiscoveryManual,
			HealthPath:    healthPath,
			ReadyPath:     readyPath,
		}
	}

	// Map LangfuseConfig
	if input.LangfuseConfig != nil {
		syncEnabled := true
		if input.LangfuseConfig.SyncEnabled != nil {
			syncEnabled = *input.LangfuseConfig.SyncEnabled
		}
		req.LangfuseConfig = &LangfuseConfig{
			ProjectID:   input.LangfuseConfig.ProjectID,
			SyncEnabled: syncEnabled,
		}
	}

	// Map Metadata
	if input.Metadata != nil {
		metadata := &AgentMetadata{}

		// Map labels from KeyValuePair array
		if len(input.Metadata.Labels) > 0 {
			labels := make(map[string]string)
			for _, kv := range input.Metadata.Labels {
				labels[kv.Key] = kv.Value
			}
			metadata.Labels = labels
		}

		// Map annotations from KeyValuePair array
		if len(input.Metadata.Annotations) > 0 {
			annotations := make(map[string]string)
			for _, kv := range input.Metadata.Annotations {
				annotations[kv.Key] = kv.Value
			}
			metadata.Annotations = annotations
		}

		req.Metadata = metadata
	}

	return req, nil
}

// MapAgentFilterInputToFilter converts GraphQL filter input to service filter.
func MapAgentFilterInputToFilter(input *model.ListAgentsFilter) *AgentFilter {
	if input == nil {
		return &AgentFilter{}
	}
	
	filter := &AgentFilter{}
	
	if input.ProjectID != nil {
		filter.ProjectID = *input.ProjectID
	}
	
	if input.Status != nil {
		status := AgentStatus(*input.Status)
		filter.Status = &status
	}
	
	if len(input.Capabilities) > 0 {
		filter.Capabilities = input.Capabilities
	}
	
	if input.SearchTerm != nil {
		filter.SearchTerm = input.SearchTerm
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
	}

	// Map HelmReleaseName if present
	if agent.HelmReleaseName != "" {
		gqlAgent.HelmReleaseName = &agent.HelmReleaseName
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
			EndpointType:  model.EndpointType(agent.Endpoint.Type),
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

		// Convert labels map to KeyValuePair array
		if len(agent.Metadata.Labels) > 0 {
			labels := make([]*model.KeyValuePair, 0, len(agent.Metadata.Labels))
			for k, v := range agent.Metadata.Labels {
				labels = append(labels, &model.KeyValuePair{
					Key:   k,
					Value: v,
				})
			}
			metadata.Labels = labels
		}

		// Convert annotations map to KeyValuePair array
		if len(agent.Metadata.Annotations) > 0 {
			annotations := make([]*model.KeyValuePair, 0, len(agent.Metadata.Annotations))
			for k, v := range agent.Metadata.Annotations {
				annotations = append(annotations, &model.KeyValuePair{
					Key:   k,
					Value: v,
				})
			}
			metadata.Annotations = annotations
		}

		gqlAgent.Metadata = metadata
	}

	// Map AuditInfo
	if agent.AuditInfo != nil {
		gqlAgent.AuditInfo = &model.AuditInfo{
			CreatedAt:  strconv.FormatInt(agent.AuditInfo.CreatedAt, 10),
			CreatedBy:  agent.AuditInfo.CreatedBy,
			UpdatedAt:  strconv.FormatInt(agent.AuditInfo.UpdatedAt, 10),
			UpdatedBy:  agent.AuditInfo.UpdatedBy,
		}
		if agent.AuditInfo.LastHealthCheck != nil {
			lastCheck := strconv.FormatInt(*agent.AuditInfo.LastHealthCheck, 10)
			gqlAgent.AuditInfo.LastHealthCheck = &lastCheck
		}
	}

	// Map spec-aligned fields
	if agent.DisplayName != "" {
		gqlAgent.DisplayName = &agent.DisplayName
	}
	if agent.Tier != "" {
		gqlAgent.Tier = &agent.Tier
	}
	if agent.Repository != "" {
		gqlAgent.Repository = &agent.Repository
	}
	if agent.License != "" {
		gqlAgent.License = &agent.License
	}
	if agent.SchemaVersion != "" {
		gqlAgent.SchemaVersion = &agent.SchemaVersion
	}
	if agent.SpecDescription != nil {
		gqlAgent.AgentDescription = &model.AgentDescription{
			Short:        agent.SpecDescription.Short,
			Long:         agent.SpecDescription.Long,
			Approach:     &agent.SpecDescription.Approach,
			LlmDependent: agent.SpecDescription.LLMDependent,
		}
	}
	if agent.SpecInstall != nil {
		replicas := agent.SpecInstall.Replicas
		gqlAgent.Install = &model.AgentInstallSpec{
			Method:    agent.SpecInstall.Method,
			Image:     &agent.SpecInstall.Image,
			Folder:    &agent.SpecInstall.Folder,
			Namespace: agent.SpecInstall.Namespace,
			Timeout:   agent.SpecInstall.Timeout,
			CPU:       &agent.SpecInstall.CPU,
			Memory:    &agent.SpecInstall.Memory,
			Replicas:  replicas,
		}
	}
	if agent.AgentLLMConfig != nil {
		gqlAgent.LlmConfig = &model.LLMConfig{
			ConfigRef:       &agent.AgentLLMConfig.ConfigRef,
			Provider:        &agent.AgentLLMConfig.Provider,
			Model:           &agent.AgentLLMConfig.Model,
			AllowUserChoice: agent.AgentLLMConfig.AllowUserChoice,
			AllowedModels:   agent.AgentLLMConfig.AllowedModels,
			DefaultModel:    &agent.AgentLLMConfig.DefaultModel,
			LlmDependent:    agent.AgentLLMConfig.LLMDependent,
		}
	}
	if len(agent.AgentInputDefs) > 0 {
		inputs := make([]*model.AgentInput, len(agent.AgentInputDefs))
		for i, inp := range agent.AgentInputDefs {
			inp := inp // capture loop variable
			inputs[i] = &model.AgentInput{
				Key:         inp.Key,
				DisplayName: inp.DisplayName,
				Description: &inp.Description,
				Type:        inp.Type,
				Required:    inp.Required,
				Default:     &inp.Default,
				Placeholder: &inp.Placeholder,
				HelmPath:    inp.HelmPath,
				Values:      inp.Values,
				Min:         inp.Min,
				Max:         inp.Max,
				Unit:        &inp.Unit,
				Advanced:    inp.Advanced,
				Group:       &inp.Group,
			}
		}
		gqlAgent.Inputs = inputs
	}
	if len(agent.ContextInjection) > 0 {
		cis := make([]*model.ContextInjection, len(agent.ContextInjection))
		for i, ci := range agent.ContextInjection {
			ci := ci // capture loop variable
			cis[i] = &model.ContextInjection{
				HelmPath:    ci.HelmPath,
				Source:      ci.Source,
				Required:    ci.Required,
				Description: &ci.Description,
			}
		}
		gqlAgent.ContextInjection = cis
	}
	if len(agent.RequiredTools) > 0 {
		tools := make([]*model.RequiredTool, len(agent.RequiredTools))
		for i, t := range agent.RequiredTools {
			t := t // capture loop variable
			tools[i] = &model.RequiredTool{
				Name:         t.Name,
				Purpose:      &t.Purpose,
				Critical:     t.Critical,
				MinCallCount: t.MinCallCount,
				MaxCallCount: t.MaxCallCount,
			}
		}
		gqlAgent.RequiredTools = tools
	}
	if len(agent.EvalMetrics) > 0 {
		gqlAgent.EvaluationMetrics = agent.EvalMetrics
	}
	if agent.Compatibility != nil {
		gqlAgent.Compatibility = &model.AgentCompatibility{
			SupportedApps:     agent.Compatibility.SupportedApps,
			UnsupportedApps:   agent.Compatibility.UnsupportedApps,
			MinimumFaultCount: agent.Compatibility.MinFaultCount,
			MaximumFaultCount: agent.Compatibility.MaxFaultCount,
		}
	}
	if agent.AgentOwner != nil {
		gqlAgent.AgentOwner = &model.AgentOwner{
			Name:  agent.AgentOwner.Name,
			Email: agent.AgentOwner.Email,
			Org:   &agent.AgentOwner.Org,
		}
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
		ResponseTime: int(result.ResponseTime),
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
