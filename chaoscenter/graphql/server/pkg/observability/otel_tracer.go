package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	otelTracerName = "chaoscenter-observability"
	serviceName    = "chaoscenter-graphql-server"
)

var otelTracerProvider *sdktrace.TracerProvider

// activeSpans stores open OTEL spans keyed by traceID (notifyID) so the
// ChaosExperimentRunEvent handler can add events to the span that was opened
// when the experiment was triggered.
var (
	activeSpans   = make(map[string]trace.Span)
	activeCtxs    = make(map[string]context.Context)
	activeNodeSpans = make(map[string]trace.Span)
	activeSpansMu sync.Mutex
)

// InitOTELTracer initializes the OpenTelemetry TracerProvider with an OTLP HTTP exporter.
// It reads configuration from environment variables:
//   - OTEL_EXPORTER_OTLP_ENDPOINT: The OTLP endpoint (e.g. http://langfuse:3000/api/public/otel)
//   - OTEL_EXPORTER_OTLP_HEADERS: Optional headers (e.g. Authorization=Basic <base64>)
//
// Returns nil if OTEL_EXPORTER_OTLP_ENDPOINT is not set (OTEL tracing disabled).
func InitOTELTracer(ctx context.Context) error {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		fmt.Println("[OTEL] OTEL_EXPORTER_OTLP_ENDPOINT not set — OTEL tracing disabled")
		return nil
	}

	// Build exporter options
	exporterOpts := []otlptracehttp.Option{
		otlptracehttp.WithEndpointURL(endpoint + "/v1/traces"),
	}

	// Explicitly parse OTEL_EXPORTER_OTLP_HEADERS and add them as options
	// (WithEndpointURL may bypass automatic env-var header detection).
	if hdrs := os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"); hdrs != "" {
		headerMap := make(map[string]string)
		for _, pair := range splitOTLPHeaders(hdrs) {
			if k, v, ok := splitHeaderKV(pair); ok {
				headerMap[k] = v
			}
		}
		if len(headerMap) > 0 {
			exporterOpts = append(exporterOpts, otlptracehttp.WithHeaders(headerMap))
			fmt.Printf("[OTEL] Injecting %d header(s) from OTEL_EXPORTER_OTLP_HEADERS\n", len(headerMap))
		}
	}

	exporter, err := otlptracehttp.New(ctx, exporterOpts...)
	if err != nil {
		return fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.TelemetrySDKLanguageGo,
			semconv.TelemetrySDKNameKey.String("opentelemetry"),
			attribute.String("service.component", "chaos-experiment-runner"),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create OTEL resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithMaxExportBatchSize(128),
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithResource(res),
	)

	otelTracerProvider = tp
	otel.SetTracerProvider(tp)

	// Capture OTEL export errors instead of silently discarding
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		fmt.Printf("[OTEL ERROR] %v\n", err)
	}))

	fmt.Printf("[OTEL] TracerProvider initialized (endpoint: %s)\n", endpoint)
	return nil
}

// ShutdownOTELTracer gracefully flushes and shuts down the TracerProvider.
// Call this during server graceful shutdown.
func ShutdownOTELTracer(ctx context.Context) error {
	if otelTracerProvider == nil {
		return nil
	}
	return otelTracerProvider.Shutdown(ctx)
}

// OTELTracerEnabled returns true if the OTEL TracerProvider has been initialized.
func OTELTracerEnabled() bool {
	return otelTracerProvider != nil
}

// BlindTracesEnabled returns true when the BLIND_TRACES env var is set to "yes".
// When enabled, OTEL fault spans omit all identifying fault attributes
// (fault name, target namespace/label/kind, engine template/name, chaos params)
// and replace them with opaque aliases (F1, F2, ...).
func BlindTracesEnabled() bool {
	return strings.EqualFold(os.Getenv("BLIND_TRACES"), "yes")
}

// GetOTELTracer returns a named tracer from the global TracerProvider.
func GetOTELTracer() trace.Tracer {
	return otel.Tracer(otelTracerName)
}

