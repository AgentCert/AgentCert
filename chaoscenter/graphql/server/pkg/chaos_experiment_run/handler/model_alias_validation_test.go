package handler

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaos_experiment/ops"
	dbChaosExperiment "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment"
)

func agentManifest(modelAlias, pickedAlias string) string {
	annotations := ""
	if pickedAlias != "" {
		annotations = `"annotations":{"` + ops.RunModelAliasAnnotation + `":"` + pickedAlias + `"},`
	}
	return `{"kind":"Workflow","metadata":{` + annotations + `"name":"bench"},"spec":{"templates":[{"name":"install-agent","container":{"image":"agentcert/agentcert-install-agent:latest","args":["-folder=flash-agent","-namespace=sock-shop","-values-json={\"agent\":{\"config\":{\"MODEL_ALIAS\":\"` + modelAlias + `\"}}}"]}}]}}`
}

func TestValidateRunModelAliasesChecksTheNewestRevision(t *testing.T) {
	rejected := errors.New("not available")
	var got []string
	handler := &ChaosExperimentRunHandler{}
	handler.SetModelAliasValidator(func(_ context.Context, projectID string, aliases []string) error {
		if projectID != "project-a" {
			t.Fatalf("validator got project %q", projectID)
		}
		got = aliases
		return rejected
	})
	// Revisions arrive in any order; the newest is the one that runs.
	workflow := dbChaosExperiment.ChaosExperimentRequest{Revision: []dbChaosExperiment.ExperimentRevision{
		{ExperimentManifest: agentManifest("newest-model", "picked-model"), UpdatedAt: 3},
		{ExperimentManifest: agentManifest("old-model", ""), UpdatedAt: 1},
	}}

	cases := []struct {
		override string
		want     []string
	}{
		{"", []string{"picked-model"}},       // the pick persisted for a multi-run batch
		{"run-model", []string{"run-model"}}, // an explicit run-time pick
	}
	for _, tc := range cases {
		err := handler.validateRunModelAliases(context.Background(), "project-a", workflow, tc.override)
		if !errors.Is(err, rejected) || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("override %q: error = %v, aliases = %v; want the validator's error for %v", tc.override, err, got, tc.want)
		}
	}
}

func TestValidateRunModelAliasesIsOptional(t *testing.T) {
	workflow := dbChaosExperiment.ChaosExperimentRequest{Revision: []dbChaosExperiment.ExperimentRevision{{ExperimentManifest: agentManifest("m", "")}}}
	if err := (&ChaosExperimentRunHandler{}).validateRunModelAliases(context.Background(), "p", workflow, ""); err != nil {
		t.Fatalf("without a validator runs must not be checked, got %v", err)
	}
}
