package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

// writeAppYAML creates a minimal catalog directory tree and writes content into
// <root>/apps/<tier>/<name>/app.yaml. It returns a cleanup function.
func writeAppYAML(t *testing.T, root, tier, name, content string) {
	t.Helper()
	dir := filepath.Join(root, "apps", tier, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("write app.yaml: %v", err)
	}
}

const validYAML = `
apiVersion: ace.io/v1
kind: AppCatalogEntry
metadata:
  name: sock-shop
  displayName: Sock Shop
  version: "1.0.0"
  tier: official
  domain: e-commerce
  capabilityDomains: [resilience]
  tags: [demo]
  maintainers:
    - name: ACE Team
      email: ace@example.com
spec:
  description:
    short: Demo microservice app
    long: A longer description
    suitableFor: [demos]
    notSuitableFor: []
  install:
    method: helm
    folder: sock-shop
    namespace:
      default: sock-shop
      configurable: true
    timeout: 5m
    wait: true
  healthProbe:
    url: "http://front-end.{{.AppNamespace}}.svc.cluster.local:80"
    expectedStatus: "200"
    initialDelaySeconds: 30
    periodSeconds: 10
    failureThreshold: 6
  loadTest:
    enabled: false
  microservices: []
  faultCompatibility: []
  inputs: []
`

func TestValidateEntry(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*AppCatalogEntry)
		wantErr bool
	}{
		{
			name:    "valid entry passes",
			mutate:  func(*AppCatalogEntry) {},
			wantErr: false,
		},
		{
			name:    "wrong apiVersion",
			mutate:  func(e *AppCatalogEntry) { e.APIVersion = "v1" },
			wantErr: true,
		},
		{
			name:    "wrong kind",
			mutate:  func(e *AppCatalogEntry) { e.Kind = "App" },
			wantErr: true,
		},
		{
			name:    "empty name",
			mutate:  func(e *AppCatalogEntry) { e.Metadata.Name = "" },
			wantErr: true,
		},
		{
			name:    "empty version",
			mutate:  func(e *AppCatalogEntry) { e.Metadata.Version = "" },
			wantErr: true,
		},
		{
			name:    "invalid tier",
			mutate:  func(e *AppCatalogEntry) { e.Metadata.Tier = "premium" },
			wantErr: true,
		},
		{
			name:    "no maintainers",
			mutate:  func(e *AppCatalogEntry) { e.Metadata.Maintainers = nil },
			wantErr: true,
		},
		{
			name:    "healthProbe URL missing template var",
			mutate:  func(e *AppCatalogEntry) { e.Spec.HealthProbe.URL = "http://front-end.sock-shop.svc/health" },
			wantErr: true,
		},
		{
			name:    "community tier passes",
			mutate:  func(e *AppCatalogEntry) { e.Metadata.Tier = "community" },
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := buildValidEntry()
			tc.mutate(entry)
			err := validateEntry(entry)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateEntry() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func buildValidEntry() *AppCatalogEntry {
	return &AppCatalogEntry{
		APIVersion: "ace.io/v1",
		Kind:       "AppCatalogEntry",
		Metadata: AppMetadata{
			Name:        "sock-shop",
			Version:     "1.0.0",
			Tier:        "official",
			Maintainers: []Maintainer{{Name: "ACE Team", Email: "ace@example.com"}},
		},
		Spec: AppSpec{
			HealthProbe: HealthProbeSpec{
				URL: "http://front-end.{{.AppNamespace}}.svc.cluster.local:80",
			},
		},
	}
}

func TestLoadAll_MissingAppsDir(t *testing.T) {
	tmp := t.TempDir()
	_, err := LoadAll(tmp)
	if err == nil {
		t.Fatal("expected error for missing apps dir, got nil")
	}
}

func TestLoadAll_OfficialBeforeCommunity(t *testing.T) {
	root := t.TempDir()
	writeAppYAML(t, root, "community", "zeta-app", makeYAML("zeta-app", "community"))
	writeAppYAML(t, root, "official", "alpha-app", makeYAML("alpha-app", "official"))

	entries, err := LoadAll(root)
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	if entries[0].Metadata.Tier != "official" {
		t.Errorf("first entry should be official, got %s", entries[0].Metadata.Tier)
	}
	if entries[1].Metadata.Tier != "community" {
		t.Errorf("second entry should be community, got %s", entries[1].Metadata.Tier)
	}
}

func TestLoadAll_SkipsInvalidYAML(t *testing.T) {
	root := t.TempDir()
	writeAppYAML(t, root, "official", "bad-app", "this: is: not: valid: yaml: {{{{")
	writeAppYAML(t, root, "official", "good-app", makeYAML("good-app", "official"))

	entries, err := LoadAll(root)
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 valid entry, got %d", len(entries))
	}
	if entries[0].Metadata.Name != "good-app" {
		t.Errorf("expected good-app, got %s", entries[0].Metadata.Name)
	}
}

func TestLoadAll_SkipsMissingAppYAML(t *testing.T) {
	root := t.TempDir()
	// Create a directory but no app.yaml inside it
	if err := os.MkdirAll(filepath.Join(root, "apps", "official", "empty-app"), 0755); err != nil {
		t.Fatal(err)
	}
	writeAppYAML(t, root, "official", "real-app", makeYAML("real-app", "official"))

	entries, err := LoadAll(root)
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry (directory without app.yaml skipped), got %d", len(entries))
	}
}

// makeYAML builds a minimal valid app.yaml string for the given name and tier.
func makeYAML(name, tier string) string {
	return `apiVersion: ace.io/v1
kind: AppCatalogEntry
metadata:
  name: ` + name + `
  displayName: ` + name + `
  version: "1.0.0"
  tier: ` + tier + `
  domain: test
  maintainers:
    - name: Tester
      email: test@example.com
spec:
  description:
    short: short
    long: long
    suitableFor: []
    notSuitableFor: []
  install:
    method: helm
    namespace:
      default: ` + name + `
      configurable: true
    timeout: 5m
    wait: true
  healthProbe:
    url: "http://` + name + `.{{.AppNamespace}}.svc.cluster.local:80"
    expectedStatus: "200"
  loadTest:
    enabled: false
  microservices: []
  faultCompatibility: []
  inputs: []
`
}
