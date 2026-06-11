package k8s

import (
	"strings"
	"testing"

	"subscriber/pkg/types"
)

func TestCreatePodLog_LogFetchFailsGracefully(t *testing.T) {
	// Point KubeConfig at "" -> GetKubeConfig uses InClusterConfig which fails
	// outside a cluster, so GetLogs errors and CreatePodLog records the fallback.
	empty := ""
	oldCfg := KubeConfig
	KubeConfig = &empty
	defer func() { KubeConfig = oldCfg }()

	k := newK8sWithFakeGQL(&fakeGQL{})
	got, err := k.CreatePodLog(types.PodLogRequest{PodName: "p", PodNamespace: "ns", PodType: "pod"})
	if err != nil {
		t.Fatalf("CreatePodLog should not return error, got: %v", err)
	}
	if got.MainPod != "Failed to get argo pod logs" {
		t.Errorf("MainPod = %q, want fallback message", got.MainPod)
	}
	if got.ChaosPod != nil {
		t.Errorf("ChaosPod should be nil for non-chaosengine pod, got %v", got.ChaosPod)
	}
}

func TestGenerateLogPayload(t *testing.T) {
	empty := ""
	oldCfg := KubeConfig
	KubeConfig = &empty
	defer func() { KubeConfig = oldCfg }()

	f := &fakeGQL{}
	k := newK8sWithFakeGQL(f)
	payload, err := k.GenerateLogPayload("cid-7", "ak-7", "v9", types.PodLogRequest{
		RequestID:       "rq-7",
		ExperimentRunID: "run-7",
		PodName:         "pod-7",
		PodType:         "pod",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(payload)
	for _, want := range []string{
		`podLog(request:`,
		`infraID: \"cid-7\"`,
		`requestID:\"rq-7\"`,
		`experimentRunID: \"run-7\"`,
		`podName: \"pod-7\"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("payload missing %q\npayload: %s", want, s)
		}
	}
}

func TestSendPodLogs_InvokesSendRequest(t *testing.T) {
	empty := ""
	oldCfg := KubeConfig
	KubeConfig = &empty
	defer func() { KubeConfig = oldCfg }()

	f := &fakeGQL{sendResponse: "ack"}
	k := newK8sWithFakeGQL(f)
	infra := map[string]string{"INFRA_ID": "i", "ACCESS_KEY": "a", "VERSION": "v", "SERVER_ADDR": "http://srv"}
	k.SendPodLogs(infra, types.PodLogRequest{PodName: "p", PodType: "pod"})
	if !f.sendCalled {
		t.Fatal("expected SendRequest to be invoked")
	}
	if f.sendServer != "http://srv" {
		t.Errorf("server = %q, want http://srv", f.sendServer)
	}
}
