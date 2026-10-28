package model_config

import (
	"os"
	"strings"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chartconfig"
)

// PlatformModelOptions are the models the platform serves from LiteLLM's
// config file, derived from the provider credentials in the server's
// environment (.env). Every project may use them.
func PlatformModelOptions() []*model.AgentModelOption {
	defaultAlias := chartconfig.DefaultAgentModelAlias()
	seen := map[string]bool{}
	models := make([]*model.AgentModelOption, 0, 6)

	addModel := func(alias, label, provider string) {
		alias = strings.TrimSpace(alias)
		if alias == "" || seen[alias] {
			return
		}
		seen[alias] = true
		models = append(models, &model.AgentModelOption{
			Alias:     alias,
			Label:     label,
			Provider:  provider,
			IsDefault: alias == defaultAlias,
		})
	}

	if ollamaModel := strings.TrimSpace(os.Getenv("OLLAMA_MODEL")); ollamaModel != "" {
		ollamaAlias := strings.ReplaceAll(ollamaModel, ":", "-")
		addModel(ollamaAlias, ollamaAlias+" (Ollama)", "Ollama")
	}

	if strings.TrimSpace(os.Getenv("AZURE_OPENAI_KEY")) != "" || strings.TrimSpace(os.Getenv("AZURE_OPENAI_API_KEY")) != "" {
		azureAlias := strings.TrimSpace(os.Getenv("AZURE_OPENAI_DEPLOYMENT"))
		if azureAlias == "" {
			azureAlias = strings.TrimSpace(os.Getenv("AZURE_OPENAI_CHAT_DEPLOYMENT_NAME"))
		}
		if azureAlias == "" {
			azureAlias = "gpt-4o"
		}
		addModel(azureAlias, azureAlias+" (Azure OpenAI)", "Azure OpenAI")
	}

	if strings.TrimSpace(os.Getenv("GEMINI_API_KEY")) != "" {
		addModel("gemini-3-flash", "gemini-3-flash (Gemini)", "Gemini")
		addModel("gemini-2.5-flash", "gemini-2.5-flash (Gemini)", "Gemini")
		addModel("gemini-2.5-flash-lite", "gemini-2.5-flash-lite (Gemini)", "Gemini")
	}

	if strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")) != "" {
		addModel("auto-free", "auto-free (OpenRouter)", "OpenRouter")
	}

	if len(models) == 0 && defaultAlias != "" {
		addModel(defaultAlias, defaultAlias+" (configured default)", "Configured")
	}

	return models
}

// isPlatformAlias reports whether alias belongs to the platform: one of its
// models, or the default an agent falls back to. A project model may not take
// such an alias, or Studio would show one name for two models.
func isPlatformAlias(alias string) bool {
	if alias == chartconfig.DefaultAgentModelAlias() {
		return true
	}
	for _, option := range PlatformModelOptions() {
		if option.Alias == alias {
			return true
		}
	}
	return false
}
