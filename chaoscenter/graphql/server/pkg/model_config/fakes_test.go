package model_config

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	dbChaosExperiment "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment"
	dbChaosExperimentRun "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment_run"
	dbModelConfig "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/model_config"
	"go.mongodb.org/mongo-driver/bson"
)

// fakeStore keeps rows in memory with the operator's guard semantics. Every
// update moves updated_at forward, like the real one.
type fakeStore struct {
	mu   sync.Mutex
	rows map[string]*dbModelConfig.ModelConfig
	tick int64
	// failLists makes the List* calls fail.
	failLists error
}

func newFakeStore(rows ...dbModelConfig.ModelConfig) *fakeStore {
	store := &fakeStore{rows: map[string]*dbModelConfig.ModelConfig{}, tick: 1_000_000}
	for _, row := range rows {
		row := row
		store.rows[row.ModelID] = &row
	}
	return store
}

func (f *fakeStore) Insert(_ context.Context, row dbModelConfig.ModelConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, existing := range f.rows {
		if existing.Alias == row.Alias {
			return dbModelConfig.ErrDuplicateAlias
		}
	}
	f.rows[row.ModelID] = &row
	return nil
}

func (f *fakeStore) Get(_ context.Context, modelID string) (*dbModelConfig.ModelConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if row, ok := f.rows[modelID]; ok && !row.IsRemoved {
		copied := *row
		return &copied, nil
	}
	return nil, nil
}

func (f *fakeStore) GetByAlias(_ context.Context, alias string) (*dbModelConfig.ModelConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.rows {
		if row.Alias == alias {
			copied := *row
			return &copied, nil
		}
	}
	return nil, nil
}

func (f *fakeStore) ListByProject(_ context.Context, projectID string, statuses ...dbModelConfig.SyncStatus) ([]dbModelConfig.ModelConfig, error) {
	return f.list(func(row *dbModelConfig.ModelConfig) bool {
		return row.ProjectID == projectID && (len(statuses) == 0 || hasStatus(statuses, row.SyncStatus))
	})
}

func (f *fakeStore) ListLive(context.Context) ([]dbModelConfig.ModelConfig, error) {
	return f.list(func(*dbModelConfig.ModelConfig) bool { return true })
}

func (f *fakeStore) list(keep func(*dbModelConfig.ModelConfig) bool) ([]dbModelConfig.ModelConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failLists != nil {
		return nil, f.failLists
	}
	rows := []dbModelConfig.ModelConfig{}
	for _, row := range f.rows {
		if !row.IsRemoved && keep(row) {
			rows = append(rows, *row)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt < rows[j].CreatedAt })
	return rows, nil
}

func (f *fakeStore) matches(modelID string, guard dbModelConfig.Guard) (*dbModelConfig.ModelConfig, bool) {
	row, ok := f.rows[modelID]
	if !ok || row.IsRemoved {
		return nil, false
	}
	if len(guard.Statuses) > 0 && !hasStatus(guard.Statuses, row.SyncStatus) {
		return nil, false
	}
	if guard.UpdatedAt != 0 && row.UpdatedAt != guard.UpdatedAt {
		return nil, false
	}
	return row, true
}

func (f *fakeStore) Update(_ context.Context, modelID string, guard dbModelConfig.Guard, changes dbModelConfig.Changes) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.matches(modelID, guard)
	if !ok {
		return false, nil
	}
	f.tick++
	row.UpdatedAt = f.tick
	if changes.Status != "" {
		row.SyncStatus = changes.Status
	}
	if changes.LastError != nil {
		row.LastError = *changes.LastError
	}
	if changes.KeyHint != nil {
		row.APIKeyHint = *changes.KeyHint
	}
	if changes.Removed {
		row.IsRemoved = true
	}
	if changes.By != "" {
		row.UpdatedBy.Username = changes.By
	}
	return true, nil
}

func (f *fakeStore) Remove(_ context.Context, modelID string, guard dbModelConfig.Guard) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.matches(modelID, guard); !ok {
		return false, nil
	}
	delete(f.rows, modelID)
	return true, nil
}

func (f *fakeStore) row(modelID string) *dbModelConfig.ModelConfig {
	f.mu.Lock()
	defer f.mu.Unlock()
	if row, ok := f.rows[modelID]; ok {
		copied := *row
		return &copied
	}
	return nil
}

func (f *fakeStore) only() dbModelConfig.ModelConfig {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.rows) != 1 {
		panic(fmt.Sprintf("want exactly one row, have %d", len(f.rows)))
	}
	for _, row := range f.rows {
		return *row
	}
	panic("unreachable")
}

func hasStatus(statuses []dbModelConfig.SyncStatus, status dbModelConfig.SyncStatus) bool {
	for _, candidate := range statuses {
		if candidate == status {
			return true
		}
	}
	return false
}

