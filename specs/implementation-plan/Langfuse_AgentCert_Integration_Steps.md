# Langfuse + AgentCert Integration - Step-by-Step Implementation Guide

**Document Version**: 1.0  
**Author**: GitHub Copilot (Architect)  
**Date**: January 20, 2026  
**Related Design**: [Langfuse_AgentCert_Integration_Design.md](./Langfuse_AgentCert_Integration_Design.md)

---

## Prerequisites

Before starting the implementation, ensure you have:

- [ ] Access to AgentCert repository (`feature/langfuse-agentcert-integration` branch)
- [ ] Langfuse account (Cloud or self-hosted instance)
- [ ] Go 1.21+ installed
- [ ] Node.js 18+ installed
- [ ] MongoDB running locally or accessible
- [ ] Kubernetes cluster access (for deployment testing)

---

## Step 1: Set Up Langfuse Environment

### 1.1 Create Langfuse Account

**Option A: Langfuse Cloud (Recommended for development)**

1. Go to https://cloud.langfuse.com
2. Sign up with GitHub/Google or email
3. Create a new project named `agentcert-dev`
4. Navigate to **Settings → API Keys**
5. Generate a new API key pair:
   - Copy the **Secret Key** (starts with `sk-lf-`)
   - Copy the **Public Key** (starts with `pk-lf-`)

**Option B: Self-Hosted Langfuse**

```bash
# Clone Langfuse repository
git clone https://github.com/langfuse/langfuse.git
cd langfuse

# Start with Docker Compose
docker-compose up -d

# Access at http://localhost:3000
# Create project and API keys via UI
```

### 1.2 Configure Environment Variables

Create a `.env.langfuse` file in the repository root (add to `.gitignore`):

```bash
# Langfuse Configuration
LANGFUSE_ENABLED=true
LANGFUSE_BASE_URL=https://cloud.langfuse.com
LANGFUSE_PUBLIC_KEY=pk-lf-your-public-key
LANGFUSE_SECRET_KEY=sk-lf-your-secret-key
LANGFUSE_PROJECT_ID=your-project-id
```

---

## Step 2: Create Langfuse Go Client Package

### 2.1 Create Package Structure

```bash
# Navigate to GraphQL server package directory
cd chaoscenter/graphql/server/pkg

# Create langfuse package
mkdir -p langfuse
```

### 2.2 Create Configuration Module

Create `pkg/langfuse/config.go`:

```go
package langfuse

import (
	"errors"
	"os"
)

// Config holds Langfuse client configuration
type Config struct {
	Enabled   bool
	BaseURL   string
	PublicKey string
	SecretKey string
	ProjectID string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	enabled := os.Getenv("LANGFUSE_ENABLED") == "true"
	
	if !enabled {
		return &Config{Enabled: false}, nil
	}
	
	baseURL := os.Getenv("LANGFUSE_BASE_URL")
	if baseURL == "" {
		baseURL = "https://cloud.langfuse.com"
	}
	
	secretKey := os.Getenv("LANGFUSE_SECRET_KEY")
	if secretKey == "" {
		return nil, errors.New("LANGFUSE_SECRET_KEY is required when Langfuse is enabled")
	}
	
	publicKey := os.Getenv("LANGFUSE_PUBLIC_KEY")
	if publicKey == "" {
		return nil, errors.New("LANGFUSE_PUBLIC_KEY is required when Langfuse is enabled")
	}
	
	return &Config{
		Enabled:   true,
		BaseURL:   baseURL,
		PublicKey: publicKey,
		SecretKey: secretKey,
		ProjectID: os.Getenv("LANGFUSE_PROJECT_ID"),
	}, nil
}
```

### 2.3 Create Data Models

Create `pkg/langfuse/models.go`:

```go
package langfuse

import "time"

// User represents an agent in Langfuse
type User struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// UserPayload is the request body for creating/updating users
type UserPayload struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Trace represents a benchmark execution trace
type Trace struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	UserID    string                 `json:"userId,omitempty"`
	SessionID string                 `json:"sessionId,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Tags      []string               `json:"tags,omitempty"`
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
}

