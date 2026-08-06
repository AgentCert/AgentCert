// Package certification implements the server-side orchestrator that drives
// the four certifier APIs (bucketing-extraction + polling, then
// aggregation-certification + polling) on behalf of the UI, and persists
// progress in MongoDB.
//
// See docs/certification-flow.md for the data-model and flow.
package certification

import "time"

// Run-level workflow status values.
const (
	RunStatusBucketingInitiated = "BUCKETING_INITIATED"
	RunStatusBucketingTriggered = "BUCKETING_TRIGGERED"
	RunStatusBucketingRunning   = "BUCKETING_RUNNING"
	RunStatusBucketingCompleted = "BUCKETING_COMPLETED"
	RunStatusBucketingFailed    = "BUCKETING_FAILED"
)

// Aggregation-level workflow status values.
const (
	AggStatusTriggered = "AGGREGATION_TRIGGERED"
	AggStatusRunning   = "AGGREGATION_RUNNING"
	AggStatusCompleted = "AGGREGATION_COMPLETED"
	AggStatusFailed    = "AGGREGATION_FAILED"
)

// Experiment-level summary status values.
const (
	ExperimentStatusRunsInProgress           = "RUNS_IN_PROGRESS"
	ExperimentStatusReadyForAggregation      = "READY_FOR_AGGREGATION"
	ExperimentStatusAggregationInProgress    = "AGGREGATION_IN_PROGRESS"
	ExperimentStatusCertificateReady         = "EXPERIMENT_CERTIFICATE_READY"
	ExperimentStatusAggregationFailed        = "AGGREGATION_FAILED"
)

// PdfStatus values for the latest certificate.
const (
	PdfStatusNotFetched = "NOT_FETCHED"
)

// AggregationPolicy modes.
const (
	PolicyAllRunsCompleted = "ALL_RUNS_COMPLETED"
)

// CertificateExperiment is the parent summary document — one per
// (projectId, experimentId).
type CertificateExperiment struct {
	ID                       string             `bson:"_id,omitempty"`
	ProjectID                string             `bson:"projectId"`
	AgentID                  string             `bson:"agentId"`
	AgentName                string             `bson:"agentName"`
	ExperimentID             string             `bson:"experimentId"`
	Status                   string             `bson:"status"`
	ExpectedRuns             int                `bson:"expectedRuns"`
	RunCounts                RunCounts          `bson:"runCounts"`
	AggregationPolicy        AggregationPolicy  `bson:"aggregationPolicy"`
	ActiveAggregationVersion int                `bson:"activeAggregationVersion"`
	LatestCertificate        *LatestCertificate `bson:"latestCertificate,omitempty"`
	CreatedAt                time.Time          `bson:"createdAt"`
	UpdatedAt                time.Time          `bson:"updatedAt"`
}

type RunCounts struct {
	Total     int `bson:"total"`
	Running   int `bson:"running"`
	Completed int `bson:"completed"`
	Failed    int `bson:"failed"`
}

type AggregationPolicy struct {
	Mode string `bson:"mode"`
}

type LatestCertificate struct {
	Status          string     `bson:"status"`
	CertificationID string     `bson:"certificationId,omitempty"`
	PdfStatus       string     `bson:"pdfStatus,omitempty"`
	GeneratedAt     *time.Time `bson:"generatedAt,omitempty"`
}

// CertificateRunWorkflow tracks the bucketing lifecycle for a single
// experiment run.
type CertificateRunWorkflow struct {
	ID              string         `bson:"_id,omitempty"`
	ProjectID       string         `bson:"projectId"`
	AgentID         string         `bson:"agentId"`
	ExperimentID    string         `bson:"experimentId"`
	ExperimentRunID string         `bson:"experimentRunId"`
	RunSequence     int            `bson:"runSequence"`
	Status          string         `bson:"status"`
	Bucketing       BucketingState `bson:"bucketing"`
	Result          RunResult      `bson:"result"`
	Error           *WorkflowError `bson:"error,omitempty"`
	CreatedAt       time.Time      `bson:"createdAt"`
	UpdatedAt       time.Time      `bson:"updatedAt"`
	Version         int            `bson:"version"`
}

type BucketingState struct {
	TaskID       string     `bson:"taskId,omitempty"`
	PollURL      string     `bson:"pollUrl,omitempty"`
	LastPolledAt *time.Time `bson:"lastPolledAt,omitempty"`
	NextPollAt   *time.Time `bson:"nextPollAt,omitempty"`
	Attempt      int        `bson:"attempt"`
	StartedAt    *time.Time `bson:"startedAt,omitempty"`
	CompletedAt  *time.Time `bson:"completedAt,omitempty"`
}

type RunResult struct {
	MetricsReady    bool   `bson:"metricsReady"`
	MetricsLocation string `bson:"metricsLocation,omitempty"`
}

type WorkflowError struct {
	Code         string     `bson:"code,omitempty"`
	Reason       string     `bson:"reason,omitempty"`
	Retryable    *bool      `bson:"retryable,omitempty"`
	LastFailedAt *time.Time `bson:"lastFailedAt,omitempty"`
}

// CertificateAggregationWorkflow tracks one aggregation/certificate version
// per (projectId, agentId, experimentId, aggregationVersion).
type CertificateAggregationWorkflow struct {
	ID                 string             `bson:"_id,omitempty"`
	ProjectID          string             `bson:"projectId"`
	AgentID            string             `bson:"agentId"`
	AgentName          string             `bson:"agentName"`
	ExperimentID       string             `bson:"experimentId"`
	AggregationVersion int                `bson:"aggregationVersion"`
	Status             string             `bson:"status"`
	InputSnapshot      AggregationInput   `bson:"inputSnapshot"`
	Aggregation        AggregationState   `bson:"aggregation"`
	Certificate        *AggCertificate    `bson:"certificate,omitempty"`
	Error              *WorkflowError     `bson:"error,omitempty"`
	CreatedAt          time.Time          `bson:"createdAt"`
	UpdatedAt          time.Time          `bson:"updatedAt"`
	Version            int                `bson:"version"`
}

type AggregationInput struct {
	RunIDs          []string `bson:"runIds"`
	SuccessfulRuns  int      `bson:"successfulRuns"`
	FailedRuns      int      `bson:"failedRuns"`
}

type AggregationState struct {
	CertTaskID   string     `bson:"certTaskId,omitempty"`
	PollURL      string     `bson:"pollUrl,omitempty"`
	LastPolledAt *time.Time `bson:"lastPolledAt,omitempty"`
	NextPollAt   *time.Time `bson:"nextPollAt,omitempty"`
	Attempt      int        `bson:"attempt"`
	StartedAt    *time.Time `bson:"startedAt,omitempty"`
	CompletedAt  *time.Time `bson:"completedAt,omitempty"`
}

type AggCertificate struct {
	CertificationID string     `bson:"certificationId,omitempty"`
	ChecksumSha256  string     `bson:"checksumSha256,omitempty"`
	GeneratedAt     *time.Time `bson:"generatedAt,omitempty"`
	ExpiresAt       *time.Time `bson:"expiresAt,omitempty"`
}
