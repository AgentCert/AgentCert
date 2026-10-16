package ops

import (
	"strings"
	"testing"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func cleanupInstallTemplate(name, kind, folder, namespace, release string) v1alpha1.Template {
	args := []string{"-folder=" + folder, "-namespace=" + namespace}
	if release != "" {
		args = append(args, "-release="+release)
	}
	return v1alpha1.Template{
		Name: name,
		Metadata: v1alpha1.Metadata{Annotations: map[string]string{
			"agentcert.io/install-type": kind,
		}},
		Container: &corev1.Container{Image: "installer", Args: args},
	}
}

func TestGuaranteedCleanupUsesOnExitAndExactReleases(t *testing.T) {
	spec := &v1alpha1.WorkflowSpec{
		Entrypoint: "root",
		Templates: []v1alpha1.Template{
			{Name: "root", Steps: []v1alpha1.ParallelSteps{{Steps: []v1alpha1.WorkflowStep{
				{Name: uninstallAllTemplateName, Template: uninstallAllTemplateName},
			}}}},
			cleanupInstallTemplate("install-application", "application", "sock-shop", "shop-ns", "shop-release"),
			cleanupInstallTemplate("install-agent", "agent", "flash-agent", "agent-system", "flash-release"),
		},
	}

	if err := ApplyGuaranteedCleanupPatch(spec); err != nil {
		t.Fatal(err)
	}
	if spec.OnExit != uninstallAllTemplateName {
		t.Fatalf("onExit=%q, want %q", spec.OnExit, uninstallAllTemplateName)
	}
	if len(spec.Templates[0].Steps) != 0 {
		t.Fatalf("legacy normal cleanup step was not removed: %#v", spec.Templates[0].Steps)
	}

	var cleanup *v1alpha1.Template
	for i := range spec.Templates {
		if spec.Templates[i].Name == uninstallAllTemplateName {
			cleanup = &spec.Templates[i]
		}
	}
	if cleanup == nil || cleanup.Container == nil {
		t.Fatal("cleanup template was not created")
	}
	env := map[string]string{}
	for _, item := range cleanup.Container.Env {
		env[item.Name] = item.Value
	}
	if env["APP_RELEASE"] != "shop-release" || env["AGENT_RELEASE"] != "flash-release" {
		t.Fatalf("cleanup guessed release names: %#v", env)
	}
	if strings.Contains(cleanup.Container.Args[0], "delete namespace") ||
		strings.Contains(cleanup.Container.Args[0], "--all") ||
		!strings.Contains(cleanup.Container.Args[0], "workflow_run_id") {
		t.Fatalf("cleanup is not run-scoped and non-destructive: %s", cleanup.Container.Args[0])
	}
}

func TestGuaranteedCleanupPreservesExistingOnExit(t *testing.T) {
	spec := &v1alpha1.WorkflowSpec{
		OnExit: "existing-exit",
		Templates: []v1alpha1.Template{
			{Name: "existing-exit", Container: &corev1.Container{Image: "busybox"}},
			cleanupInstallTemplate("install-application", "application", "sock-shop", "shop-ns", ""),
			cleanupInstallTemplate("install-agent", "agent", "flash-agent", "shop-ns", ""),
		},
	}
	if err := ApplyGuaranteedCleanupPatch(spec); err != nil {
		t.Fatal(err)
	}
	if spec.OnExit != "ace-on-exit" {
		t.Fatalf("existing onExit was not wrapped: %q", spec.OnExit)
	}
}

func TestGuaranteedCleanupSkipsWorkflowsWithoutAgent(t *testing.T) {
	spec := &v1alpha1.WorkflowSpec{Templates: []v1alpha1.Template{
		cleanupInstallTemplate("install-application", "application", "sock-shop", "shop-ns", ""),
	}}
	if err := ApplyGuaranteedCleanupPatch(spec); err != nil {
		t.Fatal(err)
	}
	if spec.OnExit != "" {
		t.Fatalf("plain Litmus workflow unexpectedly changed: %q", spec.OnExit)
	}
}
