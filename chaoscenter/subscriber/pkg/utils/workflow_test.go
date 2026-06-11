package utils

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"subscriber/pkg/types"

	wfv1alpha1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	v1alpha12 "github.com/litmuschaos/chaos-operator/pkg/client/clientset/versioned/typed/litmuschaos/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	argoV1alpha1 "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned/typed/workflow/v1alpha1"
)

// fakeEvents implements events.SubscriberEvents recording the calls
// WorkflowRequest makes, returning canned values.
type fakeEvents struct {
	listWf  *wfv1alpha1.WorkflowList
	listErr error
	getWf   *wfv1alpha1.Workflow
	getErr  error

	stopEngineErr  error
	stopWfErr      error
	stopEngineCnt  int
	stopWfCnt      int
	lastStopEngNS  string
	lastStopRunID  string
}

func (f *fakeEvents) ListWorkflowObject(wfid string) (*wfv1alpha1.WorkflowList, error) {
	return f.listWf, f.listErr
}
func (f *fakeEvents) GetWorkflowObj(uid string) (*wfv1alpha1.Workflow, error) {
	return f.getWf, f.getErr
}
func (f *fakeEvents) StopChaosEngineState(namespace string, workflowRunID *string) error {
	f.stopEngineCnt++
	f.lastStopEngNS = namespace
	if workflowRunID != nil {
		f.lastStopRunID = *workflowRunID
	}
	return f.stopEngineErr
}
func (f *fakeEvents) StopWorkflow(wfName, namespace string) error {
	f.stopWfCnt++
	return f.stopWfErr
}

// Unused-by-WorkflowRequest interface methods.
func (f *fakeEvents) ChaosEventWatcher(chan struct{}, chan types.WorkflowEvent, map[string]string) {}
func (f *fakeEvents) CheckChaosData(wfv1alpha1.NodeStatus, string, *v1alpha12.LitmuschaosV1alpha1Client) (string, *types.ChaosData, error) {
	return "", nil, nil
}
func (f *fakeEvents) GenerateWorkflowPayload(string, string, string, string, types.WorkflowEvent) ([]byte, error) {
	return nil, nil
}
func (f *fakeEvents) WorkflowEventWatcher(chan struct{}, chan types.WorkflowEvent, map[string]string) {}
func (f *fakeEvents) WorkflowEventHandler(*wfv1alpha1.Workflow, *wfv1alpha1.Workflow, string, int64) (types.WorkflowEvent, error) {
	return types.WorkflowEvent{}, nil
}
func (f *fakeEvents) SendWorkflowUpdates(map[string]string, types.WorkflowEvent) (string, error) {
	return "", nil
}
func (f *fakeEvents) WorkflowUpdates(map[string]string, chan types.WorkflowEvent) {}

// fakeK8s implements k8s.SubscriberK8s; only GetKubeConfig is meaningful here.
type fakeK8s struct {
	config    *rest.Config
	configErr error
}

func (f *fakeK8s) GetKubeConfig() (*rest.Config, error) { return f.config, f.configErr }

func (f *fakeK8s) GetLogs(string, string, string) (string, error)         { return "", nil }
func (f *fakeK8s) CreatePodLog(types.PodLogRequest) (types.PodLog, error) { return types.PodLog{}, nil }
func (f *fakeK8s) SendPodLogs(map[string]string, types.PodLogRequest)     {}
func (f *fakeK8s) GenerateLogPayload(string, string, string, types.PodLogRequest) ([]byte, error) {
	return nil, nil
}
func (f *fakeK8s) GetKubernetesNamespaces(types.KubeNamespaceRequest) ([]*types.KubeNamespace, error) {
	return nil, nil
}
func (f *fakeK8s) GetKubernetesObjects(types.KubeObjRequest) (*types.KubeObject, error) {
	return nil, nil
}
func (f *fakeK8s) GetObjectDataByNamespace(string, dynamic.Interface, schema.GroupVersionResource) ([]types.ObjectData, error) {
	return nil, nil
}
func (f *fakeK8s) GenerateKubeObject(string, string, string, types.KubeObjRequest) ([]byte, error) {
	return nil, nil
}
func (f *fakeK8s) GenerateKubeNamespace(string, string, string, types.KubeNamespaceRequest) ([]byte, error) {
	return nil, nil
}
func (f *fakeK8s) SendKubeObjects(map[string]string, types.KubeObjRequest) error      { return nil }
func (f *fakeK8s) SendKubeNamespaces(map[string]string, types.KubeNamespaceRequest) error {
	return nil
}
func (f *fakeK8s) CheckComponentStatus(string) error                   { return nil }
func (f *fakeK8s) IsAgentConfirmed() (bool, string, error)             { return false, "", nil }
func (f *fakeK8s) AgentRegister(string) (bool, error)                  { return false, nil }
func (f *fakeK8s) AgentOperations(types.Action) (*unstructured.Unstructured, error) {
	return nil, nil
}
func (f *fakeK8s) AgentConfirm(map[string]string) ([]byte, error)      { return nil, nil }
func (f *fakeK8s) GetGenericK8sClient() (*kubernetes.Clientset, error) { return nil, nil }
func (f *fakeK8s) GetDynamicAndDiscoveryClient() (discovery.DiscoveryInterface, dynamic.Interface, error) {
	return nil, nil, nil
}
func (f *fakeK8s) GenerateArgoClient(string) (argoV1alpha1.WorkflowInterface, error) {
	return nil, nil
}

func agentData() map[string]string {
	return map[string]string{"AGENT_NAMESPACE": "agent-ns", "INFRA_NAMESPACE": "infra-ns"}
}

func TestWorkflowRequest_UnknownTypeIsNoop(t *testing.T) {
	ev := &fakeEvents{}
	u := NewSubscriberUtils(ev, &fakeK8s{})
	if err := u.WorkflowRequest(agentData(), "unknown_type", "x", "user"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.stopEngineCnt != 0 || ev.stopWfCnt != 0 {
		t.Error("no event ops should fire for unknown request type")
	}
}

func TestWorkflowRequest_Delete_ListError(t *testing.T) {
	ev := &fakeEvents{listErr: errors.New("list failed")}
	u := NewSubscriberUtils(ev, &fakeK8s{})
	err := u.WorkflowRequest(agentData(), "workflow_delete", "wf-id", "user")
	if err == nil {
		t.Fatal("expected error when ListWorkflowObject fails")
	}
}

func TestWorkflowRequest_RunDelete_GetError(t *testing.T) {
	ev := &fakeEvents{getErr: errors.New("get failed")}
	u := NewSubscriberUtils(ev, &fakeK8s{})
	err := u.WorkflowRequest(agentData(), "workflow_run_delete", "uid", "user")
	if err == nil {
		t.Fatal("expected error when GetWorkflowObj fails")
	}
}

func TestWorkflowRequest_RunStop_GetError(t *testing.T) {
	ev := &fakeEvents{getErr: errors.New("get failed")}
	u := NewSubscriberUtils(ev, &fakeK8s{})
	err := u.WorkflowRequest(agentData(), "workflow_run_stop", "uid", "user")
	if err == nil {
		t.Fatal("expected error when GetWorkflowObj fails")
	}
}

func TestWorkflowRequest_RunStop_StopsEngineAndWorkflow(t *testing.T) {
	ev := &fakeEvents{
		getWf: &wfv1alpha1.Workflow{
			ObjectMeta: metav1.ObjectMeta{Name: "wf-1", Namespace: "infra-ns"},
		},
	}
	u := NewSubscriberUtils(ev, &fakeK8s{})
	err := u.WorkflowRequest(agentData(), "workflow_run_stop", "run-id-9", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.stopEngineCnt != 1 {
		t.Errorf("StopChaosEngineState called %d times, want 1", ev.stopEngineCnt)
	}
	if ev.lastStopEngNS != "infra-ns" {
		t.Errorf("StopChaosEngineState namespace = %q, want infra-ns", ev.lastStopEngNS)
	}
	if ev.lastStopRunID != "run-id-9" {
		t.Errorf("StopChaosEngineState runID = %q, want run-id-9", ev.lastStopRunID)
	}
	if ev.stopWfCnt != 1 {
		t.Errorf("StopWorkflow called %d times, want 1", ev.stopWfCnt)
	}
}

func TestWorkflowRequest_RunStop_ToleratesStopErrors(t *testing.T) {
	// StopChaosEngineState and StopWorkflow errors are logged, not returned.
	ev := &fakeEvents{
		getWf:         &wfv1alpha1.Workflow{ObjectMeta: metav1.ObjectMeta{Name: "wf", Namespace: "infra-ns"}},
		stopEngineErr: errors.New("engine stop failed"),
		stopWfErr:     errors.New("wf stop failed"),
	}
	u := NewSubscriberUtils(ev, &fakeK8s{})
	if err := u.WorkflowRequest(agentData(), "workflow_run_stop", "uid", "user"); err != nil {
		t.Fatalf("WorkflowRequest should tolerate stop errors, got: %v", err)
	}
}

func TestWorkflowRequest_Delete_IteratesWorkflows(t *testing.T) {
	// Build an httptest-backed argo server so the inner DeleteWorkflow path runs.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	ev := &fakeEvents{
		listWf: &wfv1alpha1.WorkflowList{
			Items: wfv1alpha1.Workflows{
				{ObjectMeta: metav1.ObjectMeta{Name: "wf-a", Namespace: "infra-ns", UID: "uid-a"}},
			},
		},
	}
	k := &fakeK8s{config: &rest.Config{Host: srv.URL}}
	u := NewSubscriberUtils(ev, k)
	if err := u.WorkflowRequest(agentData(), "workflow_delete", "wf-id", "user"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.stopEngineCnt != 1 {
		t.Errorf("StopChaosEngineState called %d times, want 1 (one per workflow)", ev.stopEngineCnt)
	}
}

func TestDeleteWorkflow_ConfigError(t *testing.T) {
	k := &fakeK8s{configErr: errors.New("no config")}
	u := NewSubscriberUtils(&fakeEvents{}, k)
	err := u.DeleteWorkflow("wf", agentData())
	if err == nil {
		t.Fatal("expected error when GetKubeConfig fails")
	}
}

func TestDeleteWorkflow_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	k := &fakeK8s{config: &rest.Config{Host: srv.URL}}
	u := NewSubscriberUtils(&fakeEvents{}, k)
	if err := u.DeleteWorkflow("wf-name", agentData()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
