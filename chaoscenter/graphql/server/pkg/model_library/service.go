package model_library

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"k8s.io/client-go/kubernetes"

	model_library_db "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/model_library"
)

// ModelConfig is the service-layer representation (no API key).
type ModelConfig struct {
	Alias       string
	Provider    string
	Model       string
	BaseURL     *string
	SecretRef   string
	AgentsUsing []string
	Status      string
	LastTested  *string
}

type CreateModelConfigRequest struct {
	Alias    string
	Provider string
	Model    string
	BaseURL  *string
	APIKey   string
}

type UpdateModelConfigRequest struct {
	Provider string
	Model    string
	BaseURL  *string
	APIKey   string
}

type TestModelConfigRequest struct {
	Provider string
	Model    string
	APIKey   string
	BaseURL  *string
}

type ModelConfigTestResult struct {
	Success      bool
	LatencyMs    *int
	ErrorMessage *string
}

// ModelLibraryService provides Model Library operations.
type ModelLibraryService interface {
	CreateModelConfig(ctx context.Context, projectID string, req CreateModelConfigRequest) (*ModelConfig, error)
	UpdateModelConfig(ctx context.Context, projectID, alias string, req UpdateModelConfigRequest) (*ModelConfig, error)
	DeleteModelConfig(ctx context.Context, projectID, alias string) error
	GetModelConfig(ctx context.Context, projectID, alias string) (*ModelConfig, error)
	ListModelConfigs(ctx context.Context, projectID string) ([]*ModelConfig, error)
	TestModelConfig(ctx context.Context, req TestModelConfigRequest) (*ModelConfigTestResult, error)
	RotateAPIKey(ctx context.Context, projectID, alias, newAPIKey string) (*ModelConfig, error)
	GetLiteLLMUpstreamForAlias(ctx context.Context, projectID, alias string) (string, error)
}

type serviceImpl struct {
	db        *model_library_db.Operations
	litellm   LiteLLMClient
	k8sClient kubernetes.Interface
}

func NewService(db *model_library_db.Operations, litellm LiteLLMClient, k8sClient kubernetes.Interface) ModelLibraryService {
	return &serviceImpl{db: db, litellm: litellm, k8sClient: k8sClient}
}

var nonAlphaNum = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func sanitize(s string) string {
	return strings.ToLower(nonAlphaNum.ReplaceAllString(s, "-"))
}

func secretName(alias, projectID string) string {
	return fmt.Sprintf("ace-model-%s-%s", sanitize(alias), sanitize(projectID))
}

func envVarName(alias string) string {
	return "ACE_MODEL_" + strings.ToUpper(nonAlphaNum.ReplaceAllString(alias, "_"))
}

func docToConfig(doc model_library_db.ModelConfigDocument) *ModelConfig {
	cfg := &ModelConfig{
		Alias:       doc.Alias,
		Provider:    doc.Provider,
		Model:       doc.Model,
		BaseURL:     doc.BaseURL,
		SecretRef:   doc.SecretRef,
		AgentsUsing: doc.AgentsUsing,
		Status:      doc.Status,
	}
	if doc.LastTestedAt != nil {
		t := time.Unix(*doc.LastTestedAt, 0).UTC().Format(time.RFC3339)
		cfg.LastTested = &t
	}
	return cfg
}

func (s *serviceImpl) CreateModelConfig(ctx context.Context, projectID string, req CreateModelConfigRequest) (*ModelConfig, error) {
	if _, err := s.db.GetModelConfigByAlias(ctx, projectID, req.Alias); err == nil {
		return nil, ErrDuplicateAlias{Alias: req.Alias}
	}
	secretRef := secretName(req.Alias, projectID)
	envVar := envVarName(req.Alias)
	if err := CreateOrUpdateSecret(ctx, s.k8sClient, secretRef, map[string][]byte{
		"API_KEY": []byte(req.APIKey),
	}); err != nil {
		return nil, fmt.Errorf("creating K8s secret: %w", err)
	}
	baseURL := ""
	if req.BaseURL != nil {
		baseURL = *req.BaseURL
	}
	deployID, err := s.litellm.AddModel(ctx, AddModelRequest{
		ModelName:    req.Alias,
		Provider:     req.Provider,
		Model:        req.Model,
		APIKeyEnvVar: "os.environ/" + envVar,
		BaseURL:      baseURL,
	})
	if err != nil {
		_ = DeleteSecret(ctx, s.k8sClient, secretRef)
		return nil, fmt.Errorf("LiteLLM registration: %w", err)
	}
	doc := model_library_db.ModelConfigDocument{
		ProjectID:       projectID,
		Alias:           req.Alias,
		Provider:        req.Provider,
		Model:           req.Model,
		BaseURL:         req.BaseURL,
		SecretRef:       secretRef,
		LiteLLMDeployID: deployID,
		Status:          "untested",
		AgentsUsing:     []string{},
	}
	if err := s.db.InsertModelConfig(ctx, doc); err != nil {
		return nil, err
	}
	return docToConfig(doc), nil
}

