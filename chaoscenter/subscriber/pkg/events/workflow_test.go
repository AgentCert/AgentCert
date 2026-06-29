package events

import (
	"testing"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUpdateWorkflowStatus(t *testing.T) {
	tests := []struct {
		in   v1alpha1.WorkflowPhase
		want string
	}{
		{v1alpha1.WorkflowRunning, "Running"},
		{v1alpha1.WorkflowSucceeded, "Completed"},
		{v1alpha1.WorkflowFailed, "Completed"},
		{v1alpha1.WorkflowPending, "Pending"},
		{v1alpha1.WorkflowError, "Error"},
		{v1alpha1.WorkflowPhase("Unknown"), "Pending"},
		{v1alpha1.WorkflowPhase(""), "Pending"},
	}
	for _, tt := range tests {
		t.Run(string(tt.in), func(t *testing.T) {
			if got := updateWorkflowStatus(tt.in); got != tt.want {
				t.Errorf("updateWorkflowStatus(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestWorkflowEventHandler_IgnoresInvalidMetadata(t *testing.T) {
	ev := newEventsWithFakeGQL(t)
	// No workflow_id label -> ignored, returns empty event and nil error,
	// without ever touching the cluster.
	wf := &v1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			UID:    "uid-1",
			Labels: map[string]string{},
		},
	}
	got, err := ev.WorkflowEventHandler(nil, wf, "ADD", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WorkflowID != "" || got.UID != "" {
		t.Errorf("expected empty WorkflowEvent for invalid metadata, got %+v", got)
	}
}

func TestWorkflowEventHandler_StartTimeAfterCreation(t *testing.T) {
	ev := newEventsWithFakeGQL(t)
	// Valid workflow_id but creation timestamp before subscriber start time -> error,
	// returned before any cluster call.
	wf := &v1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			UID:               "uid-2",
			Labels:            map[string]string{"workflow_id": "wf-2"},
			CreationTimestamp: metav1.Unix(1, 0), // 1 second -> 1000 ms
		},
	}
	// startTime far in the future relative to creation -> triggers the guard.
	_, err := ev.WorkflowEventHandler(nil, wf, "ADD", 9_000_000_000_000)
	if err == nil {
		t.Fatal("expected error when startTime is greater than creation timestamp")
	}
}
