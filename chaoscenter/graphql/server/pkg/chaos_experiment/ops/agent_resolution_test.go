package ops

import (
	"context"
	"strings"
	"testing"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	agentRegistry "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/agent_registry"
)

// fakeRegistry holds agents in registration order, like the Mongo collection
// GetAgentByNamespace's FindOne scans.
type fakeRegistry struct {
	agentRegistry.Operator
	agents []*agentRegistry.Agent
}

func (f *fakeRegistry) GetAgentByNamespace(_ context.Context, namespace string) (*agentRegistry.Agent, error) {
	for _, a := range f.agents {
		if a.Namespace == namespace {
			return a, nil
		}
	}
	return nil, agentRegistry.ErrAgentNotFound
}

func (f *fakeRegistry) GetAgentByNamespaceAndName(_ context.Context, namespace, name string) (*agentRegistry.Agent, error) {
	for _, a := range f.agents {
		if a.Namespace == namespace && a.Name == name {
			return a, nil
		}
	}
	return nil, agentRegistry.ErrAgentNotFound
}

func installAgentStep(folder, namespace string) v1alpha1.Template {
	return v1alpha1.Template{
		Name:      "install-agent",
		Container: &corev1.Container{Image: "agentcert/agentcert-install-agent:latest", Args: []string{"-folder=" + folder, "-namespace=" + namespace}},
	}
}

func TestResolveWorkflowAgentPicksTheInstalledAgentNotTheFirstInNamespace(t *testing.T) {
	registry := &fakeRegistry{agents: []*agentRegistry.Agent{
		{AgentID: "flash-id", Name: "flash-agent", Namespace: "sock-shop"},
		{AgentID: "sre-id", Name: "sre-agent-comprehensive", Namespace: "sock-shop"},
	}}
	templates := []v1alpha1.Template{installAgentStep("sre-agent-comprehensive", "sock-shop")}

	got := ResolveWorkflowAgent(context.Background(), registry, templates, "litmus")
	if got == nil || got.AgentID != "sre-id" {
		t.Fatalf("resolved %+v, want sre-agent-comprehensive", got)
	}
}

func TestResolveWorkflowAgentDoesNotBorrowAnotherAgentsIdentity(t *testing.T) {
	registry := &fakeRegistry{agents: []*agentRegistry.Agent{
		{AgentID: "flash-id", Name: "flash-agent", Namespace: "sock-shop"},
	}}
	templates := []v1alpha1.Template{installAgentStep("sre-agent-crewai", "sock-shop")}

	if got := ResolveWorkflowAgent(context.Background(), registry, templates, "litmus"); got != nil {
		t.Fatalf("an unregistered agent resolved to %+v; install-agent must self-register instead", got)
	}
}

func TestResolveManifestAgentReadsStoredWorkflow(t *testing.T) {
	registry := &fakeRegistry{agents: []*agentRegistry.Agent{
		{AgentID: "flash-id", Name: "flash-agent", Namespace: "book-info"},
	}}
	manifest := `{"kind":"Workflow","spec":{"templates":[` +
		`{"name":"uninstall-all","container":{"image":"agentcert/agentcert-install-agent:latest","command":["sh","-c"],"args":["true"]}},` +
		`{"name":"install-agent","container":{"image":"agentcert/agentcert-install-agent:latest","args":["-folder=flash-agent","-namespace=book-info"]}}]}}`

	got := ResolveManifestAgent(context.Background(), registry, manifest, "")
	if got == nil || got.AgentID != "flash-id" {
		t.Fatalf("resolved %+v, want the book-info flash-agent", got)
	}
}

func TestUninstallAllIsNotAnAgentInstallStep(t *testing.T) {
	cleanup := v1alpha1.Template{
		Name: uninstallAllTemplateName,
		// Revisions saved before the fix carry this annotation on the cleanup step.
		Metadata:  v1alpha1.Metadata{Annotations: map[string]string{"agentcert.io/install-type": "agent"}},
		Container: &corev1.Container{Image: "agentcert/agentcert-install-agent:latest", Command: []string{"sh", "-c"}, Args: []string{"true"}},
	}
	if IsAgentInstallStep(cleanup) {
		t.Fatal("uninstall-all was classified as the agent install step")
	}

	templates := []v1alpha1.Template{cleanup, installAgentStep("flash-agent", "sock-shop")}
	InjectExperimentContextArgs(templates, "")
	if len(templates[0].Container.Args) != 1 {
		t.Fatalf("install-agent context args were appended to uninstall-all: %v", templates[0].Container.Args)
	}
	if !strings.Contains(strings.Join(templates[1].Container.Args, " "), "agent.config.EXPERIMENT_ID=") {
		t.Fatal("install-agent step did not receive its context args")
	}
}

func TestGuaranteedCleanupTargetsChaosNamespaceAndToleratesSelfDeletedRelease(t *testing.T) {
	spec := &v1alpha1.WorkflowSpec{
		Arguments: v1alpha1.Arguments{Parameters: []v1alpha1.Parameter{{Name: "adminModeNamespace", Value: v1alpha1.AnyStringPtr("litmus")}}},
		Templates: []v1alpha1.Template{
			cleanupInstallTemplate("install-application", "application", "sock-shop", "sock-shop", ""),
			cleanupInstallTemplate("install-agent", "agent", "flash-agent", "sock-shop", ""),
		},
	}
	if err := ApplyGuaranteedCleanupPatch(spec); err != nil {
		t.Fatal(err)
	}
	// Re-applying at run time must still find the releases: the cleanup
	// template it created must not be mistaken for the agent install step.
	if err := ApplyGuaranteedCleanupPatch(spec); err != nil {
		t.Fatal(err)
	}
	var cleanup *v1alpha1.Template
	for i := range spec.Templates {
		if spec.Templates[i].Name == uninstallAllTemplateName {
			cleanup = &spec.Templates[i]
		}
	}
	if cleanup == nil {
		t.Fatal("cleanup template missing")
	}
	env := map[string]string{}
	for _, e := range cleanup.Container.Env {
		env[e.Name] = e.Value
	}
	if env["CHAOS_NAMESPACE"] != "{{workflow.parameters.adminModeNamespace}}" {
		t.Fatalf("ChaosEngines are cleaned up in %q, not the chaos namespace", env["CHAOS_NAMESPACE"])
	}
	if env["AGENT_RELEASE"] != "flash-agent" || env["APP_RELEASE"] != "sock-shop" {
		t.Fatalf("releases lost on re-apply: %#v", env)
	}
	if !strings.Contains(cleanup.Container.Args[0], `helm status "$1" -n "$2"`) {
		t.Fatal("helm uninstall failures are not re-checked against the release's actual state")
	}
}
