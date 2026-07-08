package graph

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	expdef "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/experiment_definition"
)

func experimentInputToDoc(input model.AceExperimentInput) *expdef.ExperimentDefinitionDoc {
	doc := &expdef.ExperimentDefinitionDoc{
		Name:    input.Name,
		Version: "1.0.0",
		TargetApp: expdef.TargetAppSpec{
			Name:    input.TargetApp.Name,
			Version: input.TargetApp.Version,
		},
		ModelSelection: expdef.ModelSelection{
			Mode: modelSelectionModeToService(input.ModelSelection.Mode),
		},
	}
	if input.ModelSelection.FixedModel != nil {
		doc.ModelSelection.FixedModel = *input.ModelSelection.FixedModel
	}
	if input.DisplayName != nil {
		doc.DisplayName = *input.DisplayName
	}
	if input.Hypothesis != nil {
		doc.Hypothesis = *input.Hypothesis
	}
	if input.Tags != nil {
		doc.Tags = input.Tags
	}
	if input.EvaluationMetrics != nil {
		doc.EvaluationMetrics = input.EvaluationMetrics
	}
	if input.TargetApp.InstallParams != nil {
		doc.TargetApp.InstallParams = kvPairsToMap(input.TargetApp.InstallParams)
	}
	if input.AgentConstraints != nil {
		doc.AgentConstraints = expdef.AgentConstraints{
			RequiredCapabilities: input.AgentConstraints.RequiredCapabilities,
			SupportedAgents:      input.AgentConstraints.SupportedAgents,
			BlockedAgents:        input.AgentConstraints.BlockedAgents,
		}
	}
	if input.SuccessCriteria != nil {
		sc := expdef.SuccessCriteria{}
		for _, ps := range input.SuccessCriteria.PerStep {
			if ps != nil {
				sc.PerStep = append(sc.PerStep, expdef.PerStepCriteria{
					StepName:           ps.StepName,
					DetectWithinSecs:   ps.DetectWithinSecs,
					MitigateWithinSecs: ps.MitigateWithinSecs,
				})
			}
		}
		if input.SuccessCriteria.Overall != nil {
			sc.Overall = &expdef.OverallCriteria{
				ToolCallEfficiencyMin: input.SuccessCriteria.Overall.ToolCallEfficiencyMin,
				FalsePositiveRateMax:  input.SuccessCriteria.Overall.FalsePositiveRateMax,
				RootCauseAccuracyMin:  input.SuccessCriteria.Overall.RootCauseAccuracyMin,
			}
		}
		doc.SuccessCriteria = sc
	}

	for _, s := range input.Steps {
		if s == nil {
			continue
		}
		step := expdef.ExperimentStep{
			Name: s.Name,
			Type: stepTypeToService(s.Type),
		}
		if s.Description != nil {
			step.Description = *s.Description
		}
		if s.Duration != nil {
			step.Duration = *s.Duration
		}
		if s.FaultRef != nil {
			step.FaultRef = *s.FaultRef
		}
		if s.DependsOn != nil {
			step.DependsOn = *s.DependsOn
		}
		if s.Target != nil {
			t := expdef.StepTarget{Microservice: s.Target.Microservice}
			if s.Target.ExplicitPodName != nil {
				t.ExplicitPodName = *s.Target.ExplicitPodName
			}
			step.Target = &t
		}
		if s.Params != nil {
			step.Params = kvPairsToMap(s.Params)
		}
		if s.GroundTruthOverride != nil {
			step.GroundTruthOverride = &expdef.GroundTruthOverride{
				DetectWithinSecs:   s.GroundTruthOverride.DetectWithinSecs,
				MitigateWithinSecs: s.GroundTruthOverride.MitigateWithinSecs,
			}
		}
		if s.Probe != nil {
			probe := &expdef.StepProbe{
				URL:            s.Probe.URL,
				ExpectedStatus: s.Probe.ExpectedStatus,
			}
			if s.Probe.TimeoutSecs != nil {
				probe.TimeoutSecs = *s.Probe.TimeoutSecs
			}
			if s.Probe.Retries != nil {
				probe.Retries = *s.Probe.Retries
			}
			step.Probe = probe
		}
		for _, pf := range s.Faults {
			if pf == nil {
				continue
			}
			entry := expdef.ParallelFaultEntry{
				FaultRef: pf.FaultRef,
				Target:   expdef.StepTarget{Microservice: pf.Target.Microservice},
			}
			if pf.Target.ExplicitPodName != nil {
				entry.Target.ExplicitPodName = *pf.Target.ExplicitPodName
			}
			if pf.Params != nil {
				entry.Params = kvPairsToMap(pf.Params)
			}
			step.Faults = append(step.Faults, entry)
		}
		doc.Steps = append(doc.Steps, step)
	}
	return doc
}

