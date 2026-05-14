package certification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a thin HTTP wrapper around the Certifier service that exposes
// the four endpoints the orchestrator drives:
//   - POST /api/v1/bucketing-extraction
//   - GET  /api/v1/tasks
//   - POST /api/v1/aggregation-certification
//   - GET  /api/v1/cert-tasks
//
// All methods accept context for cancellation and return parsed responses
// or a structured *APIError when the upstream returns a non-2xx status.
type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// APIError carries non-2xx responses back to callers.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("certifier api error: status=%d body=%s", e.StatusCode, e.Body)
}

// ----- bucketing extraction -----

type TraceSource struct {
	Type string `json:"type"`
}

type StorageConfig struct {
	Type          string `json:"type"`
	ContainerName string `json:"container_name,omitempty"`
	MetricsDir    string `json:"metrics_dir,omitempty"`
}

type BucketingRequest struct {
	AgentID       string        `json:"agent_id"`
	ExperimentID  string        `json:"experiment_id"`
	RunID         string        `json:"run_id"`
	TraceSource   TraceSource   `json:"trace_source"`
	StorageConfig StorageConfig `json:"storage_config"`
}

type BucketingAcceptedResponse struct {
	Status  string `json:"status"`
	TaskID  string `json:"task_id"`
	PollURL string `json:"poll_url"`
}

func (c *Client) StartBucketing(ctx context.Context, req BucketingRequest) (*BucketingAcceptedResponse, error) {
	var out BucketingAcceptedResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/bucketing-extraction", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ----- bucketing poll -----

type BucketingTaskStatus struct {
	TaskID      string                 `json:"task_id"`
	Status      string                 `json:"status"`
	Stage       string                 `json:"stage"`
	Result      map[string]interface{} `json:"result"`
	Error       interface{}            `json:"error"`
	CompletedAt string                 `json:"completed_at"`
}

func (c *Client) PollBucketing(ctx context.Context, experimentID, runID string) (*BucketingTaskStatus, error) {
	path := fmt.Sprintf("/api/v1/tasks?experiment_id=%s&experiment_run_id=%s", experimentID, runID)
	var out BucketingTaskStatus
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ----- aggregation -----

type AggregationRequest struct {
	AgentID            string        `json:"agent_id"`
	AgentName          string        `json:"agent_name"`
	ExperimentID       string        `json:"experiment_id"`
	CertificationRunID string        `json:"certification_run_id"`
	RunsPerFault       int           `json:"runs_per_fault"`
	StorageConfig      StorageConfig `json:"storage_config"`
}

type AggregationAcceptedResponse struct {
	Status     string `json:"status"`
	CertTaskID string `json:"cert_task_id"`
	PollURL    string `json:"poll_url"`
}

func (c *Client) StartAggregation(ctx context.Context, req AggregationRequest) (*AggregationAcceptedResponse, error) {
	var out AggregationAcceptedResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/aggregation-certification", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ----- aggregation poll -----

type AggregationTaskStatus struct {
	CertTaskID  string                 `json:"cert_task_id"`
	Status      string                 `json:"status"`
	Stage       string                 `json:"stage"`
	Result      map[string]interface{} `json:"result"`
	Error       *AggregationErrorBody  `json:"error"`
	CompletedAt string                 `json:"completed_at"`
}

type AggregationErrorBody struct {
	ErrorCode   string `json:"error_code"`
	Message     string `json:"message"`
	FailedStage string `json:"failed_stage"`
	Detail      string `json:"detail"`
}

func (c *Client) PollAggregation(ctx context.Context, experimentID string) (*AggregationTaskStatus, error) {
	path := fmt.Sprintf("/api/v1/cert-tasks?experiment_id=%s", experimentID)
	var out AggregationTaskStatus
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// do is the shared request helper; it marshals body, decodes JSON, and
// produces an *APIError for non-2xx responses.  409 is returned to the
// caller as a regular response (not an error) since the orchestrator
// treats it as "already-active" and continues polling.
func (c *Client) do(ctx context.Context, method, path string, body, out interface{}) error {
	if c.baseURL == "" {
		return fmt.Errorf("certifier base url not configured")
	}
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out == nil {
			return nil
		}
		return json.Unmarshal(respBody, out)
	}
	// 409 still gives a body we want to parse so the orchestrator can
	// recover the existing task_id.
	if resp.StatusCode == http.StatusConflict && out != nil {
		_ = json.Unmarshal(respBody, out)
	}
	return &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
}
