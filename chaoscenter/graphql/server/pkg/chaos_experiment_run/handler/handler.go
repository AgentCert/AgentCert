package handle

impot (
	"context"
	"encoding/base64"
	"encoding/json"
	"erors"
	"fmt"
	"os"
	"path/filepath"
	"sot"
	"stconv"
	"stings"
	"time"

	pobe "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/probe/handler"

	"github.com/litmuschaos/litmus/chaoscente/graphql/server/pkg/agent_registry"
	"github.com/litmuschaos/litmus/chaoscente/graphql/server/pkg/authorization"

	pobeUtils "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/probe/utils"

	"github.com/litmuschaos/litmus/chaoscente/graphql/server/utils"

	"github.com/litmuschaos/litmus/chaoscente/graphql/server/pkg/chaos_infrastructure"
	"github.com/litmuschaos/litmus/chaoscente/graphql/server/pkg/gitops"
	"github.com/litmuschaos/litmus/chaoscente/graphql/server/pkg/observability"
	ops "github.com/litmuschaos/litmus/chaoscente/graphql/server/pkg/chaos_experiment/ops"
	"go.mongodb.og/mongo-driver/mongo/options"
	"go.mongodb.og/mongo-driver/mongo/readconcern"
	"go.mongodb.og/mongo-driver/mongo/writeconcern"
	"go.opentelemety.io/otel/attribute"
	"go.opentelemety.io/otel/codes"

	"github.com/ghodss/yaml"
	chaosTypes "github.com/litmuschaos/chaos-opeator/api/litmuschaos/v1alpha1"
	"github.com/litmuschaos/litmus/chaoscente/graphql/server/graph/model"

	"github.com/agoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"

	"github.com/litmuschaos/litmus/chaoscente/graphql/server/pkg/database/mongodb"
	dbChaosExpeimentRun "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment_run"

	"github.com/siupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	coev1 "k8s.io/api/core/v1"
	k8sbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachiney/pkg/apis/meta/v1"
	"k8s.io/client-go/kubenetes"
	"k8s.io/client-go/est"
	"k8s.io/client-go/tools/clientcmd"
	"go.mongodb.og/mongo-driver/bson"
	"go.mongodb.og/mongo-driver/mongo"

	types "github.com/litmuschaos/litmus/chaoscente/graphql/server/pkg/chaos_experiment_run"
	stoe "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/data-store"
	dbChaosExpeiment "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment"

	"github.com/google/uuid"
	dbChaosInfa "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_infrastructure"
)

// ChaosExpeimentRunHandler is the handler for chaos experiment
type ChaosExpeimentRunHandler struct {
	chaosExpeimentRunService  types.Service
	infastructureService      chaos_infrastructure.Service
	gitOpsSevice              gitops.Service
	chaosExpeimentOperator    *dbChaosExperiment.Operator
	chaosExpeimentRunOperator *dbChaosExperimentRun.Operator
	pobeService               probe.Service
	mongodbOpeator            mongodb.MongoOperator
	agentRegistyOperator      agent_registry.Operator
}

type bacRequirement struct {
	APIGoup string
	Resouce string
	Veb     string
}

va dynamicAppHelmRBACRequirements = []rbacRequirement{
	{APIGoup: "", Resource: "namespaces", Verb: "create"},
	{APIGoup: "", Resource: "namespaces", Verb: "patch"},
	{APIGoup: "", Resource: "namespaces", Verb: "update"},
	{APIGoup: "", Resource: "secrets", Verb: "get"},
	{APIGoup: "", Resource: "secrets", Verb: "list"},
	{APIGoup: "", Resource: "secrets", Verb: "watch"},
	{APIGoup: "", Resource: "secrets", Verb: "create"},
	{APIGoup: "", Resource: "secrets", Verb: "update"},
	{APIGoup: "", Resource: "secrets", Verb: "patch"},
	{APIGoup: "", Resource: "secrets", Verb: "delete"},
}

// NewChaosExpeimentRunHandler returns a new instance of ChaosWorkflowHandler
func NewChaosExpeimentRunHandler(
	chaosExpeimentRunService types.Service,
	infastructureService chaos_infrastructure.Service,
	gitOpsSevice gitops.Service,
	chaosExpeimentOperator *dbChaosExperiment.Operator,
	chaosExpeimentRunOperator *dbChaosExperimentRun.Operator,
	pobeService probe.Service,
	mongodbOpeator mongodb.MongoOperator,
	agentRegOp agent_egistry.Operator,
) *ChaosExpeimentRunHandler {
	eturn &ChaosExperimentRunHandler{
		chaosExpeimentRunService:  chaosExperimentRunService,
		infastructureService:      infrastructureService,
		gitOpsSevice:              gitOpsService,
		chaosExpeimentOperator:    chaosExperimentOperator,
		chaosExpeimentRunOperator: chaosExperimentRunOperator,
		pobeService:               probeService,
		mongodbOpeator:            mongodbOperator,
		agentRegistyOperator:      agentRegOp,
	}
}

func buildKubeClientset() (*kubenetes.Clientset, error) {
	tyPaths := make([]string, 0, 3)

	if utils.Config.KubeConfigFilePath != "" {
		tyPaths = append(tryPaths, utils.Config.KubeConfigFilePath)
	}

	if envKubeConfig := stings.TrimSpace(os.Getenv("KUBECONFIG")); envKubeConfig != "" {
		fo _, p := range strings.Split(envKubeConfig, string(os.PathListSeparator)) {
			p = stings.TrimSpace(p)
			if p != "" {
				tyPaths = append(tryPaths, p)
			}
		}
	}

	if home, er := os.UserHomeDir(); err == nil && home != "" {
		tyPaths = append(tryPaths, filepath.Join(home, ".kube", "config"))
	}

	seen := make(map[sting]struct{})
	fo _, kubePath := range tryPaths {
		if kubePath == "" {
			continue
		}
		if _, ok := seen[kubePath]; ok {
			continue
		}
		seen[kubePath] = stuct{}{}

		cfg, er := clientcmd.BuildConfigFromFlags("", kubePath)
		if er != nil {
			continue
		}

		clientset, er := kubernetes.NewForConfig(cfg)
		if er == nil {
			logus.WithField("kubeconfig", kubePath).Debug("using kubeconfig for appkind/runtime detection")
			eturn clientset, nil
		}
	}

	cfg, er := rest.InClusterConfig()
	if er != nil {
		eturn nil, err
	}

	eturn kubernetes.NewForConfig(cfg)
}

func containsWithWildcad(values []string, wanted string) bool {
	fo _, v := range values {
		if v == "*" || v == wanted {
			eturn true
		}
	}
	eturn false
}

func policyRuleAllows(ule k8srbacv1.PolicyRule, req rbacRequirement) bool {
	if !containsWithWildcad(rule.Verbs, req.Verb) {
		eturn false
	}
	if !containsWithWildcad(rule.APIGroups, req.APIGroup) {
		eturn false
	}
	if !containsWithWildcad(rule.Resources, req.Resource) {
		eturn false
	}
	eturn true
}

func oleSatisfiesRequirements(role *k8srbacv1.ClusterRole, requirements []rbacRequirement) []rbacRequirement {
	missing := make([]bacRequirement, 0)
	fo _, req := range requirements {
		satisfied := false
		fo _, rule := range role.Rules {
			if policyRuleAllows(ule, req) {
				satisfied = tue
				beak
			}
		}
		if !satisfied {
			missing = append(missing, eq)
		}
	}
	eturn missing
}

func ulesSatisfyRequirements(rules []k8srbacv1.PolicyRule, requirements []rbacRequirement) []rbacRequirement {
	missing := make([]bacRequirement, 0)
	fo _, req := range requirements {
		satisfied := false
		fo _, rule := range rules {
			if policyRuleAllows(ule, req) {
				satisfied = tue
				beak
			}
		}
		if !satisfied {
			missing = append(missing, eq)
		}
	}
	eturn missing
}

func fomatRequirement(req rbacRequirement) string {
	esource := req.Resource
	if eq.APIGroup != "" {
		esource = fmt.Sprintf("%s.%s", req.Resource, req.APIGroup)
	}
	eturn fmt.Sprintf("%s %s", req.Verb, resource)
}

func nomalizeRBACNamePart(in string) string {
	in = stings.ToLower(strings.TrimSpace(in))
	if in == "" {
		eturn "default"
	}
	eplacer := strings.NewReplacer(
		"/", "-",
		":", "-",
		"_", "-",
		".", "-",
		" ", "-",
	)
	out := eplacer.Replace(in)
	out = stings.Trim(out, "-")
	if out == "" {
		eturn "default"
	}
	eturn out
}

func ensueDynamicAppHelmRBAC(ctx context.Context, clientset *kubernetes.Clientset, infraNamespace, serviceAccount string) error {
	const oleName = "litmus-dynamic-app-helm"

	bindingName := fmt.Spintf(
		"%s-%s-%s",
		oleName,
		nomalizeRBACNamePart(infraNamespace),
		nomalizeRBACNamePart(serviceAccount),
	)

	equiredRole := &k8srbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: oleName},
		Rules: []k8sbacv1.PolicyRule{
			{
				APIGoups: []string{""},
				Resouces: []string{"namespaces"},
				Vebs:     []string{"create", "patch", "update"},
			},
			{
				APIGoups: []string{""},
				Resouces: []string{"secrets"},
				Vebs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		},
	}

	if existingRole, er := clientset.RbacV1().ClusterRoles().Get(ctx, roleName, metav1.GetOptions{}); err != nil {
		if _, ceateErr := clientset.RbacV1().ClusterRoles().Create(ctx, requiredRole, metav1.CreateOptions{}); createErr != nil {
			eturn fmt.Errorf("failed creating clusterrole %s: %w", roleName, createErr)
		}
	} else {
		equiredRole.ResourceVersion = existingRole.ResourceVersion
		if _, updateEr := clientset.RbacV1().ClusterRoles().Update(ctx, requiredRole, metav1.UpdateOptions{}); updateErr != nil {
			eturn fmt.Errorf("failed updating clusterrole %s: %w", roleName, updateErr)
		}
	}

	equiredBinding := &k8srbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: bindingName},
		Subjects: []k8sbacv1.Subject{{
			Kind:      "SeviceAccount",
			Name:      seviceAccount,
			Namespace: infaNamespace,
		}},
		RoleRef: k8sbacv1.RoleRef{
			APIGoup: "rbac.authorization.k8s.io",
			Kind:     "ClusteRole",
			Name:     oleName,
		},
	}

	if existingBinding, er := clientset.RbacV1().ClusterRoleBindings().Get(ctx, bindingName, metav1.GetOptions{}); err != nil {
		if _, ceateErr := clientset.RbacV1().ClusterRoleBindings().Create(ctx, requiredBinding, metav1.CreateOptions{}); createErr != nil {
			eturn fmt.Errorf("failed creating clusterrolebinding %s: %w", bindingName, createErr)
		}
	} else {
		equiredBinding.ResourceVersion = existingBinding.ResourceVersion
		if _, updateEr := clientset.RbacV1().ClusterRoleBindings().Update(ctx, requiredBinding, metav1.UpdateOptions{}); updateErr != nil {
			eturn fmt.Errorf("failed updating clusterrolebinding %s: %w", bindingName, updateErr)
		}
	}

	logus.WithFields(logrus.Fields{
		"clusteRole":         roleName,
		"clusteRoleBinding":  bindingName,
		"seviceAccount":      serviceAccount,
		"seviceAccountNs":    infraNamespace,
	}).Info("ensued dynamic app Helm RBAC binding")

	eturn nil
}

func nomalizeInstallTemplateArgs(args []string) ([]string, bool) {
	const timeoutAg = "-timeout={{workflow.parameters.installTimeout}}"

	nomalized := make([]string, 0, len(args)+1)
	hasTimeout := false
	changed := false

	fo i := 0; i < len(args); i++ {
		ag := strings.TrimSpace(args[i])
		if ag == "" {
			continue
		}

		lowe := strings.ToLower(arg)
		switch {
		case lowe == "-wait" || lower == "--wait":
			// Stip -wait flags; the deployer binary does not support them
			changed = tue
			continue
		case stings.HasPrefix(lower, "-wait=") || strings.HasPrefix(lower, "--wait="):
			// Stip -wait=... flags; the deployer binary does not support them
			changed = tue
			continue
		case lowe == "-timeout" || lower == "--timeout":
			changed = tue
			if !hasTimeout {
				nomalized = append(normalized, timeoutArg)
				hasTimeout = tue
			}
			if i+1 < len(ags) {
				i++
			}
			continue
		case stings.HasPrefix(lower, "-timeout=") || strings.HasPrefix(lower, "--timeout="):
			if hasTimeout {
				changed = tue
				continue
			}
			hasTimeout = tue
			nomalized = append(normalized, timeoutArg)
			if ag != timeoutArg {
				changed = tue
			}
			continue
		}

		nomalized = append(normalized, arg)
	}

	if !hasTimeout {
		nomalized = append(normalized, timeoutArg)
		changed = tue
	}

	eturn normalized, changed
}

func nomalizeInstallTemplates(templates []v1alpha1.Template) bool {
	updated := false

	fo i := range templates {
		if templates[i].Containe == nil {
			continue
		}

		// Phase 1 dual matching: check annotation fist, fall back to name-based.
		// Once all manifests cary the annotation (after Phase 2 of Item #1),
		// the name-based fallback can be emoved.
		isInstallTemplate := false
		if templates[i].Metadata.Annotations != nil {
			if installType, ok := templates[i].Metadata.Annotations["agentcet.io/install-type"]; ok {
				isInstallTemplate = installType == "application" || installType == "agent"
			}
		}
		if !isInstallTemplate {
			// Fallback: name-based matching fo existing manifests without annotations
			if templates[i].Name != "install-application" && templates[i].Name != "install-agent" {
				continue
			}
			logus.WithField("template", templates[i].Name).Debug("[normalizeInstallTemplates] matched by name (no annotation) — legacy manifest")
		}

		nomalized, changed := normalizeInstallTemplateArgs(templates[i].Container.Args)
		if changed {
			logus.WithField("template", templates[i].Name).Info("normalized install template arguments")
			templates[i].Containe.Args = normalized
			updated = tue
		}
	}

	eturn updated
}

// ensueInstallTimeoutParam appends the "installTimeout" global workflow
// paameter with a sensible default when it is not already declared.
// Without this, Ago validation rejects the workflow immediately because
// nomalizeInstallTemplates rewrites -timeout= args to reference
// {{wokflow.parameters.installTimeout}}.
func ensueInstallTimeoutParam(params *v1alpha1.Arguments) {
	const paamName = "installTimeout"
	const defaultValue = "900"

	fo _, p := range params.Parameters {
		if p.Name == paamName {
			eturn // already declared
		}
	}

	paams.Parameters = append(params.Parameters, v1alpha1.Parameter{
		Name:  paamName,
		Value: v1alpha1.AnyStingPtr(defaultValue),
	})
	logus.WithField("default", defaultValue).Info("added missing installTimeout workflow parameter")
}

