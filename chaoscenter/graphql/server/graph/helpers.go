package graph

// Helpers shared by the resolvers. They live here, not in a *.resolvers.go
// file, because gqlgen rewrites those and drops helpers it does not recognize.

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
