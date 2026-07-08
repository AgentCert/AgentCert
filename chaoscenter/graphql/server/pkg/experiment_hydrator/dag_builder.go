package experiment_hydrator

import (
	"fmt"

	expdef "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/experiment_definition"
)

// buildDAG converts the experiment step list into Argo DAG tasks and template definitions.
func buildDAG(
	def *expdef.ExperimentDefinitionDoc,
	agent *AgentSpec,
	params HydrationParams,
) ([]ArgoDAGTask, []ArgoTemplate, error) {
	var tasks []ArgoDAGTask

	// Special tasks: install-app, install-agent, teardown
	tasks = append(tasks, ArgoDAGTask{
		Name:     "install-app",
		Template: "install-app-tmpl",
	})
	tasks = append(tasks, ArgoDAGTask{
		Name:         "install-agent",
		Template:     "install-agent-tmpl",
		Dependencies: []string{"install-app"},
	})

	// Track the last step names for sequential dependency
	prevDeps := []string{"install-agent"}

	for i, step := range def.Steps {
		taskName := fmt.Sprintf("step-%s", step.Name)
		deps := prevDeps

		// If this step has an explicit dependsOn, override sequential deps
		if step.DependsOn != "" {
			deps = []string{fmt.Sprintf("step-%s", step.DependsOn)}
		}

		task := ArgoDAGTask{
			Name:         taskName,
			Dependencies: deps,
		}

		switch step.Type {
		case expdef.StepTypeObserve, expdef.StepTypeWait:
			dur := step.Duration
			if dur == "" {
				dur = "30s"
			}
			task.Template = "observe-tmpl"
			task.Arguments = &ArgoArguments{
				Parameters: []ArgoParameter{{Name: "duration", Value: dur}},
			}

		case expdef.StepTypeFault:
			if step.Target == nil {
				// No target specified — skip microservice lookup
				step.Target = &expdef.StepTarget{Microservice: ""}
			}
			ceYAML, err := renderChaosEngine(step, params, i)
			if err != nil {
				return nil, nil, fmt.Errorf("step %s: %w", step.Name, err)
			}
			task.Template = "litmus-fault-tmpl"
			task.Arguments = &ArgoArguments{
				Parameters: []ArgoParameter{{Name: "chaosEngineYaml", Value: ceYAML}},
			}

		case expdef.StepTypeVerify:
			if step.Probe == nil {
				return nil, nil, fmt.Errorf("step %s: verify step requires a probe configuration", step.Name)
			}
			task.Template = "http-probe-tmpl"
			task.Arguments = &ArgoArguments{
				Parameters: []ArgoParameter{
					{Name: "url", Value: step.Probe.URL},
					{Name: "expectedStatus", Value: fmt.Sprintf("%d", step.Probe.ExpectedStatus)},
				},
			}

		case expdef.StepTypeParallelFault:
			// Fan out: one task per fault in the parallel set, all with the same deps
			for j, pf := range step.Faults {
				pTaskName := fmt.Sprintf("step-%s-fault-%d", step.Name, j)
				ceYAML, err := renderParallelFaultChaosEngine(pf, params, i, j)
				if err != nil {
					return nil, nil, fmt.Errorf("step %s fault %d: %w", step.Name, j, err)
				}
				tasks = append(tasks, ArgoDAGTask{
					Name:         pTaskName,
					Template:     "litmus-fault-tmpl",
					Dependencies: deps,
					Arguments: &ArgoArguments{
						Parameters: []ArgoParameter{{Name: "chaosEngineYaml", Value: ceYAML}},
					},
				})
			}
			// The prevDeps for the next step must wait for ALL parallel tasks
			nextDeps := make([]string, len(step.Faults))
			for j := range step.Faults {
				nextDeps[j] = fmt.Sprintf("step-%s-fault-%d", step.Name, j)
			}
			prevDeps = nextDeps
			continue // skip the default task append below

		default:
			return nil, nil, fmt.Errorf("unknown step type: %s", step.Type)
		}

		tasks = append(tasks, task)
		prevDeps = []string{taskName}
	}

	// Teardown always runs last
	tasks = append(tasks, ArgoDAGTask{
		Name:         "teardown",
		Template:     "teardown-tmpl",
		Dependencies: prevDeps,
	})

	// Add standard reusable templates
	templates := buildStandardTemplates(agent, params)

	return tasks, templates, nil
}

