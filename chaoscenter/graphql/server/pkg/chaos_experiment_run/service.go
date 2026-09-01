package chaos_experiment_run

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
	"github.com/sirupsen/logrus"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaos_infrastructure"

	dbChaosExperimentRun "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment_run"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	store "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/data-store"
	dbChaosExperiment "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment"

	dbChaosInfra "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_infrastructure"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"

	"go.mongodb.org/mongo-driver/bson"
)

type Service interface {
	ProcessExperimentRunDelete(ctx context.Context, query bson.D, workflowRunID *string, experimentRun dbChaosExperimentRun.ChaosExperimentRun, workflow dbChaosExperiment.ChaosExperimentRequest, username string, r *store.StateData) error
	ProcessCompletedExperimentRun(execData ExecutionData, wfID string, runID string) (ExperimentRunMetrics, error)
	ProcessExperimentRunStop(ctx context.Context, query bson.D, experimentRunID *string, experiment dbChaosExperiment.ChaosExperimentRequest, username string, projectID string, r *store.StateData) error
}

// chaosWorkflowService is the implementation of the chaos workflow service
type chaosExperimentRunService struct {
	chaosExperimentOperator     *dbChaosExperiment.Operator
	chaosInfrastructureOperator *dbChaosInfra.Operator
	chaosExperimentRunOperator  *dbChaosExperimentRun.Operator
}

// NewChaosExperimentRunService returns a new instance of the chaos workflow run service
func NewChaosExperimentRunService(chaosWorkflowOperator *dbChaosExperiment.Operator, clusterOperator *dbChaosInfra.Operator, chaosExperimentRunOperator *dbChaosExperimentRun.Operator) Service {
	return &chaosExperimentRunService{
		chaosExperimentOperator:     chaosWorkflowOperator,
		chaosInfrastructureOperator: clusterOperator,
		chaosExperimentRunOperator:  chaosExperimentRunOperator,
	}
}

// ProcessExperimentRunDelete deletes a workflow entry and updates the database
func (c *chaosExperimentRunService) ProcessExperimentRunDelete(ctx context.Context, query bson.D, workflowRunID *string, experimentRun dbChaosExperimentRun.ChaosExperimentRun, workflow dbChaosExperiment.ChaosExperimentRequest, username string, r *store.StateData) error {
	update := bson.D{
		{"$set", bson.D{
			{"is_removed", experimentRun.IsRemoved},
			{"updated_at", time.Now().UnixMilli()},
			{"updated_by", mongodb.UserDetailResponse{
				Username: username,
			}},
		}},
	}

	err := c.chaosExperimentRunOperator.UpdateExperimentRunWithQuery(ctx, query, update)
	if err != nil {
		return err
	}
	if r != nil {
		chaos_infrastructure.SendExperimentToSubscriber(experimentRun.ProjectID, &model.ChaosExperimentRequest{
			InfraID: workflow.InfraID,
		}, &username, workflowRunID, "workflow_run_delete", r)
	}

	return nil
}

// ProcessExperimentRunStop deletes a workflow entry and updates the database
func (c *chaosExperimentRunService) ProcessExperimentRunStop(ctx context.Context, query bson.D, experimentRunID *string, experiment dbChaosExperiment.ChaosExperimentRequest, username string, projectID string, r *store.StateData) error {
	now := time.Now().UnixMilli()
	updatedBy := mongodb.UserDetailResponse{Username: username}

	update := bson.D{
		{"$set", bson.D{
			{"phase", "Stopped"},
			{"completed", true},
			{"updated_at", now},
			{"updated_by", updatedBy},
		}},
	}

	err := c.chaosExperimentRunOperator.UpdateExperimentRunWithQuery(ctx, query, update)
	if err != nil {
		return err
	}

	// Also update the denormalized copy in chaosExperiments.recent_experiment_run_details
	// so the UI reflects Stopped immediately without waiting for the Argo callback.
	if experimentRunID != nil && *experimentRunID != "" {
		expFilter := bson.D{
			{"experiment_id", experiment.ExperimentID},
			{"recent_experiment_run_details.experiment_run_id", *experimentRunID},
		}
		expUpdate := bson.D{
			{"$set", bson.D{
				{"recent_experiment_run_details.$.phase", "Stopped"},
				{"recent_experiment_run_details.$.completed", true},
				{"recent_experiment_run_details.$.updated_at", now},
				{"recent_experiment_run_details.$.updated_by", updatedBy},
			}},
		}
		// Non-fatal but always logged: without this update the UI reads stale "Running"
		// from recent_experiment_run_details even though chaosExperimentRuns is Stopped.
		// Note: the subscriber's terminal event cannot reconcile this on its own because
		// the completed=true guard on chaosExperimentRuns causes the transaction to abort.
		if err := c.chaosExperimentOperator.UpdateChaosExperiment(ctx, expFilter, expUpdate); err != nil {
			logrus.WithError(err).Warn("failed to update recent_experiment_run_details on stop; UI may show stale Running state")
		}
	}

	if r != nil {
		chaos_infrastructure.SendExperimentToSubscriber(projectID, &model.ChaosExperimentRequest{
			InfraID: experiment.InfraID,
		}, &username, experimentRunID, "workflow_run_stop", r)
	}

	return nil
}

