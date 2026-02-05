package langfuse

import (
	"context"
	"time"

	"github.com/99designs/gqlgen/graphql"
)

// GraphQLMiddleware creates a gqlgen middleware for tracing GraphQL operations
type GraphQLMiddleware struct {
	client *Client
}

// NewGraphQLMiddleware creates a new GraphQL middleware for Langfuse tracing
func NewGraphQLMiddleware(client *Client) *GraphQLMiddleware {
	return &GraphQLMiddleware{
		client: client,
	}
}

// ExtensionName returns the extension name for gqlgen
func (m *GraphQLMiddleware) ExtensionName() string {
	return "LangfuseTracing"
}

// Validate validates the schema (required by gqlgen interface)
func (m *GraphQLMiddleware) Validate(schema graphql.ExecutableSchema) error {
	return nil
}

// InterceptOperation traces the entire GraphQL operation
func (m *GraphQLMiddleware) InterceptOperation(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
	if !m.client.IsEnabled() {
		return next(ctx)
	}

	oc := graphql.GetOperationContext(ctx)
	operationName := oc.OperationName
	if operationName == "" {
		operationName = "anonymous"
	}

	// Create trace for this operation
	trace := m.client.CreateTrace(
		operationName,
		"", // userID can be extracted from context if available
		map[string]interface{}{
			"operationType": string(oc.Operation.Operation),
			"rawQuery":      oc.RawQuery,
		},
		map[string]interface{}{
			"query":     oc.RawQuery,
			"variables": oc.Variables,
		},
	)

	// Store trace in context
	ctx = context.WithValue(ctx, TraceContextKey, trace)

	return next(ctx)
}

// InterceptResponse traces the GraphQL response
func (m *GraphQLMiddleware) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	if !m.client.IsEnabled() {
		return next(ctx)
	}

	resp := next(ctx)

	// Get trace from context
	if traceVal := ctx.Value(TraceContextKey); traceVal != nil {
		if trace, ok := traceVal.(*Trace); ok {
			output := map[string]interface{}{
				"hasErrors": len(resp.Errors) > 0,
			}
			if len(resp.Errors) > 0 {
				errors := make([]string, len(resp.Errors))
				for i, err := range resp.Errors {
					errors[i] = err.Message
				}
				output["errors"] = errors
			}
			trace.End(output)
		}
	}

	return resp
}

// InterceptField traces individual field resolutions (optional, can be expensive)
func (m *GraphQLMiddleware) InterceptField(ctx context.Context, next graphql.Resolver) (interface{}, error) {
	if !m.client.IsEnabled() {
		return next(ctx)
	}

	fc := graphql.GetFieldContext(ctx)

	// Only trace root-level fields to avoid too much noise
	if fc.Object != "Query" && fc.Object != "Mutation" && fc.Object != "Subscription" {
		return next(ctx)
	}

	// Get trace from context
	var span *Span
	if traceVal := ctx.Value(TraceContextKey); traceVal != nil {
		if trace, ok := traceVal.(*Trace); ok {
			span = trace.CreateSpan(
				fc.Field.Name,
				map[string]interface{}{
					"object":    fc.Object,
					"fieldName": fc.Field.Name,
				},
				fc.Args,
			)
		}
	}

	startTime := time.Now()
	result, err := next(ctx)
	duration := time.Since(startTime)

	if span != nil {
		status := "success"
		if err != nil {
			status = "error: " + err.Error()
		}
		span.End(map[string]interface{}{
			"duration_ms": duration.Milliseconds(),
		}, status)
	}

	return result, err
}

// GetTraceFromContext retrieves the Langfuse trace from context
func GetTraceFromContext(ctx context.Context) *Trace {
	if traceVal := ctx.Value(TraceContextKey); traceVal != nil {
		if trace, ok := traceVal.(*Trace); ok {
			return trace
		}
	}
	return nil
}

// TraceFunction is a helper to trace a function call within a GraphQL operation
func TraceFunction(ctx context.Context, name string, input interface{}, fn func() (interface{}, error)) (interface{}, error) {
	trace := GetTraceFromContext(ctx)
	if trace == nil || !trace.client.IsEnabled() {
		return fn()
	}

	span := trace.CreateSpan(name, nil, input)
	startTime := time.Now()
	
	result, err := fn()
	
	duration := time.Since(startTime)
	status := "success"
	if err != nil {
		status = "error: " + err.Error()
	}
	
	span.End(map[string]interface{}{
		"result":      result,
		"duration_ms": duration.Milliseconds(),
	}, status)

	return result, err
}

// TraceLLMCall traces an LLM/AI call within a GraphQL operation
func TraceLLMCall(ctx context.Context, name string, model string, input interface{}, fn func() (interface{}, int, int, error)) (interface{}, error) {
	trace := GetTraceFromContext(ctx)
	if trace == nil || !trace.client.IsEnabled() {
		result, _, _, err := fn()
		return result, err
	}

	gen := trace.CreateGeneration(name, model, input, nil)
	
	result, promptTokens, completionTokens, err := fn()
	
	gen.End(result, promptTokens, completionTokens)

	return result, err
}
