package ops

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chartconfig"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"
	corev1 "k8s.io/api/core/v1"
)

const testAgentValues = `
agent:
  config:
    SCAN_INTERVAL: "60"
    MODEL_ALIAS: "gpt-4o"
configurations:
  - key: agent.config.SCAN_INTERVAL
    type: integer
    min: 1
  - key: agent.config.MODEL_ALIAS
    type: model
`

const testAppValues = `
bookInfo:
  productpage:
    replicas: 1
configurations:
  - key: bookInfo.productpage.replicas
    type: integer
    min: 1
    max: 5
`

// withTestHubs points the agent and app hubs at temporary chart directories
// holding flash-agent and bookinfo.
func withTestHubs(t *testing.T) {
	t.Helper()
	agentHub, appHub := t.TempDir(), t.TempDir()
	writeChart(t, filepath.Join(agentHub, "charts", "flash-agent"), testAgentValues)
	writeChart(t, filepath.Join(appHub, "charts", "bookinfo"), testAppValues)

	prevAgent, prevApp := utils.Config.DefaultAgentHubPath, utils.Config.DefaultAppHubPath
	utils.Config.DefaultAgentHubPath, utils.Config.DefaultAppHubPath = agentHub, appHub
	t.Cleanup(func() {
		utils.Config.DefaultAgentHubPath, utils.Config.DefaultAppHubPath = prevAgent, prevApp
	})
}

func writeChart(t *testing.T, dir, values string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "values.yaml"), []byte(values), 0o644); err != nil {
		t.Fatal(err)
	}
}

func withValues(t v1alpha1.Template, valuesJSON string) v1alpha1.Template {
	t.Container.Args = append(t.Container.Args, "-values-json="+valuesJSON)
	return t
}

func configuredSpec(appValues, agentValues string) *v1alpha1.WorkflowSpec {
	app := installTemplate("install-application", "agentcert/agentcert-install-app:latest", "bookinfo", "book-info")
	agent := installTemplate("install-agent", "agentcert/agentcert-install-agent:latest", "flash-agent", "book-info")
	if appValues != "" {
		app = withValues(app, appValues)
	}
	if agentValues != "" {
		agent = withValues(agent, agentValues)
	}
	return &v1alpha1.WorkflowSpec{Templates: []v1alpha1.Template{app, agent}}
}

func TestValidSettingsPassValidation(t *testing.T) {
	withTestHubs(t)
	spec := configuredSpec(`{"bookInfo":{"productpage":{"replicas":3}}}`, `{"agent":{"config":{"SCAN_INTERVAL":"90"}}}`)
	if err := ValidateExperimentStructure(spec); err != nil {
		t.Fatalf("ValidateExperimentStructure() error = %v", err)
	}
}

func TestStepsWithoutSettingsSkipTheChartLookup(t *testing.T) {
	// No test hubs: a lookup would fail, so passing proves none happened.
	if err := ValidateExperimentStructure(configuredSpec("", "")); err != nil {
		t.Fatalf("ValidateExperimentStructure() error = %v", err)
	}
}

func TestInjectedKeysArePlatformOwnedAndNotDuplicated(t *testing.T) {
	step := installAgentStep("flash-agent", "sock-shop")
	step.Container.Args = append(step.Container.Args, "-set=agent.config.AGENT_ID=stale", "--server-addr=stale")
	templates := []v1alpha1.Template{step}
	InjectExperimentContextArgs(templates, "")
	InjectExperimentContextArgs(templates, "")
	seen := map[string]bool{}
	args := templates[0].Container.Args
	for i, arg := range args {
		if strings.Contains(arg, "=stale") {
			t.Fatalf("stale context survived: %s", arg)
		}
		if arg != "--set" || i+1 == len(args) {
			continue
		}
		key, _, _ := strings.Cut(args[i+1], "=")
		if !chartconfig.IsPlatformOwned(key) {
			t.Errorf("injected key %s is not protected", key)
		}
		if seen[key] {
			t.Errorf("injected key %s is duplicated", key)
		}
		seen[key] = true
	}
}

