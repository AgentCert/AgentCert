package ops

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	probeUtils "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/probe/utils"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"
	"github.com/sirupsen/logrus"

	agentRegistry "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/agent_registry"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaos_infrastructure"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/agenthub"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/observability"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
	dbChaosExperimentRun "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment_run"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	store "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/data-store"
	dbChaosExperiment "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment"

	dbChaosInfra "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_infrastructure"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/ghodss/yaml"
	"github.com/google/uuid"
	chaosTypes "github.com/litmuschaos/chaos-operator/api/litmuschaos/v1alpha1"
	scheduleTypes "github.com/litmuschaos/chaos-scheduler/api/litmuschaos/v1alpha1"
	probe "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/probe/handler"
	"go.mongodb.org/mongo-driver/bson"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Service interface {
	ProcessExperiment(ctx context.Context, workflow *model.ChaosExperimentRequest, projectID string, revID string) (*model.ChaosExperimentRequest, *dbChaosExperiment.ChaosExperimentType, error)
	ProcessExperimentCreation(ctx context.Context, input *model.ChaosExperimentRequest, username string, projectID string, wfType *dbChaosExperiment.ChaosExperimentType, revisionID string, r *store.StateData) error
	ProcessExperimentUpdate(workflow *model.ChaosExperimentRequest, username string, wfType *dbChaosExperiment.ChaosExperimentType, revisionID string, updateRevision bool, projectID string, r *store.StateData) error
	ProcessExperimentDelete(query bson.D, workflow dbChaosExperiment.ChaosExperimentRequest, username string, r *store.StateData) error
	UpdateRuntimeCronWorkflowConfiguration(cronWorkflowManifest v1alpha1.CronWorkflow, experiment dbChaosExperiment.ChaosExperimentRequest) (v1alpha1.CronWorkflow, []string, error)
}

// chaosWorkflowService is the implementation of the chaos workflow service
type chaosExperimentService struct {
	chaosExperimentOperator     *dbChaosExperiment.Operator
	chaosInfrastructureOperator *dbChaosInfra.Operator
	chaosExperimentRunOperator  *dbChaosExperimentRun.Operator
	probeService                probe.Service
	agentRegistryOperator       agentRegistry.Operator
}

// NewChaosExperimentService returns a new instance of the chaos workflow service
func NewChaosExperimentService(chaosWorkflowOperator *dbChaosExperiment.Operator, clusterOperator *dbChaosInfra.Operator, chaosExperimentRunOperator *dbChaosExperimentRun.Operator, probeService probe.Service, agentRegOp agentRegistry.Operator) Service {
	return &chaosExperimentService{
		chaosExperimentOperator:     chaosWorkflowOperator,
		chaosInfrastructureOperator: clusterOperator,
		chaosExperimentRunOperator:  chaosExperimentRunOperator,
		probeService:                probeService,
		agentRegistryOperator:       agentRegOp,
	}
}

// ProcessExperiment takes the workflow and processes it as required
func (c *chaosExperimentService) ProcessExperiment(ctx context.Context, workflow *model.ChaosExperimentRequest, projectID string, revID string) (*model.ChaosExperimentRequest, *dbChaosExperiment.ChaosExperimentType, error) {
	// security check for chaos_infra access
	infra, err := c.chaosInfrastructureOperator.GetInfra(workflow.InfraID)
	if err != nil {
		return nil, nil, errors.New("failed to get infra details: " + err.Error())
	}

	if !infra.IsActive {
		return nil, nil, errors.New("experiment scheduling failed due to inactive infra")
	}

	if !infra.IsInfraConfirmed {
		return nil, nil, errors.New("experiment scheduling failed due to unconfirmed infra")
	}

	if infra.IsRemoved {
		return nil, nil, errors.New("experiment scheduling failed due to removed infra")
	}

	if infra.ProjectID != projectID {
		return nil, nil, errors.New("ProjectID doesn't match with the chaos_infra identifiers")
	}

	wfType := dbChaosExperiment.NonCronExperiment
	var (
		workflowID = uuid.New().String()
		weights    = make(map[string]int)
		objMeta    unstructured.Unstructured
	)

	if len(workflow.Weightages) > 0 {
		for _, weight := range workflow.Weightages {
			weights[weight.FaultName] = weight.Weightage
		}
	}

	if workflow.ExperimentID == nil || (*workflow.ExperimentID) == "" {
		workflow.ExperimentID = &workflowID
	}

	err = json.Unmarshal([]byte(workflow.ExperimentManifest), &objMeta)
	if err != nil {
		return nil, nil, errors.New("failed to unmarshal workflow manifest1")
	}

	// workflow name in struct should match with actual workflow name
	if workflow.ExperimentName != objMeta.GetName() {
		return nil, nil, errors.New(objMeta.GetKind() + " name doesn't match")
	}

	switch strings.ToLower(objMeta.GetKind()) {
	case "workflow":
		{
			err = c.processExperimentManifest(ctx, workflow, weights, revID, projectID)
			if err != nil {
				return nil, nil, err
			}
		}
	case "cronworkflow":
		{
			wfType = dbChaosExperiment.CronExperiment
			err = c.processCronExperimentManifest(ctx, workflow, weights, revID, projectID)
			if err != nil {
				return nil, nil, err
			}
		}
	case "chaosengine":
		{
			wfType = dbChaosExperiment.ChaosEngine
			err = c.processChaosEngineManifest(ctx, workflow, weights, revID, projectID)
			if err != nil {
				return nil, nil, err
			}

		}
	case "chaosschedule":
		{
			wfType = dbChaosExperiment.ChaosEngine
			err = c.processChaosScheduleManifest(ctx, workflow, weights, revID, projectID)
			if err != nil {
				return nil, nil, err
			}
		}
	default:
		{
			return nil, nil, errors.New("not a valid object, only workflows/cron workflows/chaos engines supported")
		}
	}

	return workflow, &wfType, nil
}

// ProcessExperimentCreation creates new workflow entry and sends the workflow to the specific chaos_infra for execution
func (c *chaosExperimentService) ProcessExperimentCreation(ctx context.Context, input *model.ChaosExperimentRequest, username string, projectID string, wfType *dbChaosExperiment.ChaosExperimentType, revisionID string, r *store.StateData) error {
	var (
		weightages []*dbChaosExperiment.WeightagesInput
		revision   []dbChaosExperiment.ExperimentRevision
	)
	if input.Weightages != nil {
		//TODO: Once we make the new chaos terminology change in APIs, then we can we the copier instead of for loop
		for _, v := range input.Weightages {
			weightages = append(weightages, &dbChaosExperiment.WeightagesInput{
				FaultName: v.FaultName,
				Weightage: v.Weightage,
			})
		}
	}

	timeNow := time.Now().UnixMilli()

	revision = append(revision, dbChaosExperiment.ExperimentRevision{
		RevisionID:         revisionID,
		ExperimentManifest: input.ExperimentManifest,
		UpdatedAt:          timeNow,
		Weightages:         weightages,
	})

	newChaosExperiment := dbChaosExperiment.ChaosExperimentRequest{
		ExperimentID:       *input.ExperimentID,
		CronSyntax:         input.CronSyntax,
		ExperimentType:     *wfType,
		IsCustomExperiment: input.IsCustomExperiment,
		InfraID:            input.InfraID,
		ResourceDetails: mongodb.ResourceDetails{
			Name:        input.ExperimentName,
			Description: input.ExperimentDescription,
			Tags:        input.Tags,
		},
		ProjectID: projectID,
		Audit: mongodb.Audit{
			CreatedAt: timeNow,
			UpdatedAt: timeNow,
			IsRemoved: false,
			CreatedBy: mongodb.UserDetailResponse{
				Username: username,
			},
			UpdatedBy: mongodb.UserDetailResponse{
				Username: username,
			},
		},
		Revision:                   revision,
		RecentExperimentRunDetails: []dbChaosExperiment.ExperimentRunDetail{},
	}

	err := c.chaosExperimentOperator.InsertChaosExperiment(ctx, newChaosExperiment)
	if err != nil {
		return err
	}
	if r != nil {
		chaos_infrastructure.SendExperimentToSubscriber(projectID, input, &username, nil, "create", r)
	}
	return nil
}

// ProcessExperimentUpdate updates the workflow entry and sends update resource request to required agent
func (c *chaosExperimentService) ProcessExperimentUpdate(workflow *model.ChaosExperimentRequest, username string, wfType *dbChaosExperiment.ChaosExperimentType, revisionID string, updateRevision bool, projectID string, r *store.StateData) error {
	var (
		weightages  []*dbChaosExperiment.WeightagesInput
		workflowObj unstructured.Unstructured
	)

	if workflow.Weightages != nil {
		//TODO: Once we make the new chaos terminology change in APIs, then we can use the copier instead of for loop
		for _, v := range workflow.Weightages {
			weightages = append(weightages, &dbChaosExperiment.WeightagesInput{
				FaultName: v.FaultName,
				Weightage: v.Weightage,
			})
		}
	}

	workflowRevision := dbChaosExperiment.ExperimentRevision{
		RevisionID:         revisionID,
		ExperimentManifest: workflow.ExperimentManifest,
		UpdatedAt:          time.Now().UnixMilli(),
		Weightages:         weightages,
	}

	query := bson.D{
		{"experiment_id", workflow.ExperimentID},
		{"project_id", projectID},
	}

	update := bson.D{
		{"$set", bson.D{
			{"experiment_type", *wfType},
			{"cron_syntax", workflow.CronSyntax},
			{"name", workflow.ExperimentName},
			{"tags", workflow.Tags},
			{"infra_id", workflow.InfraID},
			{"description", workflow.ExperimentDescription},
			{"is_custom_experiment", workflow.IsCustomExperiment},
			{"updated_at", time.Now().UnixMilli()},
			{"updated_by", mongodb.UserDetailResponse{
				Username: username,
			}},
		}},
		{"$push", bson.D{
			{"revision", workflowRevision},
		}},
	}

	// This case is required while disabling/enabling cron experiments
	if updateRevision {
		query = bson.D{
			{"experiment_id", workflow.ExperimentID},
			{"project_id", projectID},
			{"revision.revision_id", revisionID},
		}
		update = bson.D{
			{"$set", bson.D{
				{"updated_at", time.Now().UnixMilli()},
				{"updated_by", mongodb.UserDetailResponse{
					Username: username,
				}},
				{"revision.$.updated_at", time.Now().UnixMilli()},
				{"revision.$.experiment_manifest", workflow.ExperimentManifest},
			}},
		}
	}

	err := c.chaosExperimentOperator.UpdateChaosExperiment(context.Background(), query, update)
	if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(workflow.ExperimentManifest), &workflowObj)
	if err != nil {
		return errors.New("failed to unmarshal workflow manifest")
	}

	if r != nil {
		chaos_infrastructure.SendExperimentToSubscriber(projectID, workflow, &username, nil, "update", r)
	}
	return nil
}

