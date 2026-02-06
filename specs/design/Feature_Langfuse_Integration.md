# Feature: Langfuse Observability Integration
## Design Document

**Document Version**: 1.0  
**Date**: February 6, 2026

---

## 1. Prerequisites

### 1.1 Langfuse Server
**Langfuse must be running and accessible before starting the GraphQL server.**

- Default URL: `http://localhost:3002`
- Langfuse can be deployed via Docker or self-hosted
- Ensure ClickHouse and PostgreSQL backends are healthy

### 1.2 API Keys
Obtain API keys from Langfuse dashboard:
1. Open Langfuse UI (`http://localhost:3002`)
2. Navigate to **Settings → API Keys**
3. Create or copy existing Public Key and Secret Key
4. Configure in `.env` file (see Section 5)

### 1.3 Verify Langfuse Health
```bash
curl http://localhost:3002/api/public/health
# Expected: {"status":"OK","version":"x.x.x"}
```

---

## 2. Overview

### 2.1 Purpose
Integrate Langfuse observability tracing into the LitmusChaos GraphQL server to capture and monitor all GraphQL operations for debugging, performance analysis, and audit purposes.

### 2.2 Scope
- GraphQL operation tracing (queries, mutations, subscriptions)
- Automatic trace creation for each request
- Span tracking for field-level resolution
- Configuration via environment variables

---

## 3. Architecture

### 3.1 Integration Flow

```
GraphQL Request
      ↓
Langfuse Middleware (InterceptOperation)
      ↓ Creates Trace
GraphQL Resolver Execution
      ↓ Creates Spans for root fields
Langfuse Middleware (InterceptResponse)
      ↓ Ends Trace with output
Background Flush (every 5s)
      ↓
Langfuse API (/api/public/ingestion)
```

### 3.2 Components

| Component | File | Purpose |
|-----------|------|---------|
| Client | `pkg/langfuse/client.go` | HTTP client for Langfuse API, event batching |
| Middleware | `pkg/langfuse/middleware.go` | gqlgen extension for automatic tracing |
| Config | `utils/variables.go` | Environment variable definitions |
| Init | `server.go` | Client initialization and middleware setup |

---

## 4. Files Changed

### 4.1 New Files

| File | Lines | Description |
|------|-------|-------------|
| `pkg/langfuse/client.go` | ~495 | Langfuse HTTP client with batched event sending |
| `pkg/langfuse/middleware.go` | ~188 | gqlgen middleware for GraphQL tracing |
| `.env` | 5 | Environment configuration (development) |

### 4.2 Modified Files

| File | Changes | Description |
|------|---------|-------------|
| `server.go` | +28 | Import langfuse package, initialize client, add middleware |
| `utils/variables.go` | +5 | Add Langfuse config fields to Configuration struct |
| `go.mod` | +1 | Add google/uuid dependency |
| `go.sum` | +2 | Dependency checksums |

---

## 5. Configuration

### 5.1 Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `LANGFUSE_ENABLED` | No | `false` | Enable/disable tracing |
| `LANGFUSE_HOST` | Yes* | `http://localhost:3002` | Langfuse server URL |
| `LANGFUSE_PUBLIC_KEY` | Yes* | - | API public key |
| `LANGFUSE_SECRET_KEY` | Yes* | - | API secret key |

*Required when `LANGFUSE_ENABLED=true`

### 5.2 Example .env

```env
LANGFUSE_ENABLED=true
LANGFUSE_HOST=http://localhost:3002
LANGFUSE_PUBLIC_KEY=pk-lf-xxxxx
LANGFUSE_SECRET_KEY=sk-lf-xxxxx
```

---

## 6. Implementation Details

### 6.1 Client (`pkg/langfuse/client.go`)

**Key Features:**
- Singleton pattern via `Initialize()` and `GetClient()`
- Thread-safe event queue with mutex
- Background flush every 5 seconds
- Batch flush when 10+ events accumulated
- Basic auth for Langfuse API
- Support for traces, spans, and generations

**Event Types:**
- `trace-create` / `trace-update`
- `span-create` / `span-update`  
- `generation-create` / `generation-update`
- `score-create`

### 6.2 Middleware (`pkg/langfuse/middleware.go`)

**Implements gqlgen interfaces:**
- `ExtensionName()` - Returns "LangfuseTracing"
- `Validate()` - Schema validation (no-op)
- `InterceptOperation()` - Creates trace at request start
- `InterceptResponse()` - Ends trace with response data
- `InterceptField()` - Creates spans for Query/Mutation/Subscription root fields

**Context Propagation:**
- Trace stored in context via `TraceContextKey`
- Helper `GetTraceFromContext()` for resolver access

### 6.3 Server Integration (`server.go`)

```go
// In init()
langfuseEnabled, _ := strconv.ParseBool(utils.Config.LangfuseEnabled)
langfuse.Initialize(langfuse.Config{
    Host:      utils.Config.LangfuseHost,
    PublicKey: utils.Config.LangfusePublicKey,
    SecretKey: utils.Config.LangfuseSecretKey,
    Enabled:   langfuseEnabled,
})

// In main()
if langfuseClient := langfuse.GetClient(); langfuseClient != nil && langfuseClient.IsEnabled() {
    srv.Use(langfuse.NewGraphQLMiddleware(langfuseClient))
    log.Info("Langfuse GraphQL tracing middleware enabled")
    defer langfuse.Flush()
}
```

---

## 7. Usage

### 7.1 Automatic Tracing
All GraphQL operations are automatically traced when enabled. No code changes required in resolvers.

### 7.2 Manual Tracing in Resolvers (Optional)

```go
// Get trace from context
trace := langfuse.GetTraceFromContext(ctx)
if trace != nil {
    // Create a span for custom operation
    span := trace.CreateSpan("custom-operation", metadata, input)
    defer span.End(output, "success")
}

// For LLM calls
gen := trace.CreateGeneration("llm-call", "gpt-4", prompt, nil)
defer gen.End(response, promptTokens, completionTokens)
```

---

## 8. Verification

### 8.1 Build
```bash
cd chaoscenter/graphql/server
go build -v .
```

### 8.2 Run
```bash
# Set environment variables or use .env file
go run server.go
```

### 8.3 Expected Logs
```
"msg":"Langfuse tracing initialized"
"msg":"Langfuse GraphQL tracing middleware enabled"
```

**Debug Level** (only visible with debug logging enabled):
```
"msg":"Successfully sent X events to Langfuse"
```

**Error Level** (visible on failures):
```
"msg":"Failed to send events to Langfuse: <error>"
"msg":"Langfuse API returned error status: <status_code>"
```

### 8.4 Verify Traces
1. Open Langfuse dashboard
2. Navigate to Traces
3. See GraphQL operations with:
   - Operation name
   - Query/variables as input
   - Response status as output
   - Field-level spans

---

## 9. Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/google/uuid` | v1.3.0+ | Generate trace/span IDs |
| `github.com/sirupsen/logrus` | existing | Logging |
| `github.com/99designs/gqlgen` | existing | GraphQL middleware interface |

---


