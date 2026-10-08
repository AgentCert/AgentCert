package imageref

import (
	"os"
	"strings"
	"testing"
)

const jfrog = "infyartifactory.jfrog.io/docker-local"

// Same cases as scripts/tests/test-registry-tooling.sh (bash / Python parity).
func TestResolve(t *testing.T) {
	cases := []struct {
		registry, ns, in, want string
	}{
		{"", "agentcert", "mongo:5", "agentcert/mongo:5"},
		{"", "agentcert", "gaiadocker/iproute2", "agentcert/gaiadocker-iproute2:latest"},
		{"", "agentcert", "docker.io/library/mongo", "agentcert/mongo:latest"},
		{"", "agentcert", "quay.io/containers/kubernetes_mcp_server:v0.0.67", "agentcert/containers-kubernetes_mcp_server:v0.0.67"},
		{"", "agentcert", "registry.k8s.io/pause:3.9", "agentcert/pause:3.9"},
		{"", "agentcert", "quay.io/containers/x:v1", "quay.io/containers/x:v1"}, // no frozen copy
		{"", "agentcert", "localhost:5000/a/b", "localhost:5000/a/b:latest"},    // no frozen copy
		{"", "agentcert", "agentcert/certifier:latest", "agentcert/certifier:latest"},
		{"", "agentcert", "litmuschaos.docker.scarf.sh/litmuschaos/go-runner:latest", "agentcert/litmuschaos-go-runner:latest"},
		{"", "none", "quay.io/containers/x:v1", "quay.io/containers/x:v1"},
		// No frozen copy (not a deploy/images.txt mirror row): keeps its upstream name.
		{"", "agentcert", "nginx:1.25", "nginx:1.25"},
		{"", "agentcert", "registry.example.com/team/app:v2", "registry.example.com/team/app:v2"},
		{"", "agentcert", "devth/alpine-bench", "devth/alpine-bench:latest"}, // optional row, not mirrored
		{jfrog, "agentcert", "nginx:1.25", jfrog + "/nginx:1.25"},            // private registry: always prefixed
		{"", "", "mongo:5", "agentcert/mongo:5"},                             // empty namespace = default
		{jfrog, "agentcert", "mongo:5", jfrog + "/mongo:5"},
		{jfrog, "agentcert", "quay.io/containers/x:v1", jfrog + "/quay.io/containers/x:v1"},
		{jfrog, "agentcert", "cgr.dev/chainguard/minio", jfrog + "/cgr.dev/chainguard/minio:latest"},
		{jfrog, "agentcert", jfrog + "/mongo:5", jfrog + "/mongo:5"}, // idempotent
		{"https://infyartifactory.jfrog.io/ui/native/docker-local/", "", "mongo:5", jfrog + "/mongo:5"},
		{jfrog, "", "{{workflow.parameters.image}}", "{{workflow.parameters.image}}"},
		{jfrog, "", "", ""},
	}
	for _, c := range cases {
		if got := New(c.registry, c.ns).Resolve(c.in); got != c.want {
			t.Errorf("Resolve(%q) with registry=%q ns=%q = %q, want %q", c.in, c.registry, c.ns, got, c.want)
		}
	}
	// The flat-name mapping itself (used for images that have a frozen copy).
	for in, want := range map[string]string{
		"quay.io/containers/x:v1":   "agentcert/containers-x:v1",
		"localhost:5000/a/b:latest": "agentcert/a-b:latest",
		"agentcert/certifier:v1":    "agentcert/certifier:v1",
	} {
		if got := FlatRef(in, "agentcert"); got != want {
			t.Errorf("FlatRef(%q) = %q, want %q", in, got, want)
		}
	}
	// Idempotent under the frozen-copy rule too.
	r := New("", "")
	for _, in := range []string{"mongo:5", "quay.io/containers/x:v1", "litmuschaos/k8s:latest"} {
		once := r.Resolve(in)
		if twice := r.Resolve(once); twice != once {
			t.Errorf("not idempotent: %q -> %q -> %q", in, once, twice)
		}
	}
}

func TestNormalizeSource(t *testing.T) {
	for in, want := range map[string]string{"jfrog": "registry", "DockerHub": "registry", "registry": "registry", "local": "local", "": "def"} {
		if got := NormalizeSource(in, "def"); got != want {
			t.Errorf("NormalizeSource(%q) = %q, want %q", in, got, want)
		}
	}
}

// A ChaosExperiment + ChaosEngine as embedded in workflow artifacts.
const faultText = `apiVersion: litmuschaos.io/v1alpha1
kind: ChaosExperiment
spec:
  definition:
    image: "litmuschaos.docker.scarf.sh/litmuschaos/go-runner:latest"
    env:
      - name: LIB_IMAGE
        value: "litmuschaos.docker.scarf.sh/litmuschaos/go-runner:latest"
      - name: TC_IMAGE
        value: gaiadocker/iproute2
      - name: INVALID_IMAGE
        value: 'quay.io/it-bench/hello-bench-invalid:1.0.0'
      - name: INVALID_ARCH_IMAGE
        value: arm64v8/busybox:1.36.1-musl
      - name: TOTAL_CHAOS_DURATION
        value: "60"
      - name: DEBUG_IMAGE
        value: busybox:1.36
    image: agentcert/itbench-experiment:dev
`