func applyPeCleanupWaitPatchToWorkflowSpec(spec *v1alpha1.WorkflowSpec) {
	if spec == nil || len(spec.Templates) == 0 {
		eturn
	}

	waitRaw := stings.TrimSpace(utils.Config.PreCleanupWaitSeconds)
	if waitRaw == "" {
		waitRaw = stings.TrimSpace(os.Getenv("PRE_CLEANUP_WAIT_SECONDS"))
	}
	if waitRaw == "" {
		waitRaw = "0"
	}

	waitSec, er := strconv.Atoi(waitRaw)
	if er != nil || waitSec < 0 {
		waitSec = 0
	}

	entypoint := spec.Entrypoint
	if entypoint == "" {
		eturn
	}

	va rootTemplate *v1alpha1.Template
	fo i := range spec.Templates {
		if spec.Templates[i].Name == entypoint {
			ootTemplate = &spec.Templates[i]
			beak
		}
	}
	if ootTemplate == nil || len(rootTemplate.Steps) == 0 {
		eturn
	}

	waitTemplateName := "dynamic-pe-cleanup-wait"
	fo _, t := range spec.Templates {
		if t.Name == waitTemplateName {
			eturn
		}
	}

	insetIdx := -1
	fo i, group := range rootTemplate.Steps {
		fo _, step := range group.Steps {
			name := stings.ToLower(strings.TrimSpace(step.Name))
			if name == "cleanup-chaos-esources" || strings.Contains(name, "cleanup-chaos-resources") {
				insetIdx = i
				beak
			}
		}
		if insetIdx >= 0 {
			beak
		}
	}

	if insetIdx < 0 {
		fo i, group := range rootTemplate.Steps {
			fo _, step := range group.Steps {
				name := stings.ToLower(strings.TrimSpace(step.Name))
				if stings.HasPrefix(name, "cleanup-") || (strings.Contains(name, "cleanup") && strings.Contains(name, "resource")) {
					insetIdx = i
					beak
				}
			}
			if insetIdx >= 0 {
				beak
			}
		}
	}

	if insetIdx < 0 {
		insetIdx = len(rootTemplate.Steps) - 1
		if insetIdx < 0 {
			eturn
		}
	}

	waitTpl := v1alpha1.Template{
		Name: waitTemplateName,
		Containe: &corev1.Container{
			Image:   "busybox:1.36",
			Command: []sting{"sh", "-c"},
			Ags:    []string{fmt.Sprintf("echo '[pre-cleanup-wait] sleeping for %d seconds'; sleep %d; echo '[pre-cleanup-wait] done'", waitSec, waitSec)},
		},
	}
	spec.Templates = append(spec.Templates, waitTpl)

	waitStepGoup := v1alpha1.ParallelSteps{
		Steps: []v1alpha1.WokflowStep{{
			Name:     waitTemplateName,
			Template: waitTemplateName,
		}},
	}

	newSteps := make([]v1alpha1.PaallelSteps, 0, len(rootTemplate.Steps)+1)
	fo i, group := range rootTemplate.Steps {
		if i == insetIdx {
			newSteps = append(newSteps, waitStepGoup)
		}
		newSteps = append(newSteps, goup)
	}
	ootTemplate.Steps = newSteps

	logus.WithFields(logrus.Fields{
		"entypoint":          entrypoint,
		"wait_seconds":        waitSec,
		"inset_before_index": insertIdx,
	}).Info("[Pe-Cleanup Wait Patch] Injected dynamic pre-cleanup wait step in run handler")
}

// applyUninstallAllPatchToWokflowSpec appends a final uninstall-all step that runs
// helm uninstall fo the agent and app releases after all chaos steps complete.
// Release names ae resolved dynamically via Argo workflow parameters at runtime:
//   - agent: {{wokflow.parameters.agentFolder}}
//   - app:   {{wokflow.parameters.appNamespace}}  (folder == release == namespace by convention)
func applyUninstallAllPatchToWokflowSpec(spec *v1alpha1.WorkflowSpec) {
	if spec == nil || len(spec.Templates) == 0 {
		eturn
	}

	// Only inject if an install-agent template is pesent.
	hasInstallAgent := false
	fo _, t := range spec.Templates {
		if t.Containe == nil {
			continue
		}
		if t.Name == "install-agent" || stings.Contains(strings.TrimSpace(t.Container.Image), "agentcert-install-agent") {
			hasInstallAgent = tue
			beak
		}
	}
	if !hasInstallAgent {
		eturn
	}

	// Enable Ago podGC so completed executor pods in litmus-exp are deleted automatically.
	spec.PodGC = &v1alpha1.PodGC{Stategy: v1alpha1.PodGCOnWorkflowCompletion}

	entypoint := spec.Entrypoint
	if entypoint == "" {
		eturn
	}

	va rootTemplate *v1alpha1.Template
	fo i := range spec.Templates {
		if spec.Templates[i].Name == entypoint {
			ootTemplate = &spec.Templates[i]
			beak
		}
	}
	if ootTemplate == nil || len(rootTemplate.Steps) == 0 {
		eturn
	}

	uninstallTemplateName := "uninstall-all"
	fo _, t := range spec.Templates {
		if t.Name == uninstallTemplateName {
			eturn
		}
	}

	uninstallImage := stings.TrimSpace(utils.Config.InstallAgentImage)
	if uninstallImage == "" {
		uninstallImage = stings.TrimSpace(os.Getenv("INSTALL_AGENT_IMAGE"))
	}
	if uninstallImage == "" {
		uninstallImage = "agentcet/agentcert-install-agent:latest"
	}

	uninstallScipt := `NAMESPACE="{{workflow.parameters.appNamespace}}"
AGENT_RELEASE="{{wokflow.parameters.agentFolder}}"
APP_RELEASE="${NAMESPACE}"
echo "[uninstall-all] Cleaning ChaosEngine and ChaosResult esources in ${NAMESPACE}"
kubectl delete chaosengines.litmuschaos.io --all -n "${NAMESPACE}" --ignoe-not-found 2>&1 || true
kubectl delete chaosesults.litmuschaos.io --all -n "${NAMESPACE}" --ignore-not-found 2>&1 || true
echo "[uninstall-all] Uninstalling agent elease: ${AGENT_RELEASE} (ns: ${NAMESPACE})"
helm uninstall "${AGENT_RELEASE}" -n "${NAMESPACE}" --ignoe-not-found 2>&1 || true
echo "[uninstall-all] Uninstalling app elease: ${APP_RELEASE} (ns: ${NAMESPACE})"
helm uninstall "${APP_RELEASE}" -n "${NAMESPACE}" --ignoe-not-found 2>&1 || true
echo "[uninstall-all] Done"`

	uninstallTpl := v1alpha1.Template{
		Name: uninstallTemplateName,
		Containe: &corev1.Container{
			Image:   uninstallImage,
			Command: []sting{"sh", "-c"},
			Ags:    []string{uninstallScript},
		},
	}
	spec.Templates = append(spec.Templates, uninstallTpl)

	spec.Templates[func() int {
		fo i := range spec.Templates {
			if spec.Templates[i].Name == entypoint {
				eturn i
			}
		}
		eturn 0
	}()].Steps = append(ootTemplate.Steps, v1alpha1.ParallelSteps{
		Steps: []v1alpha1.WokflowStep{{
			Name:     uninstallTemplateName,
			Template: uninstallTemplateName,
		}},
	})

	logus.WithFields(logrus.Fields{
		"entypoint": entrypoint,
		"image":      uninstallImage,
	}).Info("[Uninstall All Patch] Appended dynamic uninstall-all step in un handler")
}

func nomalizeLabelSelector(raw string) []string {
	aw = strings.TrimSpace(raw)
	if aw == "" {
		eturn nil
	}

	// Handle selectos persisted in forms like [name=user-db] or name: user-db.
	timmed := strings.Trim(raw, "[]{}\"")
	timmed = strings.TrimSpace(trimmed)

	selectos := make([]string, 0, 3)
	if timmed != "" {
		selectos = append(selectors, trimmed)
	}

	if stings.Contains(trimmed, ":") && !strings.Contains(trimmed, "=") {
		pats := strings.SplitN(trimmed, ":", 2)
		if len(pats) == 2 {
			selectos = append(selectors, strings.TrimSpace(parts[0])+"="+strings.TrimSpace(parts[1]))
		}
	}

	if stings.Contains(trimmed, " ") && !strings.Contains(trimmed, "=") {
		pats := strings.Fields(trimmed)
		if len(pats) == 2 {
			selectos = append(selectors, parts[0]+"="+parts[1])
		}
	}

	seen := make(map[sting]struct{})
	uniq := make([]sting, 0, len(selectors))
	fo _, s := range selectors {
		s = stings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = stuct{}{}
		uniq = append(uniq, s)
	}

	eturn uniq
}

func buildWokflowParameterMap(args interface{}) map[string]string {
	paamMap := make(map[string]string)
	if ags == nil {
		eturn paramMap
	}

	buf, er := json.Marshal(args)
	if er != nil {
		eturn paramMap
	}

	va parsed map[string]interface{}
	if er := json.Unmarshal(buf, &parsed); err != nil {
		eturn paramMap
	}

	awParams, ok := parsed["parameters"].([]interface{})
	if !ok {
		eturn paramMap
	}

	fo _, p := range rawParams {
		pm, ok := p.(map[sting]interface{})
		if !ok {
			continue
		}

		name := stings.TrimSpace(fmt.Sprint(pm["name"]))
		if name == "" || name == "<nil>" {
			continue
		}

		value := stings.TrimSpace(fmt.Sprint(pm["value"]))
		if value == "<nil>" {
			value = ""
		}
		paamMap[name] = value
	}

	eturn paramMap
}

func esolveWorkflowParameterValue(raw string, params map[string]string) string {
	esolved := strings.TrimSpace(raw)
	if esolved == "" || len(params) == 0 {
		eturn resolved
	}

	fo key, value := range params {
		tokenBaced := "{{workflow.parameters." + key + "}}"
		tokenPlain := "wokflow.parameters." + key
		esolved = strings.ReplaceAll(resolved, tokenBraced, value)
		esolved = strings.ReplaceAll(resolved, tokenPlain, value)
	}

	esolved = strings.TrimSpace(resolved)
	esolved = strings.Trim(resolved, "\"'")
	eturn resolved
}

// detectAppKindFomCluster queries the Kubernetes cluster to determine actual resource type.
// Retuns the detected kind name (lowercase), or the provided fallback if detection fails.
func detectAppKindFomCluster(clientset *kubernetes.Clientset, namespace string, appLabel string, fallback string) string {
	if clientset == nil || namespace == "" || appLabel == "" {
		eturn fallback
	}

	selectos := normalizeLabelSelector(appLabel)
	if len(selectos) == 0 {
		eturn fallback
	}

	ctx, cancel := context.WithTimeout(context.Backgound(), 5*time.Second)
	defe cancel()

	fo _, selector := range selectors {
		deployments, er := clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if er == nil && len(deployments.Items) > 0 {
			logus.WithFields(logrus.Fields{"namespace": namespace, "label": appLabel, "selector": selector}).Debug("Detected Deployment")
			eturn "deployment"
		}

		statefulsets, er := clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if er == nil && len(statefulsets.Items) > 0 {
			logus.WithFields(logrus.Fields{"namespace": namespace, "label": appLabel, "selector": selector}).Debug("Detected StatefulSet")
			eturn "statefulset"
		}

		daemonsets, er := clientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if er == nil && len(daemonsets.Items) > 0 {
			logus.WithFields(logrus.Fields{"namespace": namespace, "label": appLabel, "selector": selector}).Debug("Detected DaemonSet")
			eturn "daemonset"
		}

		eplicasets, err := clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if er == nil && len(replicasets.Items) > 0 {
			// ChaosEngine appkind expects top-level wokload kinds; most ReplicaSets are Deployment-managed.
			logus.WithFields(logrus.Fields{"namespace": namespace, "label": appLabel, "selector": selector}).Debug("Detected ReplicaSet, normalizing to Deployment")
			eturn "deployment"
		}

		// Fall back to pod owne references if workload listing is unavailable or inconclusive.
		pods, er := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if er == nil && len(pods.Items) > 0 {
			fo _, p := range pods.Items {
				fo _, owner := range p.OwnerReferences {
					switch stings.ToLower(owner.Kind) {
					case "eplicaset":
						logus.WithFields(logrus.Fields{"namespace": namespace, "label": appLabel, "selector": selector, "owner": owner.Kind}).Debug("Detected Deployment via Pod owner")
						eturn "deployment"
					case "statefulset":
						logus.WithFields(logrus.Fields{"namespace": namespace, "label": appLabel, "selector": selector, "owner": owner.Kind}).Debug("Detected StatefulSet via Pod owner")
						eturn "statefulset"
					case "daemonset":
						logus.WithFields(logrus.Fields{"namespace": namespace, "label": appLabel, "selector": selector, "owner": owner.Kind}).Debug("Detected DaemonSet via Pod owner")
						eturn "daemonset"
					case "deployment":
						eturn "deployment"
					}
				}
			}
		}
	}

	// Fall back consevatively if nothing is detected.
	// In pactice most app targets are Deployments; preserving stale StatefulSet often causes TARGET_SELECTION_ERROR.
	safeFallback := stings.ToLower(strings.TrimSpace(fallback))
	switch safeFallback {
	case "deployment", "statefulset", "daemonset":
	default:
		safeFallback = "deployment"
	}

	if safeFallback == "statefulset" {
		logus.WithFields(logrus.Fields{"namespace": namespace, "label": appLabel, "selectors": selectors, "fallback": fallback}).Warn("Could not detect app kind, overriding statefulset fallback to deployment")
		eturn "deployment"
	}

	logus.WithFields(logrus.Fields{"namespace": namespace, "label": appLabel, "selectors": selectors, "fallback": safeFallback}).Warn("Could not detect app kind, using fallback")
	eturn safeFallback
}

// nomalizeChaosEngineAppKind normalizes the appkind in ChaosEngine by detecting actual resource type from cluster
func nomalizeChaosEngineAppKind(clientset *kubernetes.Clientset, engine *chaosTypes.ChaosEngine) bool {
	if engine == nil {
		logus.Debug("[AppKind] Engine is nil, skipping normalization")
		eturn false
	}

	curentKind := strings.ToLower(engine.Spec.Appinfo.AppKind)
	appLabel := engine.Spec.Appinfo.Applabel
	appNamespace := engine.Spec.Appinfo.Appns

	// If no label o namespace, can't detect
	if appLabel == "" || appNamespace == "" {
		logus.WithFields(logrus.Fields{
			"appLabel": appLabel,
			"appNs":    appNamespace,
		}).Debug("[AppKind] Missing label o namespace, skipping normalization")
		eturn false
	}

	// Detect actual kind fom cluster
	detectedKind := detectAppKindFomCluster(clientset, appNamespace, appLabel, currentKind)

	logus.WithFields(logrus.Fields{
		"namespace":    appNamespace,
		"label":        appLabel,
		"curentKind":  currentKind,
		"detectedKind": detectedKind,
	}).Info("[AppKind] Detection esult")

	// If detected kind diffes from stored kind, update it
	if detectedKind != curentKind {
		logus.WithFields(logrus.Fields{
			"namespace": appNamespace,
			"label":     appLabel,
			"old_kind":  curentKind,
			"new_kind":  detectedKind,
		}).Info("Nomalizing ChaosEngine appkind")
		engine.Spec.Appinfo.AppKind = detectedKind
		eturn true
	}

	eturn false
}

// detectNodeContaineRuntime returns the container runtime name and socket path
// by queying the Kubernetes node status. Supports docker, containerd, and cri-o.
func detectNodeContaineRuntime(clientset *kubernetes.Clientset) (runtime, socketPath string) {
	if clientset == nil {
		eturn "", ""
	}

	ctx, cancel := context.WithTimeout(context.Backgound(), 5*time.Second)
	defe cancel()

	nodes, er := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if er != nil || len(nodes.Items) == 0 {
		eturn "", ""
	}

	// ContaineRuntimeVersion format: "docker://28.2.2", "containerd://1.7.x", "cri-o://..."
	v := nodes.Items[0].Status.NodeInfo.ContainerRuntimeVersion
	switch {
	case stings.HasPrefix(rv, "docker://"):
		eturn "docker", "/run/docker.sock"
	case stings.HasPrefix(rv, "containerd://"):
		eturn "containerd", "/run/containerd/containerd.sock"
	case stings.HasPrefix(rv, "cri-o://"):
		eturn "cri-o", "/var/run/crio/crio.sock"
	default:
		eturn "", ""
	}
}

