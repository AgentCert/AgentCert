package events

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	chaosv1alpha1 "github.com/litmuschaos/chaos-operator/api/litmuschaos/v1alpha1"
	litmusV1alpha1 "github.com/litmuschaos/chaos-operator/pkg/client/clientset/versioned/typed/litmuschaos/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// chaosServer routes requests by URL path: ChaosEngine Get/List vs ChaosResult Get.
func chaosServer(t *testing.T, engine *chaosv1alpha1.ChaosEngine, engineList *chaosv1alpha1.ChaosEngineList, result *chaosv1alpha1.ChaosResult) *litmusV1alpha1.LitmuschaosV1alpha1Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "chaosresults"):
			_ = json.NewEncoder(w).Encode(result)
		case strings.Contains(r.URL.Path, "chaosengines"):
			// A List has no resource name segment after "chaosengines".
			trimmed := strings.TrimSuffix(r.URL.Path, "/")
			if strings.HasSuffix(trimmed, "chaosengines") {
				_ = json.NewEncoder(w).Encode(engineList)
			} else {
				_ = json.NewEncoder(w).Encode(engine)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := litmusV1alpha1.NewForConfig(&rest.Config{Host: srv.URL})
	if err != nil {
		t.Fatalf("failed to build litmus client: %v", err)
	}
	return client
}

func TestGetChaosData_WithExperimentAndResult(t *testing.T) {
	ev := newEventsWithFakeGQL(t)

	engine := &chaosv1alpha1.ChaosEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "pod-delete",
			Namespace:         "litmus",
			UID:               "eng-uid",
			CreationTimestamp: metav1.Unix(1000, 0),
		},
		Status: chaosv1alpha1.ChaosEngineStatus{
			EngineStatus: chaosv1alpha1.EngineStatusCompleted,
			Experiments: []chaosv1alpha1.ExperimentStatuses{
				{
					Name:           "pod-delete",
					ExpPod:         "exp-pod",
					Runner:         "runner-pod",
					Verdict:        "Pass",
					LastUpdateTime: metav1.Unix(1500, 0),
				},
			},
		},
	}
	result := &chaosv1alpha1.ChaosResult{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-delete-pod-delete", Namespace: "litmus"},
		Status: chaosv1alpha1.ChaosResultStatus{
			ExperimentStatus: chaosv1alpha1.TestStatus{
				Phase:                  "Completed",
				Verdict:                "Pass",
				ProbeSuccessPercentage: "100",
			},
		},
	}
	client := chaosServer(t, engine, nil, result)

	manifest := `{"apiVersion":"litmuschaos.io/v1alpha1","kind":"ChaosEngine","metadata":{"name":"pod-delete","namespace":"litmus"}}`
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
		t.Fatal("expected ChaosData")
	}
	if cd.ExperimentName != "pod-delete" {
		t.Errorf("ExperimentName = %q, want pod-delete", cd.ExperimentName)
	}
	if cd.ExperimentPod != "exp-pod" {
		t.Errorf("ExperimentPod = %q, want exp-pod", cd.ExperimentPod)
	}
	if cd.RunnerPod != "runner-pod" {
		t.Errorf("RunnerPod = %q, want runner-pod", cd.RunnerPod)
	}
	if cd.ProbeSuccessPercentage != "100" {
		t.Errorf("ProbeSuccessPercentage = %q, want 100", cd.ProbeSuccessPercentage)
	}
	if cd.ChaosResult == nil {
		t.Error("expected ChaosResult to be attached")
	}
}

func TestGetChaosData_StoppedEngineMarksFail(t *testing.T) {
	ev := newEventsWithFakeGQL(t)
	engine := &chaosv1alpha1.ChaosEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "eng-stop",
			Namespace:         "litmus",
			CreationTimestamp: metav1.Unix(1000, 0),
		},
		Status: chaosv1alpha1.ChaosEngineStatus{
			EngineStatus: chaosv1alpha1.EngineStatusStopped,
		},
	}
	client := chaosServer(t, engine, nil, nil)

	manifest := `{"apiVersion":"litmuschaos.io/v1alpha1","kind":"ChaosEngine","metadata":{"name":"eng-stop","namespace":"litmus"}}`
	ns := nodeStatusWithArtifact("Pod", "Running", manifest)
	ns.StartedAt = metav1.Unix(500, 0)

	_, cd, err := ev.CheckChaosData(ns, "litmus", client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cd == nil {
		t.Fatal("expected ChaosData")
	}
	if cd.ExperimentVerdict != "Fail" {
		t.Errorf("ExperimentVerdict = %q, want Fail (stopped engine)", cd.ExperimentVerdict)
	}
}

