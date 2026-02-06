package helm

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// HelmService provides methods for Helm chart operations
type HelmService struct {
	kubeconfig string
}

// NewHelmService creates a new HelmService instance
func NewHelmService(kubeconfig string) *HelmService {
	return &HelmService{
		kubeconfig: kubeconfig,
	}
}

// InstallChart installs a Helm chart from a .tgz file
func (h *HelmService) InstallChart(ctx context.Context, chartPath string, request *HelmChartInstallRequest) (*HelmChartInstallResponse, error) {
	// Validate the chart file exists
	if _, err := os.Stat(chartPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("chart file not found: %s", chartPath)
	}

	// Extract chart name from the .tgz file
	chartName, err := h.extractChartName(chartPath)
	if err != nil {
		log.Warnf("Could not extract chart name: %v, using file name", err)
		chartName = strings.TrimSuffix(filepath.Base(chartPath), ".tgz")
	}

	// Use release name from request or default to chart name
	releaseName := request.ReleaseName
	if releaseName == "" {
		releaseName = fmt.Sprintf("%s-%s", chartName, request.EnvironmentName)
		// Clean the release name to be valid for Helm
		releaseName = strings.ToLower(releaseName)
		releaseName = strings.ReplaceAll(releaseName, "_", "-")
		releaseName = strings.ReplaceAll(releaseName, " ", "-")
	}

	// Use namespace from request or default to environment name
	namespace := request.Namespace
	if namespace == "" {
		namespace = strings.ToLower(request.EnvironmentName)
		namespace = strings.ReplaceAll(namespace, "_", "-")
		namespace = strings.ReplaceAll(namespace, " ", "-")
	}

	// Build helm install command
	// Note: Not using --wait to avoid timeout issues with slow-starting pods
	// The install will return immediately and pods will start in the background
	args := []string{
		"install",
		releaseName,
		chartPath,
		"--namespace", namespace,
		"--create-namespace",
	}

	// Add kubeconfig if specified
	if h.kubeconfig != "" {
		args = append(args, "--kubeconfig", h.kubeconfig)
	}

	log.Infof("Executing helm command: helm %s", strings.Join(args, " "))

	// Execute helm install command
	cmd := exec.CommandContext(ctx, "helm", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Errorf("Helm install failed: %v, output: %s", err, string(output))
		return &HelmChartInstallResponse{
			Success: false,
			Message: fmt.Sprintf("Helm install failed: %s", string(output)),
		}, err
	}

	log.Infof("Helm install successful: %s", string(output))

	return &HelmChartInstallResponse{
		Success:     true,
		Message:     fmt.Sprintf("Successfully installed chart '%s' as release '%s' in namespace '%s'", chartName, releaseName, namespace),
		ReleaseName: releaseName,
		Namespace:   namespace,
		Status:      "deployed",
	}, nil
}

// UpgradeChart upgrades an existing Helm release
func (h *HelmService) UpgradeChart(ctx context.Context, chartPath string, request *HelmChartInstallRequest) (*HelmChartInstallResponse, error) {
	// Validate the chart file exists
	if _, err := os.Stat(chartPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("chart file not found: %s", chartPath)
	}

	releaseName := request.ReleaseName
	if releaseName == "" {
		return nil, fmt.Errorf("release name is required for upgrade")
	}

	namespace := request.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// Build helm upgrade command
	// Note: Not using --wait to avoid timeout issues
	args := []string{
		"upgrade",
		releaseName,
		chartPath,
		"--namespace", namespace,
	}

	// Add kubeconfig if specified
	if h.kubeconfig != "" {
		args = append(args, "--kubeconfig", h.kubeconfig)
	}

	log.Infof("Executing helm command: helm %s", strings.Join(args, " "))

	// Execute helm upgrade command
	cmd := exec.CommandContext(ctx, "helm", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Errorf("Helm upgrade failed: %v, output: %s", err, string(output))
		return &HelmChartInstallResponse{
			Success: false,
			Message: fmt.Sprintf("Helm upgrade failed: %s", string(output)),
		}, err
	}

	log.Infof("Helm upgrade successful: %s", string(output))

	return &HelmChartInstallResponse{
		Success:     true,
		Message:     fmt.Sprintf("Successfully upgraded release '%s' in namespace '%s'", releaseName, namespace),
		ReleaseName: releaseName,
		Namespace:   namespace,
		Status:      "deployed",
	}, nil
}

