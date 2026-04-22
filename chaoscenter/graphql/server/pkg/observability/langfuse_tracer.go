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
		Name:              details.ExperimentName,
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
			"serviceAccount": details.AgentServiceAccount,
			"namespace":      details.Namespace,
			"phase":          details.Phase,
			"priority":       details.Priority,
		},
		Metadata: map[string]interface{}{
			"agent_name":      details.AgentName,
			"agent_platform":  details.AgentPlatform,
			"agent_id":        details.AgentID,
			"service_account": details.AgentServiceAccount,
			"experimentType": details.ExperimentType,
			"phase":          details.Phase,
			"priority":       details.Priority,
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
		Name:    endDetails.ExperimentName,
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
