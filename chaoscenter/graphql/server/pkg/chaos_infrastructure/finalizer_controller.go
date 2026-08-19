package chaos_infrastructure

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// InfraFinalizer is the finalizer key used to mark infrastructure namespaces
	// that require cleanup before actual deletion
	InfraFinalizer = "chaos.litmuschaos.io/cleanup"
)

// litmusCRDFinalizerTargets lists the LitmusChaos CRDs known to attach their own
// per-object finalizer, keyed by the finalizer name their owning controller sets.
// Their controller runs inside the infrastructure's own subscriber pod, which is
// already gone by the time this cleanup runs (DeleteInfra tears it down first) —
// nothing will ever clear these finalizers on their own, so a namespace holding
// any of these objects stays stuck in Terminating forever. ChaosEngine is the
// only Litmus CRD whose controller (chaosengine_controller.go) actually sets a
// finalizer as of chaos-operator@e96a7ee; chaosresults is swept defensively in
// case that changes upstream, tolerating "no finalizer"/"CRD absent" as a no-op.
var litmusCRDFinalizerTargets = []schema.GroupVersionResource{
	{Group: "litmuschaos.io", Version: "v1alpha1", Resource: "chaosengines"},
	{Group: "litmuschaos.io", Version: "v1alpha1", Resource: "chaosresults"},
}

// FinalizerController manages infrastructure namespace cleanup via Kubernetes finalizers
type FinalizerController struct {
	clientset     *kubernetes.Clientset
	dynamicClient dynamic.Interface
	stopChan      chan struct{}
}

// NewFinalizerController creates a new finalizer controller instance
func NewFinalizerController() (*FinalizerController, error) {
	cfg, err := buildKubeRestConfig()
	if err != nil {
		logrus.Warnf("Failed to build Kubernetes REST config for finalizer controller: %v. "+
			"Finalizer-based cleanup will not be available. Manual namespace cleanup will be required.", err)
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logrus.Warnf("Failed to build Kubernetes clientset for finalizer controller: %v. "+
			"Finalizer-based cleanup will not be available. Manual namespace cleanup will be required.", err)
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		logrus.Warnf("Failed to build dynamic client for finalizer controller: %v. "+
			"Orphaned LitmusChaos CRDs (ChaosEngine/ChaosResult) will not be cleaned up, which can leave "+
			"namespaces stuck in Terminating.", err)
		// Non-fatal: namespace/job/pod cleanup can still proceed without the dynamic client.
	}

	return &FinalizerController{
		clientset:     clientset,
		dynamicClient: dynamicClient,
		stopChan:      make(chan struct{}),
	}, nil
}

// AddFinalizerToNamespace adds the infrastructure cleanup finalizer to a namespace
// This prevents immediate deletion and allows cleanup logic to run first
func (fc *FinalizerController) AddFinalizerToNamespace(ctx context.Context, namespaceName string) error {
	if fc.clientset == nil {
		logrus.Warnf("Finalizer controller not available, skipping AddFinalizerToNamespace for %s", namespaceName)
		return fmt.Errorf("finalizer controller not initialized")
	}

	// Skip adding finalizer to the ACE platform namespace
	if namespaceName == "ace" {
		logrus.Debugf("Skipping finalizer on platform namespace 'ace'")
		return nil
	}

	ns, err := fc.clientset.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
	if err != nil {
		logrus.Warnf("Failed to get namespace %s for finalizer addition: %v", namespaceName, err)
		return err
	}

	// Check if finalizer already exists
	for _, finalizer := range ns.Finalizers {
		if finalizer == InfraFinalizer {
			logrus.Debugf("Finalizer already exists on namespace %s", namespaceName)
			return nil
		}
	}

	// Add the finalizer
	ns.Finalizers = append(ns.Finalizers, InfraFinalizer)
	_, err = fc.clientset.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{})
	if err != nil {
		logrus.Warnf("Failed to add finalizer to namespace %s: %v", namespaceName, err)
		return err
	}

	logrus.Infof("Added cleanup finalizer to namespace %s", namespaceName)
	return nil
}

