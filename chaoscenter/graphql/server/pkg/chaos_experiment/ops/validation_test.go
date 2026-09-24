package ops

import (
	"context"
	"strings"
	"testing"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func installTemplate(name, image, folder, namespace string) v1alpha1.Template {
	return v1alpha1.Template{
		Name: name,
		Container: &corev1.Container{
			Image: image,
			Args:  []string{"-folder=" + folder, "-namespace=" + namespace, "-create-namespace", "-wait"},
		},
	}
}

func faultsTemplate(engineYAMLs ...string) v1alpha1.Template {
	artifacts := make(v1alpha1.Artifacts, 0, len(engineYAMLs))
	for i, data := range engineYAMLs {
		artifacts = append(artifacts, v1alpha1.Artifact{
			Name:             "fault-" + string(rune('a'+i)),
			ArtifactLocation: v1alpha1.ArtifactLocation{Raw: &v1alpha1.RawArtifact{Data: data}},
		})
	}
	return v1alpha1.Template{
		Name:   "install-chaos-faults",
		Inputs: v1alpha1.Inputs{Artifacts: artifacts},
	}
}

func engineYAML(faultName string) string {
	return "kind: ChaosEngine\nmetadata:\n  name: " + faultName + "\nspec:\n  experiments:\n    - name: " + faultName + "\n"
}

func testCtx() context.Context { return context.Background() }

// A draft with no faults and no application is a legitimate save.
func TestEmptyDraftIsAllowed(t *testing.T) {
	svc := &chaosExperimentService{}
	if err := svc.validateExperimentComposition(testCtx(), "p1", &v1alpha1.WorkflowSpec{}); err != nil {
		t.Errorf("empty draft should be savable, got: %v", err)
	}
}

// A stock LitmusChaos workflow imported from a hub has faults but no ACE agent
// and no appNamespace reference. It must keep working.
func TestPlainLitmusWorkflowIsAllowed(t *testing.T) {
	svc := &chaosExperimentService{}
	spec := &v1alpha1.WorkflowSpec{
		Templates: []v1alpha1.Template{faultsTemplate(engineYAML("pod-delete"))},
	}
	if err := svc.validateExperimentComposition(testCtx(), "p1", spec); err != nil {
		t.Errorf("a workflow needing no application should be savable, got: %v", err)
	}
}

// The case that produced the Argo rejection: an agent is installed but nothing
// declares the namespace it is installed into.
func TestAgentWithoutApplicationIsRejected(t *testing.T) {
	svc := &chaosExperimentService{}
	spec := &v1alpha1.WorkflowSpec{
		Templates: []v1alpha1.Template{
			installTemplate("install-agent", "agentcert/agentcert-install-agent:latest", "sre-agent", "sock-shop"),
			faultsTemplate(engineYAML("pod-delete")),
		},
	}
	err := svc.validateExperimentComposition(testCtx(), "p1", spec)
	if err == nil {
		t.Fatal("expected rejection when an agent is installed without an application")
	}
	if !strings.Contains(err.Error(), "installs no application") {
		t.Errorf("error should name the missing step, got: %v", err)
	}
}

// The other half of the same failure: a template interpolates appNamespace but
// no install-application step can supply it.
func TestAppNamespaceReferenceWithoutApplicationIsRejected(t *testing.T) {
	svc := &chaosExperimentService{}
	spec := &v1alpha1.WorkflowSpec{
		Templates: []v1alpha1.Template{
			{
				Name:      "some-step",
				Container: &corev1.Container{Image: "busybox", Args: []string{"-n={{workflow.parameters.appNamespace}}"}},
			},
		},
	}
	if err := svc.validateExperimentComposition(testCtx(), "p1", spec); err == nil {
		t.Fatal("expected rejection when appNamespace is referenced but unresolvable")
	}
}

func TestInstallApplicationWithoutNamespaceIsRejected(t *testing.T) {
	svc := &chaosExperimentService{}
	spec := &v1alpha1.WorkflowSpec{
		Templates: []v1alpha1.Template{
			installTemplate("install-application", "agentcert/agentcert-install-app:latest", "sock-shop", ""),
			installTemplate("install-agent", "agentcert/agentcert-install-agent:latest", "sre-agent", "sock-shop"),
		},
	}
	if err := svc.validateExperimentComposition(testCtx(), "p1", spec); err == nil {
		t.Fatal("expected rejection when -namespace= is absent")
	}
}

// appNamespace is a single scalar workflow parameter, so two applications
// cannot both be represented.
func TestMultipleInstallApplicationsAreRejected(t *testing.T) {
	svc := &chaosExperimentService{}
	spec := &v1alpha1.WorkflowSpec{
		Templates: []v1alpha1.Template{
			installTemplate("install-application", "agentcert/agentcert-install-app:latest", "sock-shop", "sock-shop"),
			installTemplate("install-application-2", "agentcert/agentcert-install-app:latest", "bookinfo", "book-info"),
			faultsTemplate(engineYAML("pod-delete")),
		},
	}
	err := svc.validateExperimentComposition(testCtx(), "p1", spec)
	if err == nil {
		t.Fatal("expected rejection for two install-application steps")
	}
	if !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("error should explain the single-application invariant, got: %v", err)
	}
}

