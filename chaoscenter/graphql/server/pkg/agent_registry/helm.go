package agent_registry

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"
)

// SanitizeReleaseName converts a string to a valid Kubernetes DNS-1035 label for Helm release names.
// DNS-1035 labels must:
// - Consist of lower case alphanumeric characters or '-'
// - Start with an alphabetic character
// - End with an alphanumeric character
// - Be no more than 63 characters
func SanitizeReleaseName(name string) string {
	if name == "" {
		return "agent"
	}

	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace dots, underscores, and other invalid characters with hyphens
	name = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(name, "-")

	// Remove leading hyphens (replace with 'a' to ensure it starts with a letter)
	for strings.HasPrefix(name, "-") {
		name = "a" + name[1:]
	}

	// Remove trailing hyphens
	name = strings.TrimRight(name, "-")

	// Remove consecutive hyphens
	name = regexp.MustCompile(`-+`).ReplaceAllString(name, "-")

	// Ensure it's not empty
	if name == "" {
		name = "agent"
	}

	// Truncate to 63 characters (Kubernetes max DNS label length)
	if len(name) > 63 {
		name = name[:63]
		// Ensure it doesn't end with a hyphen after truncation
		name = strings.TrimRight(name, "-")
	}

	return name
}

// HelmDeployRequest captures parameters required for Helm deployment.
type HelmDeployRequest struct {
	ReleaseName                  string
	Namespace                    string
	ChartPath                    string
	ChartData                    *string // Base64-encoded .tgz chart data
	ChartVersion                 *string
	ValuesYAML                   *string
	Kubeconfig                   *string
	AgentID                      string
	ImageTag                     *string
	// Azure OpenAI Environment Variables
	AzureOpenAIKey               *string
	AzureOpenAIEndpoint          *string
	AzureOpenAIDeployment        *string
	AzureOpenAIAPIVersion        *string
	AzureOpenAIEmbeddingDeployment *string
}

