package experiment_definition

import "fmt"

// ErrExperimentNotFound is returned when a definition name does not exist.
type ErrExperimentNotFound struct {
	Name string
}

func (e ErrExperimentNotFound) Error() string {
	return fmt.Sprintf("experiment definition not found: %q", e.Name)
}

// ErrDuplicateName is returned when creating a definition with a name that already exists.
type ErrDuplicateName struct {
	Name string
}

func (e ErrDuplicateName) Error() string {
	return fmt.Sprintf("experiment definition already exists: %q", e.Name)
}

// ErrRunNotFound is returned when a run ID does not exist.
type ErrRunNotFound struct {
	RunID string
}

func (e ErrRunNotFound) Error() string {
	return fmt.Sprintf("experiment run not found: %q", e.RunID)
}
