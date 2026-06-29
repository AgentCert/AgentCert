package requests

import (
	"errors"
	"testing"

	"subscriber/pkg/types"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	v1alpha12 "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned/typed/workflow/v1alpha1"
)

// fakeK8s records which SubscriberK8s methods RequestProcessor invokes. Only the
// methods exercised by RequestProcessor carry behaviour; the rest are no-ops that
// satisfy the interface.
type fakeK8s struct {
	sendKubeObjectsCalled    bool
	sendKubeNamespacesCalled bool
	sendPodLogsCalled        bool
	agentOpsCalled           bool

	lastKubeObjReq   types.KubeObjRequest
	lastKubeNSReq    types.KubeNamespaceRequest
	lastPodLogReq    types.PodLogRequest
	lastAction       types.Action
	sendKubeObjErr   error
	sendKubeNSErr    error
	agentOpsErr      error
}

func (f *fakeK8s) SendKubeObjects(infraData map[string]string, r types.KubeObjRequest) error {
	f.sendKubeObjectsCalled = true
	f.lastKubeObjReq = r
	return f.sendKubeObjErr
}
func (f *fakeK8s) SendKubeNamespaces(infraData map[string]string, r types.KubeNamespaceRequest) error {
	f.sendKubeNamespacesCalled = true
	f.lastKubeNSReq = r
	return f.sendKubeNSErr
}
func (f *fakeK8s) SendPodLogs(infraData map[string]string, r types.PodLogRequest) {
	f.sendPodLogsCalled = true
	f.lastPodLogReq = r
}
func (f *fakeK8s) AgentOperations(a types.Action) (*unstructured.Unstructured, error) {
	f.agentOpsCalled = true
	f.lastAction = a
	return nil, f.agentOpsErr
}

// Unused-by-RequestProcessor methods.
func (f *fakeK8s) GetLogs(string, string, string) (string, error)          { return "", nil }
func (f *fakeK8s) CreatePodLog(types.PodLogRequest) (types.PodLog, error)  { return types.PodLog{}, nil }
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
func (f *fakeK8s) CheckComponentStatus(string) error                  { return nil }
func (f *fakeK8s) IsAgentConfirmed() (bool, string, error)            { return false, "", nil }
func (f *fakeK8s) AgentRegister(string) (bool, error)                 { return false, nil }
func (f *fakeK8s) AgentConfirm(map[string]string) ([]byte, error)     { return nil, nil }
func (f *fakeK8s) GetKubeConfig() (*rest.Config, error)               { return nil, nil }
func (f *fakeK8s) GetGenericK8sClient() (*kubernetes.Clientset, error) { return nil, nil }
func (f *fakeK8s) GetDynamicAndDiscoveryClient() (discovery.DiscoveryInterface, dynamic.Interface, error) {
	return nil, nil, nil
}
func (f *fakeK8s) GenerateArgoClient(string) (v1alpha12.WorkflowInterface, error) { return nil, nil }

// fakeUtils records WorkflowRequest invocations.
type fakeUtils struct {
	called      bool
	lastReqType string
	lastExt     string
	err         error
}

func (f *fakeUtils) WorkflowRequest(agentData map[string]string, requestType, externalData, uuid string) error {
	f.called = true
	f.lastReqType = requestType
	f.lastExt = externalData
	return f.err
}
func (f *fakeUtils) DeleteWorkflow(string, map[string]string) error { return nil }

// rawDataFor builds a RawData with the given request type and external data.
func rawDataFor(requestType, externalData string) types.RawData {
	return types.RawData{
		Type: "data",
		Payload: types.Payload{
			Data: types.Data{
				InfraConnect: types.InfraConnect{
					Action: types.Action{
						RequestID:    "req-1",
						RequestType:  requestType,
						ExternalData: externalData,
					},
				},
			},
		},
	}
}

func TestRequestProcessor_KubeObject(t *testing.T) {
	k := &fakeK8s{}
	req := NewSubscriberRequests(k, &fakeUtils{})

	ext := `{"namespace":"ns1","objectType":"pods"}`
	err := req.RequestProcessor(map[string]string{}, rawDataFor("kubeobject", ext))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !k.sendKubeObjectsCalled {
		t.Fatal("expected SendKubeObjects to be called")
	}
	if k.lastKubeObjReq.RequestID != "req-1" {
		t.Errorf("RequestID = %q, want req-1", k.lastKubeObjReq.RequestID)
	}
	if k.lastKubeObjReq.Namespace != "ns1" {
		t.Errorf("Namespace = %q, want ns1", k.lastKubeObjReq.Namespace)
	}
}

func TestRequestProcessor_KubeObject_BadJSON(t *testing.T) {
	k := &fakeK8s{}
	req := NewSubscriberRequests(k, &fakeUtils{})
	err := req.RequestProcessor(map[string]string{}, rawDataFor("kubeobjects", "not-json"))
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
	if k.sendKubeObjectsCalled {
		t.Error("SendKubeObjects should not be called on bad JSON")
	}
}

