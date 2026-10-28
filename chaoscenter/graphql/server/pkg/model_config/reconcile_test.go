package model_config

import (
	"context"
	"errors"
	"testing"
	"time"

	dbModelConfig "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/model_config"
)

// aged is a stored model created and last changed `age` before testNow.
func aged(id string, status dbModelConfig.SyncStatus, age time.Duration) dbModelConfig.ModelConfig {
	row := storedModel(id, projectA, "alias-"+id, status)
	row.CreatedAt = testNow.Add(-age).UnixMilli()
	row.UpdatedAt = row.CreatedAt
	return row
}

func TestReconcileTransitions(t *testing.T) {
	cases := []struct {
		name      string
		row       dbModelConfig.ModelConfig
		served    bool
		connected bool
		want      dbModelConfig.SyncStatus // "" = row removed
	}{
		{"a pending model LiteLLM has becomes active", aged("p1", dbModelConfig.Pending, time.Hour), true, true, dbModelConfig.Active},
		{"a pending model that never reached LiteLLM is removed", aged("p2", dbModelConfig.Pending, time.Hour), false, true, ""},
		{"a pending model may still be on its way", aged("p3", dbModelConfig.Pending, 10*time.Second), false, true, dbModelConfig.Pending},
		{"an active model LiteLLM lost needs its key again", aged("a1", dbModelConfig.Active, time.Hour), false, true, dbModelConfig.NeedsKey},
		{"an active model just created may not be listed yet", aged("a2", dbModelConfig.Active, 30*time.Second), false, true, dbModelConfig.Active},
		{"an active model LiteLLM has stays active", aged("a3", dbModelConfig.Active, time.Hour), true, true, dbModelConfig.Active},
		{"a model back in LiteLLM is active again", aged("n1", dbModelConfig.NeedsKey, time.Hour), true, true, dbModelConfig.Active},
		// Without its database LiteLLM lists no database models: nothing
		// can be concluded from their absence.
		{"no database: an active model is not judged lost", aged("a4", dbModelConfig.Active, time.Hour), false, false, dbModelConfig.Active},
		{"no database: a pending model is not removed", aged("p4", dbModelConfig.Pending, time.Hour), false, false, dbModelConfig.Pending},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, gateway := newFakeStore(tc.row), newFakeGateway()
			gateway.connected = tc.connected
			if tc.served {
				gateway.served[tc.row.ModelID] = RemoteModel{Alias: tc.row.Alias, ModelID: tc.row.ModelID, DBModel: true}
			}
			svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)

			if err := svc.Reconcile(context.Background()); err != nil {
				t.Fatalf("Reconcile() error = %v", err)
			}
			got := store.row(tc.row.ModelID)
			switch {
			case tc.want == "" && got != nil:
				t.Fatalf("row = %+v, want it removed", got)
			case tc.want != "" && (got == nil || got.SyncStatus != tc.want):
				t.Fatalf("row = %+v, want %s", got, tc.want)
			}
		})
	}
}

func TestReconcileNeverOverwritesAConcurrentChange(t *testing.T) {
	row := aged("a1", dbModelConfig.Active, time.Hour)
	store, gateway := newFakeStore(row), newFakeGateway()
	svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)
	// A key rotation re-registers the model after reconcile read the rows
	// but before it decides: the row is no longer the one it read.
	svc.gateway = &interleavingGateway{fakeGateway: gateway, before: func() {
		_, _ = store.Update(context.Background(), "a1", dbModelConfig.Guard{}, dbModelConfig.Changes{KeyHint: ptr("****0000")})
	}}

	if err := svc.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if got := store.row("a1"); got.SyncStatus != dbModelConfig.Active {
		t.Fatalf("status = %s; reconcile must not act on a row changed after it read it", got.SyncStatus)
	}
}

// interleavingGateway runs before ahead of ListModels.
type interleavingGateway struct {
	*fakeGateway
	before func()
}

func (g *interleavingGateway) ListModels(ctx context.Context) ([]RemoteModel, error) {
	g.before()
	return g.fakeGateway.ListModels(ctx)
}

func TestReconcileWithoutModelsDoesNotCallLiteLLM(t *testing.T) {
	gateway := newFakeGateway()
	svc := newTestService(newFakeStore(), gateway, fakeRuns{}, fakeExperiments{}, testNow)
	if err := svc.Reconcile(context.Background()); err != nil || len(gateway.calls) != 0 {
		t.Fatalf("Reconcile() = %v, calls %v; a deployment that never used the registry must not poll LiteLLM", err, gateway.calls)
	}
}

func TestReconcileLeavesModelsACEDidNotCreate(t *testing.T) {
	store, gateway := newFakeStore(aged("a1", dbModelConfig.Active, time.Hour)), newFakeGateway()
	gateway.served["a1"] = RemoteModel{Alias: "alias-a1", ModelID: "a1", DBModel: true}
	gateway.served["foreign"] = RemoteModel{Alias: "made-in-litellm-ui", ModelID: "foreign", DBModel: true}
	svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)

	for i := 0; i < 2; i++ {
		if err := svc.Reconcile(context.Background()); err != nil {
			t.Fatalf("Reconcile() error = %v", err)
		}
	}
	if !gateway.has("foreign") || !svc.warnedOrphans["foreign"] {
		t.Fatal("a model ACE has no record of must be warned about, never deleted")
	}
}

func TestReconcileReportsAnUnreachableGateway(t *testing.T) {
	store, gateway := newFakeStore(aged("a1", dbModelConfig.Active, time.Hour)), newFakeGateway()
	gateway.listErr = ErrUnreachable
	svc := newTestService(store, gateway, fakeRuns{}, fakeExperiments{}, testNow)

	if err := svc.Reconcile(context.Background()); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("Reconcile() error = %v, want ErrUnreachable", err)
	}
	if store.row("a1").SyncStatus != dbModelConfig.Active {
		t.Fatal("an unreachable gateway must change nothing")
	}
}

func TestRunReconcilerStopsWithItsContext(t *testing.T) {
	gateway := newFakeGateway()
	svc := newTestService(newFakeStore(), gateway, fakeRuns{}, fakeExperiments{}, testNow)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.RunReconciler(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RunReconciler did not return after its context ended")
	}
}