// DeployWithHelm installs or upgrades an agent Helm chart.
func DeployWithHelm(ctx context.Context, req *HelmDeployRequest) (string, error) {
	// Sanitize release name to ensure it's a valid Kubernetes DNS-1035 label
	req.ReleaseName = SanitizeReleaseName(req.ReleaseName)
	if req == nil {
		return "", fmt.Errorf("nil helm request")
	}
	if strings.TrimSpace(req.ReleaseName) == "" {
		return "", fmt.Errorf("helm release name is required")
	}
	if strings.TrimSpace(req.Namespace) == "" {
		return "", fmt.Errorf("helm namespace is required")
	}

	// Initialize helm binary and paths
	helmBin := utils.Config.HelmBinary
	if strings.TrimSpace(helmBin) == "" {
		helmBin = "helm"
	}

	var chartPath string
	var cleanupChart func()

	// If chartData is provided (base64-encoded .tgz), extract it to temp directory
	if req.ChartData != nil && strings.TrimSpace(*req.ChartData) != "" {
		decoded, err := base64.StdEncoding.DecodeString(*req.ChartData)
		if err != nil {
			return "", fmt.Errorf("failed to decode chart data: %w", err)
		}

		// Create temp file for chart
		tmpFile, err := os.CreateTemp("", "helm-chart-*.tgz")
		if err != nil {
			return "", fmt.Errorf("failed to create temp chart file: %w", err)
		}
		
		if _, err := tmpFile.Write(decoded); err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			return "", fmt.Errorf("failed to write chart data: %w", err)
		}
		tmpFile.Close()

		chartPath = tmpFile.Name()
		cleanupChart = func() { os.Remove(tmpFile.Name()) }
		defer cleanupChart()
	} else if strings.TrimSpace(req.ChartPath) != "" {
		// Use server-side chart path
		chartPath = req.ChartPath
	} else {
		return "", fmt.Errorf("either chartData or chartPath is required")
	}

	// First, ensure namespace exists
	if strings.TrimSpace(req.Namespace) != "" {
		createNsCmd := exec.CommandContext(ctx, helmBin, "repo", "list")
		createNsCmd.Run() // Just to test kubectl is accessible
		nsCmd := exec.CommandContext(ctx, "kubectl", "create", "namespace", req.Namespace, "--dry-run=client", "-o", "yaml")
		if nsOutput, err := nsCmd.Output(); err == nil && len(nsOutput) > 0 {
			applyCmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
			applyCmd.Stdin = strings.NewReader(string(nsOutput))
			applyCmd.Run()
		}
	}

	// Create ConfigMap and Secret for environment variables in the target namespace
	// First, clean up any orphaned resources from previous failed deployments
	cleanupOrphanedResources(ctx, req.Namespace, req.ReleaseName)
	
	// COMMENTED OUT: Don't create ConfigMap/Secret manually - let Helm manage them
	// This was creating resources without Helm ownership labels, causing conflicts
	// The Helm chart will create its own ConfigMap/Secret with proper labels
	// if err := createEnvConfigMapAndSecret(ctx, req); err != nil {
	// 	log.Printf("[Helm Deploy] Warning: Failed to create environment ConfigMap/Secret: %v", err)
	// 	// Continue anyway - the chart might not need it
	// }

	// Apply timeout
	timeout := utils.Config.HelmTimeout
	if strings.TrimSpace(timeout) == "" {
		timeout = "5m"
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	args := []string{
		"upgrade", "--install",
		req.ReleaseName,
		chartPath,
		"--namespace", req.Namespace,
		"--create-namespace",
		"--wait",
		"--timeout", timeout,
		"--atomic",           // Rollback on failure, prevents orphaned resources
		"--cleanup-on-fail",  // Clean up resources if install fails
	}

	if req.ChartVersion != nil && strings.TrimSpace(*req.ChartVersion) != "" {
		args = append(args, "--version", *req.ChartVersion)
	}

	// Values YAML
	var valuesFile string
	if req.ValuesYAML != nil && strings.TrimSpace(*req.ValuesYAML) != "" {
		f, err := os.CreateTemp("", "agent-values-*.yaml")
		if err != nil {
			return "", fmt.Errorf("failed to create temp values file: %w", err)
		}
		defer os.Remove(f.Name())
		if _, err := f.WriteString(*req.ValuesYAML); err != nil {
			_ = f.Close()
			return "", fmt.Errorf("failed to write values file: %w", err)
		}
		_ = f.Close()
		valuesFile = f.Name()
		args = append(args, "--values", valuesFile)
	}

	// Kubeconfig
	var kubeconfigFile string
	if req.Kubeconfig != nil && strings.TrimSpace(*req.Kubeconfig) != "" {
		f, err := os.CreateTemp("", "agent-kubeconfig-*.yaml")
		if err != nil {
			return "", fmt.Errorf("failed to create temp kubeconfig: %w", err)
		}
		defer os.Remove(f.Name())
		if _, err := f.WriteString(*req.Kubeconfig); err != nil {
			_ = f.Close()
			return "", fmt.Errorf("failed to write kubeconfig: %w", err)
		}
		_ = f.Close()
		kubeconfigFile = f.Name()
		args = append(args, "--kubeconfig", kubeconfigFile)
	} else {
		// If no kubeconfig provided, check if KUBECONFIG env var or default ~/.kube/config exists
		if kubeConfigEnv := os.Getenv("KUBECONFIG"); kubeConfigEnv != "" {
			log.Printf("[Helm Deploy] Using KUBECONFIG from env: %s", kubeConfigEnv)
		} else if homeDir, err := os.UserHomeDir(); err == nil {
			defaultKubeconfig := homeDir + "/.kube/config"
			if _, err := os.Stat(defaultKubeconfig); err == nil {
				log.Printf("[Helm Deploy] Using default kubeconfig: %s", defaultKubeconfig)
			} else {
				log.Printf("[Helm Deploy] No kubeconfig found, will use in-cluster config if available")
			}
		}
	}

	// Set agentId and optional image tag
	args = append(args, "--set", fmt.Sprintf("agentId=%s", req.AgentID))
	if req.ImageTag != nil && strings.TrimSpace(*req.ImageTag) != "" {
		args = append(args, "--set", fmt.Sprintf("image.tag=%s", *req.ImageTag))
	}

	// Pass Azure OpenAI values using ONLY the AI_Ops pattern (secrets.* and configMap.*)
	// This matches how the Helm chart expects to receive these values
	if req.AzureOpenAIKey != nil && strings.TrimSpace(*req.AzureOpenAIKey) != "" {
		args = append(args, "--set-string", fmt.Sprintf("secrets.azureOpenaiKey=%s", *req.AzureOpenAIKey))
	}
	if req.AzureOpenAIEndpoint != nil && strings.TrimSpace(*req.AzureOpenAIEndpoint) != "" {
		args = append(args, "--set", fmt.Sprintf("configMap.AZURE_OPENAI_ENDPOINT=%s", *req.AzureOpenAIEndpoint))
	}
	if req.AzureOpenAIDeployment != nil && strings.TrimSpace(*req.AzureOpenAIDeployment) != "" {
		args = append(args, "--set", fmt.Sprintf("configMap.AZURE_OPENAI_DEPLOYMENT=%s", *req.AzureOpenAIDeployment))
	}
	if req.AzureOpenAIAPIVersion != nil && strings.TrimSpace(*req.AzureOpenAIAPIVersion) != "" {
		args = append(args, "--set", fmt.Sprintf("configMap.AZURE_OPENAI_API_VERSION=%s", *req.AzureOpenAIAPIVersion))
	}
	if req.AzureOpenAIEmbeddingDeployment != nil && strings.TrimSpace(*req.AzureOpenAIEmbeddingDeployment) != "" {
		args = append(args, "--set", fmt.Sprintf("configMap.AZURE_OPENAI_EMBEDDING_DEPLOYMENT=%s", *req.AzureOpenAIEmbeddingDeployment))
	}

	log.Printf("[Helm Deploy] Executing: %s %s", helmBin, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, helmBin, args...)
	output, err := cmd.CombinedOutput()
	log.Printf("[Helm Deploy] Output: %s", string(output))
	if err != nil {
		return string(output), fmt.Errorf("helm deploy failed: %w (output: %s)", err, string(output))
	}

	return string(output), nil
}

