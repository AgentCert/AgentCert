package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	probe "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/probe/handler"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/authorization"

	probeUtils "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/probe/utils"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaos_infrastructure"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/gitops"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/observability"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/ghodss/yaml"
	chaosTypes "github.com/litmuschaos/chaos-operator/api/litmuschaos/v1alpha1"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
	dbChaosExperimentRun "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment_run"

	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	k8srbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	types "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaos_experiment_run"
	store "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/data-store"
	dbChaosExperiment "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment"

	"github.com/google/uuid"
	dbChaosInfra "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_infrastructure"
)

// ChaosExperimentRunHandler is the handler for chaos experiment
type ChaosExperimentRunHandler struct {
	chaosExperimentRunService  types.Service
	infrastructureService      chaos_infrastructure.Service
	gitOpsService              gitops.Service
	chaosExperimentOperator    *dbChaosExperiment.Operator
	chaosExperimentRunOperator *dbChaosExperimentRun.Operator
	probeService               probe.Service
	mongodbOperator            mongodb.MongoOperator
}

type rbacRequirement struct {
	APIGroup string
	Resource string
	Verb     string
}

var dynamicAppHelmRBACRequirements = []rbacRequirement{
	{APIGroup: "", Resource: "namespaces", Verb: "create"},
	{APIGroup: "", Resource: "namespaces", Verb: "patch"},
	{APIGroup: "", Resource: "namespaces", Verb: "update"},
	{APIGroup: "", Resource: "secrets", Verb: "get"},
	{APIGroup: "", Resource: "secrets", Verb: "list"},
	{APIGroup: "", Resource: "secrets", Verb: "watch"},
	{APIGroup: "", Resource: "secrets", Verb: "create"},
	{APIGroup: "", Resource: "secrets", Verb: "update"},
	{APIGroup: "", Resource: "secrets", Verb: "patch"},
	{APIGroup: "", Resource: "secrets", Verb: "delete"},
}

// NewChaosExperimentRunHandler returns a new instance of ChaosWorkflowHandler
func NewChaosExperimentRunHandler(
	chaosExperimentRunService types.Service,
	infrastructureService chaos_infrastructure.Service,
	gitOpsService gitops.Service,
	chaosExperimentOperator *dbChaosExperiment.Operator,
	chaosExperimentRunOperator *dbChaosExperimentRun.Operator,
	probeService probe.Service,
	mongodbOperator mongodb.MongoOperator,
) *ChaosExperimentRunHandler {
	return &ChaosExperimentRunHandler{
		chaosExperimentRunService:  chaosExperimentRunService,
		infrastructureService:      infrastructureService,
		gitOpsService:              gitOpsService,
		chaosExperimentOperator:    chaosExperimentOperator,
		chaosExperimentRunOperator: chaosExperimentRunOperator,
		probeService:               probeService,
		mongodbOperator:            mongodbOperator,
	}
}

func buildKubeClientset() (*kubernetes.Clientset, error) {
	tryPaths := make([]string, 0, 3)

	if utils.Config.KubeConfigFilePath != "" {
		tryPaths = append(tryPaths, utils.Config.KubeConfigFilePath)
	}

	if envKubeConfig := strings.TrimSpace(os.Getenv("KUBECONFIG")); envKubeConfig != "" {
		for _, p := range strings.Split(envKubeConfig, string(os.PathListSeparator)) {
			p = strings.TrimSpace(p)
			if p != "" {
				tryPaths = append(tryPaths, p)
			}
		}
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		tryPaths = append(tryPaths, filepath.Join(home, ".kube", "config"))
	}

	seen := make(map[string]struct{})
	for _, kubePath := range tryPaths {
		if kubePath == "" {
			continue
		}
		if _, ok := seen[kubePath]; ok {
			continue
		}
		seen[kubePath] = struct{}{}

		cfg, err := clientcmd.BuildConfigFromFlags("", kubePath)
		if err != nil {
			continue
		}

		clientset, err := kubernetes.NewForConfig(cfg)
		if err == nil {
			logrus.WithField("kubeconfig", kubePath).Debug("using kubeconfig for appkind/runtime detection")
			return clientset, nil
		}
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(cfg)
}

func containsWithWildcard(values []string, wanted string) bool {
	for _, v := range values {
		if v == "*" || v == wanted {
			return true
		}
	}
	return false
}

func policyRuleAllows(rule k8srbacv1.PolicyRule, req rbacRequirement) bool {
	if !containsWithWildcard(rule.Verbs, req.Verb) {
		return false
	}
	if !containsWithWildcard(rule.APIGroups, req.APIGroup) {
		return false
	}
	if !containsWithWildcard(rule.Resources, req.Resource) {
		return false
	}
	return true
}

func roleSatisfiesRequirements(role *k8srbacv1.ClusterRole, requirements []rbacRequirement) []rbacRequirement {
	missing := make([]rbacRequirement, 0)
	for _, req := range requirements {
		satisfied := false
		for _, rule := range role.Rules {
			if policyRuleAllows(rule, req) {
				satisfied = true
				break
			}
		}
		if !satisfied {
			missing = append(missing, req)
		}
	}
	return missing
}

func rulesSatisfyRequirements(rules []k8srbacv1.PolicyRule, requirements []rbacRequirement) []rbacRequirement {
	missing := make([]rbacRequirement, 0)
	for _, req := range requirements {
		satisfied := false
		for _, rule := range rules {
			if policyRuleAllows(rule, req) {
				satisfied = true
				break
			}
		}
		if !satisfied {
			missing = append(missing, req)
		}
	}
	return missing
}

func formatRequirement(req rbacRequirement) string {
	resource := req.Resource
	if req.APIGroup != "" {
		resource = fmt.Sprintf("%s.%s", req.Resource, req.APIGroup)
	}
	return fmt.Sprintf("%s %s", req.Verb, resource)
}

func normalizeRBACNamePart(in string) string {
	in = strings.ToLower(strings.TrimSpace(in))
	if in == "" {
		return "default"
	}
	replacer := strings.NewReplacer(
		"/", "-",
		":", "-",
		"_", "-",
		".", "-",
		" ", "-",
	)
	out := replacer.Replace(in)
	out = strings.Trim(out, "-")
	if out == "" {
		return "default"
	}
	return out
}

func ensureDynamicAppHelmRBAC(ctx context.Context, clientset *kubernetes.Clientset, infraNamespace, serviceAccount string) error {
	const roleName = "litmus-dynamic-app-helm"

	bindingName := fmt.Sprintf(
		"%s-%s-%s",
		roleName,
		normalizeRBACNamePart(infraNamespace),
		normalizeRBACNamePart(serviceAccount),
	)

	requiredRole := &k8srbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: roleName},
		Rules: []k8srbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"create", "patch", "update"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		},
	}

	if existingRole, err := clientset.RbacV1().ClusterRoles().Get(ctx, roleName, metav1.GetOptions{}); err != nil {
		if _, createErr := clientset.RbacV1().ClusterRoles().Create(ctx, requiredRole, metav1.CreateOptions{}); createErr != nil {
			return fmt.Errorf("failed creating clusterrole %s: %w", roleName, createErr)
		}
	} else {
		requiredRole.ResourceVersion = existingRole.ResourceVersion
		if _, updateErr := clientset.RbacV1().ClusterRoles().Update(ctx, requiredRole, metav1.UpdateOptions{}); updateErr != nil {
			return fmt.Errorf("failed updating clusterrole %s: %w", roleName, updateErr)
		}
	}

	requiredBinding := &k8srbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: bindingName},
		Subjects: []k8srbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      serviceAccount,
			Namespace: infraNamespace,
		}},
		RoleRef: k8srbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
		},
	}

	if existingBinding, err := clientset.RbacV1().ClusterRoleBindings().Get(ctx, bindingName, metav1.GetOptions{}); err != nil {
		if _, createErr := clientset.RbacV1().ClusterRoleBindings().Create(ctx, requiredBinding, metav1.CreateOptions{}); createErr != nil {
			return fmt.Errorf("failed creating clusterrolebinding %s: %w", bindingName, createErr)
		}
	} else {
		requiredBinding.ResourceVersion = existingBinding.ResourceVersion
		if _, updateErr := clientset.RbacV1().ClusterRoleBindings().Update(ctx, requiredBinding, metav1.UpdateOptions{}); updateErr != nil {
			return fmt.Errorf("failed updating clusterrolebinding %s: %w", bindingName, updateErr)
		}
	}

	logrus.WithFields(logrus.Fields{
		"clusterRole":         roleName,
		"clusterRoleBinding":  bindingName,
		"serviceAccount":      serviceAccount,
		"serviceAccountNs":    infraNamespace,
	}).Info("ensured dynamic app Helm RBAC binding")

	return nil
}

func normalizeInstallTemplateArgs(args []string) ([]string, bool) {
	const timeoutArg = "-timeout={{workflow.parameters.installTimeout}}"

	normalized := make([]string, 0, len(args)+1)
	hasTimeout := false
	changed := false

	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}

		lower := strings.ToLower(arg)
		switch {
		case lower == "-wait" || lower == "--wait":
			// Strip -wait flags; the deployer binary does not support them
			changed = true
			continue
		case strings.HasPrefix(lower, "-wait=") || strings.HasPrefix(lower, "--wait="):
			// Strip -wait=... flags; the deployer binary does not support them
			changed = true
			continue
		case lower == "-timeout" || lower == "--timeout":
			changed = true
			if !hasTimeout {
				normalized = append(normalized, timeoutArg)
				hasTimeout = true
			}
			if i+1 < len(args) {
				i++
			}
			continue
		case strings.HasPrefix(lower, "-timeout=") || strings.HasPrefix(lower, "--timeout="):
			if hasTimeout {
				changed = true
				continue
			}
			hasTimeout = true
			normalized = append(normalized, timeoutArg)
			if arg != timeoutArg {
				changed = true
			}
			continue
		}

		normalized = append(normalized, arg)
	}

	if !hasTimeout {
		normalized = append(normalized, timeoutArg)
		changed = true
	}

	return normalized, changed
}

func normalizeInstallTemplates(templates []v1alpha1.Template) bool {
	updated := false

	for i := range templates {
		if templates[i].Container == nil {
			continue
		}

		if templates[i].Name != "install-application" && templates[i].Name != "install-agent" {
			continue
		}

		normalized, changed := normalizeInstallTemplateArgs(templates[i].Container.Args)
		if changed {
			logrus.WithField("template", templates[i].Name).Info("normalized install template arguments")
			templates[i].Container.Args = normalized
			updated = true
		}
	}

	return updated
}

// ensureInstallTimeoutParam appends the "installTimeout" global workflow
// parameter with a sensible default when it is not already declared.
// Without this, Argo validation rejects the workflow immediately because
// normalizeInstallTemplates rewrites -timeout= args to reference
// {{workflow.parameters.installTimeout}}.
func ensureInstallTimeoutParam(params *v1alpha1.Arguments) {
	const paramName = "installTimeout"
	const defaultValue = "900s"

	for _, p := range params.Parameters {
		if p.Name == paramName {
			return // already declared
		}
	}

	params.Parameters = append(params.Parameters, v1alpha1.Parameter{
		Name:  paramName,
		Value: v1alpha1.AnyStringPtr(defaultValue),
	})
	logrus.WithField("default", defaultValue).Info("added missing installTimeout workflow parameter")
}

