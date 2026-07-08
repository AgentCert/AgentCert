package graph

import (
	"context"
	"os"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/authorization"
)

// getUsername extracts the authenticated username from the request context token.
// Returns an empty string when no token is present or it cannot be decoded.
func getUsername(ctx context.Context) string {
	tkn, ok := ctx.Value(authorization.AuthKey).(string)
	if !ok || tkn == "" {
		return ""
	}
	username, err := authorization.GetUsername(tkn)
	if err != nil {
		return ""
	}
	return username
}

// envOrDefault returns the value of the named environment variable, or
// defaultVal when the variable is unset or empty.
func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
