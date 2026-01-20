package langfuse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// CreateScore creates a new score (evaluation metric)
// Scores are used to store TTD, TTR, success rate, and other benchmark metrics
func (c *Client) CreateScore(ctx context.Context, score ScorePayload) error {
	if score.TraceID == "" {
		return fmt.Errorf("trace ID is required")
	}
	if score.Name == "" {
		return fmt.Errorf("score name is required")
	}

	// Use batch ingestion for better performance
	c.addToBatch("score-create", score)
	return nil
}

// CreateScoreSync creates a score synchronously
func (c *Client) CreateScoreSync(ctx context.Context, score ScorePayload) (*Score, error) {
	if score.TraceID == "" {
		return nil, fmt.Errorf("trace ID is required")
	}
	if score.Name == "" {
		return nil, fmt.Errorf("score name is required")
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, "/api/public/scores", score)
	if err != nil {
		return nil, fmt.Errorf("failed to create score: %w", err)
	}

	var result Score
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal score: %w", err)
	}

	return &result, nil
}

// GetScores retrieves scores for a trace
func (c *Client) GetScores(ctx context.Context, traceID string) ([]Score, error) {
	if traceID == "" {
		return nil, fmt.Errorf("trace ID is required")
	}

	params := url.Values{}
	params.Set("traceId", traceID)

	path := "/api/public/scores?" + params.Encode()
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get scores: %w", err)
	}

	var result struct {
		Data []Score `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal scores: %w", err)
	}

	return result.Data, nil
}

// GetScoresByName retrieves scores by name across all traces
func (c *Client) GetScoresByName(ctx context.Context, name string, filter MetricsFilter) ([]Score, error) {
	if name == "" {
		return nil, fmt.Errorf("score name is required")
	}

	params := url.Values{}
	params.Set("name", name)

	if filter.UserID != "" {
		params.Set("userId", filter.UserID)
	}

	path := "/api/public/scores?" + params.Encode()
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get scores by name: %w", err)
	}

	var result struct {
		Data []Score `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal scores: %w", err)
	}

	return result.Data, nil
}

// DeleteScore deletes a score by ID
func (c *Client) DeleteScore(ctx context.Context, scoreID string) error {
	if scoreID == "" {
		return fmt.Errorf("score ID is required")
	}

	path := fmt.Sprintf("/api/public/scores/%s", scoreID)
	_, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("failed to delete score: %w", err)
	}

	return nil
}

// Predefined score names for AgentCert benchmarks
const (
	ScoreTimeToDetect    = "time_to_detect"     // TTD in seconds
	ScoreTimeToRemediate = "time_to_remediate"  // TTR in seconds
	ScoreTimeToRecover   = "time_to_recover"    // Time to full recovery in seconds
	ScoreSuccess         = "success"            // 1 for success, 0 for failure
	ScorePartialSuccess  = "partial_success"    // Percentage (0-100)
	ScoreFalsePositive   = "false_positive"     // Count of false positives
	ScoreActionsCount    = "actions_count"      // Number of actions taken
	ScoreTokensUsed      = "tokens_used"        // LLM tokens consumed
	ScoreCost            = "cost"               // Cost in USD
)

// CreateBenchmarkScores creates standard benchmark scores for a trace
func (c *Client) CreateBenchmarkScores(ctx context.Context, traceID string, ttd, ttr float64, success bool, comment string) error {
	successValue := 0.0
	if success {
		successValue = 1.0
	}

	// Create TTD score
	if err := c.CreateScore(ctx, ScorePayload{
		TraceID:  traceID,
		Name:     ScoreTimeToDetect,
		Value:    ttd,
		Comment:  fmt.Sprintf("Time to detect: %.2f seconds", ttd),
		DataType: "NUMERIC",
	}); err != nil {
		return err
	}

	// Create TTR score
	if err := c.CreateScore(ctx, ScorePayload{
		TraceID:  traceID,
		Name:     ScoreTimeToRemediate,
		Value:    ttr,
		Comment:  fmt.Sprintf("Time to remediate: %.2f seconds", ttr),
		DataType: "NUMERIC",
	}); err != nil {
		return err
	}

	// Create success score
	if err := c.CreateScore(ctx, ScorePayload{
		TraceID:  traceID,
		Name:     ScoreSuccess,
		Value:    successValue,
		Comment:  comment,
		DataType: "NUMERIC",
	}); err != nil {
		return err
	}

	return nil
}
