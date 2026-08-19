package utils

var (
	SupportedPrivateGitRepository = []string{"github", "gitlab"}
)

type Configuration struct {
	Version                     string   `required:"true"`
	InfraDeployments            string   `required:"true" split_words:"true"`
	DbServer                    string   `required:"true" split_words:"true"`
	DbUser                      string   `split_words:"true"`
	DbPassword                  string   `split_words:"true"`
	SubscriberImage             string   `required:"true" split_words:"true"`
	EventTrackerImage           string   `required:"true" split_words:"true"`
	ArgoWorkflowControllerImage string   `required:"true" split_words:"true"`
	ArgoWorkflowExecutorImage   string   `required:"true" split_words:"true"`
	LitmusChaosOperatorImage    string   `required:"true" split_words:"true"`
	LitmusChaosRunnerImage      string   `required:"true" split_words:"true"`
	LitmusChaosExporterImage    string   `required:"true" split_words:"true"`
	ContainerRuntimeExecutor    string   `required:"true" split_words:"true"`
	KubernetesMcpServerImage    string   `split_words:"true" default:"quay.io/containers/kubernetes_mcp_server:latest"`
	PrometheusMcpServerImage    string   `split_words:"true" default:"ghcr.io/pab1it0/prometheus-mcp-server:latest"`
	PrometheusMcpUrl            string   `split_words:"true" default:"http://prometheus.monitoring.svc.cluster.local:9090"`
	WorkflowHelperImageVersion  string   `required:"true" split_words:"true"`
	InstallApplicationImage            string `split_words:"true" default:"agentcert/agentcert-install-app:latest"`
	InstallApplicationImagePullPolicy  string `split_words:"true" default:"IfNotPresent"`
	InstallAgentImage                  string `split_words:"true" default:"agentcert/agentcert-install-agent:latest"`
	InstallAgentImagePullPolicy        string `split_words:"true" default:"IfNotPresent"`
	// LitmusHelperImagesRegistryPrefix is prepended to all litmus helper image refs at workflow
	// submission time (e.g. "infyartifactory.jfrog.io/docker-local/" for JFrog, "" for Docker Hub).
	LitmusHelperImagesRegistryPrefix   string `split_words:"true" default:""`
	// LitmusHelperImagesPullPolicy is applied to all litmus helper image templates at submission
	// time. Use "IfNotPresent" when images are pre-loaded into KinD (local mode) and "Always"
	// when they are pulled from a registry at run time (jfrog / dockerhub).
	LitmusHelperImagesPullPolicy       string `split_words:"true" default:""`
	FlashAgentImage             string   `split_words:"true" default:"agentcert/agentcert-flash-agent:latest"`
	AgentSidecarImage           string   `split_words:"true" default:"agentcert/agent-sidecar:latest"`
	ChaosCenterUiEndpoint       string   `split_words:"true" default:"https://localhost:8080"`
	// ChaosCenterPublicEndpoint, when set, is the base URL a human (or a
	// `kubectl`/`curl` running on their behalf) should use to reach this
	// ChaosCenter instance from outside the cluster — e.g. the KinD-mapped
	// "http://localhost:<KIND_HOSTPORT_WEB>" for a local dev deployment.
	// It is deliberately distinct from ChaosCenterUiEndpoint, which is the
	// *in-cluster* address subscriber pods use to call back to graphql: the
	// two audiences (a human's shell vs. a pod's network namespace) are
	// rarely reachable via the same address. Used to build the
	// `kubectl apply -f <url>` manifest-download link returned by
	// RegisterInfra so it is correct regardless of what host:port the
	// browser that requested it happened to be tunneled/forwarded through
	// (see OPEN_WEIGHT_CERTIFICATION_HANDOFF.md for the incident that
	// motivated this). Falls back to the request's Referer/Host when unset.
	ChaosCenterPublicEndpoint   string   `split_words:"true" default:""`
	TlsCertB64                  string   `split_words:"true"`
	LitmusAuthGrpcEndpoint      string   `split_words:"true" default:"localhost"`
	LitmusAuthGrpcPort          string   `split_words:"true" default:"3030"`
	KubeConfigFilePath          string   `split_words:"true"`
	RemoteHubMaxSize            string   `split_words:"true"`
	SkipSslVerify               string   `split_words:"true"`
	RestPort                    string   `split_words:"true" default:"8080"`
	GrpcPort                    string   `split_words:"true" default:"8000"`
	InfraCompatibleVersions     string   `required:"true" split_words:"true"`
	DefaultHubGitURL            string   `required:"true" split_words:"true" default:"https://github.com/agentcert/chaos-charts"`
	GitUsername                 string   `required:"true" split_words:"true" default:"litmus"`
	DefaultHubBranchName        string   `required:"true" split_words:"true"`
	CustomChaosHubPath          string   `split_words:"true" default:"/tmp/"`
	DefaultChaosHubPath         string   `split_words:"true" default:"/tmp/default/"`
	HubSourceMode               string   `split_words:"true" default:"default"`
	EnableGQLIntrospection      string   `split_words:"true" default:"false"`
	EnableInternalTls           string   `split_words:"true" default:"false"`
	TlsCertPath                 string   `split_words:"true"`
	TlsKeyPath                  string   `split_words:"true"`
	CaCertTlsPath               string   `split_words:"true"`
	DefaultAgentChartPath       string   `split_words:"true"`
	AgentHubSourceMode          string   `split_words:"true" default:"default"`
	DefaultAgentHubGitURL       string   `split_words:"true" default:"https://github.com/agentcert/agent-charts"`
	DefaultAgentHubBranchName   string   `split_words:"true" default:"main"`
	DefaultAgentHubPath         string   `split_words:"true" default:"/tmp/default-agents/"`
	AppHubSourceMode            string   `split_words:"true" default:"default"`
	DefaultAppHubGitURL         string   `split_words:"true" default:"https://github.com/agentcert/app-charts"`
	DefaultAppHubBranchName     string   `split_words:"true" default:"main"`
	DefaultAppHubPath           string   `split_words:"true" default:"/tmp/default-apps/"`
	HelmBinary                  string   `split_words:"true" default:"helm"`
	HelmTimeout                 string   `split_words:"true" default:"5m"`
	PreCleanupWaitSeconds       string   `split_words:"true" default:"0"`
	CertifierBaseURL            string   `split_words:"true" default:"http://localhost:8088"`
	CertificatePDFBaseURL       string   `split_words:"true" default:"http://localhost:8089"`
	AllowedOrigins              []string `split_words:"true" default:"^(http://|https://|)litmuschaos.io(:[0-9]+|)?,^(http://|https://|)localhost(:[0-9]+|),^[a-z0-9.-]+\\.svc\\.cluster\\.local(:[0-9]+)?"`
}

var Config Configuration
