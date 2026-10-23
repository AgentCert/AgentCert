package faultconfig

import (
	"errors"
	"fmt"
	"strings"

	chaosTypes "github.com/litmuschaos/chaos-operator/api/litmuschaos/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// MaterializeTargets adapts resource targets to the operator's legacy appinfo
// schema. The SDK accepts other kinds through TARGETS (for example Service),
// while the operator only accepts workload kinds in appinfo. Call after target
// parameters are resolved, and only on the manifest being dispatched.
func MaterializeTargets(engine *chaosTypes.ChaosEngine) (bool, error) {
	if engine == nil {
		return false, nil
	}
	app := engine.Spec.Appinfo
	kind := strings.ToLower(strings.TrimSpace(app.AppKind))
	switch kind {
	case "", "deployment", "statefulset", "daemonset", "deploymentconfig", "rollout":
		return false, nil
	}
	namespace, selector := strings.TrimSpace(app.Appns), strings.TrimSpace(app.Applabel)
	if namespace == "" || selector == "" || len(engine.Spec.Experiments) == 0 {
		return false, errors.New("resource target requires a namespace, selector and experiment")
	}
	if strings.ContainsAny(kind+namespace+selector, ":;[]") {
		return false, errors.New("resource target contains reserved TARGETS delimiters")
	}
	if engine.Spec.Selectors != nil {
		return false, errors.New("resource target cannot combine appinfo and selectors")
	}
	target := corev1.EnvVar{Name: "TARGETS", Value: fmt.Sprintf("%s:%s:[%s]", kind, namespace, selector)}
	for i := range engine.Spec.Experiments {
		envs := engine.Spec.Experiments[i].Spec.Components.ENV
		filtered := make([]corev1.EnvVar, 0, len(envs)+1)
		for _, env := range envs {
			if env.Name != target.Name {
				filtered = append(filtered, env)
			}
		}
		engine.Spec.Experiments[i].Spec.Components.ENV = append(filtered, target)
	}
	engine.Spec.Appinfo = chaosTypes.ApplicationParams{}
	return true, nil
}
