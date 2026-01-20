package langfuse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// CreateSession creates a new session (benchmark project run)
// Sessions group related traces together for multi-run benchmarks
func (c *Client) CreateSession(ctx context.Context, session SessionPayload) error {
	if session.ID == "" {
		return fmt.Errorf("session ID is required")
	}

	// Use batch ingestion for better performance
	c.addToBatch("session-create", session)
	return nil
}

// CreateSessionSync creates a session synchronously
func (c *Client) CreateSessionSync(ctx context.Context, session SessionPayload) (*Session, error) {
	if session.ID == "" {
		return nil, fmt.Errorf("session ID is required")
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, "/api/public/sessions", session)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	var result Session
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &result, nil
}

// GetSession retrieves a session by ID
func (c *Client) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID is required")
	}

	path := fmt.Sprintf("/api/public/sessions/%s", sessionID)
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(respBody, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}
