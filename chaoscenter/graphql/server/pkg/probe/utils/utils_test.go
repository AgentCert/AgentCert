package utils

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/litmuschaos/chaos-operator/api/litmuschaos/v1alpha1"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	dbChaosExperiment "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment"
)

func TestProbeInputsToProbeRequestConverter(t *testing.T) {
	t.Run("missing timeout/interval returns error", func(t *testing.T) {
		_, err := ProbeInputsToProbeRequestConverter(v1alpha1.ProbeAttributes{
			Name: "p",
			Type: string(model.ProbeTypeHTTPProbe),
		})
		if err == nil || !strings.Contains(err.Error(), "ProbeTimeout and Interval") {
			t.Fatalf("expected timeout/interval error, got %v", err)
		}
	})

	base := v1alpha1.RunProperty{ProbeTimeout: "5s", Interval: "2s"}

	t.Run("http GET probe with defaults", func(t *testing.T) {
		in := v1alpha1.ProbeAttributes{
			Name:          "httpp",
			Type:          string(model.ProbeTypeHTTPProbe),
			RunProperties: base,
			HTTPProbeInputs: &v1alpha1.HTTPProbeInputs{
				URL: "http://example.com",
				Method: v1alpha1.HTTPMethod{
					Get: &v1alpha1.GetMethod{}, // empty -> defaults applied
				},
			},
		}
		out, err := ProbeInputsToProbeRequestConverter(in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Type != model.ProbeTypeHTTPProbe {
			t.Errorf("type = %v", out.Type)
		}
		if out.KubernetesHTTPProperties == nil || out.KubernetesHTTPProperties.Method.Get == nil {
			t.Fatal("expected http GET properties")
		}
		if out.KubernetesHTTPProperties.Method.Get.Criteria != "oneOf" {
			t.Errorf("default criteria = %q, want oneOf", out.KubernetesHTTPProperties.Method.Get.Criteria)
		}
		if out.KubernetesHTTPProperties.Method.Get.ResponseCode != "200" {
			t.Errorf("default responseCode = %q, want 200", out.KubernetesHTTPProperties.Method.Get.ResponseCode)
		}
		if !strings.HasPrefix(out.Name, "httpp-") {
			t.Errorf("name should be suffixed with uuid, got %q", out.Name)
		}
		if out.InfrastructureType != model.InfrastructureTypeKubernetes {
			t.Errorf("infra type = %v", out.InfrastructureType)
		}
	})

	t.Run("http probe without method errors", func(t *testing.T) {
		in := v1alpha1.ProbeAttributes{
			Name:            "h",
			Type:            string(model.ProbeTypeHTTPProbe),
			RunProperties:   base,
			HTTPProbeInputs: &v1alpha1.HTTPProbeInputs{URL: "http://x"},
		}
		if _, err := ProbeInputsToProbeRequestConverter(in); err == nil {
			t.Error("expected GET/POST method error")
		}
	})

	t.Run("prom probe requires endpoint", func(t *testing.T) {
		in := v1alpha1.ProbeAttributes{
			Name:            "prom",
			Type:            string(model.ProbeTypePromProbe),
			RunProperties:   base,
			PromProbeInputs: &v1alpha1.PromProbeInputs{},
		}
		if _, err := ProbeInputsToProbeRequestConverter(in); err == nil {
			t.Error("expected endpoint error")
		}

		in.PromProbeInputs.Endpoint = "http://prom:9090"
		out, err := ProbeInputsToProbeRequestConverter(in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.PromProperties == nil || out.PromProperties.Endpoint != "http://prom:9090" {
			t.Errorf("prom endpoint not set")
		}
	})

	t.Run("k8s probe requires resource/operation/version", func(t *testing.T) {
		in := v1alpha1.ProbeAttributes{
			Name:           "k8s",
			Type:           string(model.ProbeTypeK8sProbe),
			RunProperties:  base,
			K8sProbeInputs: &v1alpha1.K8sProbeInputs{Resource: "pods"},
		}
		if _, err := ProbeInputsToProbeRequestConverter(in); err == nil {
			t.Error("expected missing field error")
		}

		in.K8sProbeInputs.Operation = "present"
		in.K8sProbeInputs.Version = "v1"
		out, err := ProbeInputsToProbeRequestConverter(in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.K8sProperties == nil || out.K8sProperties.Resource != "pods" {
			t.Errorf("k8s properties not populated")
		}
	})

	t.Run("cmd probe", func(t *testing.T) {
		in := v1alpha1.ProbeAttributes{
			Name:          "cmd",
			Type:          string(model.ProbeTypeCmdProbe),
			RunProperties: base,
			CmdProbeInputs: &v1alpha1.CmdProbeInputs{
				Command: "echo hi",
				Comparator: v1alpha1.ComparatorInfo{
					Type: "string", Criteria: "contains", Value: "hi",
				},
			},
		}
		out, err := ProbeInputsToProbeRequestConverter(in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.KubernetesCMDProperties == nil || out.KubernetesCMDProperties.Command != "echo hi" {
			t.Errorf("cmd properties not populated")
		}
	})
}

func TestInsertProbeRefAnnotation(t *testing.T) {
	t.Run("adds annotation when none present", func(t *testing.T) {
		raw := "metadata:\n  name: my-engine\nkind: ChaosEngine\n"
		out, err := InsertProbeRefAnnotation(raw, "probe-ref-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var parsed map[string]interface{}
		if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
			t.Fatalf("output not valid yaml: %v", err)
		}
		meta := parsed["metadata"].(map[string]interface{})
		ann := meta["annotations"].(map[string]interface{})
		if ann["probeRef"] != "probe-ref-1" {
			t.Errorf("probeRef = %v, want probe-ref-1", ann["probeRef"])
		}
	})

	t.Run("merges into existing annotations", func(t *testing.T) {
		raw := "metadata:\n  name: e\n  annotations:\n    existing: keep\n"
		out, err := InsertProbeRefAnnotation(raw, "ref")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "existing: keep") || !strings.Contains(out, "probeRef: ref") {
			t.Errorf("expected both annotations preserved, got:\n%s", out)
		}
	})

	t.Run("errors when metadata missing", func(t *testing.T) {
		if _, err := InsertProbeRefAnnotation("kind: Foo\n", "ref"); err == nil {
			t.Error("expected metadata-not-found error")
		}
	})

	t.Run("errors on invalid yaml", func(t *testing.T) {
		if _, err := InsertProbeRefAnnotation("\t: not valid", "ref"); err == nil {
			t.Error("expected yaml error")
		}
	})
}

// buildWorkflowManifest builds a minimal Argo Workflow JSON manifest containing
// a single template with a ChaosEngine artifact carrying a probe annotation.
func buildWorkflowManifest(t *testing.T, cron bool) string {
	t.Helper()
	annotations, _ := json.Marshal([]dbChaosExperiment.ProbeAnnotations{{Name: "my-probe"}})
	engine := map[string]interface{}{
		"kind":       "ChaosEngine",
		"apiVersion": "litmuschaos.io/v1alpha1",
		"metadata": map[string]interface{}{
			"name":        "engine",
			"annotations": map[string]string{"probeRef": string(annotations)},
		},
	}
	engineJSON, _ := json.Marshal(engine)

	template := map[string]interface{}{
		"name": "pod-delete",
		"inputs": map[string]interface{}{
			"artifacts": []map[string]interface{}{
				{
					"name": "pod-delete",
					"raw":  map[string]interface{}{"data": string(engineJSON)},
				},
			},
		},
	}

	var manifest map[string]interface{}
	if cron {
		manifest = map[string]interface{}{
			"kind": "CronWorkflow",
			"spec": map[string]interface{}{
				"workflowSpec": map[string]interface{}{
					"templates": []interface{}{template},
				},
			},
		}
	} else {
		manifest = map[string]interface{}{
			"kind": "Workflow",
			"spec": map[string]interface{}{
				"templates": []interface{}{template},
			},
		}
	}
	b, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	return string(b)
}

func TestParseProbesFromManifest(t *testing.T) {
	nonCron := dbChaosExperiment.ChaosExperimentType("experiment")
	cron := dbChaosExperiment.ChaosExperimentType("cronexperiment")

	t.Run("non-cron manifest extracts probes", func(t *testing.T) {
		probes, err := ParseProbesFromManifest(&nonCron, buildWorkflowManifest(t, false))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(probes) != 1 || probes[0].FaultName != "pod-delete" {
			t.Fatalf("unexpected probes: %+v", probes)
		}
		if len(probes[0].ProbeNames) != 1 || probes[0].ProbeNames[0] != "my-probe" {
			t.Errorf("probe names = %v, want [my-probe]", probes[0].ProbeNames)
		}
	})

	t.Run("cron manifest extracts probes", func(t *testing.T) {
		probes, err := ParseProbesFromManifest(&cron, buildWorkflowManifest(t, true))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(probes) != 1 || probes[0].FaultName != "pod-delete" {
			t.Fatalf("unexpected probes: %+v", probes)
		}
	})

	t.Run("invalid json errors", func(t *testing.T) {
		if _, err := ParseProbesFromManifest(&nonCron, "{not json"); err == nil {
			t.Error("expected unmarshal error")
		}
	})
}

func TestParseProbesFromManifestForRuns(t *testing.T) {
	nonCron := dbChaosExperiment.ChaosExperimentType("experiment")
	cron := dbChaosExperiment.ChaosExperimentType("cronexperiment")

	probes, err := ParseProbesFromManifestForRuns(&nonCron, buildWorkflowManifest(t, false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(probes) != 1 || probes[0].ProbeNames[0] != "my-probe" {
		t.Fatalf("unexpected probes: %+v", probes)
	}

	cronProbes, err := ParseProbesFromManifestForRuns(&cron, buildWorkflowManifest(t, true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cronProbes) != 1 {
		t.Errorf("cron probes len = %d, want 1", len(cronProbes))
	}

	if _, err := ParseProbesFromManifestForRuns(&nonCron, "garbage"); err == nil {
		t.Error("expected error for invalid manifest")
	}
}

func TestParseProbeRefAnnotation(t *testing.T) {
	// Regression: a ChaosEngine assembled from a blank Chaos Studio canvas
	// carries a "probeRef" annotation whose value is "" or "[]" for faults with
	// no probes (the ITBench teardown/uninstall steps in particular), and may
	// carry unrelated annotation keys. None of these are parse errors.
	cases := []struct {
		name        string
		annotations map[string]string
		wantNames   []string
		wantErr     bool
	}{
		{"nil map", nil, nil, false},
		{"no probeRef key", map[string]string{"step_pod_name": "{{pod.name}}"}, nil, false},
		{"empty probeRef", map[string]string{"probeRef": ""}, nil, false},
		{"whitespace probeRef", map[string]string{"probeRef": "  "}, nil, false},
		{"empty array probeRef", map[string]string{"probeRef": "[]"}, nil, false},
		{"null probeRef", map[string]string{"probeRef": "null"}, nil, false},
		{
			"foreign key with non-json value is ignored",
			map[string]string{"probeRef": "[]", "litmuschaos.io/multiRunEnabled": "true"},
			nil, false,
		},
		{
			"populated probeRef",
			map[string]string{"probeRef": `[{"name":"p1","mode":"SOT"},{"name":"p2","mode":"Edge"}]`},
			[]string{"p1", "p2"}, false,
		},
		{"malformed probeRef errors", map[string]string{"probeRef": "[{"}, nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			names, err := ParseProbeRefNames(tc.annotations)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got names=%v", names)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Join(names, ",") != strings.Join(tc.wantNames, ",") {
				t.Errorf("names = %v, want %v", names, tc.wantNames)
			}
		})
	}
}
