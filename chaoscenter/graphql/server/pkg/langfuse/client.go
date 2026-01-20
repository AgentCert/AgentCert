package langfuse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// Client is the Langfuse API client
type Client struct {
	config     *Config
	httpClient *http.Client
	logger     *logrus.Logger

	// Batch ingestion
	batchMu     sync.Mutex
	batchEvents []IngestionEvent
	batchTimer  *time.Timer
	batchSize   int
	flushDelay  time.Duration
}

// ClientOption is a function that configures the client
type ClientOption func(*Client)

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithBatchSize sets the batch size for ingestion
func WithBatchSize(size int) ClientOption {
	return func(c *Client) {
		c.batchSize = size
	}
}

// WithFlushDelay sets the flush delay for batch ingestion
func WithFlushDelay(delay time.Duration) ClientOption {
	return func(c *Client) {
		c.flushDelay = delay
	}
}

// NewClient creates a new Langfuse client
func NewClient(config *Config, logger *logrus.Logger, opts ...ClientOption) *Client {
	c := &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:      logger,
		batchEvents: make([]IngestionEvent, 0),
		batchSize:   100,
		flushDelay:  time.Second,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// IsEnabled returns true if Langfuse integration is enabled
func (c *Client) IsEnabled() bool {
	return c.config != nil && c.config.Enabled
}

// GetConfig returns the client configuration
func (c *Client) GetConfig() *Config {
	return c.config
}

// doRequest performs an HTTP request with authentication and retry logic
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	if !c.IsEnabled() {
		return nil, ErrNotEnabled
	}

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	url := c.config.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication header (Basic auth with public:secret)
	req.SetBasicAuth(c.config.PublicKey, c.config.SecretKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "AgentCert-Langfuse-Client/1.0")

	// Retry logic with exponential backoff
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			c.logger.Warnf("Langfuse request failed (attempt %d/%d): %v", attempt+1, maxRetries, err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, nil
		}

		langfuseErr := &LangfuseError{
			StatusCode: resp.StatusCode,
			Message:    resp.Status,
			Details:    string(respBody),
		}

		if !langfuseErr.IsRetryable() {
			return nil, langfuseErr
		}

		lastErr = langfuseErr
		c.logger.Warnf("Langfuse request returned %d (attempt %d/%d): %s", 
			resp.StatusCode, attempt+1, maxRetries, string(respBody))
	}

	return nil, fmt.Errorf("langfuse request failed after %d attempts: %w", maxRetries, lastErr)
}

// addToBatch adds an event to the batch queue
func (c *Client) addToBatch(eventType string, body interface{}) {
	if !c.IsEnabled() {
		return
	}

	c.batchMu.Lock()
	defer c.batchMu.Unlock()

	event := IngestionEvent{
		ID:        uuid.New().String(),
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Body:      body,
	}

	c.batchEvents = append(c.batchEvents, event)

	// Flush if batch size is reached
	if len(c.batchEvents) >= c.batchSize {
		go c.FlushBatch(context.Background())
		return
	}

	// Reset or start the flush timer
	if c.batchTimer == nil {
		c.batchTimer = time.AfterFunc(c.flushDelay, func() {
			c.FlushBatch(context.Background())
		})
	} else {
		c.batchTimer.Reset(c.flushDelay)
	}
}

// FlushBatch sends all queued events to Langfuse
func (c *Client) FlushBatch(ctx context.Context) error {
	c.batchMu.Lock()
	if len(c.batchEvents) == 0 {
		c.batchMu.Unlock()
		return nil
	}

	events := c.batchEvents
	c.batchEvents = make([]IngestionEvent, 0)
	if c.batchTimer != nil {
		c.batchTimer.Stop()
		c.batchTimer = nil
	}
	c.batchMu.Unlock()

	request := IngestionRequest{
		Batch: events,
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, "/api/public/ingestion", request)
	if err != nil {
		c.logger.Errorf("Failed to flush batch to Langfuse: %v", err)
		// Re-queue failed events
		c.batchMu.Lock()
		c.batchEvents = append(events, c.batchEvents...)
		c.batchMu.Unlock()
		return err
	}

	var response IngestionResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		c.logger.Warnf("Failed to parse ingestion response: %v", err)
	} else if len(response.Errors) > 0 {
		for _, e := range response.Errors {
			c.logger.Warnf("Ingestion error for event %s: %s - %s", e.ID, e.Message, e.Error)
		}
	}

	c.logger.Debugf("Flushed %d events to Langfuse, %d succeeded, %d failed",
		len(events), len(response.Successes), len(response.Errors))

	return nil
}

// Close flushes any remaining events and closes the client
func (c *Client) Close(ctx context.Context) error {
	return c.FlushBatch(ctx)
}

// HealthCheck verifies connectivity to Langfuse
func (c *Client) HealthCheck(ctx context.Context) error {
	if !c.IsEnabled() {
		return ErrNotEnabled
	}

	// Try to list traces with limit 1 to verify connectivity
	_, err := c.doRequest(ctx, http.MethodGet, "/api/public/traces?limit=1", nil)
	if err != nil {
		return fmt.Errorf("langfuse health check failed: %w", err)
	}

	return nil
}
