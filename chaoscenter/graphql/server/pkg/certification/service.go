package certification

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
)

// PollInterval is the cadence at which we poll certifier task endpoints.
// The spec requires "every 1 minute" while the task is running.
const PollInterval = 60 * time.Second

// MaxPollAttempts caps the number of poll iterations so a stuck upstream
// cannot leak goroutines forever.  At PollInterval=60s this is ~2 hours.
const MaxPollAttempts = 120

// Service is the orchestrator entry point used by the GraphQL resolver.
// It is concurrency-safe: only one orchestration runs per
// (projectId, agentId, experimentId, runId) at a time.
type Service struct {
	op     *Operator
	client *Client

	mu     sync.Mutex
	active map[string]struct{}
}

func NewService(op *Operator, client *Client) *Service {
	return &Service{op: op, client: client, active: make(map[string]struct{})}
}

// StartCertificationGeneration is the synchronous front-door:
// it persists the parent + run docs, dispatches the long-running
// pipeline on a background goroutine and returns immediately.  The
// returned status reflects what was just persisted, not the eventual
// outcome.
type StartInput struct {
	ProjectID       string
	AgentID         string
	AgentName       string
	ExperimentID    string
	ExperimentRunID string
	ExpectedRuns    int
}

type StartOutput struct {
	Status                       string
	ExperimentRunWorkflowStatus  string
	Message                      string
}

func (s *Service) StartCertificationGeneration(ctx context.Context, in StartInput) (*StartOutput, error) {
	if err := validateStartInput(in); err != nil {
		return nil, err
	}

	// Parent summary: created on first sight; expectedRuns ratchets up via $max
	// so adding runs to an already-certified experiment expands the gate target.
	if err := s.op.UpsertExperiment(ctx, &CertificateExperiment{
		ProjectID:         in.ProjectID,
		AgentID:           in.AgentID,
		AgentName:         in.AgentName,
		ExperimentID:      in.ExperimentID,
		ExpectedRuns:      in.ExpectedRuns,
		AggregationPolicy: AggregationPolicy{Mode: PolicyAllRunsCompleted},
	}); err != nil {
		return nil, fmt.Errorf("upsert experiment: %w", err)
	}

	// If the experiment was already fully certified and a new (or replacement)
	// run is arriving, reset the top-level status back to RUNS_IN_PROGRESS so
	// the UI reflects that the pipeline is running again.
	if err := s.op.ResetStatusIfCertified(ctx, in.ProjectID, in.ExperimentID); err != nil {
		return nil, fmt.Errorf("reset experiment status: %w", err)
	}

	// If the run previously failed bucketing, clear its state so the pipeline
	// retries from scratch rather than skipping due to the stale task ID.
	if err := s.op.ResetFailedRunWorkflow(ctx, in.ProjectID, in.AgentID, in.ExperimentID, in.ExperimentRunID); err != nil {
		return nil, fmt.Errorf("reset failed run workflow: %w", err)
	}

	// Run-level workflow: created with status BUCKETING_TRIGGERED on first
	// sight; idempotent for runs that are already in a non-failed state.
	if err := s.op.UpsertRunWorkflowInitial(ctx, &CertificateRunWorkflow{
		ProjectID:       in.ProjectID,
		AgentID:         in.AgentID,
		ExperimentID:    in.ExperimentID,
		ExperimentRunID: in.ExperimentRunID,
	}); err != nil {
		return nil, fmt.Errorf("upsert run workflow: %w", err)
	}

	key := orchestrationKey(in.ProjectID, in.AgentID, in.ExperimentID, in.ExperimentRunID)
	if !s.tryAcquire(key) {
		return &StartOutput{
			Status:                      "ALREADY_RUNNING",
			ExperimentRunWorkflowStatus: RunStatusBucketingRunning,
			Message:                     "orchestration already in progress for this run",
		}, nil
	}
	go func() {
		defer s.release(key)
		// Detached context: HTTP request lifetime must not cancel the
		// long-running orchestration.
		bg, cancel := context.WithTimeout(context.Background(), 4*time.Hour)
		defer cancel()
		s.runPipeline(bg, in)
	}()

	return &StartOutput{
		Status:                      "ACCEPTED",
		ExperimentRunWorkflowStatus: RunStatusBucketingTriggered,
		Message:                     "certification pipeline started",
	}, nil
}

