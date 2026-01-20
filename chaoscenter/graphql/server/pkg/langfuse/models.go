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
	Input     interface{}            `json:"input,omitempty"`
	Output    interface{}            `json:"output,omitempty"`
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
	Public    bool                   `json:"public,omitempty"`
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
	TotalCount int     `json:"meta"`
}

// Span represents an agent action within a trace
type Span struct {
	ID        string                 `json:"id"`
	TraceID   string                 `json:"traceId"`
	ParentID  string                 `json:"parentObservationId,omitempty"`
	Name      string                 `json:"name"`
	StartTime time.Time              `json:"startTime"`
	EndTime   time.Time              `json:"endTime,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Input     interface{}            `json:"input,omitempty"`
	Output    interface{}            `json:"output,omitempty"`
	Level     string                 `json:"level,omitempty"` // DEBUG, DEFAULT, WARNING, ERROR
	StatusMsg string                 `json:"statusMessage,omitempty"`
}

// SpanPayload is the request body for creating spans
type SpanPayload struct {
	TraceID   string                 `json:"traceId"`
	ParentID  string                 `json:"parentObservationId,omitempty"`
	Name      string                 `json:"name"`
	StartTime time.Time              `json:"startTime"`
	EndTime   time.Time              `json:"endTime,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Input     interface{}            `json:"input,omitempty"`
	Output    interface{}            `json:"output,omitempty"`
	Level     string                 `json:"level,omitempty"`
	StatusMsg string                 `json:"statusMessage,omitempty"`
}

// Generation represents an LLM generation within a trace
type Generation struct {
	ID               string                 `json:"id"`
	TraceID          string                 `json:"traceId"`
	ParentID         string                 `json:"parentObservationId,omitempty"`
	Name             string                 `json:"name"`
	StartTime        time.Time              `json:"startTime"`
	EndTime          time.Time              `json:"endTime,omitempty"`
	Model            string                 `json:"model,omitempty"`
	ModelParameters  map[string]interface{} `json:"modelParameters,omitempty"`
	Input            interface{}            `json:"input,omitempty"`
	Output           interface{}            `json:"output,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	Usage            *Usage                 `json:"usage,omitempty"`
	Level            string                 `json:"level,omitempty"`
	StatusMsg        string                 `json:"statusMessage,omitempty"`
	CompletionStart  time.Time              `json:"completionStartTime,omitempty"`
}

// GenerationPayload is the request body for creating generations
type GenerationPayload struct {
	TraceID          string                 `json:"traceId"`
	ParentID         string                 `json:"parentObservationId,omitempty"`
	Name             string                 `json:"name"`
	StartTime        time.Time              `json:"startTime"`
	EndTime          time.Time              `json:"endTime,omitempty"`
	Model            string                 `json:"model,omitempty"`
	ModelParameters  map[string]interface{} `json:"modelParameters,omitempty"`
	Input            interface{}            `json:"input,omitempty"`
	Output           interface{}            `json:"output,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	Usage            *Usage                 `json:"usage,omitempty"`
	Level            string                 `json:"level,omitempty"`
	StatusMsg        string                 `json:"statusMessage,omitempty"`
	CompletionStart  time.Time              `json:"completionStartTime,omitempty"`
}

// Usage represents token usage for LLM generations
type Usage struct {
	PromptTokens     int `json:"promptTokens,omitempty"`
	CompletionTokens int `json:"completionTokens,omitempty"`
	TotalTokens      int `json:"totalTokens,omitempty"`
}

// Score represents an evaluation metric
type Score struct {
	ID          string  `json:"id"`
	TraceID     string  `json:"traceId"`
	Name        string  `json:"name"`
	Value       float64 `json:"value"`
	Comment     string  `json:"comment,omitempty"`
	DataType    string  `json:"dataType,omitempty"` // "NUMERIC" or "CATEGORICAL"
	Source      string  `json:"source,omitempty"`   // "API", "EVAL", "ANNOTATION"
	ConfigID    string  `json:"configId,omitempty"`
	ObservationID string `json:"observationId,omitempty"`
}

// ScorePayload is the request body for creating scores
type ScorePayload struct {
	TraceID       string  `json:"traceId"`
	Name          string  `json:"name"`
	Value         float64 `json:"value"`
	Comment       string  `json:"comment,omitempty"`
	DataType      string  `json:"dataType,omitempty"`
	ObservationID string  `json:"observationId,omitempty"`
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
	Name      string    `json:"name,omitempty"`
	FromDate  time.Time `json:"fromDate,omitempty"`
	ToDate    time.Time `json:"toDate,omitempty"`
}

// MetricsResponse contains aggregated metrics
type MetricsResponse struct {
	TotalTraces  int                `json:"totalTraces"`
	TotalScores  int                `json:"totalScores"`
	AvgScores    map[string]float64 `json:"avgScores"`
	ScoresByName map[string][]Score `json:"scoresByName"`
}

// IngestionEvent represents a single event in batch ingestion
type IngestionEvent struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"` // "trace-create", "span-create", "generation-create", "score-create"
	Timestamp time.Time   `json:"timestamp"`
	Body      interface{} `json:"body"`
}

// IngestionRequest is the batch ingestion request
type IngestionRequest struct {
	Batch    []IngestionEvent `json:"batch"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// IngestionResponse is the batch ingestion response
type IngestionResponse struct {
	Successes []string `json:"successes"`
	Errors    []struct {
		ID      string `json:"id"`
		Status  int    `json:"status"`
		Message string `json:"message"`
		Error   string `json:"error"`
	} `json:"errors"`
}
