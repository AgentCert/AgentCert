package handler

import (
	"strings"
	"testing"

	k8srbacv1 "k8s.io/api/rbac/v1"
)

const shippedChaosAdminManifest = "../../../manifests/cluster/2b_litmus_admin_rbac.yaml"

func shippedChaosAdminRules(t *testing.T) []k8srbacv1.PolicyRule {
	t.Helper()
	want, err := expectedClusterRoleRules(shippedChaosAdminManifest, chaosAdminClusterRoleName, "litmus")
	if err != nil {
		t.Fatal(err)
	}
	if len(want) == 0 {
		t.Fatal("no rules parsed from the shipped manifest")
	}
	return want
}

func TestPreflightRejectsStaleChaosAdminRole(t *testing.T) {
	// The narrow upstream role an infra connected under an older server keeps.
	stale := []k8srbacv1.PolicyRule{
		{APIGroups: []string{""}, Resources: []string{"pods", "events"}, Verbs: []string{"create", "get", "list", "patch", "update", "delete"}},
		{APIGroups: []string{"apps"}, Resources: []string{"deployments", "statefulsets"}, Verbs: []string{"get", "list", "patch", "update"}},
	}
	err := staleChaosAdminRBAC("test", "litmus", shippedChaosAdminRules(t), stale)
	if err == nil {
		t.Fatal("a run was allowed under a litmus-admin role that cannot read resourcequotas")
	}
	for _, want := range []string{"out-of-date RBAC", "setup.sh --restart", "and "} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error lacks %q: %v", want, err)
		}
	}
}

func TestPreflightAcceptsCurrentChaosAdminRole(t *testing.T) {
	want := shippedChaosAdminRules(t)
	if err := staleChaosAdminRBAC("test", "litmus", want, want); err != nil {
		t.Fatalf("the shipped role itself was rejected: %v", err)
	}
	wildcard := []k8srbacv1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}}}
	if err := staleChaosAdminRBAC("test", "litmus", want, wildcard); err != nil {
		t.Fatalf("a cluster-admin-equivalent role was rejected: %v", err)
	}
}

func TestShippedChaosAdminRoleGrantsResourceQuotas(t *testing.T) {
	missing := rulesSatisfyRequirements(shippedChaosAdminRules(t), []rbacRequirement{
		{APIGroup: "", Resource: "resourcequotas", Verb: "get"},
		{APIGroup: "", Resource: "namespaces", Verb: "delete"},
	})
	if len(missing) != 0 {
		t.Fatalf("shipped litmus-admin role lacks %v", missing)
	}
}
