package graph

import (
	"os"
	"strings"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/agent_registry"
)

func mapHelmEnvVarInputs(inputs []*model.HelmEnvVarInput) []agent_registry.HelmEnvVar {
	out := make([]agent_registry.HelmEnvVar, 0, len(inputs))
	for _, in := range inputs {
		if in == nil || strings.TrimSpace(in.Value) == "" {
			continue
		}
		sensitive := false
		if in.Sensitive != nil {
			sensitive = *in.Sensitive
		}
		out = append(out, agent_registry.HelmEnvVar{
			Name:      in.Name,
			Value:     in.Value,
			Sensitive: sensitive,
		})
	}
	return out
}

func resolveAgentInstallNamespace(userNamespace string) (string, string) {
	sysNs := strings.TrimSpace(os.Getenv("AGENT_INSTALL_NAMESPACE"))
	if sysNs == "" {
		return userNamespace, ""
	}
	return sysNs, userNamespace
}
