package handlers

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/authorization"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/helm"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	// MaxUploadSize is the maximum file size for helm chart upload (50MB)
	MaxUploadSize = 50 << 20 // 50 MB
)

// HelmChartUploadHandler handles Helm chart file uploads and installation
func HelmChartUploadHandler(mongoClient *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Verify JWT token from Authorization header
		jwt := ""
		if c.Request.Header.Get("Authorization") != "" {
			jwt = c.Request.Header.Get("Authorization")
			if len(jwt) > 7 && jwt[:7] == "Bearer " {
				jwt = jwt[7:]
			}
		}

		if jwt == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Authorization token is required",
			})
			return
		}

		// Check if token is revoked
		if authorization.IsRevokedToken(jwt, mongoClient) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Token is revoked",
			})
			return
		}

		// Limit request body size
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxUploadSize)

		// Parse multipart form
		if err := c.Request.ParseMultipartForm(MaxUploadSize); err != nil {
			log.Errorf("Failed to parse multipart form: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Failed to parse form data. File may be too large (max 50MB).",
			})
			return
		}

		// Get form values
		environmentID := c.PostForm("environmentId")
		environmentName := c.PostForm("environmentName")
		projectID := c.PostForm("projectId")
		releaseName := c.PostForm("releaseName")
		namespace := c.PostForm("namespace")

		// Validate required fields
		if environmentID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "environmentId is required",
			})
			return
		}

		if environmentName == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "environmentName is required",
			})
			return
		}

		if projectID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "projectId is required",
			})
			return
		}

		// Get the uploaded file
		file, header, err := c.Request.FormFile("chartFile")
		if err != nil {
			log.Errorf("Failed to get uploaded file: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Chart file is required. Please upload a .tgz file.",
			})
			return
		}
		defer file.Close()

		// Validate file extension
		filename := header.Filename
		if len(filename) < 4 || filename[len(filename)-4:] != ".tgz" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid file type. Only .tgz files are allowed.",
			})
			return
		}

		log.Infof("Received Helm chart upload: file=%s, environmentID=%s, environmentName=%s, projectID=%s",
			filename, environmentID, environmentName, projectID)

		// Read file content
		fileContent, err := io.ReadAll(file)
		if err != nil {
			log.Errorf("Failed to read file content: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to read uploaded file",
			})
			return
		}

		// Save the chart to a temporary file
		chartPath, err := helm.SaveUploadedChart(fileContent, filename)
		if err != nil {
			log.Errorf("Failed to save chart file: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to save chart file",
			})
			return
		}

		// Create the install request
		installRequest := &helm.HelmChartInstallRequest{
			EnvironmentID:   environmentID,
			EnvironmentName: environmentName,
			ProjectID:       projectID,
			ReleaseName:     releaseName,
			Namespace:       namespace,
		}

		// Create Helm service and install the chart
		// Use kubeconfig from environment or default
		kubeconfig := utils.Config.KubeConfigFilePath
		helmService := helm.NewHelmService(kubeconfig)

		response, err := helmService.InstallOrUpgradeChart(c.Request.Context(), chartPath, installRequest)

		// Clean up the temporary file
		if cleanupErr := helm.CleanupChartFile(chartPath); cleanupErr != nil {
			log.Warnf("Failed to cleanup chart file: %v", cleanupErr)
		}

		if err != nil {
			log.Errorf("Helm install/upgrade failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": response.Message,
			})
			return
		}

		log.Infof("Helm chart installed successfully: release=%s, namespace=%s", response.ReleaseName, response.Namespace)

		c.JSON(http.StatusOK, gin.H{
			"success":       true,
			"message":       response.Message,
			"releaseName":   response.ReleaseName,
			"namespace":     response.Namespace,
			"status":        response.Status,
			"environmentId": environmentID,
		})
	}
}