// ProcessExperimentDelete deletes the workflow entry and sends delete resource request to required chaos_infra
func (c *chaosExperimentService) ProcessExperimentDelete(query bson.D, workflow dbChaosExperiment.ChaosExperimentRequest, username string, r *store.StateData) error {
	var (
		wc      = writeconcern.New(writeconcern.WMajority())
		rc      = readconcern.Snapshot()
		txnOpts = options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)
		ctx     = context.Background()
	)

	session, err := mongodb.MgoClient.StartSession()
	if err != nil {
		return err
	}

	err = mongo.WithSession(ctx, session, func(sessionContext mongo.SessionContext) error {
		if err = session.StartTransaction(txnOpts); err != nil {
			return err
		}

		//Update chaosExperiments collection
		update := bson.D{
			{"$set", bson.D{
				{"is_removed", true},
				{"updated_by", mongodb.UserDetailResponse{
					Username: username,
				}},
				{"updated_at", time.Now().UnixMilli()},
			}},
		}
		err = c.chaosExperimentOperator.UpdateChaosExperiment(sessionContext, query, update)
		if err != nil {
			return err
		}

		//Update chaosExperimentRuns collection
		err = c.chaosExperimentRunOperator.UpdateExperimentRunsWithQuery(sessionContext, bson.D{{"experiment_id", workflow.ExperimentID}}, update)
		if err != nil {
			return err
		}
		if err = session.CommitTransaction(sessionContext); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		if abortErr := session.AbortTransaction(ctx); abortErr != nil {
			return abortErr
		}
		return err
	}

	session.EndSession(ctx)
	if r != nil {
		chaos_infrastructure.SendExperimentToSubscriber(workflow.ProjectID, &model.ChaosExperimentRequest{
			InfraID: workflow.InfraID,
		}, &username, &workflow.ExperimentID, "workflow_delete", r)
	}

	return nil
}

func (c *chaosExperimentService) processExperimentManifest(ctx context.Context, workflow *model.ChaosExperimentRequest, weights map[string]int, revID, projectID string) error {
	var (
		newWeights       []*model.WeightagesInput
		workflowManifest v1alpha1.Workflow
	)

	err := json.Unmarshal([]byte(workflow.ExperimentManifest), &workflowManifest)
	if err != nil {
		return errors.New("failed to unmarshal workflow manifest")
	}

	applyInstallAgentTemplateOverrides(workflowManifest.Spec.Templates)
	applyInstallApplicationTemplateOverrides(workflowManifest.Spec.Templates)
	injectExperimentContextArgs(workflowManifest.Spec.Templates)

	// Inject agentId as a workflow-level parameter so that install-agent
	// can forward it via --set agentId={{workflow.parameters.agentId}}.
	// Always ensure agentId is present as a workflow parameter so that
	// {{workflow.parameters.agentId}} is resolvable by Argo even on first run.
	// Try to resolve it from MongoDB; fall back to empty string (install-agent
	// will call RegisterAgent itself when it receives an empty agentId).
	if c.agentRegistryOperator != nil {
		agentIDStr := ""
		if infra, err := c.chaosInfrastructureOperator.GetInfra(workflow.InfraID); err == nil && infra.InfraNamespace != nil {
			// Extract the correct namespace from the install-agent template args.
			agentNS := ExtractInstallAgentNamespace(workflowManifest.Spec.Templates)
			if agentNS == "" {
				agentNS = *infra.InfraNamespace // fallback to infra namespace
			}
			if agent, agentErr := c.agentRegistryOperator.GetAgentByNamespace(ctx, agentNS); agentErr == nil && agent != nil {
				agentIDStr = agent.AgentID
				logrus.WithField("agentId", agentIDStr).Info("resolved agentId from registry")
			} else {
				logrus.WithField("namespace", agentNS).Info("no agent record found; agentId will be empty (install-agent will self-register)")
			}
		}
		// Inject the parameter, replacing any existing value to keep exactly one entry.
		found := false
		for i, p := range workflowManifest.Spec.Arguments.Parameters {
			if p.Name == "agentId" {
				workflowManifest.Spec.Arguments.Parameters[i].Value = v1alpha1.AnyStringPtr(agentIDStr)
				found = true
				break
			}
		}
		if !found {
			workflowManifest.Spec.Arguments.Parameters = append(workflowManifest.Spec.Arguments.Parameters, v1alpha1.Parameter{
				Name:  "agentId",
				Value: v1alpha1.AnyStringPtr(agentIDStr),
			})
		}
		logrus.WithField("agentId", agentIDStr).Info("injected agentId workflow parameter")
	}

	if workflowManifest.Labels == nil {
		workflowManifest.Labels = map[string]string{
			"workflow_id":      *workflow.ExperimentID,
			"infra_id":         workflow.InfraID,
			"workflows.argoproj.io/controller-instanceid": workflow.InfraID,
			"revision_id":      revID,
			"experiment_name":  workflow.ExperimentName,
		}
	} else {
		workflowManifest.Labels["workflow_id"] = *workflow.ExperimentID
		workflowManifest.Labels["infra_id"] = workflow.InfraID
		workflowManifest.Labels["workflows.argoproj.io/controller-instanceid"] = workflow.InfraID
		workflowManifest.Labels["revision_id"] = revID
		workflowManifest.Labels["experiment_name"] = workflow.ExperimentName
	}

	for i, template := range workflowManifest.Spec.Templates {
		artifact := template.Inputs.Artifacts
		if len(artifact) > 0 {
			if artifact[0].Raw == nil {
				continue
			}
			rawData := artifact[0].Raw.Data
			if !workflow.IsCustomExperiment {
				if normalizedRaw, changed, normErr := normalizeProbeExecutionSettings(rawData); normErr != nil {
					logrus.WithError(normErr).Warn("failed to normalize probe execution settings")
				} else if changed {
					artifact[0].Raw.Data = normalizedRaw
					rawData = normalizedRaw
				}
			}

			var data = rawData
			if len(data) > 0 {
				// This replacement is required because chaos engine yaml have a syntax template. example:{{ workflow.parameters.adminModeNamespace }}
				// And it is not able the unmarshal the yamlstring to chaos engine struct
				data = strings.ReplaceAll(data, "{{", "")
				data = strings.ReplaceAll(data, "}}", "")

				var meta chaosTypes.ChaosEngine
				err := yaml.Unmarshal([]byte(data), &meta)
				if err != nil {
					return errors.New("failed to unmarshal chaosengine")
				}

				if strings.ToLower(meta.Kind) == "chaosengine" {
					var exprname string
					if len(meta.Spec.Experiments) > 0 {
						exprname = meta.GenerateName
						if len(exprname) == 0 {
							return errors.New("empty chaos experiment name")
						}
					} else {
						return errors.New("no experiments specified in chaosengine - " + meta.Name)
					}

					// Check if probeRef annotation is present in chaosengine, if not then create new probes
					if _, ok := meta.GetObjectMeta().GetAnnotations()["probeRef"]; !ok {
						// Check if probes are specified in chaosengine
						if meta.Spec.Experiments[0].Spec.Probe != nil {
							type probeRef struct {
								Name string `json:"name"`
								Mode string `json:"mode"`
							}
							probeRefs := []probeRef{}
							for _, p := range meta.Spec.Experiments[0].Spec.Probe {
								// Generate new probes for the experiment
								probe, err := probeUtils.ProbeInputsToProbeRequestConverter(p)
								if err != nil {
									return err
								}
								result, err := c.probeService.AddProbe(ctx, probe, projectID)
								if err != nil {
									return err
								}
								// If probes are created then update the probeRef annotation in chaosengine
								probeRefs = append(probeRefs, probeRef{
									Name: result.Name,
									Mode: p.Mode,
								})
							}
							probeRefBytes, _ := json.Marshal(probeRefs)
							rawYaml, err := probeUtils.InsertProbeRefAnnotation(artifact[0].Raw.Data, string(probeRefBytes))
							if err != nil {
								return err
							}
							artifact[0].Raw.Data = rawYaml
						} else {
							return errors.New("no probes specified in chaosengine - " + meta.Name)
						}
					}

					if val, ok := weights[exprname]; ok {
						workflowManifest.Spec.Templates[i].Metadata.Labels = map[string]string{
							"weight": strconv.Itoa(val),
						}
					} else if val, ok := workflowManifest.Spec.Templates[i].Metadata.Labels["weight"]; ok {
						intVal, err := strconv.Atoi(val)
						if err != nil {
							return errors.New("failed to convert")
						}
						newWeights = append(newWeights, &model.WeightagesInput{
							FaultName: exprname,
							Weightage: intVal,
						})
					} else {
						newWeights = append(newWeights, &model.WeightagesInput{
							FaultName: exprname,
							Weightage: 10,
						})

						workflowManifest.Spec.Templates[i].Metadata.Labels = map[string]string{
							"weight": "10",
						}
					}
				}
			}
		}
	}

	workflow.Weightages = append(workflow.Weightages, newWeights...)

	// Apply readiness normalization patch automatically
	// RBAC is now handled at infra setup time via enable-chaos-infra.sh with admin identity
	err = c.applyInstallApplicationReadinessPatch(&workflowManifest)
	if err != nil {
		logrus.Errorf("Failed to apply readiness patch: %v", err)
		// Log but don't fail - readiness patch is optional
	}

	err = c.applyPreCleanupWaitPatch(&workflowManifest)
	if err != nil {
		logrus.Errorf("Failed to apply pre-cleanup wait patch: %v", err)
		// Log but don't fail - wait patch is optional
	}

	err = c.applyUninstallAllPatch(&workflowManifest)
	if err != nil {
		logrus.Errorf("Failed to apply uninstall-all patch: %v", err)
		// Log but don't fail - uninstall patch is optional
	}

	// Enable Argo podGC so completed executor pods in litmus-exp are deleted automatically.
	workflowManifest.Spec.PodGC = &v1alpha1.PodGC{Strategy: v1alpha1.PodGCOnWorkflowCompletion}

	out, err := json.Marshal(workflowManifest)
	if err != nil {
		return err
	}

	workflow.ExperimentManifest = string(out)
	return nil
}

