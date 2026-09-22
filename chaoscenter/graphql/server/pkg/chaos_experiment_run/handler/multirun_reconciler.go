package handler

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
	dbChaosExperiment "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment"
	dbChaosExperimentRun "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment_run"
	store "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/data-store"
)

const (
	// multiRunReconcileInterval is how often stalled batches are swept.
	multiRunReconcileInterval = 2 * time.Minute

	// queuedRunStallTimeout is how long a run may sit in Queued before it is
	// declared undeliverable. A Queued run is one the server created but the
	// subscriber never turned into an Argo workflow (dropped dispatch, infra
	// disconnected, subscriber restart), so it will never emit a completion
	// event and would hold its batch's in-flight gate shut forever.
	queuedRunStallTimeout = 10 * time.Minute

	// multiRunDispatchGrace is how long after the last run activity the
	// reconciler leaves a batch alone. It has to exceed the chain's own
	// inter-run delay so the reconciler never races the normal dispatch path.
	multiRunDispatchGrace = 10 * time.Minute
)

// nonTerminalRunPhases are the phases a run can still move out of on its own.
var nonTerminalRunPhases = map[string]bool{
	"Running": true,
	"Queued":  true,
}

// StartMultiRunReconciler sweeps sequential multi-run batches that stopped
// making progress and restarts them.
//
// advanceMultiRunChain is event-driven: every run of a batch is dispatched by
// the completion event of the previous one, after an in-process timer. That
// makes the batch only as durable as the event and the process — a completion
// event that never arrives (subscriber dropped the dispatch, run wedged in
// Queued) or a server restart during the inter-run delay leaves the batch
// permanently short of maxRuns with nothing left to wake it. The symptom is a
// batch that reports 1/2 forever: run 1 completes, run 2 never appears.
//
// This loop is the safety net for exactly that. It only ever acts on a batch
// that is quiescent — no run of the experiment Running or recently Queued —
// so it cannot race the event-driven path, and it recomputes the batch
// counters from the runs that actually exist rather than trusting the
// in-memory bookkeeping it is there to repair.
func (c *ChaosExperimentRunHandler) StartMultiRunReconciler(ctx context.Context) {
	ticker := time.NewTicker(multiRunReconcileInterval)
	defer ticker.Stop()

	logrus.Infof("[Multi-Run] reconciler started (interval=%v, queued stall timeout=%v)", multiRunReconcileInterval, queuedRunStallTimeout)

	for {
		select {
		case <-ctx.Done():
			logrus.Info("[Multi-Run] reconciler stopped")
			return
		case <-ticker.C:
			c.reconcileMultiRunBatches(ctx)
		}
	}
}

func (c *ChaosExperimentRunHandler) reconcileMultiRunBatches(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("[Multi-Run] PANIC in reconciler sweep: %v", r)
		}
	}()

	experiments, err := c.chaosExperimentOperator.GetExperiments(bson.D{
		{"is_removed", false},
		{"planned_runs", bson.D{{"$gt", 1}}},
		{"multi_run_state.batch_done", bson.D{{"$ne", true}}},
	})
	if err != nil {
		logrus.WithError(err).Error("[Multi-Run] reconciler could not list multi-run experiments")
		return
	}

	for _, experiment := range experiments {
		c.reconcileMultiRunBatch(ctx, experiment)
	}
}

