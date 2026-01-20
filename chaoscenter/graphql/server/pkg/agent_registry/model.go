package agent_registry

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Agent represents an AI agent registered in AgentCert
type Agent struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	AgentID         string             `bson:"agentId" json:"agentId"`                   // Unique identifier (UUID)
	ProjectID       string             `bson:"projectId" json:"projectId"`               // Associated project
	Name            string             `bson:"name" json:"name"`                         // Human-readable name
	Description     string             `bson:"description,omitempty" json:"description"` // Agent description
	Vendor          string             `bson:"vendor,omitempty" json:"vendor"`           // Vendor/creator name
	Version         string             `bson:"version" json:"version"`                   // Semantic version
	Capabilities    []string           `bson:"capabilities" json:"capabilities"`         // List of capabilities
	ContainerImage  *ContainerImage    `bson:"containerImage,omitempty" json:"containerImage"`
	Endpoint        *AgentEndpoint     `bson:"endpoint,omitempty" json:"endpoint"`
	Namespace       string             `bson:"namespace,omitempty" json:"namespace"`       // Kubernetes namespace
	ServiceAccount  string             `bson:"serviceAccount,omitempty" json:"serviceAccount"` // K8s service account
	Status          AgentStatus        `bson:"status" json:"status"`
	HealthCheck     *HealthCheckConfig `bson:"healthCheck,omitempty" json:"healthCheck"`
	LastHealthCheck *HealthCheckResult `bson:"lastHealthCheck,omitempty" json:"lastHealthCheck"`
	LangfuseConfig  *LangfuseConfig    `bson:"langfuseConfig,omitempty" json:"langfuseConfig"`
	Metadata        map[string]string  `bson:"metadata,omitempty" json:"metadata"` // Custom key-value pairs
	Tags            []string           `bson:"tags,omitempty" json:"tags"`         // Searchable tags
	AuditInfo       AuditInfo          `bson:"auditInfo" json:"auditInfo"`
}

// ContainerImage represents the container image details of an agent
type ContainerImage struct {
	Registry   string   `bson:"registry,omitempty" json:"registry"`     // Container registry (e.g., docker.io)
	Repository string   `bson:"repository" json:"repository"`           // Image repository
	Tag        string   `bson:"tag" json:"tag"`                         // Image tag
	Digest     string   `bson:"digest,omitempty" json:"digest"`         // Image digest (sha256:...)
	PullPolicy string   `bson:"pullPolicy,omitempty" json:"pullPolicy"` // Always, IfNotPresent, Never
	PullSecrets []string `bson:"pullSecrets,omitempty" json:"pullSecrets"` // Image pull secrets
}

// FullImageName returns the complete image name with registry, repository, and tag
func (c *ContainerImage) FullImageName() string {
	name := c.Repository
	if c.Registry != "" {
		name = c.Registry + "/" + name
	}
	if c.Digest != "" {
		name = name + "@" + c.Digest
	} else if c.Tag != "" {
		name = name + ":" + c.Tag
	}
	return name
}

// AgentEndpoint represents the network endpoint of an agent
type AgentEndpoint struct {
	DiscoveryType EndpointDiscoveryType `bson:"discoveryType" json:"discoveryType"`
	EndpointType  EndpointType          `bson:"endpointType" json:"endpointType"`
	URL           string                `bson:"url,omitempty" json:"url"`                   // For manual discovery
	ServiceName   string                `bson:"serviceName,omitempty" json:"serviceName"`   // For auto discovery
	Port          int                   `bson:"port,omitempty" json:"port"`                 // Service port
	TLSEnabled    bool                  `bson:"tlsEnabled" json:"tlsEnabled"`               // Whether TLS is enabled
	CertSecretRef string                `bson:"certSecretRef,omitempty" json:"certSecretRef"` // K8s secret for TLS certs
}

// HealthCheckConfig defines health check settings for an agent
type HealthCheckConfig struct {
	Enabled       bool   `bson:"enabled" json:"enabled"`
	Path          string `bson:"path" json:"path"`                   // Health check endpoint path
	IntervalSec   int    `bson:"intervalSec" json:"intervalSec"`     // Check interval in seconds
	TimeoutSec    int    `bson:"timeoutSec" json:"timeoutSec"`       // Request timeout in seconds
	MaxRetries    int    `bson:"maxRetries" json:"maxRetries"`       // Max consecutive failures before marking inactive
	SuccessThresh int    `bson:"successThresh" json:"successThresh"` // Successes needed to mark active
}

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	Timestamp     time.Time `bson:"timestamp" json:"timestamp"`
	Success       bool      `bson:"success" json:"success"`
	StatusCode    int       `bson:"statusCode,omitempty" json:"statusCode"`
	ResponseTime  int64     `bson:"responseTimeMs" json:"responseTimeMs"` // Response time in milliseconds
	Message       string    `bson:"message,omitempty" json:"message"`
	ConsecFails   int       `bson:"consecFails" json:"consecFails"`       // Consecutive failures
	ConsecSuccess int       `bson:"consecSuccess" json:"consecSuccess"`   // Consecutive successes
}