// applyRBACPatch injects a step before install-application to grant the Litmus server
// service account workload-discovery permissions required for dynamic app kind detection.
func (c *chaosExperimentService) applyRBACPatch(wf *v1alpha1.Workflow) error {
	if wf == nil || len(wf.Spec.Templates) == 0 {
		return nil
	}

	entrypoint := wf.Spec.Entrypoint
	if entrypoint == "" {
		return nil
	}

	var rootTemplate *v1alpha1.Template
	for i, t := range wf.Spec.Templates {
		if t.Name == entrypoint {
			rootTemplate = &wf.Spec.Templates[i]
			break
		}
	}

	if rootTemplate == nil || len(rootTemplate.Steps) == 0 {
		return nil
	}

	stepGroupIdx := -1
	for i, stepGroup := range rootTemplate.Steps {
		for _, step := range stepGroup.Steps {
			if step.Name == "install-application" {
				stepGroupIdx = i
				break
			}
		}
		if stepGroupIdx >= 0 {
			break
		}
	}

	if stepGroupIdx < 0 {
		return nil
	}

	rbacStepName := "apply-workload-rbac"
	rbacTemplateExists := false
	for _, t := range wf.Spec.Templates {
		if t.Name == rbacStepName {
			rbacTemplateExists = true
			break
		}
	}

	if !rbacTemplateExists {
		rbacTpl := v1alpha1.Template{
			Name: rbacStepName,
			Container: &corev1.Container{
				Image:   "litmuschaos/k8s:latest",
				Command: []string{"sh", "-c"},
				Args: []string{`set -eu

discover_target_subject() {
	SA_NAME=""
	SA_NAMESPACE=""

	# Prefer this workflow pod's own service account (the identity that runs
	# install/apply steps and needs the permissions on any cluster).
	SA_NAMESPACE=$(cat /var/run/secrets/kubernetes.io/serviceaccount/namespace 2>/dev/null || true)
	POD_NAME=$(hostname)
	if [ -n "$SA_NAMESPACE" ] && [ -n "$POD_NAME" ]; then
		SA_NAME=$(kubectl -n "$SA_NAMESPACE" get pod "$POD_NAME" -o jsonpath='{.spec.serviceAccountName}' 2>/dev/null || true)
	fi

	# Otherwise, prefer the existing infra binding subject if already configured.
	if [ -z "$SA_NAME" ] || [ -z "$SA_NAMESPACE" ]; then
		SA_NAME=$(kubectl get clusterrolebinding infra-cluster-role-binding -o jsonpath='{.subjects[0].name}' 2>/dev/null || true)
		SA_NAMESPACE=$(kubectl get clusterrolebinding infra-cluster-role-binding -o jsonpath='{.subjects[0].namespace}' 2>/dev/null || true)
	fi

	# Last fallback: discover likely control-plane server deployment SA.
	if [ -z "$SA_NAME" ] || [ -z "$SA_NAMESPACE" ]; then
		CANDIDATE=$(kubectl get deploy -A --no-headers 2>/dev/null | awk '$2 ~ /(server|graphql|portal)/ {print $1" "$2; exit}')
		if [ -n "$CANDIDATE" ]; then
			SA_NAMESPACE=$(echo "$CANDIDATE" | awk '{print $1}')
			DEPLOY_NAME=$(echo "$CANDIDATE" | awk '{print $2}')
			SA_NAME=$(kubectl -n "$SA_NAMESPACE" get deploy "$DEPLOY_NAME" -o jsonpath='{.spec.template.spec.serviceAccountName}' 2>/dev/null || true)
		fi
	fi

	if [ -z "$SA_NAME" ] || [ -z "$SA_NAMESPACE" ]; then
		echo "[rbac-patch] failed to discover service account subject"
		return 1
	fi

	echo "$SA_NAME|$SA_NAMESPACE"
}

SUBJECT=$(discover_target_subject)
SA_NAME=$(echo "$SUBJECT" | cut -d'|' -f1)
SA_NAMESPACE=$(echo "$SUBJECT" | cut -d'|' -f2)

RBAC_SUBJECT_SUFFIX=$(printf "%s-%s" "$SA_NAMESPACE" "$SA_NAME" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9-' '-' | sed 's/^-*//;s/-*$//' | cut -c1-25)
WORKLOAD_CRB_NAME="litmus-workload-discoverer-${RBAC_SUBJECT_SUFFIX}"
APP_RB_NAME="litmus-app-installer-binding-${RBAC_SUBJECT_SUFFIX}"

echo "[rbac-patch] applying workload-discovery RBAC for serviceaccount ${SA_NAMESPACE}/${SA_NAME}"

kubectl apply -f - <<'EOF'
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: litmus-workload-discoverer
rules:
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
  verbs: ["list", "get", "watch"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["list", "get"]
EOF

kubectl create clusterrolebinding "${WORKLOAD_CRB_NAME}" \
	--clusterrole=litmus-workload-discoverer \
	--serviceaccount="${SA_NAMESPACE}:${SA_NAME}" \
	--dry-run=client -o yaml | kubectl apply -f -

echo "[rbac-patch] ClusterRole and ClusterRoleBinding for workload-discovery applied"`},
			},
		}

		wf.Spec.Templates = append(wf.Spec.Templates, rbacTpl)
	}

	// Remove any stale placements of apply-workload-rbac so it can be re-inserted
	// deterministically before install-application.
	filteredSteps := make([]v1alpha1.ParallelSteps, 0, len(rootTemplate.Steps))
	for _, stepGroup := range rootTemplate.Steps {
		newGroup := v1alpha1.ParallelSteps{Steps: make([]v1alpha1.WorkflowStep, 0, len(stepGroup.Steps))}
		for _, step := range stepGroup.Steps {
			if step.Name == rbacStepName {
				continue
			}
			newGroup.Steps = append(newGroup.Steps, step)
		}
		if len(newGroup.Steps) > 0 {
			filteredSteps = append(filteredSteps, newGroup)
		}
	}
	rootTemplate.Steps = filteredSteps

	// Recompute install step index after filtering.
	stepGroupIdx = -1
	for i, stepGroup := range rootTemplate.Steps {
		for _, step := range stepGroup.Steps {
			if step.Name == "install-application" {
				stepGroupIdx = i
				break
			}
		}
		if stepGroupIdx >= 0 {
			break
		}
	}
	if stepGroupIdx < 0 {
		return nil
	}

	newStepGroup := v1alpha1.ParallelSteps{
		Steps: []v1alpha1.WorkflowStep{{
			Name:     rbacStepName,
			Template: rbacStepName,
		}},
	}

	newSteps := make([]v1alpha1.ParallelSteps, 0, len(rootTemplate.Steps)+1)
	for i, stepGroup := range rootTemplate.Steps {
		if i == stepGroupIdx {
			newSteps = append(newSteps, newStepGroup)
		}
		newSteps = append(newSteps, stepGroup)
	}
	rootTemplate.Steps = newSteps

	logrus.WithFields(logrus.Fields{
		"entrypoint":         entrypoint,
		"install_step_index": stepGroupIdx,
	}).Info("[RBAC Patch] Successfully injected workload-discovery RBAC step")

	return nil
}

func (c *chaosExperimentService) processCronExperimentManifest(ctx context.Context, workflow *model.ChaosExperimentRequest, weights map[string]int, revID, projectID string) error {
	var (
		newWeights             []*model.WeightagesInput
		cronExperimentManifest v1alpha1.CronWorkflow
	)

	err := json.Unmarshal([]byte(workflow.ExperimentManifest), &cronExperimentManifest)
	if err != nil {
		return errors.New("failed to unmarshal workflow manifest")
	}

	applyInstallAgentTemplateOverrides(cronExperimentManifest.Spec.WorkflowSpec.Templates)
	applyInstallApplicationTemplateOverrides(cronExperimentManifest.Spec.WorkflowSpec.Templates)
	injectExperimentContextArgs(cronExperimentManifest.Spec.WorkflowSpec.Templates)

	if strings.TrimSpace(cronExperimentManifest.Spec.Schedule) == "" {
		return errors.New("failed to process cron workflow, cron syntax not provided in manifest")
	}

	if cronExperimentManifest.Labels == nil {
		cronExperimentManifest.Labels = map[string]string{
			"workflow_id":      *workflow.ExperimentID,
			"infra_id":         workflow.InfraID,
			"workflows.argoproj.io/controller-instanceid": workflow.InfraID,
			"revision_id":      revID,
			"experiment_name":  workflow.ExperimentName,
		}
	} else {
		cronExperimentManifest.Labels["workflow_id"] = *workflow.ExperimentID
		cronExperimentManifest.Labels["infra_id"] = workflow.InfraID
		cronExperimentManifest.Labels["workflows.argoproj.io/controller-instanceid"] = workflow.InfraID
		cronExperimentManifest.Labels["revision_id"] = revID
		cronExperimentManifest.Labels["experiment_name"] = workflow.ExperimentName
	}

	if cronExperimentManifest.Spec.WorkflowMetadata == nil {
		cronExperimentManifest.Spec.WorkflowMetadata = &v1.ObjectMeta{
			Labels: map[string]string{
				"workflow_id":      *workflow.ExperimentID,
				"infra_id":         workflow.InfraID,
				"workflows.argoproj.io/controller-instanceid": workflow.InfraID,
				"revision_id":      revID,
				"experiment_name":  workflow.ExperimentName,
			},
		}
	} else {
		if cronExperimentManifest.Spec.WorkflowMetadata.Labels == nil {
			cronExperimentManifest.Spec.WorkflowMetadata.Labels = map[string]string{
				"workflow_id":      *workflow.ExperimentID,
				"infra_id":         workflow.InfraID,
				"workflows.argoproj.io/controller-instanceid": workflow.InfraID,
				"revision_id":      revID,
				"experiment_name":  workflow.ExperimentName,
			}
		} else {
			cronExperimentManifest.Spec.WorkflowMetadata.Labels["workflow_id"] = *workflow.ExperimentID
			cronExperimentManifest.Spec.WorkflowMetadata.Labels["infra_id"] = workflow.InfraID
			cronExperimentManifest.Spec.WorkflowMetadata.Labels["workflows.argoproj.io/controller-instanceid"] = workflow.InfraID
			cronExperimentManifest.Spec.WorkflowMetadata.Labels["revision_id"] = revID
			cronExperimentManifest.Spec.WorkflowMetadata.Labels["experiment_name"] = workflow.ExperimentName
		}
	}

	for i, template := range cronExperimentManifest.Spec.WorkflowSpec.Templates {

		artifact := template.Inputs.Artifacts
		if len(artifact) > 0 {
			if artifact[0].Raw == nil {
				continue
			}
			var data = artifact[0].Raw.Data
			if len(data) > 0 {
				// This replacement is required because chaos engine yaml have a syntax template. example:{{ workflow.parameters.adminModeNamespace }}
				// And it is not able the unmarshal the yamlstring to chaos engine struct
				data = strings.ReplaceAll(data, "{{", "")
				data = strings.ReplaceAll(data, "}}", "")

				var meta chaosTypes.ChaosEngine
				err = yaml.Unmarshal([]byte(data), &meta)
				if err != nil {
					return errors.New("failed to unmarshal chaosengine")
				}

				if strings.ToLower(meta.Kind) == "chaosengine" {
					var exprname string
					if len(meta.Spec.Experiments) > 0 {
						exprname = meta.GenerateName
						if len(exprname) == 0 {
							return errors.New("empty chaos experiment name")
						}
					} else {
						return errors.New("no experiments specified in chaosengine - " + meta.Name)
					}
					// Check if probeRef annotation is present in chaosengine, if not then create new probes
					if _, ok := meta.GetObjectMeta().GetAnnotations()["probeRef"]; !ok {
						// Check if probes are specified in chaosengine
						if meta.Spec.Experiments[0].Spec.Probe != nil {
							type probeRef struct {
								Name string `json:"name"`
								Mode string `json:"mode"`
							}
							probeRefs := []probeRef{}
							for _, p := range meta.Spec.Experiments[0].Spec.Probe {
								// Generate new probes for the experiment
								probe, err := probeUtils.ProbeInputsToProbeRequestConverter(p)
								if err != nil {
									return err
								}
								result, err := c.probeService.AddProbe(ctx, probe, projectID)
								if err != nil {
									return err
								}
								// If probes are created then update the probeRef annotation in chaosengine
								probeRefs = append(probeRefs, probeRef{
									Name: result.Name,
									Mode: p.Mode,
								})
							}
							probeRefBytes, _ := json.Marshal(probeRefs)
							rawYaml, err := probeUtils.InsertProbeRefAnnotation(artifact[0].Raw.Data, string(probeRefBytes))
							if err != nil {
								return err
							}
							artifact[0].Raw.Data = rawYaml
						} else {
							return errors.New("no probes specified in chaosengine - " + meta.Name)
						}
					}
					if val, ok := weights[exprname]; ok {
						cronExperimentManifest.Spec.WorkflowSpec.Templates[i].Metadata.Labels = map[string]string{
							"weight": strconv.Itoa(val),
						}
					} else if val, ok := cronExperimentManifest.Spec.WorkflowSpec.Templates[i].Metadata.Labels["weight"]; ok {
						intVal, err := strconv.Atoi(val)
						if err != nil {
							return errors.New("failed to convert")
						}

						newWeights = append(newWeights, &model.WeightagesInput{
							FaultName: exprname,
							Weightage: intVal,
						})
					} else {
						newWeights = append(newWeights, &model.WeightagesInput{
							FaultName: exprname,
							Weightage: 10,
						})
						cronExperimentManifest.Spec.WorkflowSpec.Templates[i].Metadata.Labels = map[string]string{
							"weight": "10",
						}
					}
				}
			}
		}
	}

	workflow.Weightages = append(workflow.Weightages, newWeights...)
	out, err := json.Marshal(cronExperimentManifest)
	if err != nil {
		return err
	}
	workflow.ExperimentManifest = string(out)
	workflow.CronSyntax = cronExperimentManifest.Spec.Schedule
	return nil
}

