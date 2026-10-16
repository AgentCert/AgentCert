package observability

import (
	"context"
	"testing"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/agent_registry"
)

// fakeLangfuseClient is a minimal LangfuseClient that records every
// CreateObservation payload it receives, for asserting on emitted IDs.
type fakeLangfuseClient struct {
	observations []*agent_registry.LangfuseObservationPayload
}

func (f *fakeLangfuseClient) CreateOrUpdateUser(ctx context.Context, payload *agent_registry.LangfuseUserPayload) error {
	return nil
}
func (f *fakeLangfuseClient) DeleteUser(ctx context.Context, agentID string) error { return nil }
func (f *fakeLangfuseClient) TraceExperiment(ctx context.Context, trace *agent_registry.ExperimentTrace) error {
	return nil
}
func (f *fakeLangfuseClient) GetTraceMetadata(ctx context.Context, traceID string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}
func (f *fakeLangfuseClient) CreateObservation(ctx context.Context, payload *agent_registry.LangfuseObservationPayload) error {
	f.observations = append(f.observations, payload)
	return nil
}
func (f *fakeLangfuseClient) CreateScore(ctx context.Context, payload *agent_registry.LangfuseScorePayload) error {
	return nil
}

func newTestTracer(client agent_registry.LangfuseClient) *LangfuseTracer {
	return &LangfuseTracer{
		client:        client,
		enabled:       true,
		emittedFaults: make(map[string]string),
	}
}

// TestEmitFaultSpanAtInjection_StableIDAcrossEarlyNameFlip reproduces the
// pod-cpu-hog production bug: an early tick resolves FaultName to the raw
// generateName-suffixed engine name (ExperimentName hasn't populated yet),
// while a later tick resolves it to the canonical name once ExperimentName
// is available. Both ticks describe the same ChaosEngine (same EngineUID).
// Without EngineUID-keying, these two calls mint two different Langfuse
// observation IDs — a permanent orphaned span. With it, both must upsert the
// exact same observation ID.
func TestEmitFaultSpanAtInjection_StableIDAcrossEarlyNameFlip(t *testing.T) {
	fake := &fakeLangfuseClient{}
	tracer := newTestTracer(fake)

	const traceID = "trace-abc"
	const engineUID = "fe2c5c87-3de8-4278-bcce-a54bbea7f402"

	// Early tick: ExperimentName not yet resolved, handler.go's fallback
	// hands us the raw generateName-suffixed engine name.
	tracer.EmitFaultSpanAtInjection(context.Background(), traceID, FaultInjectionDetails{
		FaultName: "pod-cpu-hog7gl46",
		EngineUID: engineUID,
		StartedAt: "2026-08-04T18:04:38Z",
	}, nil)

	// Later tick: ExperimentName has resolved to the canonical fault name.
	tracer.EmitFaultSpanAtInjection(context.Background(), traceID, FaultInjectionDetails{
		FaultName:        "pod-cpu-hog",
		EngineUID:        engineUID,
		StartedAt:        "2026-08-04T18:04:38Z",
		FinishedAt:       "2026-08-04T18:11:46Z",
		InjectionVerdict: "Fail",
	}, nil)

	if len(fake.observations) != 2 {
		t.Fatalf("expected 2 CreateObservation calls (one per tick), got %d", len(fake.observations))
	}

	firstID := fake.observations[0].ID
	secondID := fake.observations[1].ID
	if firstID != secondID {
		t.Fatalf("expected both ticks to upsert the same observation ID, got %q and %q — this is the orphaned-span bug", firstID, secondID)
	}

	wantID := faultObservationID(traceID, engineUID)
	if firstID != wantID {
		t.Fatalf("expected observation ID %q (keyed on EngineUID), got %q", wantID, firstID)
	}
}

// TestEmitFaultSpanAtInjection_FallsBackToFaultNameWithoutUID ensures callers
// that don't populate EngineUID (e.g. code paths predating this field) still
// get a deterministic ID rather than an empty/invalid one.
func TestEmitFaultSpanAtInjection_FallsBackToFaultNameWithoutUID(t *testing.T) {
	fake := &fakeLangfuseClient{}
	tracer := newTestTracer(fake)

	tracer.EmitFaultSpanAtInjection(context.Background(), "trace-xyz", FaultInjectionDetails{
		FaultName: "pod-delete",
		StartedAt: "2026-08-04T18:11:56Z",
	}, nil)

	if len(fake.observations) != 1 {
		t.Fatalf("expected 1 CreateObservation call, got %d", len(fake.observations))
	}

	wantID := faultObservationID("trace-xyz", "pod-delete")
	if got := fake.observations[0].ID; got != wantID {
		t.Fatalf("expected observation ID %q, got %q", wantID, got)
	}
}
