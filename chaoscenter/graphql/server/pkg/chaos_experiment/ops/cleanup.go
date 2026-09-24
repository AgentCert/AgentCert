package ops

import (
	"fmt"
	"os"
	"strings"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

const uninstallAllTemplateName = "uninstall-all"

// ApplyGuaranteedCleanupPatch installs an Argo onExit handler. Unlike a final
// normal step, onExit runs after success, failure, or cancellation.
func ApplyGuaranteedCleanupPatch(spec *v1alpha1.WorkflowSpec) error {
	if spec == nil || len(spec.Templates) == 0 {
		return nil
	}

	var appRelease, appNamespace, agentRelease, agentNamespace string
	for _, template := range spec.Templates {
		if template.Container == nil {
			continue
		}
		if isInstallStepTemplate(template, "application") {
			appRelease = installStepArg(template.Container.Args, "release")
			if appRelease == "" {
				appRelease = installStepArg(template.Container.Args, "folder")
			}
			appNamespace = installStepArg(template.Container.Args, "namespace")
		}
		if isInstallStepTemplate(template, "agent") {
			agentRelease = installStepArg(template.Container.Args, "release")
			if agentRelease == "" {
				agentRelease = installStepArg(template.Container.Args, "folder")
			}
			agentNamespace = installStepArg(template.Container.Args, "namespace")
		}
	}
	if agentRelease == "" {
		return nil
	}
	if appRelease == "" || appNamespace == "" {
		return fmt.Errorf("cannot create cleanup handler without application release and namespace")
	}
	if agentNamespace == "" {
		agentNamespace = strings.TrimSpace(os.Getenv("AGENT_INSTALL_NAMESPACE"))
	}
	if agentNamespace == "" {
		agentNamespace = appNamespace
	}

	// A stored revision may contain the legacy normal cleanup step. Remove that
	// invocation before making the existing template an onExit handler.
	for i := range spec.Templates {
		if len(spec.Templates[i].Steps) == 0 {
			continue
		}
		groups := make([]v1alpha1.ParallelSteps, 0, len(spec.Templates[i].Steps))
		for _, group := range spec.Templates[i].Steps {
			steps := group.Steps[:0]
			for _, step := range group.Steps {
				if step.Name != uninstallAllTemplateName && step.Template != uninstallAllTemplateName {
					steps = append(steps, step)
				}
			}
			if len(steps) > 0 {
				group.Steps = steps
				groups = append(groups, group)
			}
		}
		spec.Templates[i].Steps = groups
	}

	cleanupImage := strings.TrimSpace(utils.Config.InstallAgentImage)
	if cleanupImage == "" {
		cleanupImage = strings.TrimSpace(os.Getenv("INSTALL_AGENT_IMAGE"))
	}
	if cleanupImage == "" {
		cleanupImage = "agentcert/agentcert-install-agent:latest"
	}

	cleanupScript := `set -u
failures=0
run_cleanup() {
  echo "[uninstall-all] $*"
  "$@" || failures=$((failures + 1))
}
run_cleanup kubectl delete chaosengines.litmuschaos.io -n "$APP_NAMESPACE" -l "workflow_run_id=$WORKFLOW_RUN_ID" --ignore-not-found
run_cleanup kubectl delete chaosresults.litmuschaos.io -n "$APP_NAMESPACE" -l "workflow_run_id=$WORKFLOW_RUN_ID" --ignore-not-found
run_cleanup helm uninstall "$AGENT_RELEASE" -n "$AGENT_NAMESPACE" --ignore-not-found --wait --timeout 5m
run_cleanup helm uninstall "$APP_RELEASE" -n "$APP_NAMESPACE" --ignore-not-found --wait --timeout 5m
if [ "$failures" -ne 0 ]; then
  echo "[uninstall-all] cleanup completed with $failures failure(s)" >&2
  exit 1
fi
echo "[uninstall-all] cleanup completed"`

	cleanupTemplate := v1alpha1.Template{
		Name: uninstallAllTemplateName,
		Container: &corev1.Container{
			Image:   cleanupImage,
			Command: []string{"sh", "-c"},
			Args:    []string{cleanupScript},
			Env: []corev1.EnvVar{
				{Name: "APP_NAMESPACE", Value: appNamespace},
				{Name: "APP_RELEASE", Value: appRelease},
				{Name: "AGENT_NAMESPACE", Value: agentNamespace},
				{Name: "AGENT_RELEASE", Value: agentRelease},
				{Name: "WORKFLOW_RUN_ID", Value: "{{workflow.uid}}"},
			},
		},
	}

	templateIndex := -1
	for i := range spec.Templates {
		if spec.Templates[i].Name == uninstallAllTemplateName {
			templateIndex = i
			break
		}
	}
	if templateIndex >= 0 {
		spec.Templates[templateIndex] = cleanupTemplate
	} else {
		spec.Templates = append(spec.Templates, cleanupTemplate)
	}

	previousOnExit := strings.TrimSpace(spec.OnExit)
	switch previousOnExit {
	case "", uninstallAllTemplateName:
		spec.OnExit = uninstallAllTemplateName
	default:
		const wrapperName = "ace-on-exit"
		wrapper := v1alpha1.Template{
			Name: wrapperName,
			Steps: []v1alpha1.ParallelSteps{
				{Steps: []v1alpha1.WorkflowStep{{Name: uninstallAllTemplateName, Template: uninstallAllTemplateName}}},
				{Steps: []v1alpha1.WorkflowStep{{Name: "previous-on-exit", Template: previousOnExit}}},
			},
		}
		found := false
		for i := range spec.Templates {
			if spec.Templates[i].Name == wrapperName {
				spec.Templates[i] = wrapper
				found = true
				break
			}
		}
		if !found {
			spec.Templates = append(spec.Templates, wrapper)
		}
		spec.OnExit = wrapperName
	}

	spec.PodGC = &v1alpha1.PodGC{Strategy: v1alpha1.PodGCOnWorkflowCompletion}
	logrus.WithFields(logrus.Fields{
		"appRelease":     appRelease,
		"appNamespace":   appNamespace,
		"agentRelease":   agentRelease,
		"agentNamespace": agentNamespace,
	}).Info("[Uninstall All Patch] Configured guaranteed onExit cleanup")
	return nil
}
