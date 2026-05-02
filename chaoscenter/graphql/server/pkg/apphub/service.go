package apphub

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	chaosHubOps "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaoshub/ops"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"
	log "github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	DefaultAppHubID           = "apphub-default-001"
	DefaultAppHubName         = "app-charts"
	DefaultAppHubSyncInterval = 6 * time.Hour
)

// Service defines the AppsHub service interface.
type Service interface {
	ListAppHubCategories(ctx context.Context, projectID string) ([]*model.AppHubCategory, error)
	GetAppHubStatus(ctx context.Context, projectID string) (*model.AppHubStatus, error)
	SyncDefaultAppHub()
}

type appHubService struct{}

// NewService returns a new AppsHub service instance.
func NewService() Service {
	return &appHubService{}
}

// getAppHubGitURL returns the Git URL for the app hub.
func getAppHubGitURL() string {
	url := utils.Config.DefaultAppHubGitURL
	if url == "" {
		url = "https://github.com/agentcert/app-charts"
	}
	return url
}

// getAppHubBranch returns the Git branch for the app hub.
func getAppHubBranch() string {
	branch := utils.Config.DefaultAppHubBranchName
	if branch == "" {
		branch = "main"
	}
	return branch
}

func isCustomAppHubMode() bool {
	return strings.EqualFold(strings.TrimSpace(utils.Config.AppHubSourceMode), "custom")
}

// getDefaultBasePath returns the OS-appropriate base path for default hub clones.
func getDefaultBasePath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.TempDir(), "default")
	}
	return "/tmp/default"
}

// getAppChartsPath returns the filesystem path where app charts are cloned.
// Expects charts to be at {basePath}/charts.
func getAppChartsPath() string {
	path := strings.TrimSpace(utils.Config.DefaultAppHubPath)
	if path == "" {
		path = getDefaultBasePath()
	}
	return filepath.Join(path, "charts")
}

// getAppClonePath returns the filesystem path for the app hub clone.
func getAppClonePath() string {
	path := strings.TrimSpace(utils.Config.DefaultAppHubPath)
	if path == "" {
		path = getDefaultBasePath()
	}
	return path
}

// ListAppHubCategories reads app charts from the filesystem and enriches
// them with live deployment status from the Kubernetes cluster.
func (s *appHubService) ListAppHubCategories(ctx context.Context, projectID string) ([]*model.AppHubCategory, error) {
	chartsPath := getAppChartsPath()

	// Read chart data from filesystem
	categories, err := GetAppChartsData(chartsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read app charts: %w", err)
	}

	// Enrich with deployment status from Kubernetes
	s.enrichWithDeploymentStatus(ctx, categories)

	return categories, nil
}

// enrichWithDeploymentStatus queries the Kubernetes API to determine
// which applications and microservices are running in the cluster.
func (s *appHubService) enrichWithDeploymentStatus(ctx context.Context, categories []*model.AppHubCategory) {
	// Create in-cluster Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		log.WithError(err).Debug("not running in-cluster, skipping deployment status enrichment")
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.WithError(err).Warn("failed to create Kubernetes client for enrichment")
		return
	}

	for _, category := range categories {
		for _, app := range category.Applications {
			ns := app.Namespace

			// List deployments in the namespace
			deployments, err := clientset.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
			if err != nil || len(deployments.Items) == 0 {
				app.IsDeployed = false
				continue
			}

			app.IsDeployed = true
			running := 0
			for _, ms := range app.Microservices {
				for _, dep := range deployments.Items {
					if dep.Name == ms.Name {
						ready := int(dep.Status.ReadyReplicas)
						desired := 1
						if dep.Spec.Replicas != nil {
							desired = int(*dep.Spec.Replicas)
						}
						ms.ReadyReplicas = &ready
						ms.DesiredReplicas = &desired
						isRunning := ready > 0
						ms.IsRunning = &isRunning
						if isRunning {
							running++
						}
						break
					}
				}
			}
			runningStr := fmt.Sprintf("%d/%d", running, len(app.Microservices))
			app.RunningServices = &runningStr
		}
	}
}

// GetAppHubStatus returns the current status of the AppsHub.
func (s *appHubService) GetAppHubStatus(ctx context.Context, projectID string) (*model.AppHubStatus, error) {
	chartsPath := getAppChartsPath()

	// Check if the hub is available (cloned)
	isAvailable := true
	if _, err := os.Stat(chartsPath); os.IsNotExist(err) {
		isAvailable = false
	}

	// Count apps
	totalApps := 0
	deployedApps := 0
	if isAvailable {
		categories, err := GetAppChartsData(chartsPath)
		if err == nil {
			for _, cat := range categories {
				for _, app := range cat.Applications {
					totalApps++
					if app.IsDeployed {
						deployedApps++
					}
				}
			}
		}
	}

	return &model.AppHubStatus{
		ID:           DefaultAppHubID,
		Name:         DefaultAppHubName,
		RepoURL:      getAppHubGitURL(),
		RepoBranch:   getAppHubBranch(),
		IsAvailable:  isAvailable,
		TotalApps:    totalApps,
		DeployedApps: deployedApps,
		IsDefault:    true,
		LastSyncedAt: time.Now().Format(time.RFC3339),
	}, nil
}

// SyncDefaultAppHub is a background goroutine that periodically clones/pulls
// the app-charts repo, analogous to SyncDefaultChaosHubs.
func (s *appHubService) SyncDefaultAppHub() {
	if isCustomAppHubMode() {
		log.WithField("path", getAppClonePath()).Info("app hub source mode is custom; skipping default git sync")
		return
	}

	log.Info("starting default app hub sync goroutine")
	for {
		repoURL := getAppHubGitURL()
		branch := getAppHubBranch()

		clonePath := getAppClonePath()

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(clonePath), 0755); err != nil {
			log.WithError(err).Error("failed to create parent directory for app hub")
			time.Sleep(DefaultAppHubSyncInterval)
			continue
		}

		var err error
		if _, statErr := os.Stat(filepath.Join(clonePath, ".git")); os.IsNotExist(statErr) {
			log.WithFields(log.Fields{
				"repoURL":   repoURL,
				"branch":    branch,
				"clonePath": clonePath,
			}).Info("cloning app hub repo")
			err = chaosHubOps.ClonePublicRepoToPath(repoURL, branch, clonePath)
		} else {
			log.WithFields(log.Fields{
				"clonePath": clonePath,
				"branch":    branch,
			}).Info("pulling latest for app hub repo")
			err = chaosHubOps.PullRepoAtPath(clonePath, branch)
		}

		if err != nil {
			log.WithFields(log.Fields{
				"repoUrl": repoURL,
				"branch":  branch,
				"hubName": DefaultAppHubName,
			}).WithError(err).Error("failed to sync default app hub")
		} else {
			log.WithFields(log.Fields{
				"repoUrl": repoURL,
				"branch":  branch,
			}).Info("successfully synced default app hub")
		}

		time.Sleep(DefaultAppHubSyncInterval)
	}
}
