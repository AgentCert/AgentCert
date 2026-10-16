package ops

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/tidwall/gjson"

	agentRegistry "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/agent_registry"
)

// ResolveWorkflowAgent returns the registered agent that a workflow's
// install-agent step deploys, or nil when it is not registered yet (the
// install-agent binary registers it on first install).
//
// Every catalogue agent installs into the application's namespace, so over
// time several agents share one namespace (flash-agent, sre-agent-comprehensive
// and sre-agent-crewai can all target sock-shop). A namespace-only lookup
// returns whichever registered first, which injected the wrong agentId into
// the install step and attributed runs, traces and certificates to the wrong
// agent. install-agent registers under the chart's name, which is the step's
// chart folder for every catalogue agent, so (namespace, folder) identifies it.
// The namespace-only lookup is used only when the step names no folder.
func ResolveWorkflowAgent(ctx context.Context, registry agentRegistry.Operator, templates []v1alpha1.Template, fallbackNamespace string) *agentRegistry.Agent {
	if registry == nil {
		return nil
	}
	namespace := ExtractInstallAgentNamespace(templates)
	if namespace == "" {
		namespace = fallbackNamespace
	}
	if namespace == "" {
		return nil
	}
	if folder := ExtractInstallAgentFolder(templates); folder != "" {
		if agent, err := registry.GetAgentByNamespaceAndName(ctx, namespace, folder); err == nil && agent != nil {
			return agent
		}
		return nil
	}
	if agent, err := registry.GetAgentByNamespace(ctx, namespace); err == nil && agent != nil {
		return agent
	}
	return nil
}

// ResolveManifestAgent is ResolveWorkflowAgent for a stored experiment
// manifest (a JSON Workflow or CronWorkflow).
func ResolveManifestAgent(ctx context.Context, registry agentRegistry.Operator, manifest, fallbackNamespace string) *agentRegistry.Agent {
	templates, err := manifestTemplates(manifest)
	if err != nil {
		return nil
	}
	return ResolveWorkflowAgent(ctx, registry, templates, fallbackNamespace)
}

func manifestTemplates(manifest string) ([]v1alpha1.Template, error) {
	if strings.EqualFold(gjson.Get(manifest, "kind").String(), "CronWorkflow") {
		var cron v1alpha1.CronWorkflow
		if err := json.Unmarshal([]byte(manifest), &cron); err != nil {
			return nil, err
		}
		return cron.Spec.WorkflowSpec.Templates, nil
	}
	var wf v1alpha1.Workflow
	if err := json.Unmarshal([]byte(manifest), &wf); err != nil {
		return nil, err
	}
	return wf.Spec.Templates, nil
}
