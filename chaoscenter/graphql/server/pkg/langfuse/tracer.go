package langfuse

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// BenchmarkTracer provides a high-level API for tracing AgentCert benchmarks
type BenchmarkTracer struct {
	client    *Client
	traceID   string
	sessionID string
	userID    string // Agent ID
	startTime time.Time
}

// NewBenchmarkTracer creates a new tracer for a benchmark run
func (c *Client) NewBenchmarkTracer(agentID, sessionID, benchmarkName string, metadata map[string]interface{}) *BenchmarkTracer {
	traceID := uuid.New().String()
	startTime := time.Now().UTC()

	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["agentId"] = agentID
	metadata["benchmarkStart"] = startTime.Format(time.RFC3339)

	// Create the trace asynchronously
	c.CreateTraceAsync(TracePayload{
		ID:        traceID,
		Name:      benchmarkName,
		UserID:    agentID,
		SessionID: sessionID,
		Metadata:  metadata,
		Tags:      []string{"benchmark", "agentcert"},
	})

	return &BenchmarkTracer{
		client:    c,
		traceID:   traceID,
		sessionID: sessionID,
		userID:    agentID,
		startTime: startTime,
	}
}

// TraceID returns the trace ID
func (t *BenchmarkTracer) TraceID() string {
	return t.traceID
}

// StartSpan starts a new span for an agent action
func (t *BenchmarkTracer) StartSpan(name string, input interface{}) *SpanHandle {
	spanID := uuid.New().String()
	startTime := time.Now().UTC()

	t.client.CreateSpanAsync(SpanPayload{
		TraceID:   t.traceID,
		Name:      name,
		StartTime: startTime,
		Input:     input,
		Level:     SpanLevelDefault,
	})

	return &SpanHandle{
		client:    t.client,
		traceID:   t.traceID,
		spanID:    spanID,
		startTime: startTime,
	}
}

// RecordGeneration records an LLM generation
func (t *BenchmarkTracer) RecordGeneration(name, model string, input, output interface{}, usage *Usage) {
	t.client.CreateGenerationAsync(GenerationPayload{
		TraceID:   t.traceID,
		Name:      name,
		Model:     model,
		StartTime: time.Now().UTC(),
		Input:     input,
		Output:    output,
		Usage:     usage,
	})
}

// RecordScore records an evaluation metric
func (t *BenchmarkTracer) RecordScore(name string, value float64, comment string) {
	t.client.CreateScore(context.Background(), ScorePayload{
		TraceID:  t.traceID,
		Name:     name,
		Value:    value,
		Comment:  comment,
		DataType: "NUMERIC",
	})
}

// RecordBenchmarkResults records the final benchmark results
func (t *BenchmarkTracer) RecordBenchmarkResults(ttd, ttr float64, success bool, output interface{}) {
	endTime := time.Now().UTC()
	duration := endTime.Sub(t.startTime).Seconds()

	// Update trace with output
	t.client.UpdateTrace(context.Background(), t.traceID, TracePayload{
		Output: output,
		Metadata: map[string]interface{}{
			"benchmarkEnd": endTime.Format(time.RFC3339),
			"duration":     duration,
			"success":      success,
		},
	})

	// Record standard benchmark scores
	t.client.CreateBenchmarkScores(context.Background(), t.traceID, ttd, ttr, success, "Benchmark completed")
}

// Complete finalizes the trace and flushes all events
func (t *BenchmarkTracer) Complete(ctx context.Context) error {
	return t.client.FlushBatch(ctx)
}

// SpanHandle represents an active span
type SpanHandle struct {
	client    *Client
	traceID   string
	spanID    string
	startTime time.Time
}

// End completes the span with output
func (s *SpanHandle) End(output interface{}, err error) {
	endTime := time.Now().UTC()
	level := SpanLevelDefault
	statusMsg := ""

	if err != nil {
		level = SpanLevelError
		statusMsg = err.Error()
	}

	s.client.UpdateSpan(context.Background(), s.spanID, SpanPayload{
		TraceID:   s.traceID,
		EndTime:   endTime,
		Output:    output,
		Level:     level,
		StatusMsg: statusMsg,
	})
}

// EndWithWarning completes the span with a warning
func (s *SpanHandle) EndWithWarning(output interface{}, warning string) {
	s.client.UpdateSpan(context.Background(), s.spanID, SpanPayload{
		TraceID:   s.traceID,
		EndTime:   time.Now().UTC(),
		Output:    output,
		Level:     SpanLevelWarning,
		StatusMsg: warning,
	})
}
