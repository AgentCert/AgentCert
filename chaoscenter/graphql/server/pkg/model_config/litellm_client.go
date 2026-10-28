package model_config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	adminTimeout = 30 * time.Second
	// testTimeout stays under the 60 s nginx gives a GraphQL request, so a
	// slow model reports a timeout instead of a gateway error page. LiteLLM
	// gives up on a health check after 60 s itself.
	testTimeout      = 55 * time.Second
	maxErrorLength   = 500
	maxResponseBytes = 4 << 20
)

// Deployment is a model as LiteLLM serves it, without its API key: the key is
// passed alongside on the calls that need one and is never stored by ACE.
type Deployment struct {
	ModelID    string
	Alias      string
	Model      string
	APIBase    string
	APIVersion string
}

func (d Deployment) params(apiKey string) map[string]string {
	params := map[string]string{"model": d.Model, "api_base": d.APIBase, "api_key": apiKey}
	if d.APIVersion != "" {
		params["api_version"] = d.APIVersion
	}
	return params
}

// RemoteModel is one entry of LiteLLM's model list.
type RemoteModel struct {
	Alias   string
	ModelID string
	// DBModel: stored in LiteLLM's database, not its config file.
	DBModel bool
}

// TestResult is the outcome of sending a model one test prompt.
type TestResult struct {
	OK        bool
	LatencyMs int
	Error     string
}

// LiteLLMClient is ACE's client for LiteLLM's admin API, authenticated with
// the master key. Writes are never retried: an unknown outcome
// (ErrUnreachable) is left to reconcile, which looks the model up by its ID.
type LiteLLMClient interface {
	// AddModel registers d under d.ModelID (POST /model/new).
	AddModel(ctx context.Context, d Deployment, apiKey string) error
	// UpdateModel replaces d's parameters and key (PATCH /model/{id}/update).
	// An unknown ID is ErrNotFound.
	UpdateModel(ctx context.Context, d Deployment, apiKey string) error
	// DeleteModel removes a model (POST /model/delete). An unknown ID is
	// ErrNotFound.
	DeleteModel(ctx context.Context, modelID string) error
	// ListModels returns every model LiteLLM serves (GET /v1/model/info).
	ListModels(ctx context.Context) ([]RemoteModel, error)
	// RegistryConnected reports whether LiteLLM is connected to its database
	// (GET /health/readiness). Without one it serves no database models, so
	// their absence from ListModels means nothing.
	RegistryConnected(ctx context.Context) (bool, error)
	// TestConnection sends a test prompt with d's parameters without
	// registering anything (POST /health/test_connection).
	TestConnection(ctx context.Context, d Deployment, apiKey string) (TestResult, error)
	// HealthCheck sends a test prompt to a registered model with its stored
	// key (GET /health?model_id=). An unknown ID is ErrNotFound.
	HealthCheck(ctx context.Context, modelID string) (TestResult, error)
}

type litellmClient struct {
	baseURL      string
	masterKey    string
	http         *http.Client
	adminTimeout time.Duration
	testTimeout  time.Duration
}

// NewLiteLLMClient returns a client for the LiteLLM proxy at baseURL.
func NewLiteLLMClient(baseURL, masterKey string) LiteLLMClient {
	return &litellmClient{baseURL: strings.TrimRight(baseURL, "/"), masterKey: masterKey, http: &http.Client{},
		adminTimeout: adminTimeout, testTimeout: testTimeout}
}

// AdminURL is where the server reaches LiteLLM's admin API: adminURL
// (LITELLM_ADMIN_URL) when set, else the agents' OPENAI_BASE_URL without /v1,
// the same derivation as the agent sidecar's upstream.
func AdminURL(adminURL, openAIBaseURL string) string {
	if explicit := strings.TrimSpace(adminURL); explicit != "" {
		return strings.TrimRight(explicit, "/")
	}
	return strings.TrimRight(strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(openAIBaseURL), "/"), "/v1"), "/")
}

type modelRequest struct {
	ModelName     string            `json:"model_name,omitempty"`
	LitellmParams map[string]string `json:"litellm_params"`
	ModelInfo     map[string]string `json:"model_info,omitempty"`
}

func (c *litellmClient) AddModel(ctx context.Context, d Deployment, apiKey string) error {
	if err := requireKey(apiKey); err != nil {
		return err
	}
	request := modelRequest{ModelName: d.Alias, LitellmParams: d.params(apiKey), ModelInfo: map[string]string{"id": d.ModelID}}
	return c.write(ctx, http.MethodPost, "/model/new", request, apiKey)
}

// UpdateModel sends every parameter, not just the key, so the request is
// complete whether or not LiteLLM merges litellm_params (1.82 does).
func (c *litellmClient) UpdateModel(ctx context.Context, d Deployment, apiKey string) error {
	if err := requireKey(apiKey); err != nil {
		return err
	}
	return c.write(ctx, http.MethodPatch, "/model/"+url.PathEscape(d.ModelID)+"/update", modelRequest{LitellmParams: d.params(apiKey)}, apiKey)
}