func (c *chaosExperimentService) processChaosEngineManifest(ctx context.Context, workflow *model.ChaosExperimentRequest, weights map[string]int, revID, projectID string) error {
	var (
		newWeights       []*model.WeightagesInput
		workflowManifest chaosTypes.ChaosEngine
	)

	err := json.Unmarshal([]byte(workflow.ExperimentManifest), &workflowManifest)
	if err != nil {
		return errors.New("failed to unmarshal workflow manifest")
	}

	if workflowManifest.Labels == nil {
		workflowManifest.Labels = map[string]string{
			"workflow_id": *workflow.ExperimentID,
			"infra_id":    workflow.InfraID,
			"type":        "standalone_workflow",
			"revision_id": revID,
		}
	} else {
		workflowManifest.Labels["workflow_id"] = *workflow.ExperimentID
		workflowManifest.Labels["infra_id"] = workflow.InfraID
		workflowManifest.Labels["type"] = "standalone_workflow"
		workflowManifest.Labels["revision_id"] = revID
	}

	if len(workflowManifest.Spec.Experiments) == 0 {
		return errors.New("no experiments specified in chaosengine - " + workflowManifest.Name)
	}
	exprName := workflowManifest.Spec.Experiments[0].Name
	if len(exprName) == 0 {
		return errors.New("empty chaos experiment name")
	}
	// Check if probeRef annotation is present in chaosengine, if not then create new probes
	if _, ok := workflowManifest.GetObjectMeta().GetAnnotations()["probeRef"]; !ok {
		// Check if probes are specified in chaosengine
		if workflowManifest.Spec.Experiments[0].Spec.Probe != nil {
			type probeRef struct {
				Name string `json:"name"`
				Mode string `json:"mode"`
			}
			probeRefs := []probeRef{}
			for _, p := range workflowManifest.Spec.Experiments[0].Spec.Probe {
				// Generate new probes for the experiment
				probe, err := probeUtils.ProbeInputsToProbeRequestConverter(p)
				if err != nil {
					return err
				}
				result, err := c.probeService.AddProbe(ctx, probe, projectID)
				if err != nil {
					return err
				}
				// If probes are created then update the probeRef annotation in chaosengine
				probeRefs = append(probeRefs, probeRef{
					Name: result.Name,
					Mode: p.Mode,
				})
			}
			probeRefBytes, _ := json.Marshal(probeRefs)
			if workflowManifest.GetObjectMeta().GetAnnotations() == nil {
				workflowManifest.GetObjectMeta().SetAnnotations(map[string]string{})
			}
			workflowManifest.GetObjectMeta().GetAnnotations()["probeRef"] = string(probeRefBytes)
		} else {
			return errors.New("no probes specified in chaosengine - " + workflowManifest.Name)
		}
	}

	if val, ok := weights[exprName]; ok {
		workflowManifest.Labels["weight"] = strconv.Itoa(val)
	} else if val, ok := workflowManifest.Labels["weight"]; ok {
		intVal, err := strconv.Atoi(val)
		if err != nil {
			return errors.New("failed to convert")
		}
		newWeights = append(newWeights, &model.WeightagesInput{
			FaultName: exprName,
			Weightage: intVal,
		})
	} else {
		newWeights = append(newWeights, &model.WeightagesInput{
			FaultName: exprName,
			Weightage: 10,
		})
		workflowManifest.Labels["weight"] = "10"
	}
	workflow.Weightages = append(workflow.Weightages, newWeights...)
	out, err := json.Marshal(workflowManifest)
	if err != nil {
		return err
	}

	workflow.ExperimentManifest = string(out)
	return nil
}

func (c *chaosExperimentService) processChaosScheduleManifest(ctx context.Context, workflow *model.ChaosExperimentRequest, weights map[string]int, revID, projectID string) error {
	var (
		newWeights       []*model.WeightagesInput
		workflowManifest scheduleTypes.ChaosSchedule
	)
	err := json.Unmarshal([]byte(workflow.ExperimentManifest), &workflowManifest)
	if err != nil {
		return errors.New("failed to unmarshal workflow manifest")
	}

	if workflowManifest.Labels == nil {
		workflowManifest.Labels = map[string]string{
			"workflow_id": *workflow.ExperimentID,
			"infra_id":    workflow.InfraID,
			"type":        "standalone_workflow",
			"revision_id": revID,
		}
	} else {
		workflowManifest.Labels["workflow_id"] = *workflow.ExperimentID
		workflowManifest.Labels["infra_id"] = workflow.InfraID
		workflowManifest.Labels["type"] = "standalone_workflow"
		workflowManifest.Labels["revision_id"] = revID
	}
	if len(workflowManifest.Spec.EngineTemplateSpec.Experiments) == 0 {
		return errors.New("no experiments specified in chaos engine - " + workflowManifest.Name)
	}

	exprName := workflowManifest.Spec.EngineTemplateSpec.Experiments[0].Name
	if len(exprName) == 0 {
		return errors.New("empty chaos experiment name")
	}
	// Check if probeRef annotation is present in chaosengine, if not then create new probes
	if _, ok := workflowManifest.GetObjectMeta().GetAnnotations()["probeRef"]; !ok {
		// Check if probes are specified in chaosengine
		if workflowManifest.Spec.EngineTemplateSpec.Experiments[0].Spec.Probe != nil {
			type probeRef struct {
				Name string `json:"name"`
				Mode string `json:"mode"`
			}
			probeRefs := []probeRef{}
			for _, p := range workflowManifest.Spec.EngineTemplateSpec.Experiments[0].Spec.Probe {
				// Generate new probes for the experiment
				probe, err := probeUtils.ProbeInputsToProbeRequestConverter(p)
				if err != nil {
					return err
				}
				result, err := c.probeService.AddProbe(ctx, probe, projectID)
				if err != nil {
					return err
				}
				// If probes are created then update the probeRef annotation in chaosengine
				probeRefs = append(probeRefs, probeRef{
					Name: result.Name,
					Mode: p.Mode,
				})
			}
			probeRefBytes, _ := json.Marshal(probeRefs)
			if workflowManifest.GetObjectMeta().GetAnnotations() == nil {
				workflowManifest.GetObjectMeta().SetAnnotations(map[string]string{})
			}
			workflowManifest.GetObjectMeta().GetAnnotations()["probeRef"] = string(probeRefBytes)
		} else {
			return errors.New("no probes specified in chaosengine - " + workflowManifest.Name)
		}
	}

	if val, ok := weights[exprName]; ok {
		workflowManifest.Labels["weight"] = strconv.Itoa(val)
	} else if val, ok := workflowManifest.Labels["weight"]; ok {
		intVal, err := strconv.Atoi(val)
		if err != nil {
			return errors.New("failed to convert")
		}
		newWeights = append(newWeights, &model.WeightagesInput{
			FaultName: exprName,
			Weightage: intVal,
		})
	} else {
		newWeights = append(newWeights, &model.WeightagesInput{
			FaultName: exprName,
			Weightage: 10,
		})
		workflowManifest.Labels["weight"] = "10"
	}
	workflow.Weightages = append(workflow.Weightages, newWeights...)
	out, err := json.Marshal(workflowManifest)
	if err != nil {
		return err
	}

	workflow.ExperimentManifest = string(out)
	return nil
}

func (c *chaosExperimentService) UpdateRuntimeCronWorkflowConfiguration(cronWorkflowManifest v1alpha1.CronWorkflow, experiment dbChaosExperiment.ChaosExperimentRequest) (v1alpha1.CronWorkflow, []string, error) {
	var (
		faults []string
	)
	for i, template := range cronWorkflowManifest.Spec.WorkflowSpec.Templates {
		artifact := template.Inputs.Artifacts
		if len(artifact) > 0 {
			if artifact[0].Raw == nil {
				continue
			}
			data := artifact[0].Raw.Data
			if len(data) > 0 {
				var meta chaosTypes.ChaosEngine
				annotation := make(map[string]string)
				err := yaml.Unmarshal([]byte(data), &meta)
				if err != nil {
					return cronWorkflowManifest, faults, errors.New("failed to unmarshal chaosengine")
				}
				if strings.ToLower(meta.Kind) == "chaosengine" {
					faults = append(faults, meta.GenerateName)
					if meta.Annotations != nil {
						annotation = meta.Annotations
					}

					var annotationArray []string
					for _, key := range annotation {

						var manifestAnnotation []dbChaosExperiment.ProbeAnnotations
						err := json.Unmarshal([]byte(key), &manifestAnnotation)
						if err != nil {
							return cronWorkflowManifest, faults, errors.New("failed to unmarshal experiment annotation object")
						}
						for _, annotationKey := range manifestAnnotation {
							annotationArray = append(annotationArray, annotationKey.Name)
						}
					}

					meta.Annotations = annotation

					if meta.Labels == nil {
						meta.Labels = map[string]string{
							"infra_id":        experiment.InfraID,
							"step_pod_name":   "{{pod.name}}",
							"workflow_run_id": "{{workflow.uid}}",
						}
					} else {
						meta.Labels["infra_id"] = experiment.InfraID
						meta.Labels["step_pod_name"] = "{{pod.name}}"
						meta.Labels["workflow_run_id"] = "{{workflow.uid}}"
					}

					if len(meta.Spec.Experiments[0].Spec.Probe) != 0 {
						meta.Spec.Experiments[0].Spec.Probe = utils.TransformProbe(meta.Spec.Experiments[0].Spec.Probe)
					}
					res, err := yaml.Marshal(&meta)
					if err != nil {
						return cronWorkflowManifest, faults, errors.New("failed to marshal chaosengine")
					}
					cronWorkflowManifest.Spec.WorkflowSpec.Templates[i].Inputs.Artifacts[0].Raw.Data = string(res)
				}
			}
		}
	}
	return cronWorkflowManifest, faults, nil
}