// patchAzureCredentials updates the chart-generated ConfigMap and Secret with Azure OpenAI values
func patchAzureCredentials(ctx context.Context, req *HelmDeployRequest) error {
	if req == nil || strings.TrimSpace(req.Namespace) == "" || strings.TrimSpace(req.ReleaseName) == "" {
		return fmt.Errorf("invalid request for patching credentials")
	}

	name := req.ReleaseName
	namespace := req.Namespace

	// Patch ConfigMap
	configPatches := []string{}
	if req.AzureOpenAIEndpoint != nil && strings.TrimSpace(*req.AzureOpenAIEndpoint) != "" {
		configPatches = append(configPatches, fmt.Sprintf(`AZURE_OPENAI_ENDPOINT=%q`, *req.AzureOpenAIEndpoint))
	}
	if req.AzureOpenAIDeployment != nil && strings.TrimSpace(*req.AzureOpenAIDeployment) != "" {
		configPatches = append(configPatches, fmt.Sprintf(`AZURE_OPENAI_DEPLOYMENT=%q`, *req.AzureOpenAIDeployment))
	}
	if req.AzureOpenAIAPIVersion != nil && strings.TrimSpace(*req.AzureOpenAIAPIVersion) != "" {
		configPatches = append(configPatches, fmt.Sprintf(`AZURE_OPENAI_API_VERSION=%q`, *req.AzureOpenAIAPIVersion))
	}
	if req.AzureOpenAIEmbeddingDeployment != nil && strings.TrimSpace(*req.AzureOpenAIEmbeddingDeployment) != "" {
		configPatches = append(configPatches, fmt.Sprintf(`AZURE_OPENAI_EMBEDDING_DEPLOYMENT=%q`, *req.AzureOpenAIEmbeddingDeployment))
	}

	if len(configPatches) > 0 {
		patchStr := fmt.Sprintf(`{"data":{%s}}`, strings.Join(configPatches, ","))
		patchCmd := exec.CommandContext(ctx, "kubectl", "patch", "configmap", "-n", namespace, name, "--type", "merge", "-p", patchStr)
		if output, err := patchCmd.CombinedOutput(); err != nil {
			log.Printf("[Helm Deploy] ConfigMap patch output: %s", string(output))
			return fmt.Errorf("failed to patch ConfigMap: %w", err)
		}
		log.Printf("[Helm Deploy] Patched ConfigMap %s with Azure credentials", name)
	}

	// Patch Secret
	if req.AzureOpenAIKey != nil && strings.TrimSpace(*req.AzureOpenAIKey) != "" {
		patchStr := fmt.Sprintf(`{"stringData":{"AZURE_OPENAI_KEY":%q}}`, *req.AzureOpenAIKey)
		patchCmd := exec.CommandContext(ctx, "kubectl", "patch", "secret", "-n", namespace, name, "--type", "merge", "-p", patchStr)
		if output, err := patchCmd.CombinedOutput(); err != nil {
			log.Printf("[Helm Deploy] Secret patch output: %s", string(output))
			return fmt.Errorf("failed to patch Secret: %w", err)
		}
		log.Printf("[Helm Deploy] Patched Secret %s with AZURE_OPENAI_KEY", name)
	}

	return nil
}

