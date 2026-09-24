// Package faultcatalog answers the two questions the Chaos Studio builder and
// the experiment Save/Run validator both need:
//
//	which applications may this fault be pointed at?
//	which application does this install-application step actually install?
//
// It composes two independently-onboarded sources rather than owning a copy of
// either:
//
//	applications — app-charts/charts/applications.chartserviceversion.yaml
//	               (the AppsHub), so onboarding an app is a chart edit only
//	faults       — chaos-charts/faults/fault-capabilities.yaml (the ChaosHub),
//	               which classifies a fault but never lists applications
//
// Every default is permissive, so extending one side never requires editing the
// other: a newly onboarded application works with every generic fault, and a
// fault missing from the capability catalog is treated as generic rather than as
// unusable. Restriction is opt-in, via a fault's requiredApp or an agent's
// compatibleApplications.
package faultcatalog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
)

// CatalogFileName is the capability catalog's fixed name inside a hub's faults/ directory.
const CatalogFileName = "fault-capabilities.yaml"

const (
	// ClassificationGeneric marks a fault that works against any target application.
	ClassificationGeneric = "generic"
	// ClassificationAppSpecific marks a fault pinned to one application by name.
	ClassificationAppSpecific = "application-specific"
)

// Application is one registered target application.
type Application struct {
	Key       string
	Folders   []string
	Namespace string
	LabelKey  string
	Services  []string
}

// Fault is one fault's resolved compatibility. CompatibleApps is derived against
// the live application registry, never read verbatim from a file.
type Fault struct {
	Name                string
	Classification      string
	CompatibleApps      []string
	WorkloadKinds       []string
	RequiredServices    []string
	KnownFailingTargets []string
}

// Catalog is an immutable, resolved view over both hubs.
type Catalog struct {
	Applications []Application
	Faults       []Fault

	byAppKey   map[string]Application
	byFolder   map[string]string // folder or alias -> app key
	byAppNS    map[string]string // namespace -> app key
	faultIndex map[string]Fault
}

// ---------------------------------------------------------------------------
// On-disk shape of the capability catalog
// ---------------------------------------------------------------------------

type rawIncompatibility struct {
	KnownFailingTargets []string `yaml:"knownFailingTargets"`
}

type rawTargetRequirements struct {
	WorkloadKinds    []string `yaml:"workloadKinds"`
	RequiredApp      string   `yaml:"requiredApp"`
	RequiredServices []string `yaml:"requiredServices"`
}

type rawFault struct {
	Classification     string                `yaml:"classification"`
	TargetRequirements rawTargetRequirements `yaml:"targetRequirements"`
	IncompatibleWith   []rawIncompatibility  `yaml:"incompatibleWith"`
}

type rawCatalog struct {
	Spec struct {
		Faults map[string]rawFault `yaml:"faults"`
	} `yaml:"spec"`
}

// ---------------------------------------------------------------------------
// Loading
// ---------------------------------------------------------------------------

// CatalogPath returns the capability catalog file to read for a hub's faults/
// directory.
//
// FAULT_CAPABILITIES_PATH overrides the hub copy. The hub's own copy lives
// inside a directory the server git-clones at request time, and mounting a file
// into it pre-creates that directory, which makes the clone fail -- so an
// override needs a path outside the clone rather than a mount over it.
func CatalogPath(faultsDir string) string {
	if override := strings.TrimSpace(os.Getenv("FAULT_CAPABILITIES_PATH")); override != "" {
		return override
	}
	return filepath.Join(faultsDir, CatalogFileName)
}

