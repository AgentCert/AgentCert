package experiment_hydrator

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	expdef "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/experiment_definition"
)

func TestHydrate_ObserveStep(t *testing.T) {
	def := &expdef.ExperimentDefinitionDoc{
		Name:    "test-experiment",
		Version: "1.0.0",
		Steps: []expdef.ExperimentStep{
			{Name: "baseline", Type: expdef.StepTypeObserve, Duration: "30s"},
		},
		TargetApp:      expdef.TargetAppSpec{Name: "sock-shop"},
		ModelSelection: expdef.ModelSelection{Mode: expdef.ModelSelectionAgentDefault},
	}
	agent := &AgentSpec{Name: "flash-agent", Version: "1.0.0"}
	params := HydrationParams{
		RunID:           "run-abc123",
		AppNamespace:    "sock-shop-run-abc123",
		LitellmUpstream: "http://litellm.litmus.svc.cluster.local:4000",
		AgentSecretName: "ace-agent-secret-run-abc123",
		MicroserviceMap: map[string]MicroserviceInfo{},
	}

	yamlStr, err := Hydrate(def, agent, params)
	if err != nil {
		t.Fatalf("Hydrate returned error: %v", err)
	}
	if len(yamlStr) == 0 {
		t.Fatal("Hydrate returned empty YAML")
	}

	// Validate it parses back
	var check map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlStr), &check); err != nil {
		t.Fatalf("generated YAML does not parse: %v\nYAML:\n%s", err, yamlStr)
	}

	// Check key fields
	if check["kind"] != "Workflow" {
		t.Errorf("expected kind=Workflow, got %v", check["kind"])
	}
	if check["apiVersion"] != "argoproj.io/v1alpha1" {
		t.Errorf("expected apiVersion=argoproj.io/v1alpha1, got %v", check["apiVersion"])
	}
}

func TestHydrate_NilDef(t *testing.T) {
	_, err := Hydrate(nil, &AgentSpec{Name: "a"}, HydrationParams{RunID: "x"})
	if err == nil {
		t.Fatal("expected error for nil definition")
	}
}

func TestHydrate_EmptyRunID(t *testing.T) {
	def := &expdef.ExperimentDefinitionDoc{Name: "test", Version: "1.0.0"}
	_, err := Hydrate(def, &AgentSpec{Name: "a"}, HydrationParams{})
	if err == nil {
		t.Fatal("expected error for empty RunID")
	}
}

func TestHydrate_ContainsRunID(t *testing.T) {
	def := &expdef.ExperimentDefinitionDoc{
		Name:    "my-exp",
		Version: "1.0.0",
		Steps: []expdef.ExperimentStep{
			{Name: "wait", Type: expdef.StepTypeWait, Duration: "10s"},
		},
		TargetApp:      expdef.TargetAppSpec{Name: "sock-shop"},
		ModelSelection: expdef.ModelSelection{Mode: expdef.ModelSelectionAgentDefault},
	}
	params := HydrationParams{
		RunID:        "run-xyz789",
		AppNamespace: "sock-shop-run-xyz789",
	}
	yamlStr, err := Hydrate(def, &AgentSpec{Name: "agent"}, params)
	if err != nil {
		t.Fatalf("Hydrate error: %v", err)
	}
	if !strings.Contains(yamlStr, "run-xyz789") {
		t.Errorf("expected YAML to contain run ID 'run-xyz789'")
	}
}

func TestHydrate_FaultStepMissingMicroservice(t *testing.T) {
	def := &expdef.ExperimentDefinitionDoc{
		Name:    "fault-exp",
		Version: "1.0.0",
		Steps: []expdef.ExperimentStep{
			{
				Name:     "inject",
				Type:     expdef.StepTypeFault,
				FaultRef: "pod-delete",
				Target:   &expdef.StepTarget{Microservice: "nonexistent-svc"},
			},
		},
		TargetApp:      expdef.TargetAppSpec{Name: "sock-shop"},
		ModelSelection: expdef.ModelSelection{Mode: expdef.ModelSelectionAgentDefault},
	}
	params := HydrationParams{
		RunID:           "run-test",
		MicroserviceMap: map[string]MicroserviceInfo{}, // empty — no microservices
	}
	_, err := Hydrate(def, &AgentSpec{Name: "agent"}, params)
	if err == nil {
		t.Fatal("expected error for missing microservice in map")
	}
}
