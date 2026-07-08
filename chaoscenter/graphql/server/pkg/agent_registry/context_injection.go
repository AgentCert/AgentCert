package agent_registry

import (
	"context"
	"fmt"
	"os"
)

// DefaultContextInjections returns the 3 mandatory context injections that every agent must have.
// These use Argo template expressions that are resolved at workflow execution time.
func DefaultContextInjections() []ContextInjectDef {
	return []ContextInjectDef{
		{
			HelmPath:    "agent.notifyId",
			Source:      "{{workflow.name}}",
			Required:    true,
			Description: "Workflow name — agent uses this as the experiment correlation ID",
		},
		{
			HelmPath:    "agent.workflowUid",
			Source:      "{{workflow.uid}}",
			Required:    true,
			Description: "Workflow UID — certifier uses this to match agent events to runs",
		},
		{
			HelmPath:    "sidecar.upstream",
			Source:      "{{workflow.parameters.litellmUpstream}}",
			Required:    true,
			Description: "LiteLLM proxy — routes LLM API calls through ACE for tracing",
		},
	}
}

// MergeContextInjections merges caller-provided injections with the defaults.
// Default injections come first; caller injections are appended (deduped by HelmPath).
func MergeContextInjections(defaults []ContextInjectDef, extra []ContextInjectDef) []ContextInjectDef {
	seen := make(map[string]struct{}, len(defaults))
	merged := make([]ContextInjectDef, 0, len(defaults)+len(extra))
	for _, ci := range defaults {
		seen[ci.HelmPath] = struct{}{}
		merged = append(merged, ci)
	}
	for _, ci := range extra {
		if _, exists := seen[ci.HelmPath]; !exists {
			merged = append(merged, ci)
		}
	}
	return merged
}

// BuildInstallAgentContextArgs converts context injections to --set args for the install-agent binary.
// Argo template expressions ({{workflow.xxx}}) are passed as literal strings — Argo resolves them.
func BuildInstallAgentContextArgs(injections []ContextInjectDef) []string {
	args := make([]string, 0, len(injections))
	for _, ci := range injections {
		args = append(args, fmt.Sprintf("--set=%s=%s", ci.HelmPath, ci.Source))
	}
	return args
}

// ResolveLiteLLMUpstream returns the LiteLLM proxy upstream URL from the LITELLM_PROXY_URL env var.
// If not set, returns the default cluster-internal URL.
func ResolveLiteLLMUpstream() string {
	base := os.Getenv("LITELLM_PROXY_URL")
	if base == "" {
		base = "http://litellm.litmus.svc.cluster.local:4000"
	}
	return base + "/v1"
}

// BuildLiteLLMWorkflowParam returns the LiteLLM upstream URL to embed as an Argo workflow parameter.
// Returns an empty string when the agent is not LLM-dependent (no parameter needed).
func BuildLiteLLMWorkflowParam(_ context.Context, agentLLMConfig *AgentLLMConfig) string {
	if agentLLMConfig == nil || !agentLLMConfig.LLMDependent {
		return ""
	}
	return ResolveLiteLLMUpstream()
}
