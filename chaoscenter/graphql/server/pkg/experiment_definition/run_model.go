package experiment_definition

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RunCollectionName is the MongoDB collection for ACE experiment runs.
const RunCollectionName = "experiment_runs_ext"

// RunStatus represents the lifecycle state of an experiment run.
type RunStatus string

const (
	RunStatusQueued    RunStatus = "QUEUED"
	RunStatusRunning   RunStatus = "RUNNING"
	RunStatusCompleted RunStatus = "COMPLETED"
	RunStatusFailed    RunStatus = "FAILED"
	RunStatusAborted   RunStatus = "ABORTED"
)

// StatusEvent records a single status transition.
type StatusEvent struct {
	Status    RunStatus `bson:"status"`
	Timestamp time.Time `bson:"timestamp"`
	Reason    string    `bson:"reason,omitempty"`
}

// AceExperimentRunDoc is the MongoDB document for an experiment run.
// Documents are immutable after reaching a terminal status (COMPLETED, FAILED, ABORTED).
type AceExperimentRunDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	RunID     string             `bson:"runID"`
	ProjectID string             `bson:"projectID"`

	// Definition reference
	DefinitionName    string `bson:"definitionName"`
	DefinitionVersion string `bson:"definitionVersion"`

	// Agent reference
	AgentName    string `bson:"agentName"`
	AgentVersion string `bson:"agentVersion"`

	// Model used (always populated — resolved at run submit time)
	ModelUsed     string `bson:"modelUsed"`
	ModelProvider string `bson:"modelProvider"`

	// Execution references
	ArgoWorkflowName  string `bson:"argoWorkflowName"`
	LangfuseTraceID   string `bson:"langfuseTraceId,omitempty"`
	CertifierReportID string `bson:"certifierReportId,omitempty"`

	// Status
	Status        RunStatus     `bson:"status"`
	StatusHistory []StatusEvent `bson:"statusHistory"`

	// Timing
	StartedAt   *time.Time `bson:"startedAt,omitempty"`
	CompletedAt *time.Time `bson:"completedAt,omitempty"`
	CreatedAt   time.Time  `bson:"createdAt"`
	CreatedBy   string     `bson:"createdBy"`
}

// IsTerminal returns true if the run has reached a terminal state.
func (r *AceExperimentRunDoc) IsTerminal() bool {
	switch r.Status {
	case RunStatusCompleted, RunStatusFailed, RunStatusAborted:
		return true
	}
	return false
}