// runPipeline executes bucketing -> poll -> gate -> aggregation -> poll.
func (s *Service) runPipeline(ctx context.Context, in StartInput) {
	logger := log.WithFields(log.Fields{
		"projectId": in.ProjectID, "agentId": in.AgentID,
		"experimentId": in.ExperimentID, "runId": in.ExperimentRunID,
	})

	// Step 1: kick off bucketing.
	if err := s.triggerBucketing(ctx, in); err != nil {
		logger.WithError(err).Error("bucketing trigger failed")
		return
	}

	// Step 2: poll bucketing until terminal.
	if err := s.pollBucketing(ctx, in); err != nil {
		logger.WithError(err).Error("bucketing poll failed")
		// fall-through: gate evaluation also accepts BUCKETING_FAILED runs.
	}

	// Step 3: gate.  Only the run that observes ALL runs in a terminal
	// state will proceed past this point; all earlier runs return.
	ok, runIDs, successful, failed, err := s.evaluateGate(ctx, in)
	if err != nil {
		logger.WithError(err).Error("aggregation gate evaluation failed")
		return
	}
	if !ok {
		logger.Info("aggregation gate not yet satisfied; waiting for more runs")
		return
	}

	_ = s.op.UpdateExperimentStatus(ctx, in.ProjectID, in.ExperimentID, ExperimentStatusAggregationInProgress)

	// Step 4: aggregation.
	version, alreadyRunning, err := s.startAggregation(ctx, in, runIDs, successful, failed)
	if err != nil {
		logger.WithError(err).Error("aggregation start failed")
		return
	}
	if alreadyRunning {
		logger.WithField("aggregationVersion", version).Info("aggregation already in progress; skipping duplicate poll")
		return
	}

	// Step 5: poll aggregation.
	if err := s.pollAggregation(ctx, in, version); err != nil {
		logger.WithError(err).Error("aggregation poll failed")
	}
}

func (s *Service) triggerBucketing(ctx context.Context, in StartInput) error {
	// Skip if a task_id is already saved (idempotency requirement #7.2).
	doc, err := s.op.GetRunWorkflow(ctx, in.ProjectID, in.AgentID, in.ExperimentID, in.ExperimentRunID)
	if err != nil {
		return err
	}
	if doc != nil && doc.Bucketing.TaskID != "" {
		return nil
	}

	now := time.Now().UTC()
	resp, err := s.client.StartBucketing(ctx, BucketingRequest{
		AgentID:       in.AgentID,
		ExperimentID:  in.ExperimentID,
		RunID:         in.ExperimentRunID,
		TraceSource:   TraceSource{Type: "langfuse"},
		StorageConfig: StorageConfig{Type: "local"},
	})
	if err != nil {
		// 409 with a parsed task_id is treated as "already active".
		if apiErr, ok := err.(*APIError); ok && apiErr.StatusCode == 409 && resp != nil && resp.TaskID != "" {
			return s.op.UpdateRunWorkflow(ctx, in.ProjectID, in.AgentID, in.ExperimentID, in.ExperimentRunID, bson.D{
				{Key: "status", Value: RunStatusBucketingRunning},
				{Key: "bucketing.taskId", Value: resp.TaskID},
				{Key: "bucketing.pollUrl", Value: resp.PollURL},
				{Key: "bucketing.startedAt", Value: now},
			})
		}
		_ = s.op.UpdateRunWorkflow(ctx, in.ProjectID, in.AgentID, in.ExperimentID, in.ExperimentRunID, bson.D{
			{Key: "status", Value: RunStatusBucketingFailed},
			{Key: "error", Value: WorkflowError{Code: "BUCKETING_TRIGGER_FAILED", Reason: err.Error(), LastFailedAt: &now}},
		})
		return err
	}

	return s.op.UpdateRunWorkflow(ctx, in.ProjectID, in.AgentID, in.ExperimentID, in.ExperimentRunID, bson.D{
		{Key: "status", Value: RunStatusBucketingRunning},
		{Key: "bucketing.taskId", Value: resp.TaskID},
		{Key: "bucketing.pollUrl", Value: resp.PollURL},
		{Key: "bucketing.startedAt", Value: now},
	})
}

