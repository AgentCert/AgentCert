package k8s

import (
	"context"
	"errors"
	"maps"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
)

// An existing schedule must receive the latest validated workflow spec. Keep
// controller status, ownership and finalizers from the live object, and retry
// when the cron controller updates its resource version concurrently.
func updateCronWorkflow(ctx context.Context, resource dynamic.ResourceInterface, desired *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	spec, ok := desired.Object["spec"]
	if !ok {
		return nil, errors.New("CronWorkflow has no spec")
	}
	var response *unstructured.Unstructured
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := resource.Get(ctx, desired.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		current.Object["spec"] = runtime.DeepCopyJSONValue(spec)
		current.SetLabels(mergeCronMetadata(current.GetLabels(), desired.GetLabels()))
		current.SetAnnotations(mergeCronMetadata(current.GetAnnotations(), desired.GetAnnotations()))
		response, err = resource.Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	return response, err
}

func mergeCronMetadata(current, desired map[string]string) map[string]string {
	merged := make(map[string]string, len(current)+len(desired))
	maps.Copy(merged, current)
	maps.Copy(merged, desired)
	return merged
}
