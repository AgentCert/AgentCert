package agenthub

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// ContextInjectionMapping represents a single Helm --set arg pair that the server
// injects into the install template at experiment-save time.
type ContextInjectionMapping struct {
	HelmPath string `yaml:"helmPath"`
	Source   string `yaml:"source"`
}

// AgentEntry represents a single agent in the chartserviceversion YAML.
type AgentEntry struct {
	Name                string                    `yaml:"name"`
	DisplayName         string                    `yaml:"displayName"`
	Description         string                    `yaml:"description"`
	Version             string                    `yaml:"version"`
	Capabilities        []string                  `yaml:"capabilities"`
	InstallTemplateName string                    `yaml:"installTemplateName,omitempty"`
	InstallImage        string                    `yaml:"installImage,omitempty"`
	ContextInjection    []ContextInjectionMapping `yaml:"contextInjection,omitempty"`
}

// AgentCSVSpec is the spec section of the agent chartserviceversion YAML.
type AgentCSVSpec struct {
	DisplayName         string       `yaml:"displayName"`
	CategoryDescription string       `yaml:"categoryDescription"`
	Keywords            []string     `yaml:"keywords"`
	Agents              []AgentEntry `yaml:"agents"`
}

// AgentCSVMetadata is the metadata section of the agent chartserviceversion YAML.
type AgentCSVMetadata struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Annotations struct {
		Categories       string `yaml:"categories"`
		Vendor           string `yaml:"vendor"`
		ChartDescription string `yaml:"chartDescription"`
	} `yaml:"annotations"`
}

// AgentChartServiceVersion represents the full chartserviceversion YAML for agents.
type AgentChartServiceVersion struct {
	APIVersion string           `yaml:"apiVersion"`
	Kind       string           `yaml:"kind"`
	Metadata   AgentCSVMetadata `yaml:"metadata"`
	Spec       AgentCSVSpec     `yaml:"spec"`
}

// GetAgentChartsData reads the agents chartserviceversion.yaml from the given path
// and returns AgentHubCategory entries, analogous to handler.GetChartsData for ChaosHub.
func GetAgentChartsData(chartsPath string) ([]*model.AgentHubCategory, error) {
	csvFiles, err := findCSVFiles(chartsPath)
	if err != nil {
		return nil, fmt.Errorf("error reading agent charts directory %s: %w", chartsPath, err)
	}

	var categories []*model.AgentHubCategory
	for _, csvFile := range csvFiles {
		csv, err := readAgentCSV(csvFile)
		if err != nil {
			log.WithError(err).Errorf("failed to read agent CSV file: %s", csvFile)
			continue
		}

		category := &model.AgentHubCategory{
			DisplayName:         csv.Spec.DisplayName,
			CategoryDescription: csv.Spec.CategoryDescription,
			Agents:              make([]*model.AgentHubEntry, 0, len(csv.Spec.Agents)),
		}

		for _, agent := range csv.Spec.Agents {
			entry := &model.AgentHubEntry{
				Name:         agent.Name,
				DisplayName:  agent.DisplayName,
				Description:  agent.Description,
				Version:      agent.Version,
				Capabilities: agent.Capabilities,
				IsDeployed:   false, // Will be enriched later
			}

			// Populate injection metadata from CSV (Item #3)
			if agent.InstallTemplateName != "" {
				s := agent.InstallTemplateName
				entry.InstallTemplateName = &s
			}
			if agent.InstallImage != "" {
				s := agent.InstallImage
				entry.InstallImage = &s
			}
			if len(agent.ContextInjection) > 0 {
				ci := make([]*model.ContextInjectionMapping, 0, len(agent.ContextInjection))
				for _, m := range agent.ContextInjection {
					ci = append(ci, &model.ContextInjectionMapping{
						HelmPath: m.HelmPath,
						Source:   m.Source,
					})
				}
				entry.ContextInjection = ci
			}

			category.Agents = append(category.Agents, entry)
		}

		categories = append(categories, category)
	}

	return categories, nil
}

// GetAllAgentEntries reads all agent entries from the chartserviceversion files,
// preserving the full metadata including contextInjection, installTemplateName,
// and installImage. Used by the experiment service for metadata-driven injection.
func GetAllAgentEntries(chartsPath string) ([]AgentEntry, error) {
	csvFiles, err := findCSVFiles(chartsPath)
	if err != nil {
		return nil, fmt.Errorf("error reading agent charts directory %s: %w", chartsPath, err)
	}

	var entries []AgentEntry
	for _, csvFile := range csvFiles {
		csv, err := readAgentCSV(csvFile)
		if err != nil {
			log.WithError(err).Errorf("failed to read agent CSV file: %s", csvFile)
			continue
		}
		entries = append(entries, csv.Spec.Agents...)
	}
	return entries, nil
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

// readAgentCSV reads and parses a single agent chartserviceversion YAML file.
func readAgentCSV(path string) (*AgentChartServiceVersion, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	var csv AgentChartServiceVersion
	if err := yaml.Unmarshal(data, &csv); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s: %w", path, err)
	}

	return &csv, nil
}
