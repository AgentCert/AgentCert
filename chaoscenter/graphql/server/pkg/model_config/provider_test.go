package model_config

import (
	"errors"
	"strings"
	"testing"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
)

func ptr(value string) *string { return &value }

func TestNormalizeConnectionAppliesTheProviderRules(t *testing.T) {
	cases := []struct {
		name string
		in   model.ModelConnectionInput
		want connection
	}{
		{
			name: "azure: prefix added, version kept",
			in:   model.ModelConnectionInput{Provider: model.ModelProviderAzureOpenai, Model: " gpt-4o ", APIBase: ptr("https://acme.openai.azure.com"), APIVersion: ptr("2024-08-01-preview"), APIKey: ptr(" az-key-123456789 ")},
			want: connection{provider: model.ModelProviderAzureOpenai, model: "azure/gpt-4o", apiBase: "https://acme.openai.azure.com", apiVersion: "2024-08-01-preview", apiKey: "az-key-123456789"},
		},
		{
			name: "openai: an existing prefix is kept, the default endpoint filled in, a version dropped",
			in:   model.ModelConnectionInput{Provider: model.ModelProviderOpenai, Model: "openai/gpt-4o-mini", APIVersion: ptr("ignored"), APIKey: ptr("sk-openai")},
			want: connection{provider: model.ModelProviderOpenai, model: "openai/gpt-4o-mini", apiBase: "https://api.openai.com/v1", apiKey: "sk-openai"},
		},
		{
			name: "gemini default endpoint",
			in:   model.ModelConnectionInput{Provider: model.ModelProviderGemini, Model: "gemini-2.5-pro", APIKey: ptr("g-key")},
			want: connection{provider: model.ModelProviderGemini, model: "gemini/gemini-2.5-pro", apiBase: "https://generativelanguage.googleapis.com/v1beta", apiKey: "g-key"},
		},
		{
			name: "openrouter keeps the vendor path",
			in:   model.ModelConnectionInput{Provider: model.ModelProviderOpenrouter, Model: "anthropic/claude-3.5-sonnet", APIKey: ptr("or-key")},
			want: connection{provider: model.ModelProviderOpenrouter, model: "openrouter/anthropic/claude-3.5-sonnet", apiBase: "https://openrouter.ai/api/v1", apiKey: "or-key"},
		},
		{
			name: "ollama needs no key; both prefixes accepted",
			in:   model.ModelConnectionInput{Provider: model.ModelProviderOllama, Model: "ollama/qwen2.5:32b-instruct", APIBase: ptr("http://ollama.ace.svc.cluster.local:11434")},
			want: connection{provider: model.ModelProviderOllama, model: "ollama/qwen2.5:32b-instruct", apiBase: "http://ollama.ace.svc.cluster.local:11434"},
		},
		{
			name: "ollama default prefix is the chat one",
			in:   model.ModelConnectionInput{Provider: model.ModelProviderOllama, Model: "qwen2.5:7b", APIBase: ptr("http://ollama:11434")},
			want: connection{provider: model.ModelProviderOllama, model: "ollama_chat/qwen2.5:7b", apiBase: "http://ollama:11434"},
		},
		{
			name: "openai-compatible with a slash in the served name",
			in:   model.ModelConnectionInput{Provider: model.ModelProviderOpenaiCompatible, Model: "meta-llama/Llama-3.1-8B-Instruct", APIBase: ptr("http://vllm:8000/v1")},
			want: connection{provider: model.ModelProviderOpenaiCompatible, model: "openai/meta-llama/Llama-3.1-8B-Instruct", apiBase: "http://vllm:8000/v1"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeConnection(tc.in)
			if err != nil {
				t.Fatalf("normalizeConnection() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("normalizeConnection() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestNormalizeConnectionRejects(t *testing.T) {
	azure := func(edit func(*model.ModelConnectionInput)) model.ModelConnectionInput {
		in := model.ModelConnectionInput{Provider: model.ModelProviderAzureOpenai, Model: "gpt-4o", APIBase: ptr("https://acme.openai.azure.com"), APIVersion: ptr("2024-08-01-preview"), APIKey: ptr("az-key")}
		edit(&in)
		return in
	}
	cases := []struct {
		name string
		in   model.ModelConnectionInput
		want string
	}{
		// Each of these would make LiteLLM resolve a secret of its own and
		// send it to the endpoint the project owner chose.
		{"an env reference as the key", azure(func(in *model.ModelConnectionInput) { in.APIKey = ptr("os.environ/AZURE_OPENAI_API_KEY") }), "secret on the gateway"},
		{"an env reference in any case", azure(func(in *model.ModelConnectionInput) { in.APIKey = ptr(" OS.ENVIRON/LITELLM_MASTER_KEY") }), "secret on the gateway"},
		{"an env reference as the model", azure(func(in *model.ModelConnectionInput) { in.Model = "os.environ/LITELLM_AZURE_CHAT_MODEL" }), "secret on the gateway"},
		{"an env reference as the endpoint", azure(func(in *model.ModelConnectionInput) { in.APIBase = ptr("os.environ/AZURE_OPENAI_ENDPOINT") }), "secret on the gateway"},
		{"an OIDC identity token", azure(func(in *model.ModelConnectionInput) { in.APIKey = ptr("oidc/azure/api://AzureADTokenExchange") }), "secret on the gateway"},
		{"a missing key for a keyed provider", azure(func(in *model.ModelConnectionInput) { in.APIKey = ptr("   ") }), "needs an API key"},
		{"a missing version for azure", azure(func(in *model.ModelConnectionInput) { in.APIVersion = nil }), "needs an API version"},
		{"a missing endpoint for azure", azure(func(in *model.ModelConnectionInput) { in.APIBase = nil }), "needs an API base URL"},
		{"a non-http endpoint", azure(func(in *model.ModelConnectionInput) { in.APIBase = ptr("file:///etc/passwd") }), "http(s) URL"},
		{"credentials in the endpoint", azure(func(in *model.ModelConnectionInput) { in.APIBase = ptr("https://user:pass@acme.openai.azure.com") }), "must not contain credentials"},
		{"a model of only the prefix", azure(func(in *model.ModelConnectionInput) { in.Model = "azure/" }), "may only contain"},
		{"a model with spaces", azure(func(in *model.ModelConnectionInput) { in.Model = "gpt 4o" }), "may only contain"},
		{"a key with whitespace inside", azure(func(in *model.ModelConnectionInput) { in.APIKey = ptr("sk-abc def") }), "malformed"},
		{"an unknown provider", model.ModelConnectionInput{Provider: "BEDROCK", Model: "x"}, "unknown provider"},
		{"ollama without an endpoint", model.ModelConnectionInput{Provider: model.ModelProviderOllama, Model: "qwen2.5:7b"}, "needs an API base URL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := normalizeConnection(tc.in)
			if !errors.Is(err, ErrInvalidModel) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("normalizeConnection() error = %v, want ErrInvalidModel containing %q", err, tc.want)
			}
		})
	}
}

func TestAliasLabelAndKeyHelpers(t *testing.T) {
	for _, alias := range []string{"gpt-4o-bench", "qwen2.5-32b", "a1", "claude_3.5"} {
		if _, err := normalizeAlias(alias); err != nil {
			t.Errorf("normalizeAlias(%q) error = %v", alias, err)
		}
	}
	for _, alias := range []string{"", "a", "GPT-4o", "-lead", "has space", "x/y", strings.Repeat("a", 64)} {
		if _, err := normalizeAlias(alias); !errors.Is(err, ErrInvalidModel) {
			t.Errorf("normalizeAlias(%q) error = %v, want ErrInvalidModel", alias, err)
		}
	}
	if label, _ := normalizeLabel("  ", "gpt-4o-bench"); label != "gpt-4o-bench" {
		t.Errorf("an empty label = %q, want the alias", label)
	}
	if _, err := normalizeLabel(strings.Repeat("x", maxLabelLength+1), "a1"); err == nil {
		t.Error("an over-long label must be rejected")
	}
	for key, want := range map[string]string{"": "", "short-key": "****", "sk-proj-abcdefgh1234": "****1234"} {
		if got := keyHint(key); got != want {
			t.Errorf("keyHint(%q) = %q, want %q", key, got, want)
		}
	}
	if keyToSend("") != noKeyPlaceholder || keyToSend("sk-1") != "sk-1" {
		t.Error("keyToSend must substitute the placeholder for an empty key only")
	}
}
