package agent_registry

// MongoDB collection name
const (
	AgentRegistryCollection = "agent_registry"
)

// AgentStatus represents the lifecycle status of an agent
type AgentStatus string

const (
	AgentStatusRegistered  AgentStatus = "REGISTERED"  // Initial state after registration
	AgentStatusValidating  AgentStatus = "VALIDATING"  // Health check in progress
	AgentStatusActive      AgentStatus = "ACTIVE"      // Passed health check, ready for benchmarks
	AgentStatusInactive    AgentStatus = "INACTIVE"    // Failed health check or manually deactivated
	AgentStatusDeleted     AgentStatus = "DELETED"     // Soft deleted
)

// IsValid checks if the status is a valid AgentStatus
func (s AgentStatus) IsValid() bool {
	switch s {
	case AgentStatusRegistered, AgentStatusValidating, AgentStatusActive, AgentStatusInactive, AgentStatusDeleted:
		return true
	}
	return false
}

// CanTransitionTo checks if a status transition is valid
func (s AgentStatus) CanTransitionTo(target AgentStatus) bool {
	transitions := map[AgentStatus][]AgentStatus{
		AgentStatusRegistered: {AgentStatusValidating, AgentStatusActive, AgentStatusInactive, AgentStatusDeleted},
		AgentStatusValidating: {AgentStatusActive, AgentStatusInactive, AgentStatusDeleted},
		AgentStatusActive:     {AgentStatusValidating, AgentStatusInactive, AgentStatusDeleted},
		AgentStatusInactive:   {AgentStatusValidating, AgentStatusActive, AgentStatusDeleted},
		AgentStatusDeleted:    {}, // Cannot transition from deleted
	}

	allowed, ok := transitions[s]
	if !ok {
		return false
	}

	for _, a := range allowed {
		if a == target {
			return true
		}
	}
	return false
}

// EndpointDiscoveryType defines how the agent endpoint is discovered
type EndpointDiscoveryType string

const (
	EndpointDiscoveryAuto   EndpointDiscoveryType = "AUTO"   // Discover via Kubernetes service
	EndpointDiscoveryManual EndpointDiscoveryType = "MANUAL" // Manually specified endpoint
)

// EndpointType defines the protocol of the agent endpoint
type EndpointType string

const (
	EndpointTypeREST EndpointType = "REST"
	EndpointTypeGRPC EndpointType = "GRPC"
)

// Default configuration values
const (
	DefaultHealthCheckPath      = "/health"
	DefaultHealthCheckInterval  = 60  // seconds
	DefaultHealthCheckTimeout   = 10  // seconds
	DefaultMaxHealthRetries     = 3
	DefaultLangfuseSyncRetries  = 3
	DefaultLangfuseSyncInterval = 300 // seconds (5 minutes)
)

// Predefined capability categories for agents
var CapabilityCategories = map[string][]string{
	"detection": {
		"pod-crash-detection",
		"node-failure-detection",
		"network-latency-detection",
		"resource-exhaustion-detection",
		"service-degradation-detection",
	},
	"diagnosis": {
		"log-analysis",
		"metric-analysis",
		"trace-analysis",
		"root-cause-analysis",
		"dependency-mapping",
	},
	"remediation": {
		"pod-restart",
		"pod-reschedule",
		"deployment-rollback",
		"resource-scaling",
		"config-correction",
		"network-recovery",
	},
	"prevention": {
		"anomaly-prediction",
		"capacity-planning",
		"drift-detection",
		"policy-enforcement",
	},
}

// AllCapabilities returns a flat list of all capabilities
func AllCapabilities() []string {
	var all []string
	for _, caps := range CapabilityCategories {
		all = append(all, caps...)
	}
	return all
}

// IsValidCapability checks if a capability is recognized
func IsValidCapability(capability string) bool {
	for _, caps := range CapabilityCategories {
		for _, c := range caps {
			if c == capability {
				return true
			}
		}
	}
	return false
}