// EmitExperimentStartSpan creates and immediately ends an instant span named
// "experiment-triggered". Because it ends right away, the OTEL exporter flushes
// it to Langfuse immediately, making it the FIRST span in the trace timeline.
// It carries only identity/initial metadata (experiment ID, infra ID, etc.).
func EmitExperimentStartSpan(ctx context.Context, attrs ...attribute.KeyValue) {
	tracer := GetOTELTracer()
	_, span := tracer.Start(ctx, "experiment-triggered",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attrs...),
	)
	span.End() // end immediately → exported right away → appears first in Langfuse
}

// StartExperimentSpan creates the long-running root span ("experiment-run")
// for an experiment run and stores it in the active span map keyed by traceID
// (typically the notifyID). This span is ended later by EndExperimentSpan()
// when the experiment completes, covering the full experiment lifecycle.
//
// The notifyID UUID is converted to a 32-char hex OTEL trace ID (hyphens stripped)
// so that OTEL spans and LiteLLM generations (keyed by the same notifyID) land
// in the same Langfuse trace.
func StartExperimentSpan(ctx context.Context, traceID string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	tracer := GetOTELTracer()

	// Inject the notifyID as the OTEL trace ID.
	// UUID "58b4a037-5af5-..." → hex "58b4a0375af5..." (32 chars, valid OTEL trace ID).
	// Use context.Background() (not the caller's ctx) so that any existing local span
	// in the gRPC handler context does NOT override our phantom remote parent.
	injectCtx := ctx
	hexID := strings.ReplaceAll(traceID, "-", "")
	if len(hexID) == 32 {
		if otelTraceID, err := trace.TraceIDFromHex(hexID); err == nil {
			if phantomSpanID, err2 := trace.SpanIDFromHex(hexID[:16]); err2 == nil {
				remoteCtx := trace.NewSpanContext(trace.SpanContextConfig{
					TraceID:    otelTraceID,
					SpanID:     phantomSpanID,
					TraceFlags: trace.FlagsSampled,
					Remote:     true,
				})
				// context.Background() strips any inherited local span so the phantom
				// parent (with the desired trace ID) is used unconditionally.
				injectCtx = trace.ContextWithRemoteSpanContext(context.Background(), remoteCtx)
			}
		}
	}

	spanCtx, span := tracer.Start(injectCtx, "experiment-run",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attrs...),
	)

	activeSpansMu.Lock()
	activeSpans[traceID] = span
	activeCtxs[traceID] = spanCtx
	activeSpansMu.Unlock()

	return spanCtx, span
}

// GetExperimentSpan retrieves an active experiment span by traceID.
// Returns nil, nil if not found.
func GetExperimentSpan(traceID string) (context.Context, trace.Span) {
	activeSpansMu.Lock()
	defer activeSpansMu.Unlock()

	span, ok := activeSpans[traceID]
	if !ok {
		return nil, nil
	}
	return activeCtxs[traceID], span
}

// LinkedLangfuseTraceID returns the active OTEL trace ID when available so
// Langfuse REST observations/scores can attach to the same OTEL-exported trace.
// Falls back to the caller-provided logical trace key when no active span exists.
func LinkedLangfuseTraceID(traceID string) string {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return ""
	}

	activeSpansMu.Lock()
	span, ok := activeSpans[traceID]
	activeSpansMu.Unlock()
	if !ok || span == nil {
		return traceID
	}

	spanCtx := span.SpanContext()
	if !spanCtx.IsValid() {
		return traceID
	}

	otelTraceID := strings.TrimSpace(spanCtx.TraceID().String())
	if otelTraceID == "" {
		return traceID
	}

	return otelTraceID
}

// EndExperimentSpan ends the active experiment span and removes it from the map.
func EndExperimentSpan(traceID string) {
	activeSpansMu.Lock()
	span, ok := activeSpans[traceID]
	nodeSpans := make([]trace.Span, 0)
	nodePrefix := workflowNodeSpanKey(traceID, "")
	for key, nodeSpan := range activeNodeSpans {
		if strings.HasPrefix(key, nodePrefix) {
			nodeSpans = append(nodeSpans, nodeSpan)
			delete(activeNodeSpans, key)
		}
	}
	if ok {
		delete(activeSpans, traceID)
		delete(activeCtxs, traceID)
	}
	activeSpansMu.Unlock()

	for _, nodeSpan := range nodeSpans {
		if nodeSpan != nil {
			nodeSpan.End()
		}
	}

	if ok && span != nil {
		span.End()
		// Force-flush so the span is exported immediately
		if otelTracerProvider != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := otelTracerProvider.ForceFlush(ctx); err != nil {
				fmt.Printf("[OTEL ERROR] ForceFlush failed: %v\n", err)
			} else {
				fmt.Println("[OTEL] ForceFlush succeeded after EndExperimentSpan")
			}
		}
	}
}

// AddExperimentEvent adds a timestamped event to the active experiment span.
func AddExperimentEvent(traceID string, eventName string, attrs ...attribute.KeyValue) {
	activeSpansMu.Lock()
	span, ok := activeSpans[traceID]
	activeSpansMu.Unlock()

	if ok && span != nil {
		span.AddEvent(eventName, trace.WithAttributes(attrs...))
	}
}

// SetExperimentSpanAttributes sets attributes on the active experiment span.
func SetExperimentSpanAttributes(traceID string, attrs ...attribute.KeyValue) {
	activeSpansMu.Lock()
	span, ok := activeSpans[traceID]
	activeSpansMu.Unlock()

	if ok && span != nil {
		span.SetAttributes(attrs...)
	}
}

// RebindExperimentSpan moves active span state from oldTraceID to newTraceID.
// This is used when an experiment starts keyed by notifyID and later receives
// the canonical experiment_run_id from workflow events.
func RebindExperimentSpan(oldTraceID string, newTraceID string) {
	oldTraceID = strings.TrimSpace(oldTraceID)
	newTraceID = strings.TrimSpace(newTraceID)
	if oldTraceID == "" || newTraceID == "" || oldTraceID == newTraceID {
		return
	}

	activeSpansMu.Lock()
	defer activeSpansMu.Unlock()

	if _, exists := activeSpans[newTraceID]; exists {
		return
	}

	if span, ok := activeSpans[oldTraceID]; ok {
		activeSpans[newTraceID] = span
		delete(activeSpans, oldTraceID)
	}

	if spanCtx, ok := activeCtxs[oldTraceID]; ok {
		activeCtxs[newTraceID] = spanCtx
		delete(activeCtxs, oldTraceID)
	}

	oldPrefix := oldTraceID + ":"
	for key, span := range activeSpans {
		if strings.HasPrefix(key, oldPrefix) {
			suffix := strings.TrimPrefix(key, oldPrefix)
			newKey := newTraceID + ":" + suffix
			if _, exists := activeSpans[newKey]; !exists {
				activeSpans[newKey] = span
			}
			delete(activeSpans, key)
		}
	}

	oldNodePrefix := oldTraceID + ":node:"
	for key, span := range activeNodeSpans {
		if strings.HasPrefix(key, oldNodePrefix) {
			suffix := strings.TrimPrefix(key, oldNodePrefix)
			newKey := newTraceID + ":node:" + suffix
			if _, exists := activeNodeSpans[newKey]; !exists {
				activeNodeSpans[newKey] = span
			}
			delete(activeNodeSpans, key)
		}
	}
}

// StartFaultSpan creates a child span under the active experiment span for a specific fault.
// Returns the child span (caller should call span.End() when fault execution data arrives).
// The child span is stored in a separate map keyed by "traceID:faultName".
func StartFaultSpan(traceID string, faultName string, attrs ...attribute.KeyValue) trace.Span {
	return StartFaultSpanNamed(traceID, faultName, "fault: "+faultName, attrs...)
}

