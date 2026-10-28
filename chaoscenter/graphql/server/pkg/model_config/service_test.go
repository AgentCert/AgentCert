package model_config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaos_experiment/ops"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
	dbChaosExperiment "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment"
	dbChaosExperimentRun "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment_run"
	dbModelConfig "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/model_config"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	projectA = "project-a"
	projectB = "project-b"
)

var testNow = time.UnixMilli(5_000_000_000)

// platformEnv clears every variable PlatformModelOptions reads, then sets vars.
func platformEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for _, key := range []string{"OLLAMA_MODEL", "AZURE_OPENAI_KEY", "AZURE_OPENAI_API_KEY", "AZURE_OPENAI_DEPLOYMENT",
		"AZURE_OPENAI_CHAT_DEPLOYMENT_NAME", "GEMINI_API_KEY", "OPENROUTER_API_KEY", "FLASH_AGENT_MODEL", "MODEL_ALIAS"} {
		t.Setenv(key, vars[key])
	}
}

func azureRequest(alias string) model.CreateModelConfigInput {
	return model.CreateModelConfigInput{Alias: alias, Label: ptr("GPT-4o bench"), Connection: &model.ModelConnectionInput{
		Provider: model.ModelProviderAzureOpenai, Model: "gpt-4o", APIBase: ptr("https://acme.openai.azure.com"),
		APIVersion: ptr("2024-08-01-preview"), APIKey: ptr("az-secret-key-1234"),
	}}
}

func storedModel(id, project, alias string, status dbModelConfig.SyncStatus) dbModelConfig.ModelConfig {
	return dbModelConfig.ModelConfig{
		Audit:     mongodb.Audit{CreatedAt: testNow.Add(-time.Hour).UnixMilli(), UpdatedAt: testNow.Add(-time.Hour).UnixMilli()},
		ProjectID: project, ModelID: id, Alias: alias, Label: alias, Provider: string(model.ModelProviderAzureOpenai),
		Model: "azure/gpt-4o", APIBase: "https://acme.openai.azure.com", APIVersion: "2024-08-01-preview",
		APIKeyHint: "****1234", SyncStatus: status,
	}
}

func TestCreateRegistersTheModelUnderACEsID(t *testing.T) {
	platformEnv(t, nil)
	store, gateway := newFakeStore(), newFakeGateway("static-model")
	svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)

	created, err := svc.Create(context.Background(), projectA, azureRequest("gpt-4o-bench"), "alice")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	row := store.only()
	if created.SyncStatus != model.ModelSyncStatusActive || row.SyncStatus != dbModelConfig.Active {
		t.Fatalf("status = %s / %s, want ACTIVE", created.SyncStatus, row.SyncStatus)
	}
	if !gateway.has(row.ModelID) || gateway.keys[row.ModelID] != "az-secret-key-1234" {
		t.Fatalf("LiteLLM must hold the model under ACE's ID %s with the user's key", row.ModelID)
	}
	if row.Model != "azure/gpt-4o" || row.APIKeyHint != "****1234" || row.CreatedBy.Username != "alice" || row.Label != "GPT-4o bench" {
		t.Fatalf("stored row = %+v", row)
	}
	if created.APIKeyHint == nil || *created.APIKeyHint != "****1234" {
		t.Fatal("the response must carry the masked key only")
	}
}

func TestListReturnsTheProjectsLiveModels(t *testing.T) {
	removed := storedModel("m2", projectA, "deleted-one", dbModelConfig.Deleting)
	removed.IsRemoved = true
	lost := storedModel("m3", projectA, "lost-one", dbModelConfig.NeedsKey)
	lost.LastError = "LiteLLM no longer has this model"
	store := newFakeStore(storedModel("m1", projectA, "gpt-4o-bench", dbModelConfig.Active), removed, lost, storedModel("m4", projectB, "other", dbModelConfig.Active))
	svc := newTestService(store, newFakeGateway(), fakeRuns{}, fakeExperiments{}, testNow)

	configs, err := svc.List(context.Background(), projectA)
	if err != nil || len(configs) != 2 {
		t.Fatalf("List() = %d models, %v; want the project's two live ones", len(configs), err)
	}
	for _, config := range configs {
		if config.ProjectID != projectA || config.Provider != model.ModelProviderAzureOpenai {
			t.Fatalf("List() returned %+v", config)
		}
		if config.Alias == "lost-one" && (config.SyncStatus != model.ModelSyncStatusNeedsKey || config.LastError == nil) {
			t.Fatalf("a lost model must say why: %+v", config)
		}
	}
}