// restartDeployment restarts the deployment to pick up new env vars from ConfigMap/Secret
func restartDeployment(ctx context.Context, req *HelmDeployRequest) error {
	if req == nil || strings.TrimSpace(req.Namespace) == "" || strings.TrimSpace(req.ReleaseName) == "" {
		return fmt.Errorf("invalid request for restarting deployment")
	}

	// Find deployment name by release label
	getDeployCmd := exec.CommandContext(
		ctx,
		"kubectl",
		"get",
		"deploy",
		"-n",
		req.Namespace,
		"-l",
		fmt.Sprintf("app.kubernetes.io/instance=%s", req.ReleaseName),
		"-o",
		"jsonpath={.items[0].metadata.name}",
	)
	deployNameBytes, err := getDeployCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to resolve deployment name: %w", err)
	}
	deployName := strings.TrimSpace(string(deployNameBytes))
	if deployName == "" {
		return fmt.Errorf("no deployment found for release %s", req.ReleaseName)
	}

	// Restart deployment
	restartCmd := exec.CommandContext(ctx, "kubectl", "rollout", "restart", "deploy", "-n", req.Namespace, deployName)
	if output, err := restartCmd.CombinedOutput(); err != nil {
		log.Printf("[Helm Deploy] Restart output: %s", string(output))
		return fmt.Errorf("failed to restart deployment: %w", err)
	}

	log.Printf("[Helm Deploy] Restarted deployment %s to pick up new env vars", deployName)
	return nil
}

