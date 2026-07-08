package catalog

// AppCatalogEntry is the internal Go representation of an app.yaml file.
type AppCatalogEntry struct {
	APIVersion string      `yaml:"apiVersion"`
	Kind       string      `yaml:"kind"`
	Metadata   AppMetadata `yaml:"metadata"`
	Spec       AppSpec     `yaml:"spec"`
}

type AppMetadata struct {
	Name              string       `yaml:"name"`
	DisplayName       string       `yaml:"displayName"`
	Version           string       `yaml:"version"`
	Tier              string       `yaml:"tier"`
	Domain            string       `yaml:"domain"`
	CapabilityDomains []string     `yaml:"capabilityDomains"`
	Tags              []string     `yaml:"tags"`
	Maintainers       []Maintainer `yaml:"maintainers"`
	License           string       `yaml:"license"`
	Repository        string       `yaml:"repository"`
	CreatedAt         string       `yaml:"createdAt"`
	UpdatedAt         string       `yaml:"updatedAt"`
}

type Maintainer struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
}

type AppSpec struct {
	Description        AppDescription     `yaml:"description"`
	Install            InstallSpec        `yaml:"install"`
	HealthProbe        HealthProbeSpec    `yaml:"healthProbe"`
	LoadTest           LoadTestSpec       `yaml:"loadTest"`
	Microservices      []MicroserviceSpec `yaml:"microservices"`
	Observability      ObservabilitySpec  `yaml:"observability"`
	FaultCompatibility []FaultCompatEntry `yaml:"faultCompatibility"`
	GroundTruth        GroundTruthSpec    `yaml:"groundTruth"`
	RBAC               RBACSpec           `yaml:"rbac"`
	Inputs             []AppInput         `yaml:"inputs"`
}

type AppDescription struct {
	Short          string   `yaml:"short"`
	Long           string   `yaml:"long"`
	SuitableFor    []string `yaml:"suitableFor"`
	NotSuitableFor []string `yaml:"notSuitableFor"`
}

type InstallSpec struct {
	Method              string        `yaml:"method"`
	Folder              string        `yaml:"folder"`
	ChartRef            *ChartRef     `yaml:"chartRef,omitempty"`
	Namespace           NamespaceSpec `yaml:"namespace"`
	Timeout             string        `yaml:"timeout"`
	Wait                bool          `yaml:"wait"`
	AdditionalManifests []string      `yaml:"additionalManifests"`
}

type ChartRef struct {
	Repo    string `yaml:"repo"`
	Chart   string `yaml:"chart"`
	Version string `yaml:"version"`
}

type NamespaceSpec struct {
	Default      string `yaml:"default"`
	Configurable bool   `yaml:"configurable"`
}

type HealthProbeSpec struct {
	URL                 string            `yaml:"url"`
	ExpectedStatus      string            `yaml:"expectedStatus"`
	InitialDelaySeconds int               `yaml:"initialDelaySeconds"`
	PeriodSeconds       int               `yaml:"periodSeconds"`
	FailureThreshold    int               `yaml:"failureThreshold"`
	Headers             map[string]string `yaml:"headers"`
	InsecureSkipTLS     bool              `yaml:"insecureSkipTLS"`
}

type LoadTestSpec struct {
	Enabled          bool     `yaml:"enabled"`
	Method           string   `yaml:"method"`
	Image            string   `yaml:"image"`
	Args             []string `yaml:"args"`
	InstallNamespace string   `yaml:"installNamespace"`
}

type MicroserviceSpec struct {
	Name           string   `yaml:"name"`
	DisplayName    string   `yaml:"displayName"`
	Description    string   `yaml:"description"`
	K8s            K8sSpec  `yaml:"k8s"`
	RelevantFaults []string `yaml:"relevantFaults"`
	Criticality    string   `yaml:"criticality"`
	DependsOn      []string `yaml:"dependsOn"`
	SLA            *SLASpec `yaml:"sla,omitempty"`
}

type K8sSpec struct {
	Label         string `yaml:"label"`
	Kind          string `yaml:"kind"`
	Namespace     string `yaml:"namespace"`
	ContainerName string `yaml:"containerName"`
}

type SLASpec struct {
	ErrorRateThreshold float64 `yaml:"errorRateThreshold"`
}

type ObservabilitySpec struct {
	Prometheus PrometheusSpec `yaml:"prometheus"`
}

type PrometheusSpec struct {
	ServiceMonitor bool        `yaml:"serviceMonitor"`
	AlertRules     []AlertRule `yaml:"alertRules"`
}

type AlertRule struct {
	Name        string            `yaml:"name"`
	Severity    string            `yaml:"severity"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for"`
	Annotations map[string]string `yaml:"annotations"`
}

type FaultCompatEntry struct {
	FaultName          string   `yaml:"faultName"`
	Compatible         bool     `yaml:"compatible"`
	Notes              string   `yaml:"notes"`
	RecommendedTargets []string `yaml:"recommendedTargets"`
}

type GroundTruthSpec struct {
	Version            string              `yaml:"version"`
	FaultAlertMappings []FaultAlertMapping `yaml:"faultAlertMappings"`
}

type FaultAlertMapping struct {
	FaultName             string   `yaml:"faultName"`
	TargetService         string   `yaml:"targetService"`
	ExpectedAlerts        []string `yaml:"expectedAlerts"`
	ExpectedRootCause     string   `yaml:"expectedRootCause"`
	ExpectedRemediation   string   `yaml:"expectedRemediation"`
	MaxDetectionTimeSecs  int      `yaml:"maxDetectionTimeSecs"`
	MaxMitigationTimeSecs int      `yaml:"maxMitigationTimeSecs"`
}

type RBACSpec struct {
	ChaosRunnerPermissions []interface{} `yaml:"chaosRunnerPermissions"`
}

type AppInput struct {
	Key         string   `yaml:"key"`
	DisplayName string   `yaml:"displayName"`
	Description string   `yaml:"description"`
	Type        string   `yaml:"type"`
	Required    bool     `yaml:"required"`
	Default     string   `yaml:"default"`
	HelmPath    string   `yaml:"helmPath"`
	Values      []string `yaml:"values"`
	Min         *int     `yaml:"min,omitempty"`
	Max         *int     `yaml:"max,omitempty"`
	Unit        string   `yaml:"unit"`
	Advanced    bool     `yaml:"advanced"`
}