func TestCreateWithAKnownFailureLeavesNothingAndFreesTheAlias(t *testing.T) {
	platformEnv(t, nil)
	store, gateway := newFakeStore(), newFakeGateway()
	gateway.addErr = fmt.Errorf("%w: unsupported model", ErrRejected)
	svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)

	if _, err := svc.Create(context.Background(), projectA, azureRequest("gpt-4o-bench"), "alice"); !errors.Is(err, ErrRejected) {
		t.Fatalf("Create() error = %v, want ErrRejected", err)
	}
	if len(store.rows) != 0 {
		t.Fatalf("a model LiteLLM rejected must leave no row, have %d", len(store.rows))
	}
	gateway.addErr = nil
	if _, err := svc.Create(context.Background(), projectA, azureRequest("gpt-4o-bench"), "alice"); err != nil {
		t.Fatalf("the alias of a rejected model must be free again: %v", err)
	}
}

func TestCreateWithAnUnknownOutcomeStaysPending(t *testing.T) {
	platformEnv(t, nil)
	store, gateway := newFakeStore(), newFakeGateway()
	gateway.addErr = fmt.Errorf("%w: timeout", ErrUnreachable)
	svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)

	if _, err := svc.Create(context.Background(), projectA, azureRequest("gpt-4o-bench"), "alice"); !errors.Is(err, ErrPendingConfirmation) {
		t.Fatalf("Create() error = %v, want ErrPendingConfirmation", err)
	}
	row := store.only()
	if row.SyncStatus != dbModelConfig.Pending || !strings.Contains(row.LastError, "timeout") {
		t.Fatalf("row = %+v, want PENDING with the error recorded", row)
	}
}

func TestCreateRejectsAliasesInUse(t *testing.T) {
	cases := []struct {
		name   string
		env    map[string]string
		static []string
		rows   []dbModelConfig.ModelConfig
	}{
		{name: "served from LiteLLM's config file", static: []string{"gpt-4o-bench"}},
		{name: "a deleted model's alias is never reused", rows: []dbModelConfig.ModelConfig{func() dbModelConfig.ModelConfig {
			row := storedModel("old", projectB, "gpt-4o-bench", dbModelConfig.Deleting)
			row.IsRemoved = true
			return row
		}()}},
		{name: "a platform model", env: map[string]string{"OLLAMA_MODEL": "gpt-4o-bench"}},
		{name: "the platform default", env: map[string]string{"FLASH_AGENT_MODEL": "gpt-4o-bench"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			platformEnv(t, tc.env)
			store, gateway := newFakeStore(tc.rows...), newFakeGateway(tc.static...)
			svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)

			if _, err := svc.Create(context.Background(), projectA, azureRequest("gpt-4o-bench"), "alice"); !errors.Is(err, ErrAliasTaken) {
				t.Fatalf("Create() error = %v, want ErrAliasTaken", err)
			}
			for _, call := range gateway.calls {
				if strings.HasPrefix(call, "add") {
					t.Fatal("LiteLLM must not be asked to add a taken alias")
				}
			}
		})
	}
}

func TestCreateValidatesBeforeTouchingAnything(t *testing.T) {
	platformEnv(t, nil)
	store, gateway := newFakeStore(), newFakeGateway()
	svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)
	request := azureRequest("gpt-4o-bench")
	request.Connection.APIKey = ptr("os.environ/AZURE_OPENAI_API_KEY")

	if _, err := svc.Create(context.Background(), projectA, request, "alice"); !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("Create() error = %v, want ErrInvalidModel", err)
	}
	if len(store.rows) != 0 || len(gateway.calls) != 0 {
		t.Fatalf("an invalid model must not reach MongoDB (%d rows) or LiteLLM (%v)", len(store.rows), gateway.calls)
	}
}

func TestCreateOfAKeylessModelSendsThePlaceholder(t *testing.T) {
	platformEnv(t, nil)
	store, gateway := newFakeStore(), newFakeGateway()
	svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)
	request := model.CreateModelConfigInput{Alias: "qwen-local", Connection: &model.ModelConnectionInput{
		Provider: model.ModelProviderOllama, Model: "qwen2.5:7b", APIBase: ptr("http://ollama:11434"),
	}}

	created, err := svc.Create(context.Background(), projectA, request, "alice")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if gateway.keys[created.ModelID] != noKeyPlaceholder || created.APIKeyHint != nil || created.Label != "qwen-local" {
		t.Fatalf("key sent = %q, hint = %v, label = %q", gateway.keys[created.ModelID], created.APIKeyHint, created.Label)
	}
}

