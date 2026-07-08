package agent_registry

// Agent represents an AI agent registered in the platform.
type Agent struct {
	AgentID         string          `bson:"agentId" json:"agentId"`
	ProjectID       string          `bson:"projectId" json:"projectId"`
	Name            string          `bson:"name" json:"name"`
	Version         string          `bson:"version" json:"version"`
	Vendor          string          `bson:"vendor" json:"vendor"`
	Capabilities    []string        `bson:"capabilities" json:"capabilities"`
	ContainerImage  *ContainerImage `bson:"containerImage" json:"containerImage"`
	Namespace       string          `bson:"namespace" json:"namespace"`
	HelmReleaseName string          `bson:"helmReleaseName,omitempty" json:"helmReleaseName,omitempty"`
	Endpoint        *AgentEndpoint  `bson:"endpoint" json:"endpoint"`
	LangfuseConfig  *LangfuseConfig `bson:"langfuseConfig,omitempty" json:"langfuseConfig,omitempty"`
	Status          AgentStatus     `bson:"status" json:"status"`
	Metadata        *AgentMetadata  `bson:"metadata,omitempty" json:"metadata,omitempty"`
	AuditInfo       *AuditInfo      `bson:"auditInfo" json:"auditInfo"`

	// Spec-aligned fields (Stage 1)
	DisplayName      string
	Tier             string
	SpecDescription  *AgentSpecDescription
	SpecInstall      *AgentSpecInstall
	AgentLLMConfig   *AgentLLMConfig
	AgentInputDefs   []AgentInputDef
	ContextInjection []ContextInjectDef
	RequiredTools    []RequiredToolDef
	EvalMetrics      []string
	Compatibility    *AgentCompatibility
	AgentOwner       *AgentOwnerInfo
	Repository       string
	License          string
	SchemaVersion    string
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

// AgentSpecDescription holds description fields from the spec.
type AgentSpecDescription struct {
	Short        string
	Long         string
	Approach     string
	LLMDependent bool
}

// AgentSpecInstall holds install configuration from the spec.
type AgentSpecInstall struct {
	Method    string
	Image     string
	Folder    string
	Namespace string
	Timeout   string
	CPU       string
	Memory    string
	Replicas  int
}

// AgentLLMConfig holds LLM configuration from the spec.
type AgentLLMConfig struct {
	ConfigRef       string
	Provider        string
	Model           string
	AllowUserChoice bool
	AllowedModels   []string
	DefaultModel    string
	LLMDependent    bool
}

// AgentInputDef represents a single configurable input parameter for an agent.
type AgentInputDef struct {
	Key         string
	DisplayName string
	Description string
	Type        string
	Required    bool
	Default     string
	Placeholder string
	HelmPath    string
	Values      []string
	Min         *int
	Max         *int
	Unit        string
	Advanced    bool
	Group       string
}

// ContextInjectDef represents a context injection entry from the spec.
type ContextInjectDef struct {
	HelmPath    string
	Source      string
	Required    bool
	Description string
}

// RequiredToolDef represents a required MCP tool from the spec.
type RequiredToolDef struct {
	Name         string
	Purpose      string
	Critical     bool
	MinCallCount int
	MaxCallCount *int
}

// AgentCompatibility holds compatibility information from the spec.
type AgentCompatibility struct {
	SupportedApps   []string
	UnsupportedApps []string
	MinFaultCount   int
	MaxFaultCount   int
}

// AgentOwnerInfo holds owner information from the spec.
type AgentOwnerInfo struct {
	Name  string
	Email string
	Org   string
}
