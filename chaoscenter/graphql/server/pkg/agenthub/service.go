package agenthub

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/agent_registry"
	chaosHubOps "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaoshub/ops"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"
	log "github.com/sirupsen/logrus"
)

const (
	DefaultAgentHubID               = "agenthub-default-001"
	DefaultAgentHubName             = "Agent Charts"
	DefaultAgentHubSyncInterval     = 6 * time.Hour
)

// Service defines the AgentHub service interface.
type Service interface {
	ListAgentHubCategories(ctx context.Context, projectID string) ([]*model.AgentHubCategory, error)
	GetAgentHubStatus(ctx context.Context, projectID string) (*model.AgentHubStatus, error)
	SyncDefaultAgentHub()
}

type agentHubService struct {
	agentRegistryService agent_registry.Service
}

// NewService returns a new AgentHub service instance.
func NewService(agentRegistryService agent_registry.Service) Service {
	return &agentHubService{
		agentRegistryService: agentRegistryService,
	}
}

// getAgentHubGitURL returns the Git URL for the agent hub.
func getAgentHubGitURL() string {
	url := utils.Config.DefaultAgentHubGitURL
	if url == "" {
		url = "https://github.com/agentcert/agent-charts"
	}
	return url
}

// getAgentHubBranch returns the Git branch for the agent hub.
func getAgentHubBranch() string {
	branch := utils.Config.DefaultAgentHubBranchName
	if branch == "" {
		branch = "main"
	}
	return branch
}

// getDefaultBasePath returns the OS-appropriate base path for default hub clones.
// On Windows it uses os.TempDir(), on Linux/Mac it uses /tmp.
func getDefaultBasePath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.TempDir(), "default")
	}
	return "/tmp/default"
}

// getAgentChartsPath returns the filesystem path where agent charts are cloned.
// Aligns with GetClonePath in chaoshub/ops which clones default hubs to {basePath}/{HubName}.
func getAgentChartsPath() string {
	path := utils.Config.DefaultAgentHubPath
	if path == "" {
		path = getDefaultBasePath()
	}
	return filepath.Join(path, DefaultAgentHubName, "charts")
}

// getAgentClonePath returns the filesystem path for the agent hub clone.
func getAgentClonePath() string {
	path := utils.Config.DefaultAgentHubPath
	if path == "" {
		path = getDefaultBasePath()
	}
	return filepath.Join(path, DefaultAgentHubName)
}

// ListAgentHubCategories reads agent charts from the filesystem and enriches
// them with live deployment status from the Agent Registry.
func (s *agentHubService) ListAgentHubCategories(ctx context.Context, projectID string) ([]*model.AgentHubCategory, error) {
	chartsPath := getAgentChartsPath()

	// Read chart data from filesystem (like GetChartsData in ChaosHub)
	categories, err := GetAgentChartsData(chartsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read agent charts: %w", err)
	}

	// Enrich with deployment status from Agent Registry
	if s.agentRegistryService != nil {
		s.enrichWithDeploymentStatus(ctx, projectID, categories)
	}

	return categories, nil
}

// enrichWithDeploymentStatus cross-references chart entries with the Agent Registry
// to populate isDeployed, deploymentStatus, agentID, namespace, helmReleaseName.
func (s *agentHubService) enrichWithDeploymentStatus(ctx context.Context, projectID string, categories []*model.AgentHubCategory) {
	agents, err := s.agentRegistryService.ListAgents(ctx, nil, nil)
	if err != nil {
		log.WithError(err).Warn("failed to list agents from registry for enrichment")
		return
	}

	// Build name→agent lookup map
	type agentInfo struct {
		ID              string
		Status          string
		Namespace       string
		HelmReleaseName string
	}
	deployedMap := make(map[string]agentInfo)
	for _, agent := range agents.Agents {
		deployedMap[agent.Name] = agentInfo{
			ID:              agent.AgentID,
			Status:          string(agent.Status),
			Namespace:       agent.Namespace,
			HelmReleaseName: agent.HelmReleaseName,
		}
	}

	// Enrich each chart entry with live status
	for _, category := range categories {
		for _, entry := range category.Agents {
			if info, ok := deployedMap[entry.Name]; ok {
				entry.IsDeployed = true
				entry.DeploymentStatus = &info.Status
				entry.AgentID = &info.ID
				entry.Namespace = &info.Namespace
				entry.HelmReleaseName = &info.HelmReleaseName
			}
		}
	}
}

