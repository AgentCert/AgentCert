package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// LoadAll reads all app.yaml files from the given catalog root directory.
// Invalid entries are logged and skipped.
// Returns entries sorted: official first (alphabetical), then community (alphabetical).
func LoadAll(catalogDir string) ([]*AppCatalogEntry, error) {
	appsDir := filepath.Join(catalogDir, "apps")

	if _, err := os.Stat(appsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("catalog apps directory not found: %s", appsDir)
	}

	var official, community []*AppCatalogEntry

	for _, tier := range []string{"official", "community"} {
		tierDir := filepath.Join(appsDir, tier)
		if _, err := os.Stat(tierDir); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(tierDir)
		if err != nil {
			log.WithError(err).Errorf("failed to read catalog tier directory: %s", tierDir)
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			appYAML := filepath.Join(tierDir, entry.Name(), "app.yaml")
			if _, err := os.Stat(appYAML); os.IsNotExist(err) {
				continue
			}

			app, err := loadAppYAML(appYAML)
			if err != nil {
				log.WithFields(log.Fields{"file": appYAML, "error": err}).Warn("skipping invalid app.yaml")
				continue
			}

			if err := validateEntry(app); err != nil {
				log.WithFields(log.Fields{"app": app.Metadata.Name, "error": err}).Warn("skipping app.yaml that failed validation")
				continue
			}

			switch tier {
			case "official":
				official = append(official, app)
			case "community":
				community = append(community, app)
			}
		}
	}

	return append(official, community...), nil
}

func loadAppYAML(path string) (*AppCatalogEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	var entry AppCatalogEntry
	if err := yaml.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	return &entry, nil
}

// validateEntry checks required fields and the {{.AppNamespace}} template variable rule.
func validateEntry(app *AppCatalogEntry) error {
	if app.APIVersion != "ace.io/v1" {
		return fmt.Errorf("apiVersion must be ace.io/v1, got %q", app.APIVersion)
	}
	if app.Kind != "AppCatalogEntry" {
		return fmt.Errorf("kind must be AppCatalogEntry, got %q", app.Kind)
	}
	if app.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	if app.Metadata.Version == "" {
		return fmt.Errorf("metadata.version is required (app: %s)", app.Metadata.Name)
	}
	if app.Metadata.Tier != "official" && app.Metadata.Tier != "community" {
		return fmt.Errorf("metadata.tier must be 'official' or 'community', got %q (app: %s)", app.Metadata.Tier, app.Metadata.Name)
	}
	if len(app.Metadata.Maintainers) == 0 {
		return fmt.Errorf("metadata.maintainers must have at least one entry (app: %s)", app.Metadata.Name)
	}
	if !strings.Contains(app.Spec.HealthProbe.URL, "{{.AppNamespace}}") {
		return fmt.Errorf("healthProbe.url must contain {{.AppNamespace}}, got %q (app: %s)", app.Spec.HealthProbe.URL, app.Metadata.Name)
	}
	return nil
}
