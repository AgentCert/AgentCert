package faultconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/ghodss/yaml"
	chaosTypes "github.com/litmuschaos/chaos-operator/api/litmuschaos/v1alpha1"
)

type Provider func(context.Context, Source, string) (*Definition, error)

var workflowParameter = regexp.MustCompile(`^\{\{\s*workflow\.parameters\.([A-Za-z0-9_-]+)\s*\}\}$`)

// ValidateWorkflow is shared by save, manual dispatch and cron dispatch.
// Source annotations select trusted hub files; schemas in a manifest are never
// trusted. Unregistered, unannotated imported faults retain legacy behaviour.
func ValidateWorkflow(ctx context.Context, projectID string, spec *v1alpha1.WorkflowSpec, resolve Provider) error {
	if spec == nil {
		return nil
	}
	resolved := make(map[Source]*Definition)
	for _, template := range spec.Templates {
		for _, artifact := range template.Inputs.Artifacts {
			if artifact.Raw == nil {
				continue
			}
			var header map[string]interface{}
			if err := yaml.Unmarshal([]byte(artifact.Raw.Data), &header); err != nil || header["kind"] != "ChaosEngine" {
				continue // Other Argo artifacts may contain scripts or non-chaos data.
			}
			var engine chaosTypes.ChaosEngine
			if err := yaml.Unmarshal([]byte(artifact.Raw.Data), &engine); err != nil {
				return fmt.Errorf("%s: invalid chaos artifact: %w", template.Name, err)
			}
			rawSource := engine.Annotations[SourceAnnotation]
			var source Source
			if rawSource != "" {
				if err := decodeSource(rawSource, &source); err != nil {
					return fmt.Errorf("%s: invalid fault source: %w", template.Name, err)
				}
				if len(engine.Spec.Experiments) != 1 {
					return fmt.Errorf("%s: a configured fault source must identify exactly one experiment", template.Name)
				}
			}
			for _, experiment := range engine.Spec.Experiments {
				ref := source
				if rawSource == "" {
					ref.Name = experiment.Name // Look up known built-in faults for legacy workflows.
				}
				definition, found := resolved[ref]
				if !found {
					var err error
					if resolve == nil {
						err = ErrNotFound
					} else {
						definition, err = resolve(ctx, ref, projectID)
					}
					if errors.Is(err, ErrNotFound) && rawSource == "" {
						resolved[ref] = nil
						continue
					}
					if err != nil {
						return fmt.Errorf("%s: cannot resolve fault configuration: %w", template.Name, err)
					}
					resolved[ref] = definition
				}
				if definition == nil {
					continue
				}
				if rawSource != "" {
					// The source names the hub directory; the engine names the
					// experiment fault.yaml declares, which can differ.
					want := definition.Experiment
					if want == "" {
						want = ref.Name
					}
					if experiment.Name != want {
						return fmt.Errorf("%s: fault source %q does not define experiment %q", template.Name, ref.Name, experiment.Name)
					}
				}
				// Only an engine built from the current definition is held to
				// it exactly. A hub update must not stop saved experiments and
				// schedules from running, so engines from an older revision are
				// checked like legacy ones: their declared settings must still
				// be valid against the current schema.
				validate := validateDeclaredSettings
				if rawSource != "" && ref.Revision == definition.Revision {
					validate = ValidateEnvironment
				}
				envs := experiment.Spec.Components.ENV
				if rawSource == "" {
					// Imported hub workflows can reference whole workflow parameters.
					// Validate the resolved value; the form itself accepts literals only.
					for i := range envs {
						match := workflowParameter.FindStringSubmatch(envs[i].Value)
						if match == nil {
							continue
						}
						for _, parameter := range spec.Arguments.Parameters {
							if parameter.Name == match[1] && parameter.Value != nil {
								envs[i].Value = parameter.Value.String()
								break
							}
						}
					}
				}
				if err := validate(definition, envs); err != nil {
					return fmt.Errorf("%s: invalid settings for %s: %w", template.Name, experiment.Name, err)
				}
			}
		}
	}
	return nil
}

// ApplyLegacyOnDemandDuration retains the old 30-second default for imported
// workflows. Configuration-enabled faults execute their validated duration.
func ApplyLegacyOnDemandDuration(engine *chaosTypes.ChaosEngine) {
	if engine.Annotations["litmuschaos.io/injectionType"] != "ON_DEMAND" || engine.Annotations[SourceAnnotation] != "" {
		return
	}
	for i := range engine.Spec.Experiments {
		for j := range engine.Spec.Experiments[i].Spec.Components.ENV {
			if engine.Spec.Experiments[i].Spec.Components.ENV[j].Name == "TOTAL_CHAOS_DURATION" {
				engine.Spec.Experiments[i].Spec.Components.ENV[j].Value = "30"
			}
		}
	}
}

func decodeSource(raw string, source *Source) error {
	if len(raw) > 4096 {
		return errors.New("annotation is too large")
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(source); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("annotation contains trailing data")
	}
	if source.HubID == "" || !folderName.MatchString(source.Category) || !folderName.MatchString(source.Name) || len(source.Revision) != 64 {
		return errors.New("hubID, category, name and content revision are required")
	}
	return nil
}
