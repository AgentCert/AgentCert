package experiment_definition

import (
	"context"
	"testing"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/fault_catalog"
)

// mockFaultCatalog is a test double that returns ErrFaultNotFound for unknown names.
type mockFaultCatalog struct {
	known map[string]bool
}

func (m *mockFaultCatalog) ListFaults(scope fault_catalog.FaultScope, domain string, targetApp string) []*fault_catalog.FaultCatalogEntry {
	return nil
}

func (m *mockFaultCatalog) GetFault(name string) (*fault_catalog.FaultCatalogEntry, error) {
	if m.known[name] {
		return &fault_catalog.FaultCatalogEntry{
			Metadata: fault_catalog.FaultMetadata{Name: name},
		}, nil
	}
	return nil, fault_catalog.ErrFaultNotFound{Name: name}
}

func (m *mockFaultCatalog) FaultsForApp(ctx context.Context, appName string) ([]*fault_catalog.FaultCatalogEntry, error) {
	return nil, nil
}

// mockRepository is a no-op repository for unit tests.
type mockRepository struct{}

func (r *mockRepository) Create(ctx context.Context, doc *ExperimentDefinitionDoc) error {
	return nil
}
func (r *mockRepository) GetByName(ctx context.Context, projectID, name string) (*ExperimentDefinitionDoc, error) {
	return nil, ErrExperimentNotFound{Name: name}
}
func (r *mockRepository) List(ctx context.Context, projectID string, filter ListFilter) ([]*ExperimentDefinitionDoc, error) {
	return nil, nil
}
func (r *mockRepository) Update(ctx context.Context, projectID, name string, update *ExperimentDefinitionDoc) error {
	return nil
}
func (r *mockRepository) Delete(ctx context.Context, projectID, name string) error {
	return nil
}

func TestValidateFaultRefs_UnknownFault(t *testing.T) {
	catalog := &mockFaultCatalog{known: map[string]bool{"pod-delete": true}}
	svc := NewService(&mockRepository{}, catalog)

	doc := &ExperimentDefinitionDoc{
		Name:      "test-exp",
		ProjectID: "proj1",
		Steps: []ExperimentStep{
			{Name: "inject", Type: StepTypeFault, FaultRef: "nonexistent-fault"},
		},
		ModelSelection: ModelSelection{Mode: ModelSelectionAgentDefault},
		TargetApp:      TargetAppSpec{Name: "sock-shop", Version: ">=1.0.0"},
	}

	err := svc.Create(context.Background(), "proj1", doc)
	if err == nil {
		t.Fatal("expected error for unknown faultRef, got nil")
	}
}

func TestValidateFaultRefs_KnownFault(t *testing.T) {
	catalog := &mockFaultCatalog{known: map[string]bool{"pod-delete": true}}
	svc := NewService(&mockRepository{}, catalog)

	doc := &ExperimentDefinitionDoc{
		Name:      "test-exp",
		ProjectID: "proj1",
		Steps: []ExperimentStep{
			{Name: "baseline", Type: StepTypeObserve, Duration: "30s"},
			{Name: "inject", Type: StepTypeFault, FaultRef: "pod-delete"},
		},
		ModelSelection: ModelSelection{Mode: ModelSelectionAgentDefault},
		TargetApp:      TargetAppSpec{Name: "sock-shop", Version: ">=1.0.0"},
	}

	err := svc.Create(context.Background(), "proj1", doc)
	if err != nil {
		t.Fatalf("expected no error for known faultRef, got: %v", err)
	}
}

func TestValidateFaultRefs_NilCatalog(t *testing.T) {
	svc := NewService(&mockRepository{}, nil)

	doc := &ExperimentDefinitionDoc{
		Name:      "test-exp",
		ProjectID: "proj1",
		Steps: []ExperimentStep{
			{Name: "inject", Type: StepTypeFault, FaultRef: "any-fault"},
		},
		ModelSelection: ModelSelection{Mode: ModelSelectionAgentDefault},
		TargetApp:      TargetAppSpec{Name: "sock-shop", Version: ">=1.0.0"},
	}

	// nil catalog means no validation — should not error
	err := svc.Create(context.Background(), "proj1", doc)
	if err != nil {
		t.Fatalf("expected no error with nil fault catalog, got: %v", err)
	}
}
