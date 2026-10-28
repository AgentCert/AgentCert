// Package model_config is the registry of LLM models a project onboards from
// the UI. MongoDB holds the record (which models exist, who owns them, their
// status); LiteLLM holds the API keys and serves the models. Every write
// records its intent in MongoDB before calling LiteLLM, so an interruption
// leaves a status that reconcile completes.
package model_config

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chartconfig"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
	dbChaosExperiment "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment"
	dbChaosExperimentRun "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment_run"
	dbModelConfig "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/model_config"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
)

// Service is the interface for the LLM model registry
type Service interface {
	List(ctx context.Context, projectID string) ([]*model.ModelConfig, error)
	Create(ctx context.Context, projectID string, request model.CreateModelConfigInput, username string) (*model.ModelConfig, error)
	Delete(ctx context.Context, projectID, modelID, username string) error
	UpdateKey(ctx context.Context, projectID, modelID, apiKey, username string) (*model.ModelConfig, error)
	TestConnection(ctx context.Context, request model.ModelConnectionInput) (*model.ModelTestResult, error)
	Test(ctx context.Context, projectID, modelID string) (*model.ModelTestResult, error)
	// AgentModelOptions are the models Studio offers a project: the platform
	// models plus the project's active ones. It never fails; without MongoDB
	// it falls back to the platform models.
	AgentModelOptions(ctx context.Context, projectID string) []*model.AgentModelOption
	// ValidateAliases is the save and run gate (ops.ModelAliasValidator).
	ValidateAliases(ctx context.Context, projectID string, aliases []string) error
	// Reconcile completes interrupted writes and notices models LiteLLM lost.
	Reconcile(ctx context.Context) error
	// RunReconciler reconciles until ctx ends.
	RunReconciler(ctx context.Context)
}

// Store is the modelConfig collection (dbModelConfig.Operator).
type Store interface {
	Insert(ctx context.Context, modelConfig dbModelConfig.ModelConfig) error
	Get(ctx context.Context, modelID string) (*dbModelConfig.ModelConfig, error)
	GetByAlias(ctx context.Context, alias string) (*dbModelConfig.ModelConfig, error)
	ListByProject(ctx context.Context, projectID string, statuses ...dbModelConfig.SyncStatus) ([]dbModelConfig.ModelConfig, error)
	ListLive(ctx context.Context) ([]dbModelConfig.ModelConfig, error)
	Update(ctx context.Context, modelID string, guard dbModelConfig.Guard, changes dbModelConfig.Changes) (bool, error)
	Remove(ctx context.Context, modelID string, guard dbModelConfig.Guard) (bool, error)
}

// RunLister and ExperimentLister read what the delete guard checks: the
// project's in-flight runs and its experiments.
type RunLister interface {
	GetExperimentRuns(query bson.D) ([]dbChaosExperimentRun.ChaosExperimentRun, error)
}

type ExperimentLister interface {
	GetExperiments(query bson.D) ([]dbChaosExperiment.ChaosExperimentRequest, error)
}

type service struct {
	store       Store
	gateway     LiteLLMClient
	runs        RunLister
	experiments ExperimentLister
	now         func() time.Time

	// reconcileMu serializes reconcile passes; warnedOrphans keeps the
	// warning about a LiteLLM model ACE did not create to one per model.
	reconcileMu   sync.Mutex
	warnedOrphans map[string]bool
}

// NewService returns the model registry service.
func NewService(store Store, gateway LiteLLMClient, runs RunLister, experiments ExperimentLister) Service {
	return &service{store: store, gateway: gateway, runs: runs, experiments: experiments, now: time.Now, warnedOrphans: map[string]bool{}}
}

func (s *service) List(ctx context.Context, projectID string) ([]*model.ModelConfig, error) {
	rows, err := s.store.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	configs := make([]*model.ModelConfig, 0, len(rows))
	for _, row := range rows {
		configs = append(configs, toModelConfig(row))
	}
	return configs, nil
}

