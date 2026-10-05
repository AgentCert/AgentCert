package handler

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
	k8srbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	ops "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaos_experiment/ops"
	dbChaosInfra "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_infrastructure"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/faultcatalog"
)

const (
	// chaosServiceAccountName is the chaosServiceAccount every fault's
	// ChaosEngine runs under (see utils.UsesUnscopedChaosServiceAccount).
	chaosServiceAccountName = "litmus-admin"
	// chaosAdminClusterRoleName and chaosAdminRBACManifest are the ClusterRole
	// the cluster-scoped infra manifest grants it, and the file it comes from.
	chaosAdminClusterRoleName = "litmus-admin-cluster-role"
	chaosAdminRBACManifest    = "manifests/cluster/2b_litmus_admin_rbac.yaml"

	// allowNodeDestructiveFaultsEnv opts out of the single-node guard.
	allowNodeDestructiveFaultsEnv = "ACE_ALLOW_NODE_DESTRUCTIVE_FAULTS"
)

var (
	preflightCatalogOnce sync.Once
	preflightCatalog     faultcatalog.Service
)

func preflightFaultCatalog() faultcatalog.Service {
	preflightCatalogOnce.Do(func() { preflightCatalog = faultcatalog.NewService() })
	return preflightCatalog
}

// preflightChaosTargets rejects a run that cannot inject its faults, before any
// of it is dispatched. Both conditions below otherwise surface only as a run
// that "succeeds" in Argo — litmus-checker exits 0 on a verdict of Error —
// while no fault was injected, and a certificate is then built for it.
func (c *ChaosExperimentRunHandler) preflightChaosTargets(ctx context.Context, projectID string, infra *dbChaosInfra.ChaosInfra, templates []v1alpha1.Template) error {
	faults, err := ops.ChaosFaultNames(templates)
	if err != nil || len(faults) == 0 {
		return nil
	}
	clientset, err := buildKubeClientset()
	if err != nil {
		logrus.WithError(err).Warn("[Preflight] no kubernetes client; skipping chaos service account and node checks")
		return nil
	}
	if err := preflightChaosServiceAccountRBAC(ctx, clientset, infra, chaosAdminRBACManifest); err != nil {
		return err
	}
	return preflightSingleNodeFaults(ctx, clientset, projectID, faults)
}

// preflightChaosServiceAccountRBAC fails when litmus-admin lacks permissions
// that the infra manifest this server ships grants it.
//
// That ClusterRole is written once, when the infrastructure is connected, and
// nothing reconciles it afterwards. An infra connected while an older server
// build was running keeps the older, narrower role after the server is
// upgraded, and every fault and teardown step then fails with "forbidden"
// (e.g. `cannot get resource "resourcequotas"`) while Argo reports success.
func preflightChaosServiceAccountRBAC(ctx context.Context, clientset kubernetes.Interface, infra *dbChaosInfra.ChaosInfra, manifestPath string) error {
	if infra == nil || infra.InfraScope != "cluster" {
		return nil
	}
	infraNamespace := infraNamespaceOrDefault(infra.InfraNamespace)

	want, err := expectedClusterRoleRules(manifestPath, chaosAdminClusterRoleName, infraNamespace)
	if err != nil {
		logrus.WithError(err).Warn("[Preflight] could not read the shipped litmus-admin ClusterRole; skipping its check")
		return nil
	}

	bindings, err := clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		logrus.WithError(err).Warn("[Preflight] could not list ClusterRoleBindings; skipping the litmus-admin check")
		return nil
	}
	var have []k8srbacv1.PolicyRule
	for _, binding := range bindings.Items {
		if binding.RoleRef.Kind != "ClusterRole" {
			continue
		}
		for _, subject := range binding.Subjects {
			if subject.Kind == "ServiceAccount" && subject.Name == chaosServiceAccountName && subject.Namespace == infraNamespace {
				if role, err := clientset.RbacV1().ClusterRoles().Get(ctx, binding.RoleRef.Name, metav1.GetOptions{}); err == nil {
					have = append(have, role.Rules...)
				}
				break
			}
		}
	}

	return staleChaosAdminRBAC(infra.Name, infraNamespace, want, have)
}