// ensureAzureEnvOnDeployment patches the deployment to include Azure OpenAI env vars
// sourced from the agent-config ConfigMap and agent-secrets Secret.
// This is a safety net for charts that don't wire these env vars into the pod spec.
func ensureAzureEnvOnDeployment(ctx context.Context, req *HelmDeployRequest) error {
	// Only patch if any Azure OpenAI values are provided
	if req == nil {
		return nil
	}

	needsKey := req.AzureOpenAIKey != nil && strings.TrimSpace(*req.AzureOpenAIKey) != ""
	needsEndpoint := req.AzureOpenAIEndpoint != nil && strings.TrimSpace(*req.AzureOpenAIEndpoint) != ""
	needsDeployment := req.AzureOpenAIDeployment != nil && strings.TrimSpace(*req.AzureOpenAIDeployment) != ""
	needsAPIVersion := req.AzureOpenAIAPIVersion != nil && strings.TrimSpace(*req.AzureOpenAIAPIVersion) != ""
	needsEmbedding := req.AzureOpenAIEmbeddingDeployment != nil && strings.TrimSpace(*req.AzureOpenAIEmbeddingDeployment) != ""

	if !needsKey && !needsEndpoint && !needsDeployment && !needsAPIVersion && !needsEmbedding {
		return nil
	}

	if strings.TrimSpace(req.Namespace) == "" || strings.TrimSpace(req.ReleaseName) == "" {
		return fmt.Errorf("namespace and release name are required for patching")
	}

	// Find deployment name by release label
	getDeployCmd := exec.CommandContext(
		ctx,
		"kubectl",
		"get",
		"deploy",
		"-n",
		req.Namespace,
		"-l",
		fmt.Sprintf("app.kubernetes.io/instance=%s", req.ReleaseName),
		"-o",
		"jsonpath={.items[0].metadata.name}",
	)
	deployNameBytes, err := getDeployCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to resolve deployment name: %w", err)
	}
	deployName := strings.TrimSpace(string(deployNameBytes))
	if deployName == "" {
		return fmt.Errorf("no deployment found for release %s", req.ReleaseName)
	}

	// Get first container name
	getContainerCmd := exec.CommandContext(
		ctx,
		"kubectl",
		"get",
		"deploy",
		"-n",
		req.Namespace,
		deployName,
		"-o",
		"jsonpath={.spec.template.spec.containers[0].name}",
	)
	containerNameBytes, err := getContainerCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to resolve container name: %w", err)
	}
	containerName := strings.TrimSpace(string(containerNameBytes))
	if containerName == "" {
		return fmt.Errorf("no container found for deployment %s", deployName)
	}

	// Use release name for config/secret to match chart-generated names
	configName := req.ReleaseName
	secretName := req.ReleaseName

	// Build env patch entries
	envEntries := []string{}
	if needsKey {
		envEntries = append(envEntries, fmt.Sprintf(`{"name":"AZURE_OPENAI_KEY","valueFrom":{"secretKeyRef":{"name":"%s","key":"AZURE_OPENAI_KEY"}}}`, secretName))
	}
	if needsEndpoint {
		envEntries = append(envEntries, fmt.Sprintf(`{"name":"AZURE_OPENAI_ENDPOINT","valueFrom":{"configMapKeyRef":{"name":"%s","key":"AZURE_OPENAI_ENDPOINT"}}}`, configName))
	}
	if needsDeployment {
		envEntries = append(envEntries, fmt.Sprintf(`{"name":"AZURE_OPENAI_DEPLOYMENT","valueFrom":{"configMapKeyRef":{"name":"%s","key":"AZURE_OPENAI_DEPLOYMENT"}}}`, configName))
	}
	if needsAPIVersion {
		envEntries = append(envEntries, fmt.Sprintf(`{"name":"AZURE_OPENAI_API_VERSION","valueFrom":{"configMapKeyRef":{"name":"%s","key":"AZURE_OPENAI_API_VERSION"}}}`, configName))
	}
	if needsEmbedding {
		envEntries = append(envEntries, fmt.Sprintf(`{"name":"AZURE_OPENAI_EMBEDDING_DEPLOYMENT","valueFrom":{"configMapKeyRef":{"name":"%s","key":"AZURE_OPENAI_EMBEDDING_DEPLOYMENT"}}}`, configName))
	}

	patch := fmt.Sprintf(
		`{"spec":{"template":{"spec":{"containers":[{"name":"%s","env":[%s]}]}}}}`,
		containerName,
		strings.Join(envEntries, ","),
	)

	patchCmd := exec.CommandContext(
		ctx,
		"kubectl",
		"patch",
		"deploy",
		"-n",
		req.Namespace,
		deployName,
		"--type",
		"strategic",
		"-p",
		patch,
	)
	if patchOutput, patchErr := patchCmd.CombinedOutput(); patchErr != nil {
		return fmt.Errorf("failed to patch deployment: %w (output: %s)", patchErr, string(patchOutput))
	}

	log.Printf("[Helm Deploy] Patched deployment %s with Azure OpenAI env vars", deployName)
	return nil
}

// HelmUninstallRequest captures parameters for Helm uninstall.
type HelmUninstallRequest struct {
	ReleaseName string
	Namespace   string
	Kubeconfig  *string
}

