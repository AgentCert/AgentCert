package agent_registry

import (
	"gopkg.in/yaml.v3"
)

// GenerateAgentYAML produces the canonical agent.yaml string from an Agent model.
// The output matches the ACE spec §5 AgentCatalogEntry schema.
func GenerateAgentYAML(agent *Agent) (string, error) {
	metadata := map[string]interface{}{
		"name":    agent.Name,
		"version": agent.Version,
	}
	if agent.DisplayName != "" {
		metadata["displayName"] = agent.DisplayName
	}
	if agent.Tier != "" {
		metadata["tier"] = agent.Tier
	}
	if agent.Repository != "" {
		metadata["repository"] = agent.Repository
	}
	if agent.License != "" {
		metadata["license"] = agent.License
	}
	if agent.AgentOwner != nil {
		ownerMap := map[string]interface{}{
			"name":  agent.AgentOwner.Name,
			"email": agent.AgentOwner.Email,
		}
		if agent.AgentOwner.Org != "" {
			ownerMap["org"] = agent.AgentOwner.Org
		}
		metadata["owner"] = ownerMap
	}

	spec := map[string]interface{}{}

	if agent.SpecDescription != nil {
		descMap := map[string]interface{}{
			"short":        agent.SpecDescription.Short,
			"long":         agent.SpecDescription.Long,
			"llmDependent": agent.SpecDescription.LLMDependent,
		}
		if agent.SpecDescription.Approach != "" {
			descMap["approach"] = agent.SpecDescription.Approach
		}
		spec["description"] = descMap
	}

	if agent.SpecInstall != nil {
		installMap := map[string]interface{}{
			"method":    agent.SpecInstall.Method,
			"image":     agent.SpecInstall.Image,
			"namespace": agent.SpecInstall.Namespace,
		}
		if agent.SpecInstall.Timeout != "" {
			installMap["timeout"] = agent.SpecInstall.Timeout
		}
		spec["install"] = installMap
	}

	if len(agent.Capabilities) > 0 {
		spec["capabilities"] = agent.Capabilities
	}

	if len(agent.EvalMetrics) > 0 {
		spec["evaluationMetrics"] = agent.EvalMetrics
	}

	if agent.AgentLLMConfig != nil {
		llmMap := map[string]interface{}{
			"dependent": agent.AgentLLMConfig.LLMDependent,
		}
		if agent.AgentLLMConfig.Provider != "" {
			llmMap["provider"] = agent.AgentLLMConfig.Provider
		}
		if agent.AgentLLMConfig.Model != "" {
			llmMap["model"] = agent.AgentLLMConfig.Model
		}
		if agent.AgentLLMConfig.ConfigRef != "" {
			llmMap["configRef"] = agent.AgentLLMConfig.ConfigRef
		}
		spec["llmConfig"] = llmMap
	}

	if len(agent.AgentInputDefs) > 0 {
		inputList := make([]map[string]interface{}, 0, len(agent.AgentInputDefs))
		for _, inp := range agent.AgentInputDefs {
			m := map[string]interface{}{
				"key":      inp.Key,
				"type":     inp.Type,
				"required": inp.Required,
				"helmPath": inp.HelmPath,
			}
			if inp.DisplayName != "" {
				m["displayName"] = inp.DisplayName
			}
			if inp.Default != "" {
				m["default"] = inp.Default
			}
			inputList = append(inputList, m)
		}
		spec["inputs"] = inputList
	}

	if len(agent.ContextInjection) > 0 {
		ciList := make([]map[string]interface{}, 0, len(agent.ContextInjection))
		for _, ci := range agent.ContextInjection {
			ciList = append(ciList, map[string]interface{}{
				"helmPath": ci.HelmPath,
				"source":   ci.Source,
				"required": ci.Required,
			})
		}
		spec["contextInjection"] = ciList
	}

	if len(agent.RequiredTools) > 0 {
		toolList := make([]map[string]interface{}, 0, len(agent.RequiredTools))
		for _, t := range agent.RequiredTools {
			toolList = append(toolList, map[string]interface{}{
				"name":     t.Name,
				"critical": t.Critical,
			})
		}
		spec["requiredTools"] = toolList
	}

	if agent.Compatibility != nil {
		compMap := map[string]interface{}{}
		if len(agent.Compatibility.SupportedApps) > 0 {
			compMap["supportedApps"] = agent.Compatibility.SupportedApps
		}
		if len(agent.Compatibility.UnsupportedApps) > 0 {
			compMap["unsupportedApps"] = agent.Compatibility.UnsupportedApps
		}
		spec["compatibility"] = compMap
	}

	doc := map[string]interface{}{
		"apiVersion": "ace.io/v1",
		"kind":       "AgentCatalogEntry",
		"metadata":   metadata,
		"spec":       spec,
	}

	out, err := yaml.Marshal(doc)
	return string(out), err
}