// loadFaults parses the capability catalog from a hub's faults/ directory.
//
// A missing catalog is not an error: a custom chaos hub need not ship one, and
// every fault in it is then treated as generic. Only a malformed one fails.
func loadFaults(faultsDir string) (map[string]rawFault, error) {
	path := CatalogPath(faultsDir)
	data, err := os.ReadFile(path) // #nosec G304 -- path is server-controlled hub content
	if err != nil {
		if os.IsNotExist(err) {
			if strings.TrimSpace(os.Getenv("FAULT_CAPABILITIES_PATH")) == "" {
				return map[string]rawFault{}, nil
			}
			return nil, fmt.Errorf("configured fault capability catalog does not exist: %s", path)
		}
		return nil, fmt.Errorf("read fault capability catalog %s: %w", path, err)
	}

	var raw rawCatalog
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse fault capability catalog %s: %w", path, err)
	}
	if raw.Spec.Faults == nil {
		return map[string]rawFault{}, nil
	}
	return raw.Spec.Faults, nil
}

// Build resolves an application registry and a fault classification map into a
// catalog. Applications are supplied by the caller (from the AppsHub) so this
// package never needs to know how an application is onboarded.
func Build(apps []Application, faults map[string]rawFault) *Catalog {
	c := &Catalog{
		byAppKey:   make(map[string]Application, len(apps)),
		byFolder:   make(map[string]string),
		byAppNS:    make(map[string]string),
		faultIndex: make(map[string]Fault, len(faults)),
	}

	sorted := append([]Application(nil), apps...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })

	appKeys := make([]string, 0, len(sorted))
	for _, app := range sorted {
		if app.Key == "" {
			continue
		}
		c.Applications = append(c.Applications, app)
		c.byAppKey[app.Key] = app
		appKeys = append(appKeys, app.Key)
		for _, folder := range app.Folders {
			c.byFolder[normalize(folder)] = app.Key
		}
		if app.Namespace != "" {
			// First registration wins, so two apps sharing a namespace cannot
			// make an earlier one unresolvable.
			if _, taken := c.byAppNS[normalize(app.Namespace)]; !taken {
				c.byAppNS[normalize(app.Namespace)] = app.Key
			}
		}
	}

	faultNames := make([]string, 0, len(faults))
	for name := range faults {
		faultNames = append(faultNames, name)
	}
	sort.Strings(faultNames)

	for _, name := range faultNames {
		raw := faults[name]
		fault := Fault{
			Name:             name,
			Classification:   raw.Classification,
			WorkloadKinds:    append([]string(nil), raw.TargetRequirements.WorkloadKinds...),
			RequiredServices: append([]string(nil), raw.TargetRequirements.RequiredServices...),
			CompatibleApps:   deriveCompatibleApps(raw, appKeys, c.byAppKey),
		}
		for _, inc := range raw.IncompatibleWith {
			fault.KnownFailingTargets = append(fault.KnownFailingTargets, inc.KnownFailingTargets...)
		}
		c.Faults = append(c.Faults, fault)
		c.faultIndex[name] = fault
	}

	return c
}

// deriveCompatibleApps implements the catalog's stated rule: a generic fault is
// compatible with every registered application — so onboarding an application
// widens every generic fault with no further edit — and an application-specific
// one only with the application it pins.
//
// A pin naming an application that is not (or no longer) registered yields an
// empty set rather than silently widening to everything; the CI validator turns
// that case into a hard error so it surfaces at build time.
func deriveCompatibleApps(raw rawFault, appKeys []string, known map[string]Application) []string {
	if raw.Classification == ClassificationAppSpecific || raw.TargetRequirements.RequiredApp != "" {
		required := raw.TargetRequirements.RequiredApp
		if _, ok := known[required]; !ok {
			return []string{}
		}
		return []string{required}
	}
	return append([]string(nil), appKeys...)
}