func (c *ChaosExperimentRunHandler) reconcileMultiRunBatch(ctx context.Context, experiment dbChaosExperiment.ChaosExperimentRequest) {
	expID := experiment.ExperimentID
	logFields := logrus.Fields{"projectID": experiment.ProjectID, "experimentID": expID}

	if len(experiment.Revision) == 0 {
		return
	}
	manifest := experiment.Revision[len(experiment.Revision)-1].ExperimentManifest
	if gjson.Get(manifest, `metadata.annotations.litmuschaos\.io/multiRunEnabled`).String() != "true" {
		return
	}
	maxRuns, err := strconv.Atoi(gjson.Get(manifest, `metadata.annotations.litmuschaos\.io/maxRuns`).String())
	if err != nil || maxRuns <= 1 {
		return
	}

	runs, err := c.chaosExperimentRunOperator.GetExperimentRuns(bson.D{
		{"experiment_id", expID},
		{"is_removed", false},
	})
	if err != nil {
		logrus.WithFields(logFields).WithError(err).Error("[Multi-Run] reconciler could not list runs")
		return
	}
	if len(runs) == 0 {
		return
	}

	now := time.Now().UnixMilli()
	var (
		terminalIDs []string
		inFlight    int
		lastActive  int64
	)

	for _, run := range runs {
		if run.Audit.UpdatedAt > lastActive {
			lastActive = run.Audit.UpdatedAt
		}

		if run.Completed || !nonTerminalRunPhases[run.Phase] {
			terminalIDs = append(terminalIDs, multiRunRunKey(run))
			continue
		}

		// A Running run is left alone: a long fault can legitimately go a long
		// while without an Argo node transition, and there is no safe staleness
		// threshold that does not risk killing a healthy run.
		if run.Phase != "Queued" {
			inFlight++
			continue
		}

		if now-run.Audit.UpdatedAt < queuedRunStallTimeout.Milliseconds() {
			inFlight++
			continue
		}

		if err := c.expireStalledQueuedRun(ctx, experiment, run); err != nil {
			logrus.WithFields(logFields).WithError(err).Error("[Multi-Run] reconciler could not expire a stalled Queued run")
			inFlight++
			continue
		}
		terminalIDs = append(terminalIDs, multiRunRunKey(run))
	}

	if inFlight > 0 {
		return
	}

	// Counters are recomputed from the runs that exist: the chain's $inc
	// bookkeeping is exactly what a crashed dispatch leaves wrong, so trusting
	// it here would keep the batch stuck at the dispatch ceiling.
	batchDone := len(runs) >= maxRuns || len(terminalIDs) >= maxRuns
	setState := bson.D{
		{"multi_run_state.completed_run_ids", terminalIDs},
		{"multi_run_state.launched", max(len(runs)-1, 0)},
	}
	if batchDone {
		setState = append(setState, bson.E{Key: "multi_run_state.batch_done", Value: true})
	}
	if err := c.chaosExperimentOperator.UpdateChaosExperiment(ctx,
		bson.D{{"experiment_id", expID}},
		bson.D{{"$set", setState}},
	); err != nil {
		logrus.WithFields(logFields).WithError(err).Error("[Multi-Run] reconciler could not repair batch state")
		return
	}
	if batchDone {
		logrus.WithFields(logFields).Infof("[Multi-Run] reconciler latched batch complete (%d/%d runs)", len(runs), maxRuns)
		return
	}

	// Leave a freshly finished batch to the event-driven path, which dispatches
	// the next run after its own delay.
	if now-lastActive < multiRunDispatchGrace.Milliseconds() {
		return
	}

	logrus.WithFields(logFields).Warnf(
		"[Multi-Run] batch stalled at %d/%d runs with nothing in flight for %v; dispatching the next run",
		len(runs), maxRuns, time.Duration(now-lastActive)*time.Millisecond)

	current, err := c.chaosExperimentOperator.GetExperiment(ctx, bson.D{{"experiment_id", expID}})
	if err != nil {
		logrus.WithFields(logFields).WithError(err).Error("[Multi-Run] reconciler could not re-read the experiment before dispatch")
		return
	}
	if _, err := c.RunChaosWorkFlow(ctx, experiment.ProjectID, current, store.Store, ""); err != nil {
		logrus.WithFields(logFields).WithError(err).Error("[Multi-Run] reconciler could not dispatch the next run; retrying on the next sweep")
		return
	}

	// After this dispatch the batch has len(runs)+1 runs, of which all but the
	// first were chain-dispatched — keep `launched` in step so the event-driven
	// ceiling still bounds the rest of the batch.
	if err := c.chaosExperimentOperator.UpdateChaosExperiment(ctx,
		bson.D{{"experiment_id", expID}},
		bson.D{{"$set", bson.D{{"multi_run_state.launched", len(runs)}}}},
	); err != nil {
		logrus.WithFields(logFields).WithError(err).Warn("[Multi-Run] reconciler dispatched a run but could not update the launched counter")
	}
}

// expireStalledQueuedRun marks a run the subscriber never picked up as failed,
// in both the run collection and the denormalized copy the UI reads, so the
// batch stops waiting on an event that will never arrive.
func (c *ChaosExperimentRunHandler) expireStalledQueuedRun(ctx context.Context, experiment dbChaosExperiment.ChaosExperimentRequest, run dbChaosExperimentRun.ChaosExperimentRun) error {
	now := time.Now().UnixMilli()
	updatedBy := mongodb.UserDetailResponse{Username: "system"}

	runQuery := bson.D{{"experiment_id", run.ExperimentID}}
	elemMatch := bson.D{}
	if strings.TrimSpace(run.ExperimentRunID) != "" {
		runQuery = append(runQuery, bson.E{Key: "experiment_run_id", Value: run.ExperimentRunID})
		elemMatch = append(elemMatch, bson.E{Key: "experiment_run_id", Value: run.ExperimentRunID})
	} else if run.NotifyID != nil {
		runQuery = append(runQuery, bson.E{Key: "notify_id", Value: *run.NotifyID})
		elemMatch = append(elemMatch, bson.E{Key: "notify_id", Value: *run.NotifyID})
	} else {
		return nil
	}

	if err := c.chaosExperimentRunOperator.UpdateExperimentRunWithQuery(ctx, runQuery, bson.D{
		{"$set", bson.D{
			{"phase", "Error"},
			{"completed", true},
			{"updated_at", now},
			{"updated_by", updatedBy},
		}},
	}); err != nil {
		return err
	}

	if err := c.chaosExperimentOperator.UpdateChaosExperiment(ctx,
		bson.D{
			{"experiment_id", experiment.ExperimentID},
			{"recent_experiment_run_details", bson.D{{"$elemMatch", elemMatch}}},
		},
		bson.D{{"$set", bson.D{
			{"recent_experiment_run_details.$.phase", "Error"},
			{"recent_experiment_run_details.$.completed", true},
			{"recent_experiment_run_details.$.updated_at", now},
			{"recent_experiment_run_details.$.updated_by", updatedBy},
		}}},
	); err != nil {
		logrus.WithError(err).Warn("[Multi-Run] expired a stalled Queued run but could not update recent_experiment_run_details; the UI may show a stale Queued state")
	}

	logrus.Infof("[Multi-Run] expired run %s of experiment %s: it stayed Queued for more than %v, so the subscriber never picked it up",
		multiRunRunKey(run), experiment.ExperimentID, queuedRunStallTimeout)
	return nil
}

// multiRunRunKey identifies a run for batch accounting. A run the subscriber
// never picked up has no Argo UID yet, so it is only addressable by notify_id.
func multiRunRunKey(run dbChaosExperimentRun.ChaosExperimentRun) string {
	if id := strings.TrimSpace(run.ExperimentRunID); id != "" {
		return id
	}
	if run.NotifyID != nil {
		return *run.NotifyID
	}
	return ""
}
