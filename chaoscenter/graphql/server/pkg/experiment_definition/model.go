package experiment_definition

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CollectionName is the MongoDB collection for experiment definitions.
const CollectionName = "experiment_definitions"

// ExperimentStepType is the type of a single step in the experiment sequence.
type ExperimentStepType string

const (
	StepTypeObserve       ExperimentStepType = "observe"
	StepTypeFault         ExperimentStepType = "fault"
	StepTypeVerify        ExperimentStepType = "verify"
	StepTypeWait          ExperimentStepType = "wait"
	StepTypeParallelFault ExperimentStepType = "parallel-fault"
)

// ModelSelectionMode controls which LLM model is used for a run.
type ModelSelectionMode string

const (
	ModelSelectionAgentDefault    ModelSelectionMode = "agent-default"
	ModelSelectionFixed           ModelSelectionMode = "fixed"
	ModelSelectionUserChoosesAtRun ModelSelectionMode = "user-chooses-at-run"
)

// StepTarget identifies the microservice within the app to target.
type StepTarget struct {
	Microservice    string `bson:"microservice" json:"microservice"`
	ExplicitPodName string `bson:"explicitPodName,omitempty" json:"explicitPodName,omitempty"`
}

// StepProbe is the health probe for a verify step.
type StepProbe struct {
	URL            string `bson:"url" json:"url"`
	ExpectedStatus int    `bson:"expectedStatus" json:"expectedStatus"`
	TimeoutSecs    int    `bson:"timeoutSecs,omitempty" json:"timeoutSecs,omitempty"`
	Retries        int    `bson:"retries,omitempty" json:"retries,omitempty"`
}

// GroundTruthOverride allows per-step override of the fault's default SLAs.
type GroundTruthOverride struct {
	DetectWithinSecs   *int `bson:"detectWithinSecs,omitempty" json:"detectWithinSecs,omitempty"`
	MitigateWithinSecs *int `bson:"mitigateWithinSecs,omitempty" json:"mitigateWithinSecs,omitempty"`
}

// ParallelFaultEntry is one fault within a parallel-fault step.
type ParallelFaultEntry struct {
	FaultRef string            `bson:"faultRef" json:"faultRef"`
	Target   StepTarget        `bson:"target" json:"target"`
	Params   map[string]string `bson:"params,omitempty" json:"params,omitempty"`
}

// ExperimentStep is one step in the experiment step sequence.
type ExperimentStep struct {
	Name        string             `bson:"name" json:"name"`
	Type        ExperimentStepType `bson:"type" json:"type"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`

	// For observe and wait steps
	Duration string `bson:"duration,omitempty" json:"duration,omitempty"` // e.g. "30s"

	// For fault steps
	FaultRef            string               `bson:"faultRef,omitempty" json:"faultRef,omitempty"`
	Target              *StepTarget          `bson:"target,omitempty" json:"target,omitempty"`
	Params              map[string]string    `bson:"params,omitempty" json:"params,omitempty"`
	DependsOn           string               `bson:"dependsOn,omitempty" json:"dependsOn,omitempty"`
	GroundTruthOverride *GroundTruthOverride `bson:"groundTruthOverride,omitempty" json:"groundTruthOverride,omitempty"`

	// For verify steps
	Probe *StepProbe `bson:"probe,omitempty" json:"probe,omitempty"`

	// For parallel-fault steps
	Faults []ParallelFaultEntry `bson:"faults,omitempty" json:"faults,omitempty"`
}

// PerStepCriteria holds success criteria for a named fault step.
type PerStepCriteria struct {
	StepName           string `bson:"stepName" json:"stepName"`
	DetectWithinSecs   int    `bson:"detectWithinSecs" json:"detectWithinSecs"`
	MitigateWithinSecs int    `bson:"mitigateWithinSecs" json:"mitigateWithinSecs"`
}

// OverallCriteria holds experiment-level success thresholds.
type OverallCriteria struct {
	ToolCallEfficiencyMin float64 `bson:"toolCallEfficiencyMin" json:"toolCallEfficiencyMin"`
	FalsePositiveRateMax  float64 `bson:"falsePositiveRateMax" json:"falsePositiveRateMax"`
	RootCauseAccuracyMin  float64 `bson:"rootCauseAccuracyMin" json:"rootCauseAccuracyMin"`
}

// SuccessCriteria is the success criteria block.
type SuccessCriteria struct {
	PerStep []PerStepCriteria `bson:"perStep,omitempty" json:"perStep,omitempty"`
	Overall *OverallCriteria  `bson:"overall,omitempty" json:"overall,omitempty"`
}

// AgentConstraints declares which agents are compatible with this experiment.
type AgentConstraints struct {
	RequiredCapabilities []string `bson:"requiredCapabilities,omitempty" json:"requiredCapabilities,omitempty"`
	SupportedAgents      []string `bson:"supportedAgents,omitempty" json:"supportedAgents,omitempty"`
	BlockedAgents        []string `bson:"blockedAgents,omitempty" json:"blockedAgents,omitempty"`
}

// ModelSelection controls LLM model behavior for this experiment.
type ModelSelection struct {
	Mode       ModelSelectionMode `bson:"mode" json:"mode"`
	FixedModel string             `bson:"fixedModel,omitempty" json:"fixedModel,omitempty"`
}

// TargetAppSpec identifies the app this experiment runs against.
type TargetAppSpec struct {
	Name         string            `bson:"name" json:"name"`
	Version      string            `bson:"version" json:"version"` // SemVer range, e.g. ">=1.0.0"
	InstallParams map[string]string `bson:"installParams,omitempty" json:"installParams,omitempty"`
}

// ExperimentDefinitionDoc is the MongoDB document for an experiment definition.
type ExperimentDefinitionDoc struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Name        string             `bson:"name"`
	ProjectID   string             `bson:"projectID"`
	DisplayName string             `bson:"displayName,omitempty"`
	Version     string             `bson:"version"`
	Hypothesis  string             `bson:"hypothesis,omitempty"`
	Tags        []string           `bson:"tags,omitempty"`
	Author      struct {
		Name  string `bson:"name"`
		Email string `bson:"email"`
	} `bson:"author,omitempty"`

	TargetApp         TargetAppSpec    `bson:"targetApp"`
	AgentConstraints  AgentConstraints `bson:"agentConstraints,omitempty"`
	ModelSelection    ModelSelection   `bson:"modelSelection"`
	Steps             []ExperimentStep `bson:"steps"`
	SuccessCriteria   SuccessCriteria  `bson:"successCriteria,omitempty"`
	EvaluationMetrics []string         `bson:"evaluationMetrics,omitempty"`

	// Lifecycle
	Status    string    `bson:"status"` // DRAFT | READY
	CreatedAt time.Time `bson:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt"`
	CreatedBy string    `bson:"createdBy"`
}
