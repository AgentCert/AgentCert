package agent_registry

import (
	"errors"
	"fmt"
)

var (
	// ErrAgentNotFound is returned when an agent is not found
	ErrAgentNotFound = errors.New("agent not found")

	// ErrDuplicateAgentName is returned when an agent name already exists in the project
	ErrDuplicateAgentName = errors.New("agent name already exists in project")

	// ErrInvalidAgentName is returned when an agent name format is invalid
	ErrInvalidAgentName = errors.New("invalid agent name format")

	// ErrInvalidVersion is returned when a version format is invalid
	ErrInvalidVersion = errors.New("invalid version format")

	// ErrInvalidCapabilities is returned when capabilities are invalid
	ErrInvalidCapabilities = errors.New("invalid capabilities")

	// ErrInvalidContainerImage is returned when container image format is invalid
	ErrInvalidContainerImage = errors.New("invalid container image format")

	// ErrHealthCheckFailed is returned when health check fails
	ErrHealthCheckFailed = errors.New("health check failed")

	// ErrLangfuseSyncFailed is returned when Langfuse synchronization fails
	ErrLangfuseSyncFailed = errors.New("langfuse synchronization failed")

	// ErrUnauthorized is returned when user is not authenticated
	ErrUnauthorized = errors.New("unauthorized")

	// ErrInsufficientPermissions is returned when user lacks required permissions
	ErrInsufficientPermissions = errors.New("insufficient permissions")
)

// ValidationError represents a validation error with details
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// NewValidationError creates a new ValidationError
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
