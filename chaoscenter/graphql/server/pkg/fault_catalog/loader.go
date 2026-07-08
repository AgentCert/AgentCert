package fault_catalog

import (
	"os"
	"path/filepath"
	"sync"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// CatalogIndex is the in-memory index of all loaded faults.
// Key: fault metadata name (unique across the full catalog).
type CatalogIndex struct {
	mu     sync.RWMutex
	byName map[string]*FaultCatalogEntry
	// Convenience slices for filtered queries
	general     []*FaultCatalogEntry
	byDomain    map[string][]*FaultCatalogEntry // domain -> entries
	byTargetApp map[string][]*FaultCatalogEntry // targetApp -> entries
}

var globalIndex = &CatalogIndex{
	byName:      make(map[string]*FaultCatalogEntry),
	byDomain:    make(map[string][]*FaultCatalogEntry),
	byTargetApp: make(map[string][]*FaultCatalogEntry),
}

// LoadCatalog reads all fault.yaml files from the catalog root and populates
// the global in-memory index. catalogRoot is typically the path to the
// catalog/ directory in the ACE monorepo, configurable via ACE_CATALOG_ROOT.
//
// It walks:
//
//	catalogRoot/faults/general/<fault-name>/fault.yaml
//	catalogRoot/faults/domains/<domain>/<fault-name>/fault.yaml
//	catalogRoot/apps/<tier>/<app-name>/faults/<fault-name>/fault.yaml
func LoadCatalog(catalogRoot string) error {
	entries, err := walkFaultYAMLs(catalogRoot)
	if err != nil {
		return err
	}

	globalIndex.mu.Lock()
	defer globalIndex.mu.Unlock()

	// Reset index
	globalIndex.byName = make(map[string]*FaultCatalogEntry)
	globalIndex.general = nil
	globalIndex.byDomain = make(map[string][]*FaultCatalogEntry)
	globalIndex.byTargetApp = make(map[string][]*FaultCatalogEntry)

	for i := range entries {
		e := &entries[i]
		name := e.Metadata.Name

		if _, exists := globalIndex.byName[name]; exists {
			log.Warnf("fault_catalog: duplicate fault name %q in %s — skipping", name, e.FilePath)
			continue
		}

		globalIndex.byName[name] = e

		switch e.Metadata.Scope {
		case ScopeGeneral:
			globalIndex.general = append(globalIndex.general, e)
		case ScopeDomain:
			if e.Metadata.Domain != nil {
				d := *e.Metadata.Domain
				globalIndex.byDomain[d] = append(globalIndex.byDomain[d], e)
			}
		case ScopeAppSpecific:
			if e.Metadata.TargetApp != nil {
				a := *e.Metadata.TargetApp
				globalIndex.byTargetApp[a] = append(globalIndex.byTargetApp[a], e)
			}
		}
	}

	log.Infof("fault_catalog: loaded %d faults (%d general, %d domain-specific, %d app-specific)",
		len(globalIndex.byName),
		len(globalIndex.general),
		totalDomainFaults(),
		totalAppFaults(),
	)
	return nil
}

// walkFaultYAMLs finds all fault.yaml files under catalogRoot and returns
// the parsed FaultCatalogEntry structs.
func walkFaultYAMLs(catalogRoot string) ([]FaultCatalogEntry, error) {
	var entries []FaultCatalogEntry

	err := filepath.Walk(catalogRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Base(path) != "fault.yaml" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			log.Warnf("fault_catalog: failed to read %s: %v", path, err)
			return nil // skip, don't abort entire load
		}

		var entry FaultCatalogEntry
		if err := yaml.Unmarshal(data, &entry); err != nil {
			log.Warnf("fault_catalog: failed to parse %s: %v — skipping", path, err)
			return nil
		}

		if entry.Kind != "FaultCatalogEntry" {
			return nil // skip non-fault YAML files (e.g., app.yaml)
		}

		entry.FilePath = path
		entries = append(entries, entry)
		log.Debugf("fault_catalog: loaded fault %q from %s", entry.Metadata.Name, path)
		return nil
	})

	return entries, err
}

func totalDomainFaults() int {
	n := 0
	for _, v := range globalIndex.byDomain {
		n += len(v)
	}
	return n
}

func totalAppFaults() int {
	n := 0
	for _, v := range globalIndex.byTargetApp {
		n += len(v)
	}
	return n
}
