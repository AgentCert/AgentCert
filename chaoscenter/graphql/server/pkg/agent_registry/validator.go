package agent_registry

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// Validator defines the interface for input validation and business rule enforcement.
type Validator interface {
	// ValidateRegistration validates agent registration input
	ValidateRegistration(ctx context.Context, input *RegisterAgentRequest) error
	
	// ValidateUpdate validates agent update input
	ValidateUpdate(ctx context.Context, input *UpdateAgentRequest) error
	
	// ValidateCapabilities validates capabilities against taxonomy
	ValidateCapabilities(ctx context.Context, capabilities []string) error
	
	// ValidateContainerImage validates container image format
	ValidateContainerImage(ctx context.Context, image *ContainerImage) error
}

// validatorImpl is the concrete implementation of the Validator interface.
type validatorImpl struct {
	operator           Operator
	capabilitiesTaxonomy map[string]bool
	nameRegex          *regexp.Regexp
	semverRegex        *regexp.Regexp
}

// NewValidator creates a new Validator instance.
func NewValidator(operator Operator) Validator {
	return &validatorImpl{
		operator:           operator,
		capabilitiesTaxonomy: loadCapabilitiesTaxonomy(),
		nameRegex:          regexp.MustCompile(AgentNameRegex),
		semverRegex:        regexp.MustCompile(SemverRegex),
	}
}

// ValidateRegistration validates agent registration input.
func (v *validatorImpl) ValidateRegistration(ctx context.Context, input *RegisterAgentRequest) error {
	// Validate name format
	if err := v.validateName(input.Name); err != nil {
		return err
	}
	
	// Check name uniqueness within project
	existingAgent, err := v.operator.GetAgentByProjectAndName(ctx, input.ProjectID, input.Name)
	if err != nil && err != ErrAgentNotFound {
		return fmt.Errorf("failed to check name uniqueness: %w", err)
	}
	if existingAgent != nil {
		return ErrDuplicateAgentName
	}
	
	// Validate version format (semver)
	if err := v.validateVersion(input.Version); err != nil {
		return err
	}
	
	// Validate capabilities not empty
	if len(input.Capabilities) < MinCapabilities {
		return NewValidationError("capabilities", "at least one capability is required")
	}
	
	// Validate capabilities against taxonomy
	if err := v.ValidateCapabilities(ctx, input.Capabilities); err != nil {
		return err
	}
	
	// Validate container image if provided
	if input.ContainerImage != nil {
		if err := v.ValidateContainerImage(ctx, input.ContainerImage); err != nil {
			return err
		}
	}
	
	return nil
}

// ValidateUpdate validates agent update input.
func (v *validatorImpl) ValidateUpdate(ctx context.Context, input *UpdateAgentRequest) error {
	// If name provided, validate format
	if input.Name != nil {
		if err := v.validateName(*input.Name); err != nil {
			return err
		}
	}
	
	// If version provided, validate semver
	if input.Version != nil {
		if err := v.validateVersion(*input.Version); err != nil {
			return err
		}
	}
	
	// If capabilities provided, validate against taxonomy
	if len(input.Capabilities) > 0 {
		if err := v.ValidateCapabilities(ctx, input.Capabilities); err != nil {
			return err
		}
	}
	
	// If container image provided, validate format
	if input.ContainerImage != nil {
		if err := v.ValidateContainerImage(ctx, input.ContainerImage); err != nil {
			return err
		}
	}
	
	return nil
}

// ValidateCapabilities validates capabilities against the taxonomy.
func (v *validatorImpl) ValidateCapabilities(ctx context.Context, capabilities []string) error {
	if len(capabilities) == 0 {
		return NewValidationError("capabilities", "at least one capability is required")
	}
	
	for _, capability := range capabilities {
		if !v.capabilitiesTaxonomy[capability] {
			return NewValidationError("capabilities", 
				fmt.Sprintf("unknown capability '%s'. Must be one of the supported capabilities", capability))
		}
	}
	
	return nil
}