func (s *Service) pollBucketing(ctx context.Context, in StartInput) error {
	for attempt := 1; attempt <= MaxPollAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(PollInterval):
		}

		status, err := s.client.PollBucketing(ctx, in.ExperimentID, in.ExperimentRunID)
		now := time.Now().UTC()
		if err != nil {
			// Transient errors: keep polling, but record the attempt.
			_ = s.op.UpdateRunWorkflow(ctx, in.ProjectID, in.AgentID, in.ExperimentID, in.ExperimentRunID, bson.D{
				{Key: "bucketing.lastPolledAt", Value: now},
				{Key: "bucketing.attempt", Value: attempt},
			})
			log.WithError(err).Warn("bucketing poll transient error")
			continue
		}

		switch strings.ToUpper(status.Status) {
		case "COMPLETED":
			return s.op.UpdateRunWorkflow(ctx, in.ProjectID, in.AgentID, in.ExperimentID, in.ExperimentRunID, bson.D{
				{Key: "status", Value: RunStatusBucketingCompleted},
				{Key: "bucketing.lastPolledAt", Value: now},
				{Key: "bucketing.completedAt", Value: now},
				{Key: "bucketing.attempt", Value: attempt},
				{Key: "result.metricsReady", Value: true},
			})
		case "FAILED":
			return s.op.UpdateRunWorkflow(ctx, in.ProjectID, in.AgentID, in.ExperimentID, in.ExperimentRunID, bson.D{
				{Key: "status", Value: RunStatusBucketingFailed},
				{Key: "bucketing.lastPolledAt", Value: now},
				{Key: "bucketing.completedAt", Value: now},
				{Key: "bucketing.attempt", Value: attempt},
				{Key: "error", Value: WorkflowError{Code: "BUCKETING_FAILED", Reason: stringifyError(status.Error), LastFailedAt: &now}},
			})
		default:
			// RUNNING (or any non-terminal state) — record progress and loop.
			_ = s.op.UpdateRunWorkflow(ctx, in.ProjectID, in.AgentID, in.ExperimentID, in.ExperimentRunID, bson.D{
				{Key: "bucketing.lastPolledAt", Value: now},
				{Key: "bucketing.attempt", Value: attempt},
			})
		}
	}
	return fmt.Errorf("bucketing poll exceeded max attempts (%d)", MaxPollAttempts)
}

// evaluateGate implements the ALL_RUNS_COMPLETED policy.
//
// The gate fires when:
//   - No run is still in a non-terminal bucketing state (nothing still running).
//   - The number of SUCCESSFULLY bucketed runs >= expectedRuns.
//
// Runs that permanently failed bucketing do NOT count toward the target, so
// the caller must either re-trigger those runs (via the retry button or by
// launching a replacement run) until enough succeed.  Only COMPLETED run IDs
// are forwarded to aggregation; failed runs are excluded from the report.
func (s *Service) evaluateGate(ctx context.Context, in StartInput) (bool, []string, int, int, error) {
	exp, err := s.op.GetExperiment(ctx, in.ProjectID, in.ExperimentID)
	if err != nil {
		return false, nil, 0, 0, err
	}
	if exp == nil {
		return false, nil, 0, 0, fmt.Errorf("parent experiment doc missing")
	}
	runs, err := s.op.ListRunWorkflows(ctx, in.ProjectID, in.AgentID, in.ExperimentID)
	if err != nil {
		return false, nil, 0, 0, err
	}
	var ids []string
	successful, failed := 0, 0
	for _, r := range runs {
		switch r.Status {
		case RunStatusBucketingCompleted:
			successful++
			ids = append(ids, r.ExperimentRunID)
		case RunStatusBucketingFailed:
			failed++
		default:
			// A run is still in progress — gate must stay closed.
			return false, nil, 0, 0, nil
		}
	}
	// Not enough successful runs yet — wait for retries or replacement runs.
	if exp.ExpectedRuns > 0 && successful < exp.ExpectedRuns {
		return false, nil, 0, 0, nil
	}
	return true, ids, successful, failed, nil
}

