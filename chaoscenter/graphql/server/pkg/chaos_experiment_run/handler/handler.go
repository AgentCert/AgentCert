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

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/agent_registry"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/authorization"

	probeUtils "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/probe/utils"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaos_infrastructure"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/gitops"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/observability"
	ops "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaos_experiment/ops"
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
	"github.com/tidwall/sjson"
	corev1 "k8s.io/api/core/v1"
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
	agentRegistryOperator      agent_registry.Operator
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
	agentRegOp agent_registry.Operator,
) *ChaosExperimentRunHandler {
	return &ChaosExperimentRunHandler{
		chaosExperimentRunService:  chaosExperimentRunService,
		infrastructureService:      infrastructureService,
		gitOpsService:              gitOpsService,
		chaosExperimentOperator:    chaosExperimentOperator,
		chaosExperimentRunOperator: chaosExperimentRunOperator,
		probeService:               probeService,
		mongodbOperator:            mongodbOperator,
		agentRegistryOperator:      agentRegOp,
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

		// Phase 1 dual matching: check annotation first, fall back to name-based.
		// Once all manifests carry the annotation (after Phase 2 of Item #1),
		// the name-based fallback can be removed.
		isInstallTemplate := false
		if templates[i].Metadata.Annotations != nil {
			if installType, ok := templates[i].Metadata.Annotations["agentcert.io/install-type"]; ok {
				isInstallTemplate = installType == "application" || installType == "agent"
			}
		}
		if !isInstallTemplate {
			// Fallback: name-based matching for existing manifests without annotations
			if templates[i].Name != "install-application" && templates[i].Name != "install-agent" {
				continue
			}
			logrus.WithField("template", templates[i].Name).Debug("[normalizeInstallTemplates] matched by name (no annotation) — legacy manifest")
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
	const defaultValue = "900"

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

func applyPreCleanupWaitPatchToWorkflowSpec(spec *v1alpha1.WorkflowSpec) {
	if spec == nil || len(spec.Templates) == 0 {
		return
	}

	waitRaw := strings.TrimSpace(utils.Config.PreCleanupWaitSeconds)
	if waitRaw == "" {
		waitRaw = strings.TrimSpace(os.Getenv("PRE_CLEANUP_WAIT_SECONDS"))
	}
	if waitRaw == "" {
		waitRaw = "0"
	}

	waitSec, err := strconv.Atoi(waitRaw)
	if err != nil || waitSec < 0 {
		waitSec = 0
	}

	entrypoint := spec.Entrypoint
	if entrypoint == "" {
		return
	}

	var rootTemplate *v1alpha1.Template
	for i := range spec.Templates {
		if spec.Templates[i].Name == entrypoint {
			rootTemplate = &spec.Templates[i]
			break
		}
	}
	if rootTemplate == nil || len(rootTemplate.Steps) == 0 {
		return
	}

	waitTemplateName := "dynamic-pre-cleanup-wait"
	for _, t := range spec.Templates {
		if t.Name == waitTemplateName {
			return
		}
	}

	insertIdx := -1
	for i, group := range rootTemplate.Steps {
		for _, step := range group.Steps {
			name := strings.ToLower(strings.TrimSpace(step.Name))
			if name == "cleanup-chaos-resources" || strings.Contains(name, "cleanup-chaos-resources") {
				insertIdx = i
				break
			}
		}
		if insertIdx >= 0 {
			break
		}
	}

	if insertIdx < 0 {
		for i, group := range rootTemplate.Steps {
			for _, step := range group.Steps {
				name := strings.ToLower(strings.TrimSpace(step.Name))
				if strings.HasPrefix(name, "cleanup-") || (strings.Contains(name, "cleanup") && strings.Contains(name, "resource")) {
					insertIdx = i
					break
				}
			}
			if insertIdx >= 0 {
				break
			}
		}
	}

	if insertIdx < 0 {
		insertIdx = len(rootTemplate.Steps) - 1
		if insertIdx < 0 {
			return
		}
	}

	waitTpl := v1alpha1.Template{
		Name: waitTemplateName,
		Container: &corev1.Container{
			Image:   "busybox:1.36",
			Command: []string{"sh", "-c"},
			Args:    []string{fmt.Sprintf("echo '[pre-cleanup-wait] sleeping for %d seconds'; sleep %d; echo '[pre-cleanup-wait] done'", waitSec, waitSec)},
		},
	}
	spec.Templates = append(spec.Templates, waitTpl)

	waitStepGroup := v1alpha1.ParallelSteps{
		Steps: []v1alpha1.WorkflowStep{{
			Name:     waitTemplateName,
			Template: waitTemplateName,
		}},
	}

	newSteps := make([]v1alpha1.ParallelSteps, 0, len(rootTemplate.Steps)+1)
	for i, group := range rootTemplate.Steps {
		if i == insertIdx {
			newSteps = append(newSteps, waitStepGroup)
		}
		newSteps = append(newSteps, group)
	}
	rootTemplate.Steps = newSteps

	logrus.WithFields(logrus.Fields{
		"entrypoint":          entrypoint,
		"wait_seconds":        waitSec,
		"insert_before_index": insertIdx,
	}).Info("[Pre-Cleanup Wait Patch] Injected dynamic pre-cleanup wait step in run handler")
}

// applyUninstallAllPatchToWorkflowSpec appends a final uninstall-all step that runs
// helm uninstall for the agent and app releases after all chaos steps complete.
// Release names are resolved dynamically via Argo workflow parameters at runtime:
//   - agent: {{workflow.parameters.agentFolder}}
//   - app:   {{workflow.parameters.appNamespace}}  (folder == release == namespace by convention)
func applyUninstallAllPatchToWorkflowSpec(spec *v1alpha1.WorkflowSpec) {
	if spec == nil || len(spec.Templates) == 0 {
		return
	}

	// Only inject if an install-agent template is present.
	hasInstallAgent := false
	for _, t := range spec.Templates {
		if t.Container == nil {
			continue
		}
		if t.Name == "install-agent" || strings.Contains(strings.TrimSpace(t.Container.Image), "agentcert-install-agent") {
			hasInstallAgent = true
			break
		}
	}
	if !hasInstallAgent {
		return
	}

	// Enable Argo podGC so completed executor pods in litmus-exp are deleted automatically.
	spec.PodGC = &v1alpha1.PodGC{Strategy: v1alpha1.PodGCOnWorkflowCompletion}

	entrypoint := spec.Entrypoint
	if entrypoint == "" {
		return
	}

	var rootTemplate *v1alpha1.Template
	for i := range spec.Templates {
		if spec.Templates[i].Name == entrypoint {
			rootTemplate = &spec.Templates[i]
			break
		}
	}
	if rootTemplate == nil || len(rootTemplate.Steps) == 0 {
		return
	}

	uninstallTemplateName := "uninstall-all"
	for _, t := range spec.Templates {
		if t.Name == uninstallTemplateName {
			return
		}
	}

	uninstallImage := strings.TrimSpace(utils.Config.InstallAgentImage)
	if uninstallImage == "" {
		uninstallImage = strings.TrimSpace(os.Getenv("INSTALL_AGENT_IMAGE"))
	}
	if uninstallImage == "" {
		uninstallImage = "agentcert/agentcert-install-agent:latest"
	}

	uninstallScript := `NAMESPACE="{{workflow.parameters.appNamespace}}"
AGENT_RELEASE="{{workflow.parameters.agentFolder}}"
APP_RELEASE="${NAMESPACE}"
echo "[uninstall-all] Cleaning ChaosEngine and ChaosResult resources in ${NAMESPACE}"
kubectl delete chaosengines.litmuschaos.io --all -n "${NAMESPACE}" --ignore-not-found 2>&1 || true
kubectl delete chaosresults.litmuschaos.io --all -n "${NAMESPACE}" --ignore-not-found 2>&1 || true
echo "[uninstall-all] Uninstalling agent release: ${AGENT_RELEASE} (ns: ${NAMESPACE})"
helm uninstall "${AGENT_RELEASE}" -n "${NAMESPACE}" --ignore-not-found 2>&1 || true
echo "[uninstall-all] Uninstalling app release: ${APP_RELEASE} (ns: ${NAMESPACE})"
helm uninstall "${APP_RELEASE}" -n "${NAMESPACE}" --ignore-not-found 2>&1 || true
echo "[uninstall-all] Done"`

	uninstallTpl := v1alpha1.Template{
		Name: uninstallTemplateName,
		Container: &corev1.Container{
			Image:   uninstallImage,
			Command: []string{"sh", "-c"},
			Args:    []string{uninstallScript},
		},
	}
	spec.Templates = append(spec.Templates, uninstallTpl)

	spec.Templates[func() int {
		for i := range spec.Templates {
			if spec.Templates[i].Name == entrypoint {
				return i
			}
		}
		return 0
	}()].Steps = append(rootTemplate.Steps, v1alpha1.ParallelSteps{
		Steps: []v1alpha1.WorkflowStep{{
			Name:     uninstallTemplateName,
			Template: uninstallTemplateName,
		}},
	})

	logrus.WithFields(logrus.Fields{
		"entrypoint": entrypoint,
		"image":      uninstallImage,
	}).Info("[Uninstall All Patch] Appended dynamic uninstall-all step in run handler")
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
func traceExperimentExecution(ctx context.Context, notifyID string, experimentID string, experimentName string, experimentType string, infra dbChaosInfra.ChaosInfra, projectID string, traceAgentID string, traceAgentName string, traceAgentPlatform string) error {
	namespace := ""
	if infra.InfraNamespace != nil {
		namespace = *infra.InfraNamespace
	}
	serviceAccount := ""
	if infra.ServiceAccount != nil {
		serviceAccount = *infra.ServiceAccount
	}

	// OTEL path: emit instant start span + create long-running end span
	if observability.OTELTracerEnabled() {
		startAttrs := []attribute.KeyValue{
			attribute.String("experiment.id", experimentID),
			attribute.String("experiment.name", experimentName),
			attribute.String("experiment.type", experimentType),
			attribute.String("experiment.fault_name", "chaos-workflow"),
			attribute.String("experiment.session_id", notifyID),
			attribute.String("experiment.run_key", notifyID),
			attribute.String("infra.id", infra.InfraID),
			attribute.String("infra.name", infra.Name),
			attribute.String("infra.platform_name", infra.PlatformName),
			attribute.String("project.id", projectID),
			attribute.String("infra.namespace", namespace),
			attribute.String("infra.service_account", serviceAccount),
			attribute.String("experiment.phase", "injection"),
			attribute.String("experiment.priority", "high"),
			attribute.String("agent.id", traceAgentID),
			attribute.String("agent.name", traceAgentName),
			attribute.String("agent.platform_name", traceAgentPlatform),
		}
		// Stamp SLA contract at span creation so the certifier always finds it,
		// regardless of which scoring code path completes the run later.
		startAttrs = append(startAttrs, observability.LoadSLAFromEnv().Attributes()...)

		// Long-running root span — ended later by scoreExperimentRun, appears LAST
		spanCtx, _ := observability.StartExperimentSpan(ctx, notifyID, startAttrs...)
		logrus.Infof("[OTEL] Started experiment-run span: traceID=%s experiment=%s", notifyID, experimentName)

		// Instant child span — shares the same traceID as the root span
		observability.EmitExperimentStartSpan(spanCtx, startAttrs...)
		logrus.Infof("[OTEL] Emitted experiment-triggered span: traceID=%s experiment=%s", notifyID, experimentName)

		// Upsert Langfuse trace metadata (name, userId, sessionId, agentid) via REST
		// alongside OTEL spans. OTEL alone cannot set trace-level metadata in Langfuse.
		// Two upserts are needed:
		//   1. UUID form (notifyID with hyphens) — covers the LLM generation trace from LiteLLM
		//   2. Hex form (notifyID without hyphens) — covers the OTEL spans trace (Langfuse stores OTEL
		//      traces using the raw 32-char hex trace ID, which differs from the UUID string)
		if lft := observability.GetLangfuseTracer(); lft.IsEnabled() {
			details := &observability.ExperimentExecutionDetails{
				TraceID:             notifyID,
				ExperimentID:        experimentID,
				ExperimentName:      experimentName,
				ExperimentType:      experimentType,
				FaultName:           "chaos-workflow",
				SessionID:           notifyID,
				AgentID:             traceAgentID,
				AgentName:           traceAgentName,
				AgentPlatform:       traceAgentPlatform,
				AgentVersion:        infra.Version,
				AgentServiceAccount: serviceAccount,
				ProjectID:           projectID,
				Namespace:           namespace,
				Phase:               "injection",
				Priority:            "high",
			}
			// Upsert 1: UUID trace (LLM generations)
			_ = lft.TraceExperimentExecution(ctx, details)
			// Upsert 2: hex trace (OTEL spans) — same content, hex trace ID
			hexTraceID := strings.ReplaceAll(notifyID, "-", "")
			if len(hexTraceID) == 32 {
				hexDetails := *details
				hexDetails.TraceID = hexTraceID
				_ = lft.TraceExperimentExecution(ctx, &hexDetails)
			}
		}
		return nil
	}

	// Langfuse REST fallback
	tracer := observability.GetLangfuseTracer()
	return tracer.TraceExperimentExecution(ctx, &observability.ExperimentExecutionDetails{
		TraceID:             notifyID,
		ExperimentID:        experimentID,
		ExperimentName:      experimentName,
		ExperimentType:      experimentType,
		FaultName:           "chaos-workflow",
		SessionID:           notifyID,
		AgentID:             traceAgentID,
		AgentName:           traceAgentName,
		AgentPlatform:       traceAgentPlatform,
		AgentVersion:        infra.Version,
		AgentServiceAccount: serviceAccount,
		ProjectID:           projectID,
		Namespace:           namespace,
		Phase:               "injection",
		Priority:            "high",
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

func isTerminalWorkflowNodePhase(phase string) bool {
	phase = strings.ToLower(strings.TrimSpace(phase))
	switch phase {
	case "succeeded", "failed", "error", "completed", "skipped", "omitted":
		return true
	default:
		return false
	}
}

func syncWorkflowNodeSpans(ctx context.Context, traceID string, event model.ExperimentRunRequest, executionData types.ExecutionData, agentOp agent_registry.Operator) {
	if traceID == "" {
		return
	}

	for nodeID, node := range executionData.Nodes {
		if node.Name == "" {
			continue
		}
		// Skip Argo internal StepGroup nodes ([0], [1], etc.) — they are
		// workflow step-group wrappers, not real executable steps.
		if node.Type == "StepGroup" {
			continue
		}

		stepAttrs := []attribute.KeyValue{
			attribute.String("experiment.id", event.ExperimentID),
			attribute.String("experiment.run_id", event.ExperimentRunID),
			attribute.String("experiment.name", event.ExperimentName),
			attribute.String("experiment.type", executionData.ExperimentType),
			attribute.String("workflow.notify_id", traceID),
			attribute.String("workflow.node.id", nodeID),
			attribute.String("workflow.node.name", node.Name),
			attribute.String("workflow.node.phase", node.Phase),
			attribute.String("workflow.node.type", node.Type),
			attribute.String("workflow.node.message", node.Message),
			attribute.String("workflow.phase", executionData.Phase),
			attribute.String("workflow.event_type", executionData.EventType),
		}

		if executionData.Namespace != "" {
			stepAttrs = append(stepAttrs, attribute.String("workflow.namespace", executionData.Namespace))
		}
		if executionData.Name != "" {
			stepAttrs = append(stepAttrs, attribute.String("workflow.name", executionData.Name))
		}
		if node.StartedAt != "" {
			stepAttrs = append(stepAttrs, attribute.String("workflow.node.started_at", node.StartedAt))
		}
		if node.FinishedAt != "" {
			stepAttrs = append(stepAttrs, attribute.String("workflow.node.finished_at", node.FinishedAt))
		}
		if len(node.Children) > 0 {
			stepAttrs = append(stepAttrs, attribute.Int("workflow.node.children", len(node.Children)))
		}
		if node.ChaosExp != nil {
			if node.ChaosExp.ExperimentName != "" {
				stepAttrs = append(stepAttrs, attribute.String("fault.name", node.ChaosExp.ExperimentName))
			}
			if node.ChaosExp.EngineName != "" {
				stepAttrs = append(stepAttrs, attribute.String("fault.engine_name", node.ChaosExp.EngineName))
			}
			if node.ChaosExp.Namespace != "" {
				stepAttrs = append(stepAttrs, attribute.String("fault.namespace", node.ChaosExp.Namespace))
			}
			if node.ChaosExp.ExperimentStatus != "" {
				stepAttrs = append(stepAttrs, attribute.String("fault.status", node.ChaosExp.ExperimentStatus))
			}
			if node.ChaosExp.ExperimentVerdict != "" {
				stepAttrs = append(stepAttrs, attribute.String("fault.verdict", node.ChaosExp.ExperimentVerdict))
			}
		}

		terminal := node.FinishedAt != "" || isTerminalWorkflowNodePhase(node.Phase)
		// Emit a child span for every workflow node (install-application,
		// install-agent, chaos faults, cleanup steps, etc.) so the full
		// experiment lifecycle is visible in Langfuse / OTEL.
		if observability.OTELTracerEnabled() {
			observability.UpsertWorkflowNodeSpan(traceID, nodeID, node.Name, terminal, stepAttrs...)
		}

		// When install-agent completes successfully, the agent has just registered
		// itself in MongoDB with a fresh agent_id. Look it up and back-fill the
		// agent.id / agent.name attributes on the root experiment-run span so the
		// trace reflects the actual deployed agent identity.
		if terminal &&
			strings.ToLower(node.Phase) == "succeeded" &&
			strings.Contains(strings.ToLower(node.Name), "install-agent") &&
			agentOp != nil &&
			executionData.Namespace != "" {
			if freshAgent, lookupErr := agentOp.GetAgentByNamespace(ctx, executionData.Namespace); lookupErr == nil && freshAgent != nil {
				updateAttrs := []attribute.KeyValue{
					attribute.String("agent.id", freshAgent.AgentID),
					attribute.String("agent.name", freshAgent.Name),
				}
				if freshAgent.Vendor != "" {
					updateAttrs = append(updateAttrs, attribute.String("agent.platform_name", freshAgent.Vendor))
				}
				observability.SetExperimentSpanAttributes(traceID, updateAttrs...)
				logrus.Infof("[OTEL] Updated agent.id on experiment span after install-agent: agentID=%s traceID=%s", freshAgent.AgentID, traceID)
			}
		}
	}
}

// traceExperimentObservation logs continuous workflow events.
// When OTEL is enabled, adds events to the active experiment span.
// Falls back to Langfuse REST when OTEL is not configured.
func traceExperimentObservation(ctx context.Context, traceID string, event model.ExperimentRunRequest, executionData types.ExecutionData, metrics *types.ExperimentRunMetrics, agentOp agent_registry.Operator) {
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

		// Upsert a child span for every workflow node on every event so spans
		// open as soon as the node starts (not just at completion).
		syncWorkflowNodeSpans(ctx, traceID, event, executionData, agentOp)

		// Also update the span's top-level attributes with latest phase
		observability.SetExperimentSpanAttributes(traceID,
			attribute.String("experiment.phase", executionData.Phase),
			attribute.String("experiment.event_type", executionData.EventType),
			attribute.String("experiment.type", executionData.ExperimentType),
			attribute.String("experiment.run_id", event.ExperimentRunID),
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

	langfuseTraceID := traceID
	if observability.OTELTracerEnabled() {
		langfuseTraceID = observability.LinkedLangfuseTraceID(traceID)
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
	if !tracer.IsEnabled() || langfuseTraceID == "" {
		return
	}

	// Score 1: Resiliency Score
	_ = tracer.ScoreExperimentExecution(ctx, &observability.ExperimentScoreDetails{
		TraceID: langfuseTraceID,
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
		TraceID: langfuseTraceID,
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
		TraceID: langfuseTraceID,
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
		TraceID: langfuseTraceID,
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
		TraceID: langfuseTraceID,
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
		TraceID: langfuseTraceID,
		Name:    "experiments_na_percentage",
		Value:   naScore,
		Comment: fmt.Sprintf("Percentage of experiments not applicable: %d/%d", metrics.ExperimentsNA, metrics.TotalExperiments),
		Source:  "API",
	})

	// Score 7: Total Experiments Count
	_ = tracer.ScoreExperimentExecution(ctx, &observability.ExperimentScoreDetails{
		TraceID: langfuseTraceID,
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

	// Check if this is a multi-run experiment and block concurrent runs
	if len(workflow.Revision) > 0 {
		manifest := workflow.Revision[0].ExperimentManifest
		multiRunEnabled := gjson.Get(manifest, "metadata.annotations.litmuschaos\\.io/multiRunEnabled").String()
		
		if multiRunEnabled == "true" {
			// Query for any running experiment runs for this experiment
			runningRuns, err := dbChaosExperimentRun.NewChaosExperimentRunOperator(c.mongodbOperator).GetExperimentRuns(bson.D{
				{"experiment_id", workflow.ExperimentID},
				{"is_removed", false},
				{"completed", false},
				{"phase", string(model.ExperimentRunStatusRunning)},
			})
			if err == nil && len(runningRuns) > 0 {
				return nil, errors.New("multi-run experiment already has a running instance. Please wait for it to complete before starting another run")
			}
		}
	}

	var (
		workflowManifest v1alpha1.Workflow
	)

	currentTime := time.Now().UnixMilli()
	notifyID = uuid.New().String()

	traceAgentID := infra.InfraID
	traceAgentName := infra.Name
	traceAgentPlatform := infra.PlatformName
	if c.agentRegistryOperator != nil && infra.InfraNamespace != nil {
		agent, agentErr := c.agentRegistryOperator.GetAgentByNamespace(ctx, *infra.InfraNamespace)
		if agentErr != nil {
			logrus.WithError(agentErr).Warn("failed to lookup agent for observability trace identity")
		} else if agent != nil {
			if strings.TrimSpace(agent.AgentID) != "" {
				traceAgentID = agent.AgentID
			}
			if strings.TrimSpace(agent.Name) != "" {
				traceAgentName = agent.Name
			}
			if strings.TrimSpace(agent.Vendor) != "" {
				traceAgentPlatform = agent.Vendor
			}
		}
	}

	// Trace experiment execution start to observability backend
	traceExperimentExecution(ctx, notifyID, workflow.ExperimentID, workflow.Name, string(workflow.ExperimentType), infra, projectID, traceAgentID, traceAgentName, traceAgentPlatform)

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
	applyPreCleanupWaitPatchToWorkflowSpec(&workflowManifest.Spec)
	applyUninstallAllPatchToWorkflowSpec(&workflowManifest.Spec)

	// Emit "fault: <name>" SPAN observations to Langfuse for certifier fault bucketing.
	// Also emits a preceding "experiment_context" SPAN carrying agent/experiment identity
	// so the certifier's chronological metadata scan finds it before any fault span.
	// This is a best-effort fire-and-forget: failures are logged but do not block the run.
	go func(tid string, templates []v1alpha1.Template, expCtx observability.ExperimentContextForTrace) {
		faultDetails := ops.ExtractChaosEngineFaultDetails(templates)
		if len(faultDetails) > 0 {
			faultNames := make([]string, 0, len(faultDetails))
			for _, fd := range faultDetails {
				faultNames = append(faultNames, fd.Name)
			}
			groundTruth := ops.LoadFaultGroundTruthsDecoded(faultNames)
			if groundTruth == nil {
				groundTruth = make(map[string]interface{})
			}
			lft := observability.GetLangfuseTracer()
			lft.EmitFaultSpansForTrace(context.Background(), tid, faultDetails, groundTruth, expCtx)
		}
	}(notifyID, workflowManifest.Spec.Templates, observability.ExperimentContextForTrace{
		AgentID:        traceAgentID,
		AgentName:      traceAgentName,
		AgentPlatform:  traceAgentPlatform,
		AgentVersion:   infra.Version,
		ExperimentID:   workflow.ExperimentID,
		ExperimentName: workflow.Name,
		Namespace: func() string {
			if infra.InfraNamespace != nil {
				return *infra.InfraNamespace
			}
			return ""
		}(),
	})

	// Inject agentId as a workflow-level parameter for re-runs.
	// Always ensure the parameter exists (even as empty string) so that
	// {{workflow.parameters.agentId}} is resolvable by Argo.
	if c.agentRegistryOperator != nil {
		agentIDStr := ""
		if infra.InfraNamespace != nil {
			agentNS := ops.ExtractInstallAgentNamespace(workflowManifest.Spec.Templates)
			if agentNS == "" {
				agentNS = *infra.InfraNamespace
			}
			if agent, agentErr := c.agentRegistryOperator.GetAgentByNamespace(ctx, agentNS); agentErr == nil && agent != nil {
				agentIDStr = agent.AgentID
				logrus.WithField("agentId", agentIDStr).Info("resolved agentId from registry (re-run)")
			} else {
				logrus.WithField("namespace", agentNS).Info("no agent record found for re-run; agentId will be empty")
			}
		}
		found := false
		for i, p := range workflowManifest.Spec.Arguments.Parameters {
			if p.Name == "agentId" {
				workflowManifest.Spec.Arguments.Parameters[i].Value = v1alpha1.AnyStringPtr(agentIDStr)
				found = true
				break
			}
		}
		if !found {
			workflowManifest.Spec.Arguments.Parameters = append(workflowManifest.Spec.Arguments.Parameters, v1alpha1.Parameter{
				Name:  "agentId",
				Value: v1alpha1.AnyStringPtr(agentIDStr),
			})
		}
		logrus.WithField("agentId", agentIDStr).Info("injected agentId workflow parameter (re-run)")
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

			// OTEL Hook 1: Create a per-fault child span with config details.
			// Real fault names are always emitted on these spans:
			//   - the agent (LLM) never reads OTEL output, only MCP tool calls
			//     (sanitised by gateway.py:_sanitize_leakage_terms before any
			//     LLM input is built);
			//   - the Argo workflow step name already carries the fault name
			//     publicly, so aliasing only the OTEL span gives no privacy;
			//   - the certifier needs the real name to attach the correct
			//     ground_truth.yaml when scoring.
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

	// Get username from auth token or fall back to experiment's UpdatedBy username for system-triggered runs (e.g., multi-run)
	var username string
	if tkn, ok := ctx.Value(authorization.AuthKey).(string); ok && tkn != "" {
		username, err = authorization.GetUsername(tkn)
		if err != nil {
			return nil, err
		}
	} else {
		// System-triggered run (e.g., multi-run): use experiment's last updater or default to "system"
		if workflow.Audit.UpdatedBy.Username != "" {
			username = workflow.Audit.UpdatedBy.Username
		} else if workflow.Audit.CreatedBy.Username != "" {
			username = workflow.Audit.CreatedBy.Username
		} else {
			username = "system"
		}
		logrus.Infof("[Multi-Run] Using username '%s' for system-triggered run", username)
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
	applyPreCleanupWaitPatchToWorkflowSpec(&cronExperimentManifest.Spec.WorkflowSpec)
	applyUninstallAllPatchToWorkflowSpec(&cronExperimentManifest.Spec.WorkflowSpec)

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

	traceID := strings.TrimSpace(event.ExperimentRunID)
	notifyID := ""
	if event.NotifyID != nil {
		notifyID = strings.TrimSpace(*event.NotifyID)
	}
	if notifyID == "" && event.ExperimentRunID != "" {
		// Fallback: recover notifyID from DB for older/partial event payloads.
		experimentRun, dbErr := c.chaosExperimentRunOperator.GetExperimentRun(bson.D{{"experiment_run_id", event.ExperimentRunID}})
		if dbErr == nil && experimentRun.ExperimentRunID != "" && experimentRun.NotifyID != nil {
			notifyID = strings.TrimSpace(*experimentRun.NotifyID)
		}
	}
	if notifyID != "" {
		// Canonical key: notifyID so workflow observations and LLM generations
		// stitch under the same parent trace from experiment trigger onward.
		traceID = notifyID
	}

	if traceID == "" {
		logrus.WithFields(logFields).Warn("[Tracing] Missing both experimentRunID and notifyID for trace key")
	}
	logrus.WithFields(logFields).Infof("[Tracing] Final canonical traceID (notifyID preferred): %s", traceID)

	// G2 fix: stamp experiment.id and experiment.run_id onto the long-running experiment-run span
	if observability.OTELTracerEnabled() {
		if traceID != "" && event.ExperimentRunID != "" && traceID != event.ExperimentRunID {
			// Move any span state keyed by experiment_run_id under canonical notifyID.
			observability.RebindExperimentSpan(event.ExperimentRunID, traceID)
		}
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

				var verdictAttrs []attribute.KeyValue
				verdictAttrs = []attribute.KeyValue{
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

	// Also submit per-fault observations via Langfuse REST for Langfuse-native consumers.
	// In OTEL mode, attach them to the active OTEL-exported trace ID for proper stitching.
	if isCompleted {
		tracer := observability.GetLangfuseTracer()
		if tracer.IsEnabled() {
			langfuseTraceID := traceID
			if observability.OTELTracerEnabled() {
				langfuseTraceID = observability.LinkedLangfuseTraceID(traceID)
			}
			if langfuseTraceID == "" {
				langfuseTraceID = traceID
			}
			for _, node := range executionData.Nodes {
				if node.Type == "ChaosEngine" && node.ChaosExp != nil {
					faultName := node.ChaosExp.ExperimentName
					if faultName == "" {
						faultName = node.ChaosExp.EngineName
					}
					_ = tracer.TraceExperimentObservation(ctx, &observability.ExperimentObservationDetails{
						TraceID:   langfuseTraceID,
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
							"source":    "rest-bridge",
						},
					})
				}
			}
		}
	}

	traceExperimentObservation(ctx, traceID, event, executionData, metricsPtr, c.agentRegistryOperator)
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

	// Multi-run triggering: if experiment completed successfully and is a multi-run experiment, trigger next run
	if isCompleted && len(experiment.Revision) > 0 {
		manifest := experiment.Revision[len(experiment.Revision)-1].ExperimentManifest
		
		// Debug: Log raw annotation values
		logrus.WithFields(logFields).Infof("[Multi-Run Debug] Checking manifest for multi-run annotations...")
		
		multiRunEnabled := gjson.Get(manifest, `metadata.annotations.litmuschaos\.io/multiRunEnabled`).String()
		maxRunsStr := gjson.Get(manifest, `metadata.annotations.litmuschaos\.io/maxRuns`).String()
		currentRunStr := gjson.Get(manifest, `metadata.annotations.litmuschaos\.io/currentRun`).String()
		
		logrus.WithFields(logFields).Infof("[Multi-Run Debug] multiRunEnabled='%s', maxRuns='%s', currentRun='%s'", 
			multiRunEnabled, maxRunsStr, currentRunStr)
		
		if multiRunEnabled == "true" {
			maxRuns := 1
			if parsed, err := strconv.Atoi(maxRunsStr); err == nil && parsed > 1 {
				maxRuns = parsed
			}
			
			currentRun := 0
			if parsed, err := strconv.Atoi(currentRunStr); err == nil {
				currentRun = parsed
			}
			// This completed run means currentRun should be incremented
			currentRun++
			
			logrus.WithFields(logFields).Infof("[Multi-Run] Experiment completed. multiRunEnabled=%s, currentRun=%d, maxRuns=%d", 
				multiRunEnabled, currentRun, maxRuns)
			
			if currentRun < maxRuns {
				// More runs needed - update manifest with new currentRun and trigger next
				logrus.WithFields(logFields).Infof("[Multi-Run] Triggering run %d/%d...", currentRun+1, maxRuns)
				
				// Update the experiment manifest with incremented currentRun
				updatedManifest, err := sjson.Set(manifest, "metadata.annotations.litmuschaos\\.io/currentRun", strconv.Itoa(currentRun))
				if err != nil {
					logrus.WithFields(logFields).Errorf("[Multi-Run] Failed to update manifest currentRun: %v", err)
				} else {
					// Update revision in database
					experiment.Revision[len(experiment.Revision)-1].ExperimentManifest = updatedManifest
					
					filter := bson.D{{"experiment_id", experiment.ExperimentID}}
					update := bson.D{
						{"$set", bson.D{
							{"revision", experiment.Revision},
							{"updated_at", time.Now().UnixMilli()},
						}},
					}
					if err := c.chaosExperimentOperator.UpdateChaosExperiment(ctx, filter, update); err != nil {
						logrus.WithFields(logFields).Errorf("[Multi-Run] Failed to update experiment revision: %v", err)
					}
				}
				
				// Trigger next run in a goroutine after a delay
				// Capture values for goroutine
				expID := experiment.ExperimentID
				projID := experiment.ProjectID
				nextRun := currentRun + 1
				totalRuns := maxRuns
				handler := c
				// Extract auth token from context and store it for the goroutine
				// We cannot use the original context because it will be canceled when this function returns
				var authToken string
				if tkn, ok := ctx.Value(authorization.AuthKey).(string); ok {
					authToken = tkn
				}
				
				// Read configurable delay from annotation (default: 120 seconds = 2 minutes)
				delaySeconds := 120
				if delayStr := gjson.Get(manifest, `metadata.annotations.litmuschaos\.io/multiRunDelay`).String(); delayStr != "" {
					if parsed, err := strconv.Atoi(delayStr); err == nil && parsed > 0 {
						delaySeconds = parsed
					}
				}
				delayDuration := time.Duration(delaySeconds) * time.Second
				
				go func() {
					defer func() {
						if r := recover(); r != nil {
							logrus.Errorf("[Multi-Run] PANIC in trigger goroutine: %v", r)
						}
					}()
					
					logrus.Infof("[Multi-Run] Goroutine started, waiting %v before triggering run %d/%d for experiment %s", delayDuration, nextRun, totalRuns, expID)
					
					// Wait configured delay between runs
					time.Sleep(delayDuration)
					
					logrus.Infof("[Multi-Run] %v delay complete, fetching experiment %s", delayDuration, expID)
					
					// Re-fetch experiment with updated manifest
					updatedExperiment, err := handler.chaosExperimentOperator.GetExperiment(context.Background(), bson.D{{"experiment_id", expID}})
					if err != nil {
						logrus.Errorf("[Multi-Run] Failed to fetch updated experiment %s: %v", expID, err)
						return
					}
					
					logrus.Infof("[Multi-Run] Experiment fetched, calling RunChaosWorkFlow for %s", expID)
					
					// Create a fresh context with the auth token for the new run
					// Using context.Background() ensures the context won't be canceled
					newCtx := context.Background()
					if authToken != "" {
						newCtx = context.WithValue(newCtx, authorization.AuthKey, authToken)
					}
					
					// Trigger next run using fresh context with auth token
					// IMPORTANT: Must pass store.Store (not nil) to actually send the workflow to subscriber
					_, err = handler.RunChaosWorkFlow(newCtx, projID, updatedExperiment, store.Store)
					if err != nil {
						logrus.Errorf("[Multi-Run] Failed to trigger run %d for %s: %v", nextRun, expID, err)
					} else {
						logrus.Infof("[Multi-Run] Successfully triggered run %d/%d for %s", nextRun, totalRuns, expID)
					}
				}()
			} else {
				logrus.WithFields(logFields).Infof("[Multi-Run] All %d runs completed!", maxRuns)
				
				// Reset currentRun to 0 for next batch
				updatedManifest, err := sjson.Set(manifest, "metadata.annotations.litmuschaos\\.io/currentRun", "0")
				if err == nil {
					experiment.Revision[len(experiment.Revision)-1].ExperimentManifest = updatedManifest
					filter := bson.D{{"experiment_id", experiment.ExperimentID}}
					update := bson.D{
						{"$set", bson.D{
							{"revision", experiment.Revision},
							{"updated_at", time.Now().UnixMilli()},
						}},
					}
					_ = c.chaosExperimentOperator.UpdateChaosExperiment(ctx, filter, update)
				}
			}
		}
	}

	return fmt.Sprintf("Experiment run received for for ExperimentID: %s, ExperimentRunID: %s", event.ExperimentID, event.ExperimentRunID), nil
}