func TestCreateRacingADeleteLeavesNoModelInLiteLLM(t *testing.T) {
	platformEnv(t, nil)
	store, gateway := newFakeStore(), newFakeGateway()
	svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)
	gateway.onAdd = func() { // the row is deleted while LiteLLM adds the model
		for id := range store.rows {
			store.rows[id].SyncStatus, store.rows[id].IsRemoved = dbModelConfig.Deleting, true
		}
	}

	if _, err := svc.Create(context.Background(), projectA, azureRequest("gpt-4o-bench"), "alice"); !errors.Is(err, ErrModelBusy) {
		t.Fatalf("Create() error = %v, want ErrModelBusy", err)
	}
	if len(gateway.served) != 0 {
		t.Fatal("LiteLLM must not keep a model deleted while it was created")
	}
}

func TestCreateRacingReconcileKeepsTheModel(t *testing.T) {
	platformEnv(t, nil)
	store, gateway := newFakeStore(), newFakeGateway()
	svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)
	gateway.onAdd = func() { // reconcile marks it ACTIVE first
		for id := range store.rows {
			store.rows[id].SyncStatus = dbModelConfig.Active
		}
	}

	if _, err := svc.Create(context.Background(), projectA, azureRequest("gpt-4o-bench"), "alice"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(gateway.served) != 1 {
		t.Fatal("a model reconcile already activated must stay in LiteLLM")
	}
}

func TestDeleteRemovesTheModelAndKeepsItsAliasReserved(t *testing.T) {
	platformEnv(t, nil)
	store, gateway := newFakeStore(storedModel("m1", projectA, "gpt-4o-bench", dbModelConfig.Active)), newFakeGateway()
	gateway.served["m1"] = RemoteModel{Alias: "gpt-4o-bench", ModelID: "m1", DBModel: true}
	svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)

	if err := svc.Delete(context.Background(), projectA, "m1", "bob"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if gateway.has("m1") || !store.row("m1").IsRemoved || store.row("m1").UpdatedBy.Username != "bob" {
		t.Fatalf("LiteLLM still has it: %v; row: %+v", gateway.has("m1"), store.row("m1"))
	}
	if _, err := svc.Create(context.Background(), projectA, azureRequest("gpt-4o-bench"), "bob"); !errors.Is(err, ErrAliasTaken) {
		t.Fatalf("re-creating a deleted alias: error = %v, want ErrAliasTaken", err)
	}
}

func TestDeleteOfAModelLiteLLMLostSucceeds(t *testing.T) {
	store, gateway := newFakeStore(storedModel("m1", projectA, "gpt-4o-bench", dbModelConfig.NeedsKey)), newFakeGateway()
	svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)

	if err := svc.Delete(context.Background(), projectA, "m1", "bob"); err != nil {
		t.Fatalf("Delete() error = %v; LiteLLM's 'not found' must count as deleted", err)
	}
	if !store.row("m1").IsRemoved {
		t.Fatal("the row must be soft-deleted")
	}
}

func TestDeleteLiteLLMDidNotConfirmStaysDeletingUntilReconcile(t *testing.T) {
	store, gateway := newFakeStore(storedModel("m1", projectA, "gpt-4o-bench", dbModelConfig.Active)), newFakeGateway()
	gateway.served["m1"] = RemoteModel{Alias: "gpt-4o-bench", ModelID: "m1", DBModel: true}
	gateway.deleteErr = fmt.Errorf("%w: connection reset", ErrUnreachable)
	svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)

	if err := svc.Delete(context.Background(), projectA, "m1", "bob"); !errors.Is(err, ErrPendingConfirmation) {
		t.Fatalf("Delete() error = %v, want ErrPendingConfirmation", err)
	}
	if row := store.row("m1"); row.SyncStatus != dbModelConfig.Deleting || row.IsRemoved || row.LastError == "" {
		t.Fatalf("row = %+v, want DELETING with the error", row)
	}

	gateway.deleteErr = nil
	if err := svc.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if !store.row("m1").IsRemoved || gateway.has("m1") {
		t.Fatal("reconcile must finish the delete")
	}
}

