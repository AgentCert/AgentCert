package langfuse

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// Client represents a Langfuse client for tracing
type Client struct {
	host       string
	publicKey  string
	secretKey  string
	httpClient *http.Client
	events     []Event
	mu         sync.Mutex
	enabled    bool
}

// Event represents a Langfuse event
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Body      map[string]interface{} `json:"body"`
}

// TraceCreateBody represents the body for creating a trace
type TraceCreateBody struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name,omitempty"`
	UserID    string                 `json:"userId,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Input     interface{}            `json:"input,omitempty"`
	Output    interface{}            `json:"output,omitempty"`
}

// SpanCreateBody represents the body for creating a span
type SpanCreateBody struct {
	ID        string                 `json:"id"`
	TraceID   string                 `json:"traceId"`
	ParentID  string                 `json:"parentObservationId,omitempty"`
	Name      string                 `json:"name"`
	StartTime time.Time              `json:"startTime"`
	EndTime   *time.Time             `json:"endTime,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Input     interface{}            `json:"input,omitempty"`
	Output    interface{}            `json:"output,omitempty"`
	Level     string                 `json:"level,omitempty"`
	Status    string                 `json:"statusMessage,omitempty"`
}

// GenerationCreateBody represents the body for creating a generation (LLM call)
type GenerationCreateBody struct {
	ID               string                 `json:"id"`
	TraceID          string                 `json:"traceId"`
	ParentID         string                 `json:"parentObservationId,omitempty"`
	Name             string                 `json:"name"`
	StartTime        time.Time              `json:"startTime"`
	EndTime          *time.Time             `json:"endTime,omitempty"`
	Model            string                 `json:"model,omitempty"`
	ModelParameters  map[string]interface{} `json:"modelParameters,omitempty"`
	Input            interface{}            `json:"input,omitempty"`
	Output           interface{}            `json:"output,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	PromptTokens     int                    `json:"usage.promptTokens,omitempty"`
	CompletionTokens int                    `json:"usage.completionTokens,omitempty"`
	TotalTokens      int                    `json:"usage.totalTokens,omitempty"`
}

// Trace represents an active trace
type Trace struct {
	client    *Client
	ID        string
	Name      string
	StartTime time.Time
	UserID    string
	Metadata  map[string]interface{}
	Input     interface{}
}

// Span represents an active span within a trace
type Span struct {
	client    *Client
	ID        string
	TraceID   string
	ParentID  string
	Name      string
	StartTime time.Time
	Metadata  map[string]interface{}
	Input     interface{}
}

// Generation represents an LLM generation within a trace
type Generation struct {
	client    *Client
	ID        string
	TraceID   string
	ParentID  string
	Name      string
	StartTime time.Time
	Model     string
	Metadata  map[string]interface{}
	Input     interface{}
}

// Config holds the Langfuse client configuration
type Config struct {
	Host      string
	PublicKey string
	SecretKey string
	Enabled   bool
}

var (
	defaultClient *Client
	once          sync.Once
)

// NewClient creates a new Langfuse client
func NewClient(config Config) *Client {
	client := &Client{
		host:      config.Host,
		publicKey: config.PublicKey,
		secretKey: config.SecretKey,
		enabled:   config.Enabled,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		events: make([]Event, 0),
	}

	// Start background flush goroutine
	if config.Enabled {
		go client.backgroundFlush()
	}

	return client
}

// Initialize sets up the default Langfuse client
func Initialize(config Config) {
	once.Do(func() {
		defaultClient = NewClient(config)
		if config.Enabled {
			log.Info("Langfuse tracing initialized")
		} else {
			log.Info("Langfuse tracing disabled")
		}
	})
}

// GetClient returns the default Langfuse client
func GetClient() *Client {
	return defaultClient
}

// IsEnabled returns whether Langfuse tracing is enabled
func (c *Client) IsEnabled() bool {
	return c != nil && c.enabled
}

// CreateTrace creates a new trace
func (c *Client) CreateTrace(name string, userID string, metadata map[string]interface{}, input interface{}) *Trace {
	if !c.IsEnabled() {
		return &Trace{client: c}
	}

	trace := &Trace{
		client:    c,
		ID:        uuid.New().String(),
		Name:      name,
		StartTime: time.Now(),
		UserID:    userID,
		Metadata:  metadata,
		Input:     input,
	}

	body := map[string]interface{}{
		"id":        trace.ID,
		"name":      name,
		"timestamp": trace.StartTime,
	}
	if userID != "" {
		body["userId"] = userID
	}
	if metadata != nil {
		body["metadata"] = metadata
	}
	if input != nil {
		body["input"] = input
	}

	c.addEvent("trace-create", body)
	return trace
}

// CreateSpan creates a new span within this trace
func (t *Trace) CreateSpan(name string, metadata map[string]interface{}, input interface{}) *Span {
	if !t.client.IsEnabled() {
		return &Span{client: t.client, TraceID: t.ID}
	}

	span := &Span{
		client:    t.client,
		ID:        uuid.New().String(),
		TraceID:   t.ID,
		Name:      name,
		StartTime: time.Now(),
		Metadata:  metadata,
		Input:     input,
	}

	body := map[string]interface{}{
		"id":        span.ID,
		"traceId":   t.ID,
		"name":      name,
		"startTime": span.StartTime,
	}
	if metadata != nil {
		body["metadata"] = metadata
	}
	if input != nil {
		body["input"] = input
	}

	t.client.addEvent("span-create", body)
	return span
}

// CreateGeneration creates a new LLM generation within this trace
func (t *Trace) CreateGeneration(name string, model string, input interface{}, metadata map[string]interface{}) *Generation {
	if !t.client.IsEnabled() {
		return &Generation{client: t.client, TraceID: t.ID}
	}

	gen := &Generation{
		client:    t.client,
		ID:        uuid.New().String(),
		TraceID:   t.ID,
		Name:      name,
		StartTime: time.Now(),
		Model:     model,
		Metadata:  metadata,
		Input:     input,
	}

	body := map[string]interface{}{
		"id":        gen.ID,
		"traceId":   t.ID,
		"name":      name,
		"startTime": gen.StartTime,
	}
	if model != "" {
		body["model"] = model
	}
	if metadata != nil {
		body["metadata"] = metadata
	}
	if input != nil {
		body["input"] = input
	}

	t.client.addEvent("generation-create", body)
	return gen
}

// End finalizes the trace with output
func (t *Trace) End(output interface{}) {
	if !t.client.IsEnabled() {
		return
	}

	body := map[string]interface{}{
		"id":     t.ID,
		"output": output,
	}

	t.client.addEvent("trace-create", body)
}

// End finalizes the span with output
func (s *Span) End(output interface{}, status string) {
	if !s.client.IsEnabled() {
		return
	}

	endTime := time.Now()
	body := map[string]interface{}{
		"id":      s.ID,
		"traceId": s.TraceID,
		"endTime": endTime,
	}
	if output != nil {
		body["output"] = output
	}
	if status != "" {
		body["statusMessage"] = status
	}

	s.client.addEvent("span-update", body)
}

// End finalizes the generation with output and usage
func (g *Generation) End(output interface{}, promptTokens, completionTokens int) {
	if !g.client.IsEnabled() {
		return
	}

	endTime := time.Now()
	body := map[string]interface{}{
		"id":      g.ID,
		"traceId": g.TraceID,
		"endTime": endTime,
	}
	if output != nil {
		body["output"] = output
	}
	if promptTokens > 0 || completionTokens > 0 {
		body["usage"] = map[string]interface{}{
			"promptTokens":     promptTokens,
			"completionTokens": completionTokens,
			"totalTokens":      promptTokens + completionTokens,
		}
	}

	g.client.addEvent("generation-update", body)
}

// addEvent adds an event to the queue
func (c *Client) addEvent(eventType string, body map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	event := Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Timestamp: time.Now(),
		Body:      body,
	}

	c.events = append(c.events, event)

	// Flush if we have accumulated enough events
	if len(c.events) >= 10 {
		go c.flush()
	}
}

// backgroundFlush periodically flushes events
func (c *Client) backgroundFlush() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.flush()
	}
}

// flush sends all pending events to Langfuse
func (c *Client) flush() {
	c.mu.Lock()
	if len(c.events) == 0 {
		c.mu.Unlock()
		return
	}
	events := c.events
	c.events = make([]Event, 0)
	c.mu.Unlock()

	c.sendEvents(events)
}

// Flush manually flushes all pending events
func (c *Client) Flush() {
	if c != nil && c.enabled {
		c.flush()
	}
}

// sendEvents sends events to Langfuse API
func (c *Client) sendEvents(events []Event) {
	if len(events) == 0 {
		return
	}

	payload := map[string]interface{}{
		"batch": events,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Errorf("Failed to marshal Langfuse events: %v", err)
		return
	}

	req, err := http.NewRequest("POST", c.host+"/api/public/ingestion", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Errorf("Failed to create Langfuse request: %v", err)
		return
	}

	// Set Basic Auth header
	auth := base64.StdEncoding.EncodeToString([]byte(c.publicKey + ":" + c.secretKey))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Errorf("Failed to send events to Langfuse: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Errorf("Langfuse API returned error status: %d", resp.StatusCode)
	} else {
		log.Debugf("Successfully sent %d events to Langfuse", len(events))
	}
}

// Score adds a score to a trace
func (c *Client) Score(traceID string, name string, value float64, comment string) {
	if !c.IsEnabled() {
		return
	}

	body := map[string]interface{}{
		"id":      uuid.New().String(),
		"traceId": traceID,
		"name":    name,
		"value":   value,
	}
	if comment != "" {
		body["comment"] = comment
	}

	c.addEvent("score-create", body)
}

// Helper function to create trace from default client
func CreateTrace(name string, userID string, metadata map[string]interface{}, input interface{}) *Trace {
	if defaultClient == nil {
		return &Trace{}
	}
	return defaultClient.CreateTrace(name, userID, metadata, input)
}

// Helper function to flush default client
func Flush() {
	if defaultClient != nil {
		defaultClient.Flush()
	}
}

// TraceMiddlewareContext holds trace context for middleware
type TraceMiddlewareContext struct {
	TraceID string
	Trace   *Trace
}

// ContextKey is the type for context keys
type ContextKey string

const (
	// TraceContextKey is the context key for Langfuse trace
	TraceContextKey ContextKey = "langfuse_trace"
)

// FormatError formats an error for Langfuse output
func FormatError(err error) map[string]interface{} {
	if err == nil {
		return nil
	}
	return map[string]interface{}{
		"error":   true,
		"message": err.Error(),
	}
}

// FormatGraphQLOperation formats GraphQL operation for tracing
func FormatGraphQLOperation(operationName string, variables interface{}) map[string]interface{} {
	return map[string]interface{}{
		"operation": operationName,
		"variables": variables,
		"type":      "graphql",
	}
}
