package events

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"subscriber/pkg/types"
)

func TestStrConvTime(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want string
	}{
		{"positive", 12345, "12345"},
		{"zero", 0, "0"},
		{"negative returns empty", -1, ""},
		{"large negative returns empty", -999999, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StrConvTime(tt.in); got != tt.want {
				t.Errorf("StrConvTime(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestGetNameFromLog(t *testing.T) {
	tests := []struct {
		name string
		log  string
		want string
	}{
		{
			name: "extracts name",
			log:  "some preamble\nChaosEngine Name : pod-delete-abc123\nmore lines",
			want: "pod-delete-abc123",
		},
		{
			name: "name with hyphens",
			log:  "ChaosEngine Name : my-engine-xyz",
			want: "my-engine-xyz",
		},
		{
			name: "no match returns empty",
			log:  "nothing relevant here",
			want: "",
		},
		{
			name: "empty log returns empty",
			log:  "",
			want: "",
		},
		{
			name: "stops at non-word char",
			log:  "ChaosEngine Name : engine123 trailing",
			want: "engine123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getNameFromLog(tt.log); got != tt.want {
				t.Errorf("getNameFromLog() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetExperimentStatus(t *testing.T) {
	tests := []struct {
		name    string
		event   types.WorkflowEvent
		want    string
		wantErr bool
	}{
		{
			name:    "empty phase is invalid",
			event:   types.WorkflowEvent{Phase: ""},
			wantErr: true,
		},
		{
			name:  "stopped passes through",
			event: types.WorkflowEvent{Phase: "Stopped", FinishedAt: "100"},
			want:  "Stopped",
		},
		{
			name:  "not finished passes through",
			event: types.WorkflowEvent{Phase: "Running", FinishedAt: ""},
			want:  "Running",
		},
		{
			name: "chaosengine node with nil chaosexp counts as error",
			event: types.WorkflowEvent{
				Phase:      "Completed",
				FinishedAt: "100",
				Nodes: map[string]types.Node{
					"n1": {Type: "ChaosEngine", ChaosExp: nil},
				},
			},
			want: string(types.Error),
		},
		{
			name: "node phase error wins over probe failure",
			event: types.WorkflowEvent{
				Phase:      "Completed",
				FinishedAt: "100",
				Nodes: map[string]types.Node{
					"n1": {Type: "Pod", Phase: string(types.Error), ChaosExp: &types.ChaosData{}},
					"n2": {Type: "Pod", Phase: string(types.FaultCompletedWithProbeFailure), ChaosExp: &types.ChaosData{}},
				},
			},
			want: string(types.Error),
		},
		{
			name: "probe failure when no errors",
			event: types.WorkflowEvent{
				Phase:      "Completed",
				FinishedAt: "100",
				Nodes: map[string]types.Node{
					"n1": {Type: "Pod", Phase: string(types.FaultCompletedWithProbeFailure), ChaosExp: &types.ChaosData{}},
				},
			},
			want: string(types.FaultCompletedWithProbeFailure),
		},
		{
			name: "all good stays completed",
			event: types.WorkflowEvent{
				Phase:      "Completed",
				FinishedAt: "100",
				Nodes: map[string]types.Node{
					"n1": {Type: "Pod", Phase: string(types.FaultCompleted), ChaosExp: &types.ChaosData{}},
				},
			},
			want: "Completed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getExperimentStatus(tt.event)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("getExperimentStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateWorkflowPayload(t *testing.T) {
	ev := newEventsWithFakeGQL(t)

	notify := "notify-123"
	tests := []struct {
		name      string
		completed string
		event     types.WorkflowEvent
		// substrings expected in the generated payload
		mustContain    []string
		mustNotContain []string
	}{
		{
			name:      "basic payload without notifyID",
			completed: "false",
			event: types.WorkflowEvent{
				WorkflowID: "wf-1",
				UID:        "uid-1",
				RevisionID: "rev-1",
				Name:       "exp-name",
				UpdatedBy:  "alice",
			},
			mustContain: []string{
				`experimentID: \"wf-1\"`,
				`experimentRunID: \"uid-1\"`,
				`completed: false`,
				`experimentName:\"exp-name\"`,
				`updatedBy:\"alice\"`,
				`chaosExperimentRun(request:`,
			},
			mustNotContain: []string{`notifyID`},
		},
		{
			name:      "payload with notifyID",
			completed: "true",
			event: types.WorkflowEvent{
				WorkflowID: "wf-2",
				UID:        "uid-2",
				NotifyID:   &notify,
			},
			mustContain: []string{`notifyID:\"notify-123\"`, `completed: true`},
		},
		{
			name:      "node message quotes are stripped",
			completed: "false",
			event: types.WorkflowEvent{
				WorkflowID: "wf-3",
				UID:        "uid-3",
				Nodes: map[string]types.Node{
					"n1": {Message: `has "quotes" inside`},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := ev.GenerateWorkflowPayload("cid-x", "ak", "v1", tt.completed, tt.event)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			s := string(payload)
			for _, want := range tt.mustContain {
				if !strings.Contains(s, want) {
					t.Errorf("payload missing %q\npayload: %s", want, s)
				}
			}
			for _, no := range tt.mustNotContain {
				if strings.Contains(s, no) {
					t.Errorf("payload unexpectedly contains %q\npayload: %s", no, s)
				}
			}
			// Must always contain infraID echoing supplied cid/access/version.
			if !strings.Contains(s, `infraID: \"cid-x\"`) {
				t.Errorf("payload missing infraID cid: %s", s)
			}
		})
	}
}

func TestGenerateWorkflowPayload_ExecutionDataDecodes(t *testing.T) {
	ev := newEventsWithFakeGQL(t)
	event := types.WorkflowEvent{
		WorkflowID: "wf",
		UID:        "uid",
		Name:       "n",
		Nodes: map[string]types.Node{
			"n1": {Message: `msg "with" quotes`, Phase: "Running"},
		},
	}
	payload, err := ev.GenerateWorkflowPayload("c", "a", "v", "false", event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Extract executionData base64 blob and decode -> must be valid WorkflowEvent JSON
	// with quotes removed from node messages.
	s := string(payload)
	marker := `executionData:\"`
	idx := strings.Index(s, marker)
	if idx < 0 {
		t.Fatalf("executionData not found in payload: %s", s)
	}
	rest := s[idx+len(marker):]
	end := strings.Index(rest, `\"`)
	if end < 0 {
		t.Fatalf("could not find end of executionData")
	}
	b64 := rest[:end]
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("executionData not valid base64: %v", err)
	}
	var decodedEvent types.WorkflowEvent
	if err := json.Unmarshal(decoded, &decodedEvent); err != nil {
		t.Fatalf("decoded executionData not valid WorkflowEvent JSON: %v", err)
	}
	if strings.Contains(decodedEvent.Nodes["n1"].Message, `"`) {
		t.Errorf("node message quotes were not stripped: %q", decodedEvent.Nodes["n1"].Message)
	}
}
