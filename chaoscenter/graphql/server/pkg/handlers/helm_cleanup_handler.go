package handlers

import (
	"net/http"
	"os/exec"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// CleanupHelmRequest represents the request structure for helm cleanup
type CleanupHelmRequest struct {
	ReleaseName string `json:"releaseName"`
	Namespace   string `json:"namespace"`
}

// CleanupHelmResponse represents the response structure for helm cleanup
type CleanupHelmResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// CleanupHelmHandler handles the cleanup of helm releases
func CleanupHelmHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CleanupHelmRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Errorf("Failed to parse cleanup request: %v", err)
			c.JSON(http.StatusBadRequest, CleanupHelmResponse{
				Success: false,
				Message: "Invalid request body",
			})
			return
		}

		if req.ReleaseName == "" {
			c.JSON(http.StatusBadRequest, CleanupHelmResponse{
				Success: false,
				Message: "Release name is required",
			})
			return
		}

		releaseNamespace := req.Namespace
		if releaseNamespace == "" {
			releaseNamespace = "default"
		}

		log.Infof("Cleaning up Helm release '%s' in namespace '%s'", req.ReleaseName, releaseNamespace)

		// Step 1: Uninstall the Helm release
		cleanupCmd := exec.Command("helm", "uninstall", req.ReleaseName, "-n", releaseNamespace)
		cleanupOutput, cleanupErr := cleanupCmd.CombinedOutput()
		if cleanupErr != nil {
			log.Warnf("Helm cleanup warning (may already be deleted): %v, output: %s", cleanupErr, string(cleanupOutput))
			// Don't fail - the release might already be gone
		} else {
			log.Infof("Successfully uninstalled Helm release '%s'", req.ReleaseName)
		}

		// Step 2: Delete the namespace created by the chart (if different from default)
		if releaseNamespace != "default" && releaseNamespace != "" {
			log.Infof("Deleting namespace '%s' created by the Helm chart", releaseNamespace)
			nsCleanupCmd := exec.Command("kubectl", "delete", "namespace", releaseNamespace, "--ignore-not-found")
			nsCleanupOutput, nsCleanupErr := nsCleanupCmd.CombinedOutput()
			if nsCleanupErr != nil {
				log.Warnf("Namespace cleanup warning: %v, output: %s", nsCleanupErr, string(nsCleanupOutput))
			} else {
				log.Infof("Successfully deleted namespace '%s'", releaseNamespace)
			}
		}

		c.JSON(http.StatusOK, CleanupHelmResponse{
			Success: true,
			Message: "Cleanup completed successfully",
		})
	}
}