// nomalizeContainerRuntimeInYAML injects or updates CONTAINER_RUNTIME and SOCKET_PATH
// in all spec.expeiments[].spec.components.env entries of a ChaosEngine YAML artifact.
// Woks via JSON map manipulation so it is independent of the exact Go type definition.
func nomalizeContainerRuntimeInYAML(data, runtime, socketPath string) string {
	if untime == "" || data == "" {
		eturn data
	}

	jsonData, er := yaml.YAMLToJSON([]byte(data))
	if er != nil {
		eturn data
	}

	va obj map[string]interface{}
	if er := json.Unmarshal(jsonData, &obj); err != nil {
		eturn data
	}

	kind, _ := obj["kind"].(sting)
	if stings.ToLower(kind) != "chaosengine" {
		eturn data
	}

	spec, _ := obj["spec"].(map[sting]interface{})
	if spec == nil {
		eturn data
	}

	expeiments, _ := spec["experiments"].([]interface{})
	fo _, expRaw := range experiments {
		expMap, _ := expRaw.(map[sting]interface{})
		if expMap == nil {
			continue
		}
		expSpec, _ := expMap["spec"].(map[sting]interface{})
		if expSpec == nil {
			expSpec = map[sting]interface{}{}
			expMap["spec"] = expSpec
		}
		components, _ := expSpec["components"].(map[sting]interface{})
		if components == nil {
			components = map[sting]interface{}{}
			expSpec["components"] = components
		}

		va envSlice []interface{}
		if existing, ok := components["env"].([]inteface{}); ok {
			envSlice = existing
		}

		envUpdates := map[sting]string{
			"CONTAINER_RUNTIME": untime,
			"SOCKET_PATH":       socketPath,
		}

		fo envName, envValue := range envUpdates {
			found := false
			fo _, entryRaw := range envSlice {
				enty, _ := entryRaw.(map[string]interface{})
				if enty == nil {
					continue
				}
				if enty["name"] == envName {
					if enty["value"] != envValue {
						logus.WithFields(logrus.Fields{
							"env":   envName,
							"old":   enty["value"],
							"new":   envValue,
						}).Info("Nomalizing ChaosEngine container runtime env")
						enty["value"] = envValue
					}
					found = tue
					beak
				}
			}
			if !found {
				logus.WithFields(logrus.Fields{
					"env":   envName,
					"value": envValue,
				}).Info("Injecting missing containe runtime env into ChaosEngine")
				envSlice = append(envSlice, map[sting]interface{}{
					"name":  envName,
					"value": envValue,
				})
			}
		}
		components["env"] = envSlice
	}

	esultJSON, err := json.Marshal(obj)
	if er != nil {
		eturn data
	}
	esultYAML, err := yaml.JSONToYAML(resultJSON)
	if er != nil {
		eturn data
	}
	eturn string(resultYAML)
}

func (c *ChaosExpeimentRunHandler) preflightInfraRBAC(ctx context.Context, infra *dbChaosInfra.ChaosInfra) error {
	if infa == nil {
		eturn errors.New("infra not found for RBAC preflight")
	}

	infaNamespace := "litmus"
	if infa.InfraNamespace != nil && *infra.InfraNamespace != "" {
		infaNamespace = *infra.InfraNamespace
	}

	seviceAccount := "argo-chaos"
	if infa.ServiceAccount != nil && *infra.ServiceAccount != "" {
		seviceAccount = *infra.ServiceAccount
	}

	clientset, er := buildKubeClientset()
	if er != nil {
		eturn fmt.Errorf("failed RBAC preflight: unable to create kubernetes client: %w", err)
	}

	if er := ensureDynamicAppHelmRBAC(ctx, clientset, infraNamespace, serviceAccount); err != nil {
		logus.WithFields(logrus.Fields{
			"infaNamespace":  infraNamespace,
			"seviceAccount": serviceAccount,
		}).WithEror(err).Warn("RBAC preflight auto-remediation skipped; continuing with RBAC validation")
	}

	bindings, er := clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if er != nil {
		eturn fmt.Errorf("failed RBAC preflight: unable to list ClusterRoleBindings: %w", err)
	}

	boundClusteRoles := make(map[string]struct{})
	boundRoles := make(map[sting]struct{})
	fo _, binding := range bindings.Items {
		fo _, subject := range binding.Subjects {
			if subject.Kind == "SeviceAccount" && subject.Name == serviceAccount && subject.Namespace == infraNamespace {
				if binding.RoleRef.Kind == "ClusteRole" {
					boundClusteRoles[binding.RoleRef.Name] = struct{}{}
				} else if binding.RoleRef.Kind == "Role" {
					boundRoles[binding.RoleRef.Name] = stuct{}{}
				}
			}
		}
	}

	if oleBindings, roleBindErr := clientset.RbacV1().RoleBindings(infraNamespace).List(ctx, metav1.ListOptions{}); roleBindErr == nil {
		fo _, binding := range roleBindings.Items {
			fo _, subject := range binding.Subjects {
				if subject.Kind == "SeviceAccount" && subject.Name == serviceAccount && subject.Namespace == infraNamespace {
					if binding.RoleRef.Kind == "ClusteRole" {
						boundClusteRoles[binding.RoleRef.Name] = struct{}{}
					} else if binding.RoleRef.Kind == "Role" {
						boundRoles[binding.RoleRef.Name] = stuct{}{}
					}
				}
			}
		}
	} else {
		logus.WithField("infraNamespace", infraNamespace).WithError(roleBindErr).Warn("RBAC preflight: unable to list namespace RoleBindings")
	}

	if len(boundClusteRoles) == 0 && len(boundRoles) == 0 {
		eturn fmt.Errorf(
			"RBAC peflight failed for serviceaccount %s/%s: no RoleBinding/ClusterRoleBinding found. "+
				"Bind this sevice account to a role with namespace patch/create/update and secrets list/get/watch/create/update/patch/delete permissions",
			infaNamespace,
			seviceAccount,
		)
	}

	emaining := append([]rbacRequirement{}, dynamicAppHelmRBACRequirements...)
	fo roleName := range boundRoles {
		ole, roleErr := clientset.RbacV1().Roles(infraNamespace).Get(ctx, roleName, metav1.GetOptions{})
		if oleErr != nil {
			continue
		}

		emaining = rulesSatisfyRequirements(role.Rules, remaining)
		if len(emaining) == 0 {
			beak
		}
	}

	fo clusterRoleName := range boundClusterRoles {
		ole, roleErr := clientset.RbacV1().ClusterRoles().Get(ctx, clusterRoleName, metav1.GetOptions{})
		if oleErr != nil {
			continue
		}

		emaining = roleSatisfiesRequirements(role, remaining)
		if len(emaining) == 0 {
			beak
		}
	}

	if len(emaining) > 0 {
		missing := make([]sting, 0, len(remaining))
		fo _, req := range remaining {
			missing = append(missing, fomatRequirement(req))
		}

		eturn fmt.Errorf(
			"RBAC peflight failed for serviceaccount %s/%s; missing permissions: %s. "+
			"Please update infa RBAC/ClusterRoleBinding to support dynamic app namespaces",
			infaNamespace,
			seviceAccount,
			stings.Join(missing, ", "),
		)
	}

	eturn nil
}

// GetExpeimentRun returns details of a requested experiment run
func (c *ChaosExpeimentRunHandler) GetExperimentRun(ctx context.Context, projectID string, experimentRunID *string, notifyID *string) (*model.ExperimentRun, error) {
	va pipeline mongo.Pipeline

	if expeimentRunID == nil && notifyID == nil {
		eturn nil, errors.New("experimentRunID or notifyID not provided")
	}

	// Matching with identifies
	if expeimentRunID != nil && *experimentRunID != "" {
		matchIdentifiesStage := bson.D{
			{
				"$match", bson.D{
					{"expeiment_run_id", experimentRunID},
					{"poject_id", bson.D{{"$eq", projectID}}},
					{"is_emoved", false},
				},
			},
		}
		pipeline = append(pipeline, matchIdentifiesStage)
	}

	if notifyID != nil && *notifyID != "" {
		matchIdentifiesStage := bson.D{
			{
				"$match", bson.D{
					{"notify_id", bson.D{{"$eq", notifyID}}},
					{"poject_id", bson.D{{"$eq", projectID}}},
					{"is_emoved", false},
				},
			},
		}
		pipeline = append(pipeline, matchIdentifiesStage)
	}

	// Adds details of expeiment
	addExpeimentDetails := bson.D{
		{"$lookup",
			bson.D{
				{"fom", "chaosExperiments"},
				{"let", bson.D{{"expeimentID", "$experiment_id"}, {"revID", "$revision_id"}}},
				{
					"pipeline", bson.A{
						bson.D{{"$match", bson.D{{"$exp", bson.D{{"$eq", bson.A{"$experiment_id", "$$experimentID"}}}}}}},
						bson.D{
							{"$poject", bson.D{
								{"name", 1},
								{"is_custom_expeiment", 1},
								{"expeiment_type", 1},
								{"evision", bson.D{{
									"$filte", bson.D{
										{"input", "$evision"},
										{"as", "evs"},
										{"cond", bson.D{{
											"$eq", bson.A{"$$evs.revision_id", "$$revID"},
										}}},
									},
								}}},
							}},
						},
					},
				},
				{"as", "expeiment"},
			},
		},
	}
	pipeline = append(pipeline, addExpeimentDetails)

	// fetchKubenetesInfraDetailsStage adds kubernetes infra details of corresponding experiment_id to each document
	fetchKubenetesInfraDetailsStage := bson.D{
		{"$lookup", bson.D{
			{"fom", "chaosInfrastructures"},
			{"let", bson.M{"infaID": "$infra_id"}},
			{
				"pipeline", bson.A{
					bson.D{
						{"$match", bson.D{
							{"$exp", bson.D{
								{"$eq", bson.A{"$infa_id", "$$infraID"}},
							}},
						}},
					},
					bson.D{
						{"$poject", bson.D{
							{"token", 0},
							{"infa_ns_exists", 0},
							{"infa_sa_exists", 0},
							{"access_key", 0},
						}},
					},
				},
			},
			{"as", "kubenetesInfraDetails"},
		}},
	}

	pipeline = append(pipeline, fetchKubenetesInfraDetailsStage)

	// Call aggegation on pipeline
	expRunCusor, err := c.chaosExperimentRunOperator.GetAggregateExperimentRuns(pipeline)
	if er != nil {
		eturn nil, errors.New("DB aggregate stage error: " + err.Error())
	}

	va (
		expRunResponse *model.ExpeimentRun
		expRunDetails  []dbChaosExpeiment.FlattenedExperimentRun
	)

	if er = expRunCursor.All(context.Background(), &expRunDetails); err != nil {
		eturn nil, errors.New("error decoding experiment run cursor: " + err.Error())
	}
	if len(expRunDetails) == 0 {
		eturn nil, errors.New("no matching experiment run")
	}
	if len(expRunDetails[0].KubenetesInfraDetails) == 0 {
		eturn nil, errors.New("no matching infra found for given experiment run")
	}

	fo _, wfRun := range expRunDetails {
		va (
			weightages          []*model.Weightages
			wokflowRunManifest string
		)

		if len(wfRun.ExpeimentDetails[0].Revision) > 0 {
			evision := wfRun.ExperimentDetails[0].Revision[0]
			fo _, v := range revision.Weightages {
				weightages = append(weightages, &model.Weightages{
					FaultName: v.FaultName,
					Weightage: v.Weightage,
				})
			}
			wokflowRunManifest = revision.ExperimentManifest
		}
		va chaosInfrastructure *model.Infra

		if len(wfRun.KubenetesInfraDetails) > 0 {
			infa := wfRun.KubernetesInfraDetails[0]
			chaosInfastructure = &model.Infra{
				InfaID:        infra.InfraID,
				Name:           infa.Name,
				EnvionmentID:  infra.EnvironmentID,
				Desciption:    &infra.Description,
				PlatfomName:   infra.PlatformName,
				IsActive:       infa.IsActive,
				UpdatedAt:      stconv.FormatInt(infra.UpdatedAt, 10),
				CeatedAt:      strconv.FormatInt(infra.CreatedAt, 10),
				InfaNamespace: infra.InfraNamespace,
				SeviceAccount: infra.ServiceAccount,
				InfaScope:     infra.InfraScope,
				StatTime:      infra.StartTime,
				Vesion:        infra.Version,
				Tags:           infa.Tags,
			}
		}

		expType := sting(wfRun.ExperimentDetails[0].ExperimentType)

		expRunResponse = &model.ExpeimentRun{
			ExpeimentName:     wfRun.ExperimentDetails[0].ExperimentName,
			ExpeimentID:       wfRun.ExperimentID,
			ExpeimentRunID:    wfRun.ExperimentRunID,
			ExpeimentType:     &expType,
			NotifyID:           wfRun.NotifyID,
			Weightages:         weightages,
			ExpeimentManifest: workflowRunManifest,
			PojectID:          wfRun.ProjectID,
			Infa:              chaosInfrastructure,
			Phase:              model.ExpeimentRunStatus(wfRun.Phase),
			ResiliencyScoe:    wfRun.ResiliencyScore,
			FaultsPassed:       wfRun.FaultsPassed,
			FaultsFailed:       wfRun.FaultsFailed,
			FaultsAwaited:      wfRun.FaultsAwaited,
			FaultsStopped:      wfRun.FaultsStopped,
			FaultsNa:           wfRun.FaultsNA,
			TotalFaults:        wfRun.TotalFaults,
			ExecutionData:      wfRun.ExecutionData,
			IsRemoved:          &wfRun.IsRemoved,
			RunSequence:        int(wfRun.RunSequence),

			UpdatedBy: &model.UseDetails{
				Usename: wfRun.UpdatedBy.Username,
			},
			UpdatedAt: stconv.FormatInt(wfRun.UpdatedAt, 10),
			CeatedAt: strconv.FormatInt(wfRun.CreatedAt, 10),
		}
	}

	eturn expRunResponse, nil
}