func TestRewriteText(t *testing.T) {
	r := New(jfrog, "")
	got := r.RewriteText(faultText, func(ref string) bool { return strings.HasPrefix(ref, "agentcert/") })
	for _, want := range []string{
		`image: "` + jfrog + `/litmuschaos.docker.scarf.sh/litmuschaos/go-runner:latest"`,
		`value: "` + jfrog + `/litmuschaos.docker.scarf.sh/litmuschaos/go-runner:latest"`,
		`value: ` + jfrog + `/gaiadocker/iproute2:latest`,
		`value: ` + jfrog + `/busybox:1.36`,
		`value: 'quay.io/it-bench/hello-bench-invalid:1.0.0'`, // intentionally broken: untouched
		`value: arm64v8/busybox:1.36.1-musl`,                  // intentionally broken: untouched
		`value: "60"`,                                         // not an image setting
		`image: agentcert/itbench-experiment:dev`,             // skipped by the caller
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rewritten text missing %q:\n%s", want, got)
		}
	}
	// Open-source frozen copies.
	got = New("", "agentcert").RewriteText(faultText, nil)
	if !strings.Contains(got, `value: agentcert/gaiadocker-iproute2:latest`) ||
		!strings.Contains(got, `image: agentcert/itbench-experiment:dev`) {
		t.Errorf("frozen-copy rewrite wrong:\n%s", got)
	}
	// Nothing configured: byte-identical.
	if got := New("", "none").RewriteText(faultText, nil); got != faultText {
		t.Errorf("inactive resolver changed the text")
	}
}

const podText = `apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      initContainers:
        - name: wait
          image: busybox:latest
      containers:
        - name: app
          image: "ghcr.io/open-telemetry/demo:2.2.0-cart"
---
apiVersion: batch/v1
kind: CronJob
spec:
  jobTemplate:
    spec:
      template:
        spec:
          imagePullSecrets:
            - name: already-there
          containers:
            - name: job
              image: mongo
`

func TestPostRender(t *testing.T) {
	t.Setenv("ACE_IMAGE_REGISTRY", jfrog)
	t.Setenv("ACE_IMAGE_MIRROR_NAMESPACE", "")
	t.Setenv("ACE_IMAGE_PULL_SECRET", "registry-pull")
	got := PostRender(podText)
	for _, want := range []string{
		"image: " + jfrog + "/busybox:latest",
		`image: "` + jfrog + `/ghcr.io/open-telemetry/demo:2.2.0-cart"`,
		"image: " + jfrog + "/mongo:latest",
		"      imagePullSecrets:\n      - name: registry-pull\n      containers:",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("post-rendered output missing %q:\n%s", want, got)
		}
	}
	if n := strings.Count(got, "imagePullSecrets:"); n != 2 {
		t.Errorf("want 2 imagePullSecrets keys (1 added, 1 existing kept), got %d:\n%s", n, got)
	}
	if strings.Contains(got, "- name: registry-pull\n          containers:") {
		t.Errorf("pull secret added to a pod spec that already declares imagePullSecrets")
	}
}

func TestPullSecretName(t *testing.T) {
	t.Setenv("IMAGE_REGISTRY", "")
	t.Setenv("IMAGE_PULL_SECRET_NAME", "registry-pull")
	if got := PullSecretName(); got != "" {
		t.Errorf("public registry must not use a pull secret, got %q", got)
	}
	t.Setenv("IMAGE_REGISTRY", jfrog)
	if got := PullSecretName(); got != "registry-pull" {
		t.Errorf("PullSecretName() = %q", got)
	}
}

// TestParityFixture checks the Go rule against expectations produced by
// scripts/lib/registry.sh (scripts/tests/test-registry-tooling.sh --online
// writes the fixture: "<registry>\t<namespace>\t<ref>\t<want>" per line).
func TestParityFixture(t *testing.T) {
	path := os.Getenv("IMAGEREF_PARITY_FIXTURE")
	if path == "" {
		t.Skip("IMAGEREF_PARITY_FIXTURE not set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t") // the registry field may be empty: no trimming
		if len(f) != 4 {
			t.Fatalf("bad fixture line %q", line)
		}
		if got := New(f[0], f[1]).Resolve(f[2]); got != f[3] {
			t.Errorf("registry=%q ns=%q %q: Go %q, bash %q", f[0], f[1], f[2], got, f[3])
		}
		n++
	}
	t.Logf("%d fixture cases agree", n)
}
