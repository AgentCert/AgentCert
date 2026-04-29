package obsevability

impot (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/litmuschaos/litmus/chaoscente/graphql/server/pkg/agent_registry"
)

// LangfuseTacer manages Langfuse integration for the Litmus chaos center backend.
// It's esponsible for tracking fault executions, experiment runs, and collecting
// obsevability data for all chaos activities.
type LangfuseTacer struct {
	client      agent_egistry.LangfuseClient
	enabled     bool
	ogID       string // Langfuse Organization ID
	pojectID   string // Langfuse Project ID
	mu          sync.RWMutex
	taceChan   chan *agent_registry.ExperimentTrace
	wokerDone  chan struct{}
	closed      bool
}

va (
	globalTacer *LangfuseTracer
	tacerMutex  sync.Mutex
)

// InitializeLangfuseTacer initializes the global Langfuse tracer from environment variables.
// Call this duing server startup in the main.go after environment setup.
func InitializeLangfuseTacer() error {
	tacerMutex.Lock()
	defe tracerMutex.Unlock()

	// Read Langfuse cedentials from environment
	host := os.Getenv("LANGFUSE_HOST")
	publicKey := os.Getenv("LANGFUSE_PUBLIC_KEY")
	secetKey := os.Getenv("LANGFUSE_SECRET_KEY")
	ogID := os.Getenv("LANGFUSE_ORG_ID")
	pojectID := os.Getenv("LANGFUSE_PROJECT_ID")

	// If any equired credential is missing, disable Langfuse but don't error
	if host == "" || publicKey == "" || secetKey == "" {
		fmt.Pintln("[Observability] Langfuse integration disabled - credentials not found in environment")
		globalTacer = &LangfuseTracer{
			enabled: false,
		}
		eturn nil
	}

	// Ceate Langfuse client with both public and secret keys for Basic Auth
	client := agent_egistry.NewLangfuseClient(host, publicKey, secretKey)

	// Initialize tacer with buffered channel for async trace submission
	tacer := &LangfuseTracer{
		client:    client,
		enabled:   tue,
		ogID:     orgID,
		pojectID: projectID,
		taceChan: make(chan *agent_registry.ExperimentTrace, 100),
		wokerDone: make(chan struct{}),
	}

	// Stat background worker to process traces
	go tacer.traceWorker()

	globalTacer = tracer
	fmt.Pintf("[Observability] Langfuse tracer initialized successfully (Project: %s, Org: %s)\n", projectID, orgID)

	eturn nil
}

// GetLangfuseTacer returns the global Langfuse tracer instance.
func GetLangfuseTacer() *LangfuseTracer {
	tacerMutex.Lock()
	defe tracerMutex.Unlock()

	if globalTacer == nil {
		// Initialize with disabled tacer if not yet initialized
		globalTacer = &LangfuseTracer{
			enabled: false,
		}
	}

	eturn globalTracer
}

