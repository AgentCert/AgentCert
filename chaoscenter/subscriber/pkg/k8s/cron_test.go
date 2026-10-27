package k8s

import (
	"errors"
	"reflect"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestCronSubmissionCreatesAndUpdatesSchedules(t *testing.T) {
	for _, tc := range []struct {
		name     string
		exists   bool
		conflict bool
		forbid   bool
		kind     string
	}{
		{name: "initial schedule", kind: "CronWorkflow"},
		{name: "updated schedule", exists: true, kind: "CronWorkflow"},
		{name: "controller conflict", exists: true, conflict: true, kind: "CronWorkflow"},
		{name: "permission failure", exists: true, forbid: true, kind: "CronWorkflow"},
		{name: "immutable run", exists: true, kind: "Workflow"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gvr := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "cronworkflows"}
			if tc.kind == "Workflow" {
				gvr.Resource = "workflows"
			}
			desired := &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "argoproj.io/v1alpha1", "kind": tc.kind,
				"metadata": map[string]interface{}{"name": "config-test", "namespace": "test"},
				"spec":     map[string]interface{}{"schedule": "* * * * *", "suspend": true, "workflowSpec": map[string]interface{}{"configuration": "new"}},
			}}
			desired.SetLabels(map[string]string{"revision_id": "new"})
			desired.SetAnnotations(map[string]string{"configuration": "new"})
			old := desired.DeepCopy()
			old.Object["spec"] = map[string]interface{}{"schedule": "0 * * * *", "workflowSpec": map[string]interface{}{"configuration": "old"}}
			old.Object["status"] = map[string]interface{}{"lastScheduledTime": "2026-10-14T00:00:00Z"}
			old.SetUID(types.UID("existing"))
			old.SetFinalizers([]string{"controller-owned"})
			old.SetLabels(map[string]string{"revision_id": "old", "controller": "retained"})
			old.SetAnnotations(map[string]string{"controller": "retained"})
			objects := []runtime.Object{}
			if tc.exists {
				objects = append(objects, old)
			}
			client := fake.NewSimpleDynamicClient(runtime.NewScheme(), objects...)
			previous := dr
			dr = client.Resource(gvr).Namespace("test")
			t.Cleanup(func() { dr = previous })
			updates := 0
			client.PrependReactor("update", gvr.Resource, func(action clienttesting.Action) (bool, runtime.Object, error) {
				updates++
				if tc.forbid {
					return true, nil, apierrors.NewForbidden(gvr.GroupResource(), desired.GetName(), errors.New("denied"))
				}
				if tc.conflict && updates == 1 {
					return true, nil, apierrors.NewConflict(gvr.GroupResource(), desired.GetName(), errors.New("controller changed version"))
				}
				return false, nil, nil
			})
			before := desired.DeepCopy()
			_, err := applyRequest("create", desired)
			if tc.forbid {
				if !apierrors.IsForbidden(err) {
					t.Fatalf("permission error was lost: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, desired) {
				t.Fatal("submitted manifest was mutated")
			}
			got, err := dr.Get(Ctx, desired.GetName(), metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if tc.kind == "Workflow" {
				if !reflect.DeepEqual(got.Object["spec"], old.Object["spec"]) || updates != 0 {
					t.Fatal("existing one-off run changed")
				}
				return
			}
			if !reflect.DeepEqual(got.Object["spec"], desired.Object["spec"]) {
				t.Fatalf("schedule kept old configuration: %#v", got.Object["spec"])
			}
			if tc.exists && (got.GetUID() != old.GetUID() || !reflect.DeepEqual(got.GetFinalizers(), old.GetFinalizers()) ||
				!reflect.DeepEqual(got.Object["status"], old.Object["status"]) || got.GetLabels()["controller"] != "retained" ||
				got.GetAnnotations()["controller"] != "retained" || got.GetLabels()["revision_id"] != "new") {
				t.Fatal("controller metadata or new revision was lost")
			}
			if tc.conflict && updates != 2 {
				t.Fatalf("conflict was not retried: %d updates", updates)
			}
		})
	}
}