func TestFindChaosEngineByLabel_ByRunID(t *testing.T) {
	ev := newEventsWithFakeGQL(t)
	engineList := &chaosv1alpha1.ChaosEngineList{
		Items: []chaosv1alpha1.ChaosEngine{
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-delete-xyz", Namespace: "litmus"}},
		},
	}
	client := chaosServer(t, nil, engineList, nil)

	name, err := ev.findChaosEngineByLabel("pod-delete-", "litmus", map[string]string{"workflow_run_id": "run-9"}, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "pod-delete-xyz" {
		t.Errorf("name = %q, want pod-delete-xyz", name)
	}
}

func TestFindChaosEngineByLabel_NoMatch(t *testing.T) {
	ev := newEventsWithFakeGQL(t)
	// Empty list -> all lookups fail -> error.
	engineList := &chaosv1alpha1.ChaosEngineList{Items: []chaosv1alpha1.ChaosEngine{}}
	client := chaosServer(t, nil, engineList, nil)

	_, err := ev.findChaosEngineByLabel("prefix-", "litmus", map[string]string{}, client)
	if err == nil {
		t.Fatal("expected error when no ChaosEngine matches")
	}
}

func TestStopChaosEngineState_ConfigError(t *testing.T) {
	// GetDynamicAndDiscoveryClient fails when kube config can't be built.
	empty := ""
	setKubeConfig(t, &empty)

	ev := newEventsWithFakeGQL(t)
	runID := "run-1"
	err := ev.StopChaosEngineState("litmus", &runID)
	if err == nil {
		t.Fatal("expected error from dynamic client creation")
	}
	if !strings.Contains(err.Error(), "failed to get dynamic client") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetWorkflowObj_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		list := &v1alpha1.WorkflowList{
			Items: v1alpha1.Workflows{
				{ObjectMeta: metav1.ObjectMeta{Name: "wf-a", UID: "uid-a"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "wf-b", UID: "uid-b"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
	}))
	defer srv.Close()

	ev := newEventsWithKubeHost(t, srv.URL)
	wf, err := ev.GetWorkflowObj("uid-b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf == nil || wf.Name != "wf-b" {
		t.Fatalf("expected wf-b, got %+v", wf)
	}
}

func TestGetWorkflowObj_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&v1alpha1.WorkflowList{Items: v1alpha1.Workflows{
			{ObjectMeta: metav1.ObjectMeta{Name: "wf-a", UID: "uid-a"}},
		}})
	}))
	defer srv.Close()

	ev := newEventsWithKubeHost(t, srv.URL)
	wf, err := ev.GetWorkflowObj("missing-uid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf != nil {
		t.Errorf("expected nil for missing uid, got %+v", wf)
	}
}

func TestListWorkflowObject_Success(t *testing.T) {
	var gotLabelSelector string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLabelSelector = r.URL.Query().Get("labelSelector")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&v1alpha1.WorkflowList{Items: v1alpha1.Workflows{
			{ObjectMeta: metav1.ObjectMeta{Name: "wf-1"}},
		}})
	}))
	defer srv.Close()

	ev := newEventsWithKubeHost(t, srv.URL)
	list, err := ev.ListWorkflowObject("wf-id-77")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list.Items) != 1 {
		t.Errorf("expected 1 workflow, got %d", len(list.Items))
	}
	if !strings.Contains(gotLabelSelector, "workflow_id=wf-id-77") {
		t.Errorf("label selector = %q, want workflow_id=wf-id-77", gotLabelSelector)
	}
}

func TestStopWorkflow_Success(t *testing.T) {
	var method string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&v1alpha1.Workflow{ObjectMeta: metav1.ObjectMeta{Name: "wf-x"}})
	}))
	defer srv.Close()

	ev := newEventsWithKubeHost(t, srv.URL)
	if err := ev.StopWorkflow("wf-x", "litmus"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", method)
	}
}
