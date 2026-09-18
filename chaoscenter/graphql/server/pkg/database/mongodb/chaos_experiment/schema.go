package chaos_experiment

import (
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment_run"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_infrastructure"
)

type ChaosExperimentType string

const (
	NonCronExperiment ChaosExperimentType = "experiment"
	CronExperiment    ChaosExperimentType = "cronexperiment"
	ChaosEngine       ChaosExperimentType = "chaosengine"
)

type ExperimentRunDetail struct {
	mongodb.Audit   `bson:",inline"`
	ProjectID       string   `bson:"project_id"`
	ExperimentRunID string   `bson:"experiment_run_id"`
	Phase           string   `bson:"phase"`
	NotifyID        *string  `bson:"notify_id"`
	ResiliencyScore *float64 `bson:"resiliency_score,omitempty"`
	Completed       bool     `bson:"completed"`
	RunSequence     int      `bson:"run_sequence"`
	Probe           []Probes `bson:"probes"`
}

// ChaosExperimentRequest contains the required fields to be stored in the database for a chaos experiment input
type ChaosExperimentRequest struct {
	mongodb.ResourceDetails    `bson:",inline"`
	mongodb.Audit              `bson:",inline"`
	ProjectID                  string                `bson:"project_id"`
	ExperimentID               string                `bson:"experiment_id"`
	CronSyntax                 string                `bson:"cron_syntax"`
	InfraID                    string                `bson:"infra_id"`
	ExperimentType             ChaosExperimentType   `bson:"experiment_type"`
	Revision                   []ExperimentRevision  `bson:"revision"`
	IsCustomExperiment         bool                  `bson:"is_custom_experiment"`
	RecentExperimentRunDetails []ExperimentRunDetail `bson:"recent_experiment_run_details"` // stores the details of last 10 experiment runs
	TotalExperimentRuns        int                   `bson:"total_experiment_runs"`
	// PlannedRuns is the number of runs that constitute a single certification
	// window. For Single-run and Cron schedules this is 1; for the UI's
	// "Multi Run (Sequential runs)" schedule it equals the Maximum Runs value.
	// Sourced from manifest annotations `litmuschaos.io/multiRunEnabled` +
	// `litmuschaos.io/maxRuns` stamped by the web UI.
	PlannedRuns int `bson:"planned_runs,omitempty"`
	// MultiRunState tracks progress of a sequential multi-run batch. It is held
	// in first-class BSON fields rather than inside the manifest JSON string so
	// the chain advances through atomic conditional updates, instead of a
	// read-modify-write that concurrent completion events can interleave on.
	//
	// Pointer, not a value: omitempty does not omit a zero struct, so a value
	// field would be persisted on insert with completed_run_ids encoded as BSON
	// null — and $addToSet against a null field errors ("Cannot apply $addToSet
	// to non-array field"), which would break the chain on its first completion.
	// A nil pointer is omitted, leaving the field absent, and $addToSet creates
	// the array cleanly on first use.
	MultiRunState *MultiRunState `bson:"multi_run_state,omitempty"`
}

// MultiRunState is the durable state of one sequential multi-run batch.
type MultiRunState struct {
	// Launched counts runs the chain has dispatched for the current batch. It
	// is claimed with $inc under an upper bound, so it hard-caps how many runs
	// the chain can ever start regardless of how many completion events arrive.
	Launched int `bson:"launched"`
	// CompletedRunIDs de-duplicates completion events: the subscriber can
	// re-deliver a terminal event for a run that was already counted.
	CompletedRunIDs []string `bson:"completed_run_ids"`
	// BatchDone latches once the batch reaches maxRuns, so a straggler
	// completion arriving afterwards cannot start a fresh batch.
	BatchDone bool `bson:"batch_done"`
}

// Probes details containing fault name and the probe name which it was mapped to
type Probes struct {
	FaultName  string   `bson:"fault_name" json:"faultName"`
	ProbeNames []string `bson:"probe_names" json:"probeNames"`
}

type ProbeAnnotations struct {
	Name string     `json:"name"`
	Mode model.Mode `json:"mode"`
}

type ProbesMatched struct {
	FaultName  string   `bson:"fault_name"`
	ProbeNames []string `bson:"probe_names"`
}

// ChaosExperimentsWithRunDetails contains the required fields to be stored in the database for a chaos experiment input
type ChaosExperimentsWithRunDetails struct {
	mongodb.ResourceDetails    `bson:",inline"`
	mongodb.Audit              `bson:",inline"`
	ProjectID                  string                                    `bson:"project_id"`
	ExperimentID               string                                    `bson:"experiment_id"`
	CronSyntax                 string                                    `bson:"cron_syntax"`
	Revision                   []ExperimentRevision                      `bson:"revision"`
	ExperimentType             ChaosExperimentType                       `bson:"experiment_type"`
	KubernetesInfraDetails     []chaos_infrastructure.ChaosInfra         `bson:"kubernetesInfraDetails"`
	LastExperimentRunDetails   []chaos_experiment_run.ChaosExperimentRun `bson:"lastExpRunDetails"`
	RecentExperimentRunDetails []ExperimentRunDetail                     `bson:"recent_experiment_run_details"` // stores the details of last 10 experiment runs
	TotalExperimentRuns        int                                       `bson:"total_experiment_runs"`
	AvgResScore                float64                                   `bson:"avg_resiliency_score"`
	IsCustomExperiment         bool                                      `bson:"is_custom_experiment"`
	IsRemoved                  bool                                      `bson:"is_removed"`
}

