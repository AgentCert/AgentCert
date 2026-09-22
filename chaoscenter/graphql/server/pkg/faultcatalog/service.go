package faultcatalog

import (
	"context"
	"fmt"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/apphub"
	chaosHubHandler "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaoshub/handler"
)

// DefaultHubName is the built-in chaos hub whose working copy holds the
// capability catalog. It matches chaoshub.listDefaultHubs.
const DefaultHubName = "Litmus ChaosHub"

// Service exposes the composed catalog to the GraphQL layer and to the
// experiment Save/Run validator.
type Service interface {
	// Catalog returns the composed catalog for a project.
	Catalog(ctx context.Context, projectID string) (*Catalog, error)
	// GetFaultCatalog returns the catalog in its GraphQL shape.
	GetFaultCatalog(ctx context.Context, projectID string) (*model.FaultCatalog, error)
}

type service struct {
	cache *Cache
}

// NewService returns a catalog service backed by an mtime-invalidated cache.
func NewService() Service {
	return &service{cache: NewCache()}
}

// faultsDir resolves the default hub's faults/ directory, honouring
// HUB_SOURCE_MODE=custom (bind-mounted charts) exactly as the chaos hub handler
// does — so the catalog is always read from the same working copy the faults
// themselves are served from, and cannot describe a different revision.
func faultsDir(projectID string) string {
	return chaosHubHandler.GetChartsPath(
		model.CloningInput{Name: DefaultHubName},
		projectID,
		true,
	)
}

// registeredApplications reads the application registry straight from the
// AppsHub working copy, so an application onboarded by adding a chart entry is
// picked up with no server change and no redeploy.
func registeredApplications() ([]Application, error) {
	entries, err := apphub.GetAllAppEntries(apphub.ChartsPath())
	if err != nil {
		return nil, fmt.Errorf("read application registry: %w", err)
	}

	apps := make([]Application, 0, len(entries))
	for _, entry := range entries {
		if entry.Name == "" {
			continue
		}
		labelKey := entry.LabelKey
		if labelKey == "" {
			labelKey = apphub.DefaultAppLabelKey
		}
		services := make([]string, 0, len(entry.Microservices))
		for _, ms := range entry.Microservices {
			if ms.Name != "" {
				services = append(services, ms.Name)
			}
		}
		apps = append(apps, Application{
			Key:       entry.Name,
			Folders:   append([]string{entry.Name}, entry.Aliases...),
			Namespace: entry.Namespace,
			LabelKey:  labelKey,
			Services:  services,
		})
	}
	return apps, nil
}

func (s *service) Catalog(_ context.Context, projectID string) (*Catalog, error) {
	catalog, err := s.cache.Get(faultsDir(projectID), apphub.ChartsPath(), registeredApplications)
	if err != nil {
		return nil, fmt.Errorf("load fault capability catalog: %w", err)
	}
	return catalog, nil
}

func (s *service) GetFaultCatalog(ctx context.Context, projectID string) (*model.FaultCatalog, error) {
	catalog, err := s.Catalog(ctx, projectID)
	if err != nil {
		return nil, err
	}

	out := &model.FaultCatalog{
		Applications: make([]*model.TargetApplication, 0, len(catalog.Applications)),
		Faults:       make([]*model.FaultCompatibility, 0, len(catalog.Faults)),
	}

	for _, app := range catalog.Applications {
		out.Applications = append(out.Applications, &model.TargetApplication{
			Key:       app.Key,
			Folders:   nonNil(app.Folders),
			Namespace: app.Namespace,
			LabelKey:  app.LabelKey,
			Services:  nonNil(app.Services),
		})
	}

	for _, fault := range catalog.Faults {
		out.Faults = append(out.Faults, &model.FaultCompatibility{
			FaultName:           fault.Name,
			Classification:      fault.Classification,
			CompatibleApps:      nonNil(fault.CompatibleApps),
			WorkloadKinds:       nonNil(fault.WorkloadKinds),
			RequiredServices:    nonNil(fault.RequiredServices),
			KnownFailingTargets: nonNil(fault.KnownFailingTargets),
		})
	}

	return out, nil
}

// nonNil keeps non-nullable GraphQL list fields from marshalling as null.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