func (c *litellmClient) DeleteModel(ctx context.Context, modelID string) error {
	return c.write(ctx, http.MethodPost, "/model/delete", map[string]string{"id": modelID})
}

func (c *litellmClient) ListModels(ctx context.Context) ([]RemoteModel, error) {
	var payload struct {
		Data []struct {
			ModelName string `json:"model_name"`
			ModelInfo struct {
				ID      string `json:"id"`
				DBModel bool   `json:"db_model"`
			} `json:"model_info"`
		} `json:"data"`
	}
	if err := c.read(ctx, "/v1/model/info", &payload); err != nil {
		return nil, err
	}
	models := make([]RemoteModel, 0, len(payload.Data))
	for _, entry := range payload.Data {
		models = append(models, RemoteModel{Alias: entry.ModelName, ModelID: entry.ModelInfo.ID, DBModel: entry.ModelInfo.DBModel})
	}
	return models, nil
}

func (c *litellmClient) RegistryConnected(ctx context.Context) (bool, error) {
	var payload struct {
		DB string `json:"db"`
	}
	if err := c.read(ctx, "/health/readiness", &payload); err != nil {
		return false, err
	}
	return payload.DB == "connected", nil
}

func (c *litellmClient) TestConnection(ctx context.Context, d Deployment, apiKey string) (TestResult, error) {
	if err := requireKey(apiKey); err != nil {
		return TestResult{}, err
	}
	start := time.Now()
	status, body, err := c.do(ctx, http.MethodPost, "/health/test_connection",
		map[string]interface{}{"mode": "chat", "litellm_params": d.params(apiKey)}, c.testTimeout)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return c.timeoutResult(latency), nil
		}
		return TestResult{}, err
	}
	if err := c.classify(status, body, apiKey); err != nil {
		return TestResult{}, err
	}
	var payload struct {
		Status string `json:"status"`
		Result struct {
			Error json.RawMessage `json:"error"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return TestResult{}, fmt.Errorf("%w: unexpected answer to a connection test: %v", ErrUnreachable, err)
	}
	if payload.Status == "success" {
		return TestResult{OK: true, LatencyMs: latency}, nil
	}
	return c.testOutcome(latency, rawMessage(payload.Result.Error), apiKey), nil
}

func (c *litellmClient) HealthCheck(ctx context.Context, modelID string) (TestResult, error) {
	start := time.Now()
	status, body, err := c.do(ctx, http.MethodGet, "/health?model_id="+url.QueryEscape(modelID), nil, c.testTimeout)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return c.timeoutResult(latency), nil
		}
		return TestResult{}, err
	}
	if status == http.StatusNotFound {
		return TestResult{}, ErrNotFound
	}
	if err := c.classify(status, body); err != nil {
		return TestResult{}, err
	}
	var payload struct {
		HealthyCount       int `json:"healthy_count"`
		UnhealthyCount     int `json:"unhealthy_count"`
		UnhealthyEndpoints []struct {
			Error json.RawMessage `json:"error"`
		} `json:"unhealthy_endpoints"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return TestResult{}, fmt.Errorf("%w: unexpected answer to a health check: %v", ErrUnreachable, err)
	}
	switch {
	case payload.HealthyCount > 0 && payload.UnhealthyCount == 0:
		return TestResult{OK: true, LatencyMs: latency}, nil
	case len(payload.UnhealthyEndpoints) > 0:
		return c.testOutcome(latency, rawMessage(payload.UnhealthyEndpoints[0].Error)), nil
	default:
		return TestResult{LatencyMs: latency, Error: "LiteLLM checked no endpoint for this model"}, nil
	}
}

// testOutcome reads the error a test prompt ended with. A refusal of the
// test's own token cap means the model answered, so it is a pass.
func (c *litellmClient) testOutcome(latency int, message string, secrets ...string) TestResult {
	if containsAny(message, tokenCapRefusals) {
		return TestResult{OK: true, LatencyMs: latency}
	}
	return TestResult{LatencyMs: latency, Error: c.clean(message, secrets...)}
}

// write sends a change. It is never retried: when the outcome is unknown the
// caller keeps the model in a status reconcile completes.
func (c *litellmClient) write(ctx context.Context, method, path string, payload interface{}, secrets ...string) error {
	status, body, err := c.do(ctx, method, path, payload, c.adminTimeout)
	if err != nil {
		return err
	}
	return c.classify(status, body, secrets...)
}

// read fetches and decodes path, retrying once when LiteLLM does not answer.
func (c *litellmClient) read(ctx context.Context, path string, out interface{}) error {
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		var status int
		var body []byte
		status, body, err = c.do(ctx, http.MethodGet, path, nil, c.adminTimeout)
		if err == nil {
			err = c.classify(status, body)
		}
		if err == nil {
			if decodeErr := json.Unmarshal(body, out); decodeErr != nil {
				return fmt.Errorf("%w: unexpected answer from %s: %v", ErrUnreachable, path, decodeErr)
			}
			return nil
		}
		if !errors.Is(err, ErrUnreachable) || ctx.Err() != nil {
			return err
		}
	}
	return err
}