// buildStandardTemplates returns the reusable Argo templates.
func buildStandardTemplates(agent *AgentSpec, params HydrationParams) []ArgoTemplate {
	return []ArgoTemplate{
		{
			Name: "observe-tmpl",
			Inputs: &ArgoInputs{
				Parameters: []ArgoParameter{{Name: "duration"}},
			},
			Container: &ArgoContainer{
				Image:   "alpine:3.19",
				Command: []string{"sh", "-c"},
				Args:    []string{`sh -c 'sleep $(echo "$DURATION" | sed "s/s//")'`},
				Env: []ArgoEnvVar{
					{Name: "DURATION", Value: "{{inputs.parameters.duration}}"},
				},
			},
		},
		{
			Name: "litmus-fault-tmpl",
			Inputs: &ArgoInputs{
				Parameters: []ArgoParameter{{Name: "chaosEngineYaml"}},
			},
			Container: &ArgoContainer{
				Image:   "litmuschaos/k8s:2.14.0",
				Command: []string{"sh", "-c"},
				Args: []string{
					"echo \"$CHAOS_ENGINE_YAML\" | kubectl apply -f - && " +
						"sleep 30 && " +
						"echo \"$CHAOS_ENGINE_YAML\" | kubectl delete -f -",
				},
				Env: []ArgoEnvVar{
					{Name: "CHAOS_ENGINE_YAML", Value: "{{inputs.parameters.chaosEngineYaml}}"},
				},
			},
		},
		{
			Name: "http-probe-tmpl",
			Inputs: &ArgoInputs{
				Parameters: []ArgoParameter{
					{Name: "url"},
					{Name: "expectedStatus"},
				},
			},
			Container: &ArgoContainer{
				Image:   "curlimages/curl:8.4.0",
				Command: []string{"sh", "-c"},
				Args: []string{
					"curl -sf -o /dev/null -w '%{http_code}' \"$PROBE_URL\" | grep \"$EXPECTED_STATUS\"",
				},
				Env: []ArgoEnvVar{
					{Name: "PROBE_URL", Value: "{{inputs.parameters.url}}"},
					{Name: "EXPECTED_STATUS", Value: "{{inputs.parameters.expectedStatus}}"},
				},
			},
		},
		{
			Name: "teardown-tmpl",
			Container: &ArgoContainer{
				Image:   "alpine/helm:3.13.3",
				Command: []string{"sh", "-c"},
				Args: []string{
					"helm uninstall agent-$RUN_ID -n $APP_NS --ignore-not-found; " +
						"helm uninstall app-$RUN_ID -n $APP_NS --ignore-not-found; " +
						"kubectl delete namespace $APP_NS --ignore-not-found",
				},
				Env: []ArgoEnvVar{
					{Name: "RUN_ID", Value: params.RunID},
					{Name: "APP_NS", Value: params.AppNamespace},
				},
			},
		},
		{
			Name: "install-app-tmpl",
			Container: &ArgoContainer{
				Image:   "alpine/helm:3.13.3",
				Command: []string{"sh", "-c"},
				Args:    []string{"helm upgrade --install app-$RUN_ID <chart> -n $APP_NS --create-namespace --wait"},
				Env: []ArgoEnvVar{
					{Name: "RUN_ID", Value: params.RunID},
					{Name: "APP_NS", Value: params.AppNamespace},
				},
			},
		},
		{
			Name: "install-agent-tmpl",
			Container: &ArgoContainer{
				Image:   "alpine/helm:3.13.3",
				Command: []string{"sh", "-c"},
				Args:    []string{"helm upgrade --install agent-$RUN_ID <agent-chart> -n $APP_NS --wait"},
				Env: []ArgoEnvVar{
					{Name: "RUN_ID", Value: params.RunID},
					{Name: "APP_NS", Value: params.AppNamespace},
				},
			},
		},
	}
}
