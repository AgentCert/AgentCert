package ops

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const experimentModel = `{"agent":{"config":{"MODEL_ALIAS":"experiment-model"}}}`

func TestResolvedModelAliasesFollowTheInstallPrecedence(t *testing.T) {
	t.Setenv("FLASH_AGENT_MODEL", "platform-model")
	configured := withValues(installAgentStep("flash-agent", "sock-shop"), experimentModel)
	onDefault := installAgentStep("sre-agent-crewai", "sock-shop")
	app := installTemplate("install-application", "agentcert/agentcert-install-app:latest", "bookinfo", "book-info")

	cases := []struct {
		name      string
		templates []v1alpha1.Template
		override  string
		want      []string
	}{
		{"a step on the platform default contributes nothing", []v1alpha1.Template{onDefault}, "", nil},
		{"the experiment's own setting", []v1alpha1.Template{app, configured, onDefault}, "", []string{"experiment-model"}},
		{"a run-time pick covers every agent step, once", []v1alpha1.Template{configured, onDefault}, " run-model ", []string{"run-model"}},
		{"no agent step", []v1alpha1.Template{app}, "run-model", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolvedModelAliases(tc.templates, tc.override); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ResolvedModelAliases() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestManifestModelAliasesReadBothKindsAndThePersistedPick(t *testing.T) {
	step := withValues(installAgentStep("flash-agent", "sock-shop"), experimentModel)
	workflow := marshal(t, v1alpha1.Workflow{
		TypeMeta: v1.TypeMeta{Kind: "Workflow"},
		Spec:     v1alpha1.WorkflowSpec{Templates: []v1alpha1.Template{step}},
	})
	picked := marshal(t, v1alpha1.Workflow{
		TypeMeta:   v1.TypeMeta{Kind: "Workflow"},
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{RunModelAliasAnnotation: "picked-model"}},
		Spec:       v1alpha1.WorkflowSpec{Templates: []v1alpha1.Template{step}},
	})
	cron := marshal(t, v1alpha1.CronWorkflow{
		TypeMeta: v1.TypeMeta{Kind: "CronWorkflow"},
		Spec:     v1alpha1.CronWorkflowSpec{Schedule: "0 * * * *", WorkflowSpec: v1alpha1.WorkflowSpec{Templates: []v1alpha1.Template{step}}},
	})

	cases := []struct {
		name, manifest, override string
		want                     []string
	}{
		{"workflow setting", workflow, "", []string{"experiment-model"}},
		{"cron workflow setting", cron, "", []string{"experiment-model"}},
		{"the persisted run pick beats the setting", picked, "", []string{"picked-model"}},
		{"an explicit pick beats the persisted one", picked, "run-model", []string{"run-model"}},
		{"an unparsable manifest yields none", "{not json", "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ManifestModelAliases(tc.manifest, tc.override); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ManifestModelAliases() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSaveRunsTheModelAliasValidator(t *testing.T) {
	withTestHubs(t)
	rejected := errors.New("model not available")
	var gotProject string
	var gotAliases []string
	service := &chaosExperimentService{modelAliasValidator: func(_ context.Context, projectID string, aliases []string) error {
		gotProject, gotAliases = projectID, aliases
		return rejected
	}}
	spec := configuredSpec("", `{"agent":{"config":{"MODEL_ALIAS":"gpt-4o-bench"}}}`)

	err := service.validateExperimentComposition(context.Background(), "project-1", spec)

	if !errors.Is(err, rejected) {
		t.Fatalf("validateExperimentComposition() error = %v, want the validator's error", err)
	}
	if gotProject != "project-1" || !reflect.DeepEqual(gotAliases, []string{"gpt-4o-bench"}) {
		t.Fatalf("validator got (%q, %v), want (project-1, [gpt-4o-bench])", gotProject, gotAliases)
	}
}

func marshal(t *testing.T, value interface{}) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