func TestDeleteOfAnotherProjectsModelIsNotFound(t *testing.T) {
	store, gateway := newFakeStore(storedModel("m1", projectB, "gpt-4o-bench", dbModelConfig.Active)), newFakeGateway()
	svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)

	if err := svc.Delete(context.Background(), projectA, "m1", "mallory"); !errors.Is(err, ErrModelNotFound) {
		t.Fatalf("Delete() error = %v, want ErrModelNotFound", err)
	}
	if store.row("m1").SyncStatus != dbModelConfig.Active || len(gateway.calls) != 0 {
		t.Fatal("another project's model must be untouched")
	}
}

// experiment builds a stored experiment whose agent step uses settings, with
// an optional persisted run pick.
func experiment(t *testing.T, id, name, kind, settingsAlias, pickedAlias string, suspended bool) dbChaosExperiment.ChaosExperimentRequest {
	t.Helper()
	step := v1alpha1.Template{Name: "install-agent", Container: &corev1.Container{
		Image: "agentcert/agentcert-install-agent:latest",
		Args:  []string{"-folder=flash-agent", "-namespace=sock-shop", fmt.Sprintf(`-values-json={"agent":{"config":{"MODEL_ALIAS":%q}}}`, settingsAlias)},
	}}
	meta := v1.ObjectMeta{Name: name}
	if pickedAlias != "" {
		meta.Annotations = map[string]string{ops.RunModelAliasAnnotation: pickedAlias}
	}
	var manifest interface{} = v1alpha1.Workflow{TypeMeta: v1.TypeMeta{Kind: "Workflow"}, ObjectMeta: meta, Spec: v1alpha1.WorkflowSpec{Templates: []v1alpha1.Template{step}}}
	cronSyntax := ""
	if kind == "CronWorkflow" {
		cronSyntax = "0 * * * *"
		manifest = v1alpha1.CronWorkflow{TypeMeta: v1.TypeMeta{Kind: "CronWorkflow"}, ObjectMeta: meta,
			Spec: v1alpha1.CronWorkflowSpec{Schedule: cronSyntax, Suspend: suspended, WorkflowSpec: v1alpha1.WorkflowSpec{Templates: []v1alpha1.Template{step}}}}
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return dbChaosExperiment.ChaosExperimentRequest{
		ResourceDetails: mongodb.ResourceDetails{Name: name}, ProjectID: projectA, ExperimentID: id, CronSyntax: cronSyntax,
		Revision: []dbChaosExperiment.ExperimentRevision{{RevisionID: "old", ExperimentManifest: `{}`, UpdatedAt: 1}, {RevisionID: "new", ExperimentManifest: string(data), UpdatedAt: 2}},
	}
}

func TestDeleteGuard(t *testing.T) {
	running := fakeRuns{runs: []dbChaosExperimentRun.ChaosExperimentRun{{ExperimentID: "e1"}}}
	cases := []struct {
		name        string
		runs        fakeRuns
		experiments []dbChaosExperiment.ChaosExperimentRequest
		blockedBy   string
	}{
		{"a running experiment set to the model", running, []dbChaosExperiment.ChaosExperimentRequest{experiment(t, "e1", "bench-run", "Workflow", "gpt-4o-bench", "", false)}, "bench-run"},
		{"a running experiment whose run picked the model", running, []dbChaosExperiment.ChaosExperimentRequest{experiment(t, "e1", "picked-run", "Workflow", "other-model", "gpt-4o-bench", false)}, "picked-run"},
		{"an enabled cron experiment", fakeRuns{}, []dbChaosExperiment.ChaosExperimentRequest{experiment(t, "e2", "nightly", "CronWorkflow", "gpt-4o-bench", "", false)}, "nightly"},
		{"a suspended cron experiment does not block", fakeRuns{}, []dbChaosExperiment.ChaosExperimentRequest{experiment(t, "e2", "nightly", "CronWorkflow", "gpt-4o-bench", "", true)}, ""},
		{"an idle experiment does not block", fakeRuns{}, []dbChaosExperiment.ChaosExperimentRequest{experiment(t, "e3", "idle", "Workflow", "gpt-4o-bench", "", false)}, ""},
		{"a running experiment on another model does not block", running, []dbChaosExperiment.ChaosExperimentRequest{experiment(t, "e1", "other", "Workflow", "other-model", "", false)}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, gateway := newFakeStore(storedModel("m1", projectA, "gpt-4o-bench", dbModelConfig.Active)), newFakeGateway()
			gateway.served["m1"] = RemoteModel{Alias: "gpt-4o-bench", ModelID: "m1", DBModel: true}
			svc := newTestService(store, gateway, tc.runs, fakeExperiments{experiments: tc.experiments}, testNow)

			err := svc.Delete(context.Background(), projectA, "m1", "bob")
			if tc.blockedBy == "" {
				if err != nil {
					t.Fatalf("Delete() error = %v", err)
				}
				return
			}
			if !errors.Is(err, ErrModelInUse) || !strings.Contains(err.Error(), tc.blockedBy) {
				t.Fatalf("Delete() error = %v, want ErrModelInUse naming %q", err, tc.blockedBy)
			}
			if row := store.row("m1"); row.SyncStatus != dbModelConfig.Active || row.IsRemoved || !gateway.has("m1") {
				t.Fatalf("a blocked delete must restore the model, got %+v", row)
			}
		})
	}
}

