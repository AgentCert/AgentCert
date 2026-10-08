package ops

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"sigs.k8s.io/yaml"
)

const testRegistry = "infyartifactory.jfrog.io/docker-local"

func chaosChartsFile(t *testing.T, rel string) []byte {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "../../../../../../..")
	data, err := os.ReadFile(filepath.Join(repoRoot, "chaos-charts", rel))
	if err != nil {
		t.Skipf("chaos-charts not available: %v", err)
	}
	return data
}

// withITBenchFault appends a real ITBench fault definition (agentcert/itbench-experiment)
// as a raw artifact, the way the UI composes fault definitions into a workflow.
func withITBenchFault(t *testing.T, wf *v1alpha1.Workflow) {
	fault := chaosChartsFile(t, "faults/itbench/scaled-to-zero-kubernetes-workload/fault.yaml")
	wf.Spec.Templates = append(wf.Spec.Templates, v1alpha1.Template{
		Name: "install-itbench-fault",
		Inputs: v1alpha1.Inputs{Artifacts: []v1alpha1.Artifact{{
			Name:             "fault",
			ArtifactLocation: v1alpha1.ArtifactLocation{Raw: &v1alpha1.RawArtifact{Data: string(fault)}},
		}}},
	})
}

// isInstallerImage: install-app / install-agent are set from INSTALL_*_IMAGE
// (already resolved by setup.sh), not by ApplyRegistryImageOverrides.
func isInstallerImage(img string) bool {
	return strings.Contains(img, "agentcert-install-")
}

// loadExperiment reads a real experiment from chaos-charts.
func loadExperiment(t *testing.T, rel string) *v1alpha1.Workflow {
	t.Helper()
	data := chaosChartsFile(t, "experiments/"+rel)
	var wf v1alpha1.Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	return &wf
}

// allImages lists every container image and every raw artifact in a workflow.
func allImages(spec *v1alpha1.WorkflowSpec) (images []string, raw string) {
	var b strings.Builder
	for _, t := range spec.Templates {
		if t.Container != nil {
			images = append(images, t.Container.Image)
		}
		for _, a := range t.Inputs.Artifacts {
			if a.Raw != nil {
				b.WriteString(a.Raw.Data)
				b.WriteString("\n")
			}
		}
	}
	return images, b.String()
}

func setSources(t *testing.T, registry, litmus, itbench string) {
	t.Setenv("IMAGE_REGISTRY", registry)
	t.Setenv("IMAGE_MIRROR_NAMESPACE", "")
	t.Setenv("IMAGE_PULL_SECRET_NAME", "registry-pull")
	t.Setenv("LITMUS_IMAGES_SOURCE", litmus)
	t.Setenv("ITBENCH_EXPERIMENT_IMAGE_SOURCE", itbench)
}

func TestApplyRegistryImageOverridesPrivateRegistry(t *testing.T) {
	wf := loadExperiment(t, "otel-demo-itbench-starter/experiment.yaml")
	withITBenchFault(t, wf)
	setSources(t, testRegistry, "registry", "registry")
	ApplyRegistryImageOverrides(&wf.Spec)

	images, raw := allImages(&wf.Spec)
	for _, img := range images {
		if !strings.HasPrefix(img, testRegistry+"/") && !strings.Contains(img, "{{") && !isInstallerImage(img) {
			t.Errorf("template image not resolved: %q", img)
		}
	}
	if !strings.Contains(raw, testRegistry+"/agentcert/itbench-experiment:dev") {
		t.Errorf("itbench-experiment (registry source) not resolved in fault definitions")
	}
	if strings.Contains(raw, testRegistry+"/quay.io/it-bench/hello-bench-invalid") ||
		strings.Contains(raw, testRegistry+"/arm64v8/busybox") {
		t.Errorf("intentionally broken INVALID_*IMAGE was rewritten")
	}
	if len(wf.Spec.ImagePullSecrets) != 1 || wf.Spec.ImagePullSecrets[0].Name != "registry-pull" {
		t.Errorf("workflow imagePullSecrets = %v, want [registry-pull]", wf.Spec.ImagePullSecrets)
	}
	// Idempotent: a second pass changes nothing.
	before, beforeRaw := allImages(&wf.Spec)
	ApplyRegistryImageOverrides(&wf.Spec)
	after, afterRaw := allImages(&wf.Spec)
	if strings.Join(before, ",") != strings.Join(after, ",") || beforeRaw != afterRaw || len(wf.Spec.ImagePullSecrets) != 1 {
		t.Errorf("second ApplyRegistryImageOverrides pass changed the workflow")
	}
}

