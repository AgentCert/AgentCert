package faultconfig

import (
	"reflect"
	"testing"

	chaosTypes "github.com/litmuschaos/chaos-operator/api/litmuschaos/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func TestMaterializeTargets(t *testing.T) {
	for _, kind := range []string{"", "pod", "deployment", "statefulset", "daemonset", "deploymentconfig", "rollout"} {
		t.Run("unchanged "+kind, func(t *testing.T) {
			engine := chaosTypes.ChaosEngine{}
			engine.Spec.Appinfo.AppKind = kind
			before := engine.DeepCopy()
			changed, err := MaterializeTargets(&engine)
			if err != nil || changed || !reflect.DeepEqual(before, &engine) {
				t.Fatalf("workload target changed: %v, %v", changed, err)
			}
		})
	}
	for _, selector := range []string{"app=example", "service-a,service-b"} {
		t.Run(selector, func(t *testing.T) {
			engine := chaosTypes.ChaosEngine{}
			engine.Spec.Appinfo = chaosTypes.ApplicationParams{AppKind: "Service", Appns: "test", Applabel: selector}
			engine.Spec.Experiments = make([]chaosTypes.ExperimentList, 2)
			for i := range engine.Spec.Experiments {
				engine.Spec.Experiments[i].Spec.Components.ENV = []corev1.EnvVar{
					{Name: "TOTAL_CHAOS_DURATION", Value: "15"},
					{Name: "TARGETS", Value: "old"},
					{Name: "TARGETS", ValueFrom: &corev1.EnvVarSource{}},
				}
			}
			changed, err := MaterializeTargets(&engine)
			if err != nil || !changed || engine.Spec.Appinfo != (chaosTypes.ApplicationParams{}) {
				t.Fatalf("target was not adapted: %v, %v", changed, err)
			}
			want := []corev1.EnvVar{{Name: "TOTAL_CHAOS_DURATION", Value: "15"}, {Name: "TARGETS", Value: "service:test:[" + selector + "]"}}
			for _, exp := range engine.Spec.Experiments {
				if !reflect.DeepEqual(want, exp.Spec.Components.ENV) {
					t.Fatalf("unexpected environment: %#v", exp.Spec.Components.ENV)
				}
			}
			changed, err = MaterializeTargets(&engine)
			if changed || err != nil {
				t.Fatal("materialization is not idempotent")
			}
		})
	}
}

func TestMaterializeTargetsWithoutSelector(t *testing.T) {
	// ITBench feature-flag faults name a ConfigMap kind with an empty label and
	// address the flag through their own settings.
	engine := chaosTypes.ChaosEngine{}
	engine.Spec.Appinfo = chaosTypes.ApplicationParams{AppKind: "configmap", Appns: "otel-demo"}
	engine.Spec.Experiments = []chaosTypes.ExperimentList{{Name: "test"}}
	engine.Spec.Experiments[0].Spec.Components.ENV = []corev1.EnvVar{{Name: "FLAG_NAME", Value: "cartFailure"}}
	before := engine.Spec.Experiments[0].Spec.Components.ENV
	changed, err := MaterializeTargets(&engine)
	if err != nil || !changed || engine.Spec.Appinfo != (chaosTypes.ApplicationParams{}) {
		t.Fatalf("selector-less target not cleared: %v, %v, %#v", changed, err, engine.Spec.Appinfo)
	}
	if !reflect.DeepEqual(before, engine.Spec.Experiments[0].Spec.Components.ENV) {
		t.Fatalf("TARGETS added without a selector: %#v", engine.Spec.Experiments[0].Spec.Components.ENV)
	}
}

func TestMaterializeTargetsRejectsAmbiguousTargets(t *testing.T) {
	for _, app := range []chaosTypes.ApplicationParams{
		{AppKind: "service", Applabel: "app=test"},
		{AppKind: "service", Appns: "test", Applabel: "app=test;service:other:[all]"},
	} {
		engine := chaosTypes.ChaosEngine{}
		engine.Spec.Appinfo = app
		engine.Spec.Experiments = []chaosTypes.ExperimentList{{Name: "test"}}
		before := engine.DeepCopy()
		if _, err := MaterializeTargets(&engine); err == nil || !reflect.DeepEqual(before, &engine) {
			t.Fatalf("invalid target accepted or mutated: %#v", app)
		}
	}
	engine := chaosTypes.ChaosEngine{}
	engine.Spec.Appinfo = chaosTypes.ApplicationParams{AppKind: "service", Appns: "test", Applabel: "app=test"}
	engine.Spec.Experiments = []chaosTypes.ExperimentList{{Name: "test"}}
	engine.Spec.Selectors = &chaosTypes.Selector{}
	if _, err := MaterializeTargets(&engine); err == nil {
		t.Fatal("conflicting selectors accepted")
	}
}