// applyInstallApplicationReadinessPatch automatically adds a normalization step after install-application.
// It waits for pods to reach Running/Succeeded with a bounded timeout to avoid deadlocking the workflow.
func (c *chaosExperimentService) applyInstallApplicationReadinessPatch(wf *v1alpha1.Workflow) error {
	if wf == nil || len(wf.Spec.Templates) == 0 {
		return nil
	}

	// Find the entrypoint template
	entrypoint := wf.Spec.Entrypoint
	if entrypoint == "" {
		return nil
	}

	var rootTemplate *v1alpha1.Template
	for i, t := range wf.Spec.Templates {
		if t.Name == entrypoint {
			rootTemplate = &wf.Spec.Templates[i]
			break
		}
	}

	if rootTemplate == nil || len(rootTemplate.Steps) == 0 {
		return nil
	}

	// Look for install-application step and insert readiness check after it
	stepGroupIdx := -1
	for i, stepGroup := range rootTemplate.Steps {
		for _, step := range stepGroup.Steps {
			if step.Name == "install-application" {
				stepGroupIdx = i
				break
			}
		}
		if stepGroupIdx >= 0 {
			break
		}
	}

	if stepGroupIdx < 0 {
		// No install-application step found, nothing to patch
		return nil
	}

	// Check if readiness check already exists
	readinessStepName := "normalize-install-application-readiness"
	for _, t := range wf.Spec.Templates {
		if t.Name == readinessStepName {
			// Already patched
			return nil
		}
	}

	// Create readiness check template with pod readiness wait logic
	readinessTpl := v1alpha1.Template{
		Name: readinessStepName,
		Container: &corev1.Container{
			Image: "litmuschaos/k8s:latest",
			Command: []string{"sh", "-c"},
			Args: []string{
				`set -eu
NS="{{workflow.parameters.appNamespace}}"
DEADLINE=$(($(date +%s) + ${READINESS_WAIT_SECONDS:-600}))
echo "[readiness-check] Waiting for all pods in namespace $NS to reach Running/Succeeded..."
while [ "$(date +%s)" -lt "$DEADLINE" ]; do
	if ! kubectl get ns "$NS" >/dev/null 2>&1; then
		echo "[readiness-check] Namespace $NS not found yet; retrying..."
		sleep 5
		continue
	fi

	TOTAL=$(kubectl get pods -n "$NS" --no-headers 2>/dev/null | wc -l | tr -d ' ')
	if [ -z "$TOTAL" ] || [ "$TOTAL" -eq 0 ]; then
		echo "[readiness-check] No pods created yet in $NS; retrying..."
		sleep 5
		continue
	fi

	NOT_READY=$(kubectl get pods -n "$NS" --no-headers 2>/dev/null | awk '$3!="Running" && $3!="Succeeded" && $3!="Completed" {c++} END {print c+0}')
	RUNNING=$(kubectl get pods -n "$NS" --no-headers 2>/dev/null | awk '$3=="Running" || $3=="Succeeded" || $3=="Completed" {c++} END {print c+0}')

	echo "[readiness-check] Pods status: $RUNNING/$TOTAL in Running/Succeeded"
	if [ "$NOT_READY" -eq 0 ]; then
		echo "[readiness-check] ✓ All pods in namespace $NS are Running/Succeeded"
		exit 0
	fi

	kubectl get pods -n "$NS" --no-headers 2>/dev/null | awk '$3!="Running" && $3!="Succeeded" && $3!="Completed" {print "[readiness-check] waiting:", $1, $3, $2}' | head -n 6 || true
  sleep 5
done
echo "[readiness-check] ⚠ Timeout waiting for all pods; continuing workflow to avoid deadlock"
exit 0`,
			},
		},
	}

	// Add the template to workflow
	wf.Spec.Templates = append(wf.Spec.Templates, readinessTpl)

	// Insert a new step group after install-application with the readiness check
	newStepGroup := v1alpha1.ParallelSteps{
		Steps: []v1alpha1.WorkflowStep{{
			Name:     readinessStepName,
			Template: readinessStepName,
		}},
	}

	// Insert the new step group after install-application
	newSteps := make([]v1alpha1.ParallelSteps, 0, len(rootTemplate.Steps)+1)
	for i, stepGroup := range rootTemplate.Steps {
		newSteps = append(newSteps, stepGroup)
		if i == stepGroupIdx {
			newSteps = append(newSteps, newStepGroup)
		}
	}
	rootTemplate.Steps = newSteps

	logrus.WithFields(logrus.Fields{
		"entrypoint":         entrypoint,
		"install_step_index": stepGroupIdx,
	}).Info("[Readiness Patch] Successfully applied install-application readiness normalization")

	return nil
}

// applyPreCleanupWaitPatch inserts a dynamic sleep step before cleanup/delete phases.
// Wait duration is controlled through PRE_CLEANUP_WAIT_SECONDS (via env/config).
func (c *chaosExperimentService) applyPreCleanupWaitPatch(wf *v1alpha1.Workflow) error {
	if wf == nil || len(wf.Spec.Templates) == 0 {
		return nil
	}

	waitRaw := strings.TrimSpace(utils.Config.PreCleanupWaitSeconds)
	if waitRaw == "" {
		waitRaw = strings.TrimSpace(os.Getenv("PRE_CLEANUP_WAIT_SECONDS"))
	}
	if waitRaw == "" {
		waitRaw = "0"
	}

	waitSec, err := strconv.Atoi(waitRaw)
	if err != nil || waitSec < 0 {
		waitSec = 0
	}

	entrypoint := wf.Spec.Entrypoint
	if entrypoint == "" {
		return nil
	}

	var rootTemplate *v1alpha1.Template
	for i := range wf.Spec.Templates {
		if wf.Spec.Templates[i].Name == entrypoint {
			rootTemplate = &wf.Spec.Templates[i]
			break
		}
	}
	if rootTemplate == nil || len(rootTemplate.Steps) == 0 {
		return nil
	}

	waitTemplateName := "dynamic-pre-cleanup-wait"

	for _, t := range wf.Spec.Templates {
		if t.Name == waitTemplateName {
			return nil
		}
	}

	insertIdx := -1
	for i, group := range rootTemplate.Steps {
		for _, step := range group.Steps {
			name := strings.ToLower(strings.TrimSpace(step.Name))
			if name == "cleanup-chaos-resources" || strings.Contains(name, "cleanup-chaos-resources") {
				insertIdx = i
				break
			}
		}
		if insertIdx >= 0 {
			break
		}
	}

	if insertIdx < 0 {
		for i, group := range rootTemplate.Steps {
			for _, step := range group.Steps {
				name := strings.ToLower(strings.TrimSpace(step.Name))
				if strings.HasPrefix(name, "cleanup-") || (strings.Contains(name, "cleanup") && strings.Contains(name, "resource")) {
					insertIdx = i
					break
				}
			}
			if insertIdx >= 0 {
				break
			}
		}
	}

	if insertIdx < 0 {
		insertIdx = len(rootTemplate.Steps) - 1
		if insertIdx < 0 {
			return nil
		}
	}

	waitTpl := v1alpha1.Template{
		Name: waitTemplateName,
		Container: &corev1.Container{
			Image:   "busybox:1.36",
			Command: []string{"sh", "-c"},
			Args: []string{fmt.Sprintf("echo '[pre-cleanup-wait] sleeping for %d seconds'; sleep %d; echo '[pre-cleanup-wait] done'", waitSec, waitSec)},
		},
	}
	wf.Spec.Templates = append(wf.Spec.Templates, waitTpl)

	waitStepGroup := v1alpha1.ParallelSteps{
		Steps: []v1alpha1.WorkflowStep{{
			Name:     waitTemplateName,
			Template: waitTemplateName,
		}},
	}

	newSteps := make([]v1alpha1.ParallelSteps, 0, len(rootTemplate.Steps)+1)
	for i, group := range rootTemplate.Steps {
		if i == insertIdx {
			newSteps = append(newSteps, waitStepGroup)
		}
		newSteps = append(newSteps, group)
	}
	rootTemplate.Steps = newSteps

	logrus.WithFields(logrus.Fields{
		"entrypoint":          entrypoint,
		"wait_seconds":        waitSec,
		"insert_before_index": insertIdx,
	}).Info("[Pre-Cleanup Wait Patch] Injected dynamic pre-cleanup wait step")

	return nil
}

// applyUninstallAllPatch appends a final uninstall-all step to the workflow that
// runs helm uninstall for both the agent release and the app release after all
// chaos and cleanup steps have completed.
//
// Release names are derived fully from Argo workflow parameters at runtime:
//   - agent release  = {{workflow.parameters.agentFolder}}
//   - app release    = {{workflow.parameters.appNamespace}}  (folder == release == namespace by convention)
//   - namespace      = {{workflow.parameters.appNamespace}}
//
// The step only runs if an install-agent template is present in the workflow
// (i.e. this is a workflow that actually deployed an agent).
// The install-agent image (which ships with helm) is reused so no extra image pull is needed.
func (c *chaosExperimentService) applyUninstallAllPatch(wf *v1alpha1.Workflow) error {
	if wf == nil || len(wf.Spec.Templates) == 0 {
		return nil
	}

	// Only inject if an install-agent step exists in the workflow.
	hasInstallAgent := false
	for _, t := range wf.Spec.Templates {
		if t.Container == nil {
			continue
		}
		if t.Name == "install-agent" || strings.Contains(strings.TrimSpace(t.Container.Image), "agentcert-install-agent") {
			hasInstallAgent = true
			break
		}
	}
	if !hasInstallAgent {
		return nil
	}

	entrypoint := wf.Spec.Entrypoint
	if entrypoint == "" {
		return nil
	}

	var rootTemplate *v1alpha1.Template
	for i := range wf.Spec.Templates {
		if wf.Spec.Templates[i].Name == entrypoint {
			rootTemplate = &wf.Spec.Templates[i]
			break
		}
	}
	if rootTemplate == nil || len(rootTemplate.Steps) == 0 {
		return nil
	}

	uninstallTemplateName := "uninstall-all"

	// Idempotent: skip if already present.
	for _, t := range wf.Spec.Templates {
		if t.Name == uninstallTemplateName {
			return nil
		}
	}

	// Resolve install-agent image (same image used for install, guaranteed to have helm).
	uninstallImage := strings.TrimSpace(utils.Config.InstallAgentImage)
	if uninstallImage == "" {
		uninstallImage = strings.TrimSpace(os.Getenv("INSTALL_AGENT_IMAGE"))
	}
	if uninstallImage == "" {
		uninstallImage = "agentcert/agentcert-install-agent:latest"
	}

	// Resolve agent install namespace (where the agent helm release actually lives).
	// When AGENT_INSTALL_NAMESPACE is set on the graphql server, the agent is
	// installed in that system ns and only OBSERVES the app namespace.  Uninstall
	// must target the install ns; we still try the app ns as a fallback for
	// back-compat with legacy releases that landed in the app ns.
	agentInstallNs := strings.TrimSpace(os.Getenv("AGENT_INSTALL_NAMESPACE"))
	uninstallScript := fmt.Sprintf(`NAMESPACE="{{workflow.parameters.appNamespace}}"
AGENT_INSTALL_NS=%q
AGENT_RELEASE="{{workflow.parameters.agentFolder}}"
APP_RELEASE="${NAMESPACE}"
if [ -z "${AGENT_INSTALL_NS}" ]; then AGENT_INSTALL_NS="${NAMESPACE}"; fi
echo "[uninstall-all] Cleaning ChaosEngine and ChaosResult resources in ${NAMESPACE}"
kubectl delete chaosengines.litmuschaos.io --all -n "${NAMESPACE}" --ignore-not-found 2>&1 || true
kubectl delete chaosresults.litmuschaos.io --all -n "${NAMESPACE}" --ignore-not-found 2>&1 || true
echo "[uninstall-all] Uninstalling agent release: ${AGENT_RELEASE} (ns: ${AGENT_INSTALL_NS})"
helm uninstall "${AGENT_RELEASE}" -n "${AGENT_INSTALL_NS}" --ignore-not-found 2>&1 || true
if [ "${AGENT_INSTALL_NS}" != "${NAMESPACE}" ]; then
  echo "[uninstall-all] Fallback agent uninstall in app ns: ${NAMESPACE}"
  helm uninstall "${AGENT_RELEASE}" -n "${NAMESPACE}" --ignore-not-found 2>&1 || true
  # Drop the agent install namespace itself so the next experiment starts clean.
  # We only do this when it's distinct from the app ns AND not a system ns
  # (kube-*, default, litmus, monitoring) to avoid catastrophic deletes.
  case "${AGENT_INSTALL_NS}" in
    kube-*|default|litmus|monitoring|argo|ingress-*|cert-manager) ;;
    *)
      echo "[uninstall-all] Deleting agent install namespace: ${AGENT_INSTALL_NS}"
      kubectl delete namespace "${AGENT_INSTALL_NS}" --ignore-not-found --wait=false 2>&1 || true
      ;;
  esac
fi
echo "[uninstall-all] Uninstalling app release: ${APP_RELEASE} (ns: ${NAMESPACE})"
helm uninstall "${APP_RELEASE}" -n "${NAMESPACE}" --ignore-not-found 2>&1 || true
echo "[uninstall-all] Done"`, agentInstallNs)

	uninstallTpl := v1alpha1.Template{
		Name: uninstallTemplateName,
		Container: &corev1.Container{
			Image:   uninstallImage,
			Command: []string{"sh", "-c"},
			Args:    []string{uninstallScript},
		},
	}
	wf.Spec.Templates = append(wf.Spec.Templates, uninstallTpl)

	uninstallStepGroup := v1alpha1.ParallelSteps{
		Steps: []v1alpha1.WorkflowStep{{
			Name:     uninstallTemplateName,
			Template: uninstallTemplateName,
		}},
	}

	// Append as the very last step group.
	rootTemplate.Steps = append(rootTemplate.Steps, uninstallStepGroup)

	logrus.WithFields(logrus.Fields{
		"entrypoint": entrypoint,
		"image":      uninstallImage,
	}).Info("[Uninstall All Patch] Appended dynamic uninstall-all step")

	return nil
}

