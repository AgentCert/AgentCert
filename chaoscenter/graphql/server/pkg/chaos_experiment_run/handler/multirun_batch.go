package handler

import (
	"context"
	"strconv"
	"time"

	"github.com/tidwall/gjson"
	"go.mongodb.org/mongo-driver/bson"

	dbChaosExperiment "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment"
)

// defaultMultiRunDelay is the inter-run delay when the manifest sets none.
const defaultMultiRunDelay = 120 * time.Second

// multiRunContinuationKey marks a context as a dispatch made by the batch
// itself (the chain or the reconciler), as opposed to a user starting a run.
type multiRunContinuationKey struct{}

func withMultiRunContinuation(ctx context.Context) context.Context {
	return context.WithValue(ctx, multiRunContinuationKey{}, true)
}

func isMultiRunContinuation(ctx context.Context) bool {
	v, _ := ctx.Value(multiRunContinuationKey{}).(bool)
	return v
}

// multiRunDelay is the configured pause between runs of a batch.
func multiRunDelay(manifest string) time.Duration {
	if delayStr := gjson.Get(manifest, `metadata.annotations.litmuschaos\.io/multiRunDelay`).String(); delayStr != "" {
		if parsed, err := strconv.Atoi(delayStr); err == nil && parsed > 0 {
			return time.Duration(parsed) * time.Second
		}
	}
	return defaultMultiRunDelay
}

// multiRunGrace is how long the reconciler leaves a quiescent batch to the
// chain before treating it as stalled. It must exceed the chain's own
// inter-run delay, which the UI lets users set well beyond ten minutes;
// otherwise the reconciler dispatches mid-delay and the chain's sleeping
// timer dispatches the same run again.
func multiRunGrace(manifest string) time.Duration {
	grace := multiRunDelay(manifest) + 2*time.Minute
	if grace < multiRunDispatchGrace {
		grace = multiRunDispatchGrace
	}
	return grace
}

// beginMultiRunBatch resets the batch state when a user starts a multi-run
// experiment. Without it the state of the previous batch persists: its
// BatchDone latch makes the chain ignore the new run's completion, so every
// batch after the first stopped at one run.
func (c *ChaosExperimentRunHandler) beginMultiRunBatch(ctx context.Context, experimentID string, startedAt int64) error {
	return c.chaosExperimentOperator.UpdateChaosExperiment(ctx,
		bson.D{{"experiment_id", experimentID}},
		bson.D{{"$set", bson.D{{"multi_run_state", dbChaosExperiment.MultiRunState{
			Launched:        0,
			CompletedRunIDs: []string{},
			BatchDone:       false,
			StartedAt:       startedAt,
		}}}}},
	)
}

// runPredatesBatch reports whether a run was created before the experiment's
// current batch started, i.e. belongs to an earlier batch.
func runPredatesBatch(state *dbChaosExperiment.MultiRunState, runCreatedAt int64) bool {
	return state != nil && state.StartedAt > 0 && runCreatedAt > 0 && runCreatedAt < state.StartedAt
}