func (c *litellmClient) do(ctx context.Context, method, path string, payload interface{}, timeout time.Duration) (int, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	request.Header.Set("Content-Type", "application/json")
	if c.masterKey != "" {
		request.Header.Set("Authorization", "Bearer "+c.masterKey)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return 0, nil, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	return response.StatusCode, data, nil
}

var (
	// LiteLLM 1.82 reports a missing database with HTTP 500 on /model/new and
	// /model/delete ("No DB Connected"), on /health/test_connection and the
	// PATCH ("DB not connected"), and STORE_MODEL_IN_DB unset with a 500 or
	// 400, so these are matched by message, not status.
	registryOffMarkers = []string{"No DB Connected", "DB not connected", "STORE_MODEL_IN_DB", "only supported for DB-stored models"}
	// "not found in db" is /model/delete's 400; "not found on proxy" is the
	// PATCH's 404.
	notFoundMarkers = []string{"not found in db", "not found on proxy"}
	// LiteLLM caps every test prompt at one token (max_tokens: 1, which
	// /health/test_connection does not let a caller change). Reasoning models
	// (GPT-5, o-series) refuse the parameter, or cannot finish in one token,
	// after accepting the endpoint, key, deployment and API version: a wrong
	// one of those fails with another message (checked against Azure OpenAI).
	tokenCapRefusals = []string{"Unsupported parameter: 'max_tokens'", "max_tokens or model output limit was reached"}
	// LiteLLM wraps some messages as a Python dict: {'error': "..."}.
	pythonDictMessage = regexp.MustCompile(`^\{'(?:error|message)': ['"](.*)['"]\}$`)
)

// classify turns a LiteLLM answer into nil or a gateway error. The message of
// a 401 is dropped: LiteLLM echoes part of the key it received.
func (c *litellmClient) classify(status int, body []byte, secrets ...string) error {
	if status >= 200 && status < 300 {
		return nil
	}
	message := c.clean(responseMessage(body), secrets...)
	switch {
	case containsAny(message, registryOffMarkers):
		return ErrRegistryUnavailable
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return ErrUnauthorized
	case containsAny(message, notFoundMarkers):
		return fmt.Errorf("%w: %s", ErrNotFound, message)
	// "Failed to add model to db" is a 500 for a write that was not applied.
	case status >= 500 && !strings.Contains(message, "Failed to add model to db"):
		return fmt.Errorf("%w: HTTP %d: %s", ErrUnreachable, status, message)
	default:
		return fmt.Errorf("%w: %s", ErrRejected, message)
	}
}

// clean makes a LiteLLM message safe to show: no stack trace, no key, bounded.
func (c *litellmClient) clean(message string, secrets ...string) string {
	if i := strings.Index(message, "stack trace:"); i >= 0 {
		message = message[:i]
	}
	redact := func(secret string) {
		if len(secret) >= 4 {
			message = strings.ReplaceAll(message, secret, "****")
		}
	}
	for _, secret := range secrets {
		redact(secret)
	}
	redact(c.masterKey)
	message = strings.TrimSpace(message)
	if len(message) > maxErrorLength {
		message = message[:maxErrorLength] + "..."
	}
	return message
}

func responseMessage(body []byte) string {
	var payload struct {
		Error  json.RawMessage `json:"error"`
		Detail json.RawMessage `json:"detail"`
	}
	message := strings.TrimSpace(string(body))
	if json.Unmarshal(body, &payload) == nil {
		if text := rawMessage(payload.Error); text != "" {
			message = text
		} else if text := rawMessage(payload.Detail); text != "" {
			message = text
		}
	}
	if match := pythonDictMessage.FindStringSubmatch(message); match != nil {
		message = match[1]
	}
	return message
}

// rawMessage reads LiteLLM's error shapes: a string, {"message": ...} or
// {"error": ...}, nested.
func rawMessage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var object struct {
		Message json.RawMessage `json:"message"`
		Error   json.RawMessage `json:"error"`
	}
	if json.Unmarshal(raw, &object) != nil {
		return ""
	}
	if text := rawMessage(object.Message); text != "" {
		return text
	}
	return rawMessage(object.Error)
}

func containsAny(s string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(s, marker) {
			return true
		}
	}
	return false
}

// requireKey guards the leak noKeyPlaceholder exists for.
func requireKey(apiKey string) error {
	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("%w: refusing to send an empty API key, LiteLLM would substitute one from its own environment", ErrRejected)
	}
	return nil
}

func (c *litellmClient) timeoutResult(latency int) TestResult {
	return TestResult{LatencyMs: latency, Error: fmt.Sprintf(
		"no answer within %s: the model may still be loading, or LiteLLM cannot reach its endpoint", c.testTimeout)}
}
