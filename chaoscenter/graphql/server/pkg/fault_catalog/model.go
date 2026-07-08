package fault_catalog

// FaultScope represents the three-tier taxonomy.
type FaultScope string

const (
	ScopeGeneral     FaultScope = "general"
	ScopeDomain      FaultScope = "domain"
	ScopeAppSpecific FaultScope = "app-specific"
)

// FaultImplementationType is the execution mechanism.
type FaultImplementationType string

const (
	ImplLitmus    FaultImplementationType = "litmus"
	ImplHTTPFault FaultImplementationType = "http-fault"
	ImplScript    FaultImplementationType = "script"
	ImplExternal  FaultImplementationType = "external"
)

// ParameterType is the type of a fault parameter.
type ParameterType string

const (
	ParamTypeInteger ParameterType = "integer"
	ParamTypeString  ParameterType = "string"
	ParamTypeBoolean ParameterType = "boolean"
	ParamTypeEnum    ParameterType = "enum"
	ParamTypePercent ParameterType = "percent"
)

// GroundTruthCategory maps to the certifier's scoring categories.
type GroundTruthCategory string

const (
	GTCatAvailability  GroundTruthCategory = "availability"
	GTCatPerformance   GroundTruthCategory = "performance"
	GTCatSecurity      GroundTruthCategory = "security"
	GTCatDataIntegrity GroundTruthCategory = "data-integrity"
	GTCatConfiguration GroundTruthCategory = "configuration"
)

// CatalogTier is official vs community.
type CatalogTier string

const (
	TierOfficial  CatalogTier = "official"
	TierCommunity CatalogTier = "community"
)

// FaultMaintainer is a contact for the fault.
type FaultMaintainer struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
}

// FaultMetadata mirrors the metadata block of fault.yaml.
type FaultMetadata struct {
	Name        string           `yaml:"name"`
	DisplayName string           `yaml:"displayName"`
	Version     string           `yaml:"version"`
	Tier        CatalogTier      `yaml:"tier"`
	Scope       FaultScope       `yaml:"scope"`
	Domain      *string          `yaml:"domain"`
	TargetApp   *string          `yaml:"targetApp"`
	Tags        []string         `yaml:"tags"`
	Maintainers []FaultMaintainer `yaml:"maintainers"`
}

// FaultDescription is the human-readable description block.
type FaultDescription struct {
	Short          string   `yaml:"short"`
	Long           string   `yaml:"long"`
	SuitableFor    []string `yaml:"suitableFor"`
	NotSuitableFor []string `yaml:"notSuitableFor"`
}

// FaultImplementation defines how the fault is executed.
type FaultImplementation struct {
	Type          FaultImplementationType `yaml:"type"`
	ChaosKind     string                  `yaml:"chaosKind,omitempty"`
	ExperimentRef string                  `yaml:"experimentRef,omitempty"`
	Namespace     string                  `yaml:"namespace,omitempty"`
	// For http-fault type
	Target *struct {
		Service string `yaml:"service"`
		Port    int    `yaml:"port"`
		Path    string `yaml:"path"`
	} `yaml:"target,omitempty"`
	// For script type
	Image   string   `yaml:"image,omitempty"`
	Command []string `yaml:"command,omitempty"`
	Args    []string `yaml:"args,omitempty"`
	// For external type
	Endpoint string `yaml:"endpoint,omitempty"`
	Method   string `yaml:"method,omitempty"`
}

// FaultParameter is one configurable parameter.
type FaultParameter struct {
	Key           string        `yaml:"key"`
	DisplayName   string        `yaml:"displayName"`
	Type          ParameterType `yaml:"type"`
	Unit          string        `yaml:"unit,omitempty"`
	Default       string        `yaml:"default"`
	Min           *int          `yaml:"min,omitempty"`
	Max           *int          `yaml:"max,omitempty"`
	Required      bool          `yaml:"required"`
	Description   string        `yaml:"description"`
	LitmusEnv     string        `yaml:"litmusEnv,omitempty"`
	AllowedValues []string      `yaml:"allowedValues,omitempty"` // for enum type
}

// FaultCompatibility declares where this fault can be used.
type FaultCompatibility struct {
	TargetDomains        []string `yaml:"targetDomains"`
	IncompatibleApps     []string `yaml:"incompatibleApps"`
	RequiredCapabilities []string `yaml:"requiredCapabilities"`
}

// FaultObservability describes expected symptoms and alerts.
type FaultObservability struct {
	ExpectedSymptoms    []string `yaml:"expectedSymptoms"`
	ExpectedAlerts      []string `yaml:"expectedAlerts"`
	DetectionWindowSecs int      `yaml:"detectionWindowSecs"`
}

// FaultGroundTruth is the machine-readable expected outcome.
type FaultGroundTruth struct {
	Category           GroundTruthCategory `yaml:"category"`
	Impact             string              `yaml:"impact"` // low|medium|high|critical
	DetectWithinSecs   int                 `yaml:"detectWithinSecs"`
	MitigateWithinSecs int                 `yaml:"mitigateWithinSecs"`
	DetectionHints     []string            `yaml:"detectionHints"`
	RemediationHints   []string            `yaml:"remediationHints"`
}

// FaultSpec is the spec block of fault.yaml.
type FaultSpec struct {
	Description    FaultDescription   `yaml:"description"`
	Implementation FaultImplementation `yaml:"implementation"`
	Parameters     []FaultParameter   `yaml:"parameters"`
	Compatibility  FaultCompatibility `yaml:"compatibility"`
	Observability  FaultObservability `yaml:"observability"`
	GroundTruth    FaultGroundTruth   `yaml:"groundTruth"`
}

// FaultCatalogEntry is the top-level struct for a fault.yaml file.
type FaultCatalogEntry struct {
	APIVersion string        `yaml:"apiVersion"`
	Kind       string        `yaml:"kind"`
	Metadata   FaultMetadata `yaml:"metadata"`
	Spec       FaultSpec     `yaml:"spec"`

	// FilePath is set by the loader — not part of the YAML schema.
	FilePath string `yaml:"-"`
}
