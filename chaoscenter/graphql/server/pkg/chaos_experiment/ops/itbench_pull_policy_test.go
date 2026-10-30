package ops

import (
	"strings"
	"testing"
)

// The ITBench image is built from the checkout and side-loaded by default, but
// pulled from a registry under a moving :dev tag when one is configured. A single
// hardcoded policy is wrong in one of those two modes, so it follows the source.
func TestItbenchPullPolicyFollowsImageSource(t *testing.T) {
	for _, tc := range []struct {
		name     string
		source   string
		override string
		want     string
	}{
		{name: "default is the side-loaded build", want: "Never"},
		{name: "explicit local", source: "local", want: "Never"},
		{name: "registry re-pulls the moving tag", source: "registry", want: "Always"},
		{name: "override wins", source: "registry", override: "IfNotPresent", want: "IfNotPresent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ITBENCH_EXPERIMENT_IMAGE_SOURCE", tc.source)
			t.Setenv("ITBENCH_EXPERIMENT_IMAGE_PULL_POLICY", tc.override)
			if got := itbenchPullPolicy(); got != tc.want {
				t.Fatalf("itbenchPullPolicy() = %q, want %q", got, tc.want)
			}
		})
	}
}

// The rewrite must find the policy belonging to the ITBench image and leave every
// other container's policy untouched, including after the image was resolved to a
// private registry.
func TestRewriteItbenchPullPolicyTargetsOnlyItsOwnImage(t *testing.T) {
	const manifest = `apiVersion: litmuschaos.io/v1alpha1
kind: ChaosExperiment
spec:
  definition:
    scope: Namespaced
    image: "infyartifactory.jfrog.io/docker-local/agentcert/itbench-experiment:latest"
    imagePullPolicy: Never
    command:
      - /itbench-experiment
---
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: unrelated
          image: litmuschaos/go-runner:3.0.0
          imagePullPolicy: IfNotPresent
`
	out, changed := rewriteItbenchPullPolicy(manifest, "Always")
	if changed != 1 {
		t.Fatalf("changed = %d, want exactly the ITBench policy rewritten", changed)
	}
	if !strings.Contains(out, "    imagePullPolicy: Always") {
		t.Fatal("the ITBench image kept its old pull policy")
	}
	if !strings.Contains(out, "          imagePullPolicy: IfNotPresent") {
		t.Fatal("an unrelated container's pull policy was rewritten")
	}
	if !strings.Contains(out, "      - /itbench-experiment") {
		t.Fatal("surrounding manifest content was corrupted")
	}

	// Idempotent: applying the same policy twice changes nothing.
	if _, again := rewriteItbenchPullPolicy(out, "Always"); again != 0 {
		t.Fatalf("second pass reported %d changes, want 0", again)
	}

	// A manifest without the ITBench image must be returned untouched.
	const other = "spec:\n  containers:\n    - image: busybox:1.36\n      imagePullPolicy: IfNotPresent\n"
	if got, count := rewriteItbenchPullPolicy(other, "Always"); count != 0 || got != other {
		t.Fatal("a manifest without the ITBench image must not be modified")
	}
}
