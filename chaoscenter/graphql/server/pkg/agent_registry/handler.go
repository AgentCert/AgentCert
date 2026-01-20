package agent_registry

import (
	"context"
	"time"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/langfuse"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

// Handler provides the main interface for agent registry operations
// It ties together all components and is used by GraphQL resolvers
type Handler struct {
	service         Service
	operator        *Operator
	validator       *Validator
	healthScheduler *HealthScheduler
	langfuseSync    *LangfuseSync
	logger          *logrus.Logger
}

// HandlerConfig contains configuration for creating a Handler
type HandlerConfig struct {
	MongoDB        *mongo.Database
	LangfuseClient *langfuse.Client
	Logger         *logrus.Logger
}

// NewHandler creates a new Handler with all dependencies
func NewHandler(config HandlerConfig) *Handler {
	if config.Logger == nil {
		config.Logger = logrus.New()
	}

	operator := NewOperator(config.MongoDB)
	validator := NewValidator(operator)
	service := NewService(operator, validator, config.LangfuseClient, config.Logger)

	return &Handler{
		service:   service,
		operator:  operator,
		validator: validator,
		logger:    config.Logger,
	}
}

// Initialize creates indexes and starts background workers
func (h *Handler) Initialize(ctx context.Context) error {
	// Create indexes
	if err := h.operator.CreateIndexes(ctx); err != nil {
		h.logger.Warnf("Failed to create indexes (may already exist): %v", err)
	}

	return nil
}

// StartBackgroundWorkers starts health check and Langfuse sync schedulers
func (h *Handler) StartBackgroundWorkers(ctx context.Context, langfuseClient *langfuse.Client) {
	// Start health scheduler
	h.healthScheduler = NewHealthScheduler(h.operator, h.logger, DefaultHealthCheckInterval*time.Second)
	h.healthScheduler.Start(ctx)

	// Start Langfuse sync if enabled
	if langfuseClient != nil && langfuseClient.IsEnabled() {
		h.langfuseSync = NewLangfuseSync(h.operator, langfuseClient, h.logger, DefaultLangfuseSyncInterval*time.Second)
		h.langfuseSync.Start(ctx)
	}
}

// StopBackgroundWorkers stops all background workers
func (h *Handler) StopBackgroundWorkers() {
	if h.healthScheduler != nil {
		h.healthScheduler.Stop()
	}
	if h.langfuseSync != nil {
		h.langfuseSync.Stop()
	}
}

// RegisterAgent registers a new AI agent
func (h *Handler) RegisterAgent(ctx context.Context, input RegisterAgentInput, createdBy string) (*Agent, error) {
	return h.service.RegisterAgent(ctx, input, createdBy)
}

// GetAgent retrieves an agent by ID
func (h *Handler) GetAgent(ctx context.Context, agentID string) (*Agent, error) {
	return h.service.GetAgent(ctx, agentID)
}

// UpdateAgent updates an existing agent
func (h *Handler) UpdateAgent(ctx context.Context, agentID string, input UpdateAgentInput, updatedBy string) (*Agent, error) {
	return h.service.UpdateAgent(ctx, agentID, input, updatedBy)
}

// DeleteAgent soft-deletes an agent
func (h *Handler) DeleteAgent(ctx context.Context, agentID string, deletedBy string) error {
	return h.service.DeleteAgent(ctx, agentID, deletedBy)
}

// ListAgents lists agents with filtering and pagination
func (h *Handler) ListAgents(ctx context.Context, filter ListAgentsFilter, pagination ListAgentsPagination) (*ListAgentsResponse, error) {
	return h.service.ListAgents(ctx, filter, pagination)
}

// GetAgentsByCapabilities finds agents with specific capabilities
func (h *Handler) GetAgentsByCapabilities(ctx context.Context, projectID string, capabilities []string, activeOnly bool) ([]*Agent, error) {
	return h.service.GetAgentsByCapabilities(ctx, projectID, capabilities, activeOnly)
}

// GetCapabilitiesTaxonomy returns the predefined capability categories
func (h *Handler) GetCapabilitiesTaxonomy() map[string][]string {
	return h.service.GetCapabilitiesTaxonomy()
}

// UpdateAgentStatus updates the status of an agent
func (h *Handler) UpdateAgentStatus(ctx context.Context, agentID string, status AgentStatus, updatedBy string) (*Agent, error) {
	return h.service.UpdateAgentStatus(ctx, agentID, status, updatedBy)
}

// ValidateAgentHealth performs a health check on an agent
func (h *Handler) ValidateAgentHealth(ctx context.Context, agentID string) (*Agent, error) {
	return h.service.ValidateAgentHealth(ctx, agentID)
}

// TriggerHealthCheck performs an immediate health check on a specific agent
func (h *Handler) TriggerHealthCheck(ctx context.Context, agentID string) (*DetailedHealthCheckResult, error) {
	if h.healthScheduler == nil {
		return nil, NewOperationError("TriggerHealthCheck", agentID, ErrHealthSchedulerNotRunning)
	}
	return h.healthScheduler.CheckSingleAgent(ctx, agentID)
}

// SyncAgentToLangfuse syncs an agent to Langfuse
func (h *Handler) SyncAgentToLangfuse(ctx context.Context, agentID string) (*LangfuseSyncResult, error) {
	if h.langfuseSync == nil {
		return nil, NewOperationError("SyncAgentToLangfuse", agentID, ErrLangfuseNotEnabled)
	}
	return h.langfuseSync.SyncAgentByID(ctx, agentID)
}

// SyncAllAgentsToLangfuse syncs all Langfuse-enabled agents
func (h *Handler) SyncAllAgentsToLangfuse(ctx context.Context, projectID string) ([]*LangfuseSyncResult, error) {
	if h.langfuseSync == nil {
		return nil, NewOperationError("SyncAllAgentsToLangfuse", "", ErrLangfuseNotEnabled)
	}
	return h.langfuseSync.SyncAllAgents(ctx, projectID)
}

// CreateBenchmarkTrace creates a benchmark trace for an agent
func (h *Handler) CreateBenchmarkTrace(ctx context.Context, agentID, benchmarkID, benchmarkName string) (string, error) {
	if h.langfuseSync == nil {
		return "", nil
	}

	agent, err := h.service.GetAgent(ctx, agentID)
	if err != nil {
		return "", err
	}

	return h.langfuseSync.CreateBenchmarkTrace(ctx, agent, benchmarkID, benchmarkName)
}

// RecordBenchmarkResult records a benchmark result for an agent
func (h *Handler) RecordBenchmarkResult(ctx context.Context, traceID string, result BenchmarkResult) error {
	if h.langfuseSync == nil {
		return nil
	}

	return h.langfuseSync.RecordBenchmarkResult(ctx, traceID, result)
}

// GetStatus returns the status of the handler and its background workers
func (h *Handler) GetStatus() map[string]interface{} {
	status := map[string]interface{}{
		"initialized": true,
	}

	if h.healthScheduler != nil {
		status["healthScheduler"] = h.healthScheduler.GetSchedulerStatus()
	} else {
		status["healthScheduler"] = map[string]interface{}{"running": false}
	}

	if h.langfuseSync != nil {
		status["langfuseSync"] = h.langfuseSync.GetSyncStatus()
	} else {
		status["langfuseSync"] = map[string]interface{}{"running": false}
	}

	return status
}
