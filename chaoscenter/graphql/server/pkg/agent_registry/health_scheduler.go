package agent_registry

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// HealthCheckScheduler manages periodic health checks for registered agents.
type HealthCheckScheduler struct {
	service  Service
	interval time.Duration
	stopChan chan struct{}
	running  sync.WaitGroup
	// logger will be added for structured logging
}

// NewHealthCheckScheduler creates a new HealthCheckScheduler instance.
func NewHealthCheckScheduler(service Service, interval time.Duration) *HealthCheckScheduler {
	if interval == 0 {
		interval = 5 * time.Minute // Default to 5 minutes
	}

	return &HealthCheckScheduler{
		service:  service,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

// Start begins the health check scheduler loop.
func (s *HealthCheckScheduler) Start(ctx context.Context) {
	fmt.Printf("Starting health check scheduler with interval %v\n", s.interval)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.running.Add(1)
			s.runHealthChecks(ctx)
			s.running.Done()
		case <-s.stopChan:
			fmt.Println("Health check scheduler stopping...")
			return
		case <-ctx.Done():
			fmt.Println("Health check scheduler context cancelled")
			return
		}
	}
}

// Stop signals the scheduler to stop gracefully and waits for current cycle to complete.
func (s *HealthCheckScheduler) Stop() {
	close(s.stopChan)
	// Wait for current health check cycle to complete
	s.running.Wait()
	fmt.Println("Health check scheduler stopped")
}

// runHealthChecks executes health checks for all active agents with concurrent worker pool.
func (s *HealthCheckScheduler) runHealthChecks(ctx context.Context) {
	fmt.Println("Running health checks for agents...")

	// Fetch all agents with status ACTIVE or VALIDATING
	filter := &AgentFilter{
		Statuses: []AgentStatus{AgentStatusActive, AgentStatusValidating},
	}

	pagination := &PaginationInput{
		Page:  1,
		Limit: 1000, // Fetch up to 1000 agents per cycle
	}

	response, err := s.service.ListAgents(ctx, filter, pagination)
	if err != nil {
		fmt.Printf("Failed to fetch agents for health check: %v\n", err)
		return
	}

	if len(response.Agents) == 0 {
		fmt.Println("No agents to check")
		return
	}

	fmt.Printf("Checking health for %d agents\n", len(response.Agents))

	// Use semaphore pattern to limit concurrent checks to 10
	semaphore := make(chan struct{}, 10)
	var wg sync.WaitGroup

	successCount := 0
	failedCount := 0
	errorCount := 0
	var mu sync.Mutex // Protect counters

	for _, agent := range response.Agents {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire semaphore

		go func(agentID string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore

			// Perform health check
			result, err := s.service.ValidateAgentHealth(ctx, agentID)
			if err != nil {
				mu.Lock()
				errorCount++
				mu.Unlock()
				fmt.Printf("Error checking health for agent %s: %v\n", agentID, err)
				return
			}

			mu.Lock()
			if result.Healthy {
				successCount++
			} else {
				failedCount++
			}
			mu.Unlock()

			fmt.Printf("Health check for agent %s: healthy=%v, message=%s, responseTime=%dms\n",
				agentID, result.Healthy, result.Message, result.ResponseTime)
		}(agent.AgentID)
	}

	// Wait for all health checks to complete
	wg.Wait()

	fmt.Printf("Health check cycle completed: %d successful, %d failed, %d errors\n",
		successCount, failedCount, errorCount)
}