func (s *service) Create(ctx context.Context, projectID string, request model.CreateModelConfigInput, username string) (*model.ModelConfig, error) {
	alias, err := normalizeAlias(request.Alias)
	if err != nil {
		return nil, err
	}
	label, err := normalizeLabel(valueOf(request.Label), alias)
	if err != nil {
		return nil, err
	}
	if request.Connection == nil {
		return nil, invalidf("the connection is required")
	}
	conn, err := normalizeConnection(*request.Connection)
	if err != nil {
		return nil, err
	}
	if isPlatformAlias(alias) {
		return nil, aliasTaken(alias)
	}
	// LiteLLM load-balances deployments that share a name, so an alias it
	// already serves (a config model, or one created outside ACE) would
	// silently split traffic between two models.
	remote, err := s.gateway.ListModels(ctx)
	if err != nil {
		return nil, err
	}
	for _, served := range remote {
		if served.Alias == alias {
			return nil, aliasTaken(alias)
		}
	}

	now := s.now().UnixMilli()
	user := mongodb.UserDetailResponse{Username: username}
	row := dbModelConfig.ModelConfig{
		Audit:      mongodb.Audit{CreatedAt: now, UpdatedAt: now, CreatedBy: user, UpdatedBy: user},
		ProjectID:  projectID,
		ModelID:    uuid.NewString(),
		Alias:      alias,
		Label:      label,
		Provider:   string(conn.provider),
		Model:      conn.model,
		APIBase:    conn.apiBase,
		APIVersion: conn.apiVersion,
		APIKeyHint: keyHint(conn.apiKey),
		SyncStatus: dbModelConfig.Pending,
	}
	// The unique alias index is the race guard: of two concurrent creates of
	// one alias, the second fails here, before LiteLLM is called.
	if err := s.store.Insert(ctx, row); err != nil {
		if errors.Is(err, dbModelConfig.ErrDuplicateAlias) {
			return nil, aliasTaken(alias)
		}
		return nil, err
	}
	logFields := logrus.Fields{"projectId": projectID, "modelId": row.ModelID, "alias": alias}

	err = s.gateway.AddModel(ctx, conn.deployment(row.ModelID, alias), keyToSend(conn.apiKey))
	switch {
	case err == nil:
		row.SyncStatus = dbModelConfig.Active
		ok, err := s.store.Update(ctx, row.ModelID, dbModelConfig.Guard{Statuses: []dbModelConfig.SyncStatus{dbModelConfig.Pending}},
			dbModelConfig.Changes{Status: dbModelConfig.Active})
		if err != nil {
			// LiteLLM serves the model; reconcile records it on its next pass.
			logrus.WithFields(logFields).WithError(err).Warn("[Model Registry] model registered but not marked active")
		} else if !ok {
			if err := s.settleRacedCreate(ctx, row); err != nil {
				return nil, err
			}
		}
		logrus.WithFields(logFields).Info("[Model Registry] model registered")
		return toModelConfig(row), nil
	case errors.Is(err, ErrUnreachable):
		s.recordError(ctx, row.ModelID, err)
		logrus.WithFields(logFields).WithError(err).Warn("[Model Registry] LiteLLM did not confirm the model; reconcile will finish it")
		return nil, ErrPendingConfirmation
	default:
		// LiteLLM answered and holds nothing, so nothing references the
		// alias yet: free it.
		if _, removeErr := s.store.Remove(ctx, row.ModelID, dbModelConfig.Guard{Statuses: []dbModelConfig.SyncStatus{dbModelConfig.Pending}}); removeErr != nil {
			logrus.WithFields(logFields).WithError(removeErr).Error("[Model Registry] could not remove a model LiteLLM rejected")
		}
		return nil, err
	}
}

func (s *service) Delete(ctx context.Context, projectID, modelID, username string) error {
	row, err := s.projectModel(ctx, projectID, modelID)
	if err != nil {
		return err
	}
	previous := row.SyncStatus
	if previous != dbModelConfig.Deleting {
		// Deleting first stops new runs from picking the model while the
		// guard below looks for ones already using it.
		ok, err := s.store.Update(ctx, modelID, dbModelConfig.Guard{Statuses: []dbModelConfig.SyncStatus{previous}},
			dbModelConfig.Changes{Status: dbModelConfig.Deleting, By: username})
		if err != nil {
			return err
		}
		if !ok {
			return ErrModelBusy
		}
	}
	users, err := s.experimentsUsing(ctx, projectID, row.Alias)
	if err == nil && len(users) > 0 {
		err = fmt.Errorf("%w by %s; stop them or switch them to another model, then delete it", ErrModelInUse, strings.Join(users, ", "))
	}
	if err != nil {
		if previous != dbModelConfig.Deleting {
			if _, revertErr := s.store.Update(ctx, modelID, dbModelConfig.Guard{Statuses: []dbModelConfig.SyncStatus{dbModelConfig.Deleting}},
				dbModelConfig.Changes{Status: previous}); revertErr != nil {
				logrus.WithField("modelId", modelID).WithError(revertErr).Error("[Model Registry] could not restore a model the delete guard kept")
			}
		}
		return err
	}
	return s.finishDelete(ctx, *row)
}

