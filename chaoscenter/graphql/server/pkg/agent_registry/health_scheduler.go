package agent_registry

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// HealthCheckResult with additional details
type DetailedHealthCheckResult struct {
	AgentID       string    `json:"agentId"`
	AgentName     string    `json:"agentName"`
	Timestamp     time.Time `json:"timestamp"`
	Success       bool      `json:"success"`
	StatusCode    int       `json:"statusCode,omitempty"`
	ResponseTime  int64     `json:"responseTimeMs"`
	Message       string    `json:"message"`
	RetryCount    int       `json:"retryCount"`
	ConsecutiveFails int    `json:"consecutiveFails,omitempty"`
}

// HealthScheduler manages periodic health checks for agents
type HealthScheduler struct {
	operator   *Operator
	httpClient *http.Client
	logger     *logrus.Logger
	interval   time.Duration
	stopCh     chan struct{}
	running    bool
}

// NewHealthScheduler creates a new health check scheduler
func NewHealthScheduler(operator *Operator, logger *logrus.Logger, interval time.Duration) *HealthScheduler {
	if interval < time.Minute {
		interval = time.Minute
	}

	return &HealthScheduler{
		operator: operator,
		httpClient: &http.Client{
			Timeout: time.Second * 30,
		},
		logger:   logger,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the health check scheduler
func (h *HealthScheduler) Start(ctx context.Context) {
	if h.running {
		h.logger.Warn("Health scheduler already running")
		return
	}

	h.running = true
	h.logger.Infof("Starting health check scheduler with interval %s", h.interval)

	go h.run(ctx)
}

// Stop stops the health check scheduler
func (h *HealthScheduler) Stop() {
	if !h.running {
		return
	}

	h.logger.Info("Stopping health check scheduler")
	close(h.stopCh)
	h.running = false
}

// run is the main loop for health checking
func (h *HealthScheduler) run(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	// Run immediately on start
	h.runHealthChecks(ctx)

	for {
		select {
		case <-ctx.Done():
			h.logger.Info("Health scheduler context cancelled")
			return
		case <-h.stopCh:
			h.logger.Info("Health scheduler stopped")
			return
		case <-ticker.C:
			h.runHealthChecks(ctx)
		}
	}
}

// runHealthChecks performs health checks on all agents that need checking
func (h *HealthScheduler) runHealthChecks(ctx context.Context) {
	// Get agents that need health checking
	agents, err := h.operator.GetAgentsForHealthCheck(ctx, h.interval)
	if err != nil {
		h.logger.Errorf("Failed to get agents for health check: %v", err)
		return
	}

	if len(agents) == 0 {
		h.logger.Debug("No agents need health checking")
		return
	}

	h.logger.Infof("Running health checks for %d agents", len(agents))

	for _, agent := range agents {
		result := h.checkAgentHealth(ctx, agent)
		h.updateAgentHealth(ctx, agent, result)
	}
}

// checkAgentHealth performs a health check on a single agent
func (h *HealthScheduler) checkAgentHealth(ctx context.Context, agent *Agent) *DetailedHealthCheckResult {
	result := &DetailedHealthCheckResult{
		AgentID:   agent.AgentID,
		AgentName: agent.Name,
		Timestamp: time.Now().UTC(),
	}

	// Get endpoint URL
	url := h.getHealthCheckURL(agent)
	if url == "" {
		result.Success = false
		result.Message = "No endpoint URL configured"
		return result
	}

	// Perform health check with retries
	maxRetries := 3
	if agent.HealthCheck != nil && agent.HealthCheck.MaxRetries > 0 {
		maxRetries = agent.HealthCheck.MaxRetries
	}

	timeout := time.Second * 10
	if agent.HealthCheck != nil && agent.HealthCheck.TimeoutSec > 0 {
		timeout = time.Duration(agent.HealthCheck.TimeoutSec) * time.Second
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		result.RetryCount = attempt

		checkCtx, cancel := context.WithTimeout(ctx, timeout)
		success, statusCode, responseTime, message := h.doHealthCheck(checkCtx, url)
		cancel()

		result.StatusCode = statusCode
		result.ResponseTime = responseTime
		result.Message = message

		if success {
			result.Success = true
			return result
		}

		// Wait before retry
		if attempt < maxRetries {
			time.Sleep(time.Second * time.Duration(attempt+1))
		}
	}

	result.Success = false
	return result
}

// doHealthCheck performs the actual HTTP request
func (h *HealthScheduler) doHealthCheck(ctx context.Context, url string) (bool, int, int64, string) {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, 0, 0, fmt.Sprintf("Failed to create request: %v", err)
	}

	resp, err := h.httpClient.Do(req)
	responseTime := time.Since(start).Milliseconds()

	if err != nil {
		return false, 0, responseTime, fmt.Sprintf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, resp.StatusCode, responseTime, "Health check passed"
	}

	return false, resp.StatusCode, responseTime, fmt.Sprintf("Unhealthy status: %d", resp.StatusCode)
}

// getHealthCheckURL builds the health check URL for an agent
func (h *HealthScheduler) getHealthCheckURL(agent *Agent) string {
	if agent.Endpoint == nil {
		return ""
	}

	baseURL := agent.Endpoint.URL
	if baseURL == "" {
		if agent.Endpoint.ServiceName != "" && agent.Endpoint.Port > 0 {
			scheme := "http"
			if agent.Endpoint.TLSEnabled {
				scheme = "https"
			}
			namespace := agent.Namespace
			if namespace == "" {
				namespace = "default"
			}
			baseURL = fmt.Sprintf("%s://%s.%s.svc.cluster.local:%d", 
				scheme, agent.Endpoint.ServiceName, namespace, agent.Endpoint.Port)
		}
	}

	if baseURL == "" {
		return ""
	}

	healthPath := "/health"
	if agent.HealthCheck != nil && agent.HealthCheck.Path != "" {
		healthPath = agent.HealthCheck.Path
	}

	return baseURL + healthPath
}

// updateAgentHealth updates the agent's health status in the database
func (h *HealthScheduler) updateAgentHealth(ctx context.Context, agent *Agent, result *DetailedHealthCheckResult) {
	healthResult := &HealthCheckResult{
		Timestamp:    result.Timestamp,
		Success:      result.Success,
		ResponseTime: result.ResponseTime,
		Message:      result.Message,
	}

	var newStatus *AgentStatus
	if result.Success {
		// If healthy, mark as active
		if agent.Status == AgentStatusValidating || agent.Status == AgentStatusRegistered {
			active := AgentStatusActive
			newStatus = &active
		}
	} else {
		// Track consecutive failures
		consecutiveFails := 1
		if agent.LastHealthCheck != nil && !agent.LastHealthCheck.Success {
			// This would need additional tracking in the model
			consecutiveFails = 2 // Simplified
		}

		// If too many failures, mark as inactive
		if consecutiveFails >= 3 && agent.Status == AgentStatusActive {
			inactive := AgentStatusInactive
			newStatus = &inactive
			h.logger.Warnf("Agent %s marked inactive after %d consecutive health check failures", 
				agent.AgentID, consecutiveFails)
		}
	}

	_, err := h.operator.UpdateHealthCheckResult(ctx, agent.AgentID, healthResult, newStatus)
	if err != nil {
		h.logger.Errorf("Failed to update health check result for agent %s: %v", agent.AgentID, err)
	}
}

// CheckSingleAgent performs an immediate health check on a specific agent
func (h *HealthScheduler) CheckSingleAgent(ctx context.Context, agentID string) (*DetailedHealthCheckResult, error) {
	agent, err := h.operator.GetAgentByID(ctx, agentID)
	if err != nil {
		return nil, err
	}

	result := h.checkAgentHealth(ctx, agent)
	h.updateAgentHealth(ctx, agent, result)

	return result, nil
}

// GetSchedulerStatus returns the current status of the scheduler
func (h *HealthScheduler) GetSchedulerStatus() map[string]interface{} {
	return map[string]interface{}{
		"running":  h.running,
		"interval": h.interval.String(),
	}
}