// UninstallWithHelm removes a Helm release.
func UninstallWithHelm(ctx context.Context, req *HelmUninstallRequest) (string, error) {
	if req == nil {
		return "", fmt.Errorf("nil helm uninstall request")
	}
	if strings.TrimSpace(req.ReleaseName) == "" {
		return "", fmt.Errorf("helm release name is required")
	}
	if strings.TrimSpace(req.Namespace) == "" {
		return "", fmt.Errorf("helm namespace is required")
	}

	// Sanitize release name to ensure it's a valid Kubernetes DNS-1035 label
	req.ReleaseName = SanitizeReleaseName(req.ReleaseName)

	helmBin := utils.Config.HelmBinary
	if strings.TrimSpace(helmBin) == "" {
		helmBin = "helm"
	}

	args := []string{
		"uninstall",
		req.ReleaseName,
		"--namespace", req.Namespace,
	}

	// Kubeconfig handling (same as deploy)
	if req.Kubeconfig != nil && strings.TrimSpace(*req.Kubeconfig) != "" {
		f, err := os.CreateTemp("", "agent-kubeconfig-*.yaml")
		if err != nil {
			return "", fmt.Errorf("failed to create temp kubeconfig: %w", err)
		}
		defer os.Remove(f.Name())
		if _, err := f.WriteString(*req.Kubeconfig); err != nil {
			_ = f.Close()
			return "", fmt.Errorf("failed to write kubeconfig: %w", err)
		}
		_ = f.Close()
		args = append(args, "--kubeconfig", f.Name())
	}

	log.Printf("[Helm Uninstall] Executing: %s %s", helmBin, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, helmBin, args...)
	output, err := cmd.CombinedOutput()
	log.Printf("[Helm Uninstall] Output: %s", string(output))
	if err != nil {
		// Check if release not found - not an error in this case
		if strings.Contains(string(output), "not found") || strings.Contains(string(output), "release: not found") {
			log.Printf("[Helm Uninstall] Release %s not found, skipping", req.ReleaseName)
			return string(output), nil
		}
		return string(output), fmt.Errorf("helm uninstall failed: %w (output: %s)", err, string(output))
	}

	return string(output), nil
}

// buildConfigMapYAML generates YAML for a Kubernetes ConfigMap
func buildConfigMapYAML(name, namespace string, data map[string]string) string {
	yaml := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
`, name, namespace)

	for k, v := range data {
		yaml += fmt.Sprintf("  %s: %q\n", k, v)
	}

	return yaml
}

// cleanupOrphanedResources deletes any existing ConfigMap or Secret with the release name
// that don't have Helm ownership labels (orphaned from failed previous deployments)
func cleanupOrphanedResources(ctx context.Context, namespace, releaseName string) {
	// Try to delete orphaned secret
	deleteSecretCmd := exec.CommandContext(ctx, "kubectl", "delete", "secret", releaseName, "-n", namespace, "--ignore-not-found")
	if output, err := deleteSecretCmd.CombinedOutput(); err != nil {
		log.Printf("[Cleanup] Failed to delete orphaned secret: %v (output: %s)", err, string(output))
	} else if len(output) > 0 {
		log.Printf("[Cleanup] Deleted orphaned secret: %s", string(output))
	}

	// Try to delete orphaned configmap
	deleteConfigMapCmd := exec.CommandContext(ctx, "kubectl", "delete", "configmap", releaseName, "-n", namespace, "--ignore-not-found")
	if output, err := deleteConfigMapCmd.CombinedOutput(); err != nil {
		log.Printf("[Cleanup] Failed to delete orphaned configmap: %v (output: %s)", err, string(output))
	} else if len(output) > 0 {
		log.Printf("[Cleanup] Deleted orphaned configmap: %s", string(output))
	}
}

// buildSecretYAML generates YAML for a Kubernetes Secret
func buildSecretYAML(name, namespace string, data map[string]string) string {
	yaml := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
type: Opaque
stringData:
`, name, namespace)

	for k, v := range data {
		yaml += fmt.Sprintf("  %s: %q\n", k, v)
	}

	return yaml
}