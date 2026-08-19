package chaos_infrastructure

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/apphub"
	store "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/data-store"
	dbChaosInfra "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_infrastructure"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type SubscriberConfigurations struct {
	ServerEndpoint string
	TLSCert        string
}

func GetEndpoint(host string) (string, error) {
	// Priority 1: Use CHAOS_CENTER_UI_ENDPOINT if provided (via .env or manually set)
	if utils.Config.ChaosCenterUiEndpoint != "" {
		return utils.Config.ChaosCenterUiEndpoint + "/query", nil
	}

	// Priority 2: Fall back to Kubernetes service DNS (cluster mode)
	return host + "/query", nil
}

// GetManifestDownloadURL builds the human/kubectl-facing `kubectl apply -f
// <url>` link for a registered infra's manifest. Unlike GetEndpoint (which
// resolves the in-cluster callback address subscriber pods use), this must
// resolve to an address reachable from wherever the operator's shell
// actually has cluster access — which is frequently NOT the same as the
// browser's own address bar (SSH tunnels, VS Code port-forwarding, and
// similar setups routinely present the ChaosCenter UI to the browser on a
// different local port than the one actually reachable from a shell on the
// deployment host). host is the best-effort Referer/Host-derived fallback
// computed by the caller from the current request.
//
// base (whether from CHAOS_CENTER_PUBLIC_ENDPOINT or the host fallback) is
// the web UI's own front door — nginx there proxies /api/ to this graphql
// server's raw router (which itself registers the manifest route at
// /file/:key, no /api prefix — see server.go) and has no route for a bare
// /file/ path. So the /api segment must be added here; every supported
// deployment (Helm/KinD, flat k8s, docker compose) uses the identical
// nginx /api/ -> graphql proxy convention.
func GetManifestDownloadURL(host, token string) string {
	base := utils.Config.ChaosCenterPublicEndpoint
	if base == "" {
		base = host
	}
	return strings.TrimSuffix(base, "/") + "/api/file/" + token + ".yaml"
}

func GetK8sInfraYaml(host string, infra dbChaosInfra.ChaosInfra) ([]byte, error) {

	var config SubscriberConfigurations
	endpoint, err := GetEndpoint(host)
	if err != nil {
		return nil, err
	}
	config.ServerEndpoint = endpoint

	config.TLSCert = utils.Config.TlsCertB64

	var respData []byte
	if infra.InfraScope == ClusterScope {
		respData, err = ManifestParser(infra, "manifests/cluster", &config)
	} else if infra.InfraScope == NamespaceScope {
		respData, err = ManifestParser(infra, "manifests/namespace", &config)
	} else {
		log.Error("INFRA_SCOPE env is empty!")
	}
	if err != nil {
		return nil, err
	}

	return respData, nil
}