// TracePayload is the request body for creating traces
type TracePayload struct {
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name"`
	UserID    string                 `json:"userId,omitempty"`
	SessionID string                 `json:"sessionId,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Tags      []string               `json:"tags,omitempty"`
	Input     interface{}            `json:"input,omitempty"`
	Output    interface{}            `json:"output,omitempty"`
}

// TraceFilter for querying traces
type TraceFilter struct {
	UserID    string    `json:"userId,omitempty"`
	SessionID string    `json:"sessionId,omitempty"`
	Name      string    `json:"name,omitempty"`
	FromDate  time.Time `json:"fromDate,omitempty"`
	ToDate    time.Time `json:"toDate,omitempty"`
	Limit     int       `json:"limit,omitempty"`
	Offset    int       `json:"offset,omitempty"`
}

// TraceList is the paginated list of traces
type TraceList struct {
	Data       []Trace `json:"data"`
	TotalCount int     `json:"totalCount"`
}

// Span represents an agent action within a trace
type Span struct {
	ID        string                 `json:"id"`
	TraceID   string                 `json:"traceId"`
	Name      string                 `json:"name"`
	StartTime time.Time              `json:"startTime"`
	EndTime   time.Time              `json:"endTime,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Input     interface{}            `json:"input,omitempty"`
	Output    interface{}            `json:"output,omitempty"`
}

// SpanPayload is the request body for creating spans
type SpanPayload struct {
	TraceID   string                 `json:"traceId"`
	Name      string                 `json:"name"`
	StartTime time.Time              `json:"startTime"`
	EndTime   time.Time              `json:"endTime,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Input     interface{}            `json:"input,omitempty"`
	Output    interface{}            `json:"output,omitempty"`
}

// Score represents an evaluation metric
type Score struct {
	ID       string  `json:"id"`
	TraceID  string  `json:"traceId"`
	Name     string  `json:"name"`
	Value    float64 `json:"value"`
	Comment  string  `json:"comment,omitempty"`
	DataType string  `json:"dataType,omitempty"` // "NUMERIC" or "CATEGORICAL"
}

// ScorePayload is the request body for creating scores
type ScorePayload struct {
	TraceID  string  `json:"traceId"`
	Name     string  `json:"name"`
	Value    float64 `json:"value"`
	Comment  string  `json:"comment,omitempty"`
	DataType string  `json:"dataType,omitempty"`
}

// Session represents a benchmark project run
type Session struct {
	ID        string                 `json:"id"`
	ProjectID string                 `json:"projectId,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"createdAt"`
}