// applyInstallAgentTemplateOverrides enforces a configurable install-agent image
// and pull policy for all template-based workflow manifests.
//
// Phase 1 (metadata-driven with fallback):
//   - Try reading agent metadata from chartserviceversion.yaml
//   - If metadata exists -> match templates by installTemplateName / installImage
//   - If metadata missing (CSV not synced, parse error) -> fall back to hardcoded matching
func applyInstallAgentTemplateOverrides(templates []v1alpha1.Template) {
	targetImage := strings.TrimSpace(utils.Config.InstallAgentImage)
	if targetImage == "" {
		targetImage = strings.TrimSpace(os.Getenv("INSTALL_AGENT_IMAGE"))
	}
	if targetImage == "" {
		targetImage = "agentcert/agentcert-install-agent:latest"
	}

	targetPullPolicy := strings.TrimSpace(utils.Config.InstallAgentImagePullPolicy)
	if targetPullPolicy == "" {
		targetPullPolicy = strings.TrimSpace(os.Getenv("INSTALL_AGENT_IMAGE_PULL_POLICY"))
	}
	if targetPullPolicy == "" {
		targetPullPolicy = string(corev1.PullAlways)
	}

	// Guard against invalid values and keep behavior deterministic.
	switch corev1.PullPolicy(targetPullPolicy) {
	case corev1.PullAlways, corev1.PullIfNotPresent, corev1.PullNever:
	default:
		targetPullPolicy = string(corev1.PullAlways)
	}

	// Try metadata-driven path
	agentEntries := agenthub.GetAgentInjectionMetadata()
	if len(agentEntries) > 0 {
		applyInstallAgentTemplateOverridesFromMetadata(templates, agentEntries, targetImage, targetPullPolicy)
		return
	}

	// Fallback: hardcoded matching (will be removed in Phase 2)
	logrus.Warn("[Install Agent Patch] CSV metadata not available - using hardcoded fallback")
	applyInstallAgentTemplateOverridesFallback(templates, targetImage, targetPullPolicy)
}

// applyInstallAgentTemplateOverridesFromMetadata uses agent CSV metadata to match
// and override install templates. Each agent entry declares its own installTemplateName
// and installImage.
func applyInstallAgentTemplateOverridesFromMetadata(templates []v1alpha1.Template, agents []agenthub.AgentEntry, envImage, envPullPolicy string) {
	changed := false
	for i := range templates {
		t := &templates[i]
		if t.Container == nil {
			continue
		}

		// Find an agent entry that matches this template
		var matched *agenthub.AgentEntry
		for j := range agents {
			a := &agents[j]
			if a.InstallTemplateName != "" && t.Name == a.InstallTemplateName {
				matched = a
				break
			}
			if a.InstallImage != "" && strings.Contains(strings.TrimSpace(t.Container.Image), a.InstallImage) {
				matched = a
				break
			}
		}
		if matched == nil {
			continue
		}

		// Determine target image: env override > CSV metadata > current image (no-op)
		targetImage := envImage
		if targetImage == "" {
			targetImage = matched.InstallImage
		}
		if targetImage == "" {
			continue
		}

		if strings.TrimSpace(t.Container.Image) != targetImage {
			t.Container.Image = targetImage
			changed = true
		}
		if t.Container.ImagePullPolicy != corev1.PullPolicy(envPullPolicy) {
			t.Container.ImagePullPolicy = corev1.PullPolicy(envPullPolicy)
			changed = true
		}

		// Inject install-type annotation so handler.go can match by annotation
		// instead of by name (Item #2 — annotation-based matching).
		if t.Metadata.Annotations == nil {
			t.Metadata.Annotations = make(map[string]string)
		}
		if _, exists := t.Metadata.Annotations["agentcert.io/install-type"]; !exists {
			t.Metadata.Annotations["agentcert.io/install-type"] = "agent"
			changed = true
		}
	}

	if changed {
		logrus.WithField("source", "csv-metadata").Info("[Install Agent Patch] Applied install-agent image override from CSV metadata")
	}
}

// applyInstallAgentTemplateOverridesFallback is the original hardcoded matching logic.
// Retained for backward compatibility when CSV metadata is not yet available.
// TODO(phase2): Remove this function once CSV metadata is confirmed stable in production.
func applyInstallAgentTemplateOverridesFallback(templates []v1alpha1.Template, envImage, envPullPolicy string) {
	targetImage := envImage
	if targetImage == "" {
		targetImage = "agentcert/agentcert-install-agent:latest"
	}

	changed := false
	for i := range templates {
		t := &templates[i]
		if t.Container == nil {
			continue
		}

		isInstallAgentTemplate := t.Name == "install-agent" || strings.Contains(strings.TrimSpace(t.Container.Image), "agentcert-install-agent")
		if !isInstallAgentTemplate {
			continue
		}

		if strings.TrimSpace(t.Container.Image) != targetImage {
			t.Container.Image = targetImage
			changed = true
		}

		if t.Container.ImagePullPolicy != corev1.PullPolicy(envPullPolicy) {
			t.Container.ImagePullPolicy = corev1.PullPolicy(envPullPolicy)
			changed = true
		}
	}

	if changed {
		logrus.WithFields(logrus.Fields{
			"image":       targetImage,
			"pull_policy": envPullPolicy,
			"source":      "hardcoded-fallback",
		}).Info("[Install Agent Patch] Applied dynamic install-agent image override (fallback)")
	}
}

// applyInstallApplicationTemplateOverrides sets a safe imagePullPolicy on any
// install-application template. The image itself is intentionally NOT overridden
// here because install-application is app-specific: different experiments may use
// different installer images (sock-shop, boutique, etc.). Only the pull policy is
// normalised so that locally-loaded minikube images are used instead of always
// pulling from a remote registry.
//
// If INSTALL_APPLICATION_IMAGE_PULL_POLICY is set in the environment it is used;
// otherwise IfNotPresent is applied as a safe default.
func applyInstallApplicationTemplateOverrides(templates []v1alpha1.Template) {
	targetPullPolicy := strings.TrimSpace(utils.Config.InstallApplicationImagePullPolicy)
	if targetPullPolicy == "" {
		targetPullPolicy = string(corev1.PullAlways)
	}

	forcedSetArgs := []string{
		// MCP servers are now installed cluster-wide in the litmus-exp namespace
		// (chaoscenter graphql/server/manifests/namespace/4b_mcp_tools_deployment.yaml)
		// so the per-app sock-shop chart must NOT spawn duplicates that conflict on
		// service account / node-port and end up CrashLoopBackOff.
		"-set=mcpTools.prometheusMcpServer.enabled=false",
		"-set=mcpTools.kubernetesMcpServer.enabled=false",
	}

	switch corev1.PullPolicy(targetPullPolicy) {
	case corev1.PullAlways, corev1.PullIfNotPresent, corev1.PullNever:
	default:
		targetPullPolicy = string(corev1.PullAlways)
	}

	changed := false
	for i := range templates {
		t := &templates[i]
		if t.Container == nil {
			continue
		}

		isInstallAppTemplate := t.Name == "install-application" || strings.Contains(strings.TrimSpace(t.Container.Image), "agentcert-install-app")
		if !isInstallAppTemplate {
			continue
		}

		if t.Container.ImagePullPolicy != corev1.PullPolicy(targetPullPolicy) {
			t.Container.ImagePullPolicy = corev1.PullPolicy(targetPullPolicy)
			changed = true
		}

		for _, forcedArg := range forcedSetArgs {
			alreadyPresent := false
			for _, existingArg := range t.Container.Args {
				if existingArg == forcedArg {
					alreadyPresent = true
					break
				}
			}
			if !alreadyPresent {
				t.Container.Args = append(t.Container.Args, forcedArg)
				changed = true
			}
		}
	}

	if changed {
		logrus.WithFields(logrus.Fields{
			"pull_policy": targetPullPolicy,
		}).Info("[Install App Patch] Applied install-application imagePullPolicy override")
	}
}

// chaosEngineManifest is a minimal struct used to extract fault names from
// ChaosEngine resource manifests embedded in Argo Workflow templates.
// Uses json tags because github.com/ghodss/yaml converts YAML→JSON before
// unmarshalling, so json struct tags are the effective field mappings.
type chaosEngineManifest struct {
	Kind string `json:"kind"`
	Spec struct {
		Appinfo struct {
			Appns    string `json:"appns"`
			Applabel string `json:"applabel"`
			AppKind  string `json:"appkind"`
		} `json:"appinfo"`
		Experiments []struct {
			Name string `json:"name"`
		} `json:"experiments"`
	} `json:"spec"`
}

// ExtractChaosEngineFaults is the exported wrapper for use by other packages
// (e.g. experiment run handler) that need fault names at run time.
func ExtractChaosEngineFaults(templates []v1alpha1.Template) []string {
	return extractChaosEngineFaults(templates)
}

// LoadFaultGroundTruthsDecoded is the exported wrapper that returns the decoded
// ground truth map (fault name → data) rather than a base64 blob.
// Used by the experiment run handler to emit fault: <name> spans to Langfuse.
func LoadFaultGroundTruthsDecoded(faultNames []string) map[string]interface{} {
	b64 := loadFaultGroundTruths(faultNames)
	if b64 == "" {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil
	}
	return result
}

// extractChaosEngineFaults scans Argo Workflow resource templates for embedded
// ChaosEngine manifests and returns the deduplicated list of fault names from
// spec.experiments[].name. This is generic — it works for any chaos hub category
// (kubernetes, network, application, etc.) and requires no hardcoded fault lists.
func extractChaosEngineFaults(templates []v1alpha1.Template) []string {
	seen := make(map[string]struct{})
	var faults []string

	tryExtract := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		// Strip Argo template syntax before unmarshalling.
		raw = strings.ReplaceAll(raw, "{{", "")
		raw = strings.ReplaceAll(raw, "}}", "")
		var engine chaosEngineManifest
		if err := yaml.Unmarshal([]byte(raw), &engine); err != nil {
			return
		}
		if !strings.EqualFold(engine.Kind, "ChaosEngine") {
			return
		}
		for _, exp := range engine.Spec.Experiments {
			name := strings.TrimSpace(exp.Name)
			if name == "" {
				continue
			}
			if _, exists := seen[name]; !exists {
				seen[name] = struct{}{}
				faults = append(faults, name)
			}
		}
	}

	for _, t := range templates {
		// Path 1: Kubernetes resource template (t.Resource.Manifest)
		if t.Resource != nil && strings.TrimSpace(t.Resource.Manifest) != "" {
			tryExtract(t.Resource.Manifest)
		}
		// Path 2: Argo workflow artifact template (t.Inputs.Artifacts[0].Raw.Data)
		// This is how AgentCert workflows embed ChaosEngine manifests.
		for _, artifact := range t.Inputs.Artifacts {
			if artifact.Raw != nil && strings.TrimSpace(artifact.Raw.Data) != "" {
				tryExtract(artifact.Raw.Data)
			}
		}
	}
	return faults
}