// HelmChartUninstallHandler handles uninstalling Helm releases
func HelmChartUninstallHandler(mongoClient *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Verify JWT token
		jwt := ""
		if c.Request.Header.Get("Authorization") != "" {
			jwt = c.Request.Header.Get("Authorization")
			if len(jwt) > 7 && jwt[:7] == "Bearer " {
				jwt = jwt[7:]
			}
		}

		if jwt == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Authorization token is required",
			})
			return
		}

		if authorization.IsRevokedToken(jwt, mongoClient) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Token is revoked",
			})
			return
		}

		// Parse JSON body
		var request struct {
			ReleaseName string `json:"releaseName" binding:"required"`
			Namespace   string `json:"namespace" binding:"required"`
			ProjectID   string `json:"projectId" binding:"required"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid request body: " + err.Error(),
			})
			return
		}

		log.Infof("Received Helm chart uninstall request: release=%s, namespace=%s, projectID=%s",
			request.ReleaseName, request.Namespace, request.ProjectID)

		// Create Helm service
		kubeconfig := utils.Config.KubeConfigFilePath
		helmService := helm.NewHelmService(kubeconfig)

		response, err := helmService.UninstallChart(c.Request.Context(), request.ReleaseName, request.Namespace)
		if err != nil {
			log.Errorf("Helm uninstall failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": response.Message,
			})
			return
		}

		log.Infof("Helm chart uninstalled successfully: release=%s, namespace=%s", request.ReleaseName, request.Namespace)

		c.JSON(http.StatusOK, gin.H{
			"success":     true,
			"message":     response.Message,
			"releaseName": response.ReleaseName,
			"namespace":   response.Namespace,
			"status":      response.Status,
		})
	}
}

// HelmChartListHandler handles listing Helm releases
func HelmChartListHandler(mongoClient *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Verify JWT token
		jwt := ""
		if c.Request.Header.Get("Authorization") != "" {
			jwt = c.Request.Header.Get("Authorization")
			if len(jwt) > 7 && jwt[:7] == "Bearer " {
				jwt = jwt[7:]
			}
		}

		if jwt == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Authorization token is required",
			})
			return
		}

		if authorization.IsRevokedToken(jwt, mongoClient) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Token is revoked",
			})
			return
		}

		namespace := c.Query("namespace")

		log.Infof("Received Helm releases list request: namespace=%s", namespace)

		// Create Helm service
		kubeconfig := utils.Config.KubeConfigFilePath
		helmService := helm.NewHelmService(kubeconfig)

		releases, err := helmService.ListReleases(c.Request.Context(), namespace)
		if err != nil {
			log.Errorf("Failed to list Helm releases: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to list Helm releases: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"releases": releases,
		})
	}
}

// PortForwardHandler handles port-forwarding to a service in the cluster
func PortForwardHandler(mongoClient *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Verify JWT token
		jwt := ""
		if c.Request.Header.Get("Authorization") != "" {
			jwt = c.Request.Header.Get("Authorization")
			if len(jwt) > 7 && jwt[:7] == "Bearer " {
				jwt = jwt[7:]
			}
		}

		if jwt == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Authorization token is required",
			})
			return
		}

		if authorization.IsRevokedToken(jwt, mongoClient) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Token is revoked",
			})
			return
		}

		// Parse JSON body
		var request struct {
			Namespace     string `json:"namespace" binding:"required"`
			ServiceName   string `json:"serviceName" binding:"required"`
			ServicePort   int    `json:"servicePort" binding:"required"`
			LocalPort     int    `json:"localPort" binding:"required"`
			ProjectID     string `json:"projectId" binding:"required"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid request body: " + err.Error(),
			})
			return
		}

		log.Infof("Received port-forward request: namespace=%s, service=%s, servicePort=%d, localPort=%d",
			request.Namespace, request.ServiceName, request.ServicePort, request.LocalPort)

		// Create Helm service for port-forwarding
		kubeconfig := utils.Config.KubeConfigFilePath
		helmService := helm.NewHelmService(kubeconfig)

		response, err := helmService.StartPortForward(
			request.Namespace,
			request.ServiceName,
			request.ServicePort,
			request.LocalPort,
		)

		if err != nil {
			log.Errorf("Port-forward failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": response.Message,
			})
			return
		}

		log.Infof("Port-forward started successfully: %s", response.Message)

		c.JSON(http.StatusOK, gin.H{
			"success":     true,
			"message":     response.Message,
			"localPort":   request.LocalPort,
			"servicePort": request.ServicePort,
			"serviceName": request.ServiceName,
			"namespace":   request.Namespace,
			"url":         response.URL,
		})
	}
}

// StopPortForwardHandler handles stopping port-forward
func StopPortForwardHandler(mongoClient *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Verify JWT token
		jwt := ""
		if c.Request.Header.Get("Authorization") != "" {
			jwt = c.Request.Header.Get("Authorization")
			if len(jwt) > 7 && jwt[:7] == "Bearer " {
				jwt = jwt[7:]
			}
		}

		if jwt == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Authorization token is required",
			})
			return
		}

		if authorization.IsRevokedToken(jwt, mongoClient) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Token is revoked",
			})
			return
		}

		// Parse JSON body
		var request struct {
			LocalPort int `json:"localPort" binding:"required"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid request body: " + err.Error(),
			})
			return
		}

		log.Infof("Received stop port-forward request: localPort=%d", request.LocalPort)

		kubeconfig := utils.Config.KubeConfigFilePath
		helmService := helm.NewHelmService(kubeconfig)

		err := helmService.StopPortForward(request.LocalPort)
		if err != nil {
			log.Errorf("Failed to stop port-forward: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to stop port-forward: " + err.Error(),
			})
			return
		}

		log.Infof("Port-forward stopped successfully on port %d", request.LocalPort)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Port-forward stopped successfully",
		})
	}
}