// LangfuseConfig holds Langfuse sync configuration for an agent
type LangfuseConfig struct {
	Enabled       bool      `bson:"enabled" json:"enabled"`
	SyncedAt      time.Time `bson:"syncedAt,omitempty" json:"syncedAt"`
	LangfuseID    string    `bson:"langfuseId,omitempty" json:"langfuseId"` // User ID in Langfuse
	SyncStatus    string    `bson:"syncStatus,omitempty" json:"syncStatus"` // SYNCED, PENDING, FAILED
	LastSyncError string    `bson:"lastSyncError,omitempty" json:"lastSyncError"`
}

// AuditInfo contains creation and modification timestamps
type AuditInfo struct {
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	CreatedBy string    `bson:"createdBy,omitempty" json:"createdBy"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
	UpdatedBy string    `bson:"updatedBy,omitempty" json:"updatedBy"`
}

// RegisterAgentInput is the input for registering a new agent
type RegisterAgentInput struct {
	ProjectID      string                 `json:"projectId"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description,omitempty"`
	Vendor         string                 `json:"vendor,omitempty"`
	Version        string                 `json:"version"`
	Capabilities   []string               `json:"capabilities"`
	ContainerImage *ContainerImageInput   `json:"containerImage,omitempty"`
	Endpoint       *AgentEndpointInput    `json:"endpoint,omitempty"`
	Namespace      string                 `json:"namespace,omitempty"`
	ServiceAccount string                 `json:"serviceAccount,omitempty"`
	HealthCheck    *HealthCheckConfigInput `json:"healthCheck,omitempty"`
	Metadata       map[string]string      `json:"metadata,omitempty"`
	Tags           []string               `json:"tags,omitempty"`
	EnableLangfuse bool                   `json:"enableLangfuse"`
}

// ContainerImageInput is the input for container image details
type ContainerImageInput struct {
	Registry    string   `json:"registry,omitempty"`
	Repository  string   `json:"repository"`
	Tag         string   `json:"tag"`
	Digest      string   `json:"digest,omitempty"`
	PullPolicy  string   `json:"pullPolicy,omitempty"`
	PullSecrets []string `json:"pullSecrets,omitempty"`
}

// AgentEndpointInput is the input for agent endpoint configuration
type AgentEndpointInput struct {
	DiscoveryType string `json:"discoveryType"` // AUTO or MANUAL
	EndpointType  string `json:"endpointType"`  // REST or GRPC
	URL           string `json:"url,omitempty"`
	ServiceName   string `json:"serviceName,omitempty"`
	Port          int    `json:"port,omitempty"`
	TLSEnabled    bool   `json:"tlsEnabled"`
	CertSecretRef string `json:"certSecretRef,omitempty"`
}

// HealthCheckConfigInput is the input for health check configuration
type HealthCheckConfigInput struct {
	Enabled       bool   `json:"enabled"`
	Path          string `json:"path,omitempty"`
	IntervalSec   int    `json:"intervalSec,omitempty"`
	TimeoutSec    int    `json:"timeoutSec,omitempty"`
	MaxRetries    int    `json:"maxRetries,omitempty"`
	SuccessThresh int    `json:"successThresh,omitempty"`
}

// UpdateAgentInput is the input for updating an existing agent
type UpdateAgentInput struct {
	Description    *string                 `json:"description,omitempty"`
	Version        *string                 `json:"version,omitempty"`
	Capabilities   []string                `json:"capabilities,omitempty"`
	ContainerImage *ContainerImageInput    `json:"containerImage,omitempty"`
	Endpoint       *AgentEndpointInput     `json:"endpoint,omitempty"`
	HealthCheck    *HealthCheckConfigInput `json:"healthCheck,omitempty"`
	Metadata       map[string]string       `json:"metadata,omitempty"`
	Tags           []string                `json:"tags,omitempty"`
	EnableLangfuse *bool                   `json:"enableLangfuse,omitempty"`
}

// ListAgentsFilter defines filters for listing agents
type ListAgentsFilter struct {
	ProjectID    string       `json:"projectId,omitempty"`
	Status       *AgentStatus `json:"status,omitempty"`
	Capabilities []string     `json:"capabilities,omitempty"`
	Vendor       string       `json:"vendor,omitempty"`
	Tags         []string     `json:"tags,omitempty"`
	Search       string       `json:"search,omitempty"` // Search in name, description
}

// ListAgentsPagination defines pagination for listing agents
type ListAgentsPagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// ListAgentsResponse is the response for listing agents
type ListAgentsResponse struct {
	Agents     []*Agent `json:"agents"`
	TotalCount int64    `json:"totalCount"`
}
