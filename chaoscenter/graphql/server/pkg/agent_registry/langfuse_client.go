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
}

// LangfuseUserPayload represents the payload for Langfuse user operations.
type LangfuseUserPayload struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Metadata map[string]interface{} `json:"metadata"`
}

// langfuseClientImpl is the concrete implementation of the LangfuseClient interface.
type langfuseClientImpl struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewLangfuseClient creates a new LangfuseClient instance.
func NewLangfuseClient(baseURL, apiKey string) LangfuseClient {
	return &langfuseClientImpl{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CreateOrUpdateUser creates or updates a user in Langfuse with retry logic.
func (c *langfuseClientImpl) CreateOrUpdateUser(ctx context.Context, payload *LangfuseUserPayload) error {
	if c.baseURL == "" || c.apiKey == "" {
		return fmt.Errorf("Langfuse client not configured: baseURL or apiKey is empty")
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

		// Set headers
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
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