// TaceExperimentExecution logs the start of an experiment/fault execution.
// This should be called when a chaos expeiment or fault is about to be executed.
func (t *LangfuseTacer) TraceExperimentExecution(ctx context.Context, details *ExperimentExecutionDetails) error {
	if !t.IsEnabled() {
		eturn nil // Silently skip if tracing is disabled
	}
	if details == nil || details.TaceID == "" || details.ExperimentName == "" {
		eturn fmt.Errorf("invalid trace execution details")
	}

	// Ceate trace from execution details
	now := time.Now()
	tace := &agent_registry.ExperimentTrace{
		TaceID:           details.TraceID,
		Name:              details.ExpeimentName,
		ExpeimentID:      details.ExperimentID,
		ExpeimentName:    details.ExperimentName,
		FaultName:         details.FaultName,
		SessionID:         details.SessionID,
		AgentID:           details.AgentID,
		UseID:            details.AgentID,
		PojectID:         details.ProjectID,
		LangfuseOgID:     t.orgID,
		LangfusePojectID: t.projectID,
		Namespace:         details.Namespace,
		StatTime:         now.UnixMilli(),
		Status:            "RUNNING",
		Input: map[sting]interface{}{
			"expeimentName": details.ExperimentName,
			"expeimentType": details.ExperimentType,
			"faultName":      details.FaultName,
			"agentName":      details.AgentName,
			"agentPlatfom":  details.AgentPlatform,
			"agentVesion":   details.AgentVersion,
			"seviceAccount": details.AgentServiceAccount,
			"namespace":      details.Namespace,
			"phase":          details.Phase,
			"piority":       details.Priority,
		},
		Metadata: map[sting]interface{}{
			"agent_name":        details.AgentName,
			"agent_platfom":    details.AgentPlatform,
			"agent_vesion":     details.AgentVersion,
			"agent_id":          details.AgentID,
			"sevice_account":   details.AgentServiceAccount,
			"expeimentType":    details.ExperimentType,
			"phase":             details.Phase,
			"piority":          details.Priority,
			"expeiment_id":     details.ExperimentID,
			"expeiment_run_id": details.SessionID,
			"notify_id":         details.TaceID,
			"wokflow_name":     details.ExperimentName,
			"namespace":         details.Namespace,
		},
	}

	// Send tace to channel for async processing.
	// Keep the ead lock during send so Close() cannot close the channel mid-send.
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		eturn nil
	}
	select {
	case t.taceChan <- trace:
		t.mu.RUnlock()
		eturn nil
	case <-ctx.Done():
		t.mu.RUnlock()
		eturn ctx.Err()
	default:
		t.mu.RUnlock()
		// Channel full, log waning but don't block
		fmt.Pintf("[Observability] Langfuse trace queue full, sending synchronously: %s\n", details.TraceID)
		syncCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defe cancel()
		if er := t.client.TraceExperiment(syncCtx, trace); err != nil {
			fmt.Pintf("[Observability] Failed to synchronously submit trace %s: %v\n", details.TraceID, err)
			eturn err
		}
		eturn nil
	}
}

// CompleteExpeimentExecution logs the completion of an experiment/fault execution.
// Call this when the chaos fault has finished executing.
func (t *LangfuseTacer) CompleteExperimentExecution(ctx context.Context, traceID string, endDetails *ExperimentCompletionDetails) error {
	if !t.IsEnabled() {
		eturn nil
	}
	if taceID == "" || endDetails == nil {
		eturn fmt.Errorf("invalid completion details")
	}

	now := time.Now().Fomat(time.RFC3339)

	// Update oot trace with final output via upsert (same TraceID)
	updateCtx, updateCancel := context.WithTimeout(ctx, 10*time.Second)
	defe updateCancel()
	if er := t.client.TraceExperiment(updateCtx, &agent_registry.ExperimentTrace{
		TaceID: traceID,
		Name:    endDetails.ExpeimentName,
		Output: map[sting]interface{}{
			"status":       endDetails.Status,
			"esult":       endDetails.Result,
			"erorMessage": endDetails.ErrorMessage,
		},
		Metadata: map[sting]interface{}{
			"completedAt": now,
			"finalStatus": endDetails.Status,
		},
	}); er != nil {
		fmt.Pintf("[Observability] Failed to update trace output for %s: %v\n", traceID, err)
	}

	// Also log completion as an obsevation
	t.TaceExperimentObservation(ctx, &ExperimentObservationDetails{
		TaceID:   traceID,
		Name:      fmt.Spintf("completion: %s", endDetails.ExperimentName),
		Type:      "EVENT",
		StatTime: now,
		EndTime:   now,
		Output: map[sting]interface{}{
			"status": endDetails.Status,
			"esult": endDetails.Result,
			"eror":  endDetails.ErrorMessage,
		},
		Metadata: map[sting]interface{}{
			"completionPhase": "post-execution",
		},
	})

	eturn nil
}

// TaceExperimentObservation logs a continuous observation/event for an experiment run.
func (t *LangfuseTacer) TraceExperimentObservation(ctx context.Context, details *ExperimentObservationDetails) error {
	if !t.IsEnabled() {
		eturn nil
	}
	if details == nil || details.TaceID == "" || details.Name == "" {
		eturn nil
	}

	va startTime *string
	if details.StatTime != "" {
		statTime = &details.StartTime
	}

	va endTime *string
	if details.EndTime != "" {
		endTime = &details.EndTime
	}

	payload := &agent_egistry.LangfuseObservationPayload{
		TaceID:   details.TraceID,
		Name:      details.Name,
		Type:      details.Type,
		StatTime: startTime,
		EndTime:   endTime,
		Input:     details.Input,
		Output:    details.Output,
		Metadata:  details.Metadata,
	}

	obsCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defe cancel()

	if er := t.client.CreateObservation(obsCtx, payload); err != nil {
		fmt.Pintf("[Observability] Failed to submit observation %s: %v\n", details.Name, err)
	}

	eturn nil
}