func TestInvalidSettingsAreRejected(t *testing.T) {
	withTestHubs(t)
	cases := []struct {
		name, appValues, agentValues, want string
	}{
		{"agent value out of range", "", `{"agent":{"config":{"SCAN_INTERVAL":"0"}}}`, "install-agent step's settings for \"flash-agent\""},
		{"undeclared agent value", "", `{"agent":{"containerImage":{"tag":"evil"}}}`, "is not a setting this chart offers"},
		{"app value out of range", `{"bookInfo":{"productpage":{"replicas":9}}}`, "", "install-application step's settings for \"bookinfo\""},
		{"malformed document", "", `{"agent":`, "not a JSON object"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateExperimentStructure(configuredSpec(tc.appValues, tc.agentValues))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateExperimentStructure() error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestSettingsForAnUnknownChartFailClosed(t *testing.T) {
	withTestHubs(t)
	spec := configuredSpec("", `{"agent":{"config":{"SCAN_INTERVAL":"90"}}}`)
	spec.Templates[1].Container.Args[0] = "-folder=custom-agent"
	err := ValidateExperimentStructure(spec)
	if err == nil || !strings.Contains(err.Error(), "not in the hub") {
		t.Fatalf("ValidateExperimentStructure() error = %v, want the unknown chart rejected", err)
	}
}

func TestSettingsCannotPointTheChartLookupOutsideTheHub(t *testing.T) {
	withTestHubs(t)
	spec := configuredSpec("", `{"agent":{"config":{"SCAN_INTERVAL":"90"}}}`)
	spec.Templates[1].Container.Args[0] = "-folder=../../etc"
	err := ValidateExperimentStructure(spec)
	if err == nil || !strings.Contains(err.Error(), "does not name a chart") {
		t.Fatalf("ValidateExperimentStructure() error = %v, want the path rejected", err)
	}
}

func TestSettingsResolveATemplatedAgentFolder(t *testing.T) {
	withTestHubs(t)
	spec := configuredSpec("", `{"agent":{"config":{"SCAN_INTERVAL":"0"}}}`)
	spec.Templates[1].Container.Args[0] = "-folder={{workflow.parameters.agentFolder}}"
	spec.Arguments = v1alpha1.Arguments{Parameters: []v1alpha1.Parameter{{Name: "agentFolder", Value: v1alpha1.AnyStringPtr("flash-agent")}}}
	err := ValidateExperimentStructure(spec)
	if err == nil || !strings.Contains(err.Error(), "at least 1") {
		t.Fatalf("ValidateExperimentStructure() error = %v, want flash-agent's range check", err)
	}
}

func setArgValues(args []string, key string) []string {
	var values []string
	for i, arg := range args {
		if arg == "--set" && i+1 < len(args) && strings.HasPrefix(args[i+1], key+"=") {
			values = append(values, strings.TrimPrefix(args[i+1], key+"="))
		}
		if strings.HasPrefix(arg, "--set="+key+"=") {
			values = append(values, strings.TrimPrefix(arg, "--set="+key+"="))
		}
	}
	return values
}

func TestAgentModelPrecedence(t *testing.T) {
	t.Setenv("FLASH_AGENT_MODEL", "platform-model")
	configured := `{"agent":{"config":{"MODEL_ALIAS":"experiment-model"}}}`
	cases := []struct {
		name, values, override, want string
	}{
		{"platform default", "", "", "platform-model"},
		{"experiment setting beats the platform default", configured, "", "experiment-model"},
		{"run-time pick beats the experiment setting", configured, "run-model", "run-model"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			step := installAgentStep("flash-agent", "sock-shop")
			if tc.values != "" {
				step = withValues(step, tc.values)
			}
			// A model written by an earlier save must be replaced, not duplicated.
			step.Container.Args = append(step.Container.Args, "--set", "agent.config.MODEL_ALIAS=stale")
			templates := []v1alpha1.Template{step}

			InjectExperimentContextArgs(templates, tc.override)

			got := setArgValues(templates[0].Container.Args, "agent.config.MODEL_ALIAS")
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("MODEL_ALIAS --set values = %v, want exactly [%s]", got, tc.want)
			}
		})
	}
}

func TestInjectionKeepsUserSettingsAndUnrelatedSetArgs(t *testing.T) {
	values := `{"agent":{"config":{"SCAN_INTERVAL":"90"}}}`
	step := withValues(installAgentStep("flash-agent", "sock-shop"), values)
	step.Container.Args = append(step.Container.Args,
		"--set=agent.config.SCAN_INTERVAL={{workflow.parameters.scanInterval}}",
		"--set=agent.config.SCAN_QUERY=Keep this instruction",
		"--set=agent.config.AGENT_MODE=active",
	)
	templates := []v1alpha1.Template{step}

	InjectExperimentContextArgs(templates, "")
	InjectExperimentContextArgs(templates, "") // run-time re-injection over the saved manifest

	args := templates[0].Container.Args
	if installStepArg(args, valuesJSONFlag) != values {
		t.Fatalf("-values-json was not preserved: %v", args)
	}
	if got := setArgValues(args, "agent.config.SCAN_INTERVAL"); len(got) != 0 {
		t.Fatalf("a template --set must not override the chosen setting, got %v", got)
	}
	if got := setArgValues(args, "agent.config.SCAN_QUERY"); len(got) != 1 {
		t.Fatalf("an unrelated --set must survive untouched, got %v", got)
	}
	if got := setArgValues(args, "agent.config.AGENT_MODE"); len(got) != 1 || got[0] == "active" {
		t.Fatalf("a platform-owned --set must be replaced by the server's value, got %v", got)
	}
}

func TestSettingsUseTheFinalInstallerFlagValue(t *testing.T) {
	withTestHubs(t)
	spec := configuredSpec("", `{"agent":{"config":{"SCAN_INTERVAL":"90"}}}`)
	spec.Templates[1].Container.Args = append(spec.Templates[1].Container.Args,
		"--values-json", `{"agent":{"config":{"SCAN_INTERVAL":"0"}}}`,
	)
	if err := ValidateExperimentStructure(spec); err == nil {
		t.Fatal("the invalid final settings document must be rejected")
	}
	if got := installStepArg([]string{"-folder=old", "--folder", "new"}, "folder"); got != "new" {
		t.Fatalf("folder = %q, want new", got)
	}
}

func TestSavedSettingsSurviveWorkflowAndCronRuns(t *testing.T) {
	withTestHubs(t)
	service := chaosExperimentRunTestService.(*chaosExperimentService)
	t.Setenv("FLASH_AGENT_MODEL", "platform-model")
	const appValues = `{"bookInfo":{"productpage":{"replicas":3}}}`
	const agentValues = `{"agent":{"config":{"SCAN_INTERVAL":"90","MODEL_ALIAS":"experiment-model"}}}`
	for _, kind := range []string{"Workflow", "CronWorkflow"} {
		t.Run(kind, func(t *testing.T) {
			spec := configuredSpec(appValues, agentValues)
			spec.Entrypoint = "main"
			spec.Templates = append(spec.Templates, v1alpha1.Template{Name: "main", Steps: []v1alpha1.ParallelSteps{
				{Steps: []v1alpha1.WorkflowStep{{Name: "install-application", Template: "install-application"}}},
				{Steps: []v1alpha1.WorkflowStep{{Name: "install-agent", Template: "install-agent"}}},
			}})
			spec.Templates[0].Container.Args = append(spec.Templates[0].Container.Args, "--set", "bookInfo.productpage.replicas=1")
			spec.Templates[1].Container.Args = append(spec.Templates[1].Container.Args, "--set=agent.config.SCAN_INTERVAL=60")
			experimentID := "configured-experiment"
			request := &model.ChaosExperimentRequest{ExperimentID: &experimentID, InfraID: "infra"}
			wf := v1alpha1.Workflow{Spec: *spec}
			wf.Annotations = map[string]string{RunModelAliasAnnotation: "old-run-model"}
			if kind == "Workflow" {
				data, _ := json.Marshal(wf)
				request.ExperimentManifest = string(data)
				if err := service.processExperimentManifest(context.Background(), request, nil, "rev", "project"); err != nil {
					t.Fatal(err)
				}
				wf = v1alpha1.Workflow{}
				if err := json.Unmarshal([]byte(request.ExperimentManifest), &wf); err != nil {
					t.Fatal(err)
				}
				spec = &wf.Spec
				if _, ok := wf.Annotations[RunModelAliasAnnotation]; ok {
					t.Fatal("old run model survived save")
				}
			} else {
				cron := v1alpha1.CronWorkflow{ObjectMeta: wf.ObjectMeta, Spec: v1alpha1.CronWorkflowSpec{Schedule: "0 * * * *", WorkflowSpec: *spec}}
				cron.Spec.WorkflowMetadata = wf.ObjectMeta.DeepCopy()
				data, _ := json.Marshal(cron)
				request.ExperimentManifest = string(data)
				if err := service.processCronExperimentManifest(context.Background(), request, nil, "rev", "project"); err != nil {
					t.Fatal(err)
				}
				cron = v1alpha1.CronWorkflow{}
				if err := json.Unmarshal([]byte(request.ExperimentManifest), &cron); err != nil {
					t.Fatal(err)
				}
				spec = &cron.Spec.WorkflowSpec
				if _, ok := cron.Annotations[RunModelAliasAnnotation]; ok {
					t.Fatal("old cron run model survived save")
				}
				if _, ok := cron.Spec.WorkflowMetadata.Annotations[RunModelAliasAnnotation]; ok {
					t.Fatal("old child workflow model survived save")
				}
			}
			// Manual re-runs and multi-run dispatch reprocess this same saved spec.
			for run := 0; run < 3; run++ {
				if err := ValidateExperimentStructure(spec); err != nil {
					t.Fatal(err)
				}
				if kind == "CronWorkflow" {
					InjectCronExperimentContextArgs(spec.Templates, "")
				} else {
					InjectExperimentContextArgs(spec.Templates, "")
				}
				for _, step := range spec.Templates {
					switch installStepKind(step) {
					case "application":
						if got := installStepArg(step.Container.Args, valuesJSONFlag); got != appValues {
							t.Fatalf("app settings = %s", got)
						}
						if len(setArgValues(step.Container.Args, "bookInfo.productpage.replicas")) != 0 {
							t.Fatal("template overrides app setting")
						}
					case "agent":
						if got := installStepArg(step.Container.Args, valuesJSONFlag); got != agentValues {
							t.Fatalf("agent settings = %s", got)
						}
						got := setArgValues(step.Container.Args, "agent.config.MODEL_ALIAS")
						if len(got) != 1 || got[0] != "experiment-model" {
							t.Fatalf("run %d model = %v", run, got)
						}
					}
				}
			}
		})
	}
}

func TestAgentStepHasSettings(t *testing.T) {
	plain := []v1alpha1.Template{installAgentStep("flash-agent", "sock-shop")}
	if agentStepHasSettings(plain) {
		t.Fatal("a step without -values-json has no settings")
	}
	configured := []v1alpha1.Template{withValues(installAgentStep("flash-agent", "sock-shop"), `{}`)}
	if !agentStepHasSettings(configured) {
		t.Fatal("a step with -values-json has settings")
	}
	app := v1alpha1.Template{Name: "install-application", Container: &corev1.Container{Args: []string{"-folder=bookinfo", "-values-json={}"}}}
	if agentStepHasSettings([]v1alpha1.Template{app}) {
		t.Fatal("application settings do not configure the agent")
	}
}

func TestResolveWorkflowParameter(t *testing.T) {
	args := v1alpha1.Arguments{Parameters: []v1alpha1.Parameter{{Name: "agentFolder", Value: v1alpha1.AnyStringPtr("flash-agent")}}}
	cases := map[string]string{
		"{{workflow.parameters.agentFolder}}":        "flash-agent",
		"{{ workflow.parameters.agentFolder }}":      "flash-agent",
		"{{workflow.parameters.missing}}":            "{{workflow.parameters.missing}}",
		"flash-agent":                                "flash-agent",
		"prefix-{{workflow.parameters.agentFolder}}": "prefix-{{workflow.parameters.agentFolder}}",
	}
	for in, want := range cases {
		if got := resolveWorkflowParameter(in, args); got != want {
			t.Errorf("resolveWorkflowParameter(%q) = %q, want %q", in, got, want)
		}
	}
}
