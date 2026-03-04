package apphub

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// MicroserviceEntry represents a single microservice in the chartserviceversion YAML.
type MicroserviceEntry struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// AppEntry represents a single application in the chartserviceversion YAML.
type AppEntry struct {
	Name          string              `yaml:"name"`
	DisplayName   string              `yaml:"displayName"`
	Description   string              `yaml:"description"`
	Version       string              `yaml:"version"`
	Namespace     string              `yaml:"namespace"`
	Microservices []MicroserviceEntry `yaml:"microservices"`
}

// AppCSVSpec is the spec section of the application chartserviceversion YAML.
type AppCSVSpec struct {
	DisplayName         string     `yaml:"displayName"`
	CategoryDescription string     `yaml:"categoryDescription"`
	Keywords            []string   `yaml:"keywords"`
	Applications        []AppEntry `yaml:"applications"`
}

// AppCSVMetadata is the metadata section.
type AppCSVMetadata struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Annotations struct {
		Categories       string `yaml:"categories"`
		Vendor           string `yaml:"vendor"`
		ChartDescription string `yaml:"chartDescription"`
	} `yaml:"annotations"`
}

// AppChartServiceVersion represents the full chartserviceversion YAML for applications.
type AppChartServiceVersion struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   AppCSVMetadata `yaml:"metadata"`
	Spec       AppCSVSpec     `yaml:"spec"`
}

// GetAppChartsData reads the applications chartserviceversion.yaml from the given path
// and returns AppHubCategory entries.
func GetAppChartsData(chartsPath string) ([]*model.AppHubCategory, error) {
	csvFiles, err := findCSVFiles(chartsPath)
	if err != nil {
		return nil, fmt.Errorf("error reading app charts directory %s: %w", chartsPath, err)
	}

	var categories []*model.AppHubCategory
	for _, csvFile := range csvFiles {
		csv, err := readAppCSV(csvFile)
		if err != nil {
			log.WithError(err).Errorf("failed to read app CSV file: %s", csvFile)
			continue
		}

		category := &model.AppHubCategory{
			DisplayName:         csv.Spec.DisplayName,
			CategoryDescription: csv.Spec.CategoryDescription,
			Applications:        make([]*model.AppHubEntry, 0, len(csv.Spec.Applications)),
		}

		for _, app := range csv.Spec.Applications {
			microservices := make([]*model.Microservice, 0, len(app.Microservices))
			for _, ms := range app.Microservices {
				microservices = append(microservices, &model.Microservice{
					Name:        ms.Name,
					Description: ms.Description,
				})
			}

			entry := &model.AppHubEntry{
				Name:          app.Name,
				DisplayName:   app.DisplayName,
				Description:   app.Description,
				Version:       app.Version,
				Namespace:     app.Namespace,
				Microservices: microservices,
				IsDeployed:    false, // Will be enriched later
			}
			category.Applications = append(category.Applications, entry)
		}

		categories = append(categories, category)
	}

	return categories, nil
}

// findCSVFiles finds all chartserviceversion.yaml files in the charts directory.
func findCSVFiles(chartsPath string) ([]string, error) {
	entries, err := os.ReadDir(chartsPath)
	if err != nil {
		return nil, err
	}

	var csvFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && isCSVFile(entry.Name()) {
			csvFiles = append(csvFiles, filepath.Join(chartsPath, entry.Name()))
		}
	}

	// Also check subdirectories for CSV files
	for _, entry := range entries {
		if entry.IsDir() {
			subCSV := filepath.Join(chartsPath, entry.Name(), entry.Name()+".chartserviceversion.yaml")
			if _, err := os.Stat(subCSV); err == nil {
				csvFiles = append(csvFiles, subCSV)
			}
		}
	}

	return csvFiles, nil
}

func isCSVFile(name string) bool {
	return len(name) > len(".chartserviceversion.yaml") &&
		name[len(name)-len(".chartserviceversion.yaml"):] == ".chartserviceversion.yaml"
}

// readAppCSV reads and parses a single application chartserviceversion YAML file.
func readAppCSV(path string) (*AppChartServiceVersion, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	var csv AppChartServiceVersion
	if err := yaml.Unmarshal(data, &csv); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s: %w", path, err)
	}

	return &csv, nil
}