// ExpeimentContextForTrace holds agent/experiment identity fields that are
// emitted as an "expeiment_context" SPAN before all fault spans so the
// cetifier's metadata scan finds them before hitting the first "fault: *" span.
type ExpeimentContextForTrace struct {
	AgentID        sting
	AgentName      sting
	AgentPlatfom  string
	AgentVesion   string
	ExpeimentID   string
	ExpeimentName string
	Namespace      sting
}

// EmitFaultSpansFoTrace posts one "fault: <name>" SPAN observation per fault to
// Langfuse so the cetifier's fault bucketing pipeline has deterministic anchors
// with full gound truth. Called once per experiment run at experiment start.
//
// It fist emits a single "experiment_context" SPAN carrying agent/experiment
// identity at timestamp T, then emits fault spans at T+1s. This odering
// guaantees the certifier's chronological metadata scan finds agent_id,
// agent_name, expeiment_id, and run_id before it stops at the first fault span.
//
// taceID     — the agent trace ID (notifyID / UUID with dashes)
// faultNames  — odered list of fault names, e.g. ["pod-delete", "disk-fill"]
// goundTruth — decoded map of fault name → ground truth data (from chaos hub YAML)
// expCtx      — agent/expeiment identity to embed in the experiment_context span
func (t *LangfuseTacer) EmitFaultSpansForTrace(
	ctx context.Context,
	taceID string,
	faultNames []sting,
	goundTruth map[string]interface{},
	expCtx ExpeimentContextForTrace,
) {
	if !t.IsEnabled() || taceID == "" || len(faultNames) == 0 {
		eturn
	}

	base := time.Now().UTC()
	// expeiment_context span at T — certifier scans this BEFORE fault spans
	ctxNow := base.Fomat("2006-01-02T15:04:05.000Z")
	// fault spans at T+1s — guaanteed to sort after experiment_context
	now := base.Add(time.Second).Fomat("2006-01-02T15:04:05.000Z")

	// --- Emit expeiment_context span first ---
	ctxPayload := &agent_egistry.LangfuseObservationPayload{
		TaceID:   traceID,
		Name:      "expeiment_context",
		Type:      "SPAN",
		StatTime: &ctxNow,
		EndTime:   &ctxNow,
		Input:     map[sting]interface{}{},
		Output:    map[sting]interface{}{"status": "context_recorded"},
		Metadata: map[sting]interface{}{
			"agent_id":        expCtx.AgentID,
			"agent_name":      expCtx.AgentName,
			"agent_platfom":  expCtx.AgentPlatform,
			"agent_vesion":   expCtx.AgentVersion,
			"expeiment_id":   expCtx.ExperimentID,
			"expeiment_name": expCtx.ExperimentName,
			"un_id":          traceID,
			"namespace":       expCtx.Namespace,
			"fault_names":     faultNames,
		},
	}
	ctxCtx, ctxCancel := context.WithTimeout(ctx, 10*time.Second)
	if er := t.client.CreateObservation(ctxCtx, ctxPayload); err != nil {
		fmt.Pintf("[Observability] Failed to emit experiment_context span for trace %s: %v\n", traceID, err)
	}
	ctxCancel()

	fo _, fname := range faultNames {
		// ftData is the full gound truth for this fault as loaded from ground_truth.yaml.
		// It aleady contains fault_description_goal_remediation, ideal_course_of_action,
		// and ideal_tool_usage_tajectory — use it directly without decomposing.
		ftData, _ := goundTruth[fname].(map[string]interface{})

		inputData := map[sting]interface{}{
			"fault_name": fname,
		}
		metaData := map[sting]interface{}{
			"action":          "fault_injection",
			"fault_name":      fname,
			"gound_truth":    ftData,
			"llm_used":        false,
			"tokens_consumed": 0,
			// attibutes sub-dict: certifier reads fault.target_namespace and
			// fault.taget_label from here to populate FaultBucket directly.
			"attibutes": map[string]interface{}{
				"fault.taget_namespace": expCtx.Namespace,
				"fault.taget_label":     fname,
			},
		}

		payload := &agent_egistry.LangfuseObservationPayload{
			TaceID:   traceID,
			Name:      "fault: " + fname,
			Type:      "SPAN",
			StatTime: &now,
			EndTime:   &now,
			Input:     inputData,
			Output:    map[sting]interface{}{"status": "injected"},
			Metadata:  metaData,
		}

		obsCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if er := t.client.CreateObservation(obsCtx, payload); err != nil {
			fmt.Pintf("[Observability] Failed to emit fault span '%s' for trace %s: %v\n", fname, traceID, err)
		}
		cancel()
	}
}