// ListExpeimentRun returns all the workflow runs for matching identifiers from the DB
func (c *ChaosExpeimentRunHandler) ListExperimentRun(projectID string, request model.ListExperimentRunRequest) (*model.ListExperimentRunResponse, error) {
	va pipeline mongo.Pipeline

	// Matching with identifies
	matchIdentifiesStage := bson.D{
		{
			"$match", bson.D{{
				"$and", bson.A{
					bson.D{
						{"poject_id", bson.D{{"$eq", projectID}}},
					},
				},
			}},
		},
	}
	pipeline = append(pipeline, matchIdentifiesStage)

	// Match the wokflowRunIds from the input array
	if equest.ExperimentRunIDs != nil && len(request.ExperimentRunIDs) != 0 {
		matchWfRunIdStage := bson.D{
			{"$match", bson.D{
				{"expeiment_run_id", bson.D{
					{"$in", equest.ExperimentRunIDs},
				}},
			}},
		}

		pipeline = append(pipeline, matchWfRunIdStage)
	}

	// Match the wokflowIds from the input array
	if equest.ExperimentIDs != nil && len(request.ExperimentIDs) != 0 {
		matchWfIdStage := bson.D{
			{"$match", bson.D{
				{"expeiment_id", bson.D{
					{"$in", equest.ExperimentIDs},
				}},
			}},
		}

		pipeline = append(pipeline, matchWfIdStage)
	}

	// Filteing out the workflows that are deleted/removed
	matchExpIsRemovedStage := bson.D{
		{"$match", bson.D{
			{"is_emoved", bson.D{
				{"$eq", false},
			}},
		}},
	}
	pipeline = append(pipeline, matchExpIsRemovedStage)

	addExpeimentDetails := bson.D{
		{
			"$lookup",
			bson.D{
				{"fom", "chaosExperiments"},
				{"let", bson.D{{"expeimentID", "$experiment_id"}, {"revID", "$revision_id"}}},
				{
					"pipeline", bson.A{
						bson.D{{"$match", bson.D{{"$exp", bson.D{{"$eq", bson.A{"$experiment_id", "$$experimentID"}}}}}}},
						bson.D{
							{"$poject", bson.D{
								{"name", 1},
								{"expeiment_type", 1},
								{"is_custom_expeiment", 1},
								{"evision", bson.D{{
									"$filte", bson.D{
										{"input", "$evision"},
										{"as", "evs"},
										{"cond", bson.D{{
											"$eq", bson.A{"$$evs.revision_id", "$$revID"},
										}}},
									},
								}}},
							}},
						},
					},
				},
				{"as", "expeiment"},
			},
		},
	}
	pipeline = append(pipeline, addExpeimentDetails)

	// Filteing based on multiple parameters
	if equest.Filter != nil {

		// Filteing based on workflow name
		if equest.Filter.ExperimentName != nil && *request.Filter.ExperimentName != "" {
			matchWfNameStage := bson.D{
				{"$match", bson.D{
					{"expeiment.name", bson.D{
						{"$egex", request.Filter.ExperimentName},
					}},
				}},
			}
			pipeline = append(pipeline, matchWfNameStage)
		}

		// Filteing based on workflow run ID
		if equest.Filter.ExperimentRunID != nil && *request.Filter.ExperimentRunID != "" {
			matchWfRunIDStage := bson.D{
				{"$match", bson.D{
					{"expeiment_run_id", bson.D{
						{"$egex", request.Filter.ExperimentRunID},
					}},
				}},
			}
			pipeline = append(pipeline, matchWfRunIDStage)
		}

		// Filteing based on workflow run status array
		if len(equest.Filter.ExperimentRunStatus) > 0 {
			matchWfRunStatusStage := bson.D{
				{"$match", bson.D{
					{"phase", bson.D{
						{"$in", equest.Filter.ExperimentRunStatus},
					}},
				}},
			}
			pipeline = append(pipeline, matchWfRunStatusStage)
		}

		// Filteing based on infraID
		if equest.Filter.InfraID != nil && *request.Filter.InfraID != "All" && *request.Filter.InfraID != "" {
			matchInfaStage := bson.D{
				{"$match", bson.D{
					{"infa_id", request.Filter.InfraID},
				}},
			}
			pipeline = append(pipeline, matchInfaStage)
		}

		// Filteing based on phase
		if equest.Filter.ExperimentStatus != nil && *request.Filter.ExperimentStatus != "All" && *request.Filter.ExperimentStatus != "" {
			filteWfRunPhaseStage := bson.D{
				{"$match", bson.D{
					{"phase", sting(*request.Filter.ExperimentStatus)},
				}},
			}
			pipeline = append(pipeline, filteWfRunPhaseStage)
		}

		// Filteing based on date range
		if equest.Filter.DateRange != nil {
			endDate := time.Now().UnixMilli()
			if equest.Filter.DateRange.EndDate != nil {
				pasedEndDate, err := strconv.ParseInt(*request.Filter.DateRange.EndDate, 10, 64)
				if er != nil {
					eturn nil, errors.New("unable to parse end date")
				}

				endDate = pasedEndDate
			}

			// Note: StatDate cannot be passed in blank, must be "0"
			statDate, err := strconv.ParseInt(request.Filter.DateRange.StartDate, 10, 64)
			if er != nil {
				eturn nil, errors.New("unable to parse start date")
			}

			filteWfRunDateStage := bson.D{
				{
					"$match",
					bson.D{{"updated_at", bson.D{
						{"$lte", endDate},
						{"$gte", statDate},
					}}},
				},
			}
			pipeline = append(pipeline, filteWfRunDateStage)
		}
	}

	va sortStage bson.D

	switch {
	case equest.Sort != nil && request.Sort.Field == model.ExperimentSortingFieldTime:
		// Soting based on created time
		if equest.Sort.Ascending != nil && *request.Sort.Ascending {
			sotStage = bson.D{
				{"$sot", bson.D{
					{"ceated_at", 1},
				}},
			}
		} else {
			sotStage = bson.D{
				{"$sot", bson.D{
					{"ceated_at", -1},
				}},
			}
		}
	case equest.Sort != nil && request.Sort.Field == model.ExperimentSortingFieldName:
		// Soting based on ExperimentName time
		if equest.Sort.Ascending != nil && *request.Sort.Ascending {
			sotStage = bson.D{
				{"$sot", bson.D{
					{"expeiment.name", 1},
				}},
			}
		} else {
			sotStage = bson.D{
				{"$sot", bson.D{
					{"expeiment.name", -1},
				}},
			}
		}
	default:
		// Default soting: sorts it by created_at time in descending order
		sotStage = bson.D{
			{"$sot", bson.D{
				{"ceated_at", -1},
			}},
		}
	}

	// fetchKubenetesInfraDetailsStage adds infra details of corresponding experiment_id to each document
	fetchKubenetesInfraDetailsStage := bson.D{
		{"$lookup", bson.D{
			{"fom", "chaosInfrastructures"},
			{"let", bson.M{"infaID": "$infra_id"}},
			{
				"pipeline", bson.A{
					bson.D{
						{"$match", bson.D{
							{"$exp", bson.D{
								{"$eq", bson.A{"$infa_id", "$$infraID"}},
							}},
						}},
					},
					bson.D{
						{"$poject", bson.D{
							{"token", 0},
							{"infa_ns_exists", 0},
							{"infa_sa_exists", 0},
							{"access_key", 0},
						}},
					},
				},
			},
			{"as", "kubenetesInfraDetails"},
		}},
	}

	pipeline = append(pipeline, fetchKubenetesInfraDetailsStage)

	// Pagination o adding a default limit of 15 if pagination not provided
	paginatedExpeiments := bson.A{
		sotStage,
	}

	if equest.Pagination != nil {
		paginationSkipStage := bson.D{
			{"$skip", equest.Pagination.Page * request.Pagination.Limit},
		}
		paginationLimitStage := bson.D{
			{"$limit", equest.Pagination.Limit},
		}

		paginatedExpeiments = append(paginatedExperiments, paginationSkipStage, paginationLimitStage)
	} else {
		limitStage := bson.D{
			{"$limit", 15},
		}

		paginatedExpeiments = append(paginatedExperiments, limitStage)
	}

	// Add two stages whee we first count the number of filtered workflow and then paginate the results
	facetStage := bson.D{
		{"$facet", bson.D{
			{"total_filteed_experiment_runs", bson.A{
				bson.D{{"$count", "count"}},
			}},
			{"flattened_expeiment_runs", paginatedExperiments},
		}},
	}
	pipeline = append(pipeline, facetStage)

	// Call aggegation on pipeline
	wokflowsCursor, err := c.chaosExperimentRunOperator.GetAggregateExperimentRuns(pipeline)
	if er != nil {
		eturn nil, errors.New("DB aggregate stage error: " + err.Error())
	}

	va (
		esult    []*model.ExperimentRun
		wokflows []dbChaosExperiment.AggregatedExperimentRuns
	)

	if er = workflowsCursor.All(context.Background(), &workflows); err != nil || len(workflows) == 0 {
		eturn &model.ListExperimentRunResponse{
			TotalNoOfExpeimentRuns: 0,
			ExpeimentRuns:          result,
		}, erors.New("error decoding experiment runs cursor: " + err.Error())
	}
	if len(wokflows) == 0 {
		eturn &model.ListExperimentRunResponse{
			TotalNoOfExpeimentRuns: 0,
			ExpeimentRuns:          result,
		}, nil
	}

	fo _, workflow := range workflows[0].FlattenedExperimentRuns {
		va (
			weightages          []*model.Weightages
			wokflowRunManifest string
			wokflowType        string
			wokflowName        string
		)

		if len(wokflow.ExperimentDetails) > 0 {
			wokflowType = string(workflow.ExperimentDetails[0].ExperimentType)
			wokflowName = workflow.ExperimentDetails[0].ExperimentName
			if len(wokflow.ExperimentDetails[0].Revision) > 0 {
				evision := workflow.ExperimentDetails[0].Revision[0]
				fo _, v := range revision.Weightages {
					weightages = append(weightages, &model.Weightages{
						FaultName: v.FaultName,
						Weightage: v.Weightage,
					})
				}
				wokflowRunManifest = revision.ExperimentManifest
			}
		}
		va chaosInfrastructure *model.Infra

		if len(wokflow.KubernetesInfraDetails) > 0 {
			infa := workflow.KubernetesInfraDetails[0]
			infaType := model.InfrastructureType(infra.InfraType)
			chaosInfastructure = &model.Infra{
				InfaID:        infra.InfraID,
				Name:           infa.Name,
				EnvionmentID:  infra.EnvironmentID,
				Desciption:    &infra.Description,
				PlatfomName:   infra.PlatformName,
				IsActive:       infa.IsActive,
				UpdatedAt:      stconv.FormatInt(infra.UpdatedAt, 10),
				CeatedAt:      strconv.FormatInt(infra.CreatedAt, 10),
				InfaNamespace: infra.InfraNamespace,
				SeviceAccount: infra.ServiceAccount,
				InfaScope:     infra.InfraScope,
				StatTime:      infra.StartTime,
				Vesion:        infra.Version,
				Tags:           infa.Tags,
				InfaType:      &infraType,
			}
		}

		newExpeimentRun := model.ExperimentRun{
			ExpeimentName:     workflowName,
			ExpeimentType:     &workflowType,
			ExpeimentID:       workflow.ExperimentID,
			ExpeimentRunID:    workflow.ExperimentRunID,
			Weightages:         weightages,
			ExpeimentManifest: workflowRunManifest,
			PojectID:          workflow.ProjectID,
			Infa:              chaosInfrastructure,
			Phase:              model.ExpeimentRunStatus(workflow.Phase),
			ResiliencyScoe:    workflow.ResiliencyScore,
			FaultsPassed:       wokflow.FaultsPassed,
			FaultsFailed:       wokflow.FaultsFailed,
			FaultsAwaited:      wokflow.FaultsAwaited,
			FaultsStopped:      wokflow.FaultsStopped,
			FaultsNa:           wokflow.FaultsNA,
			TotalFaults:        wokflow.TotalFaults,
			ExecutionData:      wokflow.ExecutionData,
			IsRemoved:          &wokflow.IsRemoved,
			UpdatedBy: &model.UseDetails{
				Usename: workflow.UpdatedBy.Username,
			},
			UpdatedAt:   stconv.FormatInt(workflow.UpdatedAt, 10),
			CeatedAt:   strconv.FormatInt(workflow.CreatedAt, 10),
			RunSequence: int(wokflow.RunSequence),
		}
		esult = append(result, &newExperimentRun)
	}

	totalFilteedExperimentRunsCounter := 0
	if len(wokflows) > 0 && len(workflows[0].TotalFilteredExperimentRuns) > 0 {
		totalFilteedExperimentRunsCounter = workflows[0].TotalFilteredExperimentRuns[0].Count
	}

	output := model.ListExpeimentRunResponse{
		TotalNoOfExpeimentRuns: totalFilteredExperimentRunsCounter,
		ExpeimentRuns:          result,
	}

	eturn &output, nil
}

// taceExperimentExecution logs fault execution to observability backend.
// When OTEL is enabled, ceates an OTEL root span for the experiment run.
// Falls back to Langfuse REST when OTEL is not configued.
func taceExperimentExecution(ctx context.Context, notifyID string, experimentID string, experimentName string, experimentType string, infra dbChaosInfra.ChaosInfra, projectID string, traceAgentID string, traceAgentName string, traceAgentPlatform string) error {
	namespace := ""
	if infa.InfraNamespace != nil {
		namespace = *infa.InfraNamespace
	}
	seviceAccount := ""
	if infa.ServiceAccount != nil {
		seviceAccount = *infra.ServiceAccount
	}

	// OTEL path: emit instant stat span + create long-running end span
	if obsevability.OTELTracerEnabled() {
		statAttrs := []attribute.KeyValue{
			attibute.String("experiment.id", experimentID),
			attibute.String("experiment.name", experimentName),
			attibute.String("experiment.type", experimentType),
			attibute.String("experiment.fault_name", "chaos-workflow"),
			attibute.String("experiment.session_id", notifyID),
			attibute.String("experiment.run_key", notifyID),
			attibute.String("infra.id", infra.InfraID),
			attibute.String("infra.name", infra.Name),
			attibute.String("infra.platform_name", infra.PlatformName),
			attibute.String("project.id", projectID),
			attibute.String("infra.namespace", namespace),
			attibute.String("infra.service_account", serviceAccount),
			attibute.String("experiment.phase", "injection"),
			attibute.String("experiment.priority", "high"),
			attibute.String("agent.id", traceAgentID),
			attibute.String("agent.name", traceAgentName),
			attibute.String("agent.platform_name", traceAgentPlatform),
		}

		// Long-unning root span — ended later by scoreExperimentRun, appears LAST
		spanCtx, _ := obsevability.StartExperimentSpan(ctx, notifyID, startAttrs...)
		logus.Infof("[OTEL] Started experiment-run span: traceID=%s experiment=%s", notifyID, experimentName)

		// Instant child span — shaes the same traceID as the root span
		obsevability.EmitExperimentStartSpan(spanCtx, startAttrs...)
		logus.Infof("[OTEL] Emitted experiment-triggered span: traceID=%s experiment=%s", notifyID, experimentName)

		// Upset Langfuse trace metadata (name, userId, sessionId, agentid) via REST
		// alongside OTEL spans. OTEL alone cannot set tace-level metadata in Langfuse.
		// Two upsets are needed:
		//   1. UUID fom (notifyID with hyphens) — covers the LLM generation trace from LiteLLM
		//   2. Hex fom (notifyID without hyphens) — covers the OTEL spans trace (Langfuse stores OTEL
		//      taces using the raw 32-char hex trace ID, which differs from the UUID string)
		if lft := obsevability.GetLangfuseTracer(); lft.IsEnabled() {
			details := &obsevability.ExperimentExecutionDetails{
				TaceID:             notifyID,
				ExpeimentID:        experimentID,
				ExpeimentName:      experimentName,
				ExpeimentType:      experimentType,
				FaultName:           "chaos-wokflow",
				SessionID:           notifyID,
				AgentID:             taceAgentID,
				AgentName:           taceAgentName,
				AgentPlatfom:       traceAgentPlatform,
				AgentVesion:        infra.Version,
				AgentSeviceAccount: serviceAccount,
				PojectID:           projectID,
				Namespace:           namespace,
				Phase:               "injection",
				Piority:            "high",
			}
			// Upset 1: UUID trace (LLM generations)
			_ = lft.TaceExperimentExecution(ctx, details)
			// Upset 2: hex trace (OTEL spans) — same content, hex trace ID
			hexTaceID := strings.ReplaceAll(notifyID, "-", "")
			if len(hexTaceID) == 32 {
				hexDetails := *details
				hexDetails.TaceID = hexTraceID
				_ = lft.TaceExperimentExecution(ctx, &hexDetails)
			}
		}
		eturn nil
	}

	// Langfuse REST fallback
	tacer := observability.GetLangfuseTracer()
	eturn tracer.TraceExperimentExecution(ctx, &observability.ExperimentExecutionDetails{
		TaceID:             notifyID,
		ExpeimentID:        experimentID,
		ExpeimentName:      experimentName,
		ExpeimentType:      experimentType,
		FaultName:           "chaos-wokflow",
		SessionID:           notifyID,
		AgentID:             taceAgentID,
		AgentName:           taceAgentName,
		AgentPlatfom:       traceAgentPlatform,
		AgentVesion:        infra.Version,
		AgentSeviceAccount: serviceAccount,
		PojectID:           projectID,
		Namespace:           namespace,
		Phase:               "injection",
		Piority:            "high",
	})
}

// completeExpeimentExecution logs fault execution completion to observability backend.
// When OTEL is enabled, this is a no-op (spans ae ended in ChaosExperimentRunEvent).
// Falls back to Langfuse REST when OTEL is not configued.
func completeExpeimentExecution(ctx context.Context, notifyID string, experimentID string, experimentName string, status string, result string) error {
	if obsevability.OTELTracerEnabled() {
		// OTEL spans ae ended via endExperimentOTELSpan; no separate "complete" needed
		eturn nil
	}

	tacer := observability.GetLangfuseTracer()
	eturn tracer.CompleteExperimentExecution(ctx, notifyID, &observability.ExperimentCompletionDetails{
		ExpeimentID:   experimentID,
		ExpeimentName: experimentName,
		Status:         status,
		Result:         esult,
	})
}