func TestUpdateKeyReplacesTheKey(t *testing.T) {
	store, gateway := newFakeStore(storedModel("m1", projectA, "gpt-4o-bench", dbModelConfig.Active)), newFakeGateway()
	gateway.served["m1"] = RemoteModel{Alias: "gpt-4o-bench", ModelID: "m1", DBModel: true}
	svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)

	updated, err := svc.UpdateKey(context.Background(), projectA, "m1", "az-rotated-key-9876", "bob")
	if err != nil {
		t.Fatalf("UpdateKey() error = %v", err)
	}
	if gateway.keys["m1"] != "az-rotated-key-9876" || *updated.APIKeyHint != "****9876" || store.row("m1").APIKeyHint != "****9876" {
		t.Fatalf("key = %q, hint = %v", gateway.keys["m1"], *updated.APIKeyHint)
	}
	if strings.Join(gateway.calls, ",") != "update m1" {
		t.Fatalf("calls = %v, want one PATCH", gateway.calls)
	}
}

func TestUpdateKeyReRegistersAModelLiteLLMLost(t *testing.T) {
	row := storedModel("m1", projectA, "gpt-4o-bench", dbModelConfig.NeedsKey)
	row.LastError = "LiteLLM no longer has this model"
	store, gateway := newFakeStore(row), newFakeGateway()
	svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)

	updated, err := svc.UpdateKey(context.Background(), projectA, "m1", "az-new-key-5555", "bob")
	if err != nil {
		t.Fatalf("UpdateKey() error = %v", err)
	}
	if strings.Join(gateway.calls, ",") != "update m1,add m1" || !gateway.has("m1") {
		t.Fatalf("calls = %v, want PATCH then re-add under the same ID", gateway.calls)
	}
	if updated.SyncStatus != model.ModelSyncStatusActive || store.row("m1").SyncStatus != dbModelConfig.Active || store.row("m1").LastError != "" {
		t.Fatalf("row = %+v, want ACTIVE with the error cleared", store.row("m1"))
	}
}

func TestUpdateKeyChecksTheKeyAndTheStatus(t *testing.T) {
	ollama := storedModel("m2", projectA, "qwen-local", dbModelConfig.Active)
	ollama.Provider, ollama.Model, ollama.APIVersion, ollama.APIKeyHint = string(model.ModelProviderOllama), "ollama_chat/qwen2.5:7b", "", ""
	store := newFakeStore(storedModel("m1", projectA, "gpt-4o-bench", dbModelConfig.Active), ollama, storedModel("m3", projectA, "pending-one", dbModelConfig.Pending))
	gateway := newFakeGateway()
	gateway.served["m2"] = RemoteModel{Alias: "qwen-local", ModelID: "m2", DBModel: true}
	svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)

	if _, err := svc.UpdateKey(context.Background(), projectA, "m1", " ", "bob"); !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("an empty Azure key: error = %v, want ErrInvalidModel", err)
	}
	if _, err := svc.UpdateKey(context.Background(), projectA, "m1", "os.environ/AZURE_OPENAI_API_KEY", "bob"); !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("an env reference: error = %v, want ErrInvalidModel", err)
	}
	if _, err := svc.UpdateKey(context.Background(), projectA, "m2", "", "bob"); err != nil || gateway.keys["m2"] != noKeyPlaceholder {
		t.Fatalf("a keyless model: error = %v, key sent = %q", err, gateway.keys["m2"])
	}
	if _, err := svc.UpdateKey(context.Background(), projectA, "m3", "k-123456789012", "bob"); !errors.Is(err, ErrModelBusy) {
		t.Fatalf("a PENDING model: error = %v, want ErrModelBusy", err)
	}
}