// GetAgentHubStatus returns the current status of the AgentHub.
func (s *agentHubService) GetAgentHubStatus(ctx context.Context, projectID string) (*model.AgentHubStatus, error) {
	chartsPath := getAgentChartsPath()

	// Check if the hub is available (cloned)
	isAvailable := true
	if _, err := os.Stat(chartsPath); os.IsNotExist(err) {
		isAvailable = false
	}

	// Count agents
	totalAgents := 0
	deployedAgents := 0
	if isAvailable {
		categories, err := GetAgentChartsData(chartsPath)
		if err == nil {
			for _, cat := range categories {
				totalAgents += len(cat.Agents)
			}
		}

		// Count deployed agents from registry
		if s.agentRegistryService != nil {
			agents, err := s.agentRegistryService.ListAgents(ctx, nil, nil)
			if err == nil {
				deployedAgents = len(agents.Agents)
			}
		}
	}

	return &model.AgentHubStatus{
		ID:             DefaultAgentHubID,
		Name:           DefaultAgentHubName,
		RepoURL:        getAgentHubGitURL(),
		RepoBranch:     getAgentHubBranch(),
		IsAvailable:    isAvailable,
		TotalAgents:    totalAgents,
		DeployedAgents: deployedAgents,
		IsDefault:      true,
		LastSyncedAt:   time.Now().Format(time.RFC3339),
	}, nil
}

// SyncDefaultAgentHub is a background goroutine that periodically clones/pulls
// the agent-charts repo, analogous to SyncDefaultChaosHubs.
func (s *agentHubService) SyncDefaultAgentHub() {
	log.Info("starting default agent hub sync goroutine")
	for {
		repoURL := getAgentHubGitURL()
		branch := getAgentHubBranch()
		clonePath := getAgentClonePath()

		if err := syncHub(clonePath, repoURL, branch); err != nil {
			log.WithFields(log.Fields{
				"repoUrl":  repoURL,
				"branch":   branch,
				"hubName":  DefaultAgentHubName,
			}).WithError(err).Error("failed to sync default agent hub")
		} else {
			log.WithFields(log.Fields{
				"repoUrl": repoURL,
				"branch":  branch,
			}).Info("successfully synced default agent hub")
		}

		time.Sleep(DefaultAgentHubSyncInterval)
	}
}

// syncHub clones or pulls a git repo to the given path.
func syncHub(clonePath string, repoURL string, branch string) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(clonePath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Check if already cloned
	if _, err := os.Stat(filepath.Join(clonePath, ".git")); os.IsNotExist(err) {
		// Clone fresh using go-git directly
		log.WithFields(log.Fields{
			"repoURL":   repoURL,
			"branch":    branch,
			"clonePath": clonePath,
		}).Info("cloning agent/app hub repo")
		return chaosHubOps.ClonePublicRepoToPath(repoURL, branch, clonePath)
	}

	// Already exists, pull latest
	log.WithFields(log.Fields{
		"clonePath": clonePath,
		"branch":    branch,
	}).Info("pulling latest for agent/app hub repo")
	return chaosHubOps.PullRepoAtPath(clonePath, branch)
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// GetAgentInjectionMetadata returns the injection metadata for agents that have
// contextInjection defined in their chartserviceversion entry.
// Returns a map of installTemplateName/installImage → AgentEntry for matching.
// This is called by the experiment ops service at save time.
// If the CSV has not been synced yet or is unreadable, returns nil (caller falls
// back to hardcoded behavior).
func GetAgentInjectionMetadata() []AgentEntry {
	chartsPath := getAgentChartsPath()
	entries, err := GetAllAgentEntries(chartsPath)
	if err != nil {
		log.WithError(err).Warn("[AgentHub] failed to read agent entries for injection metadata — caller should use fallback")
		return nil
	}

	// Only return entries that actually have injection metadata
	var result []AgentEntry
	for _, e := range entries {
		if len(e.ContextInjection) > 0 {
			result = append(result, e)
		}
	}
	return result
}