// SessionPayload is the request body for creating sessions
type SessionPayload struct {
	ID        string                 `json:"id"`
	ProjectID string                 `json:"projectId,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// MetricsFilter for querying aggregated metrics
type MetricsFilter struct {
	UserID    string    `json:"userId,omitempty"`
	SessionID string    `json:"sessionId,omitempty"`
	FromDate  time.Time `json:"fromDate,omitempty"`
	ToDate    time.Time `json:"toDate,omitempty"`
}

// MetricsResponse contains aggregated metrics
type MetricsResponse struct {
	TotalTraces  int            `json:"totalTraces"`
	TotalScores  int            `json:"totalScores"`
	AvgScores    map[string]float64 `json:"avgScores"`
	ScoresByName map[string][]Score `json:"scoresByName"`
}
```

### 2.4 Create Error Types

Create `pkg/langfuse/errors.go`:

```go
package langfuse

import "fmt"

// LangfuseError represents an API error
type LangfuseError struct {
	StatusCode int
	Message    string
	Details    string
}

func (e *LangfuseError) Error() string {
	return fmt.Sprintf("langfuse error [%d]: %s - %s", e.StatusCode, e.Message, e.Details)
}

// IsRetryable returns true if the error is retryable
func (e *LangfuseError) IsRetryable() bool {
	return e.StatusCode >= 500 || e.StatusCode == 429
}

// ErrNotEnabled is returned when Langfuse is disabled
var ErrNotEnabled = fmt.Errorf("langfuse integration is not enabled")

// ErrNotFound is returned when a resource is not found
var ErrNotFound = fmt.Errorf("resource not found")
```

### 2.5 Create HTTP Client

Create `pkg/langfuse/client.go`:

```go
package langfuse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// Client is the Langfuse API client
type Client struct {
	config     *Config
	httpClient *http.Client
	logger     *logrus.Logger
}

// NewClient creates a new Langfuse client
func NewClient(config *Config, logger *logrus.Logger) *Client {
	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// IsEnabled returns true if Langfuse integration is enabled
func (c *Client) IsEnabled() bool {
	return c.config != nil && c.config.Enabled
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

	// Retry logic
	maxRetries := 3
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * time.Second
			time.Sleep(backoff)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			c.logger.Warnf("Langfuse request failed (attempt %d/%d): %v", attempt+1, maxRetries, err)
			continue
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
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
		c.logger.Warnf("Langfuse request returned %d (attempt %d/%d)", resp.StatusCode, attempt+1, maxRetries)
	}

	return nil, fmt.Errorf("langfuse request failed after %d attempts: %w", maxRetries, lastErr)
}
```

### 2.6 Create User Operations

Create `pkg/langfuse/users.go`:

```go
package langfuse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// CreateOrUpdateUser creates or updates a user (agent) in Langfuse
func (c *Client) CreateOrUpdateUser(ctx context.Context, user UserPayload) error {
	_, err := c.doRequest(ctx, http.MethodPost, "/api/public/users", user)
	if err != nil {
		return fmt.Errorf("failed to create/update user: %w", err)
	}
	return nil
}

// GetUser retrieves a user by ID
func (c *Client) GetUser(ctx context.Context, userID string) (*User, error) {
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
```

### 2.7 Create Trace Operations

Create `pkg/langfuse/traces.go`:

```go
package langfuse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// CreateTrace creates a new trace
func (c *Client) CreateTrace(ctx context.Context, trace TracePayload) (*Trace, error) {
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

// GetTrace retrieves a trace by ID
func (c *Client) GetTrace(ctx context.Context, traceID string) (*Trace, error) {
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
	if filter.Limit > 0 {
		params.Set("limit", strconv.Itoa(filter.Limit))
	}
	if filter.Offset > 0 {
		params.Set("offset", strconv.Itoa(filter.Offset))
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
```

### 2.8 Create Score Operations

Create `pkg/langfuse/scores.go`:

```go
package langfuse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// CreateScore creates a new score (evaluation metric)
func (c *Client) CreateScore(ctx context.Context, score ScorePayload) error {
	_, err := c.doRequest(ctx, http.MethodPost, "/api/public/scores", score)
	if err != nil {
		return fmt.Errorf("failed to create score: %w", err)
	}
	return nil
}

// GetScores retrieves scores for a trace
func (c *Client) GetScores(ctx context.Context, traceID string) ([]Score, error) {
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
```

### 2.9 Create Sessions Operations

Create `pkg/langfuse/sessions.go`:

```go
package langfuse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// CreateSession creates a new session (benchmark project run)
func (c *Client) CreateSession(ctx context.Context, session SessionPayload) error {
	_, err := c.doRequest(ctx, http.MethodPost, "/api/public/sessions", session)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

// GetSession retrieves a session by ID
func (c *Client) GetSession(ctx context.Context, sessionID string) (*Session, error) {
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
```

---

## Step 3: Integrate with Agent Registry Service

### 3.1 Create Langfuse Sync Module

Create `pkg/agent_registry/langfuse_sync.go`:

```go
package agent_registry

import (
	"context"
	"time"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/langfuse"
	"github.com/sirupsen/logrus"
)

// LangfuseSync handles synchronization of agent data to Langfuse
type LangfuseSync struct {
	client *langfuse.Client
	logger *logrus.Logger
}

// NewLangfuseSync creates a new LangfuseSync instance
func NewLangfuseSync(client *langfuse.Client, logger *logrus.Logger) *LangfuseSync {
	return &LangfuseSync{
		client: client,
		logger: logger,
	}
}

// SyncAgent synchronizes agent data to Langfuse
func (s *LangfuseSync) SyncAgent(ctx context.Context, agent *Agent) error {
	if !s.client.IsEnabled() {
		return nil // Silently skip if Langfuse is not enabled
	}

	user := langfuse.UserPayload{
		ID:   agent.AgentID,
		Name: agent.Name,
		Metadata: map[string]interface{}{
			"vendor":       agent.Vendor,
			"version":      agent.Version,
			"namespace":    agent.Namespace,
			"image":        agent.ContainerImage,
			"status":       agent.Status,
			"capabilities": agent.Capabilities,
			"createdAt":    agent.CreatedAt.Format(time.RFC3339),
			"updatedAt":    agent.UpdatedAt.Format(time.RFC3339),
		},
	}

	if err := s.client.CreateOrUpdateUser(ctx, user); err != nil {
		s.logger.Warnf("Failed to sync agent %s to Langfuse: %v", agent.AgentID, err)
		return err
	}

	s.logger.Infof("Successfully synced agent %s to Langfuse", agent.AgentID)
	return nil
}

// SyncAgentAsync synchronizes agent data to Langfuse asynchronously
func (s *LangfuseSync) SyncAgentAsync(agent *Agent) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.SyncAgent(ctx, agent); err != nil {
			s.logger.Errorf("Async sync to Langfuse failed for agent %s: %v", agent.AgentID, err)
		}
	}()
}

// SyncAgentDeletion marks an agent as deleted in Langfuse
func (s *LangfuseSync) SyncAgentDeletion(ctx context.Context, agentID string) error {
	if !s.client.IsEnabled() {
		return nil
	}

	user := langfuse.UserPayload{
		ID: agentID,
		Metadata: map[string]interface{}{
			"status":    "DELETED",
			"deletedAt": time.Now().Format(time.RFC3339),
		},
	}

	return s.client.CreateOrUpdateUser(ctx, user)
}
```

### 3.2 Update Agent Registry Service

Modify the Agent Registry service to use Langfuse sync:

```go
// In pkg/agent_registry/service.go

type service struct {
	operator     *Operator
	langfuseSync *LangfuseSync
	logger       *logrus.Logger
}

func NewService(operator *Operator, langfuseClient *langfuse.Client, logger *logrus.Logger) Service {
	return &service{
		operator:     operator,
		langfuseSync: NewLangfuseSync(langfuseClient, logger),
		logger:       logger,
	}
}

func (s *service) RegisterAgent(ctx context.Context, input RegisterAgentInput) (*Agent, error) {
	// ... existing validation and creation logic ...

	// Create agent in MongoDB
	agent, err := s.operator.CreateAgent(ctx, agent)
	if err != nil {
		return nil, err
	}

	// Sync to Langfuse asynchronously
	s.langfuseSync.SyncAgentAsync(agent)

	return agent, nil
}

func (s *service) UpdateAgent(ctx context.Context, agentID string, input UpdateAgentInput) (*Agent, error) {
	// ... existing update logic ...

	// Sync to Langfuse asynchronously
	s.langfuseSync.SyncAgentAsync(updatedAgent)

	return updatedAgent, nil
}

func (s *service) DeleteAgent(ctx context.Context, agentID string) error {
	// ... existing deletion logic ...

	// Mark as deleted in Langfuse
	go s.langfuseSync.SyncAgentDeletion(context.Background(), agentID)

	return nil
}
```

---

## Step 4: Create GraphQL Resolvers for Langfuse Data

### 4.1 Add GraphQL Types

Add to your GraphQL schema:

```graphql
# In graph/schema/langfuse.graphql

type TraceDetails {
    id: ID!
    name: String!
    userId: String
    sessionId: String
    metadata: Map
    tags: [String!]
    createdAt: Time!
    spans: [SpanDetails!]
    scores: [ScoreDetails!]
}

type SpanDetails {
    id: ID!
    name: String!
    startTime: Time!
    endTime: Time
    input: Map
    output: Map
}

type ScoreDetails {
    id: ID!
    name: String!
    value: Float!
    comment: String
}

type BenchmarkMetrics {
    avgTTD: Float!
    avgTTR: Float!
    successRate: Float!
    totalRuns: Int!
    recentTraces: [TraceDetails!]!
}

type AgentComparison {
    agents: [AgentMetrics!]!
}

type AgentMetrics {
    agentId: ID!
    agentName: String!
    avgTTD: Float!
    avgTTR: Float!
    successRate: Float!
    totalRuns: Int!
}

input TimeRangeInput {
    from: Time!
    to: Time!
}

extend type Query {
    # Get benchmark metrics from Langfuse
    getBenchmarkMetrics(
        projectId: ID!
        agentId: ID
        scenarioId: ID
        timeRange: TimeRangeInput
    ): BenchmarkMetrics!

    # Compare multiple agents
    compareAgents(
        projectId: ID!
        agentIds: [ID!]!
    ): AgentComparison!

    # Get agent traces
    getAgentTraces(
        agentId: ID!
        limit: Int
        offset: Int
    ): [TraceDetails!]!

    # Get trace details
    getTraceDetails(traceId: ID!): TraceDetails!
}
```

### 4.2 Implement Resolvers

Create `graph/resolver/langfuse_resolver.go`:

```go
package resolver

import (
	"context"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/langfuse"
)

func (r *queryResolver) GetBenchmarkMetrics(
	ctx context.Context,
	projectID string,
	agentID *string,
	scenarioID *string,
	timeRange *model.TimeRangeInput,
) (*model.BenchmarkMetrics, error) {
	filter := langfuse.TraceFilter{
		Limit: 100,
	}
	if agentID != nil {
		filter.UserID = *agentID
	}
	if timeRange != nil {
		filter.FromDate = timeRange.From
		filter.ToDate = timeRange.To
	}

	traces, err := r.langfuseClient.ListTraces(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Calculate metrics from traces
	var totalTTD, totalTTR float64
	var successCount, totalCount int

	for _, trace := range traces.Data {
		scores, err := r.langfuseClient.GetScores(ctx, trace.ID)
		if err != nil {
			continue
		}

		totalCount++
		for _, score := range scores {
			switch score.Name {
			case "time_to_detect":
				totalTTD += score.Value
			case "time_to_remediate":
				totalTTR += score.Value
			case "success":
				if score.Value == 1 {
					successCount++
				}
			}
		}
	}

	avgTTD := 0.0
	avgTTR := 0.0
	successRate := 0.0
	if totalCount > 0 {
		avgTTD = totalTTD / float64(totalCount)
		avgTTR = totalTTR / float64(totalCount)
		successRate = float64(successCount) / float64(totalCount) * 100
	}

	return &model.BenchmarkMetrics{
		AvgTTD:      avgTTD,
		AvgTTR:      avgTTR,
		SuccessRate: successRate,
		TotalRuns:   totalCount,
	}, nil
}

func (r *queryResolver) GetAgentTraces(
	ctx context.Context,
	agentID string,
	limit *int,
	offset *int,
) ([]*model.TraceDetails, error) {
	filter := langfuse.TraceFilter{
		UserID: agentID,
	}
	if limit != nil {
		filter.Limit = *limit
	}
	if offset != nil {
		filter.Offset = *offset
	}

	traces, err := r.langfuseClient.ListTraces(ctx, filter)
	if err != nil {
		return nil, err
	}

	var result []*model.TraceDetails
	for _, trace := range traces.Data {
		result = append(result, &model.TraceDetails{
			ID:        trace.ID,
			Name:      trace.Name,
			UserID:    &trace.UserID,
			SessionID: &trace.SessionID,
			CreatedAt: trace.CreatedAt,
		})
	}

	return result, nil
}

func (r *queryResolver) GetTraceDetails(ctx context.Context, traceID string) (*model.TraceDetails, error) {
	trace, err := r.langfuseClient.GetTrace(ctx, traceID)
	if err != nil {
		return nil, err
	}

	scores, err := r.langfuseClient.GetScores(ctx, traceID)
	if err != nil {
		return nil, err
	}

	var scoreDetails []*model.ScoreDetails
	for _, score := range scores {
		scoreDetails = append(scoreDetails, &model.ScoreDetails{
			ID:      score.ID,
			Name:    score.Name,
			Value:   score.Value,
			Comment: &score.Comment,
		})
	}

	return &model.TraceDetails{
		ID:        trace.ID,
		Name:      trace.Name,
		UserID:    &trace.UserID,
		SessionID: &trace.SessionID,
		CreatedAt: trace.CreatedAt,
		Scores:    scoreDetails,
	}, nil
}
```

---

## Step 5: Create Frontend TypeScript Client

### 5.1 Create Langfuse API Client

Create `src/api/langfuse/LangfuseClient.ts`:

```typescript
import { getConfig } from '@/config';

interface TraceDetails {
  id: string;
  name: string;
  userId?: string;
  sessionId?: string;
  metadata?: Record<string, unknown>;
  createdAt: string;
  scores?: ScoreDetails[];
}

interface ScoreDetails {
  id: string;
  name: string;
  value: number;
  comment?: string;
}

interface BenchmarkMetrics {
  avgTTD: number;
  avgTTR: number;
  successRate: number;
  totalRuns: number;
  recentTraces: TraceDetails[];
}

interface AgentComparison {
  agents: AgentMetrics[];
}

interface AgentMetrics {
  agentId: string;
  agentName: string;
  avgTTD: number;
  avgTTR: number;
  successRate: number;
  totalRuns: number;
}

export class LangfuseClient {
  private baseUrl: string;

  constructor() {
    this.baseUrl = getConfig().apiBaseUrl;
  }

  async getBenchmarkMetrics(
    projectId: string,
    agentId?: string,
    timeRange?: { from: Date; to: Date }
  ): Promise<BenchmarkMetrics> {
    const response = await this.graphqlQuery<{ getBenchmarkMetrics: BenchmarkMetrics }>(
      `query GetBenchmarkMetrics($projectId: ID!, $agentId: ID, $timeRange: TimeRangeInput) {
        getBenchmarkMetrics(projectId: $projectId, agentId: $agentId, timeRange: $timeRange) {
          avgTTD
          avgTTR
          successRate
          totalRuns
          recentTraces {
            id
            name
            createdAt
          }
        }
      }`,
      { projectId, agentId, timeRange }
    );
    return response.getBenchmarkMetrics;
  }

  async getAgentTraces(agentId: string, limit = 50, offset = 0): Promise<TraceDetails[]> {
    const response = await this.graphqlQuery<{ getAgentTraces: TraceDetails[] }>(
      `query GetAgentTraces($agentId: ID!, $limit: Int, $offset: Int) {
        getAgentTraces(agentId: $agentId, limit: $limit, offset: $offset) {
          id
          name
          userId
          sessionId
          createdAt
          scores {
            id
            name
            value
            comment
          }
        }
      }`,
      { agentId, limit, offset }
    );
    return response.getAgentTraces;
  }

  async compareAgents(projectId: string, agentIds: string[]): Promise<AgentComparison> {
    const response = await this.graphqlQuery<{ compareAgents: AgentComparison }>(
      `query CompareAgents($projectId: ID!, $agentIds: [ID!]!) {
        compareAgents(projectId: $projectId, agentIds: $agentIds) {
          agents {
            agentId
            agentName
            avgTTD
            avgTTR
            successRate
            totalRuns
          }
        }
      }`,
      { projectId, agentIds }
    );
    return response.compareAgents;
  }

  private async graphqlQuery<T>(query: string, variables: Record<string, unknown>): Promise<T> {
    const response = await fetch(`${this.baseUrl}/api/query`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include',
      body: JSON.stringify({ query, variables }),
    });

    if (!response.ok) {
      throw new Error(`GraphQL request failed: ${response.statusText}`);
    }

    const result = await response.json();
    if (result.errors) {
      throw new Error(`GraphQL errors: ${JSON.stringify(result.errors)}`);
    }

    return result.data;
  }
}

export const langfuseClient = new LangfuseClient();
```

### 5.2 Create React Hooks

Create `src/api/langfuse/hooks/useMetrics.ts`:

```typescript
import { useState, useEffect, useCallback } from 'react';
import { langfuseClient } from '../LangfuseClient';

interface BenchmarkMetrics {
  avgTTD: number;
  avgTTR: number;
  successRate: number;
  totalRuns: number;
}

interface UseMetricsResult {
  metrics: BenchmarkMetrics | null;
  loading: boolean;
  error: Error | null;
  refetch: () => void;
}

export function useMetrics(
  projectId: string,
  agentId?: string,
  pollInterval?: number
): UseMetricsResult {
  const [metrics, setMetrics] = useState<BenchmarkMetrics | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const fetchMetrics = useCallback(async () => {
    try {
      setLoading(true);
      const data = await langfuseClient.getBenchmarkMetrics(projectId, agentId);
      setMetrics(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch metrics'));
    } finally {
      setLoading(false);
    }
  }, [projectId, agentId]);

  useEffect(() => {
    fetchMetrics();

    if (pollInterval && pollInterval > 0) {
      const interval = setInterval(fetchMetrics, pollInterval);
      return () => clearInterval(interval);
    }
  }, [fetchMetrics, pollInterval]);

  return { metrics, loading, error, refetch: fetchMetrics };
}
```

---

## Step 6: Testing

### 6.1 Unit Tests for Go Client

Create `pkg/langfuse/langfuse_test.go`:

```go
package langfuse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestCreateOrUpdateUser(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/public/users", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "agent-123", "name": "Test Agent"}`))
	}))
	defer server.Close()

	config := &Config{
		Enabled:   true,
		BaseURL:   server.URL,
		PublicKey: "test-public-key",
		SecretKey: "test-secret-key",
	}

	client := NewClient(config, logrus.New())

	err := client.CreateOrUpdateUser(context.Background(), UserPayload{
		ID:   "agent-123",
		Name: "Test Agent",
		Metadata: map[string]interface{}{
			"version": "1.0.0",
		},
	})

	assert.NoError(t, err)
}