// ValidateContainerImage validates container image format.
func (v *validatorImpl) ValidateContainerImage(ctx context.Context, image *ContainerImage) error {
	if image == nil {
		return NewValidationError("containerImage", "container image is required")
	}
	
	// Validate registry is non-empty and contains dot (domain format)
	if image.Registry == "" {
		return NewValidationError("containerImage.registry", "registry is required")
	}
	if !strings.Contains(image.Registry, ".") {
		return NewValidationError("containerImage.registry", 
			"registry must be a valid domain (e.g., docker.io, gcr.io)")
	}
	
	// Validate repository is non-empty and follows standard format
	if image.Repository == "" {
		return NewValidationError("containerImage.repository", "repository is required")
	}
	// Repository should not contain protocol or registry
	if strings.Contains(image.Repository, "://") {
		return NewValidationError("containerImage.repository", 
			"repository should not contain protocol (e.g., use 'myorg/myimage' not 'https://myorg/myimage')")
	}
	
	// Validate tag is non-empty
	if image.Tag == "" {
		return NewValidationError("containerImage.tag", "tag is required")
	}
	// Tag should not contain invalid characters
	tagRegex := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	if !tagRegex.MatchString(image.Tag) {
		return NewValidationError("containerImage.tag", 
			"tag must contain only alphanumeric characters, dots, hyphens, and underscores")
	}
	
	return nil
}

// validateName validates agent name format.
func (v *validatorImpl) validateName(name string) error {
	if name == "" {
		return NewValidationError("name", "name is required")
	}
	
	if len(name) > MaxAgentNameLength {
		return NewValidationError("name", 
			fmt.Sprintf("name must not exceed %d characters", MaxAgentNameLength))
	}
	
	if !v.nameRegex.MatchString(name) {
		return NewValidationError("name", 
			"name must consist of lowercase alphanumeric characters or '-', "+
			"start with an alphanumeric character, and end with an alphanumeric character "+
			"(e.g., 'my-agent-1', 'agent-name')")
	}
	
	return nil
}

// validateVersion validates semantic version format.
func (v *validatorImpl) validateVersion(version string) error {
	if version == "" {
		return NewValidationError("version", "version is required")
	}
	
	if !v.semverRegex.MatchString(version) {
		return NewValidationError("version", 
			"version must be a valid semantic version (e.g., '1.0.0', 'v2.1.3', '1.0.0-alpha', '1.0.0+build')")
	}
	
	return nil
}

// loadCapabilitiesTaxonomy returns a map of supported capabilities.
// This function can be extended to load from configuration in the future.
func loadCapabilitiesTaxonomy() map[string]bool {
	capabilities := []string{
		// High-level category capabilities (agents may register with these)
		"chaos-engineering",
		"fault-detection",
		"fault-remediation",
		"observability",
		// Pod Chaos capabilities
		"pod-delete",
		"pod-cpu-hog",
		"pod-memory-hog",
		"pod-io-stress",
		"pod-network-latency",
		"pod-network-loss",
		"pod-network-duplication",
		"pod-network-corruption",
		"pod-dns-error",
		"pod-dns-spoof",
		// Node Chaos capabilities
		"node-drain",
		"node-cpu-hog",
		"node-memory-hog",
		"node-io-stress",
		"node-taint",
		"node-restart",
		// Network Chaos capabilities
		"network-latency",
		"network-loss",
		"network-duplication",
		"network-corruption",
		"network-partition",
		// Resource Chaos capabilities
		"disk-fill",
		"disk-io-stress",
		"cpu-stress",
		"memory-stress",
		// Container Chaos capabilities
		"container-kill",
		"container-network-latency",
		"container-network-loss",
		// Remediation capabilities
		"pod-crash-remediation",
		"pod-delete-remediation",
		"node-drain-remediation",
		"network-latency-remediation",
		"network-partition-remediation",
		"disk-pressure-remediation",
		"memory-stress-remediation",
		"cpu-stress-remediation",
		"container-kill-remediation",
		"service-unavailable-remediation",
	}
	
	taxonomy := make(map[string]bool, len(capabilities))
	for _, capability := range capabilities {
		taxonomy[capability] = true
	}
	
	return taxonomy
}
