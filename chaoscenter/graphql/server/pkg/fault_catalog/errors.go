package fault_catalog

import "fmt"

// ErrFaultNotFound is returned when a fault name does not exist in the catalog.
type ErrFaultNotFound struct {
	Name string
}

func (e ErrFaultNotFound) Error() string {
	return fmt.Sprintf("fault catalog: fault not found: %q", e.Name)
}

// ErrInvalidScope is returned when a scope value is not one of the three valid values.
type ErrInvalidScope struct {
	Scope string
}

func (e ErrInvalidScope) Error() string {
	return fmt.Sprintf("fault catalog: invalid scope %q: must be general, domain, or app-specific", e.Scope)
}

// ErrInvalidFaultYAML is returned when a fault.yaml fails to parse.
type ErrInvalidFaultYAML struct {
	Path string
	Err  error
}

func (e ErrInvalidFaultYAML) Error() string {
	return fmt.Sprintf("fault catalog: failed to parse %s: %v", e.Path, e.Err)
}