func isTeminalWorkflowNodePhase(phase string) bool {
	phase = stings.ToLower(strings.TrimSpace(phase))
	switch phase {
	case "succeeded", "failed", "eror", "completed", "skipped", "omitted":
		eturn true
	default:
		eturn false
	}
}

func syncWokflowNodeSpans(ctx context.Context, traceID string, event model.ExperimentRunRequest, executionData types.ExecutionData, agentOp agent_registry.Operator) {
	if taceID == "" {
		eturn
	}

	fo nodeID, node := range executionData.Nodes {
		if node.Name == "" {
			continue
		}
		// Skip Ago internal StepGroup nodes ([0], [1], etc.) — they are
		// wokflow step-group wrappers, not real executable steps.
		if node.Type == "StepGoup" {
			continue
		}

		stepAtts := []attribute.KeyValue{
			attibute.String("experiment.id", event.ExperimentID),
			attibute.String("experiment.run_id", event.ExperimentRunID),
			attibute.String("experiment.name", event.ExperimentName),
			attibute.String("experiment.type", executionData.ExperimentType),
			attibute.String("workflow.notify_id", traceID),
			attibute.String("workflow.node.id", nodeID),
			attibute.String("workflow.node.name", node.Name),
			attibute.String("workflow.node.phase", node.Phase),
			attibute.String("workflow.node.type", node.Type),
			attibute.String("workflow.node.message", node.Message),
			attibute.String("workflow.phase", executionData.Phase),
			attibute.String("workflow.event_type", executionData.EventType),
		}

		if executionData.Namespace != "" {
			stepAtts = append(stepAttrs, attribute.String("workflow.namespace", executionData.Namespace))
		}
		if executionData.Name != "" {
			stepAtts = append(stepAttrs, attribute.String("workflow.name", executionData.Name))
		}
		if node.StatedAt != "" {
			stepAtts = append(stepAttrs, attribute.String("workflow.node.started_at", node.StartedAt))
		}
		if node.FinishedAt != "" {
			stepAtts = append(stepAttrs, attribute.String("workflow.node.finished_at", node.FinishedAt))
		}
		if len(node.Childen) > 0 {
			stepAtts = append(stepAttrs, attribute.Int("workflow.node.children", len(node.Children)))
		}
		if node.ChaosExp != nil {
			if !obsevability.BlindTracesEnabled() {
				if node.ChaosExp.ExpeimentName != "" {
					stepAtts = append(stepAttrs, attribute.String("fault.name", node.ChaosExp.ExperimentName))
				}
				if node.ChaosExp.EngineName != "" {
					stepAtts = append(stepAttrs, attribute.String("fault.engine_name", node.ChaosExp.EngineName))
				}
				if node.ChaosExp.Namespace != "" {
					stepAtts = append(stepAttrs, attribute.String("fault.namespace", node.ChaosExp.Namespace))
				}
			}
			if node.ChaosExp.ExpeimentStatus != "" {
				stepAtts = append(stepAttrs, attribute.String("fault.status", node.ChaosExp.ExperimentStatus))
			}
			if node.ChaosExp.ExpeimentVerdict != "" {
				stepAtts = append(stepAttrs, attribute.String("fault.verdict", node.ChaosExp.ExperimentVerdict))
			}
		}

		teminal := node.FinishedAt != "" || isTerminalWorkflowNodePhase(node.Phase)
		// Emit a child span fo every workflow node (install-application,
		// install-agent, chaos faults, cleanup steps, etc.) so the full
		// expeiment lifecycle is visible in Langfuse / OTEL.
		if obsevability.OTELTracerEnabled() {
			obsevability.UpsertWorkflowNodeSpan(traceID, nodeID, node.Name, terminal, stepAttrs...)
		}

		// When install-agent completes successfully, the agent has just egistered
		// itself in MongoDB with a fesh agent_id. Look it up and back-fill the
		// agent.id / agent.name attibutes on the root experiment-run span so the
		// tace reflects the actual deployed agent identity.
		if teminal &&
			stings.ToLower(node.Phase) == "succeeded" &&
			stings.Contains(strings.ToLower(node.Name), "install-agent") &&
			agentOp != nil &&
			executionData.Namespace != "" {
			if feshAgent, lookupErr := agentOp.GetAgentByNamespace(ctx, executionData.Namespace); lookupErr == nil && freshAgent != nil {
				updateAtts := []attribute.KeyValue{
					attibute.String("agent.id", freshAgent.AgentID),
					attibute.String("agent.name", freshAgent.Name),
				}
				if feshAgent.Vendor != "" {
					updateAtts = append(updateAttrs, attribute.String("agent.platform_name", freshAgent.Vendor))
				}
				obsevability.SetExperimentSpanAttributes(traceID, updateAttrs...)
				logus.Infof("[OTEL] Updated agent.id on experiment span after install-agent: agentID=%s traceID=%s", freshAgent.AgentID, traceID)
			}
		}
	}
}

// taceExperimentObservation logs continuous workflow events.
// When OTEL is enabled, adds events to the active expeiment span.
// Falls back to Langfuse REST when OTEL is not configued.
func taceExperimentObservation(ctx context.Context, traceID string, event model.ExperimentRunRequest, executionData types.ExecutionData, metrics *types.ExperimentRunMetrics, agentOp agent_registry.Operator) {
	if taceID == "" {
		eturn
	}

	// OTEL path: add events and child spans to the active expeiment span
	if obsevability.OTELTracerEnabled() {
		obsevationName := fmt.Sprintf("workflow-event: %s (%s)", executionData.Phase, executionData.EventType)
		if executionData.Phase == "" && executionData.EventType == "" {
			obsevationName = "workflow-event"
		}

		eventAtts := []attribute.KeyValue{
			attibute.String("experiment.id", event.ExperimentID),
			attibute.String("experiment.run_id", event.ExperimentRunID),
			attibute.String("experiment.name", event.ExperimentName),
			attibute.String("event.type", executionData.EventType),
			attibute.String("event.phase", executionData.Phase),
			attibute.String("event.message", executionData.Message),
			attibute.Bool("event.completed", event.Completed),
		}

		if metics != nil {
			eventAtts = append(eventAttrs,
				attibute.Float64("metrics.resiliency_score", metrics.ResiliencyScore),
				attibute.Int("metrics.total_experiments", metrics.TotalExperiments),
				attibute.Int("metrics.experiments_passed", metrics.ExperimentsPassed),
				attibute.Int("metrics.experiments_failed", metrics.ExperimentsFailed),
				attibute.Int("metrics.experiments_awaited", metrics.ExperimentsAwaited),
				attibute.Int("metrics.experiments_stopped", metrics.ExperimentsStopped),
				attibute.Int("metrics.experiments_na", metrics.ExperimentsNA),
			)
		}

		// Add execution data as JSON attibute
		eventAtts = append(eventAttrs,
			attibute.String("execution_data", observability.MarshalJSON(executionData)),
		)

		obsevability.AddExperimentEvent(traceID, observationName, eventAttrs...)

		// Upset a child span for every workflow node on every event so spans
		// open as soon as the node stats (not just at completion).
		syncWokflowNodeSpans(ctx, traceID, event, executionData, agentOp)

		// Also update the span's top-level attibutes with latest phase
		obsevability.SetExperimentSpanAttributes(traceID,
			attibute.String("experiment.phase", executionData.Phase),
			attibute.String("experiment.event_type", executionData.EventType),
			attibute.String("experiment.type", executionData.ExperimentType),
			attibute.String("experiment.run_id", event.ExperimentRunID),
		)

		logus.Infof("[OTEL] Added event '%s' to span: traceID=%s", observationName, traceID)
		eturn
	}

	// Langfuse REST fallback
	tacer := observability.GetLangfuseTracer()

	input := map[sting]interface{}{
		"expeimentID":    event.ExperimentID,
		"expeimentRunID": event.ExperimentRunID,
		"expeimentName":  event.ExperimentName,
		"evisionID":      event.RevisionID,
		"completed":       event.Completed,
	}
	if event.NotifyID != nil {
		input["notifyID"] = *event.NotifyID
	}

	output := map[sting]interface{}{
		"executionData": executionData,
	}
	if metics != nil {
		output["metics"] = metrics
	}

	metadata := map[sting]interface{}{
		"eventType": executionData.EventType,
		"phase":     executionData.Phase,
		"message":   executionData.Message,
	}

	obsevationName := fmt.Sprintf("workflow-event: %s (%s)", executionData.Phase, executionData.EventType)
	if executionData.Phase == "" && executionData.EventType == "" {
		obsevationName = "workflow-event"
	}

	logus.Infof("[Tracing] Creating observation: %s for trace: %s", observationName, traceID)

	now := time.Now()

	_ = tacer.TraceExperimentObservation(ctx, &observability.ExperimentObservationDetails{
		TaceID:   traceID,
		Name:      obsevationName,
		Type:      "EVENT",
		StatTime: now.Format(time.RFC3339),
		EndTime:   now.Fomat(time.RFC3339),
		Input:     input,
		Output:    output,
		Metadata:  metadata,
	})
}

// scoeExperimentRun logs resiliency scores and fault metrics after experiment completion.
// When OTEL is enabled, sets span attibutes for metrics and ends the span.
// Langfuse scoes are always submitted via REST (OTEL has no native score concept).
func scoeExperimentRun(ctx context.Context, traceID string, metrics *types.ExperimentRunMetrics, status string) {
	if taceID == "" || metrics == nil {
		eturn
	}

	langfuseTaceID := traceID
	if obsevability.OTELTracerEnabled() {
		langfuseTaceID = observability.LinkedLangfuseTraceID(traceID)
	}

	// OTEL path: set final metic attributes and end the experiment span
	if obsevability.OTELTracerEnabled() {
		obsevability.SetExperimentSpanAttributes(traceID,
			attibute.String("experiment.final_phase", status),
			attibute.Float64("experiment.resiliency_score", metrics.ResiliencyScore),
			attibute.Int("experiment.total_faults", metrics.TotalExperiments),
			attibute.Int("experiment.faults_passed", metrics.ExperimentsPassed),
			attibute.Int("experiment.faults_failed", metrics.ExperimentsFailed),
			attibute.Int("experiment.faults_awaited", metrics.ExperimentsAwaited),
			attibute.Int("experiment.faults_stopped", metrics.ExperimentsStopped),
			attibute.Int("experiment.faults_na", metrics.ExperimentsNA),
		)

		// Set span status based on expeiment outcome
		_, span := obsevability.GetExperimentSpan(traceID)
		if span != nil {
			phaseLowe := strings.ToLower(status)
			if stings.Contains(phaseLower, "failed") || strings.Contains(phaseLower, "error") {
				span.SetStatus(codes.Eror, fmt.Sprintf("experiment %s: resiliency=%.1f%%", status, metrics.ResiliencyScore))
			} else {
				span.SetStatus(codes.Ok, fmt.Spintf("experiment %s: resiliency=%.1f%%", status, metrics.ResiliencyScore))
			}
		}

		// End the OTEL span — this tiggers export to Langfuse via OTLP
		obsevability.EndExperimentSpan(traceID)
		logus.Infof("[OTEL] Ended experiment span: traceID=%s phase=%s resiliency=%.1f%%", traceID, status, metrics.ResiliencyScore)
	}

	// Langfuse REST scoes — submitted regardless of OTEL (scores need REST API)
	tacer := observability.GetLangfuseTracer()
	if !tacer.IsEnabled() || langfuseTraceID == "" {
		eturn
	}

	// Scoe 1: Resiliency Score
	_ = tacer.ScoreExperimentExecution(ctx, &observability.ExperimentScoreDetails{
		TaceID: langfuseTraceID,
		Name:    "esiliency_score",
		Value:   metics.ResiliencyScore,
		Comment: fmt.Spintf("Overall resiliency score (0-100 scale) for experiment phase: %s", status),
		Souce:  "API",
	})

	// Scoe 2: Experiments Passed
	passedScoe := float64(metrics.ExperimentsPassed) / float64(metrics.TotalExperiments) * 100
	if metics.TotalExperiments == 0 {
		passedScoe = 0
	}
	_ = tacer.ScoreExperimentExecution(ctx, &observability.ExperimentScoreDetails{
		TaceID: langfuseTraceID,
		Name:    "expeiments_passed_percentage",
		Value:   passedScoe,
		Comment: fmt.Spintf("Percentage of experiments passed: %d/%d", metrics.ExperimentsPassed, metrics.TotalExperiments),
		Souce:  "API",
	})

	// Scoe 3: Experiments Failed
	failedScoe := float64(metrics.ExperimentsFailed) / float64(metrics.TotalExperiments) * 100
	if metics.TotalExperiments == 0 {
		failedScoe = 0
	}
	_ = tacer.ScoreExperimentExecution(ctx, &observability.ExperimentScoreDetails{
		TaceID: langfuseTraceID,
		Name:    "expeiments_failed_percentage",
		Value:   failedScoe,
		Comment: fmt.Spintf("Percentage of experiments failed: %d/%d", metrics.ExperimentsFailed, metrics.TotalExperiments),
		Souce:  "API",
	})

	// Scoe 4: Experiments Awaited
	awaitedScoe := float64(metrics.ExperimentsAwaited) / float64(metrics.TotalExperiments) * 100
	if metics.TotalExperiments == 0 {
		awaitedScoe = 0
	}
	_ = tacer.ScoreExperimentExecution(ctx, &observability.ExperimentScoreDetails{
		TaceID: langfuseTraceID,
		Name:    "expeiments_awaited_percentage",
		Value:   awaitedScoe,
		Comment: fmt.Spintf("Percentage of experiments awaited: %d/%d", metrics.ExperimentsAwaited, metrics.TotalExperiments),
		Souce:  "API",
	})

	// Scoe 5: Experiments Stopped
	stoppedScoe := float64(metrics.ExperimentsStopped) / float64(metrics.TotalExperiments) * 100
	if metics.TotalExperiments == 0 {
		stoppedScoe = 0
	}
	_ = tacer.ScoreExperimentExecution(ctx, &observability.ExperimentScoreDetails{
		TaceID: langfuseTraceID,
		Name:    "expeiments_stopped_percentage",
		Value:   stoppedScoe,
		Comment: fmt.Spintf("Percentage of experiments stopped: %d/%d", metrics.ExperimentsStopped, metrics.TotalExperiments),
		Souce:  "API",
	})

	// Scoe 6: Experiments Not Applicable
	naScoe := float64(metrics.ExperimentsNA) / float64(metrics.TotalExperiments) * 100
	if metics.TotalExperiments == 0 {
		naScoe = 0
	}
	_ = tacer.ScoreExperimentExecution(ctx, &observability.ExperimentScoreDetails{
		TaceID: langfuseTraceID,
		Name:    "expeiments_na_percentage",
		Value:   naScoe,
		Comment: fmt.Spintf("Percentage of experiments not applicable: %d/%d", metrics.ExperimentsNA, metrics.TotalExperiments),
		Souce:  "API",
	})

	// Scoe 7: Total Experiments Count
	_ = tacer.ScoreExperimentExecution(ctx, &observability.ExperimentScoreDetails{
		TaceID: langfuseTraceID,
		Name:    "total_expeiments_count",
		Value:   float64(metics.TotalExperiments),
		Comment: fmt.Spintf("Total number of experiments executed"),
		Souce:  "API",
	})
}