func TestListTraces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/api/public/traces")
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": [
				{"id": "trace-1", "name": "benchmark-run", "userId": "agent-123"}
			],
			"totalCount": 1
		}`))
	}))
	defer server.Close()

	config := &Config{
		Enabled:   true,
		BaseURL:   server.URL,
		PublicKey: "test-public-key",
		SecretKey: "test-secret-key",
	}

	client := NewClient(config, logrus.New())

	traces, err := client.ListTraces(context.Background(), TraceFilter{
		UserID: "agent-123",
		Limit:  10,
	})

	assert.NoError(t, err)
	assert.Len(t, traces.Data, 1)
	assert.Equal(t, "trace-1", traces.Data[0].ID)
}
```

### 6.2 Integration Tests

Create `pkg/langfuse/integration_test.go`:

```go
//go:build integration
// +build integration

package langfuse

import (
	"context"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLangfuseIntegration(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("LANGFUSE_INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test")
	}

	config, err := LoadConfig()
	require.NoError(t, err)
	require.True(t, config.Enabled, "Langfuse must be enabled for integration tests")

	client := NewClient(config, logrus.New())
	ctx := context.Background()

	// Test user creation
	t.Run("CreateUser", func(t *testing.T) {
		err := client.CreateOrUpdateUser(ctx, UserPayload{
			ID:   "test-agent-integration",
			Name: "Integration Test Agent",
			Metadata: map[string]interface{}{
				"version": "1.0.0",
				"vendor":  "AgentCert",
			},
		})
		assert.NoError(t, err)
	})

	// Test trace creation
	t.Run("CreateTrace", func(t *testing.T) {
		trace, err := client.CreateTrace(ctx, TracePayload{
			Name:   "integration-test-benchmark",
			UserID: "test-agent-integration",
			Metadata: map[string]interface{}{
				"scenario": "test-scenario",
			},
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, trace.ID)
	})
}
```

