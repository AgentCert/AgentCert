package chaos_infrastructure

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	dbChaosInfra "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_infrastructure"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"
	"gopkg.in/yaml.v3"
)

type rbacRule struct {
	APIGroups []string `yaml:"apiGroups"`
	Resources []string `yaml:"resources"`
	Verbs     []string `yaml:"verbs"`
}

type manifestDocument struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Rules []rbacRule `yaml:"rules"`
	Spec  struct {
		Definition struct {
			Permissions []rbacRule `yaml:"permissions"`
		} `yaml:"definition"`
	} `yaml:"spec"`
}

type rbacPermission struct {
	apiGroup string
	resource string
	verb     string
}

func repoRootForTest(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "../../../../../.."))
}

func permissionsFromRules(rules []rbacRule) map[rbacPermission]struct{} {
	permissions := make(map[rbacPermission]struct{})
	for _, rule := range rules {
		for _, apiGroup := range rule.APIGroups {
			for _, resource := range rule.Resources {
				for _, verb := range rule.Verbs {
					permissions[rbacPermission{apiGroup: apiGroup, resource: resource, verb: verb}] = struct{}{}
				}
			}
		}
	}
	return permissions
}

func isClusterScopedCatalogPermission(permission rbacPermission) bool {
	switch {
	case permission.apiGroup == "" && (permission.resource == "namespaces" || permission.resource == "nodes"):
		return true
	case permission.apiGroup == "rbac.authorization.k8s.io" && (permission.resource == "clusterroles" || permission.resource == "clusterrolebindings"):
		return true
	case permission.apiGroup == "scheduling.k8s.io" && permission.resource == "priorityclasses":
		return true
	default:
		return false
	}
}

