package agent_registry

import (
	"context"
	"fmt"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/model_library"
	"github.com/sirupsen/logrus"
)

// AgentSecretRef returns the Helm --set argument that wires the named K8s Secret
// into the install-agent chart as agent.secretRef. Pass this to the install-agent
// step so the Helm chart can mount the secret into the agent pod.
//
// experimentID may be a literal ID string or an Argo workflow template expression
// such as "{{workflow.labels.workflow_id}}".
func AgentSecretRef(experimentID string) string {
	return fmt.Sprintf("agent.secretRef=ace-agent-secret-%s", experimentID)
}

// UpsertExperimentSecret creates or updates the K8s Secret
// "ace-agent-secret-<experimentID>" in the litmus namespace.
// secretData maps input keys to their plain-text values (secret-type inputs only).
// If the K8s client is unavailable (local dev), this is a no-op.
func UpsertExperimentSecret(ctx context.Context, experimentID string, secretData map[string][]byte) error {
	k8sClient := model_library.GetK8sClient()
	return model_library.CreateAgentSecret(ctx, k8sClient, experimentID, secretData)
}

// DeleteExperimentSecret deletes the K8s Secret "ace-agent-secret-<experimentID>"
// from the litmus namespace. Errors are logged as warnings and not returned so
// callers treat this as non-fatal (the experiment is already gone from the DB).
func DeleteExperimentSecret(ctx context.Context, experimentID string) {
	k8sClient := model_library.GetK8sClient()
	if err := model_library.DeleteAgentSecret(ctx, k8sClient, experimentID); err != nil {
		logrus.WithField("experimentID", experimentID).WithError(err).
			Warn("failed to delete agent experiment secret (non-fatal)")
	}
}
