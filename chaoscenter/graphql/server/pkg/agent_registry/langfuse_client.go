package agent_registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LangfuseClient defines the interface for Langfuse API integration.
type LangfuseClient interface {
	// CreateOrUpdateUser creates or updates a user in Langfuse
	CreateOrUpdateUser(ctx context.Context, payload *LangfuseUserPayload) error
	
	// DeleteUser marks a user as deleted in Langfuse
	DeleteUser(ctx context.Context, agentID string) error
	
	// TraceExperiment logs a fault/experiment execution trace to Langfuse
	TraceExperiment(ctx context.Context, trace *ExperimentTrace) error

	// CreateObservation logs an observation/event against a trace
	CreateObservation(ctx context.Context, payload *LangfuseObservationPayload) error

	// CreateScore logs a score against a trace
	CreateScore(ctx context.Context, payload *LangfuseScorePayload) error
}

// LangfuseUserPayload represents the payload for Langfuse user operations.
type LangfuseUserPayload struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Metadata map[string]interface{} `json:"metadata"`
}

// ExperimentTrace represents a fault/experiment execution trace for Langfuse.
type ExperimentTrace struct {
	TraceID           string                 `json:"id"`                    // Unique trace ID
	Name              string                 `json:"name"`                  // Experiment/Fault name
	Input             map[string]interface{} `json:"input,omitempty"`       // Input parameters/config
	Output            map[string]interface{} `json:"output,omitempty"`      // Output/results data
	ExperimentID      string                 `json:"experimentId,omitempty"`
	ExperimentName    string                 `json:"experimentName,omitempty"`
	FaultName         string                 `json:"faultName,omitempty"`
	SessionID         string                 `json:"sessionId,omitempty"`    // Experiment run session
	AgentID           string                 `json:"agentId,omitempty"`      // Agent deploying the fault
	UserID            string                 `json:"userId,omitempty"`       // Langfuse user ID (infra/agent identifier)
	ProjectID         string                 `json:"projectId,omitempty"`    // AgentCert Project ID
	LangfuseOrgID     string                 `json:"langfuseOrgId,omitempty"`    // Langfuse Organization ID from env
	LangfuseProjectID string                 `json:"langfuseProjectId,omitempty"` // Langfuse Project ID from env
	Namespace         string                 `json:"namespace,omitempty"`    // K8s namespace
	StartTime         int64                  `json:"startTime"`              // Unix timestamp in milliseconds
	EndTime           *int64                 `json:"endTime,omitempty"`      // Unix timestamp in milliseconds
	Duration          *int64                 `json:"duration,omitempty"`     // Duration in milliseconds
	Status            string                 `json:"status,omitempty"`       // PASS, FAIL, RUNNING
	Result            string                 `json:"result,omitempty"`       // Detailed result
	ErrorMessage      string                 `json:"errorMessage,omitempty"` // Error message if failed
	Metadata          map[string]interface{} `json:"metadata,omitempty"`     // Additional metadata
}

// LangfuseObservationPayload represents an observation/event linked to a trace.
type LangfuseObservationPayload struct {
	ID        string                 `json:"id,omitempty"`
	TraceID   string                 `json:"traceId"`
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	StartTime *string                `json:"startTime,omitempty"`
	EndTime   *string                `json:"endTime,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	Output    map[string]interface{} `json:"output,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// LangfuseScorePayload represents a score linked to a trace.
type LangfuseScorePayload struct {
	ID      string  `json:"id,omitempty"`
	TraceID string  `json:"traceId"`
	Name    string  `json:"name"`
	Value   float64 `json:"value"`
	Comment string  `json:"comment,omitempty"`
	Source  string  `json:"source,omitempty"`
}

// langfuseClientImpl is the concrete implementation of the LangfuseClient interface.
type langfuseClientImpl struct {
	baseURL    string
	publicKey  string
	secretKey  string
	httpClient *http.Client
}

// NewLangfuseClient creates a new LangfuseClient instance.
func NewLangfuseClient(baseURL, publicKey, secretKey string) LangfuseClient {
	return &langfuseClientImpl{
		baseURL:    baseURL,
		publicKey:  publicKey,
		secretKey:  secretKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CreateOrUpdateUser creates or updates a user in Langfuse with retry logic.
func (c *langfuseClientImpl) CreateOrUpdateUser(ctx context.Context, payload *LangfuseUserPayload) error {
	if c.baseURL == "" || c.publicKey == "" || c.secretKey == "" {
		return fmt.Errorf("Langfuse client not configured: baseURL, publicKey, or secretKey is empty")
	}

	// Marshal payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Langfuse payload: %w", err)
	}

	// Retry logic with exponential backoff: 3 retries, delays 1s, 2s, 4s
	retryDelays := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	maxRetries := 3

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			delay := retryDelays[attempt-1]
			fmt.Printf("Retrying Langfuse sync after %v (attempt %d/%d)\n", delay, attempt, maxRetries)
			time.Sleep(delay)
		}

		// Create HTTP POST request
		url := fmt.Sprintf("%s/api/public/users", c.baseURL)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payloadBytes))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		// Set headers - Use Basic Auth with publicKey:secretKey
		req.SetBasicAuth(c.publicKey, c.secretKey)
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			fmt.Printf("Langfuse sync failed (attempt %d/%d): %v\n", attempt+1, maxRetries+1, err)
			continue
		}

		// Read response body
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Check response status
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
			fmt.Printf("Successfully synced agent to Langfuse (status %d)\n", resp.StatusCode)
			return nil
		}

		// Non-success status code
		lastErr = fmt.Errorf("Langfuse API returned status %d: %s", resp.StatusCode, string(body))
		fmt.Printf("Langfuse sync failed (attempt %d/%d): %v\n", attempt+1, maxRetries+1, lastErr)
	}

	// All retries exhausted
	return fmt.Errorf("failed to sync to Langfuse after %d retries: %w", maxRetries+1, lastErr)
}

