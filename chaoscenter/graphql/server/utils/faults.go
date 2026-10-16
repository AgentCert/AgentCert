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
	return matchesFaultFamily(name, teardownExperimentNames)
}

// nodeFaultNames are the generic LitmusChaos faults that act on the cluster-scoped
// `nodes` resource (cordon / drain / taint / node-resource-hog / node-service-kill).
var nodeFaultNames = map[string]bool{
	"node-drain": true, "node-taint": true, "node-restart": true,
	"node-cpu-hog": true, "node-memory-hog": true, "node-io-stress": true,
	"node-poweroff": true, "kubelet-service-kill": true, "docker-service-kill": true,
}

// itbenchFaultNames are every fault under chaos-charts/faults/itbench/ -- the
// bin/itbench-experiment SDK catalog. They are pure Kubernetes-API mutations (patch a
// workload/service/configmap, delete/recreate a Service, ...) and a few touch
// cluster-scoped resources (nodes, priorityclasses). Along with node faults and the
// teardown steps, they run under their authored ServiceAccount (litmus-admin) rather
// than the §99 per-experiment SA -- see UsesUnscopedChaosServiceAccount.
var itbenchFaultNames = map[string]bool{
	"chaos-mesh-http-abort-replacement":                            true,
	"chaos-mesh-http-body-tamper-replacement":                      true,
	"chaos-mesh-pod-failure-replacement":                           true,
	"cordoned-kubernetes-worker-node":                              true,
	"crashing-kubernetes-workload-init-container":                  true,
	"deleted-kubernetes-service":                                   true,
	"failing-name-resolution-kubernetes-workload-dns-policy":       true,
	"hanging-kubernetes-workload-init-container":                   true,
	"ingress-port-blocking-network-policy":                         true,
	"insufficient-kubernetes-resource-quota":                       true,
	"insufficient-kubernetes-workload-container-resources":         true,
	"invalid-kubernetes-service-selector":                          true,
	"invalid-kubernetes-workload-container-command":                true,
	"kubernetes-api-server-request-surge":                          true,
	"misconfigured-kubernetes-horizontal-pod-autoscaler":           true,
	"misconfigured-kubernetes-workload-container-readiness-probe":  true,
	"modified-kubernetes-workload-container-environment-variable":  true,
	"modified-target-port-kubernetes-service":                      true,
	"nonexistent-kubernetes-workload-container-image":              true,
	"nonexistent-kubernetes-workload-node":                         true,
	"nonexistent-kubernetes-workload-persistent-volume-claim":      true,
	"opentelemetry-demo-feature-flag":                              true,
	"priority-kubernetes-workload-priority-preemption":             true,
	"scaled-to-zero-kubernetes-workload":                           true,
	"unassigned-kubernetes-workload-container-resource-limits":     true,
	"unschedulable-kubernetes-workload-pod-anti-affinity-rule":     true,
	"unsupported-architecture-kubernetes-workload-container-image": true,
	"valkey-workload-changed-password":                             true,
	"valkey-workload-out-of-memory":                                true,
}

// IsNodeFault reports whether name is a generic node-level LitmusChaos fault.
func IsNodeFault(name string) bool { return matchesFaultFamily(name, nodeFaultNames) }

// IsItbenchFault reports whether name is one of the chaos-charts/faults/itbench/ SDK faults.
func IsItbenchFault(name string) bool { return matchesFaultFamily(name, itbenchFaultNames) }

// UsesUnscopedChaosServiceAccount marks the fault families that must keep their authored
// chaosServiceAccount (litmus-admin) and run cluster-wide even when the §99 per-experiment
// sandbox is re-enabled: the ITBench teardown steps, the node-level faults, and the whole
// ITBench SDK fault catalog -- each needs cross-namespace and/or cluster-scoped access
// (delete a namespace, patch a node, act on a workload in the app namespace from the infra
// namespace) that a namespaced per-run RoleBinding structurally cannot grant.
//
// The §99 sandbox is currently gated off entirely (handler.perExperimentChaosRBACEnabled),
// so today *every* fault -- including the generic pod-level ones -- runs under litmus-admin.
// This helper is part of the scaffold for completing §99 later. See innovation.md §7.5 and
// OPEN_WEIGHT_CERTIFICATION_HANDOFF.md §106.
func UsesUnscopedChaosServiceAccount(name string) bool {
	return IsTeardownExperiment(name) || IsNodeFault(name) || IsItbenchFault(name)
}

func matchesFaultFamily(name string, family map[string]bool) bool {
	for member := range family {
		if name == member || strings.HasPrefix(name, member+"-") {
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