// ExtractChaosEngineFaultDetails returns per-fault target metadata (namespace,
// label, kind) for every ChaosEngine embedded in the Argo Workflow templates.
// The certifier consumes these via the "fault: <name>" Langfuse spans to
// bucket agent activity per fault.  Order matches workflow execution order
// (and lines up with the F1/F2/... blind aliases in the OTEL emission path).
func ExtractChaosEngineFaultDetails(templates []v1alpha1.Template) []observability.FaultDetail {
	seen := make(map[string]struct{})
	var details []observability.FaultDetail

	tryExtract := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		raw = strings.ReplaceAll(raw, "{{", "")
		raw = strings.ReplaceAll(raw, "}}", "")
		var engine chaosEngineManifest
		if err := yaml.Unmarshal([]byte(raw), &engine); err != nil {
			return
		}
		if !strings.EqualFold(engine.Kind, "ChaosEngine") {
			return
		}
		for _, exp := range engine.Spec.Experiments {
			name := strings.TrimSpace(exp.Name)
			if name == "" {
				continue
			}
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			details = append(details, observability.FaultDetail{
				Name:            name,
				TargetNamespace: strings.TrimSpace(engine.Spec.Appinfo.Appns),
				TargetLabel:     strings.TrimSpace(engine.Spec.Appinfo.Applabel),
				TargetKind:      strings.TrimSpace(engine.Spec.Appinfo.AppKind),
			})
		}
	}

	for _, t := range templates {
		if t.Resource != nil && strings.TrimSpace(t.Resource.Manifest) != "" {
			tryExtract(t.Resource.Manifest)
		}
		for _, artifact := range t.Inputs.Artifacts {
			if artifact.Raw != nil && strings.TrimSpace(artifact.Raw.Data) != "" {
				tryExtract(artifact.Raw.Data)
			}
		}
	}
	return details
}

// loadFaultGroundTruths searches the chaos hub filesystem for ground_truth.yaml
// files for each given fault name and returns a base64-encoded compact JSON blob.
//
// Search pattern (generic — no hardcoded hub name or category):
//
//	<hubBase>/<any-hub-name>/faults/<any-category>/<fault>/ground_truth.yaml
//
// hubBase defaults to utils.Config.DefaultChaosHubPath ("/tmp/default/").
// Using wildcards for both the hub name and category dir makes this work for any
// chaos hub (default or custom) and any fault category onboarded in the future.
//
// The result is base64-encoded so the JSON is safe to pass through Helm --set
// without triggering comma/brace/dot escaping issues.
func loadFaultGroundTruths(faultNames []string) string {
	if len(faultNames) == 0 {
		return ""
	}

	hubBase := strings.TrimRight(strings.TrimSpace(utils.Config.DefaultChaosHubPath), "/")
	if hubBase == "" {
		hubBase = "/tmp/default"
	}

	truths := make(map[string]interface{})
	for _, fault := range faultNames {
		if fault == "" {
			continue
		}
		// Wildcard for both hub directory name and category: any onboarded hub
		// or category (kubernetes, network, application, etc.) is found automatically.
		pattern := filepath.Join(hubBase, "*", "faults", "*", fault, "ground_truth.yaml")
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			logrus.WithFields(logrus.Fields{
				"fault":   fault,
				"pattern": pattern,
			}).Debug("[Ground Truth] No ground_truth.yaml found for fault")
			continue
		}
		if len(matches) > 1 {
			logrus.WithField("fault", fault).Warnf(
				"[Ground Truth] Multiple ground_truth.yaml matches found, using first: %v", matches,
			)
		}
		data, err := os.ReadFile(matches[0])
		if err != nil {
			logrus.WithField("fault", fault).Warnf("[Ground Truth] Failed to read file %s: %v", matches[0], err)
			continue
		}
		// ghodss/yaml converts YAML → JSON-compatible Go types (map[string]interface{}).
		var parsed map[string]interface{}
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			logrus.WithField("fault", fault).Warnf("[Ground Truth] Failed to parse YAML: %v", err)
			continue
		}
		// Use the "ground_truth" key if the file has a top-level wrapper; otherwise
		// use the whole document so files with or without the wrapper both work.
		if gt, ok := parsed["ground_truth"]; ok {
			truths[fault] = gt
		} else {
			truths[fault] = parsed
		}
		logrus.WithFields(logrus.Fields{
			"fault": fault,
			"file":  matches[0],
		}).Info("[Ground Truth] Loaded ground truth for fault")
	}

	if len(truths) == 0 {
		return ""
	}

	jsonBytes, err := json.Marshal(truths)
	if err != nil {
		logrus.WithError(err).Warn("[Ground Truth] Failed to marshal ground truths to JSON")
		return ""
	}

	// Base64-encode: output is [A-Za-z0-9+/=] — safe for Helm --set with no escaping.
	return base64.StdEncoding.EncodeToString(jsonBytes)
}

// injectExperimentContextArgs appends --set flags to the install-agent template
// so that Argo Workflow template variables (experiment_id, run_id, workflow_name)
// are passed through to the configured agent Helm chart as ConfigMap values.
//
// Argo resolves {{workflow.labels.workflow_id}}, {{workflow.uid}}, and
// {{workflow.labels.subject}} at runtime before the install-agent container starts.
// These --set values override the empty defaults in values.yaml, flowing through:
//
//	Helm --set -> ConfigMap -> env var -> agent runtime reads os.environ["EXPERIMENT_ID"]
func injectExperimentContextArgs(templates []v1alpha1.Template) {
	// Extract fault names from every ChaosEngine embedded in the workflow and
	// load their ground truth definitions from the chaos hub filesystem.
	// This is generic: any fault or category added to any hub is found automatically.
	faultNames := extractChaosEngineFaults(templates)
	groundTruthB64 := loadFaultGroundTruths(faultNames)

	masterKey := strings.TrimSpace(os.Getenv("LITELLM_MASTER_KEY"))
	if masterKey == "" {
		masterKey = "sk-litellm-local-dev"
	}

	openAIKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if openAIKey == "" {
		openAIKey = masterKey
	}

	openAIBaseURL := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
	if openAIBaseURL == "" {
		openAIBaseURL = "http://litellm-proxy.litellm.svc.cluster.local:4000/v1"
	}

	// Derive sidecar upstream: strip /v1 path suffix so the sidecar can forward raw
	// HTTP requests to the base URL (self.path already contains /v1/chat/completions).
	sidecarUpstream := strings.TrimRight(strings.TrimSuffix(openAIBaseURL, "/v1"), "/")
	if sidecarUpstream == "" {
		sidecarUpstream = "http://litellm-proxy.litellm.svc.cluster.local:4000"
	}

	k8sMCPURL := strings.TrimSpace(os.Getenv("K8S_MCP_URL"))
	if k8sMCPURL == "" {
		k8sMCPURL = "http://kubernetes-mcp-server.litmus-exp.svc.cluster.local:8081/mcp"
	}

	promMCPURL := strings.TrimSpace(os.Getenv("PROM_MCP_URL"))
	if promMCPURL == "" {
		promMCPURL = "http://prometheus-mcp-server.litmus-exp.svc.cluster.local:9090/mcp"
	}

	chaosNamespace := strings.TrimSpace(os.Getenv("CHAOS_NAMESPACE"))
	if chaosNamespace == "" {
		chaosNamespace = "litmus-exp"
	}

	modelAlias := strings.TrimSpace(os.Getenv("MODEL_ALIAS"))
	if modelAlias == "" {
		// Fallback: derive from AZURE_OPENAI_DEPLOYMENT so any provider works
		modelAlias = strings.TrimSpace(os.Getenv("AZURE_OPENAI_DEPLOYMENT"))
	}

	// Sidecar image — split registry/repo:tag so each part is set independently.
	// AGENT_SIDECAR_IMAGE env var format: "registry/repository:tag" or "repository:tag".
	sidecarImageFull := strings.TrimSpace(os.Getenv("AGENT_SIDECAR_IMAGE"))
	if sidecarImageFull == "" {
		sidecarImageFull = utils.Config.AgentSidecarImage
	}
	if sidecarImageFull == "" {
		sidecarImageFull = "agentcert/agent-sidecar:latest"
	}
	sidecarImageTag := "latest"
	sidecarImageRepo := sidecarImageFull
	if idx := strings.LastIndex(sidecarImageFull, ":"); idx > 0 {
		sidecarImageTag = sidecarImageFull[idx+1:]
		sidecarImageRepo = sidecarImageFull[:idx]
	}

	// Server address for agent registration audit call inside install-agent.
	serverAddr := strings.TrimSpace(os.Getenv("SERVER_ADDR"))
	if serverAddr == "" {
		serverAddr = "http://litmusportal-server-service.litmus-chaos.svc.cluster.local:9004/query"
	}
	projectID := strings.TrimSpace(os.Getenv("LITMUS_PROJECT_ID"))
	if projectID == "" {
		projectID = "litmus-project-1"
	}

	experimentSetArgs := []string{
		// Registration: install-agent calls RegisterAgent after helm deploy and
		// injects the returned UUID back via helm upgrade --set agentId=<uuid>.
		"--server-addr", serverAddr,
		"--project-id", projectID,
		// Pass the agentId workflow parameter as a helm value so that AGENT_ID
		// is pre-populated even on the initial pod start (populated on re-runs).
		"--set", "agentId={{workflow.parameters.agentId}}",
		// Also write AGENT_ID into the ConfigMap (agent.config.*) so the sidecar
		// proxy can read it as a mounted file — env vars alone are only set if
		// agentId is non-empty, and Kubernetes doesn't update env vars in a
		// running pod without a restart.
		"--set", "agent.config.AGENT_ID={{workflow.parameters.agentId}}",
		"--set", "agent.config.NOTIFY_ID={{workflow.labels.notify_id}}",
		"--set", "agent.config.EXPERIMENT_ID={{workflow.labels.workflow_id}}",
		"--set", "agent.config.EXPERIMENT_RUN_ID={{workflow.uid}}",
		// WORKFLOW_NAME is the operator-typed experiment name (workflow.Labels
		// ["experiment_name"], set in createChaosWorkflow). It flows into the
		// agent-metadata ConfigMap so the agent-sidecar can mount it at
		// /etc/agent/metadata/WORKFLOW_NAME and set Langfuse trace_name on every
		// LLM call — without this, LiteLLM's Langfuse callback overwrites the
		// trace title to "litellm-acompletion" on the first LLM request.
		//
		// In AgentCert flows the operator chooses experiment names that are
		// distinct from fault names (faults come from the chaos hub), so this
		// does not expose ground truth to the agent. The actual fault identity
		// stays exclusively in the server-side Langfuse fault spans emitted by
		// EmitFaultSpansForTrace and is never written to the agent ConfigMap.
		"--set", "agent.config.WORKFLOW_NAME={{workflow.labels.experiment_name}}",
		// Enforce blind-agent mode at install time so experiment runs do not rely
		// on chart defaults or user-supplied values to hide chaos-specific MCP data.
		"--set", "agent.config.MCP_INCLUDE_CHAOS_TOOLS=false",
		"--set", fmt.Sprintf("agent.config.OPENAI_API_KEY=%s", openAIKey),
		"--set", fmt.Sprintf("agent.secret.LITELLM_MASTER_KEY=%s", masterKey),
		"--set", fmt.Sprintf("agent.config.OPENAI_BASE_URL=%s", openAIBaseURL),
		"--set", fmt.Sprintf("agent.config.MODEL_ALIAS=%s", modelAlias),
		"--set", fmt.Sprintf("agent.config.K8S_MCP_URL=%s", k8sMCPURL),
		"--set", fmt.Sprintf("agent.config.PROM_MCP_URL=%s", promMCPURL),
		"--set", fmt.Sprintf("agent.config.CHAOS_NAMESPACE=%s", chaosNamespace),
		"--set", "sidecar.enabled=true",
		"--set", "sidecar.injectionMode=openai-metadata",
		// Let the sidecar forward to the real LiteLLM proxy (base URL without /v1)
		"--set", fmt.Sprintf("sidecar.upstream=%s", sidecarUpstream),
		// Pin the exact sidecar image that was built and loaded into minikube.
		"--set", fmt.Sprintf("sidecar.image.repository=%s", sidecarImageRepo),
		"--set", fmt.Sprintf("sidecar.image.tag=%s", sidecarImageTag),
		"--set", "sidecar.image.pullPolicy=Always",
	}

	// Ground truth is emitted directly to Langfuse via EmitFaultSpansForTrace
	// (called in the experiment run handler). It must NOT be written into the
	// agent ConfigMap because that would mount it onto the agent's filesystem,
	// allowing the agent to read the answer key before analysis.
	if groundTruthB64 != "" {
		logrus.WithField("faults", faultNames).Info("[Ground Truth] Ground truth loaded for Langfuse trace emission (not injected into agent ConfigMap)")
	}

	// isStaleSetArg returns true for --set values from previous runs that should
	// be stripped and re-injected with fresh values.
	isStaleSetArg := func(arg string) bool {
		return strings.HasPrefix(arg, "config.openaiApiKey=") ||
			strings.HasPrefix(arg, "config.openaiBaseUrl=") ||
			strings.HasPrefix(arg, "agentId=") ||
			strings.HasPrefix(arg, "agent.config.MCP_INCLUDE_CHAOS_TOOLS=") ||
			strings.HasPrefix(arg, "agent.config.NOTIFY_ID=") ||
			strings.HasPrefix(arg, "agent.config.EXPERIMENT_ID=") ||
			strings.HasPrefix(arg, "agent.config.EXPERIMENT_RUN_ID=") ||
			strings.HasPrefix(arg, "agent.config.WORKFLOW_NAME=") ||
			strings.HasPrefix(arg, "agent.config.OPENAI_API_KEY=") ||
			strings.HasPrefix(arg, "agent.secret.LITELLM_MASTER_KEY=") ||
			strings.HasPrefix(arg, "agent.config.OPENAI_BASE_URL=") ||
			strings.HasPrefix(arg, "agent.config.K8S_MCP_URL=") ||
			strings.HasPrefix(arg, "agent.config.PROM_MCP_URL=") ||
			strings.HasPrefix(arg, "agent.config.CHAOS_NAMESPACE=") ||
			strings.HasPrefix(arg, "agent.config.MODEL_ALIAS=") ||
			// GROUND_TRUTH_JSON is no longer injected into agent ConfigMap — strip any stale value from old runs
			strings.HasPrefix(arg, "agent.config.GROUND_TRUTH_JSON=") ||
			strings.HasPrefix(arg, "sidecar.enabled=") ||
			strings.HasPrefix(arg, "sidecar.injectionMode=") ||
			strings.HasPrefix(arg, "sidecar.upstream=") ||
			strings.HasPrefix(arg, "sidecar.image.repository=") ||
			strings.HasPrefix(arg, "sidecar.image.tag=") ||
			strings.HasPrefix(arg, "sidecar.image.pullPolicy=")
	}
	// isStaleFlag returns true for named binary flags (not --set values) that
	// carry a subsequent value and should be replaced by fresh injection.
	isStaleFlag := func(arg string) bool {
		return arg == "--server-addr" || arg == "--project-id"
	}

	for i := range templates {
		t := &templates[i]
		if t.Container == nil {
			continue
		}

		isInstallAgentTemplate := t.Name == "install-agent" ||
			strings.Contains(strings.TrimSpace(t.Container.Image), "agentcert-install-agent")
		if !isInstallAgentTemplate {
			continue
		}

		if len(t.Container.Args) > 0 {
			normalizedArgs := make([]string, 0, len(t.Container.Args))
			for idx := 0; idx < len(t.Container.Args); idx++ {
				arg := t.Container.Args[idx]

				// Strip stale --set key=value (combined form)
				if strings.HasPrefix(arg, "--set=") || strings.HasPrefix(arg, "--set-string=") {
					valueArg := strings.TrimPrefix(arg, "--set=")
					valueArg = strings.TrimPrefix(valueArg, "--set-string=")
					if isStaleSetArg(valueArg) {
						continue
					}
				}

				// Strip stale --set key value (split form)
				if (arg == "--set" || arg == "--set-string") && idx+1 < len(t.Container.Args) {
					nextArg := t.Container.Args[idx+1]
					if isStaleSetArg(nextArg) {
						idx++
						continue
					}
				}

				// Strip stale binary flags that carry the next arg as their value
				if isStaleFlag(arg) && idx+1 < len(t.Container.Args) {
					idx++
					continue
				}

				normalizedArgs = append(normalizedArgs, arg)
			}
			t.Container.Args = normalizedArgs
		}

		t.Container.Args = append(t.Container.Args, experimentSetArgs...)
		logrus.WithFields(logrus.Fields{
			"template": t.Name,
			"args":     experimentSetArgs,
			"source":   "hardcoded-fallback",
		}).Info("[Experiment Context] Injected --set args for experiment context (fallback)")
	}
}