// fakeGateway is LiteLLM: a set of served models, plus forced errors.
type fakeGateway struct {
	mu         sync.Mutex
	served     map[string]RemoteModel // by model ID
	keys       map[string]string      // last key sent per model ID
	tested     []Deployment
	testedKeys []string
	connected  bool
	calls      []string

	addErr, updateErr, deleteErr, listErr error
	testResult                            TestResult
	healthErr                             error
	// onAdd runs inside AddModel, to interleave a concurrent change.
	onAdd func()
}

func newFakeGateway(static ...string) *fakeGateway {
	gateway := &fakeGateway{served: map[string]RemoteModel{}, keys: map[string]string{}, connected: true}
	for i, alias := range static {
		id := fmt.Sprintf("static-%d", i)
		gateway.served[id] = RemoteModel{Alias: alias, ModelID: id}
	}
	return gateway
}

func (f *fakeGateway) record(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
}

func (f *fakeGateway) AddModel(_ context.Context, d Deployment, apiKey string) error {
	f.record("add " + d.ModelID)
	if f.onAdd != nil {
		f.onAdd()
	}
	if f.addErr != nil {
		return f.addErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.served[d.ModelID] = RemoteModel{Alias: d.Alias, ModelID: d.ModelID, DBModel: true}
	f.keys[d.ModelID] = apiKey
	return nil
}

func (f *fakeGateway) UpdateModel(_ context.Context, d Deployment, apiKey string) error {
	f.record("update " + d.ModelID)
	if f.updateErr != nil {
		return f.updateErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.served[d.ModelID]; !ok {
		return fmt.Errorf("%w: Model %s not found on proxy.", ErrNotFound, d.ModelID)
	}
	f.keys[d.ModelID] = apiKey
	return nil
}

func (f *fakeGateway) DeleteModel(_ context.Context, modelID string) error {
	f.record("delete " + modelID)
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.served[modelID]; !ok {
		return fmt.Errorf("%w: Model with id=%s not found in db", ErrNotFound, modelID)
	}
	delete(f.served, modelID)
	return nil
}

func (f *fakeGateway) ListModels(context.Context) ([]RemoteModel, error) {
	f.record("list")
	if f.listErr != nil {
		return nil, f.listErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	models := []RemoteModel{}
	for _, served := range f.served {
		if served.DBModel && !f.connected {
			continue // without its database LiteLLM serves no database models
		}
		models = append(models, served)
	}
	return models, nil
}

func (f *fakeGateway) RegistryConnected(context.Context) (bool, error) {
	f.record("readiness")
	return f.connected, nil
}

func (f *fakeGateway) TestConnection(_ context.Context, d Deployment, apiKey string) (TestResult, error) {
	f.record("test")
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tested = append(f.tested, d)
	f.testedKeys = append(f.testedKeys, apiKey)
	return f.testResult, nil
}

func (f *fakeGateway) HealthCheck(_ context.Context, modelID string) (TestResult, error) {
	f.record("health " + modelID)
	if f.healthErr != nil {
		return TestResult{}, f.healthErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.served[modelID]; !ok {
		return TestResult{}, ErrNotFound
	}
	return f.testResult, nil
}

func (f *fakeGateway) has(modelID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.served[modelID]
	return ok
}

type fakeRuns struct {
	runs []dbChaosExperimentRun.ChaosExperimentRun
	err  error
}

func (f fakeRuns) GetExperimentRuns(bson.D) ([]dbChaosExperimentRun.ChaosExperimentRun, error) {
	return f.runs, f.err
}

// fakeExperiments returns the experiments the guard's query selects: those
// with an in-flight run, and cron experiments.
type fakeExperiments struct {
	experiments []dbChaosExperiment.ChaosExperimentRequest
}

func (f fakeExperiments) GetExperiments(query bson.D) ([]dbChaosExperiment.ChaosExperimentRequest, error) {
	wanted := map[string]bool{}
	for _, element := range query {
		if element.Key != "$or" {
			continue
		}
		for _, id := range element.Value.(bson.A)[0].(bson.D)[0].Value.(bson.D)[0].Value.(bson.A) {
			wanted[id.(string)] = true
		}
	}
	var selected []dbChaosExperiment.ChaosExperimentRequest
	for _, experiment := range f.experiments {
		if wanted[experiment.ExperimentID] || experiment.CronSyntax != "" {
			selected = append(selected, experiment)
		}
	}
	return selected, nil
}

func newTestService(store Store, gateway LiteLLMClient, runs RunLister, experiments ExperimentLister, now time.Time) *service {
	s := NewService(store, gateway, runs, experiments).(*service)
	s.now = func() time.Time { return now }
	return s
}