func (s *Service) startAggregation(ctx context.Context, in StartInput, runIDs []string, successful, failed int) (int, bool, error) {
	exp, err := s.op.GetExperiment(ctx, in.ProjectID, in.ExperimentID)
	if err != nil {
		return 0, false, err
	}

	// Determine the next aggregation version from the actual aggregation
	// table, not from exp.ActiveAggregationVersion (which only flips after
	// a certificate is finalized).  If the latest aggregation is still
	// TRIGGERED/RUNNING, treat this gate-crossing as a no-op so concurrent
	// run completions don't collide on the unique index
	// (projectId, agentId, experimentId, aggregationVersion).
	latest, err := s.op.GetLatestAggregationWorkflow(ctx, in.ProjectID, in.AgentID, in.ExperimentID)
	if err != nil {
		return 0, false, err
	}
	if latest != nil && (latest.Status == AggStatusTriggered || latest.Status == AggStatusRunning) {
		return latest.AggregationVersion, true, nil
	}
	version := exp.ActiveAggregationVersion + 1
	if latest != nil && latest.AggregationVersion >= version {
		version = latest.AggregationVersion + 1
	}
	now := time.Now().UTC()

	doc := &CertificateAggregationWorkflow{
		ProjectID:          in.ProjectID,
		AgentID:            in.AgentID,
		AgentName:          in.AgentName,
		ExperimentID:       in.ExperimentID,
		AggregationVersion: version,
		Status:             AggStatusTriggered,
		InputSnapshot: AggregationInput{
			RunIDs:         runIDs,
			SuccessfulRuns: successful,
			FailedRuns:     failed,
		},
		Aggregation: AggregationState{StartedAt: &now},
	}
	if err := s.op.CreateAggregationWorkflow(ctx, doc); err != nil {
		return 0, false, err
	}

	resp, err := s.client.StartAggregation(ctx, AggregationRequest{
		AgentID:            in.AgentID,
		AgentName:          in.AgentName,
		ExperimentID:       in.ExperimentID,
		CertificationRunID: fmt.Sprintf("v%d.0.0", version),
		RunsPerFault:       exp.ExpectedRuns,
		StorageConfig:      StorageConfig{Type: "local"},
	})
	if err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.StatusCode == 409 && resp != nil && resp.CertTaskID != "" {
			return version, false, s.op.UpdateAggregationWorkflow(ctx, in.ProjectID, in.AgentID, in.ExperimentID, version, bson.D{
				{Key: "status", Value: AggStatusRunning},
				{Key: "aggregation.certTaskId", Value: resp.CertTaskID},
				{Key: "aggregation.pollUrl", Value: resp.PollURL},
			})
		}
		_ = s.op.UpdateAggregationWorkflow(ctx, in.ProjectID, in.AgentID, in.ExperimentID, version, bson.D{
			{Key: "status", Value: AggStatusFailed},
			{Key: "error", Value: WorkflowError{Code: "AGGREGATION_TRIGGER_FAILED", Reason: err.Error(), LastFailedAt: &now}},
		})
		return version, false, err
	}

	return version, false, s.op.UpdateAggregationWorkflow(ctx, in.ProjectID, in.AgentID, in.ExperimentID, version, bson.D{
		{Key: "status", Value: AggStatusRunning},
		{Key: "aggregation.certTaskId", Value: resp.CertTaskID},
		{Key: "aggregation.pollUrl", Value: resp.PollURL},
	})
}

