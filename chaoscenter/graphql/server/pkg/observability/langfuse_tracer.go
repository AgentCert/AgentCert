package observability

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/agent_registry"
)

// LangfuseTracer manages Langfuse integration for the Litmus chaos center backend.
// It's responsible for tracking fault executions, experiment runs, and collecting
// observability data for all chaos activities.
type LangfuseTracer struct {
	client      agent_registry.LangfuseClient
	enabled     bool
	orgID       string // Langfuse Organization ID
	projectID   string // Langfuse Project ID
	mu          sync.RWMutex
	traceChan   chan *agent_registry.ExperimentTrace
	workerDone  chan struct{}
	closed      bool
}

var (
	globalTracer *LangfuseTracer
	tracerMutex  sync.Mutex
)

// InitializeLangfuseTracer initializes the global Langfuse tracer from environment variables.
// Call this during server startup in the main.go after environment setup.
func InitializeLangfuseTracer() error {
	tracerMutex.Lock()
	defer tracerMutex.Unlock()

	// Read Langfuse credentials from environment
	host := os.Getenv("LANGFUSE_HOST")
	publicKey := os.Getenv("LANGFUSE_PUBLIC_KEY")
	secretKey := os.Getenv("LANGFUSE_SECRET_KEY")
	orgID := os.Getenv("LANGFUSE_ORG_ID")
	projectID := os.Getenv("LANGFUSE_PROJECT_ID")

	// If any required credential is missing, disable Langfuse but don't error
	if host == "" || publicKey == "" || secretKey == "" {
		fmt.Println("[Observability] Langfuse integration disabled - credentials not found in environment")
		globalTracer = &LangfuseTracer{
			enabled: false,
		}
		return nil
	}

	// Create Langfuse client with both public and secret keys for Basic Auth
	client := agent_registry.NewLangfuseClient(host, publicKey, secretKey)

	// Initialize tracer with buffered channel for async trace submission
	tracer := &LangfuseTracer{
		client:    client,
		enabled:   true,
		orgID:     orgID,
		projectID: projectID,
		traceChan: make(chan *agent_registry.ExperimentTrace, 100),
		workerDone: make(chan struct{}),
	}

	// Start background worker to process traces
	go tracer.traceWorker()

	globalTracer = tracer
	fmt.Printf("[Observability] Langfuse tracer initialized successfully (Project: %s, Org: %s)\n", projectID, orgID)

	return nil
}

// GetLangfuseTracer returns the global Langfuse tracer instance.
func GetLangfuseTracer() *LangfuseTracer {
	tracerMutex.Lock()
	defer tracerMutex.Unlock()

	if globalTracer == nil {
		// Initialize with disabled tracer if not yet initialized
		globalTracer = &LangfuseTracer{
			enabled: false,
		}
	}

	return globalTracer
}

