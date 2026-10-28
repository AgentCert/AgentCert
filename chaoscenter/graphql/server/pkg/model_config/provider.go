package model_config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
)

// noKeyPlaceholder is sent as the API key of a model that needs none. LiteLLM
// must never get an empty key: it would fall back to a key from its own
// environment (OPENAI_API_KEY, AZURE_OPENAI_API_KEY, GEMINI_API_KEY, ...) or
// from a config model with the same model string, i.e. send the platform's
// credentials to an endpoint a project owner chose.
const noKeyPlaceholder = "ace-no-key"

type providerRule struct {
	label string
	// prefixes are the accepted LiteLLM model prefixes. The first is added
	// when the model is given without one, so "gpt-4o" becomes "azure/gpt-4o".
	prefixes []string
	// defaultAPIBase is used when none is given. Every stored model names its
	// endpoint, so LiteLLM never falls back to a base URL from its own
	// environment. Empty: the user must give one.
	defaultAPIBase string
	needsVersion   bool
	needsKey       bool
}

var providerRules = map[model.ModelProvider]providerRule{
	model.ModelProviderAzureOpenai:      {label: "Azure OpenAI", prefixes: []string{"azure/"}, needsVersion: true, needsKey: true},
	model.ModelProviderOpenai:           {label: "OpenAI", prefixes: []string{"openai/"}, defaultAPIBase: "https://api.openai.com/v1", needsKey: true},
	model.ModelProviderGemini:           {label: "Gemini", prefixes: []string{"gemini/"}, defaultAPIBase: "https://generativelanguage.googleapis.com/v1beta", needsKey: true},
	model.ModelProviderOpenrouter:       {label: "OpenRouter", prefixes: []string{"openrouter/"}, defaultAPIBase: "https://openrouter.ai/api/v1", needsKey: true},
	model.ModelProviderOllama:           {label: "Ollama", prefixes: []string{"ollama_chat/", "ollama/"}},
	model.ModelProviderOpenaiCompatible: {label: "OpenAI-compatible", prefixes: []string{"openai/"}},
}

