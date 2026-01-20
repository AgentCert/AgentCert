package agent_registry

import (
	"errors"
	"fmt"
)

// Common errors
var (
	ErrAgentNotFound             = errors.New("agent not found")
	ErrAgentAlreadyExists        = errors.New("agent with this name already exists in the project")
	ErrInvalidAgentID            = errors.New("invalid agent ID")
	ErrInvalidProjectID          = errors.New("project ID is required")
	ErrInvalidAgentName          = errors.New("agent name is required and must be between 3 and 100 characters")
	ErrInvalidVersion            = errors.New("version is required and must follow semantic versioning")
	ErrInvalidCapabilities       = errors.New("at least one capability is required")
	ErrInvalidStatus             = errors.New("invalid agent status")
	ErrInvalidTransition         = errors.New("invalid status transition")
	ErrHealthCheckFailed         = errors.New("agent health check failed")
	ErrLangfuseSyncFailed        = errors.New("failed to sync agent to Langfuse")
	ErrAgentDeleted              = errors.New("agent has been deleted")
	ErrHealthSchedulerNotRunning = errors.New("health scheduler is not running")
	ErrLangfuseNotEnabled        = errors.New("langfuse integration is not enabled")
)

// ValidationError represents a validation error with field details
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// MultiValidationError represents multiple validation errors
type MultiValidationError struct {
	Errors []*ValidationError
}

func (e *MultiValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	return fmt.Sprintf("validation failed with %d errors: %s", len(e.Errors), e.Errors[0].Error())
}

// Add adds a validation error
func (e *MultiValidationError) Add(field, message string) {
	e.Errors = append(e.Errors, NewValidationError(field, message))
}

// HasErrors returns true if there are validation errors
func (e *MultiValidationError) HasErrors() bool {
	return len(e.Errors) > 0
}

// OperationError represents an error during a service operation
type OperationError struct {
	Operation string
	AgentID   string
	Err       error
}

func (e *OperationError) Error() string {
	if e.AgentID != "" {
		return fmt.Sprintf("operation '%s' failed for agent '%s': %v", e.Operation, e.AgentID, e.Err)
	}
	return fmt.Sprintf("operation '%s' failed: %v", e.Operation, e.Err)
}

func (e *OperationError) Unwrap() error {
	return e.Err
}

// NewOperationError creates a new operation error
func NewOperationError(operation, agentID string, err error) *OperationError {
	return &OperationError{
		Operation: operation,
		AgentID:   agentID,
		Err:       err,
	}
}

// NotFoundError represents a resource not found error
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s with ID '%s' not found", e.Resource, e.ID)
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(resource, id string) *NotFoundError {
	return &NotFoundError{
		Resource: resource,
		ID:       id,
	}
}

// IsNotFoundError checks if an error is a NotFoundError
func IsNotFoundError(err error) bool {
	var notFound *NotFoundError
	return errors.As(err, &notFound) || errors.Is(err, ErrAgentNotFound)
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	var validationErr *ValidationError
	var multiValidationErr *MultiValidationError
	return errors.As(err, &validationErr) || errors.As(err, &multiValidationErr)
}
