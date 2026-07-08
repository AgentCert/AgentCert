package model_library

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// LiteLLMClient interacts with the LiteLLM proxy admin API.
type LiteLLMClient interface {
	AddModel(ctx context.Context, req AddModelRequest) (string, error)
	UpdateModel(ctx context.Context, modelName string, req AddModelRequest) error
	DeleteModel(ctx context.Context, modelName string) error
	TestCompletion(ctx context.Context, req TestRequest) (*TestResult, error)
}

type AddModelRequest struct {
	ModelName    string
	Provider     string
	Model        string
	APIKeyEnvVar string
	BaseURL      string
}

type TestRequest struct {
	Provider string
	Model    string
	APIKey   string
	BaseURL  string
}

type TestResult struct {
	Success   bool
	LatencyMs int
	Error     string
}

type liteLLMClientImpl struct {
	baseURL    string
	adminKey   string
	httpClient *http.Client
}

func NewLiteLLMClient() LiteLLMClient {
	baseURL := os.Getenv("LITELLM_ADMIN_URL")
	if baseURL == "" {
		baseURL = "http://litellm.litmus.svc.cluster.local:4000"
	}
	adminKey := os.Getenv("LITELLM_ADMIN_KEY")
	return &liteLLMClientImpl{
		baseURL:    strings.TrimRight(baseURL, "/"),
		adminKey:   adminKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *liteLLMClientImpl) do(ctx context.Context, method, path string, body interface{}) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reqBody = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.adminKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.adminKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	respData, _ := io.ReadAll(resp.Body)
	return respData, resp.StatusCode, nil
}

func (c *liteLLMClientImpl) AddModel(ctx context.Context, req AddModelRequest) (string, error) {
	litellmModel := req.Provider + "/" + req.Model
	if req.Provider == "ollama" || req.Provider == "custom" {
		litellmModel = req.Model
	}
	body := map[string]interface{}{
		"model_name": req.ModelName,
		"litellm_params": map[string]interface{}{
			"model":   litellmModel,
			"api_key": req.APIKeyEnvVar,
		},
	}
	if req.BaseURL != "" {
		body["litellm_params"].(map[string]interface{})["api_base"] = req.BaseURL
	}
	data, status, err := c.do(ctx, http.MethodPost, "/model/new", body)
	if err != nil {
		return "", fmt.Errorf("LiteLLM AddModel: %w", err)
	}
	if status >= 400 {
		return "", fmt.Errorf("LiteLLM AddModel status %d: %s", status, string(data))
	}
	var resp struct {
		ModelID string `json:"model_id"`
	}
	_ = json.Unmarshal(data, &resp)
	return resp.ModelID, nil
}

func (c *liteLLMClientImpl) UpdateModel(ctx context.Context, modelName string, req AddModelRequest) error {
	litellmModel := req.Provider + "/" + req.Model
	body := map[string]interface{}{
		"model_name": modelName,
		"litellm_params": map[string]interface{}{
			"model":   litellmModel,
			"api_key": req.APIKeyEnvVar,
		},
	}
	if req.BaseURL != "" {
		body["litellm_params"].(map[string]interface{})["api_base"] = req.BaseURL
	}
	data, status, err := c.do(ctx, http.MethodPost, "/model/update", body)
	if err != nil {
		return fmt.Errorf("LiteLLM UpdateModel: %w", err)
	}
	if status >= 400 {
		return fmt.Errorf("LiteLLM UpdateModel status %d: %s", status, string(data))
	}
	return nil
}

func (c *liteLLMClientImpl) DeleteModel(ctx context.Context, modelName string) error {
	body := map[string]interface{}{"model_name": modelName}
	data, status, err := c.do(ctx, http.MethodPost, "/model/delete", body)
	if err != nil {
		return fmt.Errorf("LiteLLM DeleteModel: %w", err)
	}
	if status >= 400 {
		return fmt.Errorf("LiteLLM DeleteModel status %d: %s", status, string(data))
	}
	return nil
}

func (c *liteLLMClientImpl) TestCompletion(ctx context.Context, req TestRequest) (*TestResult, error) {
	start := time.Now()
	litellmModel := req.Provider + "/" + req.Model
	if req.Provider == "ollama" || req.Provider == "custom" {
		litellmModel = req.Model
	}
	body := map[string]interface{}{
		"model": litellmModel,
		"messages": []map[string]string{
			{"role": "user", "content": "Reply with one word: OK"},
		},
		"max_tokens": 5,
	}
	// For test calls we pass the key directly via headers
	testClient := &liteLLMClientImpl{
		baseURL:  c.baseURL,
		adminKey: req.APIKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	data, status, err := testClient.do(ctx, http.MethodPost, "/chat/completions", body)
	latencyMs := int(time.Since(start).Milliseconds())
	if err != nil {
		return &TestResult{Success: false, LatencyMs: latencyMs, Error: err.Error()}, nil
	}
	if status >= 400 {
		return &TestResult{Success: false, LatencyMs: latencyMs, Error: fmt.Sprintf("status %d: %s", status, string(data))}, nil
	}
	return &TestResult{Success: true, LatencyMs: latencyMs}, nil
}
