package k8s

import (
	"sort"
	"strings"
	"testing"

	"subscriber/pkg/types"
)

// fakeGQL is a hand-written graphql.SubscriberGql used to capture SendRequest
// payloads and control MarshalGQLData behaviour without any network access.
type fakeGQL struct {
	marshalFn func(interface{}) (string, error)

	sendServer   string
	sendPayload  []byte
	sendResponse string
	sendErr      error
	sendCalled   bool
}

func (f *fakeGQL) SendRequest(server string, payload []byte) (string, error) {
	f.sendCalled = true
	f.sendServer = server
	f.sendPayload = payload
	return f.sendResponse, f.sendErr
}

func (f *fakeGQL) MarshalGQLData(data interface{}) (string, error) {
	if f.marshalFn != nil {
		return f.marshalFn(data)
	}
	// Default: return a quoted placeholder so payload slicing [1:len-1] works.
	return `"MARSHALLED"`, nil
}

func newK8sWithFakeGQL(f *fakeGQL) *k8sSubscriber {
	return &k8sSubscriber{gqlSubscriberServer: f}
}

func TestUpdateLabels(t *testing.T) {
	k := newK8sWithFakeGQL(&fakeGQL{})
	got := k.updateLabels(map[string]string{"app": "x", "tier": "backend"})
	sort.Strings(got)
	want := []string{"app=x", "tier=backend"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("label[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestUpdateLabels_Empty(t *testing.T) {
	k := newK8sWithFakeGQL(&fakeGQL{})
	if got := k.updateLabels(map[string]string{}); got != nil {
		t.Errorf("expected nil for empty labels, got %v", got)
	}
}

func TestGetKubernetesNamespaces_NamespaceScope(t *testing.T) {
	oldScope, oldNS := InfraScope, InfraNamespace
	defer func() { InfraScope, InfraNamespace = oldScope, oldNS }()
	InfraScope = "namespace"
	InfraNamespace = "litmus-ns"

	k := newK8sWithFakeGQL(&fakeGQL{})
	got, err := k.GetKubernetesNamespaces(types.KubeNamespaceRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 namespace, got %d", len(got))
	}
	if got[0].Name != "litmus-ns" {
		t.Errorf("namespace = %q, want litmus-ns", got[0].Name)
	}
}

func TestGenerateKubeNamespace_NamespaceScope(t *testing.T) {
	oldScope, oldNS := InfraScope, InfraNamespace
	defer func() { InfraScope, InfraNamespace = oldScope, oldNS }()
	InfraScope = "namespace"
	InfraNamespace = "litmus-ns"

	f := &fakeGQL{}
	k := newK8sWithFakeGQL(f)
	payload, err := k.GenerateKubeNamespace("cid-1", "ak-1", "v1", types.KubeNamespaceRequest{RequestID: "rq-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(payload)
	for _, want := range []string{
		`kubeNamespace(request:`,
		`infraID: \"cid-1\"`,
		`version: \"v1\"`,
		`accessKey: \"ak-1\"`,
		`requestID:\"rq-1\"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("payload missing %q\npayload: %s", want, s)
		}
	}
}

func TestGenerateKubeNamespace_MarshalError(t *testing.T) {
	oldScope, oldNS := InfraScope, InfraNamespace
	defer func() { InfraScope, InfraNamespace = oldScope, oldNS }()
	InfraScope = "namespace"
	InfraNamespace = "ns"

	f := &fakeGQL{marshalFn: func(interface{}) (string, error) {
		return "", errAlways
	}}
	k := newK8sWithFakeGQL(f)
	if _, err := k.GenerateKubeNamespace("c", "a", "v", types.KubeNamespaceRequest{}); err == nil {
		t.Fatal("expected marshal error to propagate")
	}
}

func TestSendKubeNamespaces_NamespaceScope(t *testing.T) {
	oldScope, oldNS := InfraScope, InfraNamespace
	defer func() { InfraScope, InfraNamespace = oldScope, oldNS }()
	InfraScope = "namespace"
	InfraNamespace = "ns"

	f := &fakeGQL{sendResponse: "OK"}
	k := newK8sWithFakeGQL(f)
	infra := map[string]string{"INFRA_ID": "i", "ACCESS_KEY": "a", "VERSION": "v", "SERVER_ADDR": "http://server"}
	if err := k.SendKubeNamespaces(infra, types.KubeNamespaceRequest{RequestID: "r"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !f.sendCalled {
		t.Fatal("expected SendRequest to be invoked")
	}
	if f.sendServer != "http://server" {
		t.Errorf("server = %q, want http://server", f.sendServer)
	}
	if !strings.Contains(string(f.sendPayload), "kubeNamespace(request:") {
		t.Errorf("payload not a kubeNamespace mutation: %s", f.sendPayload)
	}
}

func TestSendKubeNamespaces_SendError(t *testing.T) {
	oldScope, oldNS := InfraScope, InfraNamespace
	defer func() { InfraScope, InfraNamespace = oldScope, oldNS }()
	InfraScope = "namespace"
	InfraNamespace = "ns"

	f := &fakeGQL{sendErr: errAlways}
	k := newK8sWithFakeGQL(f)
	infra := map[string]string{"SERVER_ADDR": "http://server"}
	if err := k.SendKubeNamespaces(infra, types.KubeNamespaceRequest{}); err == nil {
		t.Fatal("expected error from SendRequest")
	}
}
