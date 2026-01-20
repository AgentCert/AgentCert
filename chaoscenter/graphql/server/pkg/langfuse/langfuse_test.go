package langfuse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(server *httptest.Server) *Client {
	config := &Config{
		Enabled:   true,
		BaseURL:   server.URL,
		PublicKey: "test-public-key",
		SecretKey: "test-secret-key",
		ProjectID: "test-project",
	}
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	return NewClient(config, logger)
}

func TestLoadConfig(t *testing.T) {
	t.Run("disabled when LANGFUSE_ENABLED is not true", func(t *testing.T) {
		t.Setenv("LANGFUSE_ENABLED", "false")
		config, err := LoadConfig()
		require.NoError(t, err)
		assert.False(t, config.Enabled)
	})

	t.Run("returns error when secret key is missing", func(t *testing.T) {
		t.Setenv("LANGFUSE_ENABLED", "true")
		t.Setenv("LANGFUSE_SECRET_KEY", "")
		t.Setenv("LANGFUSE_PUBLIC_KEY", "pk-test")
		_, err := LoadConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "LANGFUSE_SECRET_KEY")
	})

	t.Run("returns error when public key is missing", func(t *testing.T) {
		t.Setenv("LANGFUSE_ENABLED", "true")
		t.Setenv("LANGFUSE_SECRET_KEY", "sk-test")
		t.Setenv("LANGFUSE_PUBLIC_KEY", "")
		_, err := LoadConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "LANGFUSE_PUBLIC_KEY")
	})

	t.Run("loads config successfully", func(t *testing.T) {
		t.Setenv("LANGFUSE_ENABLED", "true")
		t.Setenv("LANGFUSE_SECRET_KEY", "sk-test")
		t.Setenv("LANGFUSE_PUBLIC_KEY", "pk-test")
		t.Setenv("LANGFUSE_BASE_URL", "https://custom.langfuse.com")
		t.Setenv("LANGFUSE_PROJECT_ID", "my-project")

		config, err := LoadConfig()
		require.NoError(t, err)
		assert.True(t, config.Enabled)
		assert.Equal(t, "sk-test", config.SecretKey)
		assert.Equal(t, "pk-test", config.PublicKey)
		assert.Equal(t, "https://custom.langfuse.com", config.BaseURL)
		assert.Equal(t, "my-project", config.ProjectID)
	})

	t.Run("uses default base URL", func(t *testing.T) {
		t.Setenv("LANGFUSE_ENABLED", "true")
		t.Setenv("LANGFUSE_SECRET_KEY", "sk-test")
		t.Setenv("LANGFUSE_PUBLIC_KEY", "pk-test")
		t.Setenv("LANGFUSE_BASE_URL", "")

		config, err := LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, "https://cloud.langfuse.com", config.BaseURL)
	})
}

func TestClient_IsEnabled(t *testing.T) {
	t.Run("returns false when config is nil", func(t *testing.T) {
		client := &Client{}
		assert.False(t, client.IsEnabled())
	})

	t.Run("returns false when disabled", func(t *testing.T) {
		client := NewClient(&Config{Enabled: false}, logrus.New())
		assert.False(t, client.IsEnabled())
	})

	t.Run("returns true when enabled", func(t *testing.T) {
		client := NewClient(&Config{Enabled: true}, logrus.New())
		assert.True(t, client.IsEnabled())
	})
}

