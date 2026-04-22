package agent_registry

import (
	"time"
)

const (
	// MongoDB collection name
	AgentRegistryCollection = "agent_registry_collection"

	// Default endpoint paths
	DefaultHealthPath = "/health"
	DefaultReadyPath  = "/ready"

	// Default timeouts
	DefaultHealthCheckTimeout = 5 * time.Second
	DefaultHTTPClientTimeout  = 10 * time.Second

	// Retry configuration
	MaxRetries          = 3
	InitialRetryDelay   = 1 * time.Second
	RetryDelayMultiplier = 2

	// Concurrent health check limit
	MaxConcurrentHealthChecks = 10

	// Validation constraints
	MaxAgentNameLength = 63
	MinCapabilities    = 1

	// Default health check interval
	DefaultHealthCheckInterval = 5 * time.Minute
)

// AgentNameRegex is the regex pattern for valid agent names
const AgentNameRegex = `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`

// SemverRegex is the regex pattern for semantic versioning
const SemverRegex = `^v?(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9.-]+))?(?:\+([a-zA-Z0-9.-]+))?$`