// RemoveFinalizerFromNamespace removes the infrastructure cleanup finalizer
// This allows the namespace to actually be deleted once cleanup is done
func (fc *FinalizerController) RemoveFinalizerFromNamespace(ctx context.Context, namespaceName string) error {
	if fc.clientset == nil {
		logrus.Warnf("Finalizer controller not available, skipping RemoveFinalizerFromNamespace for %s", namespaceName)
		return fmt.Errorf("finalizer controller not initialized")
	}

	ns, err := fc.clientset.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
	if err != nil {
		// Namespace already deleted, nothing to do
		return nil
	}

	// Remove the finalizer
	newFinalizers := make([]string, 0)
	for _, finalizer := range ns.Finalizers {
		if finalizer != InfraFinalizer {
			newFinalizers = append(newFinalizers, finalizer)
		}
	}

	if len(newFinalizers) == len(ns.Finalizers) {
		// Finalizer wasn't present
		return nil
	}

	ns.Finalizers = newFinalizers
	_, err = fc.clientset.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{})
	if err != nil {
		logrus.Warnf("Failed to remove finalizer from namespace %s: %v", namespaceName, err)
		return err
	}

	logrus.Infof("Removed cleanup finalizer from namespace %s, allowing deletion to proceed", namespaceName)
	return nil
}

// DeleteInfrastructureNamespace initiates namespace deletion with finalizer protection
// The actual cleanup is handled by the watcher; this method just triggers the deletion
func (fc *FinalizerController) DeleteInfrastructureNamespace(ctx context.Context, namespaceName string) error {
	if fc.clientset == nil {
		logrus.Warnf("Finalizer controller not available, cannot delete namespace %s. Manual cleanup required.", namespaceName)
		return fmt.Errorf("finalizer controller not initialized")
	}

	// Skip deletion of platform namespace
	if namespaceName == "ace" {
		logrus.Warnf("Refusing to delete platform namespace 'ace'")
		return fmt.Errorf("cannot delete platform namespace")
	}

	// First add finalizer if not already present
	if err := fc.AddFinalizerToNamespace(ctx, namespaceName); err != nil {
		logrus.Warnf("Failed to add finalizer before deletion of %s: %v. Proceeding with deletion anyway.", namespaceName, err)
	}

	// Initiate namespace deletion (will be held in Terminating state by the finalizer)
	err := fc.clientset.CoreV1().Namespaces().Delete(ctx, namespaceName, metav1.DeleteOptions{})
	if err != nil {
		logrus.Warnf("Failed to delete namespace %s: %v", namespaceName, err)
		return err
	}

	logrus.Infof("Initiated deletion of infrastructure namespace %s (finalizer will handle cleanup)", namespaceName)
	return nil
}

// StartWatcher starts the background namespace watcher that handles finalizer cleanup
// This should be called once at application startup
func (fc *FinalizerController) StartWatcher(ctx context.Context) {
	if fc.clientset == nil {
		logrus.Warn("Finalizer controller not initialized, skipping watcher start")
		return
	}

	go func() {
		logrus.Info("Starting infrastructure namespace finalizer watcher")
		defer logrus.Info("Infrastructure namespace finalizer watcher stopped")

		for {
			select {
			case <-fc.stopChan:
				return
			case <-ctx.Done():
				return
			default:
			}

			// Watch namespaces in Terminating state
			watcher, err := fc.clientset.CoreV1().Namespaces().Watch(ctx, metav1.ListOptions{})
			if err != nil {
				logrus.Warnf("Failed to watch namespaces: %v. Retrying in 30 seconds.", err)
				time.Sleep(30 * time.Second)
				continue
			}

			fc.processNamespaceEvents(ctx, watcher.ResultChan())

			watcher.Stop()
		}
	}()
}