func TestRequestProcessor_KubeObject_SendError(t *testing.T) {
	k := &fakeK8s{sendKubeObjErr: errors.New("boom")}
	req := NewSubscriberRequests(k, &fakeUtils{})
	err := req.RequestProcessor(map[string]string{}, rawDataFor("kubeobject", `{}`))
	if err == nil {
		t.Fatal("expected error from SendKubeObjects")
	}
}

func TestRequestProcessor_KubeNamespace(t *testing.T) {
	k := &fakeK8s{}
	req := NewSubscriberRequests(k, &fakeUtils{})
	err := req.RequestProcessor(map[string]string{}, rawDataFor("kubenamespace", `{"infraID":"i1"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !k.sendKubeNamespacesCalled {
		t.Fatal("expected SendKubeNamespaces to be called")
	}
	if k.lastKubeNSReq.RequestID != "req-1" {
		t.Errorf("RequestID = %q, want req-1", k.lastKubeNSReq.RequestID)
	}
}

func TestRequestProcessor_KubeNamespace_BadJSON(t *testing.T) {
	k := &fakeK8s{}
	req := NewSubscriberRequests(k, &fakeUtils{})
	err := req.RequestProcessor(map[string]string{}, rawDataFor("kubenamespaces", "garbage"))
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestRequestProcessor_Logs(t *testing.T) {
	k := &fakeK8s{}
	req := NewSubscriberRequests(k, &fakeUtils{})
	ext := `{"podName":"pod-x","podNamespace":"ns"}`
	err := req.RequestProcessor(map[string]string{}, rawDataFor("logs", ext))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !k.sendPodLogsCalled {
		t.Fatal("expected SendPodLogs to be called")
	}
	if k.lastPodLogReq.PodName != "pod-x" {
		t.Errorf("PodName = %q, want pod-x", k.lastPodLogReq.PodName)
	}
	if k.lastPodLogReq.RequestID != "req-1" {
		t.Errorf("RequestID = %q, want req-1", k.lastPodLogReq.RequestID)
	}
}

func TestRequestProcessor_Logs_BadJSON(t *testing.T) {
	k := &fakeK8s{}
	req := NewSubscriberRequests(k, &fakeUtils{})
	err := req.RequestProcessor(map[string]string{}, rawDataFor("logs", "bad"))
	if err == nil {
		t.Fatal("expected error reading external-data")
	}
	if k.sendPodLogsCalled {
		t.Error("SendPodLogs should not be called on bad JSON")
	}
}

func TestRequestProcessor_AgentOperations(t *testing.T) {
	for _, rt := range []string{"create", "update", "delete", "get"} {
		t.Run(rt, func(t *testing.T) {
			k := &fakeK8s{}
			req := NewSubscriberRequests(k, &fakeUtils{})
			err := req.RequestProcessor(map[string]string{}, rawDataFor(rt, ""))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !k.agentOpsCalled {
				t.Fatalf("expected AgentOperations to be called for %q", rt)
			}
			if k.lastAction.RequestType != rt {
				t.Errorf("RequestType = %q, want %q", k.lastAction.RequestType, rt)
			}
		})
	}
}

func TestRequestProcessor_AgentOperations_Error(t *testing.T) {
	k := &fakeK8s{agentOpsErr: errors.New("op failed")}
	req := NewSubscriberRequests(k, &fakeUtils{})
	err := req.RequestProcessor(map[string]string{}, rawDataFor("create", ""))
	if err == nil {
		t.Fatal("expected error from AgentOperations")
	}
}

func TestRequestProcessor_WorkflowRequests(t *testing.T) {
	for _, rt := range []string{"workflow_delete", "workflow_run_delete", "workflow_run_stop"} {
		t.Run(rt, func(t *testing.T) {
			u := &fakeUtils{}
			req := NewSubscriberRequests(&fakeK8s{}, u)
			err := req.RequestProcessor(map[string]string{}, rawDataFor(rt, "ext-data"))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !u.called {
				t.Fatalf("expected WorkflowRequest to be called for %q", rt)
			}
			if u.lastReqType != rt {
				t.Errorf("requestType = %q, want %q", u.lastReqType, rt)
			}
			if u.lastExt != "ext-data" {
				t.Errorf("externalData = %q, want ext-data", u.lastExt)
			}
		})
	}
}

func TestRequestProcessor_WorkflowRequest_Error(t *testing.T) {
	u := &fakeUtils{err: errors.New("wf failed")}
	req := NewSubscriberRequests(&fakeK8s{}, u)
	err := req.RequestProcessor(map[string]string{}, rawDataFor("workflow_delete", "x"))
	if err == nil {
		t.Fatal("expected error from WorkflowRequest")
	}
}

func TestRequestProcessor_UnknownRequestType(t *testing.T) {
	k := &fakeK8s{}
	u := &fakeUtils{}
	req := NewSubscriberRequests(k, u)
	err := req.RequestProcessor(map[string]string{}, rawDataFor("somethingelse", ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if k.sendKubeObjectsCalled || k.sendKubeNamespacesCalled || k.sendPodLogsCalled || k.agentOpsCalled || u.called {
		t.Error("no handler should fire for an unknown request type")
	}
}
