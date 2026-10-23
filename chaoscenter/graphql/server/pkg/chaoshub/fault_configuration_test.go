package chaoshub

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/faultconfig"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"
)

func TestFaultAPIResolvesBothCatalogs(t *testing.T) {
	oldMode, oldPath := utils.Config.HubSourceMode, utils.Config.DefaultChaosHubPath
	t.Cleanup(func() { utils.Config.HubSourceMode, utils.Config.DefaultChaosHubPath = oldMode, oldPath })
	root := t.TempDir()
	charts, err := filepath.Abs("../../../../../../chaos-charts")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(charts, filepath.Join(root, "Litmus ChaosHub")); err != nil {
		t.Fatal(err)
	}
	utils.Config.HubSourceMode, utils.Config.DefaultChaosHubPath = "custom", root+"/"
	service := NewService(nil)
	for _, tc := range []struct{ category, name, key string }{
		{"itbench", "modified-target-port-kubernetes-service", "BROKEN_TARGET_PORT"},
		{"kubernetes", "pod-cpu-hog", "CPU_CORES"},
	} {
		response, err := service.GetChaosFault(context.Background(), model.ExperimentRequest{HubID: DefaultHubID, Category: tc.category, ExperimentName: tc.name}, "project")
		if err != nil {
			t.Fatal(err)
		}
		if response.Fault == "" || response.Engine == "" || len(response.ConfigurationRevision) != 64 {
			t.Fatalf("incomplete response for %s", tc.name)
		}
		found := false
		for _, field := range response.Configurations {
			if field.Key == tc.key {
				found = true
				if field.Type != model.ConfigFieldTypeInteger || field.DefaultValue == "" {
					t.Fatalf("incorrect typed setting: %#v", field)
				}
			}
		}
		if !found {
			t.Fatalf("missing %s for %s", tc.key, tc.name)
		}
		definition, err := service.GetFaultConfiguration(context.Background(), faultconfig.Source{Name: tc.name}, "project")
		if err != nil || definition.Source.Category != tc.category || definition.Revision != response.ConfigurationRevision {
			t.Fatalf("legacy lookup disagrees with API: %#v, %v", definition, err)
		}
	}
	for _, request := range []model.ExperimentRequest{
		{HubID: DefaultHubID, Category: "../kubernetes", ExperimentName: "pod-cpu-hog"},
		{HubID: DefaultHubID, Category: "kubernetes", ExperimentName: "missing"},
	} {
		if _, err := service.GetChaosFault(context.Background(), request, "project"); err == nil {
			t.Fatalf("returned empty success for invalid request %#v", request)
		}
	}
}

func TestNewCategoryAndFieldsAreDiscoveredWithoutRestart(t *testing.T) {
	oldMode, oldPath := utils.Config.HubSourceMode, utils.Config.DefaultChaosHubPath
	t.Cleanup(func() { utils.Config.HubSourceMode, utils.Config.DefaultChaosHubPath = oldMode, oldPath })
	root := t.TempDir()
	utils.Config.HubSourceMode, utils.Config.DefaultChaosHubPath = "custom", root+"/"
	service := NewService(nil)
	dir := filepath.Join(root, "Litmus ChaosHub", "faults", "new-category", "new-fault")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"new-fault.chartserviceversion.yaml": "spec:\n  configurations:\n    - key: COUNT\n      type: integer\n      min: 1\n",
		"fault.yaml":                         "kind: ChaosExperiment\nspec:\n  definition:\n    env:\n      - name: COUNT\n        value: '3'\n      - name: EXTRA\n        value: '4'\n",
		"engine.yaml":                        "kind: ChaosEngine\n",
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	request := model.ExperimentRequest{HubID: DefaultHubID, Category: "new-category", ExperimentName: "new-fault"}
	first, err := service.GetChaosFault(context.Background(), request, "project")
	if err != nil || len(first.Configurations) != 1 {
		t.Fatalf("new fault was not discovered: %#v, %v", first, err)
	}
	csv := files["new-fault.chartserviceversion.yaml"] + "    - key: EXTRA\n      type: integer\n      min: 1\n"
	if err := os.WriteFile(filepath.Join(dir, "new-fault.chartserviceversion.yaml"), []byte(csv), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := service.GetChaosFault(context.Background(), request, "project")
	if err != nil || len(second.Configurations) != 2 || first.ConfigurationRevision == second.ConfigurationRevision {
		t.Fatalf("new setting was not discovered: %#v, %v", second, err)
	}
	if err := os.Remove(filepath.Join(dir, "engine.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetChaosFault(context.Background(), request, "project"); err == nil {
		t.Fatal("missing engine returned an empty success")
	}
}
