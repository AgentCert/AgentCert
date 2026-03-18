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
	PrometheusMcpServerImage    string   `split_words:"true" default:"agentcert/prometheus-mcp-server:latest"`
	PrometheusMcpUrl            string   `split_words:"true" default:"http://prometheus.monitoring.svc.cluster.local:9090"`
	WorkflowHelperImageVersion  string   `required:"true" split_words:"true"`
	ChaosCenterUiEndpoint       string   `split_words:"true" default:"https://localhost:8080"`
	TlsCertB64                  string   `split_words:"true"`
	LitmusAuthGrpcEndpoint      string   `split_words:"true" default:"localhost"`
	LitmusAuthGrpcPort          string   `split_words:"true" default:"3030"`
	KubeConfigFilePath          string   `split_words:"true"`
	RemoteHubMaxSize            string   `split_words:"true"`
	SkipSslVerify               string   `split_words:"true"`
	RestPort                    string   `split_words:"true" default:"8080"`
	GrpcPort                    string   `split_words:"true" default:"8000"`
	InfraCompatibleVersions     string   `required:"true" split_words:"true"`
	DefaultHubGitURL            string   `required:"true" default:"https://github.com/agentcert/chaos-charts"`
	GitUsername                 string   `required:"true" split_words:"true" default:"litmus"`
	DefaultHubBranchName        string   `required:"true" split_words:"true"`
	CustomChaosHubPath          string   `split_words:"true" default:"/tmp/"`
	DefaultChaosHubPath         string   `split_words:"true" default:"/tmp/default/"`
	EnableGQLIntrospection      string   `split_words:"true" default:"false"`
	EnableInternalTls           string   `split_words:"true" default:"false"`
	TlsCertPath                 string   `split_words:"true"`
	TlsKeyPath                  string   `split_words:"true"`
	CaCertTlsPath               string   `split_words:"true"`
	DefaultAgentChartPath       string   `split_words:"true"`
	DefaultAgentHubGitURL       string   `split_words:"true" default:"https://github.com/agentcert/agent-charts"`
	DefaultAgentHubBranchName   string   `split_words:"true" default:"main"`
	DefaultAgentHubPath         string   `split_words:"true" default:"/tmp/default-agents/"`
	DefaultAppHubGitURL         string   `split_words:"true" default:"https://github.com/agentcert/app-charts"`
	DefaultAppHubBranchName     string   `split_words:"true" default:"main"`
	DefaultAppHubPath           string   `split_words:"true" default:"/tmp/default-apps/"`
	HelmBinary                  string   `split_words:"true" default:"helm"`
	HelmTimeout                 string   `split_words:"true" default:"5m"`
	AllowedOrigins              []string `split_words:"true" default:"^(http://|https://|)litmuschaos.io(:[0-9]+|)?,^(http://|https://|)localhost(:[0-9]+|)"`
}

var Config Configuration
