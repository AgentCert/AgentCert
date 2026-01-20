package langfuse

import (
	"errors"
	"os"
)

// Config holds Langfuse client configuration
type Config struct {
	Enabled   bool
	BaseURL   string
	PublicKey string
	SecretKey string
	ProjectID string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	enabled := os.Getenv("LANGFUSE_ENABLED") == "true"

	if !enabled {
		return &Config{Enabled: false}, nil
	}

	baseURL := os.Getenv("LANGFUSE_BASE_URL")
	if baseURL == "" {
		baseURL = "https://cloud.langfuse.com"
	}

	secretKey := os.Getenv("LANGFUSE_SECRET_KEY")
	if secretKey == "" {
		return nil, errors.New("LANGFUSE_SECRET_KEY is required when Langfuse is enabled")
	}

	publicKey := os.Getenv("LANGFUSE_PUBLIC_KEY")
	if publicKey == "" {
		return nil, errors.New("LANGFUSE_PUBLIC_KEY is required when Langfuse is enabled")
	}

	return &Config{
		Enabled:   true,
		BaseURL:   baseURL,
		PublicKey: publicKey,
		SecretKey: secretKey,
		ProjectID: os.Getenv("LANGFUSE_PROJECT_ID"),
	}, nil
}

// NewConfig creates a new Config with provided values
func NewConfig(baseURL, publicKey, secretKey, projectID string) *Config {
	return &Config{
		Enabled:   true,
		BaseURL:   baseURL,
		PublicKey: publicKey,
		SecretKey: secretKey,
		ProjectID: projectID,
	}
}