// DeleteUser marks a user as deleted in Langfuse.
func (c *langfuseClientImpl) DeleteUser(ctx context.Context, agentID string) error {
	// Langfuse doesn't support hard delete, so we update metadata with deleted flag
	now := time.Now().Unix()
	payload := &LangfuseUserPayload{
		ID:   agentID,
		Name: "", // Name can be empty for deleted users
		Metadata: map[string]interface{}{
			"deleted":   true,
			"deletedAt": now,
		},
	}

	return c.CreateOrUpdateUser(ctx, payload)
}

// TraceExperiment logs a fault/experiment execution trace to Langfuse.
func (c *langfuseClientImpl) TraceExperiment(ctx context.Context, trace *ExperimentTrace) error {
	if c.baseURL == "" || c.publicKey == "" || c.secretKey == "" {
		return fmt.Errorf("Langfuse client not configured: baseURL, publicKey, or secretKey is empty")
	}

	// Validate required fields
	if trace.TraceID == "" {
		return fmt.Errorf("trace ID is required")
	}
	if trace.Name == "" {
		return fmt.Errorf("trace name is required")
	}

	// Marshal payload to JSON
	payloadBytes, err := json.Marshal(trace)
	if err != nil {
		return fmt.Errorf("failed to marshal experiment trace: %w", err)
	}

	// Retry logic: 3 retries with exponential backoff
	retryDelays := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	maxRetries := 3

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			delay := retryDelays[attempt-1]
			fmt.Printf("Retrying experiment trace upload after %v (attempt %d/%d)\n", delay, attempt, maxRetries)
			time.Sleep(delay)
		}

		// Create HTTP POST request to traces endpoint
		url := fmt.Sprintf("%s/api/public/traces", c.baseURL)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payloadBytes))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		// Set headers - Use Basic Auth with publicKey:secretKey
		req.SetBasicAuth(c.publicKey, c.secretKey)
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			fmt.Printf("Experiment trace upload failed (attempt %d/%d): %v\n", attempt+1, maxRetries+1, err)
			continue
		}

		// Read response body
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Check response status
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
			fmt.Printf("Successfully logged experiment trace to Langfuse (trace ID: %s, status: %d)\n", trace.TraceID, resp.StatusCode)
			return nil
		}

		// Non-success status code
		lastErr = fmt.Errorf("Langfuse API returned status %d: %s", resp.StatusCode, string(body))
		fmt.Printf("Experiment trace upload failed (attempt %d/%d): %v\n", attempt+1, maxRetries+1, lastErr)
	}

	// All retries exhausted
	return fmt.Errorf("failed to upload experiment trace after %d retries: %w", maxRetries+1, lastErr)
}

