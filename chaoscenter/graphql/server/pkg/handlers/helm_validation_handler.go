package handlers

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// HelmValidationResponse represents the response structure for helm validation
type HelmValidationResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	Details     string `json:"details,omitempty"`
	ReleaseName string `json:"releaseName,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	ChartName   string `json:"chartName,omitempty"`
}

// ValidateHelmHandler handles the validation of helm charts
// It deploys the helm chart to the local Kubernetes cluster and checks the deployment status
func ValidateHelmHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the uploaded helm package file
		file, header, err := c.Request.FormFile("helmPackage")
		if err != nil {
			log.Errorf("Failed to get helm package file: %v", err)
			c.JSON(http.StatusBadRequest, HelmValidationResponse{
				Success: false,
				Message: "Failed to get helm package file",
				Details: err.Error(),
			})
			return
		}
		defer file.Close()

		// Get environment ID (optional)
		environmentId := c.Request.FormValue("environmentId")
		log.Infof("Validating helm chart for environment: %s", environmentId)

		// Get namespace from form (default to "default" if not provided)
		namespace := c.Request.FormValue("namespace")
		if namespace == "" {
			namespace = "default"
		}
		log.Infof("Using namespace: %s", namespace)

		// Create a temporary directory to store the helm package
		tempDir, err := os.MkdirTemp("", "helm-validation-*")
		if err != nil {
			log.Errorf("Failed to create temp directory: %v", err)
			c.JSON(http.StatusInternalServerError, HelmValidationResponse{
				Success: false,
				Message: "Failed to create temporary directory",
				Details: err.Error(),
			})
			return
		}
		defer os.RemoveAll(tempDir)

		// Save the uploaded file
		tempFilePath := filepath.Join(tempDir, header.Filename)
		tempFile, err := os.Create(tempFilePath)
		if err != nil {
			log.Errorf("Failed to create temp file: %v", err)
			c.JSON(http.StatusInternalServerError, HelmValidationResponse{
				Success: false,
				Message: "Failed to save helm package",
				Details: err.Error(),
			})
			return
		}

		_, err = io.Copy(tempFile, file)
		tempFile.Close()
		if err != nil {
			log.Errorf("Failed to save helm package: %v", err)
			c.JSON(http.StatusInternalServerError, HelmValidationResponse{
				Success: false,
				Message: "Failed to save helm package",
				Details: err.Error(),
			})
			return
		}

		// Extract chart name from the package
		chartName, err := extractChartName(tempFilePath)
		if err != nil {
			log.Errorf("Failed to extract chart name: %v", err)
			c.JSON(http.StatusBadRequest, HelmValidationResponse{
				Success: false,
				Message: "Failed to extract chart name from package",
				Details: err.Error(),
			})
			return
		}

		// Generate a unique release name for validation
		releaseName := fmt.Sprintf("validate-%s-%d", chartName, time.Now().Unix())

		// Check if kubectl is available and connected to a cluster
		if err := checkKubernetesConnection(); err != nil {
			log.Errorf("Kubernetes connection check failed: %v", err)
			c.JSON(http.StatusInternalServerError, HelmValidationResponse{
				Success: false,
				Message: "Failed to connect to Kubernetes cluster",
				Details: err.Error(),
			})
			return
		}

		// Deploy the helm chart
		deployOutput, err := deployHelmChart(tempFilePath, releaseName, namespace)
		if err != nil {
			log.Errorf("Helm deployment failed: %v, output: %s", err, deployOutput)
			// Cleanup on failure
			cleanupHelmRelease(releaseName, namespace)
			c.JSON(http.StatusOK, HelmValidationResponse{
				Success: false,
				Message: "Deployment failed, please check the logs for more details",
				Details: deployOutput,
			})
			return
		}

		log.Infof("Helm install successful, waiting for pods to start...")

		// Wait for pods to be in Running state (increased timeout to 3 minutes)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		// Detect the namespace where pods are created (helm charts may create their own namespace)
		detectedNamespace := detectHelmNamespace(releaseName, namespace)
		log.Infof("Detected namespace for pods: %s", detectedNamespace)

		healthy, checkDetails := checkDeploymentHealth(ctx, releaseName, detectedNamespace)

		// NOTE: We do NOT cleanup here anymore. Cleanup will happen when user clicks Onboard.
		// This allows manual validation if needed.
		if healthy {
			log.Infof("Validation successful. Release '%s' in namespace '%s' is ready for manual inspection or onboarding.", releaseName, detectedNamespace)
			c.JSON(http.StatusOK, HelmValidationResponse{
				Success:     true,
				Message:     "Validated successfully. Application is running and ready for onboarding.",
				Details:     checkDetails,
				ReleaseName: releaseName,
				Namespace:   detectedNamespace,
				ChartName:   chartName,
			})
		} else {
			// Cleanup on validation failure
			cleanupOutput := cleanupHelmRelease(releaseName, namespace)
			log.Infof("Cleanup output (validation failed): %s", cleanupOutput)
			c.JSON(http.StatusOK, HelmValidationResponse{
				Success: false,
				Message: "Deployment failed, please check the logs for more details",
				Details: checkDetails,
			})
		}
	}
}

// extractChartName extracts the chart name from the helm package (.tgz file)
func extractChartName(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		// The first directory in the tar is usually the chart name
		if header.Typeflag == tar.TypeDir {
			name := strings.TrimSuffix(header.Name, "/")
			if !strings.Contains(name, "/") {
				return name, nil
			}
		}
	}

	// Fallback: extract from filename
	baseName := filepath.Base(filePath)
	baseName = strings.TrimSuffix(baseName, ".tgz")
	// Remove version suffix (e.g., chart-name-1.0.0 -> chart-name)
	parts := strings.Split(baseName, "-")
	if len(parts) > 1 {
		// Check if last part looks like a version
		for i := len(parts) - 1; i >= 0; i-- {
			if !isVersionLike(parts[i]) {
				return strings.Join(parts[:i+1], "-"), nil
			}
		}
	}
	return baseName, nil
}

// isVersionLike checks if a string looks like a version number
func isVersionLike(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Simple check: starts with a digit
	return s[0] >= '0' && s[0] <= '9'
}

// checkKubernetesConnection verifies that kubectl can connect to a cluster
func checkKubernetesConnection() error {
	cmd := exec.Command("kubectl", "cluster-info")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl cluster-info failed: %v, output: %s", err, string(output))
	}
	return nil
}

// deployHelmChart deploys a helm chart to the specified namespace
func deployHelmChart(chartPath, releaseName, namespace string) (string, error) {
	// Don't use --wait since we'll check pod status ourselves
	// This allows faster deployment and custom health checking
	cmd := exec.Command("helm", "install", releaseName, chartPath,
		"--namespace", namespace,
		"--create-namespace",
	)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// detectHelmNamespace detects which namespace the helm release deployed pods to
// Helm charts can deploy to different namespaces than where they're installed
func detectHelmNamespace(releaseName, defaultNamespace string) string {
	// First, try to get the list of resources deployed by this helm release
	cmd := exec.Command("helm", "get", "manifest", releaseName, "-n", defaultNamespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Warnf("Failed to get helm manifest: %v", err)
		return defaultNamespace
	}

	// Parse the manifest to find namespaces used
	manifest := string(output)
	namespaces := make(map[string]bool)
	
	// Look for namespace declarations in the manifest
	lines := strings.Split(manifest, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "namespace:") {
			ns := strings.TrimSpace(strings.TrimPrefix(trimmed, "namespace:"))
			ns = strings.Trim(ns, "\"'")
			if ns != "" {
				namespaces[ns] = true
			}
		}
	}

	// If we found a namespace other than the default, use it
	for ns := range namespaces {
		if ns != defaultNamespace && ns != "" {
			log.Infof("Detected namespace from manifest: %s", ns)
			return ns
		}
	}

	// Fallback: check which namespaces have pods
	cmd = exec.Command("kubectl", "get", "pods", "-A", "-o", "wide")
	podsOutput, err := cmd.CombinedOutput()
	if err == nil {
		podsStr := string(podsOutput)
		podsLines := strings.Split(podsStr, "\n")
		podNamespaces := make(map[string]int)
		
		for _, line := range podsLines {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				ns := fields[0]
				// Skip system namespaces
				if ns != "kube-system" && ns != "kube-public" && ns != "kube-node-lease" && ns != "local-path-storage" && ns != "NAMESPACE" {
					podNamespaces[ns]++
				}
			}
		}

		// If there's a non-default namespace with pods, prefer it
		for ns, count := range podNamespaces {
			if ns != "default" && count > 0 {
				log.Infof("Detected namespace from pods: %s (count: %d)", ns, count)
				return ns
			}
		}
	}

	return defaultNamespace
}

// checkDeploymentHealth checks if the deployed resources are healthy
// It polls for pods to be in Running state
func checkDeploymentHealth(ctx context.Context, releaseName, namespace string) (bool, string) {
	log.Infof("Checking deployment health for release %s in namespace %s", releaseName, namespace)
	
	// Wait a bit for resources to be created
	time.Sleep(5 * time.Second)
	
	// Get deployments created by this helm release using kubectl
	// The deployment names are needed to find the pods
	deplCmd := exec.CommandContext(ctx, "kubectl", "get", "deployments", "-n", namespace, "-o", "jsonpath={.items[*].metadata.name}")
	deplOutput, err := deplCmd.CombinedOutput()
	if err != nil {
		log.Warnf("Failed to get deployments: %v, output: %s", err, string(deplOutput))
	}
	deploymentNames := strings.Fields(strings.TrimSpace(string(deplOutput)))
	log.Infof("Found deployments in namespace %s: %v", namespace, deploymentNames)
	
	// Poll for pods to be running
	maxAttempts := 18 // 18 * 10s = 3 minutes
	for attempt := 0; attempt < maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return false, "Timeout waiting for pods to be ready"
		default:
		}

		// Get all pods in the namespace
		cmd := exec.CommandContext(ctx, "kubectl", "get", "pods",
			"-n", namespace,
			"-o", "wide",
		)
		podsOutput, err := cmd.CombinedOutput()
		if err != nil {
			log.Warnf("Attempt %d: Failed to get pods: %s", attempt+1, string(podsOutput))
			time.Sleep(10 * time.Second)
			continue
		}

		podsStr := string(podsOutput)
		log.Infof("Attempt %d: Pod status:\n%s", attempt+1, podsStr)

		// Filter pods that belong to this helm release
		lines := strings.Split(podsStr, "\n")
		relevantPods := []string{}
		for _, line := range lines {
			// Skip header line and empty lines
			if strings.HasPrefix(line, "NAME") || strings.TrimSpace(line) == "" {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) > 0 {
				podName := fields[0]
				// Check if pod name matches any deployment from the helm release
				for _, deployName := range deploymentNames {
					if strings.HasPrefix(podName, deployName) {
						relevantPods = append(relevantPods, line)
						break
					}
				}
			}
		}

		// If we found pods matching the release deployments, only check those
		var podsToCheck string
		if len(relevantPods) > 0 {
			podsToCheck = strings.Join(relevantPods, "\n")
			log.Infof("Attempt %d: Found %d pods matching helm deployments", attempt+1, len(relevantPods))
		} else if len(deploymentNames) == 0 {
			// If we couldn't get deployment names, be more lenient - just wait for any pods
			log.Infof("Attempt %d: No deployment names found, waiting for pods...", attempt+1)
			time.Sleep(10 * time.Second)
			continue
		} else {
			// Deployments exist but pods not created yet
			log.Infof("Attempt %d: Pods not created yet for deployments %v", attempt+1, deploymentNames)
			time.Sleep(10 * time.Second)
			continue
		}

		// Check for fatal errors only in relevant pods
		if strings.Contains(podsToCheck, "CrashLoopBackOff") ||
			strings.Contains(podsToCheck, "ImagePullBackOff") || 
			strings.Contains(podsToCheck, "ErrImagePull") ||
			strings.Contains(podsToCheck, "Error") {
			return false, fmt.Sprintf("Pods have errors:\n%s", podsToCheck)
		}

		// Count running pods
		runningCount := 0
		pendingCount := 0
		totalPods := 0
		
		checkLines := strings.Split(podsToCheck, "\n")
		for _, line := range checkLines {
			if strings.Contains(line, "Running") {
				runningCount++
				totalPods++
			} else if strings.Contains(line, "Pending") || strings.Contains(line, "ContainerCreating") {
				pendingCount++
				totalPods++
			} else if strings.Contains(line, "Completed") {
				totalPods++
			}
		}

		log.Infof("Attempt %d: Running=%d, Pending=%d, Total=%d", attempt+1, runningCount, pendingCount, totalPods)

		// If we have running pods and no pending pods, consider it successful
		if runningCount > 0 && pendingCount == 0 {
			// Get services info
			svcCmd := exec.CommandContext(ctx, "kubectl", "get", "services", "-n", namespace, "-o", "wide")
			svcOutput, _ := svcCmd.CombinedOutput()
			return true, fmt.Sprintf("Pods:\n%s\nServices:\n%s", podsStr, string(svcOutput))
		}

		// If no pods found yet, wait and try again
		if totalPods == 0 {
			log.Infof("Attempt %d: No pods found yet, waiting...", attempt+1)
		}

		time.Sleep(10 * time.Second)
	}

	// Final check
	cmd := exec.CommandContext(ctx, "kubectl", "get", "pods", "-n", namespace, "-o", "wide")
	podsOutput, _ := cmd.CombinedOutput()
	return false, fmt.Sprintf("Timeout - pods not ready:\n%s", string(podsOutput))
}

// cleanupHelmRelease removes the helm release
func cleanupHelmRelease(releaseName, namespace string) string {
	cmd := exec.Command("helm", "uninstall", releaseName, "--namespace", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Warnf("Failed to cleanup helm release %s: %v", releaseName, err)
	}
	return string(output)
}

// NamespaceListResponse represents the response for listing namespaces
type NamespaceListResponse struct {
	Success    bool     `json:"success"`
	Namespaces []string `json:"namespaces"`
	Message    string   `json:"message,omitempty"`
}

// ListNamespacesHandler returns a handler that lists all available Kubernetes namespaces
func ListNamespacesHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if kubectl is available and connected to a cluster
		if err := checkKubernetesConnection(); err != nil {
			log.Errorf("Kubernetes connection check failed: %v", err)
			c.JSON(http.StatusInternalServerError, NamespaceListResponse{
				Success: false,
				Message: "Failed to connect to Kubernetes cluster: " + err.Error(),
			})
			return
		}

		// Get all namespaces using kubectl
		cmd := exec.Command("kubectl", "get", "namespaces", "-o", "jsonpath={.items[*].metadata.name}")
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Errorf("Failed to get namespaces: %v, output: %s", err, string(output))
			c.JSON(http.StatusInternalServerError, NamespaceListResponse{
				Success: false,
				Message: "Failed to list namespaces: " + err.Error(),
			})
			return
		}

		// Parse the output (space-separated namespace names)
		namespacesStr := strings.TrimSpace(string(output))
		var namespaces []string
		if namespacesStr != "" {
			namespaces = strings.Split(namespacesStr, " ")
		}

		// Filter out empty strings
		filteredNamespaces := make([]string, 0, len(namespaces))
		for _, ns := range namespaces {
			ns = strings.TrimSpace(ns)
			if ns != "" {
				filteredNamespaces = append(filteredNamespaces, ns)
			}
		}

		log.Infof("Found %d namespaces", len(filteredNamespaces))
		c.JSON(http.StatusOK, NamespaceListResponse{
			Success:    true,
			Namespaces: filteredNamespaces,
		})
	}
}
