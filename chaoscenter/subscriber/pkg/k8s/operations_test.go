package k8s

import (
	"errors"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var errAlways = errors.New("always fails")

func TestGetLiveCheckMaxTries(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want int
	}{
		{"unset uses default", "", DefaultLiveCheckMaxTries},
		{"valid positive", "5", 5},
		{"zero falls back to default", "0", DefaultLiveCheckMaxTries},
		{"negative falls back to default", "-3", DefaultLiveCheckMaxTries},
		{"non-numeric falls back to default", "abc", DefaultLiveCheckMaxTries},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env == "" {
				t.Setenv("LIVE_CHECK_MAX_TRIES", "")
			} else {
				t.Setenv("LIVE_CHECK_MAX_TRIES", tt.env)
			}
			if got := getLiveCheckMaxTries(); got != tt.want {
				t.Errorf("getLiveCheckMaxTries() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestAddCustomLabels(t *testing.T) {
	t.Run("adds to existing labels", func(t *testing.T) {
		obj := &unstructured.Unstructured{}
		obj.SetLabels(map[string]string{"existing": "yes"})
		addCustomLabels(obj, map[string]string{"updated_by": "alice"})
		got := obj.GetLabels()
		if got["existing"] != "yes" {
			t.Errorf("existing label lost: %v", got)
		}
		if got["updated_by"] != "alice" {
			t.Errorf("custom label not added: %v", got)
		}
	})

	t.Run("creates label map when none exist", func(t *testing.T) {
		obj := &unstructured.Unstructured{}
		addCustomLabels(obj, map[string]string{"k": "v"})
		if obj.GetLabels()["k"] != "v" {
			t.Errorf("label not added to empty object: %v", obj.GetLabels())
		}
	})
}

func TestCheckComponentStatus_EmptyEnv(t *testing.T) {
	k := newK8sWithFakeGQL(&fakeGQL{})
	err := k.CheckComponentStatus("")
	if err == nil {
		t.Fatal("expected error for empty component env")
	}
	if !strings.Contains(err.Error(), "components not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentConfirm(t *testing.T) {
	f := &fakeGQL{sendResponse: `{"data":{"confirmInfraRegistration":{"isInfraConfirmed":true}}}`}
	k := newK8sWithFakeGQL(f)
	infra := map[string]string{
		"INFRA_ID":    "infra-99",
		"VERSION":     "1.2.3",
		"ACCESS_KEY":  "secret-key",
		"SERVER_ADDR": "http://server",
	}
	resp, err := k.AgentConfirm(infra)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp) != f.sendResponse {
		t.Errorf("response = %q, want %q", resp, f.sendResponse)
	}
	if !f.sendCalled {
		t.Fatal("expected SendRequest to be invoked")
	}
	p := string(f.sendPayload)
	for _, want := range []string{
		`confirmInfraRegistration`,
		`infraID: \"infra-99\"`,
		`version: \"1.2.3\"`,
		`accessKey: \"secret-key\"`,
	} {
		if !strings.Contains(p, want) {
			t.Errorf("AgentConfirm payload missing %q\npayload: %s", want, p)
		}
	}
	if f.sendServer != "http://server" {
		t.Errorf("server = %q, want http://server", f.sendServer)
	}
}

func TestAgentConfirm_SendError(t *testing.T) {
	f := &fakeGQL{sendErr: errAlways}
	k := newK8sWithFakeGQL(f)
	if _, err := k.AgentConfirm(map[string]string{}); err == nil {
		t.Fatal("expected error from SendRequest")
	}
}