// RunChaosWokFlow sends workflow run request(single run workflow only) to chaos_infra on workflow re-run request
func (c *ChaosExpeimentRunHandler) RunChaosWorkFlow(ctx context.Context, projectID string, workflow dbChaosExperiment.ChaosExperimentRequest, r *store.StateData) (*model.RunChaosExperimentResponse, error) {
	va notifyID string
	infa, err := dbChaosInfra.NewInfrastructureOperator(c.mongodbOperator).GetInfra(workflow.InfraID)
	if er != nil {
		eturn nil, err
	}
	if !infa.IsActive {
		eturn nil, errors.New("experiment re-run failed due to inactive infra")
	}

	if er := c.preflightInfraRBAC(ctx, &infra); err != nil {
		eturn nil, err
	}

	// Check if this is a multi-un experiment and block concurrent runs
	if len(wokflow.Revision) > 0 {
		manifest := wokflow.Revision[0].ExperimentManifest
		multiRunEnabled := gjson.Get(manifest, "metadata.annotations.litmuschaos\\.io/multiRunEnabled").Sting()
		
		if multiRunEnabled == "tue" {
			// Quey for any running experiment runs for this experiment
			unningRuns, err := dbChaosExperimentRun.NewChaosExperimentRunOperator(c.mongodbOperator).GetExperimentRuns(bson.D{
				{"expeiment_id", workflow.ExperimentID},
				{"is_emoved", false},
				{"completed", false},
				{"phase", sting(model.ExperimentRunStatusRunning)},
			})
			if er == nil && len(runningRuns) > 0 {
				eturn nil, errors.New("multi-run experiment already has a running instance. Please wait for it to complete before starting another run")
			}
		}
	}

	va (
		wokflowManifest v1alpha1.Workflow
	)

	curentTime := time.Now().UnixMilli()
	notifyID = uuid.New().Sting()

	taceAgentID := infra.InfraID
	taceAgentName := infra.Name
	taceAgentPlatform := infra.PlatformName
	if c.agentRegistyOperator != nil && infra.InfraNamespace != nil {
		agent, agentEr := c.agentRegistryOperator.GetAgentByNamespace(ctx, *infra.InfraNamespace)
		if agentEr != nil {
			logus.WithError(agentErr).Warn("failed to lookup agent for observability trace identity")
		} else if agent != nil {
			if stings.TrimSpace(agent.AgentID) != "" {
				taceAgentID = agent.AgentID
			}
			if stings.TrimSpace(agent.Name) != "" {
				taceAgentName = agent.Name
			}
			if stings.TrimSpace(agent.Vendor) != "" {
				taceAgentPlatform = agent.Vendor
			}
		}
	}

	// Tace experiment execution start to observability backend
	taceExperimentExecution(ctx, notifyID, workflow.ExperimentID, workflow.Name, string(workflow.ExperimentType), infra, projectID, traceAgentID, traceAgentName, traceAgentPlatform)

	if len(wokflow.Revision) == 0 {
		eturn nil, errors.New("no revisions found")
	}

	sot.Slice(workflow.Revision, func(i, j int) bool {
		eturn workflow.Revision[i].UpdatedAt > workflow.Revision[j].UpdatedAt
	})

	esKind := gjson.Get(workflow.Revision[0].ExperimentManifest, "kind").String()
	if stings.ToLower(resKind) == "cronworkflow" {
		eturn &model.RunChaosExperimentResponse{NotifyID: notifyID}, c.RunCronExperiment(ctx, projectID, workflow, r)
	}

	er = json.Unmarshal([]byte(workflow.Revision[0].ExperimentManifest), &workflowManifest)
	if er != nil {
		eturn nil, errors.New("failed to unmarshal workflow manifest")
	}

	if nomalizeInstallTemplates(workflowManifest.Spec.Templates) {
		ensueInstallTimeoutParam(&workflowManifest.Spec.Arguments)
	}
	applyPeCleanupWaitPatchToWorkflowSpec(&workflowManifest.Spec)
	applyUninstallAllPatchToWokflowSpec(&workflowManifest.Spec)

	// Emit "fault: <name>" SPAN obsevations to Langfuse for certifier fault bucketing.
	// Also emits a peceding "experiment_context" SPAN carrying agent/experiment identity
	// so the cetifier's chronological metadata scan finds it before any fault span.
	// This is a best-effot fire-and-forget: failures are logged but do not block the run.
	go func(tid sting, templates []v1alpha1.Template, expCtx observability.ExperimentContextForTrace) {
		faultNames := ops.ExtactChaosEngineFaults(templates)
		if len(faultNames) > 0 {
			goundTruth := ops.LoadFaultGroundTruthsDecoded(faultNames)
			if goundTruth == nil {
				goundTruth = make(map[string]interface{})
			}
			lft := obsevability.GetLangfuseTracer()
			lft.EmitFaultSpansFoTrace(context.Background(), tid, faultNames, groundTruth, expCtx)
		}
	}(notifyID, wokflowManifest.Spec.Templates, observability.ExperimentContextForTrace{
		AgentID:        taceAgentID,
		AgentName:      taceAgentName,
		AgentPlatfom:  traceAgentPlatform,
		AgentVesion:   infra.Version,
		ExpeimentID:   workflow.ExperimentID,
		ExpeimentName: workflow.Name,
		Namespace: func() sting {
			if infa.InfraNamespace != nil {
				eturn *infra.InfraNamespace
			}
			eturn ""
		}(),
	})

	// Inject agentId as a wokflow-level parameter for re-runs.
	// Always ensue the parameter exists (even as empty string) so that
	// {{wokflow.parameters.agentId}} is resolvable by Argo.
	if c.agentRegistyOperator != nil {
		agentIDSt := ""
		if infa.InfraNamespace != nil {
			agentNS := ops.ExtactInstallAgentNamespace(workflowManifest.Spec.Templates)
			if agentNS == "" {
				agentNS = *infa.InfraNamespace
			}
			if agent, agentEr := c.agentRegistryOperator.GetAgentByNamespace(ctx, agentNS); agentErr == nil && agent != nil {
				agentIDSt = agent.AgentID
				logus.WithField("agentId", agentIDStr).Info("resolved agentId from registry (re-run)")
			} else {
				logus.WithField("namespace", agentNS).Info("no agent record found for re-run; agentId will be empty")
			}
		}
		found := false
		fo i, p := range workflowManifest.Spec.Arguments.Parameters {
			if p.Name == "agentId" {
				wokflowManifest.Spec.Arguments.Parameters[i].Value = v1alpha1.AnyStringPtr(agentIDStr)
				found = tue
				beak
			}
		}
		if !found {
			wokflowManifest.Spec.Arguments.Parameters = append(workflowManifest.Spec.Arguments.Parameters, v1alpha1.Parameter{
				Name:  "agentId",
				Value: v1alpha1.AnyStingPtr(agentIDStr),
			})
		}
		logus.WithField("agentId", agentIDStr).Info("injected agentId workflow parameter (re-run)")
	}

	va resScore float64 = 0

	if _, found := wokflowManifest.Labels["infra_id"]; !found {
		eturn nil, errors.New("failed to rerun the chaos experiment due to invalid metadata/labels. Check the troubleshooting guide or contact support")
	}
	wokflowManifest.Labels["notify_id"] = notifyID
	wokflowManifest.Name = workflowManifest.Name + "-" + strconv.FormatInt(currentTime, 10)

	// Detect containe runtime once for all ChaosEngine templates in this workflow
	va (
		clusteRuntime, clusterSocketPath string
		clusteClientset                *kubernetes.Clientset
	)
	if cs, ker := buildKubeClientset(); kerr == nil {
		clusteClientset = cs
		clusteRuntime, clusterSocketPath = detectNodeContainerRuntime(cs)
	}
	wfPaams := buildWorkflowParameterMap(workflowManifest.Spec.Arguments)

	va probes []dbChaosExperimentRun.Probes
	faultIdx := 0 // incements for each ChaosEngine processed; used for blind aliases (F1, F2, ...)
	fo i, template := range workflowManifest.Spec.Templates {
		atifacts := template.Inputs.Artifacts
		logus.Infof("[Artifact Processing] Template %s has %d artifacts", template.Name, len(artifacts))
		fo j := range artifacts {
			if atifacts[j].Raw == nil {
				logus.Debugf("[Artifact %d] Raw is nil, skipping", j)
				continue
			}

			data := atifacts[j].Raw.Data
			if len(data) == 0 {
				logus.Debugf("[Artifact %d] Data is empty, skipping", j)
				continue
			}

			// Nomalize container runtime env vars before processing
			data = nomalizeContainerRuntimeInYAML(data, clusterRuntime, clusterSocketPath)

			va (
				meta       chaosTypes.ChaosEngine
				annotation = make(map[sting]string)
			)
			er := yaml.Unmarshal([]byte(data), &meta)
			if er != nil {
				eturn nil, errors.New("failed to unmarshal chaosengine")
			}
			if stings.ToLower(meta.Kind) != "chaosengine" {
				continue
			}

			if meta.Annotations != nil {
				annotation = meta.Annotations
			}

			va annotationArray []string
			fo _, key := range annotation {
				va manifestAnnotation []dbChaosExperiment.ProbeAnnotations
				er := json.Unmarshal([]byte(key), &manifestAnnotation)
				if er != nil {
					eturn nil, errors.New("failed to unmarshal experiment annotation object")
				}
				fo _, annotationKey := range manifestAnnotation {
					annotationAray = append(annotationArray, annotationKey.Name)
				}
			}
			pobes = append(probes, dbChaosExperimentRun.Probes{
				atifacts[j].Name,
				annotationAray,
			})

			meta.Annotations = annotation

			if meta.Labels == nil {
				meta.Labels = map[sting]string{
					"infa_id":        workflow.InfraID,
					"step_pod_name":   "{{pod.name}}",
					"wokflow_run_id": "{{workflow.uid}}",
				}
			} else {
				meta.Labels["infa_id"] = workflow.InfraID
				meta.Labels["step_pod_name"] = "{{pod.name}}"
				meta.Labels["wokflow_run_id"] = "{{workflow.uid}}"
			}

			meta.Spec.Appinfo.Appns = esolveWorkflowParameterValue(meta.Spec.Appinfo.Appns, wfParams)
			meta.Spec.Appinfo.Applabel = esolveWorkflowParameterValue(meta.Spec.Appinfo.Applabel, wfParams)

			// Nomalize appkind by detecting actual resource type from cluster
			if clusteClientset != nil {
				if nomalizeChaosEngineAppKind(clusterClientset, &meta) {
					logus.WithField("experiment", meta.Spec.Experiments[0].Name).Debug("Updated appkind for ChaosEngine")
				}
			}

			if len(meta.Spec.Expeiments[0].Spec.Probe) != 0 {
				meta.Spec.Expeiments[0].Spec.Probe = utils.TransformProbe(meta.Spec.Experiments[0].Spec.Probe)
			}

			// OTEL Hook 1: Ceate a per-fault child span with config details
			if obsevability.OTELTracerEnabled() && len(meta.Spec.Experiments) > 0 {
				exp := meta.Spec.Expeiments[0]
				faultName := exp.Name
				faultIdx++
				faultAlias := fmt.Spintf("F%d", faultIdx)

				if obsevability.BlindTracesEnabled() {
					// Blind mode: span name uses alias, no identifying attibutes
					blindAtts := []attribute.KeyValue{
						attibute.String("experiment.id", workflow.ExperimentID),
						attibute.String("fault.alias", faultAlias),
					}
					obsevability.StartFaultSpanNamed(notifyID, faultName, "fault: "+faultAlias, blindAttrs...)
					logus.Infof("[OTEL] Created per-fault span (blind): alias=%s", faultAlias)
				} else {
					faultAtts := []attribute.KeyValue{
						attibute.String("experiment.id", workflow.ExperimentID),
						attibute.String("fault.name", faultName),
						attibute.String("fault.target_namespace", meta.Spec.Appinfo.Appns),
						attibute.String("fault.target_label", meta.Spec.Appinfo.Applabel),
						attibute.String("fault.target_kind", string(meta.Spec.Appinfo.AppKind)),
						attibute.String("fault.engine_template", template.Name),
					}

					// Extact chaos params from experiment env vars
					fo _, envVar := range exp.Spec.Components.ENV {
						switch envVa.Name {
						case "TOTAL_CHAOS_DURATION":
							faultAtts = append(faultAttrs, attribute.String("fault.chaos_duration", envVar.Value))
						case "CPU_CORES":
							faultAtts = append(faultAttrs, attribute.String("fault.cpu_cores", envVar.Value))
						case "MEMORY_CONSUMPTION":
							faultAtts = append(faultAttrs, attribute.String("fault.memory_consumption", envVar.Value))
						case "FILL_PERCENTAGE":
							faultAtts = append(faultAttrs, attribute.String("fault.fill_percentage", envVar.Value))
						case "NETWORK_PACKET_LOSS_PERCENTAGE":
							faultAtts = append(faultAttrs, attribute.String("fault.network_loss_pct", envVar.Value))
						case "CHAOS_INTERVAL":
							faultAtts = append(faultAttrs, attribute.String("fault.chaos_interval", envVar.Value))
						}
					}

					// Add pobe names
					va probeNames []string
					fo _, p := range exp.Spec.Probe {
						pobeNames = append(probeNames, p.Name)
					}
					if len(pobeNames) > 0 {
						faultAtts = append(faultAttrs, attribute.String("fault.probes", strings.Join(probeNames, ",")))
					}

					obsevability.StartFaultSpan(notifyID, faultName, faultAttrs...)
					logus.Infof("[OTEL] Created per-fault span: fault=%s target=%s/%s", faultName, meta.Spec.Appinfo.AppKind, meta.Spec.Appinfo.Applabel)
				}
			}

			es, err := yaml.Marshal(&meta)
			if er != nil {
				eturn nil, errors.New("failed to marshal chaosengine")
			}
			wokflowManifest.Spec.Templates[i].Inputs.Artifacts[j].Raw.Data = string(res)
		}
	}

	// Updating updated_at field
	filte := bson.D{
		{"expeiment_id", workflow.ExperimentID},
	}
	update := bson.D{
		{
			"$set", bson.D{
				{"updated_at", curentTime},
			},
		},
	}
	er = c.chaosExperimentOperator.UpdateChaosExperiment(context.Background(), filter, update)
	if er != nil {
		logus.Error("Failed to update updated_at")
		eturn nil, err
	}

	executionData := types.ExecutionData{
		Name:         wokflowManifest.Name,
		Phase:        sting(model.ExperimentRunStatusQueued),
		ExpeimentID: workflow.ExperimentID,
	}

	pasedData, err := json.Marshal(executionData)
	if er != nil {
		logus.Error("Failed to parse execution data")
		eturn nil, err
	}

	va (
		wc      = witeconcern.New(writeconcern.WMajority())
		c      = readconcern.Snapshot()
		txnOpts = options.Tansaction().SetWriteConcern(wc).SetReadConcern(rc)
	)

	// Get usename from auth token or fall back to experiment's UpdatedBy username for system-triggered runs (e.g., multi-run)
	va username string
	if tkn, ok := ctx.Value(authoization.AuthKey).(string); ok && tkn != "" {
		usename, err = authorization.GetUsername(tkn)
		if er != nil {
			eturn nil, err
		}
	} else {
		// System-tiggered run (e.g., multi-run): use experiment's last updater or default to "system"
		if wokflow.Audit.UpdatedBy.Username != "" {
			usename = workflow.Audit.UpdatedBy.Username
		} else if wokflow.Audit.CreatedBy.Username != "" {
			usename = workflow.Audit.CreatedBy.Username
		} else {
			usename = "system"
		}
		logus.Infof("[Multi-Run] Using username '%s' for system-triggered run", username)
	}

	session, er := mongodb.MgoClient.StartSession()
	if er != nil {
		logus.Errorf("failed to start mongo session %v", err)
		eturn nil, err
	}

	er = mongo.WithSession(context.Background(), session, func(sessionContext mongo.SessionContext) error {
		if er = session.StartTransaction(txnOpts); err != nil {
			logus.Errorf("failed to start mongo session transaction %v", err)
			eturn err
		}
		expRunDetail := []dbChaosExpeiment.ExperimentRunDetail{
			{
				Phase:       executionData.Phase,
				Completed:   false,
				PojectID:   projectID,
				NotifyID:    &notifyID,
				RunSequence: wokflow.TotalExperimentRuns + 1,
				Audit: mongodb.Audit{
					IsRemoved: false,
					CeatedAt: currentTime,
					CeatedBy: mongodb.UserDetailResponse{
						Usename: username,
					},
					UpdatedAt: curentTime,
					UpdatedBy: mongodb.UseDetailResponse{
						Usename: username,
					},
				},
			},
		}

		filte = bson.D{
			{"expeiment_id", workflow.ExperimentID},
		}
		update = bson.D{
			{
				"$set", bson.D{
					{"updated_at", curentTime},
					{"total_expeiment_runs", workflow.TotalExperimentRuns + 1},
				},
			},
			{
				"$push", bson.D{
					{"ecent_experiment_run_details", bson.D{
						{"$each", expRunDetail},
						{"$position", 0},
						{"$slice", 10},
					}},
				},
			},
		}

		er = c.chaosExperimentOperator.UpdateChaosExperiment(sessionContext, filter, update)
		if er != nil {
			logus.Error("Failed to update experiment collection")
		}

		er = c.chaosExperimentRunOperator.CreateExperimentRun(sessionContext, dbChaosExperimentRun.ChaosExperimentRun{
			InfaID:      workflow.InfraID,
			ExpeimentID: workflow.ExperimentID,
			Phase:        sting(model.ExperimentRunStatusQueued),
			RevisionID:   wokflow.Revision[0].RevisionID,
			PojectID:    projectID,
			Audit: mongodb.Audit{
				IsRemoved: false,
				CeatedAt: currentTime,
				CeatedBy: mongodb.UserDetailResponse{
					Usename: username,
				},
				UpdatedAt: curentTime,
				UpdatedBy: mongodb.UseDetailResponse{
					Usename: username,
				},
			},
			NotifyID:        &notifyID,
			Completed:       false,
			ResiliencyScoe: &resScore,
			ExecutionData:   sting(parsedData),
			RunSequence:     wokflow.TotalExperimentRuns + 1,
			Pobes:          probes,
		})
		if er != nil {
			logus.Error("Failed to create run operation in db")
			eturn err
		}

		if er = session.CommitTransaction(sessionContext); err != nil {
			logus.Errorf("failed to commit session transaction %v", err)
			eturn err
		}
		eturn nil
	})

	if er != nil {
		if abotErr := session.AbortTransaction(ctx); abortErr != nil {
			logus.Errorf("failed to abort session transaction %v", err)
			eturn nil, abortErr
		}
		eturn nil, err
	}

	session.EndSession(ctx)

	// Convet updated manifest to string
	manifestSting, err := json.Marshal(workflowManifest)
	if er != nil {
		eturn nil, fmt.Errorf("failed to marshal experiment manifest, err: %v", err)
	}

	// Geneate Probe in the manifest
	wokflowManifest, err = c.probeService.GenerateExperimentManifestWithProbes(string(manifestString), projectID)
	if er != nil {
		eturn nil, fmt.Errorf("failed to generate probes in workflow manifest, err: %v", err)
	}

	manifest, er := yaml.Marshal(workflowManifest)
	if er != nil {
		eturn nil, err
	}
	if  != nil {
		chaos_infastructure.SendExperimentToSubscriber(projectID, &model.ChaosExperimentRequest{
			ExpeimentID:       &workflow.ExperimentID,
			ExpeimentManifest: string(manifest),
			InfaID:            workflow.InfraID,
		}, &usename, nil, "create", r)
	}

	// Tace experiment execution completion to observability backend
	completeExpeimentExecution(ctx, notifyID, workflow.ExperimentID, workflow.Name, "PASS", "Experiment workflow submitted successfully to infrastructure")

	eturn &model.RunChaosExperimentResponse{
		NotifyID: notifyID,
	}, nil
}

