package ops

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/agenthub"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/apphub"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chartconfig"
)

// valuesJSONFlag is the install-agent / install-app flag carrying the chart
// settings chosen in the experiment builder, as one JSON values document. The
// installers hand it to Helm as a values file, ahead of the platform's --set
// args, so precedence is: chart default < user setting < platform value.
const valuesJSONFlag = "values-json"

// RunModelAliasAnnotation is where a run's Chaos Studio model pick is
// persisted on the experiment manifest, so the backend-triggered runs of a
// multi-run batch reuse it.
const RunModelAliasAnnotation = "litmuschaos.io/modelAlias"

const errInstallStepSettings = "the %s step's settings for %q are invalid: %w"

var workflowParameterRef = regexp.MustCompile(`^\{\{\s*workflow\.parameters\.([A-Za-z0-9_-]+)\s*\}\}$`)

// validateInstallStepValues checks every install step's -values-json against
// the settings its chart declares. It fails closed: values for a chart that
// cannot be found, or that declares no such setting, are rejected rather than
// passed to Helm, because an undeclared value could swap the agent's image or
// widen its RBAC.
func validateInstallStepValues(spec *v1alpha1.WorkflowSpec) error {
	for _, t := range spec.Templates {
		kind := installStepKind(t)
		if kind == "" {
			continue
		}
		raw := installStepArg(t.Container.Args, valuesJSONFlag)
		if raw == "" {
			continue
		}
		folder := resolveWorkflowParameter(installStepArg(t.Container.Args, "folder"), spec.Arguments)
		if err := validateChartValues(kind, folder, raw); err != nil {
			return fmt.Errorf(errInstallStepSettings, t.Name, folder, err)
		}
	}
	return nil
}

func validateChartValues(kind, folder, raw string) error {
	if !isChartFolderName(folder) {
		return fmt.Errorf("-folder=%q does not name a chart", folder)
	}
	fields, err := chartconfig.Load(filepath.Join(chartsPathFor(kind), folder))
	if errors.Is(err, chartconfig.ErrChartNotFound) {
		return errors.New("the chart is not in the hub, so its settings cannot be checked")
	}
	if err != nil {
		return err
	}
	values, err := chartconfig.ParseValues(raw)
	if err != nil {
		return err
	}
	return chartconfig.Validate(fields, values)
}

// installStepKind returns "agent" or "application" for an install step, else "".
func installStepKind(t v1alpha1.Template) string {
	switch {
	case isInstallStepTemplate(t, "agent"):
		return "agent"
	case isInstallStepTemplate(t, "application"):
		return "application"
	}
	return ""
}

func chartsPathFor(kind string) string {
	if kind == "agent" {
		return agenthub.ChartsPath()
	}
	return apphub.ChartsPath()
}