// TraceExperimentExecution logs the start of an experiment/fault execution.
// This should be called when a chaos experiment or fault is about to be executed.
func (t *LangfuseTracer) TraceExperimentExecution(ctx context.Context, details *ExperimentExecutionDetails) error {
	if !t.IsEnabled() {
		return nil // Silently skip if tracing is disabled
	}
	if details == nil || details.TraceID == "" || details.ExperimentName == "" {
		return fmt.Errorf("invalid trace execution details")
	}

	// Create trace from execution details
	now := time.Now()
	trace := &agent_registry.ExperimentTrace{
		TraceID:           details.TraceID,
		// Keep root trace name aligned with OTEL StartExperimentSpan("experiment-run").
		// Experiment/fault identity stays in metadata fields.
		Name:              "experiment-run",
		ExperimentID:      details.ExperimentID,
		ExperimentName:    details.ExperimentName,
		FaultName:         details.FaultName,
		SessionID:         details.SessionID,
		AgentID:           details.AgentID,
		UserID:            details.AgentID,
		ProjectID:         details.ProjectID,
		LangfuseOrgID:     t.orgID,
		LangfuseProjectID: t.projectID,
		Namespace:         details.Namespace,
		StartTime:         now.UnixMilli(),
		Status:            "RUNNING",
		Input: map[string]interface{}{
			"experimentName": details.ExperimentName,
			"experimentType": details.ExperimentType,
			"faultName":      details.FaultName,
			"agentName":      details.AgentName,
			"agentPlatform":  details.AgentPlatform,
			"agentVersion":   details.AgentVersion,
			"serviceAccount": details.AgentServiceAccount,
			"namespace":      details.Namespace,
			"phase":          details.Phase,
			"priority":       details.Priority,
		},
		Metadata: map[string]interface{}{
			"agent_name":          details.AgentName,
			"agent_platform":      details.AgentPlatform,
			"agent_version":       details.AgentVersion,
			"agent_id":            details.AgentID,
			"service_account":     details.AgentServiceAccount,
			"experimentType":      details.ExperimentType,
			"phase":               details.Phase,
			"priority":            details.Priority,
			"experiment_id":       details.ExperimentID,
			"experiment_run_id":   details.SessionID,
			"notify_id":           details.TraceID,
			"workflow_name":       details.ExperimentName,
			"namespace":           details.Namespace,
		},
	}

	// Send trace to channel for async processing.
	// Keep the read lock during send so Close() cannot close the channel mid-send.
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return nil
	}
	select {
	case t.traceChan <- trace:
		t.mu.RUnlock()
		return nil
	case <-ctx.Done():
		t.mu.RUnlock()
		return ctx.Err()
	default:
		t.mu.RUnlock()
		// Channel full, log warning but don't block
		fmt.Printf("[Observability] Langfuse trace queue full, sending synchronously: %s\n", details.TraceID)
		syncCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := t.client.TraceExperiment(syncCtx, trace); err != nil {
			fmt.Printf("[Observability] Failed to synchronously submit trace %s: %v\n", details.TraceID, err)
			return err
		}
		return nil
	}
}

// CompleteExperimentExecution logs the completion of an experiment/fault execution.
// Call this when the chaos fault has finished executing.
func (t *LangfuseTracer) CompleteExperimentExecution(ctx context.Context, traceID string, endDetails *ExperimentCompletionDetails) error {
	if !t.IsEnabled() {
		return nil
	}
	if traceID == "" || endDetails == nil {
		return fmt.Errorf("invalid completion details")
	}

	now := time.Now().Format(time.RFC3339)

	// Update root trace with final output via upsert (same TraceID)
	updateCtx, updateCancel := context.WithTimeout(ctx, 10*time.Second)
	defer updateCancel()
	if err := t.client.TraceExperiment(updateCtx, &agent_registry.ExperimentTrace{
		TraceID: traceID,
		// Preserve canonical root trace name on completion upsert as well.
		Name:    "experiment-run",
		Output: map[string]interface{}{
			"status":       endDetails.Status,
			"result":       endDetails.Result,
			"errorMessage": endDetails.ErrorMessage,
		},
		Metadata: map[string]interface{}{
			"completedAt": now,
			"finalStatus": endDetails.Status,
		},
	}); err != nil {
		fmt.Printf("[Observability] Failed to update trace output for %s: %v\n", traceID, err)
	}

	// Also log completion as an observation
	t.TraceExperimentObservation(ctx, &ExperimentObservationDetails{
		TraceID:   traceID,
		Name:      fmt.Sprintf("completion: %s", endDetails.ExperimentName),
		Type:      "EVENT",
		StartTime: now,
		EndTime:   now,
		Output: map[string]interface{}{
			"status": endDetails.Status,
			"result": endDetails.Result,
			"error":  endDetails.ErrorMessage,
		},
		Metadata: map[string]interface{}{
			"completionPhase": "post-execution",
		},
	})

	return nil
}