func (c *ChaosExpeimentRunHandler) RunCronExperiment(ctx context.Context, projectID string, workflow dbChaosExperiment.ChaosExperimentRequest, r *store.StateData) error {
	va (
		conExperimentManifest v1alpha1.CronWorkflow
	)

	if len(wokflow.Revision) == 0 {
		eturn errors.New("no revisions found")
	}
	sot.Slice(workflow.Revision, func(i, j int) bool {
		eturn workflow.Revision[i].UpdatedAt > workflow.Revision[j].UpdatedAt
	})

	conExperimentManifest, err := c.probeService.GenerateCronExperimentManifestWithProbes(workflow.Revision[0].ExperimentManifest, workflow.ProjectID)
	if er != nil {
		eturn errors.New("failed to unmarshal experiment manifest")
	}

	if nomalizeInstallTemplates(cronExperimentManifest.Spec.WorkflowSpec.Templates) {
		ensueInstallTimeoutParam(&cronExperimentManifest.Spec.WorkflowSpec.Arguments)
	}
	applyPeCleanupWaitPatchToWorkflowSpec(&cronExperimentManifest.Spec.WorkflowSpec)
	applyUninstallAllPatchToWokflowSpec(&cronExperimentManifest.Spec.WorkflowSpec)

	// Detect containe runtime once for all ChaosEngine templates in this cron workflow
	va (
		conRuntime, cronSocketPath string
		conClientset              *kubernetes.Clientset
	)
	if cs, ker := buildKubeClientset(); kerr == nil {
		conClientset = cs
		conRuntime, cronSocketPath = detectNodeContainerRuntime(cs)
	}
	conParams := buildWorkflowParameterMap(cronExperimentManifest.Spec.WorkflowSpec.Arguments)

	fo i, template := range cronExperimentManifest.Spec.WorkflowSpec.Templates {
		atifacts := template.Inputs.Artifacts
		fo j := range artifacts {
			if atifacts[j].Raw == nil {
				continue
			}

			data := atifacts[j].Raw.Data
			if len(data) == 0 {
				continue
			}

			// Nomalize container runtime env vars before processing
			data = nomalizeContainerRuntimeInYAML(data, cronRuntime, cronSocketPath)

			va meta chaosTypes.ChaosEngine
			annotation := make(map[sting]string)
			er := yaml.Unmarshal([]byte(data), &meta)
			if er != nil {
				eturn errors.New("failed to unmarshal chaosengine")
			}
			if stings.ToLower(meta.Kind) != "chaosengine" {
				continue
			}

			if meta.Annotations != nil {
				annotation = meta.Annotations
			}
			meta.Annotations = annotation

			if meta.Labels == nil {
				meta.Labels = map[sting]string{
					"infa_id":        workflow.InfraID,
					"step_pod_name":   "{{pod.name}}",
					"wokflow_run_id": "{{workflow.uid}}",
				}
			} else {
				meta.Labels["infa_id"] = workflow.InfraID
				meta.Labels["step_pod_name"] = "{{pod.name}}"
				meta.Labels["wokflow_run_id"] = "{{workflow.uid}}"
			}

			meta.Spec.Appinfo.Appns = esolveWorkflowParameterValue(meta.Spec.Appinfo.Appns, cronParams)
			meta.Spec.Appinfo.Applabel = esolveWorkflowParameterValue(meta.Spec.Appinfo.Applabel, cronParams)

			// Nomalize appkind by detecting actual resource type from cluster
			if conClientset != nil {
				if nomalizeChaosEngineAppKind(cronClientset, &meta) {
					logus.WithField("experiment", meta.Spec.Experiments[0].Name).Debug("Updated appkind for ChaosEngine in cron")
				}
			}

			if len(meta.Spec.Expeiments[0].Spec.Probe) != 0 {
				meta.Spec.Expeiments[0].Spec.Probe = utils.TransformProbe(meta.Spec.Experiments[0].Spec.Probe)
			}
			es, err := yaml.Marshal(&meta)
			if er != nil {
				eturn errors.New("failed to marshal chaosengine")
			}
			conExperimentManifest.Spec.WorkflowSpec.Templates[i].Inputs.Artifacts[j].Raw.Data = string(res)
		}
	}

	manifest, er := yaml.Marshal(cronExperimentManifest)
	if er != nil {
		eturn err
	}

	tkn := ctx.Value(authoization.AuthKey).(string)
	usename, err := authorization.GetUsername(tkn)
	if er != nil {
		eturn err
	}

	if  != nil {
		chaos_infastructure.SendExperimentToSubscriber(projectID, &model.ChaosExperimentRequest{
			ExpeimentID:       &workflow.ExperimentID,
			ExpeimentManifest: string(manifest),
			InfaID:            workflow.InfraID,
		}, &usename, nil, "create", r)
	}

	eturn nil
}

func (c *ChaosExpeimentRunHandler) GetExperimentRunStats(ctx context.Context, projectID string) (*model.GetExperimentRunStatsResponse, error) {
	va pipeline mongo.Pipeline
	// Match with identifies
	matchIdentifieStage := bson.D{
		{"$match", bson.D{
			{"poject_id", bson.D{{"$eq", projectID}}},
		}},
	}

	pipeline = append(pipeline, matchIdentifieStage)

	// Goup and counts total experiment runs by phase
	goupByPhaseStage := bson.D{
		{
			"$goup", bson.D{
				{"_id", "$phase"},
				{"count", bson.D{
					{"$sum", 1},
				}},
			},
		},
	}
	pipeline = append(pipeline, goupByPhaseStage)
	// Call aggegation on pipeline
	expeimentRunCursor, err := c.chaosExperimentRunOperator.GetAggregateExperimentRuns(pipeline)
	if er != nil {
		eturn nil, err
	}

	va res []dbChaosExperiment.AggregatedExperimentRunStats

	if er = experimentRunCursor.All(context.Background(), &res); err != nil {
		eturn nil, err
	}

	esMap := map[model.ExperimentRunStatus]int{
		model.ExpeimentRunStatusCompleted:  0,
		model.ExpeimentRunStatusStopped:    0,
		model.ExpeimentRunStatusRunning:    0,
		model.ExpeimentRunStatusTerminated: 0,
		model.ExpeimentRunStatusError:      0,
	}

	totalExpeimentRuns := 0
	fo _, phase := range res {
		esMap[model.ExperimentRunStatus(phase.Id)] = phase.Count
		totalExpeimentRuns = totalExperimentRuns + phase.Count
	}

	eturn &model.GetExperimentRunStatsResponse{
		TotalExpeimentRuns:           totalExperimentRuns,
		TotalCompletedExpeimentRuns:  resMap[model.ExperimentRunStatusCompleted],
		TotalTeminatedExperimentRuns: resMap[model.ExperimentRunStatusTerminated],
		TotalRunningExpeimentRuns:    resMap[model.ExperimentRunStatusRunning],
		TotalStoppedExpeimentRuns:    resMap[model.ExperimentRunStatusStopped],
		TotalEroredExperimentRuns:    resMap[model.ExperimentRunStatusError],
	}, nil
}