func normalizeLabelSelector(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	// Handle selectors persisted in forms like [name=user-db] or name: user-db.
	trimmed := strings.Trim(raw, "[]{}\"")
	trimmed = strings.TrimSpace(trimmed)

	selectors := make([]string, 0, 3)
	if trimmed != "" {
		selectors = append(selectors, trimmed)
	}

	if strings.Contains(trimmed, ":") && !strings.Contains(trimmed, "=") {
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) == 2 {
			selectors = append(selectors, strings.TrimSpace(parts[0])+"="+strings.TrimSpace(parts[1]))
		}
	}

	if strings.Contains(trimmed, " ") && !strings.Contains(trimmed, "=") {
		parts := strings.Fields(trimmed)
		if len(parts) == 2 {
			selectors = append(selectors, parts[0]+"="+parts[1])
		}
	}

	seen := make(map[string]struct{})
	uniq := make([]string, 0, len(selectors))
	for _, s := range selectors {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		uniq = append(uniq, s)
	}

	return uniq
}

func buildWorkflowParameterMap(args interface{}) map[string]string {
	paramMap := make(map[string]string)
	if args == nil {
		return paramMap
	}

	buf, err := json.Marshal(args)
	if err != nil {
		return paramMap
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf, &parsed); err != nil {
		return paramMap
	}

	rawParams, ok := parsed["parameters"].([]interface{})
	if !ok {
		return paramMap
	}

	for _, p := range rawParams {
		pm, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		name := strings.TrimSpace(fmt.Sprint(pm["name"]))
		if name == "" || name == "<nil>" {
			continue
		}

		value := strings.TrimSpace(fmt.Sprint(pm["value"]))
		if value == "<nil>" {
			value = ""
		}
		paramMap[name] = value
	}

	return paramMap
}

func resolveWorkflowParameterValue(raw string, params map[string]string) string {
	resolved := strings.TrimSpace(raw)
	if resolved == "" || len(params) == 0 {
		return resolved
	}

	for key, value := range params {
		tokenBraced := "{{workflow.parameters." + key + "}}"
		tokenPlain := "workflow.parameters." + key
		resolved = strings.ReplaceAll(resolved, tokenBraced, value)
		resolved = strings.ReplaceAll(resolved, tokenPlain, value)
	}

	resolved = strings.TrimSpace(resolved)
	resolved = strings.Trim(resolved, "\"'")
	return resolved
}

