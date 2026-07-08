package model_library

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const litmusNamespace = "litmus"

// GetK8sClient returns an in-cluster Kubernetes client, or nil if not in cluster.
func GetK8sClient() kubernetes.Interface {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil
	}
	return cs
}

// CreateOrUpdateSecret creates or replaces a K8s Secret in the litmus namespace.
func CreateOrUpdateSecret(ctx context.Context, client kubernetes.Interface, name string, data map[string][]byte) error {
	if client == nil {
		return nil // no-op outside cluster
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: litmusNamespace,
			Labels:    map[string]string{"ace.io/type": "model-config"},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
	existing, err := client.CoreV1().Secrets(litmusNamespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		secret.ResourceVersion = existing.ResourceVersion
		_, err = client.CoreV1().Secrets(litmusNamespace).Update(ctx, secret, metav1.UpdateOptions{})
		return err
	}
	_, err = client.CoreV1().Secrets(litmusNamespace).Create(ctx, secret, metav1.CreateOptions{})
	return err
}

// DeleteSecret deletes a K8s Secret from the litmus namespace.
func DeleteSecret(ctx context.Context, client kubernetes.Interface, name string) error {
	if client == nil {
		return nil
	}
	return client.CoreV1().Secrets(litmusNamespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// CreateAgentSecret creates an agent-scoped secret for experiment inputs.
func CreateAgentSecret(ctx context.Context, client kubernetes.Interface, experimentID string, secretData map[string][]byte) error {
	name := fmt.Sprintf("ace-agent-secret-%s", experimentID)
	return CreateOrUpdateSecret(ctx, client, name, secretData)
}

// DeleteAgentSecret deletes an agent-scoped experiment secret.
func DeleteAgentSecret(ctx context.Context, client kubernetes.Interface, experimentID string) error {
	name := fmt.Sprintf("ace-agent-secret-%s", experimentID)
	return DeleteSecret(ctx, client, name)
}