func (s *Service) pollAggregation(ctx context.Context, in StartInput, version int) error {
	for attempt := 1; attempt <= MaxPollAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(PollInterval):
		}

		status, err := s.client.PollAggregation(ctx, in.ExperimentID)
		now := time.Now().UTC()
		if err != nil {
			_ = s.op.UpdateAggregationWorkflow(ctx, in.ProjectID, in.AgentID, in.ExperimentID, version, bson.D{
				{Key: "aggregation.lastPolledAt", Value: now},
				{Key: "aggregation.attempt", Value: attempt},
			})
			log.WithError(err).Warn("aggregation poll transient error")
			continue
		}
		switch strings.ToUpper(status.Status) {
		case "COMPLETED":
			certID := extractCertificationID(status.Result)
			_ = s.op.UpdateAggregationWorkflow(ctx, in.ProjectID, in.AgentID, in.ExperimentID, version, bson.D{
				{Key: "status", Value: AggStatusCompleted},
				{Key: "aggregation.lastPolledAt", Value: now},
				{Key: "aggregation.completedAt", Value: now},
				{Key: "aggregation.attempt", Value: attempt},
				{Key: "certificate", Value: AggCertificate{CertificationID: certID, GeneratedAt: &now}},
			})
			return s.op.FinalizeExperimentCertificate(ctx, in.ProjectID, in.ExperimentID, version, LatestCertificate{
				Status:          ExperimentStatusCertificateReady,
				CertificationID: certID,
				PdfStatus:       PdfStatusNotFetched,
				GeneratedAt:     &now,
			})
		case "FAILED":
			reason := ""
			code := "AGGREGATION_FAILED"
			if status.Error != nil {
				reason = status.Error.Message
				if status.Error.ErrorCode != "" {
					code = status.Error.ErrorCode
				}
			}
			return s.op.UpdateAggregationWorkflow(ctx, in.ProjectID, in.AgentID, in.ExperimentID, version, bson.D{
				{Key: "status", Value: AggStatusFailed},
				{Key: "aggregation.lastPolledAt", Value: now},
				{Key: "aggregation.completedAt", Value: now},
				{Key: "aggregation.attempt", Value: attempt},
				{Key: "error", Value: WorkflowError{Code: code, Reason: reason, LastFailedAt: &now}},
			})
		default:
			_ = s.op.UpdateAggregationWorkflow(ctx, in.ProjectID, in.AgentID, in.ExperimentID, version, bson.D{
				{Key: "aggregation.lastPolledAt", Value: now},
				{Key: "aggregation.attempt", Value: attempt},
			})
		}
	}
	return fmt.Errorf("aggregation poll exceeded max attempts (%d)", MaxPollAttempts)
}

// GetExperimentSummary is the read path used by the experiment-history page.
// RunCounts are computed live from the run-workflow collection so the UI always
// sees accurate per-run bucketing progress regardless of how the parent doc's
// counters were last written.
func (s *Service) GetExperimentSummary(ctx context.Context, projectID, experimentID string) (*CertificateExperiment, bool, error) {
	exp, err := s.op.GetExperiment(ctx, projectID, experimentID)
	if err != nil {
		return nil, false, err
	}
	if exp == nil {
		return nil, false, nil
	}

	runs, err := s.op.ListRunWorkflows(ctx, projectID, exp.AgentID, experimentID)
	if err != nil {
		return nil, false, err
	}
	total, completed, failed := 0, 0, 0
	for _, r := range runs {
		total++
		switch r.Status {
		case RunStatusBucketingCompleted:
			completed++
		case RunStatusBucketingFailed:
			failed++
		}
	}
	exp.RunCounts = RunCounts{
		Total:     total,
		Running:   total - completed - failed,
		Completed: completed,
		Failed:    failed,
	}

	return exp, exp.Status == ExperimentStatusCertificateReady, nil
}

// ----- helpers -----

func validateStartInput(in StartInput) error {
	if strings.TrimSpace(in.ProjectID) == "" {
		return fmt.Errorf("projectID is required")
	}
	if strings.TrimSpace(in.AgentID) == "" {
		return fmt.Errorf("agentID is required")
	}
	if strings.TrimSpace(in.ExperimentID) == "" {
		return fmt.Errorf("experimentID is required")
	}
	if strings.TrimSpace(in.ExperimentRunID) == "" {
		return fmt.Errorf("experimentRunID is required")
	}
	return nil
}

func orchestrationKey(parts ...string) string {
	return strings.Join(parts, "|")
}

func (s *Service) tryAcquire(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.active[key]; ok {
		return false
	}
	s.active[key] = struct{}{}
	return true
}

func (s *Service) release(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, key)
}

func extractCertificationID(result map[string]interface{}) string {
	if result == nil {
		return ""
	}
	if v, ok := result["certification_id"].(string); ok {
		return v
	}
	if v, ok := result["certificationId"].(string); ok {
		return v
	}
	return ""
}

func stringifyError(e interface{}) string {
	if e == nil {
		return ""
	}
	switch v := e.(type) {
	case string:
		return v
	case map[string]interface{}:
		if msg, ok := v["message"].(string); ok {
			return msg
		}
	}
	return fmt.Sprintf("%v", e)
}
