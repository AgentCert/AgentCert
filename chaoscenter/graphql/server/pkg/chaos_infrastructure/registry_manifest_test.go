package chaos_infrastructure

import (
	"path/filepath"
	"strings"
	"testing"

	dbChaosInfra "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_infrastructure"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"
	"sigs.k8s.io/yaml"
)

// Every infra pod names the registry pull secret when IMAGE_REGISTRY is set,
// and none does on the public path (the placeholder stays a YAML comment).
func TestManifestParserImagePullSecrets(t *testing.T) {
	repoRoot := repoRootForTest(t)
	t.Chdir(filepath.Join(repoRoot, "AgentCert/chaoscenter/graphql/server"))
	old := utils.Config.DefaultAppHubPath
	utils.Config.DefaultAppHubPath = filepath.Join(repoRoot, "app-charts")
	t.Cleanup(func() { utils.Config.DefaultAppHubPath = old })

	ns, sa, yes := "litmus", "litmus-admin", true
	for _, scope := range []struct{ dir, scope string }{
		{"manifests/cluster", ClusterScope},
		{"manifests/namespace", NamespaceScope},
	} {
		render := func() string {
			out, err := ManifestParser(dbChaosInfra.ChaosInfra{
				InfraNamespace: &ns, ServiceAccount: &sa, InfraScope: scope.scope,
				InfraNsExists: &yes, InfraSaExists: &yes,
			}, scope.dir, &SubscriberConfigurations{})
			if err != nil {
				t.Fatalf("%s: ManifestParser() error = %v", scope.dir, err)
			}
			return strings.ReplaceAll(string(out), "\r\n", "\n")
		}

		t.Setenv("IMAGE_REGISTRY", "")
		public := render()
		// (the CRDs mention imagePullSecrets in their schemas; look for a pod entry)
		if strings.Contains(public, "imagePullSecrets:\n      - name:") {
			t.Errorf("%s: public path must not add imagePullSecrets", scope.dir)
		}

		t.Setenv("IMAGE_REGISTRY", "infyartifactory.jfrog.io/docker-local")
		t.Setenv("IMAGE_PULL_SECRET_NAME", "registry-pull")
		private := render()
		deployments := strings.Count(private, "kind: Deployment")
		secrets := strings.Count(private, "imagePullSecrets:\n      - name: registry-pull")
		if deployments == 0 || secrets != deployments {
			t.Errorf("%s: %d Deployments but %d imagePullSecrets entries", scope.dir, deployments, secrets)
		}
		// Still valid YAML documents.
		for i, doc := range strings.Split(private, "\n---") {
			var v interface{}
			if err := yaml.Unmarshal([]byte(doc), &v); err != nil {
				t.Fatalf("%s: document %d is not valid YAML after substitution: %v", scope.dir, i, err)
			}
		}
	}
}
