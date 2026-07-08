package catalog

import (
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

var kebabRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// ValidateName checks that an app name is kebab-case and not already in the catalog.
// POST /api/catalog/validate-name
// Body: {"name": "my-app"}
// Response: {"valid": true|false, "reason": "..."}
func ValidateName(svc Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Name string `json:"name"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"valid": false, "reason": "invalid request body"})
			return
		}
		name := strings.TrimSpace(body.Name)
		if len(name) < 2 || len(name) > 63 {
			c.JSON(http.StatusOK, gin.H{"valid": false, "reason": "name must be 2-63 characters"})
			return
		}
		if !kebabRe.MatchString(name) {
			c.JSON(http.StatusOK, gin.H{"valid": false, "reason": "name must be lowercase kebab-case (a-z, 0-9, hyphens only; no leading/trailing hyphens)"})
			return
		}
		if svc != nil {
			if app, err := svc.GetApplication(c.Request.Context(), "", name); err == nil && app != nil {
				c.JSON(http.StatusOK, gin.H{"valid": false, "reason": fmt.Sprintf("app %q already exists in the catalog", name)})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"valid": true, "reason": ""})
	}
}

// DiscoverServices runs helm to parse chart templates and returns the list of
// Deployment/StatefulSet/DaemonSet workloads found in the chart.
// POST /api/catalog/discover-services
// Body: {"repoURL": "...", "chartName": "...", "chartVersion": "..."}
// Response: {"services": [...]}
func DiscoverServices() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			RepoURL      string `json:"repoURL"`
			ChartName    string `json:"chartName"`
			ChartVersion string `json:"chartVersion"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		if body.RepoURL == "" || body.ChartName == "" || body.ChartVersion == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "repoURL, chartName, and chartVersion are required"})
			return
		}

		services, err := parseHelmTemplates(body.RepoURL, body.ChartName, body.ChartVersion)
		if err != nil {
			log.WithError(err).Warn("helm service discovery failed")
			c.JSON(http.StatusOK, gin.H{"services": []interface{}{}, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"services": services})
	}
}

// DiscoveredServiceJSON is the JSON shape returned by DiscoverServices.
type DiscoveredServiceJSON struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Kind        string `json:"kind"`
	AutoExclude bool   `json:"autoExcluded"`
	Reason      string `json:"autoExclusionReason,omitempty"`
}

var autoExcludeNames = map[string]string{
	"prometheus":   "observability tool",
	"grafana":      "observability tool",
	"alertmanager": "observability tool",
	"loki":         "observability tool",
	"jaeger":       "observability tool",
	"tempo":        "observability tool",
}

var dbSuffixes = []string{"-db", "-database", "-postgres", "-mysql", "-mongo", "-redis"}

func parseHelmTemplates(repoURL, chartName, chartVersion string) ([]DiscoveredServiceJSON, error) {
	alias := sanitizeRepoAlias(repoURL)

	// helm repo add
	addCmd := exec.Command("helm", "repo", "add", alias, repoURL)
	if out, err := addCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("helm repo add: %s: %w", string(out), err)
	}

	// helm repo update (best-effort)
	exec.Command("helm", "repo", "update").Run() //nolint:errcheck

	// helm show all
	showCmd := exec.Command("helm", "show", "all", fmt.Sprintf("%s/%s", alias, chartName), "--version", chartVersion)
	out, err := showCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("helm show all: %w", err)
	}

	return extractWorkloads(string(out)), nil
}

func extractWorkloads(helmOutput string) []DiscoveredServiceJSON {
	var results []DiscoveredServiceJSON
	seen := map[string]bool{}

	docs := strings.Split(helmOutput, "\n---")
	for _, doc := range docs {
		var manifest map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &manifest); err != nil || manifest == nil {
			continue
		}
		kind, _ := manifest["kind"].(string)
		kind = strings.ToLower(kind)
		if kind != "deployment" && kind != "statefulset" && kind != "daemonset" {
			continue
		}
		name := extractName(manifest)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true

		label := fmt.Sprintf("app=%s", name)
		svc := DiscoveredServiceJSON{Name: name, Label: label, Kind: kind}

		nameLower := strings.ToLower(name)
		if reason, excluded := autoExcludeNames[nameLower]; excluded {
			svc.AutoExclude = true
			svc.Reason = reason
		} else {
			for _, suffix := range dbSuffixes {
				if strings.HasSuffix(nameLower, suffix) {
					svc.AutoExclude = true
					svc.Reason = "database workload"
					break
				}
			}
		}

		results = append(results, svc)
	}
	return results
}

// extractName retrieves the metadata.name field from a YAML manifest decoded by
// gopkg.in/yaml.v2. The top-level map has string keys (map[string]interface{})
// but nested maps may be map[interface{}]interface{}, so both are tried.
func extractName(manifest map[string]interface{}) string {
	raw, ok := manifest["metadata"]
	if !ok {
		return ""
	}
	// yaml.v2 decodes nested mappings as map[interface{}]interface{}
	if meta, ok := raw.(map[interface{}]interface{}); ok {
		name, _ := meta["name"].(string)
		return name
	}
	// fallback: some callers may decode into map[string]interface{}
	if meta, ok := raw.(map[string]interface{}); ok {
		name, _ := meta["name"].(string)
		return name
	}
	return ""
}

// sanitizeRepoAlias turns a repo URL into a safe helm repo alias.
// Only alphanumerics and hyphens are kept; max 50 chars.
func sanitizeRepoAlias(repoURL string) string {
	re := regexp.MustCompile(`[^a-z0-9-]`)
	alias := strings.ToLower(repoURL)
	// Strip scheme (e.g. "https://")
	if idx := strings.Index(alias, "//"); idx >= 0 {
		alias = alias[idx+2:]
	}
	alias = re.ReplaceAllString(alias, "-")
	alias = strings.Trim(alias, "-")
	if len(alias) > 50 {
		alias = alias[:50]
	}
	if alias == "" {
		alias = "ace-catalog-repo"
	}
	return alias
}