// staleChaosAdminRBAC reports the permissions in want that have does not grant.
func staleChaosAdminRBAC(infraName, infraNamespace string, want, have []k8srbacv1.PolicyRule) error {
	missing := rulesSatisfyRequirements(have, requirementsOf(want))
	if len(missing) == 0 {
		return nil
	}
	names := make([]string, 0, len(missing))
	for _, req := range missing {
		names = append(names, formatRequirement(req))
	}
	shown := names
	if len(shown) > 8 {
		shown = append(shown[:8:8], fmt.Sprintf("and %d more", len(names)-8))
	}
	return fmt.Errorf(
		"chaos infrastructure %q has out-of-date RBAC: service account %s/%s is missing %d permission(s) the current "+
			"infrastructure manifest grants (%s), so its faults and teardown steps would fail with \"forbidden\". "+
			"Re-apply the infrastructure manifest: re-run ./scripts/setup.sh --restart, or download the manifest for this "+
			"infrastructure from Environments and kubectl apply it",
		infraName, infraNamespace, chaosServiceAccountName, len(names), strings.Join(shown, ", "))
}

// preflightSingleNodeFaults refuses node-level faults on a single-node cluster,
// where draining, tainting, cordoning or killing the kubelet of "the target
// node" takes down every workload, the ACE platform included, and usually the
// experiment's own pods before they can revert the fault.
func preflightSingleNodeFaults(ctx context.Context, clientset kubernetes.Interface, projectID string, faults []string) error {
	if strings.EqualFold(strings.TrimSpace(os.Getenv(allowNodeDestructiveFaultsEnv)), "true") {
		return nil
	}
	catalog, err := preflightFaultCatalog().Catalog(ctx, projectID)
	if err != nil || catalog == nil {
		return nil
	}
	var dangerous []string
	for _, name := range faults {
		if fault, ok := catalog.Lookup(name); ok && fault.SingleNodeDanger {
			dangerous = append(dangerous, name)
		}
	}
	if len(dangerous) == 0 {
		return nil
	}
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logrus.WithError(err).Warn("[Preflight] could not list nodes; skipping the single-node fault check")
		return nil
	}
	if len(nodes.Items) > 1 {
		return nil
	}
	sort.Strings(dangerous)
	return fmt.Errorf(
		"fault(s) %s act on a whole node, and this cluster has only one node: injecting them would take down every "+
			"workload on it, including the ACE platform and the agent under test. Run them on a multi-node cluster, "+
			"or set %s=true on the graphql server to override",
		strings.Join(dangerous, ", "), allowNodeDestructiveFaultsEnv)
}

// expectedClusterRoleRules reads one ClusterRole's rules from an infra
// manifest template.
func expectedClusterRoleRules(path, roleName, infraNamespace string) ([]k8srbacv1.PolicyRule, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- server-controlled manifest shipped in the image
	if err != nil {
		return nil, err
	}
	content := strings.ReplaceAll(string(data), "#{INFRA_NAMESPACE}", infraNamespace)
	for _, doc := range strings.Split(content, "\n---") {
		var role k8srbacv1.ClusterRole
		if err := yaml.Unmarshal([]byte(doc), &role); err != nil {
			continue
		}
		if role.Kind == "ClusterRole" && role.Name == roleName {
			return role.Rules, nil
		}
	}
	return nil, fmt.Errorf("ClusterRole %s not found in %s", roleName, path)
}

// requirementsOf expands rules into one requirement per (group, resource, verb).
func requirementsOf(rules []k8srbacv1.PolicyRule) []rbacRequirement {
	var out []rbacRequirement
	for _, rule := range rules {
		if len(rule.ResourceNames) > 0 {
			continue
		}
		for _, group := range rule.APIGroups {
			for _, resource := range rule.Resources {
				for _, verb := range rule.Verbs {
					out = append(out, rbacRequirement{APIGroup: group, Resource: resource, Verb: verb})
				}
			}
		}
	}
	return out
}
