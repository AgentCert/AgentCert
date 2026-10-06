package agenthub

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
)

func TestGetAgentChartsDataListsEachChartsSettings(t *testing.T) {
	charts := t.TempDir()
	write(t, filepath.Join(charts, "agents.chartserviceversion.yaml"), `
spec:
  agents:
    - name: flash-agent
    - name: no-chart-agent
`)
	write(t, filepath.Join(charts, "flash-agent", "values.yaml"), `
agent:
  config:
    SCAN_INTERVAL: "60"
configurations:
  - key: agent.config.SCAN_INTERVAL
    type: integer
    min: 1
`)

	categories, err := GetAgentChartsData(charts)
	if err != nil {
		t.Fatalf("GetAgentChartsData() error = %v", err)
	}
	agents := categories[0].Agents
	settings := agents[0].Configurations
	if len(settings) != 1 {
		t.Fatalf("flash-agent settings = %d, want 1", len(settings))
	}
	got := settings[0]
	if got.Key != "agent.config.SCAN_INTERVAL" || got.Type != model.ConfigFieldTypeInteger ||
		got.ValueKind != model.ConfigValueKindString || got.DefaultValue != "60" || got.Min == nil || *got.Min != 1 {
		t.Fatalf("setting = %+v", got)
	}
	if agents[1].Configurations == nil || len(agents[1].Configurations) != 0 {
		t.Fatalf("an agent without a chart folder must offer [] settings, got %v", agents[1].Configurations)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