---

## Step 7: Deployment

### 7.1 Update Kubernetes Manifests

Create `manifests/langfuse-secret.yaml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: langfuse-credentials
  namespace: litmus
type: Opaque
stringData:
  LANGFUSE_ENABLED: "true"
  LANGFUSE_BASE_URL: "https://cloud.langfuse.com"
  LANGFUSE_PUBLIC_KEY: "${LANGFUSE_PUBLIC_KEY}"
  LANGFUSE_SECRET_KEY: "${LANGFUSE_SECRET_KEY}"
```

### 7.2 Update GraphQL Server Deployment

Add to the GraphQL server deployment:

```yaml
spec:
  containers:
    - name: graphql-server
      envFrom:
        - secretRef:
            name: langfuse-credentials
```

---

## Step 8: Verification Checklist

### 8.1 Backend Verification

- [ ] Langfuse Go client compiles without errors
- [ ] Unit tests pass: `go test ./pkg/langfuse/...`
- [ ] Agent registration creates user in Langfuse
- [ ] Agent update syncs to Langfuse
- [ ] GraphQL queries return Langfuse data

### 8.2 Frontend Verification

- [ ] TypeScript client compiles without errors
- [ ] React hooks fetch data correctly
- [ ] Dashboard displays metrics from Langfuse
- [ ] Trace explorer shows trace details

