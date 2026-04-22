package agent_registry

// Agent represents an AI agent registered in the platform.
type Agent struct {
	AgentID        string          `bson:"agentId" json:"agentId"`
	ProjectID      string          `bson:"projectId" json:"projectId"`
	Name           string          `bson:"name" json:"name"`
	Version        string          `bson:"version" json:"version"`
	Vendor         string          `bson:"vendor" json:"vendor"`
	Capabilities   []string        `bson:"capabilities" json:"capabilities"`
	ContainerImage *ContainerImage `bson:"containerImage" json:"containerImage"`
	Namespace      string          `bson:"namespace" json:"namespace"`
	HelmReleaseName string          `bson:"helmReleaseName,omitempty" json:"helmReleaseName,omitempty"`
	Endpoint       *AgentEndpoint  `bson:"endpoint" json:"endpoint"`
	LangfuseConfig *LangfuseConfig `bson:"langfuseConfig,omitempty" json:"langfuseConfig,omitempty"`
	Status         AgentStatus     `bson:"status" json:"status"`
	Metadata       *AgentMetadata  `bson:"metadata,omitempty" json:"metadata,omitempty"`
	AuditInfo      *AuditInfo      `bson:"auditInfo" json:"auditInfo"`
}

// ContainerImage represents the container image details for an agent.
type ContainerImage struct {
	Registry   string `bson:"registry" json:"registry"`
	Repository string `bson:"repository" json:"repository"`
	Tag        string `bson:"tag" json:"tag"`
}

// AgentEndpoint represents the endpoint configuration for an agent.
type AgentEndpoint struct {
	URL           string               `bson:"url" json:"url"`
	Type          EndpointType         `bson:"type" json:"type"`
	DiscoveryType EndpointDiscoveryType `bson:"discoveryType" json:"discoveryType"`
	HealthPath    string               `bson:"healthPath" json:"healthPath"`
	ReadyPath     string               `bson:"readyPath" json:"readyPath"`
}

// LangfuseConfig represents Langfuse integration configuration.
type LangfuseConfig struct {
	ProjectID    string `bson:"projectId" json:"projectId"`
	SyncEnabled  bool   `bson:"syncEnabled" json:"syncEnabled"`
	LastSyncedAt *int64 `bson:"lastSyncedAt,omitempty" json:"lastSyncedAt,omitempty"`
}

// AgentMetadata represents additional metadata for an agent.
type AgentMetadata struct {
	Labels      map[string]string `bson:"labels,omitempty" json:"labels,omitempty"`
	Annotations map[string]string `bson:"annotations,omitempty" json:"annotations,omitempty"`
}

// AuditInfo represents audit information for an agent.
type AuditInfo struct {
	CreatedAt       int64  `bson:"createdAt" json:"createdAt"`
	CreatedBy       string `bson:"createdBy" json:"createdBy"`
	UpdatedAt       int64  `bson:"updatedAt" json:"updatedAt"`
	UpdatedBy       string `bson:"updatedBy" json:"updatedBy"`
	LastHealthCheck *int64 `bson:"lastHealthCheck,omitempty" json:"lastHealthCheck,omitempty"`
}

// AgentStatus represents the current status of an agent.
type AgentStatus string

const (
	AgentStatusRegistered AgentStatus = "REGISTERED"
	AgentStatusValidating AgentStatus = "VALIDATING"
	AgentStatusActive     AgentStatus = "ACTIVE"
	AgentStatusInactive   AgentStatus = "INACTIVE"
	AgentStatusDeleted    AgentStatus = "DELETED"
)

// EndpointDiscoveryType represents how the agent endpoint was discovered.
type EndpointDiscoveryType string

const (
	EndpointDiscoveryAuto   EndpointDiscoveryType = "AUTO"
	EndpointDiscoveryManual EndpointDiscoveryType = "MANUAL"
)

// EndpointType represents the type of agent endpoint.
type EndpointType string

const (
	EndpointTypeREST EndpointType = "REST"
	EndpointTypeGRPC EndpointType = "GRPC"
)
