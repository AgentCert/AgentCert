package apps_registry

import (
	"context"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

// Handler handles HTTP requests for apps registry.
type Handler struct {
	operator Operator
}

// NewHandler creates a new Handler instance.
func NewHandler(database *mongo.Database) *Handler {
	return &Handler{
		operator: NewOperator(database),
	}
}

// RegisterAppRequest represents the request body for registering an app.
type RegisterAppRequest struct {
	ProjectID        string            `json:"projectId"`
	Name             string            `json:"name"`
	Version          string            `json:"version"`
	Description      string            `json:"description,omitempty"`
	ChartName        string            `json:"chartName,omitempty"`
	Namespace        string            `json:"namespace"`
	EnvironmentID    string            `json:"environmentId"`
	Method           string            `json:"method"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	ReleaseName      string            `json:"releaseName,omitempty"`      // Helm release name for cleanup
	ReleaseNamespace string            `json:"releaseNamespace,omitempty"` // Namespace where helm release was deployed
}

// RegisterAppResponse represents the response for registering an app.
type RegisterAppResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	App     *App   `json:"app,omitempty"`
}

// ListAppsResponse represents the response for listing apps.
type ListAppsResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	Apps       []*App `json:"apps"`
	TotalCount int64  `json:"totalCount"`
}

// RegisterApp handles the registration of a new application.
func (h *Handler) RegisterApp() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RegisterAppRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Errorf("Failed to parse register app request: %v", err)
			c.JSON(http.StatusBadRequest, RegisterAppResponse{
				Success: false,
				Message: "Invalid request body",
			})
			return
		}

		// Validate required fields
		if req.Name == "" {
			c.JSON(http.StatusBadRequest, RegisterAppResponse{
				Success: false,
				Message: "App name is required",
			})
			return
		}

		if req.ProjectID == "" {
			c.JSON(http.StatusBadRequest, RegisterAppResponse{
				Success: false,
				Message: "Project ID is required",
			})
			return
		}

		// Cleanup Helm release FIRST before any other operations (if release info provided)
		// This ensures resources are freed even if subsequent validations fail
		if req.ReleaseName != "" {
			releaseNamespace := req.ReleaseNamespace
			if releaseNamespace == "" {
				releaseNamespace = "default"
			}
			log.Infof("Cleaning up Helm release '%s' in namespace '%s' before onboarding", req.ReleaseName, releaseNamespace)
			
			// Step 1: Uninstall the Helm release
			cleanupCmd := exec.Command("helm", "uninstall", req.ReleaseName, "-n", "default")
			cleanupOutput, cleanupErr := cleanupCmd.CombinedOutput()
			if cleanupErr != nil {
				log.Warnf("Helm cleanup warning (may already be deleted): %v, output: %s", cleanupErr, string(cleanupOutput))
				// Don't fail the onboarding if cleanup fails - the release might already be gone
			} else {
				log.Infof("Successfully cleaned up Helm release '%s'", req.ReleaseName)
			}

			// Step 2: Delete the namespace created by the chart (if different from default)
			// Some charts like sock-shop create their own namespace
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
		}

		ctx := context.Background()

		// Check for duplicate name
		existingApp, err := h.operator.GetAppByProjectAndName(ctx, req.ProjectID, req.Name)
		if err != nil && err != ErrAppNotFound {
			log.Errorf("Failed to check for existing app: %v", err)
			c.JSON(http.StatusInternalServerError, RegisterAppResponse{
				Success: false,
				Message: "Failed to check for existing app",
			})
			return
		}

		if existingApp != nil {
			c.JSON(http.StatusConflict, RegisterAppResponse{
				Success: false,
				Message: "An app with this name already exists in the project",
			})
			return
		}

		// Create the app
		now := time.Now().Unix()
		app := &App{
			AppID:         uuid.New().String(),
			ProjectID:     req.ProjectID,
			Name:          req.Name,
			Version:       req.Version,
			Description:   req.Description,
			ChartName:     req.ChartName,
			Namespace:     req.Namespace,
			EnvironmentID: req.EnvironmentID,
			Method:        req.Method,
			Status:        AppStatusRegistered,
			AuditInfo: &AuditInfo{
				CreatedAt: now,
				CreatedBy: "system",
				UpdatedAt: now,
				UpdatedBy: "system",
			},
		}

		// Add metadata if provided
		if len(req.Metadata) > 0 {
			app.Metadata = &AppMetadata{
				Labels: req.Metadata,
			}
		}

		// Save to database
		if err := h.operator.CreateApp(ctx, app); err != nil {
			log.Errorf("Failed to create app: %v", err)
			c.JSON(http.StatusInternalServerError, RegisterAppResponse{
				Success: false,
				Message: "Failed to register app",
			})
			return
		}

		log.Infof("Successfully registered app: %s (ID: %s)", app.Name, app.AppID)

		c.JSON(http.StatusOK, RegisterAppResponse{
			Success: true,
			Message: "App registered successfully",
			App:     app,
		})
	}
}

// ListApps handles listing all registered applications.
func (h *Handler) ListApps() gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Query("projectId")
		environmentID := c.Query("environmentId")
		searchTerm := c.Query("search")
		status := c.Query("status")

		// Parse pagination parameters
		skip := 0
		limit := 50
		if skipStr := c.Query("skip"); skipStr != "" {
			if s, err := strconv.Atoi(skipStr); err == nil && s >= 0 {
				skip = s
			}
		}
		if limitStr := c.Query("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
				limit = l
			}
		}

		ctx := context.Background()

		filter := &AppFilter{
			ProjectID:     projectID,
			EnvironmentID: environmentID,
			SearchTerm:    searchTerm,
		}

		if status != "" {
			filter.Status = AppStatus(status)
		}

		apps, totalCount, err := h.operator.ListApps(ctx, filter, skip, limit)
		if err != nil {
			log.Errorf("Failed to list apps: %v", err)
			c.JSON(http.StatusInternalServerError, ListAppsResponse{
				Success: false,
				Message: "Failed to list apps",
			})
			return
		}

		c.JSON(http.StatusOK, ListAppsResponse{
			Success:    true,
			Message:    "Apps retrieved successfully",
			Apps:       apps,
			TotalCount: totalCount,
		})
	}
}

// GetApp handles getting a single application by ID.
func (h *Handler) GetApp() gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.Param("appId")
		if appID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "App ID is required",
			})
			return
		}

		ctx := context.Background()
		app, err := h.operator.GetApp(ctx, appID)
		if err != nil {
			if err == ErrAppNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"message": "App not found",
				})
				return
			}
			log.Errorf("Failed to get app: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to get app",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"app":     app,
		})
	}
}

// DeleteApp handles soft-deleting an application.
func (h *Handler) DeleteApp() gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.Param("appId")
		if appID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "App ID is required",
			})
			return
		}

		ctx := context.Background()
		if err := h.operator.DeleteApp(ctx, appID); err != nil {
			if err == ErrAppNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"message": "App not found",
				})
				return
			}
			log.Errorf("Failed to delete app: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to delete app",
			})
			return
		}

		log.Infof("Successfully deleted app: %s", appID)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "App deleted successfully",
		})
	}
}
