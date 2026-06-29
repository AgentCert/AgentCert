package utils

import (
	"testing"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	dbSchemaProbe "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/probe"
)

func strPtr(s string) *string { return &s }

func TestAddKubernetesHTTPProbeProperties_GETDefaults(t *testing.T) {
	req := model.ProbeRequest{
		KubernetesHTTPProperties: &model.KubernetesHTTPProbeRequest{
			ProbeTimeout: "5s",
			Interval:     "2s",
			URL:          strPtr("http://example.com"),
			Method: &model.MethodRequest{
				Get: &model.GETRequest{}, // empty -> defaults
			},
		},
	}
	out := AddKubernetesHTTPProbeProperties(&dbSchemaProbe.Probe{}, req)
	if out.KubernetesHTTPProperties == nil {
		t.Fatal("expected http properties set")
	}
	if out.KubernetesHTTPProperties.URL != "http://example.com" {
		t.Errorf("URL = %q", out.KubernetesHTTPProperties.URL)
	}
	if out.KubernetesHTTPProperties.Method.GET == nil {
		t.Fatal("expected GET method")
	}
	if out.KubernetesHTTPProperties.Method.GET.Criteria != "oneOf" {
		t.Errorf("criteria default = %q, want oneOf", out.KubernetesHTTPProperties.Method.GET.Criteria)
	}
	if out.KubernetesHTTPProperties.Method.GET.ResponseCode != "200" {
		t.Errorf("responseCode default = %q, want 200", out.KubernetesHTTPProperties.Method.GET.ResponseCode)
	}
}

func TestAddKubernetesHTTPProbeProperties_POST(t *testing.T) {
	req := model.ProbeRequest{
		KubernetesHTTPProperties: &model.KubernetesHTTPProbeRequest{
			ProbeTimeout: "5s",
			Interval:     "2s",
			URL:          nil, // nil URL must default to empty string
			Method: &model.MethodRequest{
				Post: &model.POSTRequest{
					Criteria:     "==",
					ResponseCode: "201",
					ContentType:  strPtr("application/json"),
					Body:         strPtr("{}"),
				},
			},
		},
	}
	out := AddKubernetesHTTPProbeProperties(&dbSchemaProbe.Probe{}, req)
	if out.KubernetesHTTPProperties.URL != "" {
		t.Errorf("nil URL should yield empty string, got %q", out.KubernetesHTTPProperties.URL)
	}
	if out.KubernetesHTTPProperties.Method.POST == nil {
		t.Fatal("expected POST method")
	}
	if out.KubernetesHTTPProperties.Method.POST.ResponseCode != "201" {
		t.Errorf("responseCode = %q, want 201", out.KubernetesHTTPProperties.Method.POST.ResponseCode)
	}
	if out.KubernetesHTTPProperties.Method.POST.ContentType == nil || *out.KubernetesHTTPProperties.Method.POST.ContentType != "application/json" {
		t.Errorf("contentType = %v", out.KubernetesHTTPProperties.Method.POST.ContentType)
	}
}

func TestAddKubernetesCMDProbeProperties(t *testing.T) {
	t.Run("basic without source", func(t *testing.T) {
		req := model.ProbeRequest{
			KubernetesCMDProperties: &model.KubernetesCMDProbeRequest{
				ProbeTimeout: "5s",
				Interval:     "2s",
				Command:      "echo hello",
				Comparator: &model.ComparatorInput{
					Type: "string", Value: "hello", Criteria: "contains",
				},
			},
		}
		out, err := AddKubernetesCMDProbeProperties(&dbSchemaProbe.Probe{}, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.KubernetesCMDProperties.Command != "echo hello" {
			t.Errorf("command = %q", out.KubernetesCMDProperties.Command)
		}
		if out.KubernetesCMDProperties.Comparator.Criteria != "contains" {
			t.Errorf("comparator criteria = %q", out.KubernetesCMDProperties.Comparator.Criteria)
		}
	})

	t.Run("valid source json", func(t *testing.T) {
		src := `{"image":"alpine:latest"}`
		req := model.ProbeRequest{
			KubernetesCMDProperties: &model.KubernetesCMDProbeRequest{
				ProbeTimeout: "5s",
				Interval:     "2s",
				Command:      "ls",
				Comparator:   &model.ComparatorInput{Type: "string", Value: "x", Criteria: "contains"},
				Source:       &src,
			},
		}
		out, err := AddKubernetesCMDProbeProperties(&dbSchemaProbe.Probe{}, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.KubernetesCMDProperties.Source == nil || out.KubernetesCMDProperties.Source.Image != "alpine:latest" {
			t.Errorf("source not parsed: %+v", out.KubernetesCMDProperties.Source)
		}
	})

	t.Run("invalid source json errors", func(t *testing.T) {
		bad := "{not json"
		req := model.ProbeRequest{
			KubernetesCMDProperties: &model.KubernetesCMDProbeRequest{
				ProbeTimeout: "5s",
				Interval:     "2s",
				Command:      "ls",
				Comparator:   &model.ComparatorInput{Type: "string", Value: "x", Criteria: "contains"},
				Source:       &bad,
			},
		}
		if _, err := AddKubernetesCMDProbeProperties(&dbSchemaProbe.Probe{}, req); err == nil {
			t.Error("expected error for invalid source json")
		}
	})
}

func TestAddPROMProbeProperties(t *testing.T) {
	req := model.ProbeRequest{
		PromProperties: &model.PROMProbeRequest{
			ProbeTimeout: "5s",
			Interval:     "2s",
			Endpoint:     "http://prom:9090",
			Comparator:   &model.ComparatorInput{Type: "float", Value: "1", Criteria: ">="},
			Query:        strPtr("up"),
		},
	}
	out := AddPROMProbeProperties(&dbSchemaProbe.Probe{}, req)
	if out.PROMProperties == nil {
		t.Fatal("expected prom properties")
	}
	if out.PROMProperties.Endpoint != "http://prom:9090" {
		t.Errorf("endpoint = %q", out.PROMProperties.Endpoint)
	}
	if out.PROMProperties.Query == nil || *out.PROMProperties.Query != "up" {
		t.Errorf("query = %v", out.PROMProperties.Query)
	}
	if out.PROMProperties.Comparator.Criteria != ">=" {
		t.Errorf("comparator criteria = %q", out.PROMProperties.Comparator.Criteria)
	}
}

func TestAddK8SProbeProperties(t *testing.T) {
	req := model.ProbeRequest{
		K8sProperties: &model.K8SProbeRequest{
			ProbeTimeout: "5s",
			Interval:     "2s",
			Version:      "v1",
			Resource:     "pods",
			Operation:    "present",
			Group:        strPtr("apps"),
			Namespace:    strPtr("default"),
		},
	}
	out := AddK8SProbeProperties(&dbSchemaProbe.Probe{}, req)
	if out.K8SProperties == nil {
		t.Fatal("expected k8s properties")
	}
	if out.K8SProperties.Resource != "pods" || out.K8SProperties.Operation != "present" {
		t.Errorf("k8s resource/op not set: %+v", out.K8SProperties)
	}
	if out.K8SProperties.Group == nil || *out.K8SProperties.Group != "apps" {
		t.Errorf("group not set")
	}
}