### 8.3 End-to-End Verification

- [ ] Register agent → Appears in Langfuse users
- [ ] Run benchmark → Creates trace in Langfuse
- [ ] Evaluation metrics → Stored as scores in Langfuse
- [ ] Dashboard → Shows real-time metrics from Langfuse

---

## Troubleshooting

### Common Issues

| Issue | Solution |
|-------|----------|
| Authentication errors | Verify API keys are correct and have proper permissions |
| Network timeouts | Check firewall rules, increase timeout settings |
| Missing traces | Verify NAT is configured with Langfuse SDK |
| Sync failures | Check logs for error details, verify Langfuse is accessible |

### Useful Commands

```bash
# Test Langfuse connectivity
curl -X GET "https://cloud.langfuse.com/api/public/traces" \
  -H "Authorization: Bearer <public-key>:<secret-key>"

# Run Go tests
cd chaoscenter/graphql/server
go test -v ./pkg/langfuse/...

# Run integration tests
LANGFUSE_INTEGRATION_TEST=true go test -v -tags=integration ./pkg/langfuse/...
```

---

## Summary

This guide provides step-by-step instructions for integrating Langfuse with AgentCert:

1. **Step 1**: Set up Langfuse environment and credentials
2. **Step 2**: Create Go client package for backend API calls
3. **Step 3**: Integrate with Agent Registry for automatic sync
4. **Step 4**: Create GraphQL resolvers for metrics and traces
5. **Step 5**: Create TypeScript client and React hooks for frontend
6. **Step 6**: Write unit and integration tests
7. **Step 7**: Deploy with Kubernetes manifests
8. **Step 8**: Verify end-to-end functionality

**Total Estimated Effort**: 250 hours (~7 weeks with 1 developer, ~4 weeks with 2 developers)

---

**Document End**
