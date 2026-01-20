package langfuse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// CreateTrace creates a new trace for a benchmark execution
func (c *Client) CreateTrace(ctx context.Context, trace TracePayload) (*Trace, error) {
	if trace.Name == "" {
		return nil, fmt.Errorf("trace name is required")
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, "/api/public/traces", trace)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace: %w", err)
	}

	var result Trace
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal trace: %w", err)
	}

	return &result, nil
}

// CreateTraceAsync creates a trace asynchronously via batch ingestion
func (c *Client) CreateTraceAsync(trace TracePayload) {
	c.addToBatch("trace-create", trace)
}

// UpdateTrace updates an existing trace
func (c *Client) UpdateTrace(ctx context.Context, traceID string, trace TracePayload) error {
	if traceID == "" {
		return fmt.Errorf("trace ID is required")
	}

	trace.ID = traceID
	c.addToBatch("trace-create", trace)
	return nil
}

// GetTrace retrieves a trace by ID
func (c *Client) GetTrace(ctx context.Context, traceID string) (*Trace, error) {
	if traceID == "" {
		return nil, fmt.Errorf("trace ID is required")
	}

	path := fmt.Sprintf("/api/public/traces/%s", traceID)
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get trace: %w", err)
	}

	var trace Trace
	if err := json.Unmarshal(respBody, &trace); err != nil {
		return nil, fmt.Errorf("failed to unmarshal trace: %w", err)
	}

	return &trace, nil
}

// ListTraces queries traces with filters
func (c *Client) ListTraces(ctx context.Context, filter TraceFilter) (*TraceList, error) {
	params := url.Values{}

	if filter.UserID != "" {
		params.Set("userId", filter.UserID)
	}
	if filter.SessionID != "" {
		params.Set("sessionId", filter.SessionID)
	}
	if filter.Name != "" {
		params.Set("name", filter.Name)
	}
	if !filter.FromDate.IsZero() {
		params.Set("fromTimestamp", filter.FromDate.Format(time.RFC3339))
	}
	if !filter.ToDate.IsZero() {
		params.Set("toTimestamp", filter.ToDate.Format(time.RFC3339))
	}
	if filter.Limit > 0 {
		params.Set("limit", strconv.Itoa(filter.Limit))
	}
	if filter.Offset > 0 {
		params.Set("page", strconv.Itoa(filter.Offset/filter.Limit+1))
	}

	path := "/api/public/traces"
	if len(params) > 0 {
		path = path + "?" + params.Encode()
	}

	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list traces: %w", err)
	}

	var result TraceList
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal trace list: %w", err)
	}

	return &result, nil
}

// DeleteTrace deletes a trace by ID
func (c *Client) DeleteTrace(ctx context.Context, traceID string) error {
	if traceID == "" {
		return fmt.Errorf("trace ID is required")
	}

	path := fmt.Sprintf("/api/public/traces/%s", traceID)
	_, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("failed to delete trace: %w", err)
	}

	return nil
}