// ProcessCompletedExperimentRun calculates the Resiliency Score and returns the updated ExecutionData
func (c *chaosExperimentRunService) ProcessCompletedExperimentRun(execData ExecutionData, wfID string, runID string) (ExperimentRunMetrics, error) {
	weightSum, totalTestResult := 0, 0
	var result ExperimentRunMetrics
	weightMap := map[string]int{}

	chaosWorkflows, err := c.chaosExperimentOperator.GetExperiment(context.TODO(), bson.D{
		{"experiment_id", wfID},
	})
	if err != nil {
		return result, fmt.Errorf("failed to get experiment from db on complete, error: %w", err)
	}
	for _, rev := range chaosWorkflows.Revision {
		if rev.RevisionID == execData.RevisionID {
			for _, weights := range rev.Weightages {
				// ITBench teardown steps (agent/app uninstall) can be present in an
				// older revision's Weightages -- skip them so they never enter the
				// resiliency-score denominator or the TotalExperiments count. See
				// utils.IsTeardownExperiment / HANDOFF §102.
				if utils.IsTeardownExperiment(weights.FaultName) {
					continue
				}
				weightMap[weights.FaultName] = weights.Weightage
				// Total weight calculated for all experiments
				weightSum = weightSum + weights.Weightage
			}
		}
	}

	result.TotalExperiments = len(weightMap)
	for _, value := range execData.Nodes {
		if value.Type == "ChaosEngine" {
			experimentName := ""
			if value.ChaosExp == nil {
				continue
			}

			// A ChaosEngine node that matches no weighted fault is not a graded fault
			// (e.g. an ITBench teardown step) -- it must not affect the score or the
			// pass/fail tallies.
			if value.ChaosExp.EngineName == "" || utils.IsTeardownExperiment(value.ChaosExp.EngineName) {
				continue
			}

			for expName := range weightMap {
				if strings.Contains(value.ChaosExp.EngineName, expName) {
					experimentName = expName
				}
			}
			weight, ok := weightMap[experimentName]
			if !ok {
				continue
			}
			// probeSuccessPercentage will be included only if chaosData is present
			x, _ := strconv.Atoi(value.ChaosExp.ProbeSuccessPercentage)
			totalTestResult += weight * x
			if value.ChaosExp.ExperimentVerdict == "Pass" {
				result.ExperimentsPassed += 1
			}
			if value.ChaosExp.ExperimentVerdict == "Fail" {
				result.ExperimentsFailed += 1
			}
			if value.ChaosExp.ExperimentVerdict == "Awaited" {
				result.ExperimentsAwaited += 1
			}
			if value.ChaosExp.ExperimentVerdict == "Stopped" {
				result.ExperimentsStopped += 1
			}
			if value.ChaosExp.ExperimentVerdict == "N/A" || value.ChaosExp.ExperimentVerdict == "" {
				result.ExperimentsNA += 1
			}
		}
	}
	if weightSum != 0 {
		result.ResiliencyScore = utils.Truncate(float64(totalTestResult) / float64(weightSum))
	}

	return result, nil
}
