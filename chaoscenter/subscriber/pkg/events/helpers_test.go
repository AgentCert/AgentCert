package events

import (
	"testing"

	"subscriber/pkg/graphql"
	"subscriber/pkg/k8s"
	"subscriber/pkg/types"

	argoV1alpha1 "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned/typed/workflow/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// fakeGQL is a hand-written graphql.SubscriberGql that records SendRequest calls
// and delegates MarshalGQLData to the real implementation (it is pure).
type fakeGQL struct {
	realGQL      graphql.SubscriberGql
	sendResponse string
	sendErr      error
	sendCalled   bool
	sendPayloads [][]byte
}

func newFakeGQL() *fakeGQL {
	return &fakeGQL{realGQL: graphql.NewSubscriberGql(), sendResponse: "ok"}
}

func (f *fakeGQL) SendRequest(server string, payload []byte) (string, error) {
	f.sendCalled = true
	f.sendPayloads = append(f.sendPayloads, payload)
	return f.sendResponse, f.sendErr
}

func (f *fakeGQL) MarshalGQLData(data interface{}) (string, error) {
	return f.realGQL.MarshalGQLData(data)
}

// newEventsWithGQL builds a *subscriberEvents using the supplied fake gql so that
// SendRequest can be observed without network access.
func newEventsWithGQL(f *fakeGQL) *subscriberEvents {
	return &subscriberEvents{
		gqlSubscriberServer: f,
		subscriberK8s:       k8s.NewK8sSubscriber(f),
	}
}

// setKubeConfig points the k8s package's KubeConfig pointer at the given value
// for the duration of the test.
func setKubeConfig(t *testing.T, p *string) {
	t.Helper()
	old := k8s.KubeConfig
	k8s.KubeConfig = p
	t.Cleanup(func() { k8s.KubeConfig = old })
}

// fakeKubeK8s is a SubscriberK8s whose GetKubeConfig returns a config pointed at a
// chosen host, so argo client calls in events ops hit an httptest server.
type fakeKubeK8s struct {
	config *rest.Config
}

func (f *fakeKubeK8s) GetKubeConfig() (*rest.Config, error) { return f.config, nil }

func (f *fakeKubeK8s) GetLogs(string, string, string) (string, error)         { return "", nil }
func (f *fakeKubeK8s) CreatePodLog(types.PodLogRequest) (types.PodLog, error) { return types.PodLog{}, nil }
func (f *fakeKubeK8s) SendPodLogs(map[string]string, types.PodLogRequest)     {}
func (f *fakeKubeK8s) GenerateLogPayload(string, string, string, types.PodLogRequest) ([]byte, error) {
	return nil, nil
}
func (f *fakeKubeK8s) GetKubernetesNamespaces(types.KubeNamespaceRequest) ([]*types.KubeNamespace, error) {
	return nil, nil
}
func (f *fakeKubeK8s) GetKubernetesObjects(types.KubeObjRequest) (*types.KubeObject, error) {
	return nil, nil
}
func (f *fakeKubeK8s) GetObjectDataByNamespace(string, dynamic.Interface, schema.GroupVersionResource) ([]types.ObjectData, error) {
	return nil, nil
}
func (f *fakeKubeK8s) GenerateKubeObject(string, string, string, types.KubeObjRequest) ([]byte, error) {
	return nil, nil
}
func (f *fakeKubeK8s) GenerateKubeNamespace(string, string, string, types.KubeNamespaceRequest) ([]byte, error) {
	return nil, nil
}
func (f *fakeKubeK8s) SendKubeObjects(map[string]string, types.KubeObjRequest) error      { return nil }
func (f *fakeKubeK8s) SendKubeNamespaces(map[string]string, types.KubeNamespaceRequest) error {
	return nil
}
func (f *fakeKubeK8s) CheckComponentStatus(string) error                   { return nil }
func (f *fakeKubeK8s) IsAgentConfirmed() (bool, string, error)             { return false, "", nil }
func (f *fakeKubeK8s) AgentRegister(string) (bool, error)                  { return false, nil }
func (f *fakeKubeK8s) AgentOperations(types.Action) (*unstructured.Unstructured, error) {
	return nil, nil
}
func (f *fakeKubeK8s) AgentConfirm(map[string]string) ([]byte, error)      { return nil, nil }
func (f *fakeKubeK8s) GetGenericK8sClient() (*kubernetes.Clientset, error) { return nil, nil }
func (f *fakeKubeK8s) GetDynamicAndDiscoveryClient() (discovery.DiscoveryInterface, dynamic.Interface, error) {
	return nil, nil, nil
}
func (f *fakeKubeK8s) GenerateArgoClient(string) (argoV1alpha1.WorkflowInterface, error) {
	return nil, nil
}

// newEventsWithKubeHost builds events ops whose argo/kube client calls are routed
// to the given httptest host.
func newEventsWithKubeHost(t *testing.T, host string) *subscriberEvents {
	t.Helper()
	return &subscriberEvents{
		gqlSubscriberServer: graphql.NewSubscriberGql(),
		subscriberK8s:       &fakeKubeK8s{config: &rest.Config{Host: host}},
	}
}

// newEventsWithFakeGQL builds a *subscriberEvents wired with the real (stateless)
// graphql + k8s constructors. Used for methods that exercise pure logic and never
// actually reach the network/cluster (e.g. GenerateWorkflowPayload).
func newEventsWithFakeGQL(t *testing.T) *subscriberEvents {
	t.Helper()
	gql := graphql.NewSubscriberGql()
	subK8s := k8s.NewK8sSubscriber(gql)
	return &subscriberEvents{
		gqlSubscriberServer: gql,
		subscriberK8s:       subK8s,
	}
}