// detectAppKindFromCluster queries the Kubernetes cluster to determine actual resource type.
// Returns the detected kind name (lowercase), or the provided fallback if detection fails.
func detectAppKindFromCluster(clientset *kubernetes.Clientset, namespace string, appLabel string, fallback string) string {
	if clientset == nil || namespace == "" || appLabel == "" {
		return fallback
	}

	selectors := normalizeLabelSelector(appLabel)
	if len(selectors) == 0 {
		return fallback
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, selector := range selectors {
		deployments, err := clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err == nil && len(deployments.Items) > 0 {
			logrus.WithFields(logrus.Fields{"namespace": namespace, "label": appLabel, "selector": selector}).Debug("Detected Deployment")
			return "deployment"
		}

		statefulsets, err := clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err == nil && len(statefulsets.Items) > 0 {
			logrus.WithFields(logrus.Fields{"namespace": namespace, "label": appLabel, "selector": selector}).Debug("Detected StatefulSet")
			return "statefulset"
		}

		daemonsets, err := clientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err == nil && len(daemonsets.Items) > 0 {
			logrus.WithFields(logrus.Fields{"namespace": namespace, "label": appLabel, "selector": selector}).Debug("Detected DaemonSet")
			return "daemonset"
		}

		replicasets, err := clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err == nil && len(replicasets.Items) > 0 {
			// ChaosEngine appkind expects top-level workload kinds; most ReplicaSets are Deployment-managed.
			logrus.WithFields(logrus.Fields{"namespace": namespace, "label": appLabel, "selector": selector}).Debug("Detected ReplicaSet, normalizing to Deployment")
			return "deployment"
		}

		// Fall back to pod owner references if workload listing is unavailable or inconclusive.
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err == nil && len(pods.Items) > 0 {
			for _, p := range pods.Items {
				for _, owner := range p.OwnerReferences {
					switch strings.ToLower(owner.Kind) {
					case "replicaset":
						logrus.WithFields(logrus.Fields{"namespace": namespace, "label": appLabel, "selector": selector, "owner": owner.Kind}).Debug("Detected Deployment via Pod owner")
						return "deployment"
					case "statefulset":
						logrus.WithFields(logrus.Fields{"namespace": namespace, "label": appLabel, "selector": selector, "owner": owner.Kind}).Debug("Detected StatefulSet via Pod owner")
						return "statefulset"
					case "daemonset":
						logrus.WithFields(logrus.Fields{"namespace": namespace, "label": appLabel, "selector": selector, "owner": owner.Kind}).Debug("Detected DaemonSet via Pod owner")
						return "daemonset"
					case "deployment":
						return "deployment"
					}
				}
			}
		}
	}

	// Fall back conservatively if nothing is detected.
	// In practice most app targets are Deployments; preserving stale StatefulSet often causes TARGET_SELECTION_ERROR.
	safeFallback := strings.ToLower(strings.TrimSpace(fallback))
	switch safeFallback {
	case "deployment", "statefulset", "daemonset":
	default:
		safeFallback = "deployment"
	}

	if safeFallback == "statefulset" {
		logrus.WithFields(logrus.Fields{"namespace": namespace, "label": appLabel, "selectors": selectors, "fallback": fallback}).Warn("Could not detect app kind, overriding statefulset fallback to deployment")
		return "deployment"
	}

	logrus.WithFields(logrus.Fields{"namespace": namespace, "label": appLabel, "selectors": selectors, "fallback": safeFallback}).Warn("Could not detect app kind, using fallback")
	return safeFallback
}

// normalizeChaosEngineAppKind normalizes the appkind in ChaosEngine by detecting actual resource type from cluster
func normalizeChaosEngineAppKind(clientset *kubernetes.Clientset, engine *chaosTypes.ChaosEngine) bool {
	if engine == nil {
		logrus.Debug("[AppKind] Engine is nil, skipping normalization")
		return false
	}

	currentKind := strings.ToLower(engine.Spec.Appinfo.AppKind)
	appLabel := engine.Spec.Appinfo.Applabel
	appNamespace := engine.Spec.Appinfo.Appns

	// If no label or namespace, can't detect
	if appLabel == "" || appNamespace == "" {
		logrus.WithFields(logrus.Fields{
			"appLabel": appLabel,
			"appNs":    appNamespace,
		}).Debug("[AppKind] Missing label or namespace, skipping normalization")
		return false
	}

	// Detect actual kind from cluster
	detectedKind := detectAppKindFromCluster(clientset, appNamespace, appLabel, currentKind)

	logrus.WithFields(logrus.Fields{
		"namespace":    appNamespace,
		"label":        appLabel,
		"currentKind":  currentKind,
		"detectedKind": detectedKind,
	}).Info("[AppKind] Detection result")

	// If detected kind differs from stored kind, update it
	if detectedKind != currentKind {
		logrus.WithFields(logrus.Fields{
			"namespace": appNamespace,
			"label":     appLabel,
			"old_kind":  currentKind,
			"new_kind":  detectedKind,
		}).Info("Normalizing ChaosEngine appkind")
		engine.Spec.Appinfo.AppKind = detectedKind
		return true
	}

	return false
}

// detectNodeContainerRuntime returns the container runtime name and socket path
// by querying the Kubernetes node status. Supports docker, containerd, and cri-o.
func detectNodeContainerRuntime(clientset *kubernetes.Clientset) (runtime, socketPath string) {
	if clientset == nil {
		return "", ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil || len(nodes.Items) == 0 {
		return "", ""
	}

	// ContainerRuntimeVersion format: "docker://28.2.2", "containerd://1.7.x", "cri-o://..."
	rv := nodes.Items[0].Status.NodeInfo.ContainerRuntimeVersion
	switch {
	case strings.HasPrefix(rv, "docker://"):
		return "docker", "/run/docker.sock"
	case strings.HasPrefix(rv, "containerd://"):
		return "containerd", "/run/containerd/containerd.sock"
	case strings.HasPrefix(rv, "cri-o://"):
		return "cri-o", "/var/run/crio/crio.sock"
	default:
		return "", ""
	}
}

// normalizeContainerRuntimeInYAML injects or updates CONTAINER_RUNTIME and SOCKET_PATH
// in all spec.experiments[].spec.components.env entries of a ChaosEngine YAML artifact.
// Works via JSON map manipulation so it is independent of the exact Go type definition.
func normalizeContainerRuntimeInYAML(data, runtime, socketPath string) string {
	if runtime == "" || data == "" {
		return data
	}

	jsonData, err := yaml.YAMLToJSON([]byte(data))
	if err != nil {
		return data
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(jsonData, &obj); err != nil {
		return data
	}

	kind, _ := obj["kind"].(string)
	if strings.ToLower(kind) != "chaosengine" {
		return data
	}

	spec, _ := obj["spec"].(map[string]interface{})
	if spec == nil {
		return data
	}

	experiments, _ := spec["experiments"].([]interface{})
	for _, expRaw := range experiments {
		expMap, _ := expRaw.(map[string]interface{})
		if expMap == nil {
			continue
		}
		expSpec, _ := expMap["spec"].(map[string]interface{})
		if expSpec == nil {
			expSpec = map[string]interface{}{}
			expMap["spec"] = expSpec
		}
		components, _ := expSpec["components"].(map[string]interface{})
		if components == nil {
			components = map[string]interface{}{}
			expSpec["components"] = components
		}

		var envSlice []interface{}
		if existing, ok := components["env"].([]interface{}); ok {
			envSlice = existing
		}

		envUpdates := map[string]string{
			"CONTAINER_RUNTIME": runtime,
			"SOCKET_PATH":       socketPath,
		}

		for envName, envValue := range envUpdates {
			found := false
			for _, entryRaw := range envSlice {
				entry, _ := entryRaw.(map[string]interface{})
				if entry == nil {
					continue
				}
				if entry["name"] == envName {
					if entry["value"] != envValue {
						logrus.WithFields(logrus.Fields{
							"env":   envName,
							"old":   entry["value"],
							"new":   envValue,
						}).Info("Normalizing ChaosEngine container runtime env")
						entry["value"] = envValue
					}
					found = true
					break
				}
			}
			if !found {
				logrus.WithFields(logrus.Fields{
					"env":   envName,
					"value": envValue,
				}).Info("Injecting missing container runtime env into ChaosEngine")
				envSlice = append(envSlice, map[string]interface{}{
					"name":  envName,
					"value": envValue,
				})
			}
		}
		components["env"] = envSlice
	}

	resultJSON, err := json.Marshal(obj)
	if err != nil {
		return data
	}
	resultYAML, err := yaml.JSONToYAML(resultJSON)
	if err != nil {
		return data
	}
	return string(resultYAML)
}

func (c *ChaosExperimentRunHandler) preflightInfraRBAC(ctx context.Context, infra *dbChaosInfra.ChaosInfra) error {
	if infra == nil {
		return errors.New("infra not found for RBAC preflight")
	}

	infraNamespace := "litmus"
	if infra.InfraNamespace != nil && *infra.InfraNamespace != "" {
		infraNamespace = *infra.InfraNamespace
	}

	serviceAccount := "argo-chaos"
	if infra.ServiceAccount != nil && *infra.ServiceAccount != "" {
		serviceAccount = *infra.ServiceAccount
	}

	clientset, err := buildKubeClientset()
	if err != nil {
		return fmt.Errorf("failed RBAC preflight: unable to create kubernetes client: %w", err)
	}

	if err := ensureDynamicAppHelmRBAC(ctx, clientset, infraNamespace, serviceAccount); err != nil {
		logrus.WithFields(logrus.Fields{
			"infraNamespace":  infraNamespace,
			"serviceAccount": serviceAccount,
		}).WithError(err).Warn("RBAC preflight auto-remediation skipped; continuing with RBAC validation")
	}

	bindings, err := clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed RBAC preflight: unable to list ClusterRoleBindings: %w", err)
	}

	boundClusterRoles := make(map[string]struct{})
	boundRoles := make(map[string]struct{})
	for _, binding := range bindings.Items {
		for _, subject := range binding.Subjects {
			if subject.Kind == "ServiceAccount" && subject.Name == serviceAccount && subject.Namespace == infraNamespace {
				if binding.RoleRef.Kind == "ClusterRole" {
					boundClusterRoles[binding.RoleRef.Name] = struct{}{}
				} else if binding.RoleRef.Kind == "Role" {
					boundRoles[binding.RoleRef.Name] = struct{}{}
				}
			}
		}
	}

	if roleBindings, roleBindErr := clientset.RbacV1().RoleBindings(infraNamespace).List(ctx, metav1.ListOptions{}); roleBindErr == nil {
		for _, binding := range roleBindings.Items {
			for _, subject := range binding.Subjects {
				if subject.Kind == "ServiceAccount" && subject.Name == serviceAccount && subject.Namespace == infraNamespace {
					if binding.RoleRef.Kind == "ClusterRole" {
						boundClusterRoles[binding.RoleRef.Name] = struct{}{}
					} else if binding.RoleRef.Kind == "Role" {
						boundRoles[binding.RoleRef.Name] = struct{}{}
					}
				}
			}
		}
	} else {
		logrus.WithField("infraNamespace", infraNamespace).WithError(roleBindErr).Warn("RBAC preflight: unable to list namespace RoleBindings")
	}

	if len(boundClusterRoles) == 0 && len(boundRoles) == 0 {
		return fmt.Errorf(
			"RBAC preflight failed for serviceaccount %s/%s: no RoleBinding/ClusterRoleBinding found. "+
				"Bind this service account to a role with namespace patch/create/update and secrets list/get/watch/create/update/patch/delete permissions",
			infraNamespace,
			serviceAccount,
		)
	}

	remaining := append([]rbacRequirement{}, dynamicAppHelmRBACRequirements...)
	for roleName := range boundRoles {
		role, roleErr := clientset.RbacV1().Roles(infraNamespace).Get(ctx, roleName, metav1.GetOptions{})
		if roleErr != nil {
			continue
		}

		remaining = rulesSatisfyRequirements(role.Rules, remaining)
		if len(remaining) == 0 {
			break
		}
	}

	for clusterRoleName := range boundClusterRoles {
		role, roleErr := clientset.RbacV1().ClusterRoles().Get(ctx, clusterRoleName, metav1.GetOptions{})
		if roleErr != nil {
			continue
		}

		remaining = roleSatisfiesRequirements(role, remaining)
		if len(remaining) == 0 {
			break
		}
	}

	if len(remaining) > 0 {
		missing := make([]string, 0, len(remaining))
		for _, req := range remaining {
			missing = append(missing, formatRequirement(req))
		}

		return fmt.Errorf(
			"RBAC preflight failed for serviceaccount %s/%s; missing permissions: %s. "+
			"Please update infra RBAC/ClusterRoleBinding to support dynamic app namespaces",
			infraNamespace,
			serviceAccount,
			strings.Join(missing, ", "),
		)
	}

	return nil
}

// GetExperimentRun returns details of a requested experiment run
func (c *ChaosExperimentRunHandler) GetExperimentRun(ctx context.Context, projectID string, experimentRunID *string, notifyID *string) (*model.ExperimentRun, error) {
	var pipeline mongo.Pipeline

	if experimentRunID == nil && notifyID == nil {
		return nil, errors.New("experimentRunID or notifyID not provided")
	}

	// Matching with identifiers
	if experimentRunID != nil && *experimentRunID != "" {
		matchIdentifiersStage := bson.D{
			{
				"$match", bson.D{
					{"experiment_run_id", experimentRunID},
					{"project_id", bson.D{{"$eq", projectID}}},
					{"is_removed", false},
				},
			},
		}
		pipeline = append(pipeline, matchIdentifiersStage)
	}

	if notifyID != nil && *notifyID != "" {
		matchIdentifiersStage := bson.D{
			{
				"$match", bson.D{
					{"notify_id", bson.D{{"$eq", notifyID}}},
					{"project_id", bson.D{{"$eq", projectID}}},
					{"is_removed", false},
				},
			},
		}
		pipeline = append(pipeline, matchIdentifiersStage)
	}

	// Adds details of experiment
	addExperimentDetails := bson.D{
		{"$lookup",
			bson.D{
				{"from", "chaosExperiments"},
				{"let", bson.D{{"experimentID", "$experiment_id"}, {"revID", "$revision_id"}}},
				{
					"pipeline", bson.A{
						bson.D{{"$match", bson.D{{"$expr", bson.D{{"$eq", bson.A{"$experiment_id", "$$experimentID"}}}}}}},
						bson.D{
							{"$project", bson.D{
								{"name", 1},
								{"is_custom_experiment", 1},
								{"experiment_type", 1},
								{"revision", bson.D{{
									"$filter", bson.D{
										{"input", "$revision"},
										{"as", "revs"},
										{"cond", bson.D{{
											"$eq", bson.A{"$$revs.revision_id", "$$revID"},
										}}},
									},
								}}},
							}},
						},
					},
				},
				{"as", "experiment"},
			},
		},
	}
	pipeline = append(pipeline, addExperimentDetails)

	// fetchKubernetesInfraDetailsStage adds kubernetes infra details of corresponding experiment_id to each document
	fetchKubernetesInfraDetailsStage := bson.D{
		{"$lookup", bson.D{
			{"from", "chaosInfrastructures"},
			{"let", bson.M{"infraID": "$infra_id"}},
			{
				"pipeline", bson.A{
					bson.D{
						{"$match", bson.D{
							{"$expr", bson.D{
								{"$eq", bson.A{"$infra_id", "$$infraID"}},
							}},
						}},
					},
					bson.D{
						{"$project", bson.D{
							{"token", 0},
							{"infra_ns_exists", 0},
							{"infra_sa_exists", 0},
							{"access_key", 0},
						}},
					},
				},
			},
			{"as", "kubernetesInfraDetails"},
		}},
	}

	pipeline = append(pipeline, fetchKubernetesInfraDetailsStage)

	// Call aggregation on pipeline
	expRunCursor, err := c.chaosExperimentRunOperator.GetAggregateExperimentRuns(pipeline)
	if err != nil {
		return nil, errors.New("DB aggregate stage error: " + err.Error())
	}

	var (
		expRunResponse *model.ExperimentRun
		expRunDetails  []dbChaosExperiment.FlattenedExperimentRun
	)

	if err = expRunCursor.All(context.Background(), &expRunDetails); err != nil {
		return nil, errors.New("error decoding experiment run cursor: " + err.Error())
	}
	if len(expRunDetails) == 0 {
		return nil, errors.New("no matching experiment run")
	}
	if len(expRunDetails[0].KubernetesInfraDetails) == 0 {
		return nil, errors.New("no matching infra found for given experiment run")
	}

	for _, wfRun := range expRunDetails {
		var (
			weightages          []*model.Weightages
			workflowRunManifest string
		)

		if len(wfRun.ExperimentDetails[0].Revision) > 0 {
			revision := wfRun.ExperimentDetails[0].Revision[0]
			for _, v := range revision.Weightages {
				weightages = append(weightages, &model.Weightages{
					FaultName: v.FaultName,
					Weightage: v.Weightage,
				})
			}
			workflowRunManifest = revision.ExperimentManifest
		}
		var chaosInfrastructure *model.Infra

		if len(wfRun.KubernetesInfraDetails) > 0 {
			infra := wfRun.KubernetesInfraDetails[0]
			chaosInfrastructure = &model.Infra{
				InfraID:        infra.InfraID,
				Name:           infra.Name,
				EnvironmentID:  infra.EnvironmentID,
				Description:    &infra.Description,
				PlatformName:   infra.PlatformName,
				IsActive:       infra.IsActive,
				UpdatedAt:      strconv.FormatInt(infra.UpdatedAt, 10),
				CreatedAt:      strconv.FormatInt(infra.CreatedAt, 10),
				InfraNamespace: infra.InfraNamespace,
				ServiceAccount: infra.ServiceAccount,
				InfraScope:     infra.InfraScope,
				StartTime:      infra.StartTime,
				Version:        infra.Version,
				Tags:           infra.Tags,
			}
		}

		expType := string(wfRun.ExperimentDetails[0].ExperimentType)

		expRunResponse = &model.ExperimentRun{
			ExperimentName:     wfRun.ExperimentDetails[0].ExperimentName,
			ExperimentID:       wfRun.ExperimentID,
			ExperimentRunID:    wfRun.ExperimentRunID,
			ExperimentType:     &expType,
			NotifyID:           wfRun.NotifyID,
			Weightages:         weightages,
			ExperimentManifest: workflowRunManifest,
			ProjectID:          wfRun.ProjectID,
			Infra:              chaosInfrastructure,
			Phase:              model.ExperimentRunStatus(wfRun.Phase),
			ResiliencyScore:    wfRun.ResiliencyScore,
			FaultsPassed:       wfRun.FaultsPassed,
			FaultsFailed:       wfRun.FaultsFailed,
			FaultsAwaited:      wfRun.FaultsAwaited,
			FaultsStopped:      wfRun.FaultsStopped,
			FaultsNa:           wfRun.FaultsNA,
			TotalFaults:        wfRun.TotalFaults,
			ExecutionData:      wfRun.ExecutionData,
			IsRemoved:          &wfRun.IsRemoved,
			RunSequence:        int(wfRun.RunSequence),

			UpdatedBy: &model.UserDetails{
				Username: wfRun.UpdatedBy.Username,
			},
			UpdatedAt: strconv.FormatInt(wfRun.UpdatedAt, 10),
			CreatedAt: strconv.FormatInt(wfRun.CreatedAt, 10),
		}
	}

	return expRunResponse, nil
}

// ListExperimentRun returns all the workflow runs for matching identifiers from the DB
func (c *ChaosExperimentRunHandler) ListExperimentRun(projectID string, request model.ListExperimentRunRequest) (*model.ListExperimentRunResponse, error) {
	var pipeline mongo.Pipeline

	// Matching with identifiers
	matchIdentifiersStage := bson.D{
		{
			"$match", bson.D{{
				"$and", bson.A{
					bson.D{
						{"project_id", bson.D{{"$eq", projectID}}},
					},
				},
			}},
		},
	}
	pipeline = append(pipeline, matchIdentifiersStage)

	// Match the workflowRunIds from the input array
	if request.ExperimentRunIDs != nil && len(request.ExperimentRunIDs) != 0 {
		matchWfRunIdStage := bson.D{
			{"$match", bson.D{
				{"experiment_run_id", bson.D{
					{"$in", request.ExperimentRunIDs},
				}},
			}},
		}

		pipeline = append(pipeline, matchWfRunIdStage)
	}

	// Match the workflowIds from the input array
	if request.ExperimentIDs != nil && len(request.ExperimentIDs) != 0 {
		matchWfIdStage := bson.D{
			{"$match", bson.D{
				{"experiment_id", bson.D{
					{"$in", request.ExperimentIDs},
				}},
			}},
		}

		pipeline = append(pipeline, matchWfIdStage)
	}

	// Filtering out the workflows that are deleted/removed
	matchExpIsRemovedStage := bson.D{
		{"$match", bson.D{
			{"is_removed", bson.D{
				{"$eq", false},
			}},
		}},
	}
	pipeline = append(pipeline, matchExpIsRemovedStage)

	addExperimentDetails := bson.D{
		{
			"$lookup",
			bson.D{
				{"from", "chaosExperiments"},
				{"let", bson.D{{"experimentID", "$experiment_id"}, {"revID", "$revision_id"}}},
				{
					"pipeline", bson.A{
						bson.D{{"$match", bson.D{{"$expr", bson.D{{"$eq", bson.A{"$experiment_id", "$$experimentID"}}}}}}},
						bson.D{
							{"$project", bson.D{
								{"name", 1},
								{"experiment_type", 1},
								{"is_custom_experiment", 1},
								{"revision", bson.D{{
									"$filter", bson.D{
										{"input", "$revision"},
										{"as", "revs"},
										{"cond", bson.D{{
											"$eq", bson.A{"$$revs.revision_id", "$$revID"},
										}}},
									},
								}}},
							}},
						},
					},
				},
				{"as", "experiment"},
			},
		},
	}
	pipeline = append(pipeline, addExperimentDetails)

	// Filtering based on multiple parameters
	if request.Filter != nil {

		// Filtering based on workflow name
		if request.Filter.ExperimentName != nil && *request.Filter.ExperimentName != "" {
			matchWfNameStage := bson.D{
				{"$match", bson.D{
					{"experiment.name", bson.D{
						{"$regex", request.Filter.ExperimentName},
					}},
				}},
			}
			pipeline = append(pipeline, matchWfNameStage)
		}

		// Filtering based on workflow run ID
		if request.Filter.ExperimentRunID != nil && *request.Filter.ExperimentRunID != "" {
			matchWfRunIDStage := bson.D{
				{"$match", bson.D{
					{"experiment_run_id", bson.D{
						{"$regex", request.Filter.ExperimentRunID},
					}},
				}},
			}
			pipeline = append(pipeline, matchWfRunIDStage)
		}

		// Filtering based on workflow run status array
		if len(request.Filter.ExperimentRunStatus) > 0 {
			matchWfRunStatusStage := bson.D{
				{"$match", bson.D{
					{"phase", bson.D{
						{"$in", request.Filter.ExperimentRunStatus},
					}},
				}},
			}
			pipeline = append(pipeline, matchWfRunStatusStage)
		}

		// Filtering based on infraID
		if request.Filter.InfraID != nil && *request.Filter.InfraID != "All" && *request.Filter.InfraID != "" {
			matchInfraStage := bson.D{
				{"$match", bson.D{
					{"infra_id", request.Filter.InfraID},
				}},
			}
			pipeline = append(pipeline, matchInfraStage)
		}

		// Filtering based on phase
		if request.Filter.ExperimentStatus != nil && *request.Filter.ExperimentStatus != "All" && *request.Filter.ExperimentStatus != "" {
			filterWfRunPhaseStage := bson.D{
				{"$match", bson.D{
					{"phase", string(*request.Filter.ExperimentStatus)},
				}},
			}
			pipeline = append(pipeline, filterWfRunPhaseStage)
		}

		// Filtering based on date range
		if request.Filter.DateRange != nil {
			endDate := time.Now().UnixMilli()
			if request.Filter.DateRange.EndDate != nil {
				parsedEndDate, err := strconv.ParseInt(*request.Filter.DateRange.EndDate, 10, 64)
				if err != nil {
					return nil, errors.New("unable to parse end date")
				}

				endDate = parsedEndDate
			}

			// Note: StartDate cannot be passed in blank, must be "0"
			startDate, err := strconv.ParseInt(request.Filter.DateRange.StartDate, 10, 64)
			if err != nil {
				return nil, errors.New("unable to parse start date")
			}

			filterWfRunDateStage := bson.D{
				{
					"$match",
					bson.D{{"updated_at", bson.D{
						{"$lte", endDate},
						{"$gte", startDate},
					}}},
				},
			}
			pipeline = append(pipeline, filterWfRunDateStage)
		}
	}

	var sortStage bson.D

	switch {
	case request.Sort != nil && request.Sort.Field == model.ExperimentSortingFieldTime:
		// Sorting based on created time
		if request.Sort.Ascending != nil && *request.Sort.Ascending {
			sortStage = bson.D{
				{"$sort", bson.D{
					{"created_at", 1},
				}},
			}
		} else {
			sortStage = bson.D{
				{"$sort", bson.D{
					{"created_at", -1},
				}},
			}
		}
	case request.Sort != nil && request.Sort.Field == model.ExperimentSortingFieldName:
		// Sorting based on ExperimentName time
		if request.Sort.Ascending != nil && *request.Sort.Ascending {
			sortStage = bson.D{
				{"$sort", bson.D{
					{"experiment.name", 1},
				}},
			}
		} else {
			sortStage = bson.D{
				{"$sort", bson.D{
					{"experiment.name", -1},
				}},
			}
		}
	default:
		// Default sorting: sorts it by created_at time in descending order
		sortStage = bson.D{
			{"$sort", bson.D{
				{"created_at", -1},
			}},
		}
	}

	// fetchKubernetesInfraDetailsStage adds infra details of corresponding experiment_id to each document
	fetchKubernetesInfraDetailsStage := bson.D{
		{"$lookup", bson.D{
			{"from", "chaosInfrastructures"},
			{"let", bson.M{"infraID": "$infra_id"}},
			{
				"pipeline", bson.A{
					bson.D{
						{"$match", bson.D{
							{"$expr", bson.D{
								{"$eq", bson.A{"$infra_id", "$$infraID"}},
							}},
						}},
					},
					bson.D{
						{"$project", bson.D{
							{"token", 0},
							{"infra_ns_exists", 0},
							{"infra_sa_exists", 0},
							{"access_key", 0},
						}},
					},
				},
			},
			{"as", "kubernetesInfraDetails"},
		}},
	}

	pipeline = append(pipeline, fetchKubernetesInfraDetailsStage)

	// Pagination or adding a default limit of 15 if pagination not provided
	paginatedExperiments := bson.A{
		sortStage,
	}

	if request.Pagination != nil {
		paginationSkipStage := bson.D{
			{"$skip", request.Pagination.Page * request.Pagination.Limit},
		}
		paginationLimitStage := bson.D{
			{"$limit", request.Pagination.Limit},
		}

		paginatedExperiments = append(paginatedExperiments, paginationSkipStage, paginationLimitStage)
	} else {
		limitStage := bson.D{
			{"$limit", 15},
		}

		paginatedExperiments = append(paginatedExperiments, limitStage)
	}

	// Add two stages where we first count the number of filtered workflow and then paginate the results
	facetStage := bson.D{
		{"$facet", bson.D{
			{"total_filtered_experiment_runs", bson.A{
				bson.D{{"$count", "count"}},
			}},
			{"flattened_experiment_runs", paginatedExperiments},
		}},
	}
	pipeline = append(pipeline, facetStage)

	// Call aggregation on pipeline
	workflowsCursor, err := c.chaosExperimentRunOperator.GetAggregateExperimentRuns(pipeline)
	if err != nil {
		return nil, errors.New("DB aggregate stage error: " + err.Error())
	}

	var (
		result    []*model.ExperimentRun
		workflows []dbChaosExperiment.AggregatedExperimentRuns
	)

	if err = workflowsCursor.All(context.Background(), &workflows); err != nil || len(workflows) == 0 {
		return &model.ListExperimentRunResponse{
			TotalNoOfExperimentRuns: 0,
			ExperimentRuns:          result,
		}, errors.New("error decoding experiment runs cursor: " + err.Error())
	}
	if len(workflows) == 0 {
		return &model.ListExperimentRunResponse{
			TotalNoOfExperimentRuns: 0,
			ExperimentRuns:          result,
		}, nil
	}

	for _, workflow := range workflows[0].FlattenedExperimentRuns {
		var (
			weightages          []*model.Weightages
			workflowRunManifest string
			workflowType        string
			workflowName        string
		)

		if len(workflow.ExperimentDetails) > 0 {
			workflowType = string(workflow.ExperimentDetails[0].ExperimentType)
			workflowName = workflow.ExperimentDetails[0].ExperimentName
			if len(workflow.ExperimentDetails[0].Revision) > 0 {
				revision := workflow.ExperimentDetails[0].Revision[0]
				for _, v := range revision.Weightages {
					weightages = append(weightages, &model.Weightages{
						FaultName: v.FaultName,
						Weightage: v.Weightage,
					})
				}
				workflowRunManifest = revision.ExperimentManifest
			}
		}
		var chaosInfrastructure *model.Infra

		if len(workflow.KubernetesInfraDetails) > 0 {
			infra := workflow.KubernetesInfraDetails[0]
			infraType := model.InfrastructureType(infra.InfraType)
			chaosInfrastructure = &model.Infra{
				InfraID:        infra.InfraID,
				Name:           infra.Name,
				EnvironmentID:  infra.EnvironmentID,
				Description:    &infra.Description,
				PlatformName:   infra.PlatformName,
				IsActive:       infra.IsActive,
				UpdatedAt:      strconv.FormatInt(infra.UpdatedAt, 10),
				CreatedAt:      strconv.FormatInt(infra.CreatedAt, 10),
				InfraNamespace: infra.InfraNamespace,
				ServiceAccount: infra.ServiceAccount,
				InfraScope:     infra.InfraScope,
				StartTime:      infra.StartTime,
				Version:        infra.Version,
				Tags:           infra.Tags,
				InfraType:      &infraType,
			}
		}

		newExperimentRun := model.ExperimentRun{
			ExperimentName:     workflowName,
			ExperimentType:     &workflowType,
			ExperimentID:       workflow.ExperimentID,
			ExperimentRunID:    workflow.ExperimentRunID,
			Weightages:         weightages,
			ExperimentManifest: workflowRunManifest,
			ProjectID:          workflow.ProjectID,
			Infra:              chaosInfrastructure,
			Phase:              model.ExperimentRunStatus(workflow.Phase),
			ResiliencyScore:    workflow.ResiliencyScore,
			FaultsPassed:       workflow.FaultsPassed,
			FaultsFailed:       workflow.FaultsFailed,
			FaultsAwaited:      workflow.FaultsAwaited,
			FaultsStopped:      workflow.FaultsStopped,
			FaultsNa:           workflow.FaultsNA,
			TotalFaults:        workflow.TotalFaults,
			ExecutionData:      workflow.ExecutionData,
			IsRemoved:          &workflow.IsRemoved,
			UpdatedBy: &model.UserDetails{
				Username: workflow.UpdatedBy.Username,
			},
			UpdatedAt:   strconv.FormatInt(workflow.UpdatedAt, 10),
			CreatedAt:   strconv.FormatInt(workflow.CreatedAt, 10),
			RunSequence: int(workflow.RunSequence),
		}
		result = append(result, &newExperimentRun)
	}

	totalFilteredExperimentRunsCounter := 0
	if len(workflows) > 0 && len(workflows[0].TotalFilteredExperimentRuns) > 0 {
		totalFilteredExperimentRunsCounter = workflows[0].TotalFilteredExperimentRuns[0].Count
	}

	output := model.ListExperimentRunResponse{
		TotalNoOfExperimentRuns: totalFilteredExperimentRunsCounter,
		ExperimentRuns:          result,
	}

	return &output, nil
}

// traceExperimentExecution logs fault execution to observability backend.
// When OTEL is enabled, creates an OTEL root span for the experiment run.
// Falls back to Langfuse REST when OTEL is not configured.
func traceExperimentExecution(ctx context.Context, notifyID string, experimentID string, experimentName string, infra dbChaosInfra.ChaosInfra, projectID string) error {
	namespace := ""
	if infra.InfraNamespace != nil {
		namespace = *infra.InfraNamespace
	}

	// OTEL path: emit instant start span + create long-running end span
	if observability.OTELTracerEnabled() {
		startAttrs := []attribute.KeyValue{
			attribute.String("experiment.id", experimentID),
			attribute.String("experiment.name", experimentName),
			attribute.String("experiment.fault_name", "chaos-workflow"),
			attribute.String("experiment.session_id", notifyID),
			attribute.String("infra.id", infra.InfraID),
			attribute.String("project.id", projectID),
			attribute.String("infra.namespace", namespace),
			attribute.String("experiment.phase", "injection"),
			attribute.String("experiment.priority", "high"),
		}

		// Long-running root span — ended later by scoreExperimentRun, appears LAST
		spanCtx, _ := observability.StartExperimentSpan(ctx, notifyID, startAttrs...)
		logrus.Infof("[OTEL] Started experiment-run span: traceID=%s experiment=%s", notifyID, experimentName)

		// Instant child span — shares the same traceID as the root span
		observability.EmitExperimentStartSpan(spanCtx, startAttrs...)
		logrus.Infof("[OTEL] Emitted experiment-triggered span: traceID=%s experiment=%s", notifyID, experimentName)
		return nil
	}

	// Langfuse REST fallback
	tracer := observability.GetLangfuseTracer()
	return tracer.TraceExperimentExecution(ctx, &observability.ExperimentExecutionDetails{
		TraceID:        notifyID,
		ExperimentID:   experimentID,
		ExperimentName: experimentName,
		FaultName:      "chaos-workflow",
		SessionID:      notifyID,
		AgentID:        infra.InfraID,
		ProjectID:      projectID,
		Namespace:      namespace,
		Phase:          "injection",
		Priority:       "high",
	})
}

// completeExperimentExecution logs fault execution completion to observability backend.
// When OTEL is enabled, this is a no-op (spans are ended in ChaosExperimentRunEvent).
// Falls back to Langfuse REST when OTEL is not configured.
func completeExperimentExecution(ctx context.Context, notifyID string, experimentID string, experimentName string, status string, result string) error {
	if observability.OTELTracerEnabled() {
		// OTEL spans are ended via endExperimentOTELSpan; no separate "complete" needed
		return nil
	}

	tracer := observability.GetLangfuseTracer()
	return tracer.CompleteExperimentExecution(ctx, notifyID, &observability.ExperimentCompletionDetails{
		ExperimentID:   experimentID,
		ExperimentName: experimentName,
		Status:         status,
		Result:         result,
	})
}

// traceExperimentObservation logs continuous workflow events.
// When OTEL is enabled, adds events to the active experiment span.
// Falls back to Langfuse REST when OTEL is not configured.
func traceExperimentObservation(ctx context.Context, traceID string, event model.ExperimentRunRequest, executionData types.ExecutionData, metrics *types.ExperimentRunMetrics) {
	if traceID == "" {
		return
	}

	// OTEL path: add events and child spans to the active experiment span
	if observability.OTELTracerEnabled() {
		observationName := fmt.Sprintf("workflow-event: %s (%s)", executionData.Phase, executionData.EventType)
		if executionData.Phase == "" && executionData.EventType == "" {
			observationName = "workflow-event"
		}

		eventAttrs := []attribute.KeyValue{
			attribute.String("experiment.id", event.ExperimentID),
			attribute.String("experiment.run_id", event.ExperimentRunID),
			attribute.String("experiment.name", event.ExperimentName),
			attribute.String("event.type", executionData.EventType),
			attribute.String("event.phase", executionData.Phase),
			attribute.String("event.message", executionData.Message),
			attribute.Bool("event.completed", event.Completed),
		}

		if metrics != nil {
			eventAttrs = append(eventAttrs,
				attribute.Float64("metrics.resiliency_score", metrics.ResiliencyScore),
				attribute.Int("metrics.total_experiments", metrics.TotalExperiments),
				attribute.Int("metrics.experiments_passed", metrics.ExperimentsPassed),
				attribute.Int("metrics.experiments_failed", metrics.ExperimentsFailed),
				attribute.Int("metrics.experiments_awaited", metrics.ExperimentsAwaited),
				attribute.Int("metrics.experiments_stopped", metrics.ExperimentsStopped),
				attribute.Int("metrics.experiments_na", metrics.ExperimentsNA),
			)
		}

		// Add execution data as JSON attribute
		eventAttrs = append(eventAttrs,
			attribute.String("execution_data", observability.MarshalJSON(executionData)),
		)

		observability.AddExperimentEvent(traceID, observationName, eventAttrs...)

		// Also update the span's top-level attributes with latest phase
		observability.SetExperimentSpanAttributes(traceID,
			attribute.String("experiment.phase", executionData.Phase),
			attribute.String("experiment.event_type", executionData.EventType),
		)

		logrus.Infof("[OTEL] Added event '%s' to span: traceID=%s", observationName, traceID)
		return
	}

	// Langfuse REST fallback
	tracer := observability.GetLangfuseTracer()

	input := map[string]interface{}{
		"experimentID":    event.ExperimentID,
		"experimentRunID": event.ExperimentRunID,
		"experimentName":  event.ExperimentName,
		"revisionID":      event.RevisionID,
		"completed":       event.Completed,
	}
	if event.NotifyID != nil {
		input["notifyID"] = *event.NotifyID
	}

	output := map[string]interface{}{
		"executionData": executionData,
	}
	if metrics != nil {
		output["metrics"] = metrics
	}

	metadata := map[string]interface{}{
		"eventType": executionData.EventType,
		"phase":     executionData.Phase,
		"message":   executionData.Message,
	}

	observationName := fmt.Sprintf("workflow-event: %s (%s)", executionData.Phase, executionData.EventType)
	if executionData.Phase == "" && executionData.EventType == "" {
		observationName = "workflow-event"
	}

	logrus.Infof("[Tracing] Creating observation: %s for trace: %s", observationName, traceID)

	now := time.Now()

	_ = tracer.TraceExperimentObservation(ctx, &observability.ExperimentObservationDetails{
		TraceID:   traceID,
		Name:      observationName,
		Type:      "EVENT",
		StartTime: now.Format(time.RFC3339),
		EndTime:   now.Format(time.RFC3339),
		Input:     input,
		Output:    output,
		Metadata:  metadata,
	})
}

// scoreExperimentRun logs resiliency scores and fault metrics after experiment completion.
// When OTEL is enabled, sets span attributes for metrics and ends the span.
// Langfuse scores are always submitted via REST (OTEL has no native score concept).
func scoreExperimentRun(ctx context.Context, traceID string, metrics *types.ExperimentRunMetrics, status string) {
	if traceID == "" || metrics == nil {
		return
	}

	// OTEL path: set final metric attributes and end the experiment span
	if observability.OTELTracerEnabled() {
		observability.SetExperimentSpanAttributes(traceID,
			attribute.String("experiment.final_phase", status),
			attribute.Float64("experiment.resiliency_score", metrics.ResiliencyScore),
			attribute.Int("experiment.total_faults", metrics.TotalExperiments),
			attribute.Int("experiment.faults_passed", metrics.ExperimentsPassed),
			attribute.Int("experiment.faults_failed", metrics.ExperimentsFailed),
			attribute.Int("experiment.faults_awaited", metrics.ExperimentsAwaited),
			attribute.Int("experiment.faults_stopped", metrics.ExperimentsStopped),
			attribute.Int("experiment.faults_na", metrics.ExperimentsNA),
		)

		// Set span status based on experiment outcome
		_, span := observability.GetExperimentSpan(traceID)
		if span != nil {
			phaseLower := strings.ToLower(status)
			if strings.Contains(phaseLower, "failed") || strings.Contains(phaseLower, "error") {
				span.SetStatus(codes.Error, fmt.Sprintf("experiment %s: resiliency=%.1f%%", status, metrics.ResiliencyScore))
			} else {
				span.SetStatus(codes.Ok, fmt.Sprintf("experiment %s: resiliency=%.1f%%", status, metrics.ResiliencyScore))
			}
		}

		// End the OTEL span — this triggers export to Langfuse via OTLP
		observability.EndExperimentSpan(traceID)
		logrus.Infof("[OTEL] Ended experiment span: traceID=%s phase=%s resiliency=%.1f%%", traceID, status, metrics.ResiliencyScore)
	}

	// Langfuse REST scores — submitted regardless of OTEL (scores need REST API)
	tracer := observability.GetLangfuseTracer()
	if !tracer.IsEnabled() {
		return
	}

	// Score 1: Resiliency Score
	_ = tracer.ScoreExperimentExecution(ctx, &observability.ExperimentScoreDetails{
		TraceID: traceID,
		Name:    "resiliency_score",
		Value:   metrics.ResiliencyScore,
		Comment: fmt.Sprintf("Overall resiliency score (0-100 scale) for experiment phase: %s", status),
		Source:  "API",
	})

	// Score 2: Experiments Passed
	passedScore := float64(metrics.ExperimentsPassed) / float64(metrics.TotalExperiments) * 100
	if metrics.TotalExperiments == 0 {
		passedScore = 0
	}
	_ = tracer.ScoreExperimentExecution(ctx, &observability.ExperimentScoreDetails{
		TraceID: traceID,
		Name:    "experiments_passed_percentage",
		Value:   passedScore,
		Comment: fmt.Sprintf("Percentage of experiments passed: %d/%d", metrics.ExperimentsPassed, metrics.TotalExperiments),
		Source:  "API",
	})

	// Score 3: Experiments Failed
	failedScore := float64(metrics.ExperimentsFailed) / float64(metrics.TotalExperiments) * 100
	if metrics.TotalExperiments == 0 {
		failedScore = 0
	}
	_ = tracer.ScoreExperimentExecution(ctx, &observability.ExperimentScoreDetails{
		TraceID: traceID,
		Name:    "experiments_failed_percentage",
		Value:   failedScore,
		Comment: fmt.Sprintf("Percentage of experiments failed: %d/%d", metrics.ExperimentsFailed, metrics.TotalExperiments),
		Source:  "API",
	})

	// Score 4: Experiments Awaited
	awaitedScore := float64(metrics.ExperimentsAwaited) / float64(metrics.TotalExperiments) * 100
	if metrics.TotalExperiments == 0 {
		awaitedScore = 0
	}
	_ = tracer.ScoreExperimentExecution(ctx, &observability.ExperimentScoreDetails{
		TraceID: traceID,
		Name:    "experiments_awaited_percentage",
		Value:   awaitedScore,
		Comment: fmt.Sprintf("Percentage of experiments awaited: %d/%d", metrics.ExperimentsAwaited, metrics.TotalExperiments),
		Source:  "API",
	})

	// Score 5: Experiments Stopped
	stoppedScore := float64(metrics.ExperimentsStopped) / float64(metrics.TotalExperiments) * 100
	if metrics.TotalExperiments == 0 {
		stoppedScore = 0
	}
	_ = tracer.ScoreExperimentExecution(ctx, &observability.ExperimentScoreDetails{
		TraceID: traceID,
		Name:    "experiments_stopped_percentage",
		Value:   stoppedScore,
		Comment: fmt.Sprintf("Percentage of experiments stopped: %d/%d", metrics.ExperimentsStopped, metrics.TotalExperiments),
		Source:  "API",
	})

	// Score 6: Experiments Not Applicable
	naScore := float64(metrics.ExperimentsNA) / float64(metrics.TotalExperiments) * 100
	if metrics.TotalExperiments == 0 {
		naScore = 0
	}
	_ = tracer.ScoreExperimentExecution(ctx, &observability.ExperimentScoreDetails{
		TraceID: traceID,
		Name:    "experiments_na_percentage",
		Value:   naScore,
		Comment: fmt.Sprintf("Percentage of experiments not applicable: %d/%d", metrics.ExperimentsNA, metrics.TotalExperiments),
		Source:  "API",
	})

	// Score 7: Total Experiments Count
	_ = tracer.ScoreExperimentExecution(ctx, &observability.ExperimentScoreDetails{
		TraceID: traceID,
		Name:    "total_experiments_count",
		Value:   float64(metrics.TotalExperiments),
		Comment: fmt.Sprintf("Total number of experiments executed"),
		Source:  "API",
	})
}

// RunChaosWorkFlow sends workflow run request(single run workflow only) to chaos_infra on workflow re-run request
func (c *ChaosExperimentRunHandler) RunChaosWorkFlow(ctx context.Context, projectID string, workflow dbChaosExperiment.ChaosExperimentRequest, r *store.StateData) (*model.RunChaosExperimentResponse, error) {
	var notifyID string
	infra, err := dbChaosInfra.NewInfrastructureOperator(c.mongodbOperator).GetInfra(workflow.InfraID)
	if err != nil {
		return nil, err
	}
	if !infra.IsActive {
		return nil, errors.New("experiment re-run failed due to inactive infra")
	}

	if err := c.preflightInfraRBAC(ctx, &infra); err != nil {
		return nil, err
	}

	var (
		workflowManifest v1alpha1.Workflow
	)

	currentTime := time.Now().UnixMilli()
	notifyID = uuid.New().String()

	// Trace experiment execution start to observability backend
	traceExperimentExecution(ctx, notifyID, workflow.ExperimentID, workflow.Name, infra, projectID)

	if len(workflow.Revision) == 0 {
		return nil, errors.New("no revisions found")
	}

	sort.Slice(workflow.Revision, func(i, j int) bool {
		return workflow.Revision[i].UpdatedAt > workflow.Revision[j].UpdatedAt
	})

	resKind := gjson.Get(workflow.Revision[0].ExperimentManifest, "kind").String()
	if strings.ToLower(resKind) == "cronworkflow" {
		return &model.RunChaosExperimentResponse{NotifyID: notifyID}, c.RunCronExperiment(ctx, projectID, workflow, r)
	}

	err = json.Unmarshal([]byte(workflow.Revision[0].ExperimentManifest), &workflowManifest)
	if err != nil {
		return nil, errors.New("failed to unmarshal workflow manifest")
	}

	if normalizeInstallTemplates(workflowManifest.Spec.Templates) {
		ensureInstallTimeoutParam(&workflowManifest.Spec.Arguments)
	}

	var resScore float64 = 0

	if _, found := workflowManifest.Labels["infra_id"]; !found {
		return nil, errors.New("failed to rerun the chaos experiment due to invalid metadata/labels. Check the troubleshooting guide or contact support")
	}
	workflowManifest.Labels["notify_id"] = notifyID
	workflowManifest.Name = workflowManifest.Name + "-" + strconv.FormatInt(currentTime, 10)

	// Detect container runtime once for all ChaosEngine templates in this workflow
	var (
		clusterRuntime, clusterSocketPath string
		clusterClientset                *kubernetes.Clientset
	)
	if cs, kerr := buildKubeClientset(); kerr == nil {
		clusterClientset = cs
		clusterRuntime, clusterSocketPath = detectNodeContainerRuntime(cs)
	}
	wfParams := buildWorkflowParameterMap(workflowManifest.Spec.Arguments)

	var probes []dbChaosExperimentRun.Probes
	for i, template := range workflowManifest.Spec.Templates {
		artifacts := template.Inputs.Artifacts
		logrus.Infof("[Artifact Processing] Template %s has %d artifacts", template.Name, len(artifacts))
		for j := range artifacts {
			if artifacts[j].Raw == nil {
				logrus.Debugf("[Artifact %d] Raw is nil, skipping", j)
				continue
			}

			data := artifacts[j].Raw.Data
			if len(data) == 0 {
				logrus.Debugf("[Artifact %d] Data is empty, skipping", j)
				continue
			}

			// Normalize container runtime env vars before processing
			data = normalizeContainerRuntimeInYAML(data, clusterRuntime, clusterSocketPath)

			var (
				meta       chaosTypes.ChaosEngine
				annotation = make(map[string]string)
			)
			err := yaml.Unmarshal([]byte(data), &meta)
			if err != nil {
				return nil, errors.New("failed to unmarshal chaosengine")
			}
			if strings.ToLower(meta.Kind) != "chaosengine" {
				continue
			}

			if meta.Annotations != nil {
				annotation = meta.Annotations
			}

			var annotationArray []string
			for _, key := range annotation {
				var manifestAnnotation []dbChaosExperiment.ProbeAnnotations
				err := json.Unmarshal([]byte(key), &manifestAnnotation)
				if err != nil {
					return nil, errors.New("failed to unmarshal experiment annotation object")
				}
				for _, annotationKey := range manifestAnnotation {
					annotationArray = append(annotationArray, annotationKey.Name)
				}
			}
			probes = append(probes, dbChaosExperimentRun.Probes{
				artifacts[j].Name,
				annotationArray,
			})

			meta.Annotations = annotation

			if meta.Labels == nil {
				meta.Labels = map[string]string{
					"infra_id":        workflow.InfraID,
					"step_pod_name":   "{{pod.name}}",
					"workflow_run_id": "{{workflow.uid}}",
				}
			} else {
				meta.Labels["infra_id"] = workflow.InfraID
				meta.Labels["step_pod_name"] = "{{pod.name}}"
				meta.Labels["workflow_run_id"] = "{{workflow.uid}}"
			}

			meta.Spec.Appinfo.Appns = resolveWorkflowParameterValue(meta.Spec.Appinfo.Appns, wfParams)
			meta.Spec.Appinfo.Applabel = resolveWorkflowParameterValue(meta.Spec.Appinfo.Applabel, wfParams)

			// Normalize appkind by detecting actual resource type from cluster
			if clusterClientset != nil {
				if normalizeChaosEngineAppKind(clusterClientset, &meta) {
					logrus.WithField("experiment", meta.Spec.Experiments[0].Name).Debug("Updated appkind for ChaosEngine")
				}
			}

			if len(meta.Spec.Experiments[0].Spec.Probe) != 0 {
				meta.Spec.Experiments[0].Spec.Probe = utils.TransformProbe(meta.Spec.Experiments[0].Spec.Probe)
			}

			// OTEL Hook 1: Create a per-fault child span with config details
			if observability.OTELTracerEnabled() && len(meta.Spec.Experiments) > 0 {
				exp := meta.Spec.Experiments[0]
				faultName := exp.Name

				faultAttrs := []attribute.KeyValue{
					attribute.String("experiment.id", workflow.ExperimentID),
					attribute.String("fault.name", faultName),
					attribute.String("fault.target_namespace", meta.Spec.Appinfo.Appns),
					attribute.String("fault.target_label", meta.Spec.Appinfo.Applabel),
					attribute.String("fault.target_kind", string(meta.Spec.Appinfo.AppKind)),
					attribute.String("fault.engine_template", template.Name),
				}

				// Extract chaos params from experiment env vars
				for _, envVar := range exp.Spec.Components.ENV {
					switch envVar.Name {
					case "TOTAL_CHAOS_DURATION":
						faultAttrs = append(faultAttrs, attribute.String("fault.chaos_duration", envVar.Value))
					case "CPU_CORES":
						faultAttrs = append(faultAttrs, attribute.String("fault.cpu_cores", envVar.Value))
					case "MEMORY_CONSUMPTION":
						faultAttrs = append(faultAttrs, attribute.String("fault.memory_consumption", envVar.Value))
					case "FILL_PERCENTAGE":
						faultAttrs = append(faultAttrs, attribute.String("fault.fill_percentage", envVar.Value))
					case "NETWORK_PACKET_LOSS_PERCENTAGE":
						faultAttrs = append(faultAttrs, attribute.String("fault.network_loss_pct", envVar.Value))
					case "CHAOS_INTERVAL":
						faultAttrs = append(faultAttrs, attribute.String("fault.chaos_interval", envVar.Value))
					}
				}

				// Add probe names
				var probeNames []string
				for _, p := range exp.Spec.Probe {
					probeNames = append(probeNames, p.Name)
				}
				if len(probeNames) > 0 {
					faultAttrs = append(faultAttrs, attribute.String("fault.probes", strings.Join(probeNames, ",")))
				}

				observability.StartFaultSpan(notifyID, faultName, faultAttrs...)
				logrus.Infof("[OTEL] Created per-fault span: fault=%s target=%s/%s", faultName, meta.Spec.Appinfo.AppKind, meta.Spec.Appinfo.Applabel)
			}

			res, err := yaml.Marshal(&meta)
			if err != nil {
				return nil, errors.New("failed to marshal chaosengine")
			}
			workflowManifest.Spec.Templates[i].Inputs.Artifacts[j].Raw.Data = string(res)
		}
	}

	// Updating updated_at field
	filter := bson.D{
		{"experiment_id", workflow.ExperimentID},
	}
	update := bson.D{
		{
			"$set", bson.D{
				{"updated_at", currentTime},
			},
		},
	}
	err = c.chaosExperimentOperator.UpdateChaosExperiment(context.Background(), filter, update)
	if err != nil {
		logrus.Error("Failed to update updated_at")
		return nil, err
	}

	executionData := types.ExecutionData{
		Name:         workflowManifest.Name,
		Phase:        string(model.ExperimentRunStatusQueued),
		ExperimentID: workflow.ExperimentID,
	}

	parsedData, err := json.Marshal(executionData)
	if err != nil {
		logrus.Error("Failed to parse execution data")
		return nil, err
	}

	var (
		wc      = writeconcern.New(writeconcern.WMajority())
		rc      = readconcern.Snapshot()
		txnOpts = options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)
	)

	tkn := ctx.Value(authorization.AuthKey).(string)
	username, err := authorization.GetUsername(tkn)
	if err != nil {
		return nil, err
	}

	session, err := mongodb.MgoClient.StartSession()
	if err != nil {
		logrus.Errorf("failed to start mongo session %v", err)
		return nil, err
	}

	err = mongo.WithSession(context.Background(), session, func(sessionContext mongo.SessionContext) error {
		if err = session.StartTransaction(txnOpts); err != nil {
			logrus.Errorf("failed to start mongo session transaction %v", err)
			return err
		}
		expRunDetail := []dbChaosExperiment.ExperimentRunDetail{
			{
				Phase:       executionData.Phase,
				Completed:   false,
				ProjectID:   projectID,
				NotifyID:    &notifyID,
				RunSequence: workflow.TotalExperimentRuns + 1,
				Audit: mongodb.Audit{
					IsRemoved: false,
					CreatedAt: currentTime,
					CreatedBy: mongodb.UserDetailResponse{
						Username: username,
					},
					UpdatedAt: currentTime,
					UpdatedBy: mongodb.UserDetailResponse{
						Username: username,
					},
				},
			},
		}

		filter = bson.D{
			{"experiment_id", workflow.ExperimentID},
		}
		update = bson.D{
			{
				"$set", bson.D{
					{"updated_at", currentTime},
					{"total_experiment_runs", workflow.TotalExperimentRuns + 1},
				},
			},
			{
				"$push", bson.D{
					{"recent_experiment_run_details", bson.D{
						{"$each", expRunDetail},
						{"$position", 0},
						{"$slice", 10},
					}},
				},
			},
		}

		err = c.chaosExperimentOperator.UpdateChaosExperiment(sessionContext, filter, update)
		if err != nil {
			logrus.Error("Failed to update experiment collection")
		}

		err = c.chaosExperimentRunOperator.CreateExperimentRun(sessionContext, dbChaosExperimentRun.ChaosExperimentRun{
			InfraID:      workflow.InfraID,
			ExperimentID: workflow.ExperimentID,
			Phase:        string(model.ExperimentRunStatusQueued),
			RevisionID:   workflow.Revision[0].RevisionID,
			ProjectID:    projectID,
			Audit: mongodb.Audit{
				IsRemoved: false,
				CreatedAt: currentTime,
				CreatedBy: mongodb.UserDetailResponse{
					Username: username,
				},
				UpdatedAt: currentTime,
				UpdatedBy: mongodb.UserDetailResponse{
					Username: username,
				},
			},
			NotifyID:        &notifyID,
			Completed:       false,
			ResiliencyScore: &resScore,
			ExecutionData:   string(parsedData),
			RunSequence:     workflow.TotalExperimentRuns + 1,
			Probes:          probes,
		})
		if err != nil {
			logrus.Error("Failed to create run operation in db")
			return err
		}

		if err = session.CommitTransaction(sessionContext); err != nil {
			logrus.Errorf("failed to commit session transaction %v", err)
			return err
		}
		return nil
	})

	if err != nil {
		if abortErr := session.AbortTransaction(ctx); abortErr != nil {
			logrus.Errorf("failed to abort session transaction %v", err)
			return nil, abortErr
		}
		return nil, err
	}

	session.EndSession(ctx)

	// Convert updated manifest to string
	manifestString, err := json.Marshal(workflowManifest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal experiment manifest, err: %v", err)
	}

	// Generate Probe in the manifest
	workflowManifest, err = c.probeService.GenerateExperimentManifestWithProbes(string(manifestString), projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate probes in workflow manifest, err: %v", err)
	}

	manifest, err := yaml.Marshal(workflowManifest)
	if err != nil {
		return nil, err
	}
	if r != nil {
		chaos_infrastructure.SendExperimentToSubscriber(projectID, &model.ChaosExperimentRequest{
			ExperimentID:       &workflow.ExperimentID,
			ExperimentManifest: string(manifest),
			InfraID:            workflow.InfraID,
		}, &username, nil, "create", r)
	}

	// Trace experiment execution completion to observability backend
	completeExperimentExecution(ctx, notifyID, workflow.ExperimentID, workflow.Name, "PASS", "Experiment workflow submitted successfully to infrastructure")

	return &model.RunChaosExperimentResponse{
		NotifyID: notifyID,
	}, nil
}

func (c *ChaosExperimentRunHandler) RunCronExperiment(ctx context.Context, projectID string, workflow dbChaosExperiment.ChaosExperimentRequest, r *store.StateData) error {
	var (
		cronExperimentManifest v1alpha1.CronWorkflow
	)

	if len(workflow.Revision) == 0 {
		return errors.New("no revisions found")
	}
	sort.Slice(workflow.Revision, func(i, j int) bool {
		return workflow.Revision[i].UpdatedAt > workflow.Revision[j].UpdatedAt
	})

	cronExperimentManifest, err := c.probeService.GenerateCronExperimentManifestWithProbes(workflow.Revision[0].ExperimentManifest, workflow.ProjectID)
	if err != nil {
		return errors.New("failed to unmarshal experiment manifest")
	}

	if normalizeInstallTemplates(cronExperimentManifest.Spec.WorkflowSpec.Templates) {
		ensureInstallTimeoutParam(&cronExperimentManifest.Spec.WorkflowSpec.Arguments)
	}

	// Detect container runtime once for all ChaosEngine templates in this cron workflow
	var (
		cronRuntime, cronSocketPath string
		cronClientset              *kubernetes.Clientset
	)
	if cs, kerr := buildKubeClientset(); kerr == nil {
		cronClientset = cs
		cronRuntime, cronSocketPath = detectNodeContainerRuntime(cs)
	}
	cronParams := buildWorkflowParameterMap(cronExperimentManifest.Spec.WorkflowSpec.Arguments)

	for i, template := range cronExperimentManifest.Spec.WorkflowSpec.Templates {
		artifacts := template.Inputs.Artifacts
		for j := range artifacts {
			if artifacts[j].Raw == nil {
				continue
			}

			data := artifacts[j].Raw.Data
			if len(data) == 0 {
				continue
			}

			// Normalize container runtime env vars before processing
			data = normalizeContainerRuntimeInYAML(data, cronRuntime, cronSocketPath)

			var meta chaosTypes.ChaosEngine
			annotation := make(map[string]string)
			err := yaml.Unmarshal([]byte(data), &meta)
			if err != nil {
				return errors.New("failed to unmarshal chaosengine")
			}
			if strings.ToLower(meta.Kind) != "chaosengine" {
				continue
			}

			if meta.Annotations != nil {
				annotation = meta.Annotations
			}
			meta.Annotations = annotation

			if meta.Labels == nil {
				meta.Labels = map[string]string{
					"infra_id":        workflow.InfraID,
					"step_pod_name":   "{{pod.name}}",
					"workflow_run_id": "{{workflow.uid}}",
				}
			} else {
				meta.Labels["infra_id"] = workflow.InfraID
				meta.Labels["step_pod_name"] = "{{pod.name}}"
				meta.Labels["workflow_run_id"] = "{{workflow.uid}}"
			}

			meta.Spec.Appinfo.Appns = resolveWorkflowParameterValue(meta.Spec.Appinfo.Appns, cronParams)
			meta.Spec.Appinfo.Applabel = resolveWorkflowParameterValue(meta.Spec.Appinfo.Applabel, cronParams)

			// Normalize appkind by detecting actual resource type from cluster
			if cronClientset != nil {
				if normalizeChaosEngineAppKind(cronClientset, &meta) {
					logrus.WithField("experiment", meta.Spec.Experiments[0].Name).Debug("Updated appkind for ChaosEngine in cron")
				}
			}

			if len(meta.Spec.Experiments[0].Spec.Probe) != 0 {
				meta.Spec.Experiments[0].Spec.Probe = utils.TransformProbe(meta.Spec.Experiments[0].Spec.Probe)
			}
			res, err := yaml.Marshal(&meta)
			if err != nil {
				return errors.New("failed to marshal chaosengine")
			}
			cronExperimentManifest.Spec.WorkflowSpec.Templates[i].Inputs.Artifacts[j].Raw.Data = string(res)
		}
	}

	manifest, err := yaml.Marshal(cronExperimentManifest)
	if err != nil {
		return err
	}

	tkn := ctx.Value(authorization.AuthKey).(string)
	username, err := authorization.GetUsername(tkn)
	if err != nil {
		return err
	}

	if r != nil {
		chaos_infrastructure.SendExperimentToSubscriber(projectID, &model.ChaosExperimentRequest{
			ExperimentID:       &workflow.ExperimentID,
			ExperimentManifest: string(manifest),
			InfraID:            workflow.InfraID,
		}, &username, nil, "create", r)
	}

	return nil
}

func (c *ChaosExperimentRunHandler) GetExperimentRunStats(ctx context.Context, projectID string) (*model.GetExperimentRunStatsResponse, error) {
	var pipeline mongo.Pipeline
	// Match with identifiers
	matchIdentifierStage := bson.D{
		{"$match", bson.D{
			{"project_id", bson.D{{"$eq", projectID}}},
		}},
	}

	pipeline = append(pipeline, matchIdentifierStage)

	// Group and counts total experiment runs by phase
	groupByPhaseStage := bson.D{
		{
			"$group", bson.D{
				{"_id", "$phase"},
				{"count", bson.D{
					{"$sum", 1},
				}},
			},
		},
	}
	pipeline = append(pipeline, groupByPhaseStage)
	// Call aggregation on pipeline
	experimentRunCursor, err := c.chaosExperimentRunOperator.GetAggregateExperimentRuns(pipeline)
	if err != nil {
		return nil, err
	}

	var res []dbChaosExperiment.AggregatedExperimentRunStats

	if err = experimentRunCursor.All(context.Background(), &res); err != nil {
		return nil, err
	}

	resMap := map[model.ExperimentRunStatus]int{
		model.ExperimentRunStatusCompleted:  0,
		model.ExperimentRunStatusStopped:    0,
		model.ExperimentRunStatusRunning:    0,
		model.ExperimentRunStatusTerminated: 0,
		model.ExperimentRunStatusError:      0,
	}

	totalExperimentRuns := 0
	for _, phase := range res {
		resMap[model.ExperimentRunStatus(phase.Id)] = phase.Count
		totalExperimentRuns = totalExperimentRuns + phase.Count
	}

	return &model.GetExperimentRunStatsResponse{
		TotalExperimentRuns:           totalExperimentRuns,
		TotalCompletedExperimentRuns:  resMap[model.ExperimentRunStatusCompleted],
		TotalTerminatedExperimentRuns: resMap[model.ExperimentRunStatusTerminated],
		TotalRunningExperimentRuns:    resMap[model.ExperimentRunStatusRunning],
		TotalStoppedExperimentRuns:    resMap[model.ExperimentRunStatusStopped],
		TotalErroredExperimentRuns:    resMap[model.ExperimentRunStatusError],
	}, nil
}

func (c *ChaosExperimentRunHandler) ChaosExperimentRunEvent(event model.ExperimentRunRequest) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	experiment, err := c.chaosExperimentOperator.GetExperiment(ctx, bson.D{
		{"experiment_id", event.ExperimentID},
		{"is_removed", false},
	})
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Sprintf("no experiment found with experimentID: %s, experiment run discarded: %s", event.ExperimentID, event.ExperimentRunID), nil
		}
		return "", err
	}

	logFields := logrus.Fields{
		"projectID":       experiment.ProjectID,
		"experimentID":    experiment.ExperimentID,
		"experimentRunID": event.ExperimentRunID,
		"infraID":         experiment.InfraID,
	}

	logrus.WithFields(logFields).Info("new workflow event received")

	expType := experiment.ExperimentType
	probes, err := probeUtils.ParseProbesFromManifestForRuns(&expType, experiment.Revision[len(experiment.Revision)-1].ExperimentManifest)
	if err != nil {
		return "", fmt.Errorf("unable to parse probes %v", err.Error())
	}

	var (
		executionData types.ExecutionData
		exeData       []byte
	)

	// Parse and store execution data
	if event.ExecutionData != "" {
		exeData, err = base64.StdEncoding.DecodeString(event.ExecutionData)
		if err != nil {
			logrus.WithFields(logFields).Warn("Failed to decode execution data: ", err)

			//Required for backward compatibility of subscribers
			//which are not sending execution data in base64 encoded format
			//remove it once all subscribers are updated
			exeData = []byte(event.ExperimentID)
		}
		err = json.Unmarshal(exeData, &executionData)
		if err != nil {
			return "", err
		}
	}

	var workflowRunMetrics types.ExperimentRunMetrics
	phaseLower := strings.ToLower(executionData.Phase)
	isCompleted := event.Completed || strings.Contains(phaseLower, "completed") || strings.Contains(phaseLower, "succeeded") || strings.Contains(phaseLower, "failed") || strings.Contains(phaseLower, "error")
	logrus.WithFields(logFields).Infof("[Tracing] Phase='%s' (lower='%s'), event.Completed=%v, isCompleted=%v", executionData.Phase, phaseLower, event.Completed, isCompleted)
	// Resiliency Score will be calculated only if workflow execution is completed
	if isCompleted {
		workflowRunMetrics, err = c.chaosExperimentRunService.ProcessCompletedExperimentRun(executionData, event.ExperimentID, event.ExperimentRunID)
		if err != nil {
			logrus.WithFields(logFields).Errorf("failed to process completed workflow run %v", err)
			return "", err
		}

	}

	traceID := event.ExperimentRunID
	logrus.WithFields(logFields).Infof("[Tracing] Initial traceID from event.ExperimentRunID: %s", traceID)
	
	if event.NotifyID != nil && *event.NotifyID != "" {
		traceID = *event.NotifyID
		logrus.WithFields(logFields).Infof("[Tracing] Using NotifyID from event: %s", traceID)
	} else {
		logrus.WithFields(logFields).Warn("[Tracing] No NotifyID in event, attempting database lookup")
		// Fallback: try to get NotifyID from database if event doesn't have it
		experimentRun, dbErr := c.chaosExperimentRunOperator.GetExperimentRun(bson.D{
			{"experiment_run_id", event.ExperimentRunID},
		})
		if dbErr == nil && experimentRun.ExperimentRunID != "" {
			if experimentRun.NotifyID != nil && *experimentRun.NotifyID != "" {
				traceID = *experimentRun.NotifyID
				logrus.WithFields(logFields).Infof("[Tracing] Found NotifyID from database: %s", traceID)
			} else {
				logrus.WithFields(logFields).Warn("[Tracing] Database query succeeded but NotifyID is empty")
			}
		} else {
			logrus.WithFields(logFields).Warnf("[Tracing] Database query failed or returned nil: %v", dbErr)
		}
	}
	logrus.WithFields(logFields).Infof("[Tracing] Final traceID to use: %s", traceID)

	// G2 fix: stamp experiment.id and experiment.run_id onto the long-running experiment-run span
	if observability.OTELTracerEnabled() {
		observability.SetExperimentSpanAttributes(traceID,
			attribute.String("experiment.id", event.ExperimentID),
			attribute.String("experiment.run_id", event.ExperimentRunID),
		)
	}

	var metricsPtr *types.ExperimentRunMetrics
	if isCompleted {
		metricsPtr = &workflowRunMetrics
	}

	// OTEL Hook 2: End per-fault child spans with verdicts from execution data
	if observability.OTELTracerEnabled() && isCompleted {
		for _, node := range executionData.Nodes {
			if node.Type == "ChaosEngine" && node.ChaosExp != nil {
				faultName := node.ChaosExp.ExperimentName
				if faultName == "" {
					faultName = node.ChaosExp.EngineName
				}

				verdictAttrs := []attribute.KeyValue{
					attribute.String("experiment.id", event.ExperimentID),
					attribute.String("experiment.run_id", event.ExperimentRunID),
					attribute.String("fault.verdict", node.ChaosExp.ExperimentVerdict),
					attribute.String("fault.probe_success_pct", node.ChaosExp.ProbeSuccessPercentage),
					attribute.String("fault.status", node.ChaosExp.ExperimentStatus),
					attribute.String("fault.engine_name", node.ChaosExp.EngineName),
					attribute.String("fault.namespace", node.ChaosExp.Namespace),
					attribute.String("fault.started_at", node.StartedAt),
					attribute.String("fault.finished_at", node.FinishedAt),
					attribute.String("fault.node_phase", node.Phase),
				}

				if node.ChaosExp.FailStep != "" {
					verdictAttrs = append(verdictAttrs, attribute.String("fault.fail_step", node.ChaosExp.FailStep))
				}

				// End the per-fault child span with verdict attributes
				observability.EndFaultSpan(traceID, faultName, verdictAttrs...)

				logrus.WithFields(logFields).Infof("[OTEL] Ended per-fault span: fault=%s verdict=%s probe%%=%s",
					faultName, node.ChaosExp.ExperimentVerdict, node.ChaosExp.ProbeSuccessPercentage)
			}
		}
	}

	// Also submit per-fault observations via Langfuse REST for Langfuse-native consumers
	if !observability.OTELTracerEnabled() && isCompleted {
		tracer := observability.GetLangfuseTracer()
		if tracer.IsEnabled() {
			for _, node := range executionData.Nodes {
				if node.Type == "ChaosEngine" && node.ChaosExp != nil {
					faultName := node.ChaosExp.ExperimentName
					if faultName == "" {
						faultName = node.ChaosExp.EngineName
					}
					_ = tracer.TraceExperimentObservation(ctx, &observability.ExperimentObservationDetails{
						TraceID:   traceID,
						Name:      fmt.Sprintf("fault-verdict: %s", faultName),
						Type:      "SPAN",
						StartTime: node.StartedAt,
						EndTime:   node.FinishedAt,
						Input: map[string]interface{}{
							"faultName":  faultName,
							"engineName": node.ChaosExp.EngineName,
							"namespace":  node.ChaosExp.Namespace,
						},
						Output: map[string]interface{}{
							"verdict":             node.ChaosExp.ExperimentVerdict,
							"probeSuccessPct":     node.ChaosExp.ProbeSuccessPercentage,
							"experimentStatus":    node.ChaosExp.ExperimentStatus,
							"failStep":            node.ChaosExp.FailStep,
						},
						Metadata: map[string]interface{}{
							"nodePhase": node.Phase,
							"type":      "fault-verdict",
						},
					})
				}
			}
		}
	}

	traceExperimentObservation(ctx, traceID, event, executionData, metricsPtr)
	if isCompleted {
		scoreExperimentRun(ctx, traceID, metricsPtr, executionData.Phase)
	}

	//TODO check for mongo transaction
	var (
		wc      = writeconcern.New(writeconcern.WMajority())
		rc      = readconcern.Snapshot()
		txnOpts = options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)
	)

	session, err := mongodb.MgoClient.StartSession()
	if err != nil {
		logrus.WithFields(logFields).Errorf("failed to start mongo session %v", err)
		return "", err
	}
	//
	var (
		isRemoved   = false
		currentTime = time.Now()
	)

	err = mongo.WithSession(ctx, session, func(sessionContext mongo.SessionContext) error {
		if err = session.StartTransaction(txnOpts); err != nil {
			logrus.WithFields(logFields).Errorf("failed to start mongo session transaction %v", err)
			return err
		}

		query := bson.D{
			{"experiment_id", event.ExperimentID},
			{"experiment_run_id", event.ExperimentRunID},
		}

		if event.NotifyID != nil {
			query = bson.D{
				{"experiment_id", event.ExperimentID},
				{"notify_id", event.NotifyID},
			}
		}

		experimentRunCount, err := c.chaosExperimentRunOperator.CountExperimentRuns(sessionContext, query)
		if err != nil {
			return err
		}
		updatedBy, err := base64.RawURLEncoding.DecodeString(event.UpdatedBy)
		if err != nil {
			logrus.Fatalf("Failed to parse updated by field %v", err)
		}
		expRunDetail := []dbChaosExperiment.ExperimentRunDetail{
			{
				Phase:           executionData.Phase,
				ResiliencyScore: &workflowRunMetrics.ResiliencyScore,
				ExperimentRunID: event.ExperimentRunID,
				Completed:       false,
				RunSequence:     experiment.TotalExperimentRuns + 1,
				Audit: mongodb.Audit{
					IsRemoved: false,
					CreatedAt: time.Now().UnixMilli(),
					UpdatedAt: time.Now().UnixMilli(),
					UpdatedBy: mongodb.UserDetailResponse{
						Username: string(updatedBy),
					},
				},
			},
		}
		if experimentRunCount == 0 {
			filter := bson.D{
				{"experiment_id", event.ExperimentID},
			}
			update := bson.D{
				{
					"$set", bson.D{
						{"updated_at", time.Now().UnixMilli()},
						{"total_experiment_runs", experiment.TotalExperimentRuns + 1},
					},
				},
				{
					"$push", bson.D{
						{"recent_experiment_run_details", bson.D{
							{"$each", expRunDetail},
							{"$position", 0},
							{"$slice", 10},
						}},
					},
				},
			}

			err = c.chaosExperimentOperator.UpdateChaosExperiment(sessionContext, filter, update)
			if err != nil {
				logrus.WithError(err).Error("Failed to update experiment collection")
				return err
			}
		} else if experimentRunCount > 0 {
			filter := bson.D{
				{"experiment_id", event.ExperimentID},
				{"recent_experiment_run_details.experiment_run_id", event.ExperimentRunID},
				{"recent_experiment_run_details.completed", false},
			}
			if event.NotifyID != nil {
				filter = bson.D{
					{"experiment_id", event.ExperimentID},
					{"recent_experiment_run_details.completed", false},
					{"recent_experiment_run_details.notify_id", event.NotifyID},
				}
			}
			updatedByModel := mongodb.UserDetailResponse{
				Username: string(updatedBy),
			}
			update := bson.D{
				{
					"$set", bson.D{
						{"recent_experiment_run_details.$.phase", executionData.Phase},
						{"recent_experiment_run_details.$.completed", event.Completed},
						{"recent_experiment_run_details.$.experiment_run_id", event.ExperimentRunID},
						{"recent_experiment_run_details.$.probes", probes},
						{"recent_experiment_run_details.$.resiliency_score", workflowRunMetrics.ResiliencyScore},
						{"recent_experiment_run_details.$.updated_at", currentTime.UnixMilli()},
						{"recent_experiment_run_details.$.updated_by", updatedByModel},
					},
				},
			}

			err = c.chaosExperimentOperator.UpdateChaosExperiment(sessionContext, filter, update)
			if err != nil {
				logrus.WithError(err).Error("Failed to update experiment collection")
				return err
			}
		}

		count, err := c.chaosExperimentRunOperator.UpdateExperimentRun(sessionContext, dbChaosExperimentRun.ChaosExperimentRun{
			InfraID:         event.InfraID.InfraID,
			ProjectID:       experiment.ProjectID,
			ExperimentRunID: event.ExperimentRunID,
			ExperimentID:    event.ExperimentID,
			NotifyID:        event.NotifyID,
			Phase:           executionData.Phase,
			ResiliencyScore: &workflowRunMetrics.ResiliencyScore,
			FaultsPassed:    &workflowRunMetrics.ExperimentsPassed,
			FaultsFailed:    &workflowRunMetrics.ExperimentsFailed,
			FaultsAwaited:   &workflowRunMetrics.ExperimentsAwaited,
			FaultsStopped:   &workflowRunMetrics.ExperimentsStopped,
			FaultsNA:        &workflowRunMetrics.ExperimentsNA,
			TotalFaults:     &workflowRunMetrics.TotalExperiments,
			ExecutionData:   string(exeData),
			RevisionID:      event.RevisionID,
			Completed:       event.Completed,
			Probes:          probes,
			RunSequence:     experiment.TotalExperimentRuns + 1,
			Audit: mongodb.Audit{
				IsRemoved: isRemoved,
				UpdatedAt: currentTime.UnixMilli(),
				UpdatedBy: mongodb.UserDetailResponse{
					Username: string(updatedBy),
				},
				CreatedBy: mongodb.UserDetailResponse{
					Username: string(updatedBy),
				},
			},
		})
		if err != nil {
			logrus.WithFields(logFields).Errorf("failed to update workflow run %v", err)
			return err
		}

		if count == 0 {
			err := fmt.Sprintf("experiment run has been discarded due the duplicate event, workflowId: %s, workflowRunId: %s", event.ExperimentID, event.ExperimentRunID)
			return errors.New(err)
		}

		if err = session.CommitTransaction(sessionContext); err != nil {
			logrus.WithFields(logFields).Errorf("failed to commit session transaction %v", err)
			return err
		}
		return nil
	})

	if err != nil {
		if abortErr := session.AbortTransaction(ctx); abortErr != nil {
			logrus.WithFields(logFields).Errorf("failed to abort session transaction %v", err)
			return "", abortErr
		}
		return "", err
	}

	session.EndSession(ctx)

	return fmt.Sprintf("Experiment run received for for ExperimentID: %s, ExperimentRunID: %s", event.ExperimentID, event.ExperimentRunID), nil
}