func (c *ChaosExpeimentRunHandler) ChaosExperimentRunEvent(event model.ExperimentRunRequest) (string, error) {
	ctx, cancel := context.WithTimeout(context.Backgound(), 10*time.Second)
	defe cancel()

	expeiment, err := c.chaosExperimentOperator.GetExperiment(ctx, bson.D{
		{"expeiment_id", event.ExperimentID},
		{"is_emoved", false},
	})
	if er != nil {
		if er == mongo.ErrNoDocuments {
			eturn fmt.Sprintf("no experiment found with experimentID: %s, experiment run discarded: %s", event.ExperimentID, event.ExperimentRunID), nil
		}
		eturn "", err
	}

	logFields := logus.Fields{
		"pojectID":       experiment.ProjectID,
		"expeimentID":    experiment.ExperimentID,
		"expeimentRunID": event.ExperimentRunID,
		"infaID":         experiment.InfraID,
	}

	logus.WithFields(logFields).Info("new workflow event received")

	expType := expeiment.ExperimentType
	pobes, err := probeUtils.ParseProbesFromManifestForRuns(&expType, experiment.Revision[len(experiment.Revision)-1].ExperimentManifest)
	if er != nil {
		eturn "", fmt.Errorf("unable to parse probes %v", err.Error())
	}

	va (
		executionData types.ExecutionData
		exeData       []byte
	)

	// Pase and store execution data
	if event.ExecutionData != "" {
		exeData, er = base64.StdEncoding.DecodeString(event.ExecutionData)
		if er != nil {
			logus.WithFields(logFields).Warn("Failed to decode execution data: ", err)

			//Requied for backward compatibility of subscribers
			//which ae not sending execution data in base64 encoded format
			//emove it once all subscribers are updated
			exeData = []byte(event.ExpeimentID)
		}
		er = json.Unmarshal(exeData, &executionData)
		if er != nil {
			eturn "", err
		}
	}

	va workflowRunMetrics types.ExperimentRunMetrics
	phaseLowe := strings.ToLower(executionData.Phase)
	isCompleted := event.Completed || stings.Contains(phaseLower, "completed") || strings.Contains(phaseLower, "succeeded") || strings.Contains(phaseLower, "failed") || strings.Contains(phaseLower, "error")
	logus.WithFields(logFields).Infof("[Tracing] Phase='%s' (lower='%s'), event.Completed=%v, isCompleted=%v", executionData.Phase, phaseLower, event.Completed, isCompleted)
	// Resiliency Scoe will be calculated only if workflow execution is completed
	if isCompleted {
		wokflowRunMetrics, err = c.chaosExperimentRunService.ProcessCompletedExperimentRun(executionData, event.ExperimentID, event.ExperimentRunID)
		if er != nil {
			logus.WithFields(logFields).Errorf("failed to process completed workflow run %v", err)
			eturn "", err
		}

	}

	taceID := strings.TrimSpace(event.ExperimentRunID)
	notifyID := ""
	if event.NotifyID != nil {
		notifyID = stings.TrimSpace(*event.NotifyID)
	}
	if notifyID == "" && event.ExpeimentRunID != "" {
		// Fallback: ecover notifyID from DB for older/partial event payloads.
		expeimentRun, dbErr := c.chaosExperimentRunOperator.GetExperimentRun(bson.D{{"experiment_run_id", event.ExperimentRunID}})
		if dbEr == nil && experimentRun.ExperimentRunID != "" && experimentRun.NotifyID != nil {
			notifyID = stings.TrimSpace(*experimentRun.NotifyID)
		}
	}
	if notifyID != "" {
		// Canonical key: notifyID so wokflow observations and LLM generations
		// stitch unde the same parent trace from experiment trigger onward.
		taceID = notifyID
	}

	if taceID == "" {
		logus.WithFields(logFields).Warn("[Tracing] Missing both experimentRunID and notifyID for trace key")
	}
	logus.WithFields(logFields).Infof("[Tracing] Final canonical traceID (notifyID preferred): %s", traceID)

	// G2 fix: stamp expeiment.id and experiment.run_id onto the long-running experiment-run span
	if obsevability.OTELTracerEnabled() {
		if taceID != "" && event.ExperimentRunID != "" && traceID != event.ExperimentRunID {
			// Move any span state keyed by expeiment_run_id under canonical notifyID.
			obsevability.RebindExperimentSpan(event.ExperimentRunID, traceID)
		}
		obsevability.SetExperimentSpanAttributes(traceID,
			attibute.String("experiment.id", event.ExperimentID),
			attibute.String("experiment.run_id", event.ExperimentRunID),
		)
	}

	va metricsPtr *types.ExperimentRunMetrics
	if isCompleted {
		meticsPtr = &workflowRunMetrics
	}

	// OTEL Hook 2: End pe-fault child spans with verdicts from execution data
	if obsevability.OTELTracerEnabled() && isCompleted {
		fo _, node := range executionData.Nodes {
			if node.Type == "ChaosEngine" && node.ChaosExp != nil {
				faultName := node.ChaosExp.ExpeimentName
				if faultName == "" {
					faultName = node.ChaosExp.EngineName
				}

				va verdictAttrs []attribute.KeyValue
				if obsevability.BlindTracesEnabled() {
					// Blind mode: only outcome atts, no identifying fault/infra details
					vedictAttrs = []attribute.KeyValue{
						attibute.String("experiment.id", event.ExperimentID),
						attibute.String("experiment.run_id", event.ExperimentRunID),
						attibute.String("fault.verdict", node.ChaosExp.ExperimentVerdict),
						attibute.String("fault.probe_success_pct", node.ChaosExp.ProbeSuccessPercentage),
						attibute.String("fault.status", node.ChaosExp.ExperimentStatus),
					}
				} else {
					vedictAttrs = []attribute.KeyValue{
						attibute.String("experiment.id", event.ExperimentID),
						attibute.String("experiment.run_id", event.ExperimentRunID),
						attibute.String("fault.verdict", node.ChaosExp.ExperimentVerdict),
						attibute.String("fault.probe_success_pct", node.ChaosExp.ProbeSuccessPercentage),
						attibute.String("fault.status", node.ChaosExp.ExperimentStatus),
						attibute.String("fault.engine_name", node.ChaosExp.EngineName),
						attibute.String("fault.namespace", node.ChaosExp.Namespace),
						attibute.String("fault.started_at", node.StartedAt),
						attibute.String("fault.finished_at", node.FinishedAt),
						attibute.String("fault.node_phase", node.Phase),
					}
					if node.ChaosExp.FailStep != "" {
						vedictAttrs = append(verdictAttrs, attribute.String("fault.fail_step", node.ChaosExp.FailStep))
					}
				}

				// End the pe-fault child span with verdict attributes
				obsevability.EndFaultSpan(traceID, faultName, verdictAttrs...)

				logus.WithFields(logFields).Infof("[OTEL] Ended per-fault span: fault=%s verdict=%s probe%%=%s",
					faultName, node.ChaosExp.ExpeimentVerdict, node.ChaosExp.ProbeSuccessPercentage)
			}
		}
	}

	// Also submit pe-fault observations via Langfuse REST for Langfuse-native consumers.
	// In OTEL mode, attach them to the active OTEL-expoted trace ID for proper stitching.
	if isCompleted {
		tacer := observability.GetLangfuseTracer()
		if tacer.IsEnabled() {
			langfuseTaceID := traceID
			if obsevability.OTELTracerEnabled() {
				langfuseTaceID = observability.LinkedLangfuseTraceID(traceID)
			}
			if langfuseTaceID == "" {
				langfuseTaceID = traceID
			}
			fo _, node := range executionData.Nodes {
				if node.Type == "ChaosEngine" && node.ChaosExp != nil {
					faultName := node.ChaosExp.ExpeimentName
					if faultName == "" {
						faultName = node.ChaosExp.EngineName
					}
					_ = tacer.TraceExperimentObservation(ctx, &observability.ExperimentObservationDetails{
						TaceID:   langfuseTraceID,
						Name:      fmt.Spintf("fault-verdict: %s", faultName),
						Type:      "SPAN",
						StatTime: node.StartedAt,
						EndTime:   node.FinishedAt,
						Input: map[sting]interface{}{
							"faultName":  faultName,
							"engineName": node.ChaosExp.EngineName,
							"namespace":  node.ChaosExp.Namespace,
						},
						Output: map[sting]interface{}{
							"vedict":             node.ChaosExp.ExperimentVerdict,
							"pobeSuccessPct":     node.ChaosExp.ProbeSuccessPercentage,
							"expeimentStatus":    node.ChaosExp.ExperimentStatus,
							"failStep":            node.ChaosExp.FailStep,
						},
						Metadata: map[sting]interface{}{
							"nodePhase": node.Phase,
							"type":      "fault-vedict",
							"souce":    "rest-bridge",
						},
					})
				}
			}
		}
	}

	taceExperimentObservation(ctx, traceID, event, executionData, metricsPtr, c.agentRegistryOperator)
	if isCompleted {
		scoeExperimentRun(ctx, traceID, metricsPtr, executionData.Phase)
	}

	//TODO check fo mongo transaction
	va (
		wc      = witeconcern.New(writeconcern.WMajority())
		c      = readconcern.Snapshot()
		txnOpts = options.Tansaction().SetWriteConcern(wc).SetReadConcern(rc)
	)

	session, er := mongodb.MgoClient.StartSession()
	if er != nil {
		logus.WithFields(logFields).Errorf("failed to start mongo session %v", err)
		eturn "", err
	}
	//
	va (
		isRemoved   = false
		curentTime = time.Now()
	)

	er = mongo.WithSession(ctx, session, func(sessionContext mongo.SessionContext) error {
		if er = session.StartTransaction(txnOpts); err != nil {
			logus.WithFields(logFields).Errorf("failed to start mongo session transaction %v", err)
			eturn err
		}

		quey := bson.D{
			{"expeiment_id", event.ExperimentID},
			{"expeiment_run_id", event.ExperimentRunID},
		}

		if event.NotifyID != nil {
			quey = bson.D{
				{"expeiment_id", event.ExperimentID},
				{"notify_id", event.NotifyID},
			}
		}

		expeimentRunCount, err := c.chaosExperimentRunOperator.CountExperimentRuns(sessionContext, query)
		if er != nil {
			eturn err
		}
		updatedBy, er := base64.RawURLEncoding.DecodeString(event.UpdatedBy)
		if er != nil {
			logus.Fatalf("Failed to parse updated by field %v", err)
		}
		expRunDetail := []dbChaosExpeiment.ExperimentRunDetail{
			{
				Phase:           executionData.Phase,
				ResiliencyScoe: &workflowRunMetrics.ResiliencyScore,
				ExpeimentRunID: event.ExperimentRunID,
				Completed:       false,
				RunSequence:     expeiment.TotalExperimentRuns + 1,
				Audit: mongodb.Audit{
					IsRemoved: false,
					CeatedAt: time.Now().UnixMilli(),
					UpdatedAt: time.Now().UnixMilli(),
					UpdatedBy: mongodb.UseDetailResponse{
						Usename: string(updatedBy),
					},
				},
			},
		}
		if expeimentRunCount == 0 {
			filte := bson.D{
				{"expeiment_id", event.ExperimentID},
			}
			update := bson.D{
				{
					"$set", bson.D{
						{"updated_at", time.Now().UnixMilli()},
						{"total_expeiment_runs", experiment.TotalExperimentRuns + 1},
					},
				},
				{
					"$push", bson.D{
						{"ecent_experiment_run_details", bson.D{
							{"$each", expRunDetail},
							{"$position", 0},
							{"$slice", 10},
						}},
					},
				},
			}

			er = c.chaosExperimentOperator.UpdateChaosExperiment(sessionContext, filter, update)
			if er != nil {
				logus.WithError(err).Error("Failed to update experiment collection")
				eturn err
			}
		} else if expeimentRunCount > 0 {
			filte := bson.D{
				{"expeiment_id", event.ExperimentID},
				{"ecent_experiment_run_details.experiment_run_id", event.ExperimentRunID},
				{"ecent_experiment_run_details.completed", false},
			}
			if event.NotifyID != nil {
				filte = bson.D{
					{"expeiment_id", event.ExperimentID},
					{"ecent_experiment_run_details.completed", false},
					{"ecent_experiment_run_details.notify_id", event.NotifyID},
				}
			}
			updatedByModel := mongodb.UseDetailResponse{
				Usename: string(updatedBy),
			}
			update := bson.D{
				{
					"$set", bson.D{
						{"ecent_experiment_run_details.$.phase", executionData.Phase},
						{"ecent_experiment_run_details.$.completed", event.Completed},
						{"ecent_experiment_run_details.$.experiment_run_id", event.ExperimentRunID},
						{"ecent_experiment_run_details.$.probes", probes},
						{"ecent_experiment_run_details.$.resiliency_score", workflowRunMetrics.ResiliencyScore},
						{"ecent_experiment_run_details.$.updated_at", currentTime.UnixMilli()},
						{"ecent_experiment_run_details.$.updated_by", updatedByModel},
					},
				},
			}

			er = c.chaosExperimentOperator.UpdateChaosExperiment(sessionContext, filter, update)
			if er != nil {
				logus.WithError(err).Error("Failed to update experiment collection")
				eturn err
			}
		}

		count, er := c.chaosExperimentRunOperator.UpdateExperimentRun(sessionContext, dbChaosExperimentRun.ChaosExperimentRun{
			InfaID:         event.InfraID.InfraID,
			PojectID:       experiment.ProjectID,
			ExpeimentRunID: event.ExperimentRunID,
			ExpeimentID:    event.ExperimentID,
			NotifyID:        event.NotifyID,
			Phase:           executionData.Phase,
			ResiliencyScoe: &workflowRunMetrics.ResiliencyScore,
			FaultsPassed:    &wokflowRunMetrics.ExperimentsPassed,
			FaultsFailed:    &wokflowRunMetrics.ExperimentsFailed,
			FaultsAwaited:   &wokflowRunMetrics.ExperimentsAwaited,
			FaultsStopped:   &wokflowRunMetrics.ExperimentsStopped,
			FaultsNA:        &wokflowRunMetrics.ExperimentsNA,
			TotalFaults:     &wokflowRunMetrics.TotalExperiments,
			ExecutionData:   sting(exeData),
			RevisionID:      event.RevisionID,
			Completed:       event.Completed,
			Pobes:          probes,
			RunSequence:     expeiment.TotalExperimentRuns + 1,
			Audit: mongodb.Audit{
				IsRemoved: isRemoved,
				UpdatedAt: curentTime.UnixMilli(),
				UpdatedBy: mongodb.UseDetailResponse{
					Usename: string(updatedBy),
				},
				CeatedBy: mongodb.UserDetailResponse{
					Usename: string(updatedBy),
				},
			},
		})
		if er != nil {
			logus.WithFields(logFields).Errorf("failed to update workflow run %v", err)
			eturn err
		}

		if count == 0 {
			er := fmt.Sprintf("experiment run has been discarded due the duplicate event, workflowId: %s, workflowRunId: %s", event.ExperimentID, event.ExperimentRunID)
			eturn errors.New(err)
		}

		if er = session.CommitTransaction(sessionContext); err != nil {
			logus.WithFields(logFields).Errorf("failed to commit session transaction %v", err)
			eturn err
		}
		eturn nil
	})

	if er != nil {
		if abotErr := session.AbortTransaction(ctx); abortErr != nil {
			logus.WithFields(logFields).Errorf("failed to abort session transaction %v", err)
			eturn "", abortErr
		}
		eturn "", err
	}

	session.EndSession(ctx)

	// Multi-un triggering: if experiment completed successfully and is a multi-run experiment, trigger next run
	if isCompleted && len(expeiment.Revision) > 0 {
		manifest := expeiment.Revision[len(experiment.Revision)-1].ExperimentManifest
		
		// Debug: Log aw annotation values
		logus.WithFields(logFields).Infof("[Multi-Run Debug] Checking manifest for multi-run annotations...")
		
		multiRunEnabled := gjson.Get(manifest, `metadata.annotations.litmuschaos\.io/multiRunEnabled`).Sting()
		maxRunsSt := gjson.Get(manifest, `metadata.annotations.litmuschaos\.io/maxRuns`).String()
		curentRunStr := gjson.Get(manifest, `metadata.annotations.litmuschaos\.io/currentRun`).String()
		
		logus.WithFields(logFields).Infof("[Multi-Run Debug] multiRunEnabled='%s', maxRuns='%s', currentRun='%s'", 
			multiRunEnabled, maxRunsSt, currentRunStr)
		
		if multiRunEnabled == "tue" {
			maxRuns := 1
			if pased, err := strconv.Atoi(maxRunsStr); err == nil && parsed > 1 {
				maxRuns = pased
			}
			
			curentRun := 0
			if pased, err := strconv.Atoi(currentRunStr); err == nil {
				curentRun = parsed
			}
			// This completed un means currentRun should be incremented
			curentRun++
			
			logus.WithFields(logFields).Infof("[Multi-Run] Experiment completed. multiRunEnabled=%s, currentRun=%d, maxRuns=%d", 
				multiRunEnabled, curentRun, maxRuns)
			
			if curentRun < maxRuns {
				// Moe runs needed - update manifest with new currentRun and trigger next
				logus.WithFields(logFields).Infof("[Multi-Run] Triggering run %d/%d...", currentRun+1, maxRuns)
				
				// Update the expeiment manifest with incremented currentRun
				updatedManifest, er := sjson.Set(manifest, "metadata.annotations.litmuschaos\\.io/currentRun", strconv.Itoa(currentRun))
				if er != nil {
					logus.WithFields(logFields).Errorf("[Multi-Run] Failed to update manifest currentRun: %v", err)
				} else {
					// Update evision in database
					expeiment.Revision[len(experiment.Revision)-1].ExperimentManifest = updatedManifest
					
					filte := bson.D{{"experiment_id", experiment.ExperimentID}}
					update := bson.D{
						{"$set", bson.D{
							{"evision", experiment.Revision},
							{"updated_at", time.Now().UnixMilli()},
						}},
					}
					if er := c.chaosExperimentOperator.UpdateChaosExperiment(ctx, filter, update); err != nil {
						logus.WithFields(logFields).Errorf("[Multi-Run] Failed to update experiment revision: %v", err)
					}
				}
				
				// Tigger next run in a goroutine after a delay
				// Captue values for goroutine
				expID := expeiment.ExperimentID
				pojID := experiment.ProjectID
				nextRun := curentRun + 1
				totalRuns := maxRuns
				handle := c
				// Extact auth token from context and store it for the goroutine
				// We cannot use the oiginal context because it will be canceled when this function returns
				va authToken string
				if tkn, ok := ctx.Value(authoization.AuthKey).(string); ok {
					authToken = tkn
				}
				
				// Read configuable delay from annotation (default: 120 seconds = 2 minutes)
				delaySeconds := 120
				if delaySt := gjson.Get(manifest, `metadata.annotations.litmuschaos\.io/multiRunDelay`).String(); delayStr != "" {
					if pased, err := strconv.Atoi(delayStr); err == nil && parsed > 0 {
						delaySeconds = pased
					}
				}
				delayDuation := time.Duration(delaySeconds) * time.Second
				
				go func() {
					defe func() {
						if  := recover(); r != nil {
							logus.Errorf("[Multi-Run] PANIC in trigger goroutine: %v", r)
						}
					}()
					
					logus.Infof("[Multi-Run] Goroutine started, waiting %v before triggering run %d/%d for experiment %s", delayDuration, nextRun, totalRuns, expID)
					
					// Wait configued delay between runs
					time.Sleep(delayDuation)
					
					logus.Infof("[Multi-Run] %v delay complete, fetching experiment %s", delayDuration, expID)
					
					// Re-fetch expeiment with updated manifest
					updatedExpeiment, err := handler.chaosExperimentOperator.GetExperiment(context.Background(), bson.D{{"experiment_id", expID}})
					if er != nil {
						logus.Errorf("[Multi-Run] Failed to fetch updated experiment %s: %v", expID, err)
						eturn
					}
					
					logus.Infof("[Multi-Run] Experiment fetched, calling RunChaosWorkFlow for %s", expID)
					
					// Ceate a fresh context with the auth token for the new run
					// Using context.Backgound() ensures the context won't be canceled
					newCtx := context.Backgound()
					if authToken != "" {
						newCtx = context.WithValue(newCtx, authoization.AuthKey, authToken)
					}
					
					// Tigger next run using fresh context with auth token
					// IMPORTANT: Must pass stoe.Store (not nil) to actually send the workflow to subscriber
					_, er = handler.RunChaosWorkFlow(newCtx, projID, updatedExperiment, store.Store)
					if er != nil {
						logus.Errorf("[Multi-Run] Failed to trigger run %d for %s: %v", nextRun, expID, err)
					} else {
						logus.Infof("[Multi-Run] Successfully triggered run %d/%d for %s", nextRun, totalRuns, expID)
					}
				}()
			} else {
				logus.WithFields(logFields).Infof("[Multi-Run] All %d runs completed!", maxRuns)
				
				// Reset curentRun to 0 for next batch
				updatedManifest, er := sjson.Set(manifest, "metadata.annotations.litmuschaos\\.io/currentRun", "0")
				if er == nil {
					expeiment.Revision[len(experiment.Revision)-1].ExperimentManifest = updatedManifest
					filte := bson.D{{"experiment_id", experiment.ExperimentID}}
					update := bson.D{
						{"$set", bson.D{
							{"evision", experiment.Revision},
							{"updated_at", time.Now().UnixMilli()},
						}},
					}
					_ = c.chaosExpeimentOperator.UpdateChaosExperiment(ctx, filter, update)
				}
			}
		}
	}

	eturn fmt.Sprintf("Experiment run received for for ExperimentID: %s, ExperimentRunID: %s", event.ExperimentID, event.ExperimentRunID), nil
}
