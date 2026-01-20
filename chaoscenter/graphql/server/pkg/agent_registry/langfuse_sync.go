package agent_registry

import (
	"context"
	"time"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/langfuse"
	"github.com/sirupsen/logrus"
)

// LangfuseSyncStatus represents the sync status
type LangfuseSyncStatus string

const (
	SyncStatusPending LangfuseSyncStatus = "PENDING"
	SyncStatusSynced  LangfuseSyncStatus = "SYNCED"
	SyncStatusFailed  LangfuseSyncStatus = "FAILED"
)

// LangfuseSyncResult contains details about a sync operation
type LangfuseSyncResult struct {
	AgentID      string             `json:"agentId"`
	AgentName    string             `json:"agentName"`
	LangfuseID   string             `json:"langfuseId,omitempty"`
	Status       LangfuseSyncStatus `json:"status"`
	Error        string             `json:"error,omitempty"`
	SyncedAt     time.Time          `json:"syncedAt"`
	ResponseTime int64              `json:"responseTimeMs"`
}

// LangfuseSync manages synchronization of agents to Langfuse
type LangfuseSync struct {
	operator   *Operator
	client     *langfuse.Client
	logger     *logrus.Logger
	interval   time.Duration
	stopCh     chan struct{}
	running    bool
}

// NewLangfuseSync creates a new Langfuse synchronization manager
func NewLangfuseSync(operator *Operator, client *langfuse.Client, logger *logrus.Logger, interval time.Duration) *LangfuseSync {
	if interval < time.Minute {
		interval = time.Minute * 5
	}

	return &LangfuseSync{
		operator: operator,
		client:   client,
		logger:   logger,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the sync scheduler
func (l *LangfuseSync) Start(ctx context.Context) {
	if l.running {
		l.logger.Warn("Langfuse sync already running")
		return
	}

	if l.client == nil || !l.client.IsEnabled() {
		l.logger.Info("Langfuse not enabled, sync scheduler not started")
		return
	}

	l.running = true
	l.logger.Infof("Starting Langfuse sync scheduler with interval %s", l.interval)

	go l.run(ctx)
}

// Stop stops the sync scheduler
func (l *LangfuseSync) Stop() {
	if !l.running {
		return
	}

	l.logger.Info("Stopping Langfuse sync scheduler")
	close(l.stopCh)
	l.running = false
}

// run is the main loop for syncing
func (l *LangfuseSync) run(ctx context.Context) {
	ticker := time.NewTicker(l.interval)
	defer ticker.Stop()

	// Run immediately on start
	l.syncPendingAgents(ctx)

	for {
		select {
		case <-ctx.Done():
			l.logger.Info("Langfuse sync context cancelled")
			return
		case <-l.stopCh:
			l.logger.Info("Langfuse sync stopped")
			return
		case <-ticker.C:
			l.syncPendingAgents(ctx)
		}
	}
}

// syncPendingAgents syncs all agents with pending or failed status
func (l *LangfuseSync) syncPendingAgents(ctx context.Context) {
	agents, err := l.operator.GetAgentsForLangfuseSync(ctx)
	if err != nil {
		l.logger.Errorf("Failed to get agents for Langfuse sync: %v", err)
		return
	}

	if len(agents) == 0 {
		l.logger.Debug("No agents pending Langfuse sync")
		return
	}

	l.logger.Infof("Syncing %d agents to Langfuse", len(agents))

	for _, agent := range agents {
		result := l.SyncAgent(ctx, agent)
		if result.Status == SyncStatusFailed {
			l.logger.Warnf("Failed to sync agent %s: %s", agent.AgentID, result.Error)
		} else {
			l.logger.Infof("Successfully synced agent %s to Langfuse", agent.AgentID)
		}
	}
}

// SyncAgent syncs a single agent to Langfuse
func (l *LangfuseSync) SyncAgent(ctx context.Context, agent *Agent) *LangfuseSyncResult {
	result := &LangfuseSyncResult{
		AgentID:   agent.AgentID,
		AgentName: agent.Name,
		SyncedAt:  time.Now().UTC(),
	}

	start := time.Now()

	// Build user metadata
	metadata := l.buildAgentMetadata(agent)

	// Sync to Langfuse as a user
	err := l.client.CreateOrUpdateUserSync(ctx, langfuse.UserPayload{
		ID:       agent.AgentID,
		Name:     agent.Name,
		Metadata: metadata,
	})

	result.ResponseTime = time.Since(start).Milliseconds()

	if err != nil {
		result.Status = SyncStatusFailed
		result.Error = err.Error()
		l.updateSyncStatus(ctx, agent.AgentID, result)
		return result
	}

	result.Status = SyncStatusSynced
	result.LangfuseID = agent.AgentID
	l.updateSyncStatus(ctx, agent.AgentID, result)

	return result
}

// buildAgentMetadata creates the metadata object for Langfuse
func (l *LangfuseSync) buildAgentMetadata(agent *Agent) map[string]interface{} {
	metadata := map[string]interface{}{
		"type":         "ai-agent",
		"vendor":       agent.Vendor,
		"version":      agent.Version,
		"namespace":    agent.Namespace,
		"status":       string(agent.Status),
		"capabilities": agent.Capabilities,
		"projectId":    agent.ProjectID,
		"createdAt":    agent.AuditInfo.CreatedAt.Format(time.RFC3339),
		"updatedAt":    agent.AuditInfo.UpdatedAt.Format(time.RFC3339),
	}

	if agent.Description != "" {
		metadata["description"] = agent.Description
	}

	if agent.ContainerImage != nil {
		metadata["containerImage"] = agent.ContainerImage.FullImageName()
	}

	if agent.Endpoint != nil {
		endpointInfo := map[string]interface{}{
			"type":          string(agent.Endpoint.EndpointType),
			"discoveryType": string(agent.Endpoint.DiscoveryType),
		}
		if agent.Endpoint.URL != "" {
			endpointInfo["url"] = agent.Endpoint.URL
		}
		if agent.Endpoint.ServiceName != "" {
			endpointInfo["serviceName"] = agent.Endpoint.ServiceName
			endpointInfo["port"] = agent.Endpoint.Port
		}
		metadata["endpoint"] = endpointInfo
	}

	if agent.Tags != nil && len(agent.Tags) > 0 {
		metadata["tags"] = agent.Tags
	}

	if agent.Metadata != nil && len(agent.Metadata) > 0 {
		metadata["customMetadata"] = agent.Metadata
	}

	if agent.LastHealthCheck != nil {
		metadata["lastHealthCheck"] = map[string]interface{}{
			"success":      agent.LastHealthCheck.Success,
			"timestamp":    agent.LastHealthCheck.Timestamp.Format(time.RFC3339),
			"responseTime": agent.LastHealthCheck.ResponseTime,
		}
	}

	return metadata
}

// updateSyncStatus updates the Langfuse config in the database
func (l *LangfuseSync) updateSyncStatus(ctx context.Context, agentID string, result *LangfuseSyncResult) {
	config := &LangfuseConfig{
		Enabled:    true,
		SyncStatus: string(result.Status),
		SyncedAt:   result.SyncedAt,
	}

	if result.LangfuseID != "" {
		config.LangfuseID = result.LangfuseID
	}

	if result.Error != "" {
		config.LastSyncError = result.Error
	}

	_, err := l.operator.UpdateLangfuseConfig(ctx, agentID, config)
	if err != nil {
		l.logger.Errorf("Failed to update Langfuse config for agent %s: %v", agentID, err)
	}
}

// SyncAgentByID syncs an agent by its ID
func (l *LangfuseSync) SyncAgentByID(ctx context.Context, agentID string) (*LangfuseSyncResult, error) {
	agent, err := l.operator.GetAgentByID(ctx, agentID)
	if err != nil {
		return nil, err
	}

	return l.SyncAgent(ctx, agent), nil
}

// SyncAllAgents forces sync of all Langfuse-enabled agents
func (l *LangfuseSync) SyncAllAgents(ctx context.Context, projectID string) ([]*LangfuseSyncResult, error) {
	filter := ListAgentsFilter{
		ProjectID: projectID,
	}

	response, err := l.operator.ListAgents(ctx, filter, ListAgentsPagination{Limit: 1000})
	if err != nil {
		return nil, err
	}

	var results []*LangfuseSyncResult
	for _, agent := range response.Agents {
		if agent.LangfuseConfig != nil && agent.LangfuseConfig.Enabled {
			result := l.SyncAgent(ctx, agent)
			results = append(results, result)
		}
	}

	return results, nil
}

// CreateBenchmarkTrace creates a benchmark trace for an agent
func (l *LangfuseSync) CreateBenchmarkTrace(ctx context.Context, agent *Agent, benchmarkID, benchmarkName string) (string, error) {
	if l.client == nil || !l.client.IsEnabled() {
		return "", nil
	}

	metadata := map[string]interface{}{
		"agentId":      agent.AgentID,
		"agentName":    agent.Name,
		"agentVersion": agent.Version,
		"capabilities": agent.Capabilities,
		"projectId":    agent.ProjectID,
		"benchmarkId":  benchmarkID,
	}

	tracer := l.client.NewBenchmarkTracer(agent.AgentID, benchmarkID, benchmarkName, metadata)
	return tracer.TraceID(), nil
}

// RecordBenchmarkResult records a benchmark result for an agent
func (l *LangfuseSync) RecordBenchmarkResult(ctx context.Context, traceID string, result BenchmarkResult) error {
	if l.client == nil || !l.client.IsEnabled() {
		return nil
	}

	// Create scores for the benchmark results
	if _, err := l.client.CreateScoreSync(ctx, langfuse.ScorePayload{
		TraceID: traceID,
		Name:    "benchmark_ttd",
		Value:   result.TTD,
		Comment: "Time to Detection",
	}); err != nil {
		l.logger.Warnf("Failed to record TTD score: %v", err)
	}

	if _, err := l.client.CreateScoreSync(ctx, langfuse.ScorePayload{
		TraceID: traceID,
		Name:    "benchmark_ttr",
		Value:   result.TTR,
		Comment: "Time to Resolution",
	}); err != nil {
		l.logger.Warnf("Failed to record TTR score: %v", err)
	}

	passedValue := 0.0
	if result.Passed {
		passedValue = 1.0
	}
	if _, err := l.client.CreateScoreSync(ctx, langfuse.ScorePayload{
		TraceID: traceID,
		Name:    "benchmark_passed",
		Value:   passedValue,
		Comment: "Benchmark Pass/Fail",
	}); err != nil {
		l.logger.Warnf("Failed to record passed score: %v", err)
	}

	return nil
}

// BenchmarkResult represents the result of a benchmark execution
type BenchmarkResult struct {
	Passed   bool
	Score    float64
	TTD      float64
	TTR      float64
	ErrorMsg string
	Metadata map[string]interface{}
}

// GetSyncStatus returns the current status of the sync scheduler
func (l *LangfuseSync) GetSyncStatus() map[string]interface{} {
	return map[string]interface{}{
		"running":        l.running,
		"interval":       l.interval.String(),
		"langfuseEnabled": l.client != nil && l.client.IsEnabled(),
	}
}