// processNamespaceEvents processes namespace watch events and handles cleanup
func (fc *FinalizerController) processNamespaceEvents(ctx context.Context, eventChan <-chan watch.Event) {
	for {
		select {
		case <-fc.stopChan:
			return
		case event, ok := <-eventChan:
			if !ok {
				// Channel closed
				return
			}

			ns, ok := event.Object.(*corev1.Namespace)
			if !ok {
				continue
			}

			// Only process namespaces that have our finalizer and are being deleted
			if !hasInfrastructureFinalizer(ns) {
				continue
			}

			if ns.DeletionTimestamp == nil {
				continue
			}

			logrus.Infof("Processing cleanup for terminating namespace %s", ns.Name)

			// Perform cleanup (safe, scoped operations)
			if err := fc.cleanupInfrastructureNamespace(ctx, ns); err != nil {
				logrus.Warnf("Cleanup error for namespace %s: %v. Will retry on next event.", ns.Name, err)
				continue
			}

			// Cleanup succeeded, remove finalizer to allow deletion
			if err := fc.RemoveFinalizerFromNamespace(ctx, ns.Name); err != nil {
				logrus.Warnf("Failed to remove finalizer from %s: %v. Will retry on next event.", ns.Name, err)
				continue
			}

		case <-ctx.Done():
			return
		}
	}
}

// cleanupInfrastructureNamespace performs safe, scoped cleanup of infrastructure resources
// before the namespace itself is deleted
func (fc *FinalizerController) cleanupInfrastructureNamespace(ctx context.Context, ns *corev1.Namespace) error {
	logrus.Infof("Starting cleanup of infrastructure namespace %s", ns.Name)

	// Safety check: never cleanup the platform namespace
	if ns.Name == "ace" {
		return fmt.Errorf("refusing to cleanup platform namespace 'ace'")
	}

	// Check for PVCs before cleanup (safety measure to prevent data loss)
	pvcs, err := fc.clientset.CoreV1().PersistentVolumeClaims(ns.Name).List(ctx, metav1.ListOptions{})
	if err == nil && len(pvcs.Items) > 0 {
		logrus.Warnf("Namespace %s has %d PVCs. Skipping cleanup to avoid data loss. "+
			"Manual cleanup may be needed.", ns.Name, len(pvcs.Items))
		return fmt.Errorf("namespace contains persistent volumes, aborting cleanup")
	}

	// Delete completed jobs (these accumulate during fault injection)
	if err := fc.cleanupCompletedJobs(ctx, ns.Name); err != nil {
		logrus.Warnf("Failed to cleanup completed jobs in %s: %v", ns.Name, err)
		// Don't fail cleanup entirely, continue with other steps
	}

	// Strip finalizers from orphaned LitmusChaos CRDs. Their controller (the
	// chaos-operator running in the subscriber pod) is already gone by this
	// point -- DeleteInfra tears the subscriber down before this cleanup ever
	// runs -- so nothing will ever clear these finalizers on its own, and the
	// namespace would otherwise stay stuck in Terminating forever (observed:
	// 1,171 ChaosEngine objects blocking deletion). See OPEN_WEIGHT_CERTIFICATION_HANDOFF.md.
	if err := fc.cleanupOrphanedLitmusCRDFinalizers(ctx, ns.Name); err != nil {
		logrus.Warnf("Failed to cleanup orphaned LitmusChaos CRD finalizers in %s: %v", ns.Name, err)
		// Don't fail cleanup entirely, continue with other steps
	}

	// Delete stale pods in Error state (from failed experiments)
	if err := fc.cleanupErrorPods(ctx, ns.Name); err != nil {
		logrus.Warnf("Failed to cleanup error pods in %s: %v", ns.Name, err)
		// Don't fail cleanup entirely, continue
	}

	logrus.Infof("Cleanup completed for namespace %s", ns.Name)
	return nil
}

