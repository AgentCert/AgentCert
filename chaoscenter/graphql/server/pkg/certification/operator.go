package certification

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
)

// Operator wraps Mongo CRUD operations on the three certification
// collections.  The collection types are referenced by integer enum
// constants exposed by the parent mongodb package.
type Operator struct {
	op            mongodb.MongoOperator
	experimentCol int
	runCol        int
	aggCol        int
}

// NewOperator builds an Operator using the collection-type integers
// registered in pkg/database/mongodb/init.go.
func NewOperator(op mongodb.MongoOperator, experimentCol, runCol, aggCol int) *Operator {
	return &Operator{op: op, experimentCol: experimentCol, runCol: runCol, aggCol: aggCol}
}

// ----------------- certificate_experiments -----------------

func (o *Operator) GetExperiment(ctx context.Context, projectID, experimentID string) (*CertificateExperiment, error) {
	res, err := o.op.Get(ctx, o.experimentCol, bson.D{
		{Key: "projectId", Value: projectID},
		{Key: "experimentId", Value: experimentID},
	})
	if err != nil {
		return nil, err
	}
	var doc CertificateExperiment
	if err := res.Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

// UpsertExperiment ensures a parent doc exists with the supplied identity
// fields.  Counters and status are NOT overwritten when the doc already
// exists — those are mutated only via dedicated update helpers.
func (o *Operator) UpsertExperiment(ctx context.Context, doc *CertificateExperiment) error {
	now := time.Now().UTC()
	doc.UpdatedAt = now
	if doc.AggregationPolicy.Mode == "" {
		doc.AggregationPolicy.Mode = PolicyAllRunsCompleted
	}
	filter := bson.D{
		{Key: "projectId", Value: doc.ProjectID},
		{Key: "experimentId", Value: doc.ExperimentID},
	}
	update := bson.D{
		{Key: "$setOnInsert", Value: bson.D{
			{Key: "projectId", Value: doc.ProjectID},
			{Key: "agentId", Value: doc.AgentID},
			{Key: "agentName", Value: doc.AgentName},
			{Key: "experimentId", Value: doc.ExperimentID},
			{Key: "status", Value: ExperimentStatusRunsInProgress},
			{Key: "runCounts", Value: RunCounts{}},
			{Key: "aggregationPolicy", Value: doc.AggregationPolicy},
			{Key: "activeAggregationVersion", Value: 0},
			{Key: "createdAt", Value: now},
		}},
		// $max ensures the field is set on insert AND can ratchet up if a
		// later run was started with a corrected planned-runs value.
		{Key: "$max", Value: bson.D{{Key: "expectedRuns", Value: doc.ExpectedRuns}}},
		{Key: "$set", Value: bson.D{{Key: "updatedAt", Value: now}}},
	}
	_, err := o.op.Update(ctx, o.experimentCol, filter, update, options.Update().SetUpsert(true))
	return err
}

// ResetStatusIfCertified transitions the experiment from EXPERIMENT_CERTIFICATE_READY
// or AGGREGATION_FAILED back to RUNS_IN_PROGRESS when a new or replacement run
// arrives after the previous attempt finalized or failed. It is a no-op for
// any other status.
func (o *Operator) ResetStatusIfCertified(ctx context.Context, projectID, experimentID string) error {
	_, err := o.op.Update(ctx, o.experimentCol,
		bson.D{
			{Key: "projectId", Value: projectID},
			{Key: "experimentId", Value: experimentID},
			{Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{
				ExperimentStatusCertificateReady,
				ExperimentStatusAggregationFailed,
			}}}},
		},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "status", Value: ExperimentStatusRunsInProgress},
			{Key: "updatedAt", Value: time.Now().UTC()},
		}}},
	)
	return err
}

func (o *Operator) UpdateExperimentStatus(ctx context.Context, projectID, experimentID, status string) error {
	_, err := o.op.Update(ctx, o.experimentCol,
		bson.D{{Key: "projectId", Value: projectID}, {Key: "experimentId", Value: experimentID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "status", Value: status},
			{Key: "updatedAt", Value: time.Now().UTC()},
		}}},
	)
	return err
}

func (o *Operator) FinalizeExperimentCertificate(ctx context.Context, projectID, experimentID string, version int, cert LatestCertificate) error {
	_, err := o.op.Update(ctx, o.experimentCol,
		bson.D{{Key: "projectId", Value: projectID}, {Key: "experimentId", Value: experimentID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "status", Value: ExperimentStatusCertificateReady},
			{Key: "activeAggregationVersion", Value: version},
			{Key: "latestCertificate", Value: cert},
			{Key: "updatedAt", Value: time.Now().UTC()},
		}}},
	)
	return err
}

// ----------------- certificate_run_workflows -----------------

