package handler

import (
	"sort"
	"testing"

	types "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaos_experiment_run"
)

// TestComputeFaultWindows covers the fault-window derivation used to
// populate certification.StartInput.FaultWindows for the certifier's
// fault_windows request field -- the mechanism that lets Phase 0
// fault-bucketing split a trace into per-fault buckets by timestamp for
// blind-observer agents (e.g. flash-agent) whose traces carry no `fault: *`
// OTel spans of their own.
func TestComputeFaultWindows(t *testing.T) {
	tests := []struct {
		name  string
		nodes map[string]types.Node
		want  []string // expected fault names, sorted
	}{
		{
			name:  "empty node map yields no windows",
			nodes: map[string]types.Node{},
			want:  nil,
		},
		{
			name: "ChaosEngine node uses ExperimentName",
			nodes: map[string]types.Node{
				"n1": {
					Type:       "ChaosEngine",
					StartedAt:  "2026-01-01T00:00:00Z",
					FinishedAt: "2026-01-01T00:05:00Z",
					ChaosExp: &types.ChaosData{
						ExperimentName: "pod-delete",
						EngineName:     "engine-pod-delete-abc123",
					},
				},
			},
			want: []string{"pod-delete"},
		},
		{
			name: "ChaosEngine node falls back to EngineName when ExperimentName empty",
			nodes: map[string]types.Node{
				"n1": {
					Type:       "ChaosEngine",
					StartedAt:  "2026-01-01T00:00:00Z",
					FinishedAt: "2026-01-01T00:05:00Z",
					ChaosExp: &types.ChaosData{
						ExperimentName: "",
						EngineName:     "network-loss-engine",
					},
				},
			},
			want: []string{"network-loss-engine"},
		},
		{
			name: "non-ChaosEngine step matches std- prefix convention",
			nodes: map[string]types.Node{
				"n1": {
					Type:       "Pod",
					Name:       "chaos-workflow-xyz[0].std-pod-delete-fault(0)",
					StartedAt:  "2026-01-01T00:00:00Z",
					FinishedAt: "2026-01-01T00:05:00Z",
				},
			},
			want: []string{"pod-delete-fault"},
		},
		{
			name: "non-ChaosEngine step matches itb-scenario- convention",
			nodes: map[string]types.Node{
				"n1": {
					Type:       "Pod",
					Name:       "chaos-workflow-xyz[0].itb-scenario-42-kafka-preemption(0)",
					StartedAt:  "2026-01-01T00:00:00Z",
					FinishedAt: "2026-01-01T00:05:00Z",
				},
			},
			want: []string{"Scenario-42"},
		},
		{
			name: "unmatched step name is skipped, not classified",
			nodes: map[string]types.Node{
				"n1": {
					Type:       "Pod",
					Name:       "chaos-workflow-xyz[0].install-agent(0)",
					StartedAt:  "2026-01-01T00:00:00Z",
					FinishedAt: "2026-01-01T00:05:00Z",
				},
			},
			want: nil,
		},
		{
			name: "missing FinishedAt is skipped even with a matched name",
			nodes: map[string]types.Node{
				"n1": {
					Type:      "ChaosEngine",
					StartedAt: "2026-01-01T00:00:00Z",
					// FinishedAt intentionally empty -- fault still "running".
					ChaosExp: &types.ChaosData{ExperimentName: "cpu-hog"},
				},
			},
			want: nil,
		},
		{
			name: "unparseable timestamps are skipped",
			nodes: map[string]types.Node{
				"n1": {
					Type:       "ChaosEngine",
					StartedAt:  "not-a-timestamp",
					FinishedAt: "also-not-a-timestamp",
					ChaosExp:   &types.ChaosData{ExperimentName: "disk-fill"},
				},
			},
			want: nil,
		},
		{
			name: "multiple faults across mixed node types",
			nodes: map[string]types.Node{
				"n1": {
					Type:       "ChaosEngine",
					StartedAt:  "2026-01-01T00:00:00Z",
					FinishedAt: "2026-01-01T00:05:00Z",
					ChaosExp:   &types.ChaosData{ExperimentName: "pod-delete"},
				},
				"n2": {
					Type:       "Pod",
					Name:       "wf[0].itb-scenario-7-db-latency(0)",
					StartedAt:  "2026-01-01T00:06:00Z",
					FinishedAt: "2026-01-01T00:10:00Z",
				},
				"n3": {
					// Non-fault step, should not appear.
					Type:       "Pod",
					Name:       "wf[0].install-app(0)",
					StartedAt:  "2026-01-01T00:00:00Z",
					FinishedAt: "2026-01-01T00:01:00Z",
				},
			},
			want: []string{"Scenario-7", "pod-delete"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeFaultWindows(tt.nodes)

			gotNames := make([]string, 0, len(got))
			for _, w := range got {
				gotNames = append(gotNames, w.FaultName)
				if w.StartTime == "" || w.EndTime == "" {
					t.Errorf("fault window %q has empty StartTime/EndTime: %+v", w.FaultName, w)
				}
			}
			sort.Strings(gotNames)

			if len(gotNames) != len(tt.want) {
				t.Fatalf("computeFaultWindows() = %v, want %v", gotNames, tt.want)
			}
			for i := range gotNames {
				if gotNames[i] != tt.want[i] {
					t.Fatalf("computeFaultWindows() = %v, want %v", gotNames, tt.want)
				}
			}
		})
	}
}