func TestClient_CreateOrUpdateUserSync(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/public/users", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify basic auth
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "test-public-key", user)
		assert.Equal(t, "test-secret-key", pass)

		// Parse and verify body
		var payload UserPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)
		assert.Equal(t, "agent-123", payload.ID)
		assert.Equal(t, "Test Agent", payload.Name)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "agent-123", "name": "Test Agent"}`))
	}))
	defer server.Close()

	client := newTestClient(server)

	err := client.CreateOrUpdateUserSync(context.Background(), UserPayload{
		ID:   "agent-123",
		Name: "Test Agent",
		Metadata: map[string]interface{}{
			"version": "1.0.0",
		},
	})

	assert.NoError(t, err)
}

func TestClient_CreateTrace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/public/traces", r.URL.Path)

		var payload TracePayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)
		assert.Equal(t, "benchmark-run", payload.Name)
		assert.Equal(t, "agent-123", payload.UserID)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "trace-456",
			"name": "benchmark-run",
			"userId": "agent-123",
			"createdAt": "2026-01-20T10:00:00Z"
		}`))
	}))
	defer server.Close()

	client := newTestClient(server)

	trace, err := client.CreateTrace(context.Background(), TracePayload{
		Name:   "benchmark-run",
		UserID: "agent-123",
		Metadata: map[string]interface{}{
			"scenario": "pod-crash",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "trace-456", trace.ID)
	assert.Equal(t, "benchmark-run", trace.Name)
}

func TestClient_ListTraces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/api/public/traces")
		assert.Equal(t, "agent-123", r.URL.Query().Get("userId"))
		assert.Equal(t, "10", r.URL.Query().Get("limit"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": [
				{"id": "trace-1", "name": "benchmark-run", "userId": "agent-123"},
				{"id": "trace-2", "name": "benchmark-run-2", "userId": "agent-123"}
			],
			"meta": 2
		}`))
	}))
	defer server.Close()

	client := newTestClient(server)

	traces, err := client.ListTraces(context.Background(), TraceFilter{
		UserID: "agent-123",
		Limit:  10,
	})

	require.NoError(t, err)
	assert.Len(t, traces.Data, 2)
	assert.Equal(t, "trace-1", traces.Data[0].ID)
	assert.Equal(t, "trace-2", traces.Data[1].ID)
}

func TestClient_CreateScore(t *testing.T) {
	eventReceived := make(chan bool, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/public/ingestion" {
			var req IngestionRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(req.Batch), 1)
			eventReceived <- true
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"successes": ["id-1"], "errors": []}`))
	}))
	defer server.Close()

	client := newTestClient(server)

	err := client.CreateScore(context.Background(), ScorePayload{
		TraceID:  "trace-456",
		Name:     "time_to_detect",
		Value:    12.5,
		Comment:  "TTD: 12.5 seconds",
		DataType: "NUMERIC",
	})

	require.NoError(t, err)

	// Flush and verify
	client.FlushBatch(context.Background())

	select {
	case <-eventReceived:
		// Success
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for batch ingestion")
	}
}

func TestClient_HealthCheck(t *testing.T) {
	t.Run("returns error when disabled", func(t *testing.T) {
		client := NewClient(&Config{Enabled: false}, logrus.New())
		err := client.HealthCheck(context.Background())
		assert.ErrorIs(t, err, ErrNotEnabled)
	})

	t.Run("returns nil on successful health check", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/public/traces", r.URL.Path)
			assert.Equal(t, "1", r.URL.Query().Get("limit"))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": []}`))
		}))
		defer server.Close()

		client := newTestClient(server)
		err := client.HealthCheck(context.Background())
		assert.NoError(t, err)
	})
}

func TestLangfuseError(t *testing.T) {
	t.Run("IsRetryable returns true for 5xx and 429", func(t *testing.T) {
		assert.True(t, (&LangfuseError{StatusCode: 500}).IsRetryable())
		assert.True(t, (&LangfuseError{StatusCode: 502}).IsRetryable())
		assert.True(t, (&LangfuseError{StatusCode: 429}).IsRetryable())
		assert.False(t, (&LangfuseError{StatusCode: 400}).IsRetryable())
		assert.False(t, (&LangfuseError{StatusCode: 401}).IsRetryable())
	})

	t.Run("Error formats correctly", func(t *testing.T) {
		err := &LangfuseError{
			StatusCode: 401,
			Message:    "Unauthorized",
			Details:    "Invalid API key",
		}
		assert.Contains(t, err.Error(), "401")
		assert.Contains(t, err.Error(), "Unauthorized")
		assert.Contains(t, err.Error(), "Invalid API key")
	})
}

func TestBenchmarkTracer(t *testing.T) {
	eventCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/public/ingestion" {
			var req IngestionRequest
			json.NewDecoder(r.Body).Decode(&req)
			eventCount += len(req.Batch)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"successes": [], "errors": []}`))
	}))
	defer server.Close()

	client := newTestClient(server)

	// Create a benchmark tracer
	tracer := client.NewBenchmarkTracer(
		"agent-123",
		"session-456",
		"pod-crash-benchmark",
		map[string]interface{}{"cluster": "test"},
	)

	assert.NotEmpty(t, tracer.TraceID())

	// Record a span
	span := tracer.StartSpan("query_pod_status", map[string]interface{}{"pod": "nginx-xxx"})
	span.End(map[string]interface{}{"status": "Running"}, nil)

	// Record a generation
	tracer.RecordGeneration("analyze_logs", "gpt-4", "logs...", "analysis...", &Usage{TotalTokens: 100})

	// Record benchmark results
	tracer.RecordBenchmarkResults(12.5, 45.0, true, map[string]interface{}{"remediated": true})

	// Complete and flush
	err := tracer.Complete(context.Background())
	require.NoError(t, err)

	// Verify events were batched
	assert.Greater(t, eventCount, 0)
}
