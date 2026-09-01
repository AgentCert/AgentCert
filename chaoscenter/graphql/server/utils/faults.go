package utils

import (
	"strconv"
	"strings"

	"github.com/litmuschaos/chaos-operator/api/litmuschaos/v1alpha1"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
)

// teardownExperimentNames are the ITBench harness teardown steps that are authored as
// LitmusChaos ChaosExperiments (so an experiment built in Chaos Studio can include them
// as selectable "faults") but are not faults the agent under test is graded on -- they
// uninstall the agent / target application after a run. Their verdict is decided by a
// plain helm-uninstall wrapper binary that is not built on the litmus-go experiment SDK
// and therefore never writes a Pass ChaosResult, so counting them toward the resiliency
// score always drags it down for reasons unrelated to agent behaviour. They must be kept
// out of the weightage set (the resiliency-score denominator) and out of the pass/fail
// tallies. See OPEN_WEIGHT_CERTIFICATION_HANDOFF.md §102.
var teardownExperimentNames = map[string]bool{
	"uninstall-agent":       true,
	"uninstall-application": true,
}

// IsTeardownExperiment reports whether name refers to an ITBench teardown step rather
// than a graded fault. It matches the bare ChaosExperiment name ("uninstall-agent"), a
// Chaos Studio generateName ("uninstall-agent-wgy"), and a runtime ChaosEngine name
// ("uninstall-agent-wgy-3f9c2") -- i.e. the exact name or any "<teardown>-" prefix.
func IsTeardownExperiment(name string) bool {
	for teardown := range teardownExperimentNames {
		if name == teardown || strings.HasPrefix(name, teardown+"-") {
			return true
		}
	}
	return false
}

func TransformProbe(probeList []v1alpha1.ProbeAttributes) []v1alpha1.ProbeAttributes {
	var updateProbeList []v1alpha1.ProbeAttributes

	for _, probe := range probeList {
		updatedProbe := v1alpha1.ProbeAttributes{
			Name: probe.Name,
			Type: probe.Type,
			Mode: probe.Mode,
			Data: probe.Data,
			RunProperties: v1alpha1.RunProperty{
				ProbeTimeout:         validateUnits(probe.RunProperties.ProbeTimeout, "s"),
				Interval:             validateUnits(probe.RunProperties.Interval, "s"),
				Retry:                probe.RunProperties.Retry,
				Attempt:              probe.RunProperties.Attempt,
				InitialDelay:         validateUnits(probe.RunProperties.InitialDelay, "s"),
				EvaluationTimeout:    validateUnits(probe.RunProperties.EvaluationTimeout, "s"),
				ProbePollingInterval: validateUnits(probe.RunProperties.ProbePollingInterval, "s"),
				StopOnFailure:        probe.RunProperties.StopOnFailure,
			},
		}

		if probe.RunProperties.InitialDelaySeconds != 0 {
			updatedProbe.RunProperties.InitialDelay = validateUnits(strconv.Itoa(probe.RunProperties.InitialDelaySeconds), "s")
		}

		switch model.ProbeType(probe.Type) {
		case model.ProbeTypeHTTPProbe:
			updatedProbe.RunProperties.ProbeTimeout = validateUnits(probe.RunProperties.ProbeTimeout, "ms")
			updatedProbe.HTTPProbeInputs = probe.HTTPProbeInputs
		case model.ProbeTypeCmdProbe:
			updatedProbe.CmdProbeInputs = probe.CmdProbeInputs
		case model.ProbeTypeK8sProbe:
			updatedProbe.K8sProbeInputs = probe.K8sProbeInputs
		case model.ProbeTypePromProbe:
			updatedProbe.PromProbeInputs = probe.PromProbeInputs
		}
		updateProbeList = append(updateProbeList, updatedProbe)
	}
	return updateProbeList
}