// A teardown step reuses the install image; counting it as a second application
// would reject a valid workflow.
func TestTeardownStepIsNotAnInstallStep(t *testing.T) {
	deleteStep := installTemplate("delete-application", "agentcert/agentcert-install-app:latest", "sock-shop", "sock-shop")
	deleteStep.Container.Args = append(deleteStep.Container.Args, "-operation=delete")

	if isInstallStepTemplate(deleteStep, "application") {
		t.Error("a -operation=delete step must not count as install-application")
	}

	svc := &chaosExperimentService{}
	spec := &v1alpha1.WorkflowSpec{
		Templates: []v1alpha1.Template{
			installTemplate("install-application", "agentcert/agentcert-install-app:latest", "sock-shop", "sock-shop"),
			deleteStep,
			faultsTemplate(engineYAML("pod-delete")),
		},
	}
	if err := svc.validateExperimentComposition(testCtx(), "p1", spec); err != nil {
		t.Errorf("install + teardown is one application, got: %v", err)
	}
}

func TestInstallAgentWithoutFolderIsRejected(t *testing.T) {
	svc := &chaosExperimentService{}
	agent := installTemplate("install-agent", "agentcert/agentcert-install-agent:latest", "", "sock-shop")
	agent.Container.Args = []string{"-namespace=sock-shop"}
	spec := &v1alpha1.WorkflowSpec{
		Templates: []v1alpha1.Template{
			installTemplate("install-application", "agentcert/agentcert-install-app:latest", "sock-shop", "sock-shop"),
			agent,
			faultsTemplate(engineYAML("pod-delete")),
		},
	}
	err := svc.validateExperimentComposition(testCtx(), "p1", spec)
	if err == nil {
		t.Fatal("expected rejection when install-agent has no -folder=")
	}
	if !strings.Contains(err.Error(), "agentFolder") {
		t.Errorf("error should name the unresolvable parameter, got: %v", err)
	}
}

// A structurally valid experiment passes even with no catalogue available: the
// compatibility checks are advisory, the structural ones are the gate.
func TestValidCompositionPassesWithoutCatalog(t *testing.T) {
	svc := &chaosExperimentService{}
	spec := &v1alpha1.WorkflowSpec{
		Templates: []v1alpha1.Template{
			installTemplate("install-application", "agentcert/agentcert-install-app:latest", "sock-shop", "sock-shop"),
			installTemplate("install-agent", "agentcert/agentcert-install-agent:latest", "sre-agent", "sock-shop"),
			faultsTemplate(engineYAML("pod-delete")),
		},
	}
	if err := svc.validateExperimentComposition(testCtx(), "p1", spec); err != nil {
		t.Errorf("valid composition should pass, got: %v", err)
	}
}

func TestChaosFaultNamesReadsEnginesOnly(t *testing.T) {
	templates := []v1alpha1.Template{
		faultsTemplate(
			engineYAML("pod-delete"),
			"kind: ChaosExperiment\nmetadata:\n  name: pod-delete\n", // staged CR, not a fault instance
			engineYAML("pod-cpu-hog"),
		),
	}
	got, err := chaosFaultNames(templates)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(got) != 2 || got[0] != "pod-delete" || got[1] != "pod-cpu-hog" {
		t.Errorf("want [pod-delete pod-cpu-hog], got %v", got)
	}
}

