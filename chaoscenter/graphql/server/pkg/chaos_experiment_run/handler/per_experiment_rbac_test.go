package handler

import (
	"strings"
	"testing"

	k8srbacv1 "k8s.io/api/rbac/v1"
)

func TestPerExperimentChaosServiceAccountName(t *testing.T) {
	name := perExperimentChaosServiceAccountName("A_NOTIFY.ID with spaces_and_symbols_abcdefghijklmnopqrstuvwxyz0123456789")

	if len(name) > 63 {
		t.Fatalf("service account name length = %d, want <= 63", len(name))
	}
	if !strings.HasPrefix(name, "ace-chaos-") {
		t.Fatalf("service account name = %q, want ace-chaos prefix", name)
	}
	if strings.ContainsAny(name, " ._") {
		t.Fatalf("service account name = %q, want DNS-safe normalized name", name)
	}
}

func TestAppendUniqueNonEmpty(t *testing.T) {
	got := appendUniqueNonEmpty([]string{"itbench", "bookinfo"}, "bookinfo", " ", "otel-demo", "itbench")
	want := []string{"itbench", "bookinfo", "otel-demo"}

	if len(got) != len(want) {
		t.Fatalf("appendUniqueNonEmpty() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("appendUniqueNonEmpty() = %#v, want %#v", got, want)
		}
	}
}

func TestPerExperimentChaosRBACLabels(t *testing.T) {
	labels := perExperimentChaosRBACLabels("ace-chaos-run")

	if labels["agentcert.io/rbac-scope"] != "experiment-run" {
		t.Fatalf("rbac scope label = %q", labels["agentcert.io/rbac-scope"])
	}
	if labels["agentcert.io/chaos-sa"] != "ace-chaos-run" {
		t.Fatalf("chaos service account label = %q", labels["agentcert.io/chaos-sa"])
	}
}

func TestPerExperimentChaosRulesIncludeTargetWorkloadScaleButNoClusterScopedNodeAccess(t *testing.T) {
	rules := perExperimentChaosRules()

	if !ruleAllows(rules, "apps", "deployments/scale", "patch") {
		t.Fatalf("expected rules to allow patch apps/deployments/scale")
	}
	if !ruleAllows(rules, "apps", "deployments", "list") {
		t.Fatalf("expected rules to allow list apps/deployments")
	}
	if ruleAllows(rules, "", "nodes", "patch") {
		t.Fatalf("did not expect per-experiment namespaced rules to allow patch nodes")
	}
	if ruleAllows(rules, "scheduling.k8s.io", "priorityclasses", "create") {
		t.Fatalf("did not expect per-experiment namespaced rules to allow create priorityclasses")
	}
}

func ruleAllows(rules []k8srbacv1.PolicyRule, apiGroup, resource, verb string) bool {
	for _, rule := range rules {
		if !contains(rule.APIGroups, apiGroup) || !contains(rule.Resources, resource) || !contains(rule.Verbs, verb) {
			continue
		}
		return true
	}
	return false
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