func (s *serviceImpl) UpdateModelConfig(ctx context.Context, projectID, alias string, req UpdateModelConfigRequest) (*ModelConfig, error) {
	doc, err := s.db.GetModelConfigByAlias(ctx, projectID, alias)
	if err != nil {
		return nil, ErrAliasNotFound{Alias: alias}
	}
	if req.APIKey != "" {
		if err := CreateOrUpdateSecret(ctx, s.k8sClient, doc.SecretRef, map[string][]byte{"API_KEY": []byte(req.APIKey)}); err != nil {
			return nil, fmt.Errorf("updating K8s secret: %w", err)
		}
	}
	baseURL := ""
	if req.BaseURL != nil {
		baseURL = *req.BaseURL
	}
	envVar := envVarName(alias)
	if err := s.litellm.UpdateModel(ctx, doc.LiteLLMDeployID, AddModelRequest{
		ModelName: alias, Provider: req.Provider, Model: req.Model,
		APIKeyEnvVar: "os.environ/" + envVar, BaseURL: baseURL,
	}); err != nil {
		return nil, fmt.Errorf("LiteLLM update: %w", err)
	}
	update := bson.D{
		{Key: "provider", Value: req.Provider},
		{Key: "model", Value: req.Model},
		{Key: "status", Value: "untested"},
	}
	if req.BaseURL != nil {
		update = append(update, bson.E{Key: "baseURL", Value: *req.BaseURL})
	}
	if err := s.db.UpdateModelConfig(ctx, projectID, alias, update); err != nil {
		return nil, err
	}
	updated, _ := s.db.GetModelConfigByAlias(ctx, projectID, alias)
	return docToConfig(*updated), nil
}

func (s *serviceImpl) DeleteModelConfig(ctx context.Context, projectID, alias string) error {
	doc, err := s.db.GetModelConfigByAlias(ctx, projectID, alias)
	if err != nil {
		return ErrAliasNotFound{Alias: alias}
	}
	if len(doc.AgentsUsing) > 0 {
		return ErrAliasInUse{Alias: alias, Agents: doc.AgentsUsing}
	}
	_ = s.litellm.DeleteModel(ctx, doc.LiteLLMDeployID)
	_ = DeleteSecret(ctx, s.k8sClient, doc.SecretRef)
	return s.db.DeleteModelConfig(ctx, projectID, alias)
}

func (s *serviceImpl) GetModelConfig(ctx context.Context, projectID, alias string) (*ModelConfig, error) {
	doc, err := s.db.GetModelConfigByAlias(ctx, projectID, alias)
	if err != nil {
		return nil, ErrAliasNotFound{Alias: alias}
	}
	return docToConfig(*doc), nil
}

func (s *serviceImpl) ListModelConfigs(ctx context.Context, projectID string) ([]*ModelConfig, error) {
	docs, err := s.db.ListModelConfigsByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	result := make([]*ModelConfig, len(docs))
	for i, d := range docs {
		result[i] = docToConfig(d)
	}
	return result, nil
}

func (s *serviceImpl) TestModelConfig(ctx context.Context, req TestModelConfigRequest) (*ModelConfigTestResult, error) {
	baseURL := ""
	if req.BaseURL != nil {
		baseURL = *req.BaseURL
	}
	result, err := s.litellm.TestCompletion(ctx, TestRequest{
		Provider: req.Provider,
		Model:    req.Model,
		APIKey:   req.APIKey,
		BaseURL:  baseURL,
	})
	if err != nil {
		msg := err.Error()
		return &ModelConfigTestResult{Success: false, ErrorMessage: &msg}, nil
	}
	if !result.Success {
		return &ModelConfigTestResult{Success: false, ErrorMessage: &result.Error, LatencyMs: &result.LatencyMs}, nil
	}
	return &ModelConfigTestResult{Success: true, LatencyMs: &result.LatencyMs}, nil
}

func (s *serviceImpl) RotateAPIKey(ctx context.Context, projectID, alias, newAPIKey string) (*ModelConfig, error) {
	doc, err := s.db.GetModelConfigByAlias(ctx, projectID, alias)
	if err != nil {
		return nil, ErrAliasNotFound{Alias: alias}
	}
	if err := CreateOrUpdateSecret(ctx, s.k8sClient, doc.SecretRef, map[string][]byte{"API_KEY": []byte(newAPIKey)}); err != nil {
		return nil, fmt.Errorf("rotating K8s secret: %w", err)
	}
	update := bson.D{{Key: "status", Value: "untested"}}
	_ = s.db.UpdateModelConfig(ctx, projectID, alias, update)
	updated, _ := s.db.GetModelConfigByAlias(ctx, projectID, alias)
	return docToConfig(*updated), nil
}

func (s *serviceImpl) GetLiteLLMUpstreamForAlias(ctx context.Context, projectID, alias string) (string, error) {
	baseURL := strings.TrimRight(func() string {
		u := strings.TrimRight("http://litellm.litmus.svc.cluster.local:4000", "/")
		return u
	}(), "/")
	return baseURL, nil
}
