package events

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	chaosv1alpha1 "github.com/litmuschaos/chaos-operator/api/litmuschaos/v1alpha1"
	litmusV1alpha1 "github.com/litmuschaos/chaos-operator/pkg/client/clientset/versioned/typed/litmuschaos/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

var errTest = errors.New("test error")

// nodeStatusWithArtifact builds a NodeStatus carrying a single raw artifact with
// the given manifest data, as CheckChaosData expects.
func nodeStatusWithArtifact(nodeType, phase, manifest string) v1alpha1.NodeStatus {
	return v1alpha1.NodeStatus{
		ID:    "node-id",
		Type:  v1alpha1.NodeType(nodeType),
		Phase: v1alpha1.NodePhase(phase),
		Inputs: &v1alpha1.Inputs{
			Artifacts: v1alpha1.Artifacts{
				{Name: "manifest", ArtifactLocation: v1alpha1.ArtifactLocation{Raw: &v1alpha1.RawArtifact{Data: manifest}}},
			},
		},
	}
}

func TestCheckChaosData_NonChaosEngineArtifact(t *testing.T) {
	ev := newEventsWithFakeGQL(t)
	// A Pod manifest (not a ChaosEngine) -> returns the original node type, no chaos data.
	manifest := `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p"}}`
	ns := nodeStatusWithArtifact("Pod", "Running", manifest)

	nodeType, cd, err := ev.CheckChaosData(ns, "litmus", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nodeType != "Pod" {
		t.Errorf("nodeType = %q, want Pod", nodeType)
	}
	if cd != nil {
		t.Errorf("expected nil ChaosData, got %+v", cd)
	}
}

func TestCheckChaosData_ChaosEnginePending(t *testing.T) {
	ev := newEventsWithFakeGQL(t)
	// ChaosEngine but still Pending -> classified as ChaosEngine, but no data fetched.
	manifest := `{"apiVersion":"litmuschaos.io/v1alpha1","kind":"ChaosEngine","metadata":{"name":"engine-1","namespace":"litmus"}}`
	ns := nodeStatusWithArtifact("Pod", "Pending", manifest)

	nodeType, cd, err := ev.CheckChaosData(ns, "litmus", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nodeType != "ChaosEngine" {
		t.Errorf("nodeType = %q, want ChaosEngine", nodeType)
	}
	if cd != nil {
		t.Errorf("expected nil ChaosData for Pending engine, got %+v", cd)
	}
}

func TestCheckChaosData_InvalidManifest(t *testing.T) {
	ev := newEventsWithFakeGQL(t)
	// Garbage manifest: decode fails -> nodeType stays original, no error surfaced.
	ns := nodeStatusWithArtifact("Pod", "Running", "::: not yaml or json :::")

	nodeType, cd, err := ev.CheckChaosData(ns, "litmus", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nodeType != "Pod" {
		t.Errorf("nodeType = %q, want Pod", nodeType)
	}
	if cd != nil {
		t.Errorf("expected nil ChaosData, got %+v", cd)
	}
}

// newFakeChaosClient stands up an httptest server returning the given ChaosEngine
// for Get requests, and returns a litmus client pointed at it.
func newFakeChaosClient(t *testing.T, engine *chaosv1alpha1.ChaosEngine) *litmusV1alpha1.LitmuschaosV1alpha1Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return the engine for chaosengine Get requests.
		_ = json.NewEncoder(w).Encode(engine)
	}))
	t.Cleanup(srv.Close)

	cfg := &rest.Config{Host: srv.URL}
	client, err := litmusV1alpha1.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("failed to build litmus client: %v", err)
	}
	return client
}

func TestCheckChaosData_ChaosEngineRunning_FetchesData(t *testing.T) {
	ev := newEventsWithFakeGQL(t)

	// Engine created before the node started, so getChaosData proceeds.
	engine := &chaosv1alpha1.ChaosEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "engine-1",
			Namespace:         "litmus",
			UID:               "engine-uid-123",
			CreationTimestamp: metav1.Unix(1000, 0),
			Labels:            map[string]string{"context": "ctx-1"},
		},
		Status: chaosv1alpha1.ChaosEngineStatus{
			EngineStatus: chaosv1alpha1.EngineStatusInitialized,
		},
	}
	client := newFakeChaosClient(t, engine)

	manifest := `{"apiVersion":"litmuschaos.io/v1alpha1","kind":"ChaosEngine","metadata":{"name":"engine-1","namespace":"litmus"}}`
	// Node started at/before engine creation -> getChaosData proceeds (not stale).
	ns := nodeStatusWithArtifact("Pod", "Running", manifest)
	ns.StartedAt = metav1.Unix(500, 0)

	nodeType, cd, err := ev.CheckChaosData(ns, "litmus", client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nodeType != "ChaosEngine" {
		t.Errorf("nodeType = %q, want ChaosEngine", nodeType)
	}
	if cd == nil {
		t.Fatalf("expected ChaosData, got nil")
	}
	if cd.EngineName != "engine-1" {
		t.Errorf("EngineName = %q, want engine-1", cd.EngineName)
	}
	if cd.EngineUID != "engine-uid-123" {
		t.Errorf("EngineUID = %q, want engine-uid-123", cd.EngineUID)
	}
	if cd.EngineContext != "ctx-1" {
		t.Errorf("EngineContext = %q, want ctx-1", cd.EngineContext)
	}
	// No experiments -> early return with default probe percentage.
	if cd.ProbeSuccessPercentage != "0" {
		t.Errorf("ProbeSuccessPercentage = %q, want 0", cd.ProbeSuccessPercentage)
	}
}

func TestCheckChaosData_EngineNewerThanNode_ReturnsNil(t *testing.T) {
	ev := newEventsWithFakeGQL(t)

	// Node started AFTER the engine was created -> engine is stale, getChaosData returns (nil, nil).
	engine := &chaosv1alpha1.ChaosEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "engine-2",
			Namespace:         "litmus",
			CreationTimestamp: metav1.Unix(1000, 0),
		},
	}
	client := newFakeChaosClient(t, engine)

	manifest := `{"apiVersion":"litmuschaos.io/v1alpha1","kind":"ChaosEngine","metadata":{"name":"engine-2","namespace":"litmus"}}`
	ns := nodeStatusWithArtifact("Pod", "Running", manifest)
	ns.StartedAt = metav1.Unix(5000, 0) // node newer than engine -> stale

	nodeType, cd, err := ev.CheckChaosData(ns, "litmus", client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nodeType != "ChaosEngine" {
		t.Errorf("nodeType = %q, want ChaosEngine", nodeType)
	}
	if cd != nil {
		t.Errorf("expected nil ChaosData when engine newer than node, got %+v", cd)
	}
}