func TestApplyRegistryImageOverridesLocalSources(t *testing.T) {
	wf := loadExperiment(t, "otel-demo-itbench-starter/experiment.yaml")
	withITBenchFault(t, wf)
	wantImages, wantRaw := allImages(&wf.Spec)
	// Side-loaded images keep their public names; only the pull secret is added.
	setSources(t, testRegistry, "local", "local")
	ApplyRegistryImageOverrides(&wf.Spec)
	gotImages, gotRaw := allImages(&wf.Spec)
	if strings.Join(gotImages, ",") != strings.Join(wantImages, ",") || gotRaw != wantRaw {
		t.Errorf("local sources must keep public image names")
	}
	if len(wf.Spec.ImagePullSecrets) != 1 {
		t.Errorf("pull secret should still be attached with IMAGE_REGISTRY set")
	}
}

func TestApplyRegistryImageOverridesOpenSource(t *testing.T) {
	wf := loadExperiment(t, "sock-shop-single/experiment.yaml")
	setSources(t, "", "registry", "local")
	ApplyRegistryImageOverrides(&wf.Spec)
	images, raw := allImages(&wf.Spec)
	for _, img := range images {
		if !strings.HasPrefix(img, "agentcert/") && !strings.Contains(img, "{{") && !isInstallerImage(img) {
			t.Errorf("open-source image not mapped to a frozen agentcert/ copy: %q", img)
		}
	}
	if !strings.Contains(raw, "agentcert/litmuschaos-go-runner:latest") {
		t.Errorf("fault definition go-runner not mapped to agentcert/litmuschaos-go-runner:latest")
	}
	if len(wf.Spec.ImagePullSecrets) != 0 {
		t.Errorf("public registry must not add imagePullSecrets, got %v", wf.Spec.ImagePullSecrets)
	}
}

func TestWorkflowImage(t *testing.T) {
	setSources(t, testRegistry, "registry", "local")
	if got := WorkflowImage("litmuschaos/k8s:latest"); got != testRegistry+"/litmuschaos/k8s:latest" {
		t.Errorf("WorkflowImage = %q", got)
	}
	setSources(t, testRegistry, "local", "local")
	if got := WorkflowImage("busybox:1.36"); got != "busybox:1.36" {
		t.Errorf("local runtime images must keep their name, got %q", got)
	}
}

func TestSplitChartImage(t *testing.T) {
	cases := map[string][3]string{
		"agentcert/agent-sidecar:latest":                       {"docker.io", "agentcert/agent-sidecar", "latest"},
		"agentcert/agent-sidecar":                              {"docker.io", "agentcert/agent-sidecar", "latest"},
		testRegistry + "/agentcert/agent-sidecar:5c3141cada1f": {"infyartifactory.jfrog.io", "docker-local/agentcert/agent-sidecar", "5c3141cada1f"},
		"localhost:5000/agentcert/agent-sidecar:dev":           {"localhost:5000", "agentcert/agent-sidecar", "dev"},
		"registry.example.com:8443/team/agent-sidecar":         {"registry.example.com:8443", "team/agent-sidecar", "latest"},
	}
	for in, want := range cases {
		r, repo, tag := splitChartImage(in)
		if [3]string{r, repo, tag} != want {
			t.Errorf("splitChartImage(%q) = %q %q %q, want %v", in, r, repo, tag, want)
		}
		// The chart renders "<registry>/<repository>:<tag>"; that must be the same image.
		rendered := r + "/" + repo + ":" + tag
		if r == "docker.io" {
			rendered = repo + ":" + tag
		}
		if !strings.HasPrefix(in, rendered[:strings.LastIndex(rendered, ":")]) {
			t.Errorf("chart would render %q for %q", rendered, in)
		}
	}
}