// ManifestParser parses manifests yaml and generates dynamic manifest with specified keys
func ManifestParser(infra dbChaosInfra.ChaosInfra, rootPath string, config *SubscriberConfigurations) ([]byte, error) {
	var (
		generatedYAML             []string
		defaultState              = false
		InfraNamespace            string
		ServiceAccountName        string
		DefaultInfraNamespace     = "litmus"
		DefaultServiceAccountName = "litmus"
	)

	if infra.InfraNsExists == nil {
		infra.InfraNsExists = &defaultState
	}
	if infra.InfraSaExists == nil {
		infra.InfraSaExists = &defaultState
	}

	if infra.InfraNamespace != nil && *infra.InfraNamespace != "" {
		InfraNamespace = *infra.InfraNamespace
	} else {
		InfraNamespace = DefaultInfraNamespace
	}

	if infra.ServiceAccount != nil && *infra.ServiceAccount != "" {
		ServiceAccountName = *infra.ServiceAccount
	} else {
		ServiceAccountName = DefaultServiceAccountName
	}

	skipSSL := "false"
	if infra.SkipSSL != nil && *infra.SkipSSL {
		skipSSL = "true"
	}

	var (
		namespaceConfig   = "---\napiVersion: v1\nkind: Namespace\nmetadata:\n  name: " + InfraNamespace + "\n"
		serviceAccountStr = "---\napiVersion: v1\nkind: ServiceAccount\nmetadata:\n  name: " + ServiceAccountName + "\n  namespace: " + InfraNamespace + "\n"
	)

	// Namespace discovery for the cluster-scope infra's RBAC (infra-cluster-role
	// in manifests/cluster/3a_agents_rbac.yaml) is intentionally get-by-name
	// only, never list/watch — Kubernetes RBAC can't scope list/watch to a
	// resourceNames subset, so granting list would let the infra's service
	// account enumerate every namespace on the cluster. The allowed names are
	// this infra's own namespace plus every application currently known to
	// ACE's app-charts catalog — read live from apphub, never hardcoded, so
	// adding/removing an app chart changes this on the next manifest fetch
	// with no source edit here.
	targetNamespaces := []string{InfraNamespace}
	if appNamespaces, err := apphub.GetKnownApplicationNamespaces(); err != nil {
		log.WithError(err).Warn("failed to read known application namespaces from app-charts catalog; " +
			"generated infra RBAC will only be able to see its own namespace until this is retried")
	} else {
		for _, ns := range appNamespaces {
			if ns != InfraNamespace {
				targetNamespaces = append(targetNamespaces, ns)
			}
		}
	}
	targetNamespacesEnv := strings.Join(targetNamespaces, ",")
	targetNamespacesResourceNamesJSON, err := json.Marshal(targetNamespaces)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal target namespace resourceNames: %w", err)
	}

	// Checking if the agent namespace does not exist and its scope of installation is not namespaced
	if !*infra.InfraNsExists && infra.InfraScope != "namespace" {
		generatedYAML = append(generatedYAML, fmt.Sprintf("%v", namespaceConfig))
	}

	if !*infra.InfraSaExists {
		generatedYAML = append(generatedYAML, fmt.Sprintf("%v", serviceAccountStr))
	}

	// File operations
	file, err := os.Open(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open the file %v", err)
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Errorf("failed to close the file %v", err)
		}
	}(file)

	list, err := file.Readdirnames(0) // 0 to read all files and folders
	if err != nil {
		return nil, fmt.Errorf("failed to read the file %v", err)
	}

	var nodeSelector string
	if infra.NodeSelector != nil {
		selector := strings.Split(*infra.NodeSelector, ",")
		selectorList := make(map[string]string)
		for _, el := range selector {
			kv := strings.Split(el, "=")
			selectorList[kv[0]] = kv[1]
		}

		byt, err := yaml.Marshal(
			struct {
				NodeSelector map[string]string `yaml:"nodeSelector" json:"nodeSelector"`
			}{
				NodeSelector: selectorList,
			})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal the node selector %v", err)
		}

		nodeSelector = string(utils.AddRootIndent(byt, 6))
	}

	var tolerations string
	if infra.Tolerations != nil {
		byt, err := yaml.Marshal(struct {
			Tolerations []*dbChaosInfra.Toleration `yaml:"tolerations" json:"tolerations"`
		}{
			Tolerations: infra.Tolerations,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal the tolerations %v", err)
		}

		tolerations = string(utils.AddRootIndent(byt, 6))
	}

	for _, fileName := range list {
		fileContent, err := os.ReadFile(rootPath + "/" + fileName)
		if err != nil {
			return nil, fmt.Errorf("failed to read the file %v", err)
		}

		var newContent = string(fileContent)

		newContent = strings.Replace(newContent, "#{TOLERATIONS}", tolerations, -1)
		newContent = strings.Replace(newContent, "#{INFRA_ID}", infra.InfraID, -1)
		newContent = strings.Replace(newContent, "#{ACCESS_KEY}", infra.AccessKey, -1)
		newContent = strings.Replace(newContent, "#{SERVER_ADDR}", config.ServerEndpoint, -1)
		newContent = strings.Replace(newContent, "#{SUBSCRIBER_IMAGE}", utils.Config.SubscriberImage, -1)
		newContent = strings.Replace(newContent, "#{EVENT_TRACKER_IMAGE}", utils.Config.EventTrackerImage, -1)
		newContent = strings.Replace(newContent, "#{INFRA_NAMESPACE}", InfraNamespace, -1)
		newContent = strings.Replace(newContent, "#{INFRA_SERVICE_ACCOUNT}", ServiceAccountName, -1)
		newContent = strings.Replace(newContent, "#{INFRA_SCOPE}", infra.InfraScope, -1)
		newContent = strings.Replace(newContent, "#{TARGET_APP_NAMESPACES}", targetNamespacesEnv, -1)
		newContent = strings.Replace(newContent, "#{TARGET_NAMESPACE_RESOURCE_NAMES}", string(targetNamespacesResourceNamesJSON), -1)
		newContent = strings.Replace(newContent, "#{ARGO_WORKFLOW_CONTROLLER}", utils.Config.ArgoWorkflowControllerImage, -1)
		newContent = strings.Replace(newContent, "#{LITMUS_CHAOS_OPERATOR}", utils.Config.LitmusChaosOperatorImage, -1)
		newContent = strings.Replace(newContent, "#{ARGO_WORKFLOW_EXECUTOR}", utils.Config.ArgoWorkflowExecutorImage, -1)
		newContent = strings.Replace(newContent, "#{LITMUS_CHAOS_RUNNER}", utils.Config.LitmusChaosRunnerImage, -1)
		newContent = strings.Replace(newContent, "#{LITMUS_CHAOS_EXPORTER}", utils.Config.LitmusChaosExporterImage, -1)
		newContent = strings.Replace(newContent, "#{ARGO_CONTAINER_RUNTIME_EXECUTOR}", utils.Config.ContainerRuntimeExecutor, -1)
		newContent = strings.Replace(newContent, "#{INFRA_DEPLOYMENTS}", utils.Config.InfraDeployments, -1)
		newContent = strings.Replace(newContent, "#{VERSION}", utils.Config.Version, -1)
		newContent = strings.Replace(newContent, "#{SKIP_SSL_VERIFY}", skipSSL, -1)
		newContent = strings.Replace(newContent, "#{CUSTOM_TLS_CERT}", config.TLSCert, -1)
		newContent = strings.Replace(newContent, "#{KUBERNETES_MCP_SERVER_IMAGE}", utils.Config.KubernetesMcpServerImage, -1)
		newContent = strings.Replace(newContent, "#{PROMETHEUS_MCP_SERVER_IMAGE}", utils.Config.PrometheusMcpServerImage, -1)
		newContent = strings.Replace(newContent, "#{PROMETHEUS_MCP_URL}", utils.Config.PrometheusMcpUrl, -1)

		newContent = strings.Replace(newContent, "#{START_TIME}", "\""+infra.StartTime+"\"", -1)
		if infra.IsInfraConfirmed {
			newContent = strings.Replace(newContent, "#{IS_INFRA_CONFIRMED}", "\""+"true"+"\"", -1)
		} else {
			newContent = strings.Replace(newContent, "#{IS_INFRA_CONFIRMED}", "\""+"false"+"\"", -1)
		}

		if infra.NodeSelector != nil {
			newContent = strings.Replace(newContent, "#{NODE_SELECTOR}", nodeSelector, -1)
		}
		generatedYAML = append(generatedYAML, newContent)
	}

	return []byte(strings.Join(generatedYAML, "\n")), nil
}