// AvgResScore contains average resiliency score
type AvgResScore struct {
	Id  string   `bson:"_id"`
	Avg *float64 `bson:"avg"`
}

// GetExperimentDetails contains details to be returned by GetExperiment handler
type GetExperimentDetails struct {
	ChaosExperimentsWithRunDetails `bson:"inline"`
	AvgResiliencyScore             []AvgResScore `bson:"avg_resiliency_score"`
}

type ExperimentRevision struct {
	RevisionID         string             `bson:"revision_id"`
	ExperimentManifest string             `bson:"experiment_manifest"`
	UpdatedAt          int64              `bson:"updated_at"`
	Weightages         []*WeightagesInput `bson:"weightages"`
	Probes             []Probes           `bson:"probes"`
}

// WeightagesInput contains the required fields to be stored in the database for a weightages input
type WeightagesInput struct {
	FaultName string `bson:"fault_name"`
	Weightage int    `bson:"weightage"`
}

type FaultEventMetadata struct {
	FaultName             string   `bson:"fault_name"`
	ServiceIdentifier     []string `bson:"service_identifier"`
	EnvironmentIdentifier []string `bson:"environment_identifier"`
}

type AggregatedExperimentRuns struct {
	TotalFilteredExperimentRuns []TotalFilteredData      `bson:"total_filtered_experiment_runs"`
	FlattenedExperimentRuns     []FlattenedExperimentRun `bson:"flattened_experiment_runs"`
}

type TotalFilteredData struct {
	Count int `bson:"count"`
}

type FlattenedExperimentRun struct {
	mongodb.Audit          `bson:",inline"`
	ProjectID              string                            `bson:"project_id"`
	ExperimentID           string                            `bson:"experiment_id"`
	ExperimentRunID        string                            `bson:"experiment_run_id"`
	CronSyntax             string                            `bson:"cron_syntax"`
	ExecutionData          string                            `bson:"execution_data"`
	RevisionID             string                            `bson:"revision_id"`
	InfraID                string                            `bson:"infra_id"`
	Phase                  string                            `bson:"phase"`
	NotifyID               *string                           `bson:"notify_id"`
	KubernetesInfraDetails []chaos_infrastructure.ChaosInfra `bson:"kubernetesInfraDetails,omitempty"`
	ExperimentDetails      []ExperimentDetails               `bson:"experiment"`
	ResiliencyScore        *float64                          `bson:"resiliency_score,string,omitempty"`
	FaultsPassed           *int                              `bson:"faults_passed,string,omitempty"`
	FaultsFailed           *int                              `bson:"faults_failed,string,omitempty"`
	FaultsAwaited          *int                              `bson:"faults_awaited,string,omitempty"`
	FaultsStopped          *int                              `bson:"faults_stopped,string,omitempty"`
	FaultsNA               *int                              `bson:"faults_na,string,omitempty"`
	TotalFaults            *int                              `bson:"total_faults,string,omitempty"`
	IsCustomExperiment     bool                              `bson:"is_custom_experiment"`
	Completed              bool                              `bson:"completed"`
	IsRemoved              bool                              `bson:"is_removed"`
	RunSequence            int64                             `bson:"run_sequence"`
}

type ExperimentDetails struct {
	ExperimentType     ChaosExperimentType  `bson:"experiment_type"`
	ExperimentName     string               `bson:"name"`
	IsCustomExperiment bool                 `bson:"is_custom_experiment"`
	Revision           []ExperimentRevision `bson:"revision"`
}

type AggregatedExperiments struct {
	TotalFilteredExperiments []TotalFilteredData              `bson:"total_filtered_experiments"`
	ScheduledExperiments     []ChaosExperimentsWithRunDetails `bson:"scheduled_experiments"`
}

type AggregatedExperimentRunStats struct {
	Id    string `bson:"_id"`
	Count int    `bson:"count"`
}

type CategorizedExperimentRunStats struct {
	Id    int `bson:"_id"`
	Count int `bson:"count"`
}

type AggregatedExperimentStats struct {
	TotalExperiments         []TotalFilteredData             `bson:"total_experiments"`
	TotalFilteredExperiments []CategorizedExperimentRunStats `bson:"categorized_by_resiliency_score"`
}

type AggregatedExperimentsWithProbes struct {
	TotalFilteredExperiments []TotalFilteredData              `bson:"total_filtered_experiments"`
	ScheduledExperiments     []ChaosExperimentsWithRunDetails `bson:"scheduled_experiments"`
	ProbesMatched            []ProbesMatched                  `bson:"probes_matched"`
}