func TestTests(t *testing.T) {
	store, gateway := newFakeStore(storedModel("m1", projectA, "gpt-4o-bench", dbModelConfig.NeedsKey)), newFakeGateway()
	gateway.testResult = TestResult{OK: true, LatencyMs: 42}
	svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)

	result, err := svc.TestConnection(context.Background(), model.ModelConnectionInput{Provider: model.ModelProviderOpenaiCompatible, Model: "llama3", APIBase: ptr("http://vllm:8000/v1")})
	if err != nil || !result.Ok || *result.LatencyMs != 42 {
		t.Fatalf("TestConnection() = %+v, %v", result, err)
	}
	if gateway.tested[0].Model != "openai/llama3" || gateway.testedKeys[0] != noKeyPlaceholder {
		t.Fatalf("tested %+v with key %q", gateway.tested[0], gateway.testedKeys[0])
	}
	lost, err := svc.Test(context.Background(), projectA, "m1")
	if err != nil || lost.Ok || !strings.Contains(*lost.Error, "Rotate key") {
		t.Fatalf("testing a model LiteLLM lost = %+v, %v", lost, err)
	}
}

func TestAgentModelOptions(t *testing.T) {
	platformEnv(t, map[string]string{"GEMINI_API_KEY": "g", "FLASH_AGENT_MODEL": "gemini-2.5-flash"})
	store := newFakeStore(
		storedModel("m1", projectA, "gpt-4o-bench", dbModelConfig.Active),
		storedModel("m2", projectA, "pending-one", dbModelConfig.Pending),
		storedModel("m3", projectA, "lost-one", dbModelConfig.NeedsKey),
		storedModel("m4", projectB, "other-project", dbModelConfig.Active),
	)
	svc := newTestService(store, newFakeGateway(), fakeRuns{}, fakeExperiments{}, testNow)

	options := svc.AgentModelOptions(context.Background(), projectA)
	var aliases []string
	defaults := 0
	for _, option := range options {
		aliases = append(aliases, option.Alias)
		if option.IsDefault {
			defaults++
		}
	}
	if strings.Join(aliases, ",") != "gemini-3-flash,gemini-2.5-flash,gemini-2.5-flash-lite,gpt-4o-bench" || defaults != 1 {
		t.Fatalf("options = %v (%d defaults); want the platform models then the project's active model", aliases, defaults)
	}
	if last := options[len(options)-1]; last.Label != "gpt-4o-bench (Azure OpenAI)" || last.Provider != "Azure OpenAI" {
		t.Fatalf("project option = %+v", last)
	}

	store.failLists = errors.New("mongo down")
	if got := svc.AgentModelOptions(context.Background(), projectA); len(got) != 3 {
		t.Fatalf("without MongoDB Studio must still get the %d platform models, got %d", 3, len(got))
	}
}

func TestValidateAliases(t *testing.T) {
	removed := storedModel("m3", projectA, "deleted-one", dbModelConfig.Deleting)
	removed.IsRemoved = true
	store := newFakeStore(
		storedModel("m1", projectA, "gpt-4o-bench", dbModelConfig.Active),
		storedModel("m2", projectA, "pending-one", dbModelConfig.Pending),
		removed,
		storedModel("m4", projectB, "other-project", dbModelConfig.Active),
	)
	svc := newTestService(store, newFakeGateway(), fakeRuns{}, fakeExperiments{}, testNow)

	for _, ok := range [][]string{nil, {""}, {"gpt-4o-bench"}, {"gemini-2.5-flash", "static-or-external"}} {
		if err := svc.ValidateAliases(context.Background(), projectA, ok); err != nil {
			t.Errorf("ValidateAliases(%v) error = %v", ok, err)
		}
	}
	for alias, want := range map[string]string{"other-project": "another project", "deleted-one": "was deleted", "pending-one": "still being registered"} {
		err := svc.ValidateAliases(context.Background(), projectA, []string{"gpt-4o-bench", alias})
		if !errors.Is(err, ErrModelUnavailable) || !strings.Contains(err.Error(), want) {
			t.Errorf("ValidateAliases(%q) error = %v, want ErrModelUnavailable containing %q", alias, err, want)
		}
	}
}