// SendRequestToSubscriber sends events from the graphQL server to the subscribers listening for the requests
func SendRequestToSubscriber(subscriberRequest SubscriberRequests, r store.StateData) {
	newAction := &model.InfraActionResponse{
		ProjectID: subscriberRequest.ProjectID,
		Action: &model.ActionPayload{
			K8sManifest:  subscriberRequest.K8sManifest,
			Namespace:    subscriberRequest.Namespace,
			RequestType:  subscriberRequest.RequestType,
			ExternalData: subscriberRequest.ExternalData,
			Username:     subscriberRequest.Username,
		},
	}

	r.Mutex.Lock()
	if observer, ok := r.ConnectedInfra[subscriberRequest.InfraID]; ok {
		observer <- newAction
	}
	r.Mutex.Unlock()
}

// SendExperimentToSubscriber sends the workflow to the subscriber to be handled
func SendExperimentToSubscriber(projectID string, workflow *model.ChaosExperimentRequest, username *string, externalData *string, reqType string, r *store.StateData) {

	var workflowObj unstructured.Unstructured
	err := yaml.Unmarshal([]byte(workflow.ExperimentManifest), &workflowObj)
	if err != nil {
		log.Errorf("error while parsing experiment manifest %v", err)
		return
	}

	SendRequestToSubscriber(SubscriberRequests{
		K8sManifest:  workflow.ExperimentManifest,
		RequestType:  reqType,
		ProjectID:    projectID,
		InfraID:      workflow.InfraID,
		Namespace:    workflowObj.GetNamespace(),
		ExternalData: externalData,
		Username:     username,
	}, *r)
}