// ExtractInstallAgentNamespace scans the install-agent template's container
// args for the --namespace (or -namespace) flag and returns its value.
// Install-agent deploys the agent into the APPLICATION namespace (e.g. sock-shop),
// which is different from the chaos INFRA namespace (e.g. litmus-exp).
// The correct namespace must be used when looking up the registered agent in MongoDB.
func ExtractInstallAgentNamespace(templates []v1alpha1.Template) string {
	for _, t := range templates {
		if t.Container == nil {
			continue
		}
		if t.Name != "install-agent" && !strings.Contains(strings.TrimSpace(t.Container.Image), "agentcert-install-agent") {
			continue
		}
		for i, arg := range t.Container.Args {
			if (arg == "--namespace" || arg == "-namespace") && i+1 < len(t.Container.Args) {
				return t.Container.Args[i+1]
			}
		}
	}
	return ""
}

// normalizeProbeExecutionSettings applies app-agnostic probe hardening for chaos
// execution by forcing bounded HTTP probe behavior.
func normalizeProbeExecutionSettings(raw string) (string, bool, error) {
	if strings.TrimSpace(raw) == "" {
		return raw, false, nil
	}

	var obj map[string]interface{}
	if err := yaml.Unmarshal([]byte(raw), &obj); err != nil {
		return raw, false, err
	}

	kind, _ := obj["kind"].(string)
	if strings.ToLower(kind) != "chaosengine" {
		return raw, false, nil
	}

	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return raw, false, nil
	}

	experiments, ok := spec["experiments"].([]interface{})
	if !ok || len(experiments) == 0 {
		return raw, false, nil
	}

	changed := false

	setMinDuration := func(runProps map[string]interface{}, key string, minValue string) bool {
		cur, exists := runProps[key]
		if !exists || cur == nil || strings.TrimSpace(toString(cur)) == "" {
			runProps[key] = minValue
			return true
		}

		curDur, errCur := time.ParseDuration(strings.TrimSpace(toString(cur)))
		minDur, errMin := time.ParseDuration(minValue)
		if errCur != nil || errMin != nil {
			runProps[key] = minValue
			return true
		}

		if curDur < minDur {
			runProps[key] = minValue
			return true
		}
		return false
	}

	setMinInt := func(runProps map[string]interface{}, key string, minValue int) bool {
		cur, exists := runProps[key]
		if !exists || cur == nil || strings.TrimSpace(toString(cur)) == "" {
			runProps[key] = minValue
			return true
		}

		curInt, err := strconv.Atoi(strings.TrimSpace(toString(cur)))
		if err != nil {
			runProps[key] = minValue
			return true
		}

		if curInt < minValue {
			runProps[key] = minValue
			return true
		}
		return false
	}

	for _, expAny := range experiments {
		exp, ok := expAny.(map[string]interface{})
		if !ok {
			continue
		}

		expSpec, ok := exp["spec"].(map[string]interface{})
		if !ok {
			continue
		}

		chaosDurationSec := getChaosDurationSeconds(expSpec)
		if chaosDurationSec <= 0 {
			chaosDurationSec = 60
		}

		recoveryWindowSec := chaosDurationSec * 4
		if recoveryWindowSec < 120 {
			recoveryWindowSec = 120
		}
		defaultIntervalSec := chaosDurationSec / 30
		if defaultIntervalSec < 2 {
			defaultIntervalSec = 2
		}
		if defaultIntervalSec > 10 {
			defaultIntervalSec = 10
		}

		defaultAttempt := (recoveryWindowSec + defaultIntervalSec - 1) / defaultIntervalSec
		if defaultAttempt < 5 {
			defaultAttempt = 5
		}
		defaultRetry := (defaultAttempt + 4) / 5
		if defaultRetry < 3 {
			defaultRetry = 3
		}

		defaultInitialDelaySec := chaosDurationSec
		if defaultInitialDelaySec < 20 {
			defaultInitialDelaySec = 20
		}
		if defaultInitialDelaySec > 120 {
			defaultInitialDelaySec = 120
		}

		defaultProbeTimeoutSec := defaultIntervalSec
		if defaultProbeTimeoutSec < 2 {
			defaultProbeTimeoutSec = 2
		}

		probes, ok := expSpec["probe"].([]interface{})
		if !ok {
			continue
		}

		for _, probeAny := range probes {
			probe, ok := probeAny.(map[string]interface{})
			if !ok {
				continue
			}

			probeType, _ := probe["type"].(string)
			_, hasHTTPInputs := probe["httpProbe/inputs"]
			if strings.ToLower(probeType) != "httpprobe" && !hasHTTPInputs {
				continue
			}

			mode, _ := probe["mode"].(string)
			if strings.TrimSpace(mode) == "" || strings.EqualFold(mode, "continuous") {
				probe["mode"] = "Edge"
				changed = true
			}

			runProps, ok := probe["runProperties"].(map[string]interface{})
			if !ok {
				runProps = map[string]interface{}{}
				probe["runProperties"] = runProps
				changed = true
			}

			// Hub templates can carry brittle probe settings. Enforce minimum
			// duration-derived budgets so template runs wait for recovery.
			if setMinDuration(runProps, "probeTimeout", formatDurationSeconds(defaultProbeTimeoutSec)) {
				changed = true
			}
			if setMinDuration(runProps, "interval", formatDurationSeconds(defaultIntervalSec)) {
				changed = true
			}
			if setMinDuration(runProps, "probePollingInterval", formatDurationSeconds(defaultIntervalSec)) {
				changed = true
			}
			if setMinDuration(runProps, "initialDelay", formatDurationSeconds(defaultInitialDelaySec)) {
				changed = true
			}
			if setMinInt(runProps, "attempt", defaultAttempt) {
				changed = true
			}
			if setMinInt(runProps, "retry", defaultRetry) {
				changed = true
			}
		}
	}

	if !changed {
		return raw, false, nil
	}

	out, err := yaml.Marshal(obj)
	if err != nil {
		return raw, false, err
	}

	return string(out), true, nil
}

func toString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case float32:
		return strconv.FormatInt(int64(v), 10)
	case float64:
		return strconv.FormatInt(int64(v), 10)
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func getChaosDurationSeconds(expSpec map[string]interface{}) int {
	components, ok := expSpec["components"].(map[string]interface{})
	if !ok {
		return 0
	}

	envList, ok := components["env"].([]interface{})
	if !ok {
		return 0
	}

	parseDuration := func(raw interface{}) int {
		value := strings.TrimSpace(toString(raw))
		if value == "" {
			return 0
		}
		seconds, err := strconv.Atoi(value)
		if err != nil || seconds <= 0 {
			return 0
		}
		return seconds
	}

	isDurationKey := func(name string) bool {
		upper := strings.ToUpper(strings.TrimSpace(name))
		if upper == "TOTAL_CHAOS_DURATION" || upper == "CHAOS_DURATION" {
			return true
		}
		return strings.Contains(upper, "CHAOS") && strings.Contains(upper, "DURATION")
	}

	for _, envAny := range envList {
		envItem, ok := envAny.(map[string]interface{})
		if !ok {
			continue
		}

		name := toString(envItem["name"])
		if !isDurationKey(name) {
			continue
		}

		seconds := parseDuration(envItem["value"])
		if seconds > 0 {
			return seconds
		}
	}

	return 0
}

func formatDurationSeconds(seconds int) string {
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds) + "s"
}
