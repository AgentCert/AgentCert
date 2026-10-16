package ops

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestNormalizeAgentSidecarImagePullPolicy(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "always", raw: "Always", want: string(corev1.PullAlways)},
		{name: "if not present", raw: "IfNotPresent", want: string(corev1.PullIfNotPresent)},
		{name: "never", raw: "Never", want: string(corev1.PullNever)},
		{name: "surrounding whitespace", raw: "  Never  ", want: string(corev1.PullNever)},
		{name: "empty defaults locally safe", raw: "", want: string(corev1.PullIfNotPresent)},
		{name: "invalid defaults locally safe", raw: "sometimes", want: string(corev1.PullIfNotPresent)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeAgentSidecarImagePullPolicy(tc.raw); got != tc.want {
				t.Fatalf("normalizeAgentSidecarImagePullPolicy(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
