package experiment_hydrator

import (
	"fmt"

	"gopkg.in/yaml.v3"

	expdef "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/experiment_definition"
)

// MicroserviceInfo is the resolved Kubernetes label selector for a microservice.
type MicroserviceInfo struct {
	Name      string // microservice name (from app.yaml)
	Namespace string // K8s namespace the app is installed in
	Label     string // e.g. "name=carts"
	Kind      string // deployment | statefulset | daemonset
}

// AgentSpec is the minimal agent info needed for hydration.
type AgentSpec struct {
	Name      string
	Version   string
	ChartName string // Helm chart name for agent install
	Namespace string // Namespace where agent is installed
}

// HydrationParams holds all runtime parameters for a single run.
type HydrationParams struct {
	RunID           string
	AppNamespace    string            // e.g. "sock-shop-<runID>"
	LitellmUpstream string            // e.g. "http://litellm.litmus.svc.cluster.local:4000"
	ModelOverride   string            // empty if agent-default
	AgentSecretName string            // K8s Secret name for agent secrets
	MicroserviceMap map[string]MicroserviceInfo
	ParamOverrides  map[string]string // per-step param overrides
}

// Hydrate converts an ExperimentDefinition into an Argo Workflow YAML string.
// This function is pure — it makes no K8s or MongoDB calls.
func Hydrate(
	def *expdef.ExperimentDefinitionDoc,
	agent *AgentSpec,
	params HydrationParams,
) (string, error) {
	if def == nil {
		return "", fmt.Errorf("experiment_hydrator: nil experiment definition")
	}
	if params.RunID == "" {
		return "", fmt.Errorf("experiment_hydrator: RunID is required")
	}

	dagTasks, templates, err := buildDAG(def, agent, params)
	if err != nil {
		return "", fmt.Errorf("experiment_hydrator: DAG build failed: %w", err)
	}

	// Truncate experiment name to avoid exceeding K8s 63-char name limit.
	wfName := fmt.Sprintf("%.40s-%s", def.Name, params.RunID)

	wf := ArgoWorkflow{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "Workflow",
		Metadata: ArgoMetadata{
			Name:      wfName,
			Namespace: "litmus",
			Labels: map[string]string{
				"ace.io/run-id":             params.RunID,
				"ace.io/experiment-name":    def.Name,
				"ace.io/experiment-version": def.Version,
				"ace.io/agent-name":         agent.Name,
			},
		},
		Spec: ArgoWorkflowSpec{
			Entrypoint: "experiment-dag",
			Arguments: ArgoArguments{
				Parameters: []ArgoParameter{
					{Name: "litellmUpstream", Value: params.LitellmUpstream},
					{Name: "appNamespace", Value: params.AppNamespace},
					{Name: "agentSecretName", Value: params.AgentSecretName},
					{Name: "runID", Value: params.RunID},
				},
			},
			Templates: append([]ArgoTemplate{
				{
					Name: "experiment-dag",
					DAG:  &ArgoDAG{Tasks: dagTasks},
				},
			}, templates...),
		},
	}

	out, err := yaml.Marshal(wf)
	if err != nil {
		return "", fmt.Errorf("experiment_hydrator: YAML marshal failed: %w", err)
	}

	// Validation pass — ensure output parses back cleanly
	var check map[string]interface{}
	if err := yaml.Unmarshal(out, &check); err != nil {
		return "", fmt.Errorf("experiment_hydrator: generated YAML is invalid: %w", err)
	}

	return string(out), nil
}

// HydrateAndValidate is Hydrate with an explicit validation pass.
func HydrateAndValidate(
	def *expdef.ExperimentDefinitionDoc,
	agent *AgentSpec,
	params HydrationParams,
) (string, error) {
	return Hydrate(def, agent, params)
}

// --- Argo Workflow struct types ---

// ArgoWorkflow is the top-level Argo Workflow struct.
type ArgoWorkflow struct {
	APIVersion string           `yaml:"apiVersion"`
	Kind       string           `yaml:"kind"`
	Metadata   ArgoMetadata     `yaml:"metadata"`
	Spec       ArgoWorkflowSpec `yaml:"spec"`
}

type ArgoMetadata struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

type ArgoWorkflowSpec struct {
	Entrypoint string         `yaml:"entrypoint"`
	Arguments  ArgoArguments  `yaml:"arguments"`
	Templates  []ArgoTemplate `yaml:"templates"`
}

type ArgoArguments struct {
	Parameters []ArgoParameter `yaml:"parameters"`
}

type ArgoParameter struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type ArgoTemplate struct {
	Name      string         `yaml:"name"`
	Inputs    *ArgoInputs    `yaml:"inputs,omitempty"`
	DAG       *ArgoDAG       `yaml:"dag,omitempty"`
	Container *ArgoContainer `yaml:"container,omitempty"`
	Script    *ArgoScript    `yaml:"script,omitempty"`
}

type ArgoInputs struct {
	Parameters []ArgoParameter `yaml:"parameters,omitempty"`
}

type ArgoDAG struct {
	Tasks []ArgoDAGTask `yaml:"tasks"`
}

type ArgoDAGTask struct {
	Name         string         `yaml:"name"`
	Template     string         `yaml:"template"`
	Dependencies []string       `yaml:"dependencies,omitempty"`
	Arguments    *ArgoArguments `yaml:"arguments,omitempty"`
}

type ArgoContainer struct {
	Image   string       `yaml:"image"`
	Command []string     `yaml:"command"`
	Args    []string     `yaml:"args,omitempty"`
	Env     []ArgoEnvVar `yaml:"env,omitempty"`
}

type ArgoScript struct {
	Image  string `yaml:"image"`
	Source string `yaml:"source"`
}

type ArgoEnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}