func (o *Operator) GetRunWorkflow(ctx context.Context, projectID, agentID, experimentID, runID string) (*CertificateRunWorkflow, error) {
	res, err := o.op.Get(ctx, o.runCol, bson.D{
		{Key: "projectId", Value: projectID},
		{Key: "agentId", Value: agentID},
		{Key: "experimentId", Value: experimentID},
		{Key: "experimentRunId", Value: runID},
	})
	if err != nil {
		return nil, err
	}
	var doc CertificateRunWorkflow
	if err := res.Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

// UpsertRunWorkflowInitial creates the run-workflow doc on first sight,
// transitioning to BUCKETING_TRIGGERED.  It is idempotent: subsequent
// calls leave the existing doc untouched.
func (o *Operator) UpsertRunWorkflowInitial(ctx context.Context, doc *CertificateRunWorkflow) error {
	now := time.Now().UTC()
	filter := bson.D{
		{Key: "projectId", Value: doc.ProjectID},
		{Key: "agentId", Value: doc.AgentID},
		{Key: "experimentId", Value: doc.ExperimentID},
		{Key: "experimentRunId", Value: doc.ExperimentRunID},
	}
	update := bson.D{
		{Key: "$setOnInsert", Value: bson.D{
			{Key: "projectId", Value: doc.ProjectID},
			{Key: "agentId", Value: doc.AgentID},
			{Key: "experimentId", Value: doc.ExperimentID},
			{Key: "experimentRunId", Value: doc.ExperimentRunID},
			{Key: "status", Value: RunStatusBucketingTriggered},
			{Key: "bucketing", Value: BucketingState{}},
			{Key: "result", Value: RunResult{}},
			{Key: "createdAt", Value: now},
			{Key: "version", Value: 0},
		}},
		{Key: "$set", Value: bson.D{{Key: "updatedAt", Value: now}}},
	}
	_, err := o.op.Update(ctx, o.runCol, filter, update, options.Update().SetUpsert(true))
	return err
}

// ResetFailedRunWorkflow resets a BUCKETING_FAILED run-workflow doc back to
// BUCKETING_TRIGGERED, clearing the stale task ID, poll URL, and error so the
// pipeline retries bucketing from scratch.  It is a no-op when the doc does
// not exist or is in any state other than BUCKETING_FAILED.
func (o *Operator) ResetFailedRunWorkflow(ctx context.Context, projectID, agentID, experimentID, runID string) error {
	now := time.Now().UTC()
	_, err := o.op.Update(ctx, o.runCol,
		bson.D{
			{Key: "projectId", Value: projectID},
			{Key: "agentId", Value: agentID},
			{Key: "experimentId", Value: experimentID},
			{Key: "experimentRunId", Value: runID},
			{Key: "status", Value: RunStatusBucketingFailed},
		},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "status", Value: RunStatusBucketingTriggered},
			{Key: "bucketing", Value: BucketingState{}},
			{Key: "result", Value: RunResult{}},
			{Key: "error", Value: nil},
			{Key: "updatedAt", Value: now},
		}}},
	)
	return err
}

func (o *Operator) UpdateRunWorkflow(ctx context.Context, projectID, agentID, experimentID, runID string, set bson.D) error {
	set = append(set, bson.E{Key: "updatedAt", Value: time.Now().UTC()})
	_, err := o.op.Update(ctx, o.runCol,
		bson.D{
			{Key: "projectId", Value: projectID},
			{Key: "agentId", Value: agentID},
			{Key: "experimentId", Value: experimentID},
			{Key: "experimentRunId", Value: runID},
		},
		bson.D{
			{Key: "$set", Value: set},
			{Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}},
		},
	)
	return err
}

func (o *Operator) ListRunWorkflows(ctx context.Context, projectID, agentID, experimentID string) ([]CertificateRunWorkflow, error) {
	cur, err := o.op.List(ctx, o.runCol, bson.D{
		{Key: "projectId", Value: projectID},
		{Key: "agentId", Value: agentID},
		{Key: "experimentId", Value: experimentID},
	})
	if err != nil {
		return nil, err
	}
	var docs []CertificateRunWorkflow
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// ----------------- certificate_aggregation_workflows -----------------

func (o *Operator) GetAggregationWorkflow(ctx context.Context, projectID, agentID, experimentID string, version int) (*CertificateAggregationWorkflow, error) {
	res, err := o.op.Get(ctx, o.aggCol, bson.D{
		{Key: "projectId", Value: projectID},
		{Key: "agentId", Value: agentID},
		{Key: "experimentId", Value: experimentID},
		{Key: "aggregationVersion", Value: version},
	})
	if err != nil {
		return nil, err
	}
	var doc CertificateAggregationWorkflow
	if err := res.Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

// GetLatestAggregationWorkflow returns the aggregation workflow with the
// highest aggregationVersion for the given (project, agent, experiment).
// Returns (nil, nil) when no aggregation row exists yet.
func (o *Operator) GetLatestAggregationWorkflow(ctx context.Context, projectID, agentID, experimentID string) (*CertificateAggregationWorkflow, error) {
	cur, err := o.op.List(ctx, o.aggCol,
		bson.D{
			{Key: "projectId", Value: projectID},
			{Key: "agentId", Value: agentID},
			{Key: "experimentId", Value: experimentID},
		},
		options.Find().SetSort(bson.D{{Key: "aggregationVersion", Value: -1}}).SetLimit(1),
	)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	if !cur.Next(ctx) {
		return nil, nil
	}
	var doc CertificateAggregationWorkflow
	if err := cur.Decode(&doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func (o *Operator) CreateAggregationWorkflow(ctx context.Context, doc *CertificateAggregationWorkflow) error {
	now := time.Now().UTC()
	doc.CreatedAt = now
	doc.UpdatedAt = now
	if err := o.op.Create(ctx, o.aggCol, doc); err != nil {
		return fmt.Errorf("create aggregation workflow: %w", err)
	}
	return nil
}

func (o *Operator) UpdateAggregationWorkflow(ctx context.Context, projectID, agentID, experimentID string, version int, set bson.D) error {
	set = append(set, bson.E{Key: "updatedAt", Value: time.Now().UTC()})
	_, err := o.op.Update(ctx, o.aggCol,
		bson.D{
			{Key: "projectId", Value: projectID},
			{Key: "agentId", Value: agentID},
			{Key: "experimentId", Value: experimentID},
			{Key: "aggregationVersion", Value: version},
		},
		bson.D{
			{Key: "$set", Value: set},
			{Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}},
		},
	)
	return err
}
