package experiment_definition

import (
	"context"
	"fmt"
	"strings"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/fault_catalog"
)

// Service defines experiment definition business logic.
type Service interface {
	Create(ctx context.Context, projectID string, doc *ExperimentDefinitionDoc) error
	GetByName(ctx context.Context, projectID, name string) (*ExperimentDefinitionDoc, error)
	List(ctx context.Context, projectID string, filter ListFilter) ([]*ExperimentDefinitionDoc, error)
	Update(ctx context.Context, projectID, name string, doc *ExperimentDefinitionDoc) error
	Delete(ctx context.Context, projectID, name string) error
}

type experimentService struct {
	repo         Repository
	faultCatalog fault_catalog.Service
}

// NewService returns a new experiment definition service.
func NewService(repo Repository, faultCatalog fault_catalog.Service) Service {
	return &experimentService{repo: repo, faultCatalog: faultCatalog}
}

func (s *experimentService) Create(ctx context.Context, projectID string, doc *ExperimentDefinitionDoc) error {
	doc.ProjectID = projectID
	if err := s.validateFaultRefs(doc); err != nil {
		return err
	}
	return s.repo.Create(ctx, doc)
}

func (s *experimentService) GetByName(ctx context.Context, projectID, name string) (*ExperimentDefinitionDoc, error) {
	return s.repo.GetByName(ctx, projectID, name)
}

func (s *experimentService) List(ctx context.Context, projectID string, filter ListFilter) ([]*ExperimentDefinitionDoc, error) {
	return s.repo.List(ctx, projectID, filter)
}

func (s *experimentService) Update(ctx context.Context, projectID, name string, doc *ExperimentDefinitionDoc) error {
	doc.ProjectID = projectID
	if err := s.validateFaultRefs(doc); err != nil {
		return err
	}
	return s.repo.Update(ctx, projectID, name, doc)
}

func (s *experimentService) Delete(ctx context.Context, projectID, name string) error {
	return s.repo.Delete(ctx, projectID, name)
}

// validateFaultRefs checks that every faultRef in the step list resolves in the fault catalog.
func (s *experimentService) validateFaultRefs(doc *ExperimentDefinitionDoc) error {
	if s.faultCatalog == nil {
		return nil
	}
	var invalid []string
	for _, step := range doc.Steps {
		if step.Type == StepTypeFault && step.FaultRef != "" {
			if _, err := s.faultCatalog.GetFault(step.FaultRef); err != nil {
				invalid = append(invalid, step.FaultRef)
			}
		}
		if step.Type == StepTypeParallelFault {
			for _, pf := range step.Faults {
				if _, err := s.faultCatalog.GetFault(pf.FaultRef); err != nil {
					invalid = append(invalid, pf.FaultRef)
				}
			}
		}
	}
	if len(invalid) > 0 {
		return fmt.Errorf("experiment definition references unknown fault(s): %s",
			strings.Join(invalid, ", "))
	}
	return nil
}
