package events

import (
	"strings"
	"testing"

	"subscriber/pkg/types"
)

func infraData() map[string]string {
	return map[string]string{
		"INFRA_ID":    "infra-1",
		"ACCESS_KEY":  "ak",
		"VERSION":     "v1",
		"SERVER_ADDR": "http://server",
	}
}

func TestSendWorkflowUpdates_InProgress(t *testing.T) {
	// Clear shared eventMap state.
	eventMap = make(map[string]types.WorkflowEvent)

	f := newFakeGQL()
	ev := newEventsWithGQL(f)

	event := types.WorkflowEvent{
		UID:        "uid-1",
		WorkflowID: "wf-1",
		Phase:      "Running",
		FinishedAt: "", // not finished -> completed:false, retained in eventMap
		Nodes:      map[string]types.Node{},
	}
	body, err := ev.SendWorkflowUpdates(infraData(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body != "ok" {
		t.Errorf("body = %q, want ok", body)
	}
	if !f.sendCalled {
		t.Fatal("expected SendRequest to be called")
	}
	// Not finished -> still tracked in eventMap.
	if _, ok := eventMap["uid-1"]; !ok {
		t.Error("expected event to remain in eventMap when not finished")
	}
	if !strings.Contains(string(f.sendPayloads[0]), "completed: false") {
		t.Errorf("expected completed:false payload, got %s", f.sendPayloads[0])
	}
}

func TestSendWorkflowUpdates_Finished_RemovesFromMap(t *testing.T) {
	eventMap = make(map[string]types.WorkflowEvent)

	f := newFakeGQL()
	ev := newEventsWithGQL(f)

	event := types.WorkflowEvent{
		UID:        "uid-2",
		WorkflowID: "wf-2",
		Phase:      "Completed",
		FinishedAt: "12345",
		Nodes:      map[string]types.Node{},
	}
	if _, err := ev.SendWorkflowUpdates(infraData(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := eventMap["uid-2"]; ok {
		t.Error("finished event should be deleted from eventMap")
	}
	// Last payload should be the completed:true regeneration.
	last := string(f.sendPayloads[len(f.sendPayloads)-1])
	if !strings.Contains(last, "completed: true") {
		t.Errorf("expected completed:true payload, got %s", last)
	}
}

func TestSendWorkflowUpdates_StoppedOverridesRunningNodes(t *testing.T) {
	eventMap = make(map[string]types.WorkflowEvent)

	f := newFakeGQL()
	ev := newEventsWithGQL(f)

	// Seed eventMap with a prior version of this run carrying a ChaosEngine node.
	prior := types.WorkflowEvent{
		UID: "uid-3",
		Nodes: map[string]types.Node{
			"n1": {Type: "ChaosEngine", Phase: "Running", Message: "prev", ChaosExp: &types.ChaosData{EngineName: "e"}},
		},
	}
	eventMap["uid-3"] = prior

	// New event: stopped, node still Running and missing ChaosExp.
	event := types.WorkflowEvent{
		UID:        "uid-3",
		WorkflowID: "wf-3",
		Phase:      "Stopped",
		FinishedAt: "999",
		Nodes: map[string]types.Node{
			"n1": {Type: "ChaosEngine", Phase: "Running", ChaosExp: nil},
		},
	}
	if _, err := ev.SendWorkflowUpdates(infraData(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !f.sendCalled {
		t.Fatal("expected SendRequest to be called")
	}
}

func TestSendWorkflowUpdates_SendError(t *testing.T) {
	eventMap = make(map[string]types.WorkflowEvent)

	f := newFakeGQL()
	f.sendErr = errTest
	ev := newEventsWithGQL(f)

	event := types.WorkflowEvent{UID: "uid-4", Phase: "Running", Nodes: map[string]types.Node{}}
	if _, err := ev.SendWorkflowUpdates(infraData(), event); err == nil {
		t.Fatal("expected error from SendRequest")
	}
}
