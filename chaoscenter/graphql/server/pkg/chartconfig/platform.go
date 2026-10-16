package chartconfig

import (
	"regexp"
	"strings"
)

// platformOwnedKeys are the chart values the server or installer writes on
// each install (ops.InjectExperimentContextArgs), plus a few it wrote in the
// past. Stale copies are stripped from saved manifests before fresh ones are
// appended, and a chart may not offer them as settings: a user's value would
// either be overwritten silently or cut the agent off from the platform
// (identity, tracing, MCP wiring, blind mode).
var platformOwnedKeys = map[string]bool{
	"config.openaiApiKey":                            true,
	"config.openaiBaseUrl":                           true,
	"agentId":                                        true,
	"namespaces.bookInfo":                            true,
	"namespaces.otelDemo":                            true,
	"namespaces.sockShop":                            true,
	"agent.config.AGENT_ID":                          true,
	"agent.config.MCP_INCLUDE_CHAOS_TOOLS":           true,
	"agent.config.NOTIFY_ID":                         true,
	"agent.config.EXPERIMENT_ID":                     true,
	"agent.config.EXPERIMENT_RUN_ID":                 true,
	"agent.config.WORKFLOW_NAME":                     true,
	"agent.config.WORKFLOW_UID":                      true,
	"agent.config.OPENAI_API_KEY":                    true,
	"agent.secret.LITELLM_MASTER_KEY":                true,
	"agent.config.OPENAI_BASE_URL":                   true,
	"agent.config.LANGFUSE_HOST":                     true,
	"agent.secret.LANGFUSE_PUBLIC_KEY":               true,
	"agent.secret.LANGFUSE_SECRET_KEY":               true,
	"agent.config.MCP_URLS":                          true,
	"agent.config.K8S_MCP_URL":                       true,
	"agent.config.PROM_MCP_URL":                      true,
	"agent.config.CHAOS_NAMESPACE":                   true,
	"agent.config.K8S_NAMESPACE":                     true,
	"agent.config.TARGET_APP_NAME":                   true,
	"agent.config.TARGET_NAMESPACE":                  true,
	"agent.config.AGENT_MAX_RUNTIME_SECONDS":         true,
	"agent.config.AGENT_IDLE_AFTER_MAX_RUNTIME":      true,
	"agent.config.MODEL_ALIAS":                       true,
	"agent.config.AGENT_MODE":                        true,
	"agent.config.AGENT_SCOPE_NAMESPACE":             true,
	"agent.config.MITIGATION_ALLOW_DISCOVERED_SCOPE": true,
	"agent.config.MITIGATION_REVIEW_ITERS":           true,
	"agent.config.MITIGATION_AUDIT_PATH":             true,
	"agent.config.AGENT_MEMORY_PATH":                 true,
	"agent.config.MEMORY_TTL_DAYS":                   true,
	"agent.config.REVIEWER_MODEL_ALIAS":              true,
	"agent.config.GROUND_TRUTH_JSON":                 true,
	"sidecar.enabled":                                true,
	"sidecar.injectionMode":                          true,
	"sidecar.upstream":                               true,
	"sidecar.image.registry":                         true,
	"sidecar.image.repository":                       true,
	"sidecar.image.tag":                              true,
	"sidecar.image.pullPolicy":                       true,
}

// ModelAliasKey is the agent's LLM model. The server still writes it on every
// install, but resolves it from the user's setting before its own default
// (see ops.InjectExperimentContextArgs), so it is the one platform-owned value
// a chart may offer.
const ModelAliasKey = "agent.config.MODEL_ALIAS"

// IsPlatformOwned reports whether the server or installer owns key.
func IsPlatformOwned(key string) bool {
	return platformOwnedKeys[key]
}

// IsUserSettable reports whether a platform-owned key may still be offered as
// a setting, because the server honours the user's value for it.
func IsUserSettable(key string) bool {
	return key == ModelAliasKey
}

// secretSegment matches a values-path segment that names a credential. It
// matches whole words only, so MAX_COMPLETION_TOKENS is not mistaken for one.
var secretSegment = regexp.MustCompile(`(^|[_-])(password|passwd|secret|token|api_?key|credentials?|private_?key)($|[_-])`)

// IsSecretKey reports whether key holds a credential. Settings are stored in
// the experiment manifest in plain text, so secrets cannot be offered.
func IsSecretKey(key string) bool {
	if strings.HasPrefix(key, "agent.secret.") {
		return true
	}
	for _, segment := range strings.Split(key, ".") {
		if secretSegment.MatchString(strings.ToLower(segment)) {
			return true
		}
	}
	return false
}