var (
	aliasPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,62}$`)
	modelPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@+-]{0,199}$`)
	versionPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,40}$`)
)

const maxLabelLength = 100

// connection is a validated, normalized ModelConnectionInput.
type connection struct {
	provider   model.ModelProvider
	model      string
	apiBase    string
	apiVersion string
	// apiKey as entered; empty when the model needs none.
	apiKey string
}

func (c connection) deployment(modelID, alias string) Deployment {
	return Deployment{ModelID: modelID, Alias: alias, Model: c.model, APIBase: c.apiBase, APIVersion: c.apiVersion}
}

// normalizeConnection checks a connection against its provider's rules and
// fills in what the provider implies (model prefix, default API base).
func normalizeConnection(in model.ModelConnectionInput) (connection, error) {
	rule, ok := providerRules[in.Provider]
	if !ok {
		return connection{}, invalidf("unknown provider %q", in.Provider)
	}
	fields := []struct{ name, value string }{
		{"model", in.Model}, {"API base", valueOf(in.APIBase)}, {"API version", valueOf(in.APIVersion)}, {"API key", valueOf(in.APIKey)},
	}
	for _, field := range fields {
		if isSecretReference(field.value) {
			return connection{}, invalidf("the %s may not reference a secret on the gateway; enter the value itself", field.name)
		}
	}

	modelName, err := normalizeModel(rule, in.Model)
	if err != nil {
		return connection{}, err
	}
	apiBase, err := normalizeAPIBase(rule, valueOf(in.APIBase))
	if err != nil {
		return connection{}, err
	}
	apiVersion := ""
	if rule.needsVersion {
		apiVersion = strings.TrimSpace(valueOf(in.APIVersion))
		if !versionPattern.MatchString(apiVersion) {
			return connection{}, invalidf("%s needs an API version, e.g. 2024-08-01-preview", rule.label)
		}
	}
	apiKey, err := normalizeKey(in.Provider, valueOf(in.APIKey))
	if err != nil {
		return connection{}, err
	}
	return connection{provider: in.Provider, model: modelName, apiBase: apiBase, apiVersion: apiVersion, apiKey: apiKey}, nil
}

func normalizeModel(rule providerRule, raw string) (string, error) {
	modelName := strings.TrimSpace(raw)
	if modelName == "" {
		return "", invalidf("the model is required")
	}
	prefix := ""
	for _, candidate := range rule.prefixes {
		if strings.HasPrefix(modelName, candidate) {
			prefix = candidate
			break
		}
	}
	if prefix == "" {
		prefix = rule.prefixes[0]
		modelName = prefix + modelName
	}
	if modelName == prefix || !modelPattern.MatchString(modelName) {
		return "", invalidf("the model %q may only contain letters, digits and . _ : / @ + -", raw)
	}
	return modelName, nil
}

func normalizeAPIBase(rule providerRule, raw string) (string, error) {
	apiBase := strings.TrimSpace(raw)
	if apiBase == "" {
		apiBase = rule.defaultAPIBase
	}
	if apiBase == "" {
		return "", invalidf("%s needs an API base URL", rule.label)
	}
	parsed, err := url.Parse(apiBase)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", invalidf("the API base must be an http(s) URL, e.g. https://my-resource.openai.azure.com")
	}
	if parsed.User != nil {
		return "", invalidf("the API base must not contain credentials; enter the key in the API key field")
	}
	return apiBase, nil
}

// normalizeKey checks an API key for provider. A provider that needs no key
// accepts an empty one.
func normalizeKey(provider model.ModelProvider, raw string) (string, error) {
	rule := providerRules[provider]
	apiKey := strings.TrimSpace(raw)
	if apiKey == "" {
		if rule.needsKey {
			return "", invalidf("%s needs an API key", rule.label)
		}
		return "", nil
	}
	if isSecretReference(apiKey) {
		return "", invalidf("the API key may not reference a secret on the gateway; enter the key itself")
	}
	if len(apiKey) > 4096 || strings.IndexFunc(apiKey, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return "", invalidf("the API key is malformed")
	}
	return apiKey, nil
}

// keyToSend is the API key LiteLLM is given: the user's, or the placeholder.
func keyToSend(apiKey string) string {
	if apiKey == "" {
		return noKeyPlaceholder
	}
	return apiKey
}

// isSecretReference reports whether LiteLLM would resolve value as a secret
// reference instead of using it as is: os.environ/NAME reads its own
// environment, oidc/... fetches the pod's cloud identity token.
func isSecretReference(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(lower, "os.environ/") || strings.HasPrefix(lower, "oidc/")
}

func normalizeAlias(raw string) (string, error) {
	alias := strings.TrimSpace(raw)
	if !aliasPattern.MatchString(alias) {
		return "", invalidf("the alias %q must be 2-63 characters of lowercase letters, digits, '.', '_' or '-', starting with a letter or digit", raw)
	}
	return alias, nil
}

func normalizeLabel(raw, alias string) (string, error) {
	label := strings.TrimSpace(raw)
	if label == "" {
		return alias, nil
	}
	if len(label) > maxLabelLength {
		return "", invalidf("the display name may be at most %d characters", maxLabelLength)
	}
	return label, nil
}

// keyHint masks a key for display. Short keys show no characters at all.
func keyHint(apiKey string) string {
	switch {
	case apiKey == "":
		return ""
	case len(apiKey) < 12:
		return "****"
	default:
		return "****" + apiKey[len(apiKey)-4:]
	}
}

func providerLabel(provider string) string {
	if rule, ok := providerRules[model.ModelProvider(provider)]; ok {
		return rule.label
	}
	return provider
}

func invalidf(format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", ErrInvalidModel, fmt.Sprintf(format, args...))
}

func valueOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