func TestInstallStepArgAcceptsBothForms(t *testing.T) {
	if got := installStepArg([]string{"-folder=sock-shop"}, "folder"); got != "sock-shop" {
		t.Errorf("combined form: got %q", got)
	}
	if got := installStepArg([]string{"--folder", "sock-shop"}, "folder"); got != "sock-shop" {
		t.Errorf("split form: got %q", got)
	}
	if got := installStepArg([]string{"-wait"}, "folder"); got != "" {
		t.Errorf("absent flag should yield empty, got %q", got)
	}
}

func targetedEngineYAML(faultName, namespace, kind, label string) string {
	return "kind: ChaosEngine\nmetadata:\n  name: " + faultName +
		"\nspec:\n  appinfo:\n    appns: \"" + namespace + "\"" +
		"\n    appkind: " + kind +
		"\n    applabel: " + label +
		"\n  experiments:\n    - name: " + faultName + "\n"
}

func TestMalformedChaosArtifactIsRejected(t *testing.T) {
	spec := &v1alpha1.WorkflowSpec{Templates: []v1alpha1.Template{
		faultsTemplate("kind: ChaosEngine\nspec: ["),
	}}
	if err := ValidateExperimentStructure(spec); err == nil || !strings.Contains(err.Error(), "invalid chaos artifact") {
		t.Fatalf("expected actionable parse error, got %v", err)
	}
}

func TestIncompleteFaultTargetIsRejected(t *testing.T) {
	spec := &v1alpha1.WorkflowSpec{Templates: []v1alpha1.Template{
		installTemplate("install-application", "agentcert/agentcert-install-app:latest", "sock-shop", "sock-shop"),
		faultsTemplate(targetedEngineYAML("pod-delete", "sock-shop", "deployment", "")),
	}}
	if err := ValidateExperimentStructure(spec); err == nil || !strings.Contains(err.Error(), "incomplete spec.appinfo") {
		t.Fatalf("expected incomplete appinfo error, got %v", err)
	}
}

func TestFaultTargetNamespaceMustMatchApplication(t *testing.T) {
	spec := &v1alpha1.WorkflowSpec{Templates: []v1alpha1.Template{
		installTemplate("install-application", "agentcert/agentcert-install-app:latest", "sock-shop", "sock-shop"),
		faultsTemplate(targetedEngineYAML("pod-delete", "bookinfo", "deployment", "name=carts")),
	}}
	if err := ValidateExperimentStructure(spec); err == nil || !strings.Contains(err.Error(), "targets namespace") {
		t.Fatalf("expected namespace mismatch error, got %v", err)
	}
}

func TestFaultTargetMayUseWorkflowNamespaceParameter(t *testing.T) {
	spec := &v1alpha1.WorkflowSpec{Templates: []v1alpha1.Template{
		installTemplate("install-application", "agentcert/agentcert-install-app:latest", "sock-shop", "sock-shop"),
		faultsTemplate(targetedEngineYAML("pod-delete", appNamespaceRef, "deployment", "name=carts")),
	}}
	if err := ValidateExperimentStructure(spec); err != nil {
		t.Fatalf("expected parameterized target to pass, got %v", err)
	}
}

func TestTeardownIsNotCountedAsCertifiableFault(t *testing.T) {
	got, err := chaosFaultNames([]v1alpha1.Template{
		faultsTemplate(engineYAML("pod-delete"), engineYAML("uninstall-agent")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "pod-delete" {
		t.Fatalf("expected only the injected fault, got %v", got)
	}
}

func TestExtractInstallAgentNamespaceAcceptsCombinedAndSplitForms(t *testing.T) {
	combined := installTemplate("install-agent", "agentcert/agentcert-install-agent:latest", "flash-agent", "sock-shop")
	if got := ExtractInstallAgentNamespace([]v1alpha1.Template{combined}); got != "sock-shop" {
		t.Fatalf("combined namespace: got %q", got)
	}
	split := combined
	split.Container = split.Container.DeepCopy()
	split.Container.Args = []string{"--folder", "flash-agent", "--namespace", "bookinfo"}
	if got := ExtractInstallAgentNamespace([]v1alpha1.Template{split}); got != "bookinfo" {
		t.Fatalf("split namespace: got %q", got)
	}
}
