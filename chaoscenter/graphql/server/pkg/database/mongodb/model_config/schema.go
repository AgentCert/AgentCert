package model_config

import "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"

// SyncStatus is where a model stands between ACE's record and the LiteLLM
// gateway that serves it.
type SyncStatus string

const (
	// Pending: recorded by ACE, not yet confirmed by LiteLLM.
	Pending SyncStatus = "PENDING"
	// Active: LiteLLM serves the model.
	Active SyncStatus = "ACTIVE"
	// Deleting: removal requested, not yet confirmed by LiteLLM.
	Deleting SyncStatus = "DELETING"
	// NeedsKey: LiteLLM lost the model (e.g. its database was reset).
	// Re-entering the API key registers it again.
	NeedsKey SyncStatus = "NEEDS_KEY"
)

// ModelConfig is an LLM model a project onboarded. ACE keeps every parameter
// except the API key, which only LiteLLM holds, encrypted with
// LITELLM_SALT_KEY.
type ModelConfig struct {
	mongodb.Audit `bson:",inline"`
	ProjectID     string `bson:"project_id"`
	// ModelID is also the model's ID in LiteLLM, so the outcome of an
	// interrupted call can be looked up by an ID ACE already knows.
	ModelID string `bson:"model_id"`
	// Alias is the LiteLLM model_name and the agent's MODEL_ALIAS. It is
	// unique across the platform and never reused, so an alias recorded in a
	// certificate always names one model.
	Alias    string `bson:"alias"`
	Label    string `bson:"label"`
	Provider string `bson:"provider"`
	// Model is LiteLLM's litellm_params.model, e.g. "azure/gpt-4o".
	Model      string `bson:"model"`
	APIBase    string `bson:"api_base"`
	APIVersion string `bson:"api_version"`
	// APIKeyHint is the masked key ("****1234"), empty when the model has
	// none. The key itself is never stored.
	APIKeyHint string     `bson:"api_key_hint"`
	SyncStatus SyncStatus `bson:"sync_status"`
	LastError  string     `bson:"last_error"`
}
