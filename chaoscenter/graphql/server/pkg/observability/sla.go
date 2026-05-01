package observability

import (
	"os"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
)

// SLAConfig holds the per-experiment SLA contract used by the certifier
// to score detection / mitigation latency and per-tool-call latency.
//
// Resolution order (highest priority first):
//  1. Per-experiment overrides (Argo workflow annotations) — wired in Phase 2.
//  2. Org-wide defaults from environment variables (this file).
//  3. Built-in safe defaults (used only when env unset).
//
// Env vars:
//
//	SLA_DETECT_SEC      — max seconds to detect a fault       (default 60)
//	SLA_MITIGATE_SEC    — max seconds to mitigate a fault     (default 300)
//	SLA_TOOL_CALL_SEC   — max seconds per agent tool call     (default 30)
//
// Set to 0 (or any non-positive value) to opt out of that particular SLA.
type SLAConfig struct {
	DetectSec   float64
	MitigateSec float64
	ToolCallSec float64
}

const (
	defaultSLADetectSec   = 60.0
	defaultSLAMitigateSec = 300.0
	defaultSLAToolCallSec = 30.0
)

// LoadSLAFromEnv reads the org-wide SLA defaults from environment variables,
// falling back to built-in safe defaults when an env var is unset or invalid.
func LoadSLAFromEnv() SLAConfig {
	return SLAConfig{
		DetectSec:   readFloatEnv("SLA_DETECT_SEC", defaultSLADetectSec),
		MitigateSec: readFloatEnv("SLA_MITIGATE_SEC", defaultSLAMitigateSec),
		ToolCallSec: readFloatEnv("SLA_TOOL_CALL_SEC", defaultSLAToolCallSec),
	}
}

// Attributes returns the OTEL attribute set for stamping the SLA contract
// onto the experiment-run span. Keys mirror what trace_extractor.py reads
// from `metadata.attributes.experiment.sla.*`.
func (s SLAConfig) Attributes() []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.Float64("experiment.sla.detect_sec", s.DetectSec),
		attribute.Float64("experiment.sla.mitigate_sec", s.MitigateSec),
		attribute.Float64("experiment.sla.tool_call_sec", s.ToolCallSec),
	}
}

// AsMetadata returns the same SLA values as a flat map suitable for embedding
// in a Langfuse observation's metadata block (used by EmitFaultSpansForTrace
// to mirror SLA on the agent trace as a backup data source).
func (s SLAConfig) AsMetadata() map[string]interface{} {
	return map[string]interface{}{
		"experiment.sla.detect_sec":    s.DetectSec,
		"experiment.sla.mitigate_sec":  s.MitigateSec,
		"experiment.sla.tool_call_sec": s.ToolCallSec,
	}
}

func readFloatEnv(key string, fallback float64) float64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return v
}
