package fault_catalog

import (
	"context"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/catalog"
)

// Service is the interface for fault catalog queries.
type Service interface {
	// ListFaults returns faults filtered by optional scope, domain, or targetApp.
	// Pass empty strings to return all faults in a category.
	ListFaults(scope FaultScope, domain string, targetApp string) []*FaultCatalogEntry

	// GetFault returns the fault with the given name, or ErrFaultNotFound.
	GetFault(name string) (*FaultCatalogEntry, error)

	// FaultsForApp returns all faults applicable to the given app (general +
	// domain-matching + app-specific), minus any in incompatibleApps for that app.
	// It looks up the app's domain from the catalog service internally.
	FaultsForApp(ctx context.Context, appName string) ([]*FaultCatalogEntry, error)
}

type catalogService struct {
	index      *CatalogIndex
	appCatalog catalog.Service
}

// NewService returns a new fault catalog Service backed by the global index.
// Call LoadCatalog() before creating a Service.
// appCatalog may be nil — FaultsForApp will fall back to general + app-specific only.
func NewService(appCatalog catalog.Service) Service {
	return &catalogService{index: globalIndex, appCatalog: appCatalog}
}

func (s *catalogService) ListFaults(scope FaultScope, domain string, targetApp string) []*FaultCatalogEntry {
	s.index.mu.RLock()
	defer s.index.mu.RUnlock()

	if scope == "" && domain == "" && targetApp == "" {
		// Return all faults
		all := make([]*FaultCatalogEntry, 0, len(s.index.byName))
		for _, e := range s.index.byName {
			all = append(all, e)
		}
		return all
	}

	var result []*FaultCatalogEntry
	for _, e := range s.index.byName {
		if scope != "" && e.Metadata.Scope != scope {
			continue
		}
		if domain != "" && (e.Metadata.Domain == nil || *e.Metadata.Domain != domain) {
			continue
		}
		if targetApp != "" && (e.Metadata.TargetApp == nil || *e.Metadata.TargetApp != targetApp) {
			continue
		}
		result = append(result, e)
	}
	return result
}

func (s *catalogService) GetFault(name string) (*FaultCatalogEntry, error) {
	s.index.mu.RLock()
	defer s.index.mu.RUnlock()

	e, ok := s.index.byName[name]
	if !ok {
		return nil, ErrFaultNotFound{Name: name}
	}
	return e, nil
}

// FaultsForApp returns all faults applicable to the given app name.
// It looks up the app's domain and capabilityDomains from the catalog service.
func (s *catalogService) FaultsForApp(ctx context.Context, appName string) ([]*FaultCatalogEntry, error) {
	var primaryDomain string
	var capabilityDomains []string

	if s.appCatalog != nil {
		app, err := s.appCatalog.GetApplication(ctx, "", appName)
		if err == nil && app != nil {
			primaryDomain = app.Metadata.Domain
			capabilityDomains = app.Metadata.CapabilityDomains
		}
		// If not found, fall back gracefully — general + app-specific only
	}

	return s.faultsForAppInternal(appName, primaryDomain, capabilityDomains), nil
}

// faultsForAppInternal does the actual merge and filtering.
func (s *catalogService) faultsForAppInternal(
	appName string,
	primaryDomain string,
	capabilityDomains []string,
) []*FaultCatalogEntry {
	s.index.mu.RLock()
	defer s.index.mu.RUnlock()

	seen := make(map[string]bool)
	var result []*FaultCatalogEntry

	add := func(e *FaultCatalogEntry) {
		if seen[e.Metadata.Name] {
			return
		}
		// Exclude incompatible apps
		for _, incompatible := range e.Spec.Compatibility.IncompatibleApps {
			if incompatible == appName {
				return
			}
		}
		seen[e.Metadata.Name] = true
		result = append(result, e)
	}

	// Step 1: All general faults
	for _, e := range s.index.general {
		add(e)
	}

	// Step 2: Domain faults for primaryDomain
	if primaryDomain != "" {
		for _, e := range s.index.byDomain[primaryDomain] {
			add(e)
		}
	}

	// Step 3: Domain faults for each capabilityDomain (union, no duplicates)
	for _, cd := range capabilityDomains {
		if cd == primaryDomain {
			continue // already added
		}
		for _, e := range s.index.byDomain[cd] {
			add(e)
		}
	}

	// Step 4: App-specific faults for this app
	for _, e := range s.index.byTargetApp[appName] {
		add(e)
	}

	return result
}
