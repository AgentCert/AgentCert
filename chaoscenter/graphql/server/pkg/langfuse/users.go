package langfuse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// CreateOrUpdateUser creates or updates a user (agent) in Langfuse
// Users in Langfuse map to AI Agents in AgentCert
func (c *Client) CreateOrUpdateUser(ctx context.Context, user UserPayload) error {
	if user.ID == "" {
		return fmt.Errorf("user ID is required")
	}

	// Use batch ingestion for better performance
	c.addToBatch("user-create", user)
	return nil
}

// CreateOrUpdateUserSync creates or updates a user synchronously
func (c *Client) CreateOrUpdateUserSync(ctx context.Context, user UserPayload) error {
	if user.ID == "" {
		return fmt.Errorf("user ID is required")
	}

	_, err := c.doRequest(ctx, http.MethodPost, "/api/public/users", user)
	if err != nil {
		return fmt.Errorf("failed to create/update user: %w", err)
	}
	return nil
}

// GetUser retrieves a user by ID
func (c *Client) GetUser(ctx context.Context, userID string) (*User, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	path := fmt.Sprintf("/api/public/users/%s", userID)
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	var user User
	if err := json.Unmarshal(respBody, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user: %w", err)
	}

	return &user, nil
}