func experimentDocToGraphQL(doc *expdef.ExperimentDefinitionDoc) *model.AceExperimentDefinition {
	if doc == nil {
		return nil
	}
	status := model.ExperimentDefinitionStatusDraft
	if doc.Status == "READY" {
		status = model.ExperimentDefinitionStatusReady
	}

	out := &model.AceExperimentDefinition{
		Name:      doc.Name,
		Version:   doc.Version,
		Status:    status,
		CreatedAt: doc.CreatedAt.Format(time.RFC3339),
		UpdatedAt: doc.UpdatedAt.Format(time.RFC3339),
		CreatedBy: doc.CreatedBy,
		TargetApp: &model.AceTargetApp{
			Name:    doc.TargetApp.Name,
			Version: doc.TargetApp.Version,
		},
		ModelSelection: &model.AceModelSelection{
			Mode: modelSelectionModeToGraphQL(doc.ModelSelection.Mode),
		},
	}
	if doc.DisplayName != "" {
		out.DisplayName = &doc.DisplayName
	}
	if doc.Hypothesis != "" {
		out.Hypothesis = &doc.Hypothesis
	}
	if doc.Tags != nil {
		out.Tags = doc.Tags
	}
	if doc.EvaluationMetrics != nil {
		out.EvaluationMetrics = doc.EvaluationMetrics
	}
	if doc.ModelSelection.FixedModel != "" {
		out.ModelSelection.FixedModel = &doc.ModelSelection.FixedModel
	}
	if len(doc.TargetApp.InstallParams) > 0 {
		out.TargetApp.InstallParams = mapToKVPairs(doc.TargetApp.InstallParams)
	}

	for _, s := range doc.Steps {
		step := &model.AceExperimentStep{
			Name: s.Name,
			Type: stepTypeToGraphQL(s.Type),
		}
		if s.Description != "" {
			step.Description = &s.Description
		}
		if s.Duration != "" {
			step.Duration = &s.Duration
		}
		if s.FaultRef != "" {
			step.FaultRef = &s.FaultRef
		}
		if s.DependsOn != "" {
			step.DependsOn = &s.DependsOn
		}
		if s.Target != nil {
			t := &model.StepTarget{Microservice: s.Target.Microservice}
			if s.Target.ExplicitPodName != "" {
				t.ExplicitPodName = &s.Target.ExplicitPodName
			}
			step.Target = t
		}
		if len(s.Params) > 0 {
			step.Params = mapToKVPairs(s.Params)
		}
		if s.GroundTruthOverride != nil {
			step.GroundTruthOverride = &model.GroundTruthOverride{
				DetectWithinSecs:   s.GroundTruthOverride.DetectWithinSecs,
				MitigateWithinSecs: s.GroundTruthOverride.MitigateWithinSecs,
			}
		}
		if s.Probe != nil {
			p := &model.StepProbe{
				URL:            s.Probe.URL,
				ExpectedStatus: s.Probe.ExpectedStatus,
			}
			if s.Probe.TimeoutSecs != 0 {
				p.TimeoutSecs = &s.Probe.TimeoutSecs
			}
			if s.Probe.Retries != 0 {
				p.Retries = &s.Probe.Retries
			}
			step.Probe = p
		}
		for _, pf := range s.Faults {
			entry := &model.ParallelFaultEntry{
				FaultRef: pf.FaultRef,
				Target:   &model.StepTarget{Microservice: pf.Target.Microservice},
			}
			if pf.Target.ExplicitPodName != "" {
				entry.Target.ExplicitPodName = &pf.Target.ExplicitPodName
			}
			if len(pf.Params) > 0 {
				entry.Params = mapToKVPairs(pf.Params)
			}
			step.Faults = append(step.Faults, entry)
		}
		out.Steps = append(out.Steps, step)
	}
	return out
}

func experimentDocsToGraphQL(docs []*expdef.ExperimentDefinitionDoc) []*model.AceExperimentDefinition {
	out := make([]*model.AceExperimentDefinition, 0, len(docs))
	for _, d := range docs {
		if g := experimentDocToGraphQL(d); g != nil {
			out = append(out, g)
		}
	}
	return out
}

// bumpVersion increments the patch component of a semver string.
func bumpVersion(v string) string {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return v
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return v
	}
	return fmt.Sprintf("%s.%s.%d", parts[0], parts[1], patch+1)
}

// --- internal helpers ---

func kvPairsToMap(pairs []*model.KeyValuePairInput) map[string]string {
	if pairs == nil {
		return nil
	}
	m := make(map[string]string, len(pairs))
	for _, p := range pairs {
		if p != nil {
			m[p.Key] = p.Value
		}
	}
	return m
}

func mapToKVPairs(m map[string]string) []*model.KeyValuePair {
	out := make([]*model.KeyValuePair, 0, len(m))
	for k, v := range m {
		k, v := k, v
		out = append(out, &model.KeyValuePair{Key: k, Value: v})
	}
	return out
}

func stepTypeToService(t model.StepType) expdef.ExperimentStepType {
	switch t {
	case model.StepTypeFault:
		return expdef.StepTypeFault
	case model.StepTypeVerify:
		return expdef.StepTypeVerify
	case model.StepTypeWait:
		return expdef.StepTypeWait
	case model.StepTypeParallelFault:
		return expdef.StepTypeParallelFault
	default:
		return expdef.StepTypeObserve
	}
}

func stepTypeToGraphQL(t expdef.ExperimentStepType) model.StepType {
	switch t {
	case expdef.StepTypeFault:
		return model.StepTypeFault
	case expdef.StepTypeVerify:
		return model.StepTypeVerify
	case expdef.StepTypeWait:
		return model.StepTypeWait
	case expdef.StepTypeParallelFault:
		return model.StepTypeParallelFault
	default:
		return model.StepTypeObserve
	}
}

func modelSelectionModeToService(m model.ModelSelectionMode) expdef.ModelSelectionMode {
	switch m {
	case model.ModelSelectionModeFixed:
		return expdef.ModelSelectionFixed
	case model.ModelSelectionModeUserChoosesAtRun:
		return expdef.ModelSelectionUserChoosesAtRun
	default:
		return expdef.ModelSelectionAgentDefault
	}
}

func modelSelectionModeToGraphQL(m expdef.ModelSelectionMode) model.ModelSelectionMode {
	switch m {
	case expdef.ModelSelectionFixed:
		return model.ModelSelectionModeFixed
	case expdef.ModelSelectionUserChoosesAtRun:
		return model.ModelSelectionModeUserChoosesAtRun
	default:
		return model.ModelSelectionModeAgentDefault
	}
}
