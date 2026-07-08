package catalog

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// Service provides access to the app catalog.
type Service interface {
	ListApplications(ctx context.Context, projectID string) ([]*AppCatalogEntry, error)
	GetApplication(ctx context.Context, projectID, appName string) (*AppCatalogEntry, error)
	CatalogDir() string
}

type catalogService struct {
	mu         sync.RWMutex
	index      map[string]*AppCatalogEntry
	ordered    []*AppCatalogEntry
	catalogDir string
}

// NewService creates a CatalogService, loads the catalog immediately,
// and starts a SIGHUP listener for zero-downtime reloads.
func NewService(catalogDir string) (Service, error) {
	if catalogDir == "" {
		catalogDir = "/catalog"
	}
	svc := &catalogService{catalogDir: catalogDir}
	if err := svc.reload(); err != nil {
		return nil, fmt.Errorf("initial catalog load failed: %w", err)
	}
	go svc.listenForReload()
	return svc, nil
}

func (s *catalogService) reload() error {
	entries, err := LoadAll(s.catalogDir)
	if err != nil {
		return err
	}

	index := make(map[string]*AppCatalogEntry, len(entries))
	for _, e := range entries {
		index[e.Metadata.Name] = e
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.index = index
	s.ordered = entries

	log.WithField("count", len(entries)).Info("catalog loaded")
	return nil
}

func (s *catalogService) listenForReload() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	for range ch {
		log.Info("SIGHUP received — reloading catalog")
		if err := s.reload(); err != nil {
			log.WithError(err).Error("catalog reload failed")
		}
	}
}

func (s *catalogService) ListApplications(_ context.Context, _ string) ([]*AppCatalogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*AppCatalogEntry, len(s.ordered))
	copy(result, s.ordered)
	return result, nil
}

func (s *catalogService) GetApplication(_ context.Context, _, appName string) (*AppCatalogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if spec, ok := s.index[appName]; ok {
		return spec, nil
	}
	return nil, nil
}

func (s *catalogService) CatalogDir() string {
	return s.catalogDir
}