// cleanupOrphanedLitmusCRDFinalizers strips finalizers from every object of
// each type in litmusCRDFinalizerTargets within the given namespace. Safe to
// call even when a target CRD isn't installed or has no matching objects --
// both cases are treated as a no-op, not an error, since most namespaces will
// only ever have ChaosEngines and this must never block Jobs/Pods cleanup.
func (fc *FinalizerController) cleanupOrphanedLitmusCRDFinalizers(ctx context.Context, namespaceName string) error {
	if fc.dynamicClient == nil {
		return fmt.Errorf("dynamic client not available")
	}

	var firstErr error
	for _, gvr := range litmusCRDFinalizerTargets {
		resourceClient := fc.dynamicClient.Resource(gvr).Namespace(namespaceName)
		list, err := resourceClient.List(ctx, metav1.ListOptions{})
		if err != nil {
			// CRD not installed, or transient API error -- either way, don't
			// let one resource type block the others.
			logrus.Debugf("Skipping %s in %s: %v", gvr.Resource, namespaceName, err)
			continue
		}
		for _, item := range list.Items {
			if len(item.GetFinalizers()) == 0 {
				continue
			}
			logrus.Infof("Clearing finalizers on orphaned %s %s/%s (owning controller is gone)",
				gvr.Resource, namespaceName, item.GetName())
			patch := []byte(`{"metadata":{"finalizers":[]}}`)
			if _, err := resourceClient.Patch(ctx, item.GetName(), types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
				logrus.Warnf("Failed to clear finalizers on %s %s/%s: %v", gvr.Resource, namespaceName, item.GetName(), err)
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}
	return firstErr
}

// cleanupCompletedJobs removes completed jobs to free up storage/etcd space
func (fc *FinalizerController) cleanupCompletedJobs(ctx context.Context, namespaceName string) error {
	jobs, err := fc.clientset.BatchV1().Jobs(namespaceName).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	propagationPolicy := metav1.DeletePropagationBackground
	for _, job := range jobs.Items {
		// Only delete completed or failed jobs
		if job.Status.Active == 0 && (job.Status.Succeeded > 0 || job.Status.Failed > 0) {
			logrus.Debugf("Deleting completed job %s/%s", job.Namespace, job.Name)
			err := fc.clientset.BatchV1().Jobs(namespaceName).Delete(ctx, job.Name,
				metav1.DeleteOptions{PropagationPolicy: &propagationPolicy})
			if err != nil {
				logrus.Warnf("Failed to delete job %s/%s: %v", job.Namespace, job.Name, err)
			}
		}
	}

	return nil
}

// cleanupErrorPods removes pods stuck in Error state
func (fc *FinalizerController) cleanupErrorPods(ctx context.Context, namespaceName string) error {
	pods, err := fc.clientset.CoreV1().Pods(namespaceName).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	propagationPolicy := metav1.DeletePropagationBackground
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodUnknown {
			logrus.Debugf("Deleting error pod %s/%s (phase: %s)", pod.Namespace, pod.Name, pod.Status.Phase)
			err := fc.clientset.CoreV1().Pods(namespaceName).Delete(ctx, pod.Name,
				metav1.DeleteOptions{PropagationPolicy: &propagationPolicy})
			if err != nil {
				logrus.Warnf("Failed to delete pod %s/%s: %v", pod.Namespace, pod.Name, err)
			}
		}
	}

	return nil
}

// Stop stops the finalizer controller's background watcher
func (fc *FinalizerController) Stop() {
	if fc != nil {
		close(fc.stopChan)
	}
}

// hasInfrastructureFinalizer checks if a namespace has the infrastructure cleanup finalizer
func hasInfrastructureFinalizer(ns *corev1.Namespace) bool {
	for _, finalizer := range ns.Finalizers {
		if finalizer == InfraFinalizer {
			return true
		}
	}
	return false
}

// buildKubeClientset creates a Kubernetes clientset, following the same pattern as used elsewhere in the codebase
func buildKubeRestConfig() (*rest.Config, error) {
	tryPaths := make([]string, 0, 3)

	if envKubeConfig := strings.TrimSpace(os.Getenv("KUBECONFIG")); envKubeConfig != "" {
		for _, p := range strings.Split(envKubeConfig, string(os.PathListSeparator)) {
			p = strings.TrimSpace(p)
			if p != "" {
				tryPaths = append(tryPaths, p)
			}
		}
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		tryPaths = append(tryPaths, filepath.Join(home, ".kube", "config"))
	}

	seen := make(map[string]struct{})
	for _, kubePath := range tryPaths {
		if kubePath == "" {
			continue
		}
		if _, ok := seen[kubePath]; ok {
			continue
		}
		seen[kubePath] = struct{}{}

		cfg, err := clientcmd.BuildConfigFromFlags("", kubePath)
		if err != nil {
			continue
		}

		// Verify the config actually produces a usable clientset before
		// committing to this kubeconfig path over the remaining candidates.
		if _, err := kubernetes.NewForConfig(cfg); err == nil {
			logrus.WithField("kubeconfig", kubePath).Debug("Using kubeconfig for finalizer controller")
			return cfg, nil
		}
	}

	// Try in-cluster config as fallback
	return rest.InClusterConfig()
}
