package agent_registry_db

import "go.mongodb.org/mongo-driver/bson/primitive"

const CollectionName = "agent_registry_collection"

type AgentDocument struct {
	ID            primitive.ObjectID `bson:"_id"`
	AgentID       string             `bson:"agentID"`
	ProjectID     string             `bson:"projectID"`
	Tier          string             `bson:"tier"`
	Name          string             `bson:"name"`
	DisplayName   string             `bson:"displayName"`
	Version       string             `bson:"version"`
	Description   AgentDescDoc       `bson:"description"`
	Install       AgentInstallDoc    `bson:"install"`
	LLMConfig     *LLMConfigDoc      `bson:"llmConfig,omitempty"`
	Inputs        []AgentInputDoc    `bson:"inputs"`
	ContextInject []ContextInjectDoc `bson:"contextInjection"`
	Capabilities  []string           `bson:"capabilities"`
	RequiredTools []RequiredToolDoc  `bson:"requiredTools"`
	EvalMetrics   []string           `bson:"evaluationMetrics"`
	Compatibility AgentCompatDoc     `bson:"compatibility"`
	Owner         AgentOwnerDoc      `bson:"owner"`
	Tags          []string           `bson:"tags"`
	Repository    string             `bson:"repository,omitempty"`
	License       string             `bson:"license,omitempty"`
	IsDeleted     bool               `bson:"isDeleted"`
	CreatedAt     int64              `bson:"createdAt"`
	UpdatedAt     int64              `bson:"updatedAt"`
	SchemaVersion string             `bson:"schemaVersion"`
}

type AgentDescDoc struct {
	Short        string `bson:"short"`
	Long         string `bson:"long"`
	Approach     string `bson:"approach,omitempty"`
	LLMDependent bool   `bson:"llmDependent"`
}

type AgentInstallDoc struct {
	Method    string `bson:"method"`
	Image     string `bson:"image,omitempty"`
	Folder    string `bson:"folder,omitempty"`
	Namespace string `bson:"namespace"`
	Timeout   string `bson:"timeout"`
	CPU       string `bson:"cpu,omitempty"`
	Memory    string `bson:"memory,omitempty"`
	Replicas  int    `bson:"replicas"`
}

type LLMConfigDoc struct {
	ConfigRef       string   `bson:"configRef,omitempty"`
	Provider        string   `bson:"provider,omitempty"`
	Model           string   `bson:"model,omitempty"`
	AllowUserChoice bool     `bson:"allowUserChoice"`
	AllowedModels   []string `bson:"allowedModels"`
	DefaultModel    string   `bson:"defaultModel,omitempty"`
	LLMDependent    bool     `bson:"llmDependent"`
}

type AgentInputDoc struct {
	Key         string   `bson:"key"`
	DisplayName string   `bson:"displayName"`
	Description string   `bson:"description,omitempty"`
	Type        string   `bson:"type"`
	Required    bool     `bson:"required"`
	Default     string   `bson:"default,omitempty"`
	Placeholder string   `bson:"placeholder,omitempty"`
	HelmPath    string   `bson:"helmPath"`
	Values      []string `bson:"values,omitempty"`
	Min         *int     `bson:"min,omitempty"`
	Max         *int     `bson:"max,omitempty"`
	Unit        string   `bson:"unit,omitempty"`
	Advanced    bool     `bson:"advanced"`
	Group       string   `bson:"group,omitempty"`
}

type ContextInjectDoc struct {
	HelmPath    string `bson:"helmPath"`
	Source      string `bson:"source"`
	Required    bool   `bson:"required"`
	Description string `bson:"description,omitempty"`
}

type RequiredToolDoc struct {
	Name         string `bson:"name"`
	Purpose      string `bson:"purpose,omitempty"`
	Critical     bool   `bson:"critical"`
	MinCallCount int    `bson:"minCallCount"`
	MaxCallCount *int   `bson:"maxCallCount,omitempty"`
}

type AgentCompatDoc struct {
	SupportedApps   []string `bson:"supportedApps"`
	UnsupportedApps []string `bson:"unsupportedApps"`
	MinFaultCount   int      `bson:"minimumFaultCount"`
	MaxFaultCount   int      `bson:"maximumFaultCount"`
}

type AgentOwnerDoc struct {
	Name  string `bson:"name"`
	Email string `bson:"email"`
	Org   string `bson:"org,omitempty"`
}
