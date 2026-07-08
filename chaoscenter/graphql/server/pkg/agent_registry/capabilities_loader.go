package agent_registry

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// CapabilityVocab holds all known capabilities keyed by capability key.
type CapabilityVocab struct {
	byKey    map[string]CapabilityEntry
	byDomain map[string][]CapabilityEntry
}

// CapabilityEntry represents a single capability from the vocabulary.
type CapabilityEntry struct {
	Key           string
	DisplayName   string
	Description   string
	Domain        string
	Category      string
	RelatedFaults []string
}

type capabilityDomainFile struct {
	Domain      string `yaml:"domain"`
	DisplayName string `yaml:"displayName"`
	Description string `yaml:"description"`
	Capabilities struct {
		Observe []capabilityYAML `yaml:"observe"`
		Act     []capabilityYAML `yaml:"act"`
	} `yaml:"capabilities"`
}

type capabilityYAML struct {
	Key           string   `yaml:"key"`
	DisplayName   string   `yaml:"displayName"`
	Description   string   `yaml:"description"`
	RelatedFaults []string `yaml:"relatedFaults"`
}

// LoadCapabilitiesFromDir reads all *.yaml files in dir and builds a CapabilityVocab.
func LoadCapabilitiesFromDir(dir string) (*CapabilityVocab, error) {
	vocab := &CapabilityVocab{
		byKey:    make(map[string]CapabilityEntry),
		byDomain: make(map[string][]CapabilityEntry),
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading capabilities dir %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", entry.Name(), err)
		}
		var df capabilityDomainFile
		if err := yaml.Unmarshal(data, &df); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", entry.Name(), err)
		}
		for _, cap := range df.Capabilities.Observe {
			e := CapabilityEntry{Key: cap.Key, DisplayName: cap.DisplayName,
				Description: cap.Description, Domain: df.Domain,
				Category: "observe", RelatedFaults: cap.RelatedFaults}
			vocab.byKey[cap.Key] = e
			vocab.byDomain[df.Domain] = append(vocab.byDomain[df.Domain], e)
		}
		for _, cap := range df.Capabilities.Act {
			e := CapabilityEntry{Key: cap.Key, DisplayName: cap.DisplayName,
				Description: cap.Description, Domain: df.Domain,
				Category: "act", RelatedFaults: cap.RelatedFaults}
			vocab.byKey[cap.Key] = e
			vocab.byDomain[df.Domain] = append(vocab.byDomain[df.Domain], e)
		}
	}
	return vocab, nil
}

// IsValid returns true if the key exists in the loaded vocabulary.
func (v *CapabilityVocab) IsValid(key string) bool {
	if v == nil {
		return true // permissive when vocab not loaded
	}
	_, ok := v.byKey[key]
	return ok
}

// AllEntries returns all capability entries sorted by domain then key.
func (v *CapabilityVocab) AllEntries() []CapabilityEntry {
	entries := make([]CapabilityEntry, 0, len(v.byKey))
	for _, e := range v.byKey {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Domain != entries[j].Domain {
			return entries[i].Domain < entries[j].Domain
		}
		return entries[i].Key < entries[j].Key
	})
	return entries
}