// settleRacedCreate handles a create whose row stopped being PENDING while
// LiteLLM added the model. Reconcile may already have marked it ACTIVE, which
// is fine; if it was deleted meanwhile, LiteLLM must not keep a model ACE no
// longer records.
func (s *service) settleRacedCreate(ctx context.Context, row dbModelConfig.ModelConfig) error {
	current, err := s.store.Get(ctx, row.ModelID)
	if err != nil {
		return err
	}
	if current != nil && current.SyncStatus == dbModelConfig.Active {
		return nil
	}
	if err := s.gateway.DeleteModel(ctx, row.ModelID); err != nil && !errors.Is(err, ErrNotFound) {
		logrus.WithField("modelId", row.ModelID).WithError(err).Error("[Model Registry] a model deleted while it was being created is still in LiteLLM")
	}
	return ErrModelBusy
}

// finishDelete removes a DELETING model from LiteLLM, then soft-deletes its
// row. The row keeps its alias reserved.
func (s *service) finishDelete(ctx context.Context, row dbModelConfig.ModelConfig) error {
	logFields := logrus.Fields{"projectId": row.ProjectID, "modelId": row.ModelID, "alias": row.Alias}
	if err := s.gateway.DeleteModel(ctx, row.ModelID); err != nil && !errors.Is(err, ErrNotFound) {
		s.recordError(ctx, row.ModelID, err)
		logrus.WithFields(logFields).WithError(err).Warn("[Model Registry] LiteLLM did not delete the model; reconcile will retry")
		if errors.Is(err, ErrUnreachable) {
			return ErrPendingConfirmation
		}
		return err
	}
	cleared := ""
	if _, err := s.store.Update(ctx, row.ModelID, dbModelConfig.Guard{Statuses: []dbModelConfig.SyncStatus{dbModelConfig.Deleting}},
		dbModelConfig.Changes{Removed: true, LastError: &cleared}); err != nil {
		return err
	}
	logrus.WithFields(logFields).Info("[Model Registry] model deleted")
	return nil
}

func (s *service) UpdateKey(ctx context.Context, projectID, modelID, apiKey, username string) (*model.ModelConfig, error) {
	row, err := s.projectModel(ctx, projectID, modelID)
	if err != nil {
		return nil, err
	}
	if row.SyncStatus != dbModelConfig.Active && row.SyncStatus != dbModelConfig.NeedsKey {
		return nil, ErrModelBusy
	}
	key, err := normalizeKey(model.ModelProvider(row.Provider), apiKey)
	if err != nil {
		return nil, err
	}
	deployment := Deployment{ModelID: row.ModelID, Alias: row.Alias, Model: row.Model, APIBase: row.APIBase, APIVersion: row.APIVersion}
	err = s.gateway.UpdateModel(ctx, deployment, keyToSend(key))
	if errors.Is(err, ErrNotFound) {
		// LiteLLM lost the model (NEEDS_KEY): every other parameter is ACE's,
		// so the new key is all it takes to register it again, under the
		// same ID.
		err = s.gateway.AddModel(ctx, deployment, keyToSend(key))
	}
	if err != nil {
		s.recordError(ctx, row.ModelID, err)
		if errors.Is(err, ErrUnreachable) {
			return nil, ErrPendingConfirmation
		}
		return nil, err
	}
	hint, cleared := keyHint(key), ""
	if _, err := s.store.Update(ctx, row.ModelID, dbModelConfig.Guard{Statuses: []dbModelConfig.SyncStatus{dbModelConfig.Active, dbModelConfig.NeedsKey}},
		dbModelConfig.Changes{Status: dbModelConfig.Active, KeyHint: &hint, LastError: &cleared, By: username}); err != nil {
		return nil, err
	}
	logrus.WithFields(logrus.Fields{"projectId": projectID, "modelId": modelID, "alias": row.Alias}).Info("[Model Registry] API key replaced")
	row.SyncStatus, row.APIKeyHint, row.LastError = dbModelConfig.Active, hint, ""
	row.UpdatedAt, row.UpdatedBy = s.now().UnixMilli(), mongodb.UserDetailResponse{Username: username}
	return toModelConfig(*row), nil
}