// isChartFolderName rejects anything that is not a single directory name, so
// a crafted -folder= cannot point the chart lookup outside the hub.
func isChartFolderName(folder string) bool {
	return folder != "" && folder != "." && folder != ".." && !strings.ContainsAny(folder, `/\`)
}

// resolveWorkflowParameter replaces a whole-value {{workflow.parameters.X}}
// reference with the parameter's value. ChaosHub templates write the agent as
// -folder={{workflow.parameters.agentFolder}}.
func resolveWorkflowParameter(value string, args v1alpha1.Arguments) string {
	match := workflowParameterRef.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return value
	}
	for _, p := range args.Parameters {
		if p.Name == match[1] && p.Value != nil {
			return p.Value.String()
		}
	}
	return value
}

// EnsureInstallStepParameters supplies the parameters generated install and
// cleanup arguments reference, for saved workflows and older stored revisions.
func EnsureInstallStepParameters(spec *v1alpha1.WorkflowSpec) {
	if spec == nil {
		return
	}
	ensure := func(name, value string) {
		if strings.Contains(value, "{{") {
			return
		}
		for i, parameter := range spec.Arguments.Parameters {
			if parameter.Name == name {
				if parameter.Value == nil {
					spec.Arguments.Parameters[i].Value = v1alpha1.AnyStringPtr(value)
				}
				return
			}
		}
		spec.Arguments.Parameters = append(spec.Arguments.Parameters, v1alpha1.Parameter{Name: name, Value: v1alpha1.AnyStringPtr(value)})
	}
	for _, template := range spec.Templates {
		switch installStepKind(template) {
		case "application":
			if namespace := resolveWorkflowParameter(installStepArg(template.Container.Args, "namespace"), spec.Arguments); namespace != "" {
				ensure("appNamespace", namespace)
			}
		case "agent":
			if folder := resolveWorkflowParameter(installStepArg(template.Container.Args, "folder"), spec.Arguments); folder != "" {
				ensure("agentFolder", folder)
			}
			ensure("agentId", "") // install-agent registers itself when no ID is supplied.
		}
	}
}

// Scheduled child workflows have no manual-run notify_id label. Their UID
// is the run's trace key, matching the subscriber's scheduled-run events.
func InjectCronExperimentContextArgs(templates []v1alpha1.Template, modelAliasOverride string) {
	InjectExperimentContextArgs(templates, modelAliasOverride)
	for i := range templates {
		if !IsAgentInstallStep(templates[i]) {
			continue
		}
		for j, arg := range templates[i].Container.Args {
			templates[i].Container.Args[j] = strings.ReplaceAll(arg, "{{workflow.labels.notify_id}}", "{{workflow.uid}}")
		}
	}
}

// configuredAgentValue returns the user's setting for key on an agent install
// step, if its -values-json sets one. Values are validated before any patch
// runs, so a document that fails to parse here simply sets nothing.
func configuredAgentValue(t v1alpha1.Template, key string) (string, bool) {
	if t.Container == nil {
		return "", false
	}
	raw := installStepArg(t.Container.Args, valuesJSONFlag)
	if raw == "" {
		return "", false
	}
	values, err := chartconfig.ParseValues(raw)
	if err != nil {
		return "", false
	}
	return chartconfig.Value(values, key)
}

// applyInstallStepSettings removes template --set values that would override
// explicitly chosen settings. Save and run call this before platform context
// is injected, so platform-owned values still have the final say.
func applyInstallStepSettings(templates []v1alpha1.Template) {
	for i := range templates {
		t := &templates[i]
		if installStepKind(*t) == "" {
			continue
		}
		values, err := chartconfig.ParseValues(installStepArg(t.Container.Args, valuesJSONFlag))
		if err != nil {
			continue // Invalid documents are rejected by save/run validation.
		}
		args := t.Container.Args
		kept := make([]string, 0, len(args))
		for j := 0; j < len(args); j++ {
			arg := args[j]
			flag, assignment, combined := strings.Cut(arg, "=")
			if flag == "-set" || flag == "--set" || flag == "--set-string" || flag == "--set-json" {
				if !combined && j+1 < len(args) {
					assignment = args[j+1]
				}
				key, _, _ := strings.Cut(assignment, "=")
				if _, chosen := chartconfig.Value(values, key); chosen {
					if !combined {
						j++
					}
					continue
				}
			}
			kept = append(kept, arg)
		}
		t.Container.Args = kept
	}
}

// agentStepHasSettings reports whether the experiment's agent is configured
// through the builder's settings form, which then owns the agent's model.
func agentStepHasSettings(templates []v1alpha1.Template) bool {
	for _, t := range templates {
		if t.Container != nil && IsAgentInstallStep(t) && installStepArg(t.Container.Args, valuesJSONFlag) != "" {
			return true
		}
	}
	return false
}

// resolveAgentModelAlias picks the model an agent install step runs with:
// an explicit run-time pick > the experiment's own setting > the platform
// default. The run-time pick is the Chaos Studio "Agent model" selector, or
// the pick persisted from it for the later runs of a multi-run batch.
func resolveAgentModelAlias(runOverride string, t v1alpha1.Template, platformDefault string) string {
	if override := strings.TrimSpace(runOverride); override != "" {
		return override
	}
	if configured, ok := configuredAgentValue(t, chartconfig.ModelAliasKey); ok && strings.TrimSpace(configured) != "" {
		return configured
	}
	return platformDefault
}
