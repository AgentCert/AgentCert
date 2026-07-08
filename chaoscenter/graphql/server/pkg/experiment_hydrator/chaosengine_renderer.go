package experiment_hydrator

import (
	"bytes"
	"fmt"
	"text/template"

	expdef "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/experiment_definition"
)

const chaosEngineTemplate = `apiVersion: litmuschaos.io/v1alpha1
kind: ChaosEngine
metadata:
  name: "{{.EngineName}}"
  namespace: litmus
  labels:
    ace.io/run-id: "{{.RunID}}"
spec:
  appinfo:
    appns: "{{.AppNamespace}}"
    applabel: "{{.AppLabel}}"
    appkind: deployment
  chaosServiceAccount: litmus-admin
  experiments:
    - name: {{.ExperimentRef}}
      spec:
        components:
          env:
{{range .Envs}}            - name: {{.Name}}
              value: "{{.Value}}"
{{end}}`

type chaosEngineInput struct {
	EngineName    string
	RunID         string
	AppNamespace  string
	AppLabel      string
	ExperimentRef string
	Envs          []envEntry
}

type envEntry struct {
	Name  string
	Value string
}

// renderChaosEngine produces ChaosEngine YAML for a fault step.
func renderChaosEngine(
	step expdef.ExperimentStep,
	params HydrationParams,
	stepIdx int,
) (string, error) {
	// Resolve label selector
	var appLabel, appNamespace string
	if step.Target != nil && step.Target.Microservice != "" {
		ms, ok := params.MicroserviceMap[step.Target.Microservice]
		if !ok {
			return "", fmt.Errorf("microservice %q not found in app microservice map", step.Target.Microservice)
		}
		appLabel = ms.Label
		appNamespace = ms.Namespace
	}
	if appNamespace == "" {
		appNamespace = params.AppNamespace
	}

	envs := buildEnvVarsFromParams(step.Params)

	input := chaosEngineInput{
		EngineName:    fmt.Sprintf("%s-%s", step.Name, params.RunID),
		RunID:         params.RunID,
		AppNamespace:  appNamespace,
		AppLabel:      appLabel,
		ExperimentRef: step.FaultRef,
		Envs:          envs,
	}

	return renderTemplate(chaosEngineTemplate, input)
}

// renderParallelFaultChaosEngine produces ChaosEngine YAML for one fault
// within a parallel-fault step.
func renderParallelFaultChaosEngine(
	pf expdef.ParallelFaultEntry,
	params HydrationParams,
	stepIdx, faultIdx int,
) (string, error) {
	var appLabel, appNamespace string
	if pf.Target.Microservice != "" {
		ms, ok := params.MicroserviceMap[pf.Target.Microservice]
		if !ok {
			return "", fmt.Errorf("microservice %q not found in app microservice map", pf.Target.Microservice)
		}
		appLabel = ms.Label
		appNamespace = ms.Namespace
	}
	if appNamespace == "" {
		appNamespace = params.AppNamespace
	}

	envs := buildEnvVarsFromParams(pf.Params)

	input := chaosEngineInput{
		EngineName:    fmt.Sprintf("parallel-%d-%d-%s", stepIdx, faultIdx, params.RunID),
		RunID:         params.RunID,
		AppNamespace:  appNamespace,
		AppLabel:      appLabel,
		ExperimentRef: pf.FaultRef,
		Envs:          envs,
	}

	return renderTemplate(chaosEngineTemplate, input)
}

func buildEnvVarsFromParams(p map[string]string) []envEntry {
	envs := make([]envEntry, 0, len(p))
	for k, v := range p {
		envs = append(envs, envEntry{Name: k, Value: v})
	}
	return envs
}

func renderTemplate(tmpl string, data interface{}) (string, error) {
	t, err := template.New("").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