func (s *service) TestConnection(ctx context.Context, request model.ModelConnectionInput) (*model.ModelTestResult, error) {
	conn, err := normalizeConnection(request)
	if err != nil {
		return nil, err
	}
	result, err := s.gateway.TestConnection(ctx, conn.deployment("", ""), keyToSend(conn.apiKey))
	if err != nil {
		return nil, err
	}
	return toTestResult(result), nil
}

func (s *service) Test(ctx context.Context, projectID, modelID string) (*model.ModelTestResult, error) {
	row, err := s.projectModel(ctx, projectID, modelID)
	if err != nil {
		return nil, err
	}
	if row.SyncStatus != dbModelConfig.Active && row.SyncStatus != dbModelConfig.NeedsKey {
		return nil, ErrModelBusy
	}
	result, err := s.gateway.HealthCheck(ctx, row.ModelID)
	if errors.Is(err, ErrNotFound) {
		return toTestResult(TestResult{Error: "LiteLLM no longer serves this model: re-enter its API key (Rotate key) to register it again"}), nil
	}
	if err != nil {
		return nil, err
	}
	return toTestResult(result), nil
}

func (s *service) AgentModelOptions(ctx context.Context, projectID string) []*model.AgentModelOption {
	options := PlatformModelOptions()
	rows, err := s.store.ListByProject(ctx, projectID, dbModelConfig.Active)
	if err != nil {
		logrus.WithField("projectId", projectID).WithError(err).Warn("[Model Registry] could not read project models; offering the platform models only")
		return options
	}
	seen := map[string]bool{}
	for _, option := range options {
		seen[option.Alias] = true
	}
	defaultAlias := chartconfig.DefaultAgentModelAlias()
	for _, row := range rows {
		if seen[row.Alias] {
			continue
		}
		seen[row.Alias] = true
		provider := providerLabel(row.Provider)
		options = append(options, &model.AgentModelOption{
			Alias:     row.Alias,
			Label:     fmt.Sprintf("%s (%s)", row.Label, provider),
			Provider:  provider,
			IsDefault: row.Alias == defaultAlias,
		})
	}
	return options
}

// ValidateAliases rejects a registry model the project may not use. An alias
// the registry does not know (a platform model, one in LiteLLM's config file
// or on an external gateway) is left alone, as before the registry existed;
// LiteLLM itself reports a name it does not serve.
func (s *service) ValidateAliases(ctx context.Context, projectID string, aliases []string) error {
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}
		row, err := s.store.GetByAlias(ctx, alias)
		if err != nil {
			return fmt.Errorf("could not check model %q: %w", alias, err)
		}
		switch {
		case row == nil:
			continue
		case row.ProjectID != projectID:
			return fmt.Errorf("%w: %q belongs to another project; pick one of this project's models", ErrModelUnavailable, alias)
		case row.IsRemoved:
			return fmt.Errorf("%w: %q was deleted; pick another model", ErrModelUnavailable, alias)
		case row.SyncStatus != dbModelConfig.Active:
			return fmt.Errorf("%w: %q is %s; wait for it to be active, or re-enter its API key", ErrModelUnavailable, alias, statusText(row.SyncStatus))
		}
	}
	return nil
}

// projectModel returns a live model of the project, or ErrModelNotFound for
// one that does not exist or belongs to another project.
func (s *service) projectModel(ctx context.Context, projectID, modelID string) (*dbModelConfig.ModelConfig, error) {
	row, err := s.store.Get(ctx, modelID)
	if err != nil {
		return nil, err
	}
	if row == nil || row.ProjectID != projectID {
		return nil, ErrModelNotFound
	}
	return row, nil
}

func (s *service) recordError(ctx context.Context, modelID string, cause error) {
	message := cause.Error()
	if _, err := s.store.Update(ctx, modelID, dbModelConfig.Guard{}, dbModelConfig.Changes{LastError: &message}); err != nil {
		logrus.WithField("modelId", modelID).WithError(err).Warn("[Model Registry] could not record the error")
	}
}

func aliasTaken(alias string) error {
	return fmt.Errorf("%w: %q", ErrAliasTaken, alias)
}

func statusText(status dbModelConfig.SyncStatus) string {
	switch status {
	case dbModelConfig.Pending:
		return "still being registered"
	case dbModelConfig.Deleting:
		return "being deleted"
	case dbModelConfig.NeedsKey:
		return "missing from the gateway"
	}
	return strings.ToLower(string(status))
}