// InstallOrUpgradeChart installs or upgrades a Helm chart based on whether the release exists
func (h *HelmService) InstallOrUpgradeChart(ctx context.Context, chartPath string, request *HelmChartInstallRequest) (*HelmChartInstallResponse, error) {
	// Check if release already exists
	releaseName := request.ReleaseName
	namespace := request.Namespace
	if namespace == "" {
		namespace = strings.ToLower(request.EnvironmentName)
		namespace = strings.ReplaceAll(namespace, "_", "-")
		namespace = strings.ReplaceAll(namespace, " ", "-")
	}

	if releaseName != "" {
		exists, err := h.releaseExists(ctx, releaseName, namespace)
		if err != nil {
			log.Warnf("Error checking if release exists: %v", err)
		}
		if exists {
			return h.UpgradeChart(ctx, chartPath, request)
		}
	}

	return h.InstallChart(ctx, chartPath, request)
}

// releaseExists checks if a Helm release already exists
func (h *HelmService) releaseExists(ctx context.Context, releaseName, namespace string) (bool, error) {
	args := []string{
		"status",
		releaseName,
		"--namespace", namespace,
	}

	if h.kubeconfig != "" {
		args = append(args, "--kubeconfig", h.kubeconfig)
	}

	cmd := exec.CommandContext(ctx, "helm", args...)
	err := cmd.Run()

	return err == nil, nil
}

// extractChartName extracts the chart name from the Chart.yaml in a .tgz file
func (h *HelmService) extractChartName(chartPath string) (string, error) {
	file, err := os.Open(chartPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		// Look for Chart.yaml in the archive
		if strings.HasSuffix(header.Name, "Chart.yaml") {
			content, err := io.ReadAll(tarReader)
			if err != nil {
				return "", err
			}

			// Simple parsing to extract name from Chart.yaml
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(line), "name:") {
					name := strings.TrimPrefix(strings.TrimSpace(line), "name:")
					return strings.TrimSpace(name), nil
				}
			}
		}
	}

	return "", fmt.Errorf("Chart.yaml not found in archive")
}

// UninstallChart uninstalls a Helm release
func (h *HelmService) UninstallChart(ctx context.Context, releaseName, namespace string) (*HelmChartInstallResponse, error) {
	args := []string{
		"uninstall",
		releaseName,
		"--namespace", namespace,
	}

	if h.kubeconfig != "" {
		args = append(args, "--kubeconfig", h.kubeconfig)
	}

	log.Infof("Executing helm command: helm %s", strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, "helm", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Errorf("Helm uninstall failed: %v, output: %s", err, string(output))
		return &HelmChartInstallResponse{
			Success: false,
			Message: fmt.Sprintf("Helm uninstall failed: %s", string(output)),
		}, err
	}

	log.Infof("Helm uninstall successful: %s", string(output))

	return &HelmChartInstallResponse{
		Success:     true,
		Message:     fmt.Sprintf("Successfully uninstalled release '%s' from namespace '%s'", releaseName, namespace),
		ReleaseName: releaseName,
		Namespace:   namespace,
		Status:      "uninstalled",
	}, nil
}

// ListReleases lists all Helm releases in a namespace
func (h *HelmService) ListReleases(ctx context.Context, namespace string) ([]HelmReleaseInfo, error) {
	args := []string{
		"list",
		"--namespace", namespace,
		"--output", "json",
	}

	if namespace == "" {
		args = append(args, "--all-namespaces")
	}

	if h.kubeconfig != "" {
		args = append(args, "--kubeconfig", h.kubeconfig)
	}

	cmd := exec.CommandContext(ctx, "helm", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return nil, fmt.Errorf("failed to list releases: %s", string(output))
	}

	// Parse the JSON output (simplified - would need proper JSON parsing in production)
	log.Infof("Helm list output: %s", string(output))

	return []HelmReleaseInfo{}, nil
}