// TraceExperimentObservation logs a continuous observation/event for an experiment run.
func (t *LangfuseTracer) TraceExperimentObservation(ctx context.Context, details *ExperimentObservationDetails) error {
	if !t.IsEnabled() {
		return nil
	}
	if details == nil || details.TraceID == "" || details.Name == "" {
		return nil
	}

	var startTime *string
	if details.StartTime != "" {
		startTime = &details.StartTime
	}

	var endTime *string
	if details.EndTime != "" {
		endTime = &details.EndTime
	}

	payload := &agent_registry.LangfuseObservationPayload{
		TraceID:   details.TraceID,
		Name:      details.Name,
		Type:      details.Type,
		StartTime: startTime,
		EndTime:   endTime,
		Input:     details.Input,
		Output:    details.Output,
		Metadata:  details.Metadata,
	}

	obsCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := t.client.CreateObservation(obsCtx, payload); err != nil {
		fmt.Printf("[Observability] Failed to submit observation %s: %v\n", details.Name, err)
	}

	return nil
}

// ExperimentContextForTrace holds agent/experiment identity fields that are
// emitted as an "experiment_context" SPAN before all fault spans so the
// certifier's metadata scan finds them before hitting the first "fault: *" span.
type ExperimentContextForTrace struct {
	AgentID        string
	AgentName      string
	AgentPlatform  string
	AgentVersion   string
	ExperimentID   string
	ExperimentName string
	Namespace      string
}

// EmitFaultSpansForTrace posts one "fault: <name>" SPAN observation per fault to
// Langfuse so the certifier's fault bucketing pipeline has deterministic anchors
// with full ground truth. Called once per experiment run at experiment start.
//
// It first emits a single "experiment_context" SPAN carrying agent/experiment
// identity at timestamp T, then emits fault spans at T+1s. This ordering
// guarantees the certifier's chronological metadata scan finds agent_id,
// agent_name, experiment_id, and run_id before it stops at the first fault span.
//
// traceID     — the agent trace ID (notifyID / UUID with dashes)
// faultNames  — ordered list of fault names, e.g. ["pod-delete", "disk-fill"]
// groundTruth — decoded map of fault name → ground truth data (from chaos hub YAML)
// expCtx      — agent/experiment identity to embed in the experiment_context span
func (t *LangfuseTracer) EmitFaultSpansForTrace(
	ctx context.Context,
	traceID string,
	faultNames []string,
	groundTruth map[string]interface{},
	expCtx ExperimentContextForTrace,
) {
	if !t.IsEnabled() || traceID == "" || len(faultNames) == 0 {
		return
	}

	base := time.Now().UTC()
	// experiment_context span at T — certifier scans this BEFORE fault spans
	ctxNow := base.Format("2006-01-02T15:04:05.000Z")
	// fault spans at T+1s — guaranteed to sort after experiment_context
	now := base.Add(time.Second).Format("2006-01-02T15:04:05.000Z")

	// --- Emit experiment_context span first (on agent trace) ---
	ctxPayload := &agent_registry.LangfuseObservationPayload{
		TraceID:   traceID,
		Name:      "experiment_context",
		Type:      "SPAN",
		StartTime: &ctxNow,
		EndTime:   &ctxNow,
		Input:     map[string]interface{}{},
		Output:    map[string]interface{}{"status": "context_recorded"},
		Metadata: map[string]interface{}{
			"agent_id":        expCtx.AgentID,
			"agent_name":      expCtx.AgentName,
			"agent_platform":  expCtx.AgentPlatform,
			"agent_version":   expCtx.AgentVersion,
			"experiment_id":   expCtx.ExperimentID,
			"experiment_name": expCtx.ExperimentName,
			"run_id":          traceID,
			"namespace":       expCtx.Namespace,
			"fault_names":     faultNames,
		},
	}
	ctxCtx, ctxCancel := context.WithTimeout(ctx, 10*time.Second)
	if err := t.client.CreateObservation(ctxCtx, ctxPayload); err != nil {
		fmt.Printf("[Observability] Failed to emit experiment_context span for trace %s: %v\n", traceID, err)
	}
	ctxCancel()

	for _, fname := range faultNames {
		// ftData is the full ground truth for this fault as loaded from ground_truth.yaml.
		// It already contains fault_description_goal_remediation, ideal_course_of_action,
		// and ideal_tool_usage_trajectory — use it directly without decomposing.
		ftData, _ := groundTruth[fname].(map[string]interface{})

		inputData := map[string]interface{}{
			"fault_name":   fname,
			"ground_truth": ftData,
		}
		metaData := map[string]interface{}{
			"action":          "fault_injection",
			"fault_name":      fname,
			"ground_truth":    ftData,
			"llm_used":        false,
			"tokens_consumed": 0,
			"attributes": map[string]interface{}{
				"fault.target_namespace": expCtx.Namespace,
				"fault.target_label":     fname,
			},
		}

		payload := &agent_registry.LangfuseObservationPayload{
			TraceID:   traceID,
			Name:      "fault: " + fname,
			Type:      "SPAN",
			StartTime: &now,
			EndTime:   &now,
			Input:     inputData,
			Output:    map[string]interface{}{"status": "injected"},
			Metadata:  metaData,
		}

		obsCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if err := t.client.CreateObservation(obsCtx, payload); err != nil {
			fmt.Printf("[Observability] Failed to emit fault span '%s' for trace %s: %v\n", fname, traceID, err)
		}
		cancel()
	}
}

// ScoreExperimentExecution logs a score for the experiment run.
func (t *LangfuseTracer) ScoreExperimentExecution(ctx context.Context, details *ExperimentScoreDetails) error {
	if !t.IsEnabled() {
		return nil
	}
	if details == nil || details.TraceID == "" || details.Name == "" {
		return nil
	}

	payload := &agent_registry.LangfuseScorePayload{
		TraceID: details.TraceID,
		Name:    details.Name,
		Value:   details.Value,
		Comment: details.Comment,
		Source:  details.Source,
	}

	scoreCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := t.client.CreateScore(scoreCtx, payload); err != nil {
		fmt.Printf("[Observability] Failed to submit score %s: %v\n", details.Name, err)
	}

	return nil
}

// IsEnabled returns whether Langfuse tracing is enabled.
func (t *LangfuseTracer) IsEnabled() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.enabled
}

// Close gracefully shuts down the tracer and processes remaining traces.
func (t *LangfuseTracer) Close(ctx context.Context) error {
	if !t.IsEnabled() {
		return nil
	}

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	close(t.traceChan)
	workerDone := t.workerDone
	t.mu.Unlock()

	select {
	case <-workerDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// traceWorker is a background goroutine that processes traces asynchronously.
func (t *LangfuseTracer) traceWorker() {
	defer close(t.workerDone)
	for trace := range t.traceChan {
		// Create a timeout context for each trace submission
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		// Submit trace to Langfuse
		if err := t.client.TraceExperiment(ctx, trace); err != nil {
			fmt.Printf("[Observability] Failed to submit trace %s: %v\n", trace.TraceID, err)
		}

		cancel()
	}
}

// ExperimentExecutionDetails contains details about an experiment execution.
type ExperimentExecutionDetails struct {
	TraceID        string
	ExperimentID   string
	ExperimentName string
	ExperimentType string
	FaultName      string
	SessionID      string
	AgentID        string
	AgentName      string
	AgentPlatform  string
	AgentVersion   string
	AgentServiceAccount string
	ProjectID      string
	Namespace      string
	Phase          string // e.g., "injection", "post-chaos"
	Priority       string // e.g., "high", "low"
}

// ExperimentCompletionDetails contains details about experiment completion.
type ExperimentCompletionDetails struct {
	ExperimentID   string
	ExperimentName string
	Status         string // PASS, FAIL, RUNNING
	Result         string
	ErrorMessage   string
}

// ExperimentObservationDetails contains details about a continuous observation/event.
type ExperimentObservationDetails struct {
	TraceID  string
	Name     string
	Type     string
	StartTime string
	EndTime   string
	Input    map[string]interface{}
	Output   map[string]interface{}
	Metadata map[string]interface{}
}

// ExperimentScoreDetails contains details about a scoring event.
type ExperimentScoreDetails struct {
	TraceID string
	Name    string
	Value   float64
	Comment string
	Source  string
}