func normalize(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// ---------------------------------------------------------------------------
// Queries
// ---------------------------------------------------------------------------

// ResolveApplication maps an install-application step's `-folder=` and
// `-namespace=` args onto a registered application. The folder wins: an operator
// may override the namespace, but not the chart the step installs.
func (c *Catalog) ResolveApplication(folder, namespace string) (Application, bool) {
	if c == nil {
		return Application{}, false
	}
	if key, ok := c.byFolder[normalize(folder)]; ok {
		return c.byAppKey[key], true
	}
	if key, ok := c.byAppNS[normalize(namespace)]; ok {
		return c.byAppKey[key], true
	}
	return Application{}, false
}

// Lookup returns a fault's resolved compatibility.
func (c *Catalog) Lookup(faultName string) (Fault, bool) {
	if c == nil {
		return Fault{}, false
	}
	fault, ok := c.faultIndex[faultName]
	return fault, ok
}

// IsFaultCompatible reports whether a fault may target an application.
//
// A fault absent from the catalog is compatible with everything: the catalog
// records known restrictions, it is not a registry of permitted faults, so
// onboarding a fault does not require a catalog entry before it can be used.
func (c *Catalog) IsFaultCompatible(faultName, appKey string) bool {
	fault, ok := c.Lookup(faultName)
	if !ok {
		return true
	}
	for _, app := range fault.CompatibleApps {
		if app == appKey {
			return true
		}
	}
	return false
}

// IsAgentCompatible reports whether an agent may be paired with an application.
//
// A nil declaration means the agent stated no restriction and works with every
// application, including ones onboarded after it. An explicitly empty list means
// the agent is deliberately not application-targeted.
func IsAgentCompatible(declared []string, appKey string) bool {
	if declared == nil {
		return true
	}
	for _, app := range declared {
		if app == appKey {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Caching
// ---------------------------------------------------------------------------

// Cache memoizes composed catalogs per (faults dir, app charts dir), invalidating
// when either source changes on disk. Both hubs are re-synced on a timer, so a
// TTL alone would either serve a stale catalog or re-read on every keystroke in
// the fault picker.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	catalog *Catalog
	stamps  []fileStamp
}

type fileStamp struct {
	path    string
	modTime time.Time
	size    int64
	missing bool
}

// NewCache returns an empty catalog cache.
func NewCache() *Cache {
	return &Cache{entries: make(map[string]cacheEntry)}
}

func stampFile(path string) fileStamp {
	info, err := os.Stat(path)
	if err != nil {
		return fileStamp{path: path, missing: true}
	}
	return fileStamp{path: path, modTime: info.ModTime(), size: info.Size()}
}

// stampDir summarizes an app-charts directory. The registry is spread across
// every *.chartserviceversion.yaml in it, and a new file appearing is exactly
// the "an application was onboarded" case the cache must notice — so the newest
// entry mtime and the total size are stamped, not just the directory's own.
func stampDir(dir string) fileStamp {
	info, err := os.Stat(dir)
	if err != nil {
		return fileStamp{path: dir, missing: true}
	}
	newest := info.ModTime()
	var total int64
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fileStamp{path: dir, modTime: newest}
	}
	for _, e := range entries {
		fi, err := e.Info()
		if err != nil {
			continue
		}
		if fi.ModTime().After(newest) {
			newest = fi.ModTime()
		}
		total += fi.Size()
	}
	return fileStamp{path: dir, modTime: newest, size: total}
}

func stampsEqual(a, b []fileStamp) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Get returns the composed catalog, reloading it if either source changed.
// loadApps is invoked only on a miss.
func (c *Cache) Get(faultsDir, appChartsDir string, loadApps func() ([]Application, error)) (*Catalog, error) {
	key := faultsDir + "\x00" + appChartsDir
	current := []fileStamp{
		stampFile(CatalogPath(faultsDir)),
		stampDir(appChartsDir),
	}

	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if ok && stampsEqual(entry.stamps, current) {
		return entry.catalog, nil
	}

	faults, err := loadFaults(faultsDir)
	if err != nil {
		return nil, err
	}
	apps, err := loadApps()
	if err != nil {
		return nil, err
	}
	catalog := Build(apps, faults)

	c.mu.Lock()
	c.entries[key] = cacheEntry{catalog: catalog, stamps: current}
	c.mu.Unlock()

	return catalog, nil
}
