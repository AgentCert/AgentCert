package agenthub

import (
	"context"
	"fmt"
	"os"
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

// getAgentChartsPath returns the filesystem path where agent charts are cloned.
func getAgentChartsPath() string {
	path := utils.Config.DefaultAgentHubPath
	if path == "" {
		path = "/tmp/default-agents/"
	}
	return path + DefaultAgentHubName + "/charts/"
}

// getAgentClonePath returns the filesystem path for the agent hub clone.
func getAgentClonePath() string {
	path := utils.Config.DefaultAgentHubPath
	if path == "" {
		path = "/tmp/default-agents/"
	}
	return path + DefaultAgentHubName
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

		chartsInput := model.CloningInput{
			Name:       DefaultAgentHubName,
			RepoURL:    repoURL,
			RepoBranch: branch,
			IsDefault:  true,
		}

		// Reuse the same git ops as ChaosHub, but clone to a different path.
		// We override the clone path by using a custom hub name that maps to our path.
		clonePath := getAgentClonePath()
		if err := syncHub(chartsInput, clonePath, repoURL, branch); err != nil {
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
func syncHub(chartsInput model.CloningInput, clonePath string, repoURL string, branch string) error {
	// Check if already cloned
	if _, err := os.Stat(clonePath); os.IsNotExist(err) {
		// Clone fresh
		gitConfig := chaosHubOps.ChaosHubConfig{
			HubName:       chartsInput.Name,
			RepositoryURL: repoURL,
			Branch:        branch,
			RemoteName:    "origin",
			IsDefault:     true,
			AuthType:      model.AuthTypeNone,
		}
		_ = gitConfig // We'll use GitSyncDefaultHub which handles clone-or-pull
		return chaosHubOps.GitSyncDefaultHub(chartsInput)
	}

	// Already exists, just sync
	return chaosHubOps.GitSyncDefaultHub(chartsInput)
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
