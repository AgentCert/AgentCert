package langfuse

import "fmt"

// LangfuseError represents an API error
type LangfuseError struct {
	StatusCode int
	Message    string
	Details    string
}

func (e *LangfuseError) Error() string {
	return fmt.Sprintf("langfuse error [%d]: %s - %s", e.StatusCode, e.Message, e.Details)
}

// IsRetryable returns true if the error is retryable
func (e *LangfuseError) IsRetryable() bool {
	return e.StatusCode >= 500 || e.StatusCode == 429
}

// IsNotFound returns true if the resource was not found
func (e *LangfuseError) IsNotFound() bool {
	return e.StatusCode == 404
}

// IsUnauthorized returns true if the request was unauthorized
func (e *LangfuseError) IsUnauthorized() bool {
	return e.StatusCode == 401
}

// IsForbidden returns true if the request was forbidden
func (e *LangfuseError) IsForbidden() bool {
	return e.StatusCode == 403
}

// IsRateLimited returns true if the request was rate limited
func (e *LangfuseError) IsRateLimited() bool {
	return e.StatusCode == 429
}

// ErrNotEnabled is returned when Langfuse is disabled
var ErrNotEnabled = fmt.Errorf("langfuse integration is not enabled")

// ErrNotFound is returned when a resource is not found
var ErrNotFound = fmt.Errorf("resource not found")

// ErrUnauthorized is returned when authentication fails
var ErrUnauthorized = fmt.Errorf("unauthorized: invalid API key")

// ErrInvalidPayload is returned when the request payload is invalid
var ErrInvalidPayload = fmt.Errorf("invalid payload")
