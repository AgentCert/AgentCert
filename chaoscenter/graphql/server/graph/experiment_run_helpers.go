package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/agent_registry"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/catalog"
	hydrator "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/experiment_hydrator"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/model_library"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

// argoWorkflowGVR is the GroupVersionResource for Argo Workflows.
var argoWorkflowGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "workflows",
}

// submitArgoWorkflow marshals the YAML into an Unstructured object and creates
// it in the litmus namespace via the K8s dynamic client.
// Returns the workflow name on success.
// Returns (workflowName, nil) even when not in cluster (no-op, uses workflow name from YAML).
func (r *Resolver) submitArgoWorkflow(ctx context.Context, workflowYAML string) (string, error) {
	// Parse YAML regardless of cluster availability so we always have a name.
	var obj unstructured.Unstructured
	jsonBytes, err := yaml.YAMLToJSON([]byte(workflowYAML))
	if err != nil {
		return "", fmt.Errorf("submitArgoWorkflow: failed to convert YAML to JSON: %w", err)
	}
	if err := json.Unmarshal(jsonBytes, &obj); err != nil {
		return "", fmt.Errorf("submitArgoWorkflow: failed to unmarshal workflow: %w", err)
	}
	wfName := obj.GetName()
	if wfName == "" {
		return "", fmt.Errorf("submitArgoWorkflow: workflow has no name")
	}

	// When running outside a cluster (dev/test), skip actual submission.
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return wfName, nil // not in cluster — no-op
	}

	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return "", fmt.Errorf("submitArgoWorkflow: failed to create dynamic client: %w", err)
	}

	_, err = dynClient.Resource(argoWorkflowGVR).Namespace("litmus").Create(ctx, &obj, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("submitArgoWorkflow: K8s create failed: %w", err)
	}
	return wfName, nil
}

// stopArgoWorkflow patches the Argo Workflow with shutdown=Stop so Argo
// terminates in-progress pods gracefully.
// No-ops when running outside a cluster.
func (r *Resolver) stopArgoWorkflow(ctx context.Context, workflowName string) error {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil // not in cluster — no-op
	}

	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("stopArgoWorkflow: failed to create dynamic client: %w", err)
	}

	patch := []byte(`{"spec":{"shutdown":"Stop"}}`)
	_, err = dynClient.Resource(argoWorkflowGVR).Namespace("litmus").Patch(
		ctx, workflowName, k8stypes.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

// resolveModel returns the LLM model and provider for a run.
// Uses modelOverride when the agent allows user choice; otherwise falls back
// to the agent's default model. Falls back to "gpt-4o" / "openai" when
// the agent has no LLM config.
func resolveModel(agent *agent_registry.Agent, modelOverride *string) (string, string) {
	llm := agent.AgentLLMConfig
	if llm == nil {
		return "gpt-4o", "openai"
	}

	chosenModel := llm.DefaultModel
	if llm.AllowUserChoice && modelOverride != nil && *modelOverride != "" {
		chosenModel = *modelOverride
	}
	if chosenModel == "" {
		chosenModel = "gpt-4o"
	}

	provider := llm.Provider
	if provider == "" {
		provider = inferProvider(chosenModel)
	}
	return chosenModel, provider
}

// inferProvider maps a model name prefix to its LLM provider slug.
func inferProvider(modelName string) string {
	lower := strings.ToLower(modelName)
	switch {
	case strings.HasPrefix(lower, "gpt-") || strings.HasPrefix(lower, "o1") || strings.HasPrefix(lower, "o3"):
		return "openai"
	case strings.HasPrefix(lower, "claude-"):
		return "anthropic"
	case strings.HasPrefix(lower, "gemini-"):
		return "google"
	case strings.HasPrefix(lower, "mistral-") || strings.HasPrefix(lower, "mixtral-"):
		return "mistral"
	case strings.HasPrefix(lower, "llama") || strings.HasPrefix(lower, "meta-llama"):
		return "meta"
	default:
		return "openai"
	}
}

// buildMicroserviceMap converts a catalog app entry's microservice list
// into the hydrator's MicroserviceInfo map keyed by microservice name.
// The appNamespace is substituted for {{.AppNamespace}} in K8s labels.
func buildMicroserviceMap(entry *catalog.AppCatalogEntry, appNamespace string) map[string]hydrator.MicroserviceInfo {
	m := make(map[string]hydrator.MicroserviceInfo, len(entry.Spec.Microservices))
	for _, ms := range entry.Spec.Microservices {
		ns := ms.K8s.Namespace
		if ns == "" || ns == "{{.AppNamespace}}" {
			ns = appNamespace
		}
		m[ms.Name] = hydrator.MicroserviceInfo{
			Name:      ms.Name,
			Namespace: ns,
			Label:     ms.K8s.Label,
			Kind:      ms.K8s.Kind,
		}
	}
	return m
}

// runAppNamespace returns the per-run Kubernetes namespace for the app.
// Falls back to "app-<runID>" when the definition has no target app name.
func runAppNamespace(appName, runID string) string {
	if appName == "" {
		return fmt.Sprintf("app-%s", runID)
	}
	return fmt.Sprintf("%s-%s", appName, runID)
}

// ensureRunSecret creates (or updates) a K8s Secret in the litmus namespace
// containing the caller-supplied key/value pairs. No-op when secrets is empty
// or when the resolver has no kube client (outside cluster).
func (r *Resolver) ensureRunSecret(ctx context.Context, secretName string, secrets []*model.AceSecretInput) error {
	if len(secrets) == 0 {
		return nil
	}
	data := make(map[string][]byte, len(secrets))
	for _, s := range secrets {
		if s != nil {
			data[s.Key] = []byte(s.Value)
		}
	}
	return model_library.CreateOrUpdateSecret(ctx, r.kubeClient, secretName, data)
}

// buildParamMap converts per-step parameter overrides to the flat
// "stepName/key" → value map consumed by the hydrator.
func buildParamMap(overrides []*model.AceParamInput) map[string]string {
	m := make(map[string]string, len(overrides))
	for _, p := range overrides {
		if p != nil {
			m[fmt.Sprintf("%s/%s", p.StepName, p.Key)] = p.Value
		}
	}
	return m
}

// hydrationArgs bundles the inputs needed to build HydrationParams,
// keeping buildHydrationParams within the 7-parameter limit.
type hydrationArgs struct {
	projectID       string
	runID           string
	appNamespace    string
	agentSecretName string
	chosenModel     string
	appName         string
	paramOverrides  []*model.AceParamInput
}

// buildHydrationParams assembles the HydrationParams for a single run,
// resolving the microservice map from the catalog when available.
func (r *Resolver) buildHydrationParams(ctx context.Context, args hydrationArgs) hydrator.HydrationParams {
	msMap := map[string]hydrator.MicroserviceInfo{}
	if r.catalogService != nil {
		if entry, err := r.catalogService.GetApplication(ctx, args.projectID, args.appName); err == nil && entry != nil {
			msMap = buildMicroserviceMap(entry, args.appNamespace)
		}
	}
	return hydrator.HydrationParams{
		RunID:           args.runID,
		AppNamespace:    args.appNamespace,
		LitellmUpstream: envOrDefault("LITELLM_UPSTREAM", "http://litellm.litmus.svc.cluster.local:4000"),
		ModelOverride:   args.chosenModel,
		AgentSecretName: args.agentSecretName,
		MicroserviceMap: msMap,
		ParamOverrides:  buildParamMap(args.paramOverrides),
	}
}
