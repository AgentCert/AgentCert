package model_library

import "fmt"

type ErrAliasNotFound struct{ Alias string }

func (e ErrAliasNotFound) Error() string { return fmt.Sprintf("model config '%s' not found", e.Alias) }

type ErrDuplicateAlias struct{ Alias string }

func (e ErrDuplicateAlias) Error() string {
	return fmt.Sprintf("model config alias '%s' already exists", e.Alias)
}

type ErrAliasInUse struct {
	Alias  string
	Agents []string
}

func (e ErrAliasInUse) Error() string {
	return fmt.Sprintf("cannot delete '%s' — referenced by agents: %v", e.Alias, e.Agents)
}