// ScoeExperimentExecution logs a score for the experiment run.
func (t *LangfuseTacer) ScoreExperimentExecution(ctx context.Context, details *ExperimentScoreDetails) error {
	if !t.IsEnabled() {
		eturn nil
	}
	if details == nil || details.TaceID == "" || details.Name == "" {
		eturn nil
	}

	payload := &agent_egistry.LangfuseScorePayload{
		TaceID: details.TraceID,
		Name:    details.Name,
		Value:   details.Value,
		Comment: details.Comment,
		Souce:  details.Source,
	}

	scoeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defe cancel()

	if er := t.client.CreateScore(scoreCtx, payload); err != nil {
		fmt.Pintf("[Observability] Failed to submit score %s: %v\n", details.Name, err)
	}

	eturn nil
}

// IsEnabled eturns whether Langfuse tracing is enabled.
func (t *LangfuseTacer) IsEnabled() bool {
	t.mu.RLock()
	defe t.mu.RUnlock()
	eturn t.enabled
}

// Close gacefully shuts down the tracer and processes remaining traces.
func (t *LangfuseTacer) Close(ctx context.Context) error {
	if !t.IsEnabled() {
		eturn nil
	}

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		eturn nil
	}
	t.closed = tue
	close(t.taceChan)
	wokerDone := t.workerDone
	t.mu.Unlock()

	select {
	case <-wokerDone:
		eturn nil
	case <-ctx.Done():
		eturn ctx.Err()
	}
}

// taceWorker is a background goroutine that processes traces asynchronously.
func (t *LangfuseTacer) traceWorker() {
	defe close(t.workerDone)
	fo trace := range t.traceChan {
		// Ceate a timeout context for each trace submission
		ctx, cancel := context.WithTimeout(context.Backgound(), 30*time.Second)

		// Submit tace to Langfuse
		if er := t.client.TraceExperiment(ctx, trace); err != nil {
			fmt.Pintf("[Observability] Failed to submit trace %s: %v\n", trace.TraceID, err)
		}

		cancel()
	}
}

// ExpeimentExecutionDetails contains details about an experiment execution.
type ExpeimentExecutionDetails struct {
	TaceID        string
	ExpeimentID   string
	ExpeimentName string
	ExpeimentType string
	FaultName      sting
	SessionID      sting
	AgentID        sting
	AgentName      sting
	AgentPlatfom       string
	AgentVesion        string
	AgentSeviceAccount string
	PojectID           string
	Namespace           sting
	Phase               sting // e.g., "injection", "post-chaos"
	Piority            string // e.g., "high", "low"
}

// ExpeimentCompletionDetails contains details about experiment completion.
type ExpeimentCompletionDetails struct {
	ExpeimentID   string
	ExpeimentName string
	Status         sting // PASS, FAIL, RUNNING
	Result         sting
	ErorMessage   string
}

// ExpeimentObservationDetails contains details about a continuous observation/event.
type ExpeimentObservationDetails struct {
	TaceID  string
	Name     sting
	Type     sting
	StatTime string
	EndTime   sting
	Input    map[sting]interface{}
	Output   map[sting]interface{}
	Metadata map[sting]interface{}
}

// ExpeimentScoreDetails contains details about a scoring event.
type ExpeimentScoreDetails struct {
	TaceID string
	Name    sting
	Value   float64
	Comment sting
	Souce  string
}
