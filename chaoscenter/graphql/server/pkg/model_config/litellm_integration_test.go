//go:build integration

package model_config

// Integration tests against a real LiteLLM proxy. They pin the client's error
// classification to the gateway's actual answers, so a LiteLLM upgrade that
// changes them fails here instead of silently turning a known failure into an
// unknown outcome. Run with:
//
//	LITELLM_IT_URL=http://127.0.0.1:4000 LITELLM_IT_MASTER_KEY=sk-... \
//	LITELLM_IT_UPSTREAM=http://upstream:8080/v1 \
//	LITELLM_IT_NODB_URL=http://127.0.0.1:4001 \
//	go test -tags integration -run Integration ./pkg/model_config/
//
// LITELLM_IT_URL is a proxy with its database on; LITELLM_IT_UPSTREAM an
// OpenAI-compatible endpoint that proxy can reach; LITELLM_IT_NODB_URL
// (optional) a proxy running without a database. LITELLM_IT_AZURE_BASE,
// LITELLM_IT_AZURE_KEY, LITELLM_IT_AZURE_API_VERSION and
// LITELLM_IT_AZURE_REASONING_DEPLOYMENT (optional) name an Azure OpenAI
// reasoning deployment (GPT-5 or o-series).

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func integrationEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Skipf("%s not set", name)
	}
	return value
}

func integrationDeployment(t *testing.T) Deployment {
	id := uuid.NewString()
	return Deployment{ModelID: id, Alias: "it-" + id[:8], Model: "openai/it-model", APIBase: integrationEnv(t, "LITELLM_IT_UPSTREAM")}
}

func TestIntegrationModelLifecycle(t *testing.T) {
	client := NewLiteLLMClient(integrationEnv(t, "LITELLM_IT_URL"), integrationEnv(t, "LITELLM_IT_MASTER_KEY"))
	ctx := context.Background()
	d := integrationDeployment(t)

	if connected, err := client.RegistryConnected(ctx); err != nil || !connected {
		t.Fatalf("RegistryConnected() = %v, %v; the proxy needs its database for this test", connected, err)
	}
	if result, err := client.TestConnection(ctx, d, "sk-it-key"); err != nil || !result.OK {
		t.Fatalf("TestConnection() = %+v, %v", result, err)
	}
	if err := client.AddModel(ctx, d, "sk-it-key"); err != nil {
		t.Fatalf("AddModel() error = %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteModel(context.Background(), d.ModelID) })

	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	found := false
	for _, served := range models {
		found = found || served == RemoteModel{Alias: d.Alias, ModelID: d.ModelID, DBModel: true}
	}
	if !found {
		t.Fatalf("LiteLLM must list %s under the ID ACE chose, as a database model: %+v", d.Alias, models)
	}
	if result, err := client.HealthCheck(ctx, d.ModelID); err != nil || !result.OK {
		t.Fatalf("HealthCheck() = %+v, %v", result, err)
	}
	if err := client.UpdateModel(ctx, d, "sk-it-rotated"); err != nil {
		t.Fatalf("UpdateModel() error = %v", err)
	}
	if err := client.AddModel(ctx, d, "sk-it-key"); !errors.Is(err, ErrRejected) {
		t.Fatalf("adding an existing ID: error = %v, want ErrRejected (a known failure)", err)
	}

	if err := client.DeleteModel(ctx, d.ModelID); err != nil {
		t.Fatalf("DeleteModel() error = %v", err)
	}
	if err := client.DeleteModel(ctx, d.ModelID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting it again: error = %v, want ErrNotFound", err)
	}
	if err := client.UpdateModel(ctx, d, "k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("updating a deleted model: error = %v, want ErrNotFound", err)
	}
	if _, err := client.HealthCheck(ctx, d.ModelID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("checking a deleted model: error = %v, want ErrNotFound", err)
	}
}

func TestIntegrationFailedConnectionTest(t *testing.T) {
	client := NewLiteLLMClient(integrationEnv(t, "LITELLM_IT_URL"), integrationEnv(t, "LITELLM_IT_MASTER_KEY"))
	d := Deployment{Model: "openai/x", APIBase: "http://it-nowhere.invalid:9/v1"}
	result, err := client.TestConnection(context.Background(), d, "sk-it-key")
	if err != nil || result.OK || result.Error == "" {
		t.Fatalf("TestConnection() = %+v, %v; want a failed result with its reason", result, err)
	}
}

func TestIntegrationWrongMasterKey(t *testing.T) {
	client := NewLiteLLMClient(integrationEnv(t, "LITELLM_IT_URL"), "sk-not-the-master-key")
	if err := client.AddModel(context.Background(), integrationDeployment(t), "k"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("AddModel() error = %v, want ErrUnauthorized", err)
	}
	if _, err := client.ListModels(context.Background()); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("ListModels() error = %v, want ErrUnauthorized", err)
	}
}

func TestIntegrationWithoutADatabase(t *testing.T) {
	client := NewLiteLLMClient(integrationEnv(t, "LITELLM_IT_NODB_URL"), integrationEnv(t, "LITELLM_IT_MASTER_KEY"))
	ctx := context.Background()
	d := integrationDeployment(t)
	if connected, err := client.RegistryConnected(ctx); err != nil || connected {
		t.Fatalf("RegistryConnected() = %v, %v; want false", connected, err)
	}
	for name, call := range map[string]func() error{
		"AddModel":    func() error { return client.AddModel(ctx, d, "k") },
		"UpdateModel": func() error { return client.UpdateModel(ctx, d, "k") },
		"DeleteModel": func() error { return client.DeleteModel(ctx, d.ModelID) },
		"TestConnection": func() error {
			_, err := client.TestConnection(ctx, d, "k")
			return err
		},
	} {
		if err := call(); !errors.Is(err, ErrRegistryUnavailable) {
			t.Errorf("%s() error = %v, want ErrRegistryUnavailable (a known answer, not an unknown outcome)", name, err)
		}
	}
	if _, err := client.ListModels(ctx); err != nil {
		t.Errorf("without a database LiteLLM still lists its config models: %v", err)
	}
}

// LiteLLM caps every test prompt at one token, which reasoning models refuse.
// The test must still pass such a model, and still fail it under a wrong
// deployment name. Nothing is created in LiteLLM.
func TestIntegrationReasoningModelPassesTheConnectionTest(t *testing.T) {
	client := NewLiteLLMClient(integrationEnv(t, "LITELLM_IT_URL"), integrationEnv(t, "LITELLM_IT_MASTER_KEY"))
	key := integrationEnv(t, "LITELLM_IT_AZURE_KEY")
	d := Deployment{
		Model:      "azure/" + integrationEnv(t, "LITELLM_IT_AZURE_REASONING_DEPLOYMENT"),
		APIBase:    integrationEnv(t, "LITELLM_IT_AZURE_BASE"),
		APIVersion: integrationEnv(t, "LITELLM_IT_AZURE_API_VERSION"),
	}
	ctx := context.Background()

	if result, err := client.TestConnection(ctx, d, key); err != nil || !result.OK {
		t.Fatalf("TestConnection(%s) = %+v, %v; want a pass", d.Model, result, err)
	}
	d.Model += "-missing-" + uuid.NewString()[:8]
	result, err := client.TestConnection(ctx, d, key)
	if err != nil || result.OK || !strings.Contains(result.Error, "deployment") {
		t.Fatalf("TestConnection(%s) = %+v, %v; want a failure naming the missing deployment", d.Model, result, err)
	}
}
