package langfuse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// CreateSpan creates a new span (agent action) within a trace
func (c *Client) CreateSpan(ctx context.Context, span SpanPayload) (*Span, error) {
	if span.TraceID == "" {
		return nil, fmt.Errorf("trace ID is required")
	}
	if span.Name == "" {
		return nil, fmt.Errorf("span name is required")
	}

	if span.StartTime.IsZero() {
		span.StartTime = time.Now().UTC()
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, "/api/public/spans", span)
	if err != nil {
		return nil, fmt.Errorf("failed to create span: %w", err)
	}

	var result Span
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal span: %w", err)
	}

	return &result, nil
}

// CreateSpanAsync creates a span asynchronously via batch ingestion
func (c *Client) CreateSpanAsync(span SpanPayload) {
	if span.StartTime.IsZero() {
		span.StartTime = time.Now().UTC()
	}
	c.addToBatch("span-create", span)
}

// UpdateSpan updates an existing span (e.g., to set end time)
func (c *Client) UpdateSpan(ctx context.Context, spanID string, span SpanPayload) error {
	if spanID == "" {
		return fmt.Errorf("span ID is required")
	}

	updatePayload := struct {
		SpanID string `json:"spanId"`
		SpanPayload
	}{
		SpanID:      spanID,
		SpanPayload: span,
	}

	c.addToBatch("span-update", updatePayload)
	return nil
}

// CreateGeneration creates a new LLM generation within a trace
func (c *Client) CreateGeneration(ctx context.Context, gen GenerationPayload) (*Generation, error) {
	if gen.TraceID == "" {
		return nil, fmt.Errorf("trace ID is required")
	}
	if gen.Name == "" {
		return nil, fmt.Errorf("generation name is required")
	}

	if gen.StartTime.IsZero() {
		gen.StartTime = time.Now().UTC()
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, "/api/public/generations", gen)
	if err != nil {
		return nil, fmt.Errorf("failed to create generation: %w", err)
	}

	var result Generation
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal generation: %w", err)
	}

	return &result, nil
}

// CreateGenerationAsync creates a generation asynchronously via batch ingestion
func (c *Client) CreateGenerationAsync(gen GenerationPayload) {
	if gen.StartTime.IsZero() {
		gen.StartTime = time.Now().UTC()
	}
	c.addToBatch("generation-create", gen)
}

// UpdateGeneration updates an existing generation
func (c *Client) UpdateGeneration(ctx context.Context, generationID string, gen GenerationPayload) error {
	if generationID == "" {
		return fmt.Errorf("generation ID is required")
	}

	updatePayload := struct {
		GenerationID string `json:"generationId"`
		GenerationPayload
	}{
		GenerationID:      generationID,
		GenerationPayload: gen,
	}

	c.addToBatch("generation-update", updatePayload)
	return nil
}

// Predefined span names for AgentCert agent actions
const (
	SpanFaultInjection   = "fault_injection"
	SpanFaultDetection   = "fault_detection"
	SpanDiagnosis        = "diagnosis"
	SpanRemediation      = "remediation"
	SpanVerification     = "verification"
	SpanRecovery         = "recovery"
	SpanQueryPodStatus   = "query_pod_status"
	SpanQueryLogs        = "query_logs"
	SpanRestartPod       = "restart_pod"
	SpanScaleDeployment  = "scale_deployment"
	SpanApplyManifest    = "apply_manifest"
	SpanRollback         = "rollback"
)

// Span levels
const (
	SpanLevelDebug   = "DEBUG"
	SpanLevelDefault = "DEFAULT"
	SpanLevelWarning = "WARNING"
	SpanLevelError   = "ERROR"
)