// SaveUploadedChart saves an uploaded chart file to a temporary location
func SaveUploadedChart(data []byte, filename string) (string, error) {
	// Create a temp directory for charts if it doesn't exist
	chartDir := filepath.Join(os.TempDir(), "litmus-helm-charts")
	if err := os.MkdirAll(chartDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create chart directory: %w", err)
	}

	// Create a unique filename with timestamp
	timestamp := time.Now().Unix()
	safeFilename := filepath.Base(filename)
	chartPath := filepath.Join(chartDir, fmt.Sprintf("%d-%s", timestamp, safeFilename))

	// Write the file
	if err := os.WriteFile(chartPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write chart file: %w", err)
	}

	log.Infof("Saved uploaded chart to: %s", chartPath)
	return chartPath, nil
}

// CleanupChartFile removes a temporary chart file
func CleanupChartFile(chartPath string) error {
	return os.Remove(chartPath)
}

// portForwardProcesses stores active port-forward processes
var portForwardProcesses = make(map[int]*exec.Cmd)

// StartPortForward starts port-forwarding to a service
func (h *HelmService) StartPortForward(namespace, serviceName string, servicePort, localPort int) (*PortForwardResponse, error) {
	// Check if port is already in use by our processes
	if _, exists := portForwardProcesses[localPort]; exists {
		return &PortForwardResponse{
			Success: false,
			Message: fmt.Sprintf("Port %d is already being used for port-forwarding", localPort),
		}, fmt.Errorf("port %d already in use", localPort)
	}

	// Build kubectl port-forward command
	args := []string{
		"port-forward",
		fmt.Sprintf("svc/%s", serviceName),
		fmt.Sprintf("%d:%d", localPort, servicePort),
		"-n", namespace,
	}

	if h.kubeconfig != "" {
		args = append(args, "--kubeconfig", h.kubeconfig)
	}

	log.Infof("Executing kubectl command: kubectl %s", strings.Join(args, " "))

	// Start the port-forward process in the background
	cmd := exec.Command("kubectl", args...)

	// Capture output for debugging
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		log.Errorf("Failed to start port-forward: %v", err)
		return &PortForwardResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to start port-forward: %v", err),
		}, err
	}

	// Store the process
	portForwardProcesses[localPort] = cmd

	// Give it a moment to establish
	time.Sleep(2 * time.Second)

	// Check if process is still running
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		delete(portForwardProcesses, localPort)
		return &PortForwardResponse{
			Success: false,
			Message: "Port-forward process exited immediately. Check if the service exists and is running.",
		}, fmt.Errorf("port-forward exited")
	}

	url := fmt.Sprintf("http://localhost:%d", localPort)
	log.Infof("Port-forward started successfully: %s -> %s:%d", url, serviceName, servicePort)

	return &PortForwardResponse{
		Success:   true,
		Message:   fmt.Sprintf("Port-forward started. Access the service at %s", url),
		URL:       url,
		LocalPort: localPort,
	}, nil
}

// StopPortForward stops a port-forward process
func (h *HelmService) StopPortForward(localPort int) error {
	cmd, exists := portForwardProcesses[localPort]
	if !exists {
		return fmt.Errorf("no port-forward found on port %d", localPort)
	}

	if cmd.Process != nil {
		err := cmd.Process.Kill()
		if err != nil {
			log.Warnf("Error killing port-forward process: %v", err)
		}
	}

	delete(portForwardProcesses, localPort)
	log.Infof("Port-forward stopped on port %d", localPort)
	return nil
}

// GetActivePortForwards returns all active port-forward processes
func (h *HelmService) GetActivePortForwards() []PortForwardInfo {
	var forwards []PortForwardInfo
	for port, cmd := range portForwardProcesses {
		if cmd.Process != nil {
			forwards = append(forwards, PortForwardInfo{
				LocalPort: port,
				ProcessID: cmd.Process.Pid,
			})
		}
	}
	return forwards
}
