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

// GetOTELTracer returns a named tracer from the global TracerProvider.
func GetOTELTracer() trace.Tracer {
	return otel.Tracer(otelTracerName)
}

// EmitExperimentStartSpan creates and immediately ends an instant span named
// "experiment-run-start". Because it ends right away, the OTEL exporter flushes
// it to Langfuse immediately, making it the FIRST span in the trace timeline.
// It carries only identity/initial metadata (experiment ID, infra ID, etc.).
func EmitExperimentStartSpan(ctx context.Context, attrs ...attribute.KeyValue) {
	tracer := GetOTELTracer()
	_, span := tracer.Start(ctx, "experiment-run-start",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attrs...),
	)
	span.End() // end immediately → exported right away → appears first in Langfuse
}

// StartExperimentSpan creates the long-running root span ("experiment-run-end")
// for an experiment run and stores it in the active span map keyed by traceID
// (typically the notifyID). This span is ended later by EndExperimentSpan()
// when the experiment completes, so it appears last in the trace timeline.
func StartExperimentSpan(ctx context.Context, traceID string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	tracer := GetOTELTracer()
	spanCtx, span := tracer.Start(ctx, "experiment-run-end",
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

// EndExperimentSpan ends the active experiment span and removes it from the map.
func EndExperimentSpan(traceID string) {
	activeSpansMu.Lock()
	span, ok := activeSpans[traceID]
	if ok {
		delete(activeSpans, traceID)
		delete(activeCtxs, traceID)
	}
	activeSpansMu.Unlock()

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

// StartFaultSpan creates a child span under the active experiment span for a specific fault.
// Returns the child span (caller should call span.End() when fault execution data arrives).
// The child span is stored in a separate map keyed by "traceID:faultName".
func StartFaultSpan(traceID string, faultName string, attrs ...attribute.KeyValue) trace.Span {
	activeSpansMu.Lock()
	parentCtx, ok := activeCtxs[traceID]
	activeSpansMu.Unlock()

	if !ok || parentCtx == nil {
		return nil
	}

	tracer := GetOTELTracer()
	_, faultSpan := tracer.Start(parentCtx, "fault: "+faultName,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attrs...),
	)

	faultKey := traceID + ":" + faultName
	activeSpansMu.Lock()
	activeSpans[faultKey] = faultSpan
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