// CreateObservation logs an observation/event to Langfuse using the ingestion API.
func (c *langfuseClientImpl) CreateObservation(ctx context.Context, payload *LangfuseObservationPayload) error {
	if c.baseURL == "" || c.publicKey == "" || c.secretKey == "" {
		return fmt.Errorf("Langfuse client not configured: baseURL, publicKey, or secretKey is empty")
	}
	if payload == nil || payload.TraceID == "" || payload.Name == "" {
		return fmt.Errorf("observation payload requires traceId and name")
	}

	// Generate ID if not provided
	if payload.ID == "" {
		payload.ID = fmt.Sprintf("obs-%d", time.Now().UnixNano())
	}

	// Use ingestion API with batch format
	batchPayload := map[string]interface{}{
		"batch": []map[string]interface{}{
			{
				"id":        payload.ID,
				"type":      "observation-create",
				"timestamp": time.Now().Format(time.RFC3339),
				"body": map[string]interface{}{
					"id":        payload.ID,
					"traceId":   payload.TraceID,
					"type":      payload.Type,
					"name":      payload.Name,
					"startTime": payload.StartTime,
					"endTime":   payload.EndTime,
					"input":     payload.Input,
					"output":    payload.Output,
					"metadata":  payload.Metadata,
				},
			},
		},
	}

	payloadBytes, err := json.Marshal(batchPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal observation payload: %w", err)
	}

	retryDelays := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	maxRetries := 3

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryDelays[attempt-1]
			fmt.Printf("Retrying observation upload after %v (attempt %d/%d)\n", delay, attempt, maxRetries)
			time.Sleep(delay)
		}

		url := fmt.Sprintf("%s/api/public/ingestion", c.baseURL)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payloadBytes))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		req.SetBasicAuth(c.publicKey, c.secretKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			fmt.Printf("Observation upload failed (attempt %d/%d): %v\n", attempt+1, maxRetries+1, err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusMultiStatus {
			// Log response body for debugging
			if resp.StatusCode == http.StatusMultiStatus {
				fmt.Printf("Successfully logged observation to Langfuse (id: %s, status: %d) - Response: %s\n", payload.ID, resp.StatusCode, string(body))
			} else {
				fmt.Printf("Successfully logged observation to Langfuse (id: %s, status: %d)\n", payload.ID, resp.StatusCode)
			}
			return nil
		}

		lastErr = fmt.Errorf("Langfuse API returned status %d: %s", resp.StatusCode, string(body))
		fmt.Printf("Observation upload failed (attempt %d/%d): %v\n", attempt+1, maxRetries+1, lastErr)
	}

	return fmt.Errorf("failed to upload observation after %d retries: %w", maxRetries+1, lastErr)
}

// CreateScore logs a score to Langfuse using the ingestion API.
func (c *langfuseClientImpl) CreateScore(ctx context.Context, payload *LangfuseScorePayload) error {
	if c.baseURL == "" || c.publicKey == "" || c.secretKey == "" {
		return fmt.Errorf("Langfuse client not configured: baseURL, publicKey, or secretKey is empty")
	}
	if payload == nil || payload.TraceID == "" || payload.Name == "" {
		return fmt.Errorf("score payload requires traceId and name")
	}

	// Generate ID if not provided
	if payload.ID == "" {
		payload.ID = fmt.Sprintf("score-%d", time.Now().UnixNano())
	}

	// Use ingestion API with batch format
	batchPayload := map[string]interface{}{
		"batch": []map[string]interface{}{
			{
				"id":        payload.ID,
				"type":      "score-create",
				"timestamp": time.Now().Format(time.RFC3339),
				"body": map[string]interface{}{
					"id":      payload.ID,
					"traceId": payload.TraceID,
					"name":    payload.Name,
					"value":   payload.Value,
					"comment": payload.Comment,
					"source":  payload.Source,
				},
			},
		},
	}

	payloadBytes, err := json.Marshal(batchPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal score payload: %w", err)
	}

	retryDelays := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	maxRetries := 3

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryDelays[attempt-1]
			fmt.Printf("Retrying score upload after %v (attempt %d/%d)\n", delay, attempt, maxRetries)
			time.Sleep(delay)
		}

		url := fmt.Sprintf("%s/api/public/ingestion", c.baseURL)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payloadBytes))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		req.SetBasicAuth(c.publicKey, c.secretKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			fmt.Printf("Score upload failed (attempt %d/%d): %v\n", attempt+1, maxRetries+1, err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusMultiStatus {
			// Log response body for debugging 207 responses
			if resp.StatusCode == http.StatusMultiStatus {
				fmt.Printf("Successfully logged score to Langfuse (id: %s, status: %d) - Response: %s\n", payload.ID, resp.StatusCode, string(body))
			} else {
				fmt.Printf("Successfully logged score to Langfuse (id: %s, status: %d)\n", payload.ID, resp.StatusCode)
			}
			return nil
		}

		lastErr = fmt.Errorf("Langfuse API returned status %d: %s", resp.StatusCode, string(body))
		fmt.Printf("Score upload failed (attempt %d/%d): %v\n", attempt+1, maxRetries+1, lastErr)
	}

	return fmt.Errorf("failed to upload score after %d retries: %w", maxRetries+1, lastErr)
}
