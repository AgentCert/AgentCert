package helm

// HelmChartInstallRequest represents the request payload for installing a Helm chart
type HelmChartInstallRequest struct {
	EnvironmentID   string `json:"environmentId" binding:"required"`
	EnvironmentName string `json:"environmentName" binding:"required"`
	ReleaseName     string `json:"releaseName"`
	Namespace       string `json:"namespace"`
	ProjectID       string `json:"projectId" binding:"required"`
}

// HelmChartInstallResponse represents the response after installing a Helm chart
type HelmChartInstallResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	ReleaseName string `json:"releaseName,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Status      string `json:"status,omitempty"`
}

// HelmReleaseInfo contains information about an installed Helm release
type HelmReleaseInfo struct {
	ReleaseName   string `json:"releaseName"`
	Namespace     string `json:"namespace"`
	Status        string `json:"status"`
	ChartName     string `json:"chartName"`
	ChartVersion  string `json:"chartVersion"`
	AppVersion    string `json:"appVersion"`
	EnvironmentID string `json:"environmentId"`
}

// PortForwardResponse represents the response after starting port-forward
type PortForwardResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	URL       string `json:"url,omitempty"`
	LocalPort int    `json:"localPort,omitempty"`
}

// PortForwardInfo contains information about an active port-forward
type PortForwardInfo struct {
	Namespace   string `json:"namespace"`
	ServiceName string `json:"serviceName"`
	ServicePort int    `json:"servicePort"`
	LocalPort   int    `json:"localPort"`
	ProcessID   int    `json:"processId"`
}
