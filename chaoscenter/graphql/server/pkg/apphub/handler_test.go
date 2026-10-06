package apphub

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
)

func TestGetAppChartsDataListsEachChartsSettings(t *testing.T) {
	charts := t.TempDir()
	write(t, filepath.Join(charts, "applications.chartserviceversion.yaml"), `
spec:
  applications:
    - name: bookinfo
      namespace: book-info
`)
	write(t, filepath.Join(charts, "bookinfo", "values.yaml"), `
bookInfo:
  loadGenerator:
    enabled: true
configurations:
  - key: bookInfo.loadGenerator.enabled
    type: boolean
`)

	categories, err := GetAppChartsData(charts)
	if err != nil {
		t.Fatalf("GetAppChartsData() error = %v", err)
	}
	settings := categories[0].Applications[0].Configurations
	if len(settings) != 1 || settings[0].Type != model.ConfigFieldTypeBoolean ||
		settings[0].ValueKind != model.ConfigValueKindBoolean || settings[0].DefaultValue != "true" {
		t.Fatalf("bookinfo settings = %+v", settings)
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
