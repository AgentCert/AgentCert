package handler

import (
	"context"
	"testing"
	"time"

	dbChaosExperiment "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment"
)

func TestMultiRunGraceOutlastsTheConfiguredDelay(t *testing.T) {
	manifest := `{"metadata":{"annotations":{"litmuschaos.io/multiRunDelay":"1800"}}}`
	if got := multiRunGrace(manifest); got <= 30*time.Minute {
		t.Fatalf("grace %v does not exceed a 30m inter-run delay; the reconciler would dispatch mid-delay", got)
	}
	if got := multiRunGrace(`{}`); got != multiRunDispatchGrace {
		t.Fatalf("grace with the default delay = %v, want the %v floor", got, multiRunDispatchGrace)
	}
	if got := multiRunDelay(`{}`); got != defaultMultiRunDelay {
		t.Fatalf("default delay = %v", got)
	}
}

func TestRunPredatesBatch(t *testing.T) {
	state := &dbChaosExperiment.MultiRunState{StartedAt: 1000}
	if !runPredatesBatch(state, 999) {
		t.Fatal("a run created before the batch started was counted toward it")
	}
	if runPredatesBatch(state, 1000) || runPredatesBatch(state, 2000) {
		t.Fatal("a run of the current batch was rejected")
	}
	// Batches started before StartedAt existed keep the previous behaviour.
	if runPredatesBatch(&dbChaosExperiment.MultiRunState{}, 1) || runPredatesBatch(nil, 1) {
		t.Fatal("a batch without a start marker rejected a run")
	}
}

func TestMultiRunContinuationMarker(t *testing.T) {
	if isMultiRunContinuation(context.Background()) {
		t.Fatal("a user-initiated context was treated as a batch continuation")
	}
	if !isMultiRunContinuation(withMultiRunContinuation(context.Background())) {
		t.Fatal("a batch dispatch was treated as a user-initiated run and would reset the batch")
	}
}