// StartFaultSpanNamed is like StartFaultSpan but lets the caller control the
// visible span name independently from the map key (faultKey). Use this when
// blind mode replaces the real fault name with an alias (e.g. "fault: F1") while
// still needing to look up the span by the real name in EndFaultSpan.
func StartFaultSpanNamed(traceID string, faultKey string, spanName string, attrs ...attribute.KeyValue) trace.Span {
	activeSpansMu.Lock()
	parentCtx, ok := activeCtxs[traceID]
	activeSpansMu.Unlock()

	if !ok || parentCtx == nil {
		return nil
	}

	tracer := GetOTELTracer()
	_, faultSpan := tracer.Start(parentCtx, spanName,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attrs...),
	)

	faultKey2 := traceID + ":" + faultKey
	activeSpansMu.Lock()
	activeSpans[faultKey2] = faultSpan
	activeSpansMu.Unlock()

	return faultSpan
}

// EndFaultSpan ends a per-fault child span and removes it from the map.
func EndFaultSpan(traceID string, faultName string, attrs ...attribute.KeyValue) {
	faultKey := traceID + ":" + faultName

	activeSpansMu.Lock()
	span, ok := activeSpans[faultKey]
	if ok {
		delete(activeSpans, faultKey)
	}
	activeSpansMu.Unlock()

	if ok && span != nil {
		span.SetAttributes(attrs...)
		span.End()
	}
}

func workflowNodeSpanKey(traceID, nodeID string) string {
	return traceID + ":node:" + nodeID
}

func isTerminalPhase(phase string) bool {
	phase = strings.ToLower(strings.TrimSpace(phase))
	switch phase {
	case "succeeded", "failed", "error", "completed", "skipped", "omitted":
		return true
	default:
		return false
	}
}

// UpsertWorkflowNodeSpan creates or updates a child span for a workflow node.
// When terminal is true, the span is ended and removed from the active map.
func UpsertWorkflowNodeSpan(traceID string, nodeID string, nodeName string, terminal bool, attrs ...attribute.KeyValue) {
	activeSpansMu.Lock()
	parentCtx, parentOK := activeCtxs[traceID]
	key := workflowNodeSpanKey(traceID, nodeID)
	nodeSpan, nodeOK := activeNodeSpans[key]
	activeSpansMu.Unlock()

	if !parentOK || parentCtx == nil || nodeID == "" {
		return
	}

	if !nodeOK || nodeSpan == nil {
		// If no active span exists and the node is already terminal, skip it.
		// This prevents zero-duration duplicate spans: on every workflow event all
		// previously-completed nodes are re-reported; without this guard we would
		// create a new span and end it immediately every time.
		if terminal {
			return
		}
		tracer := GetOTELTracer()
		_, span := tracer.Start(parentCtx, "workflow-step: "+nodeName,
			trace.WithSpanKind(trace.SpanKindInternal),
			trace.WithAttributes(attrs...),
		)
		activeSpansMu.Lock()
		activeNodeSpans[key] = span
		activeSpansMu.Unlock()
		nodeSpan = span
	} else {
		nodeSpan.SetAttributes(attrs...)
	}

	if terminal {
		nodeSpan.End()
		activeSpansMu.Lock()
		delete(activeNodeSpans, key)
		activeSpansMu.Unlock()
	}
}

// AddFaultEvent adds an event to an existing per-fault child span.
func AddFaultEvent(traceID string, faultName string, eventName string, attrs ...attribute.KeyValue) {
	faultKey := traceID + ":" + faultName

	activeSpansMu.Lock()
	span, ok := activeSpans[faultKey]
	activeSpansMu.Unlock()

	if ok && span != nil {
		span.AddEvent(eventName, trace.WithAttributes(attrs...))
	}
}

// MarshalJSON safely marshals a value to JSON string for span attributes.
func MarshalJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// splitOTLPHeaders splits comma-separated header pairs.
func splitOTLPHeaders(raw string) []string {
	var parts []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// splitHeaderKV splits a "Key=Value" header pair on the first '='.
func splitHeaderKV(pair string) (string, string, bool) {
	idx := strings.Index(pair, "=")
	if idx < 1 {
		return "", "", false
	}
	return strings.TrimSpace(pair[:idx]), strings.TrimSpace(pair[idx+1:]), true
}