func loadFaultCatalogPermissions(t *testing.T, repoRoot string, namespacedOnly bool) map[rbacPermission]struct{} {
	t.Helper()
	permissions := make(map[rbacPermission]struct{})
	faultsRoot := filepath.Join(repoRoot, "chaos-charts", "faults")
	err := filepath.WalkDir(faultsRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Base(path) != "fault.yaml" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		for {
			var document manifestDocument
			if err := decoder.Decode(&document); err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			if document.Kind != "ChaosExperiment" {
				continue
			}
			for permission := range permissionsFromRules(document.Spec.Definition.Permissions) {
				if namespacedOnly && isClusterScopedCatalogPermission(permission) {
					continue
				}
				permissions[permission] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to load fault catalog permissions: %v", err)
	}
	return permissions
}

func loadRolePermissions(t *testing.T, manifest []byte, roleName string) map[rbacPermission]struct{} {
	t.Helper()
	decoder := yaml.NewDecoder(bytes.NewReader(manifest))
	for {
		var document manifestDocument
		if err := decoder.Decode(&document); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("failed to parse generated manifest: %v", err)
		}
		if (document.Kind == "Role" || document.Kind == "ClusterRole") && document.Metadata.Name == roleName {
			return permissionsFromRules(document.Rules)
		}
	}
	t.Fatalf("generated manifest missing role %q", roleName)
	return nil
}

func assertPermissionCoverage(t *testing.T, roleName string, required, actual map[rbacPermission]struct{}) {
	t.Helper()
	for permission := range required {
		if _, ok := actual[permission]; !ok {
			t.Fatalf("%s missing permission: apiGroup=%q resource=%q verb=%q", roleName, permission.apiGroup, permission.resource, permission.verb)
		}
	}
}

func TestManifestParserAddsTargetNamespaceRBAC(t *testing.T) {
	repoRoot := repoRootForTest(t)
	serverRoot := filepath.Join(repoRoot, "AgentCert/chaoscenter/graphql/server")
	t.Chdir(serverRoot)

	oldAppHubPath := utils.Config.DefaultAppHubPath
	utils.Config.DefaultAppHubPath = filepath.Join(repoRoot, "app-charts")
	t.Cleanup(func() {
		utils.Config.DefaultAppHubPath = oldAppHubPath
	})

	infraNamespace := "itbench"
	serviceAccount := "litmus-admin"
	trueValue := true

	manifest, err := ManifestParser(dbChaosInfra.ChaosInfra{
		InfraNamespace: &infraNamespace,
		ServiceAccount: &serviceAccount,
		InfraScope:     NamespaceScope,
		InfraNsExists:  &trueValue,
		InfraSaExists:  &trueValue,
	}, "manifests/namespace", &SubscriberConfigurations{})
	if err != nil {
		t.Fatalf("ManifestParser() error = %v", err)
	}

	got := strings.ReplaceAll(string(manifest), "\r\n", "\n")
	if !strings.Contains(got, "kind: ClusterRole\nmetadata:\n  name: litmus-admin-target-namespace-role") {
		t.Fatalf("generated manifest missing target namespace ClusterRole")
	}
	if strings.Contains(got, "kind: ClusterRoleBinding\nmetadata:\n  name: litmus-admin") {
		t.Fatalf("generated namespace manifest must not bind litmus-admin cluster-wide")
	}

	for _, namespace := range []string{"book-info", "otel-demo", "sock-shop"} {
		if !strings.Contains(got, "kind: Namespace\nmetadata:\n  name: "+namespace) {
			t.Fatalf("generated manifest missing Namespace %q", namespace)
		}
		if !strings.Contains(got, "name: litmus-admin-target-namespace-role-binding\n  namespace: "+namespace) {
			t.Fatalf("generated manifest missing target RoleBinding in namespace %q", namespace)
		}
	}
}

func TestLitmusAdminRBACCoversFaultCatalogPermissions(t *testing.T) {
	repoRoot := repoRootForTest(t)
	serverRoot := filepath.Join(repoRoot, "AgentCert/chaoscenter/graphql/server")
	t.Chdir(serverRoot)

	oldAppHubPath := utils.Config.DefaultAppHubPath
	utils.Config.DefaultAppHubPath = filepath.Join(repoRoot, "app-charts")
	t.Cleanup(func() {
		utils.Config.DefaultAppHubPath = oldAppHubPath
	})

	infraNamespace := "itbench"
	serviceAccount := "litmus-admin"
	trueValue := true
	config := &SubscriberConfigurations{}

	clusterManifest, err := ManifestParser(dbChaosInfra.ChaosInfra{
		InfraNamespace: &infraNamespace,
		ServiceAccount: &serviceAccount,
		InfraScope:     ClusterScope,
		InfraNsExists:  &trueValue,
		InfraSaExists:  &trueValue,
	}, "manifests/cluster", config)
	if err != nil {
		t.Fatalf("ManifestParser(cluster) error = %v", err)
	}

	namespaceManifest, err := ManifestParser(dbChaosInfra.ChaosInfra{
		InfraNamespace: &infraNamespace,
		ServiceAccount: &serviceAccount,
		InfraScope:     NamespaceScope,
		InfraNsExists:  &trueValue,
		InfraSaExists:  &trueValue,
	}, "manifests/namespace", config)
	if err != nil {
		t.Fatalf("ManifestParser(namespace) error = %v", err)
	}

	allFaultPermissions := loadFaultCatalogPermissions(t, repoRoot, false)
	namespacedFaultPermissions := loadFaultCatalogPermissions(t, repoRoot, true)

	assertPermissionCoverage(t, "litmus-admin-cluster-role", allFaultPermissions,
		loadRolePermissions(t, clusterManifest, "litmus-admin-cluster-role"))
	assertPermissionCoverage(t, "litmus-admin-role", namespacedFaultPermissions,
		loadRolePermissions(t, namespaceManifest, "litmus-admin-role"))
	assertPermissionCoverage(t, "litmus-admin-target-namespace-role", namespacedFaultPermissions,
		loadRolePermissions(t, namespaceManifest, "litmus-admin-target-namespace-role"))
}
