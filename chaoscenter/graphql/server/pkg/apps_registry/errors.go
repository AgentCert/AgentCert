package apps_registry

import "errors"

var (
	// ErrAppNotFound is returned when an app is not found in the database.
	ErrAppNotFound = errors.New("app not found")

	// ErrDuplicateAppName is returned when attempting to register an app with a name that already exists.
	ErrDuplicateAppName = errors.New("app with this name already exists in the project")

	// ErrInvalidInput is returned when the input validation fails.
	ErrInvalidInput = errors.New("invalid input")
)
