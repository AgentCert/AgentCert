package ops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	probeUtils "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/probe/utils"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"
	"github.com/sirupsen/logrus"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaos_infrastructure"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/agenthub"

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
}

// NewChaosExperimentService returns a new instance of the chaos workflow service
func NewChaosExperimentService(chaosWorkflowOperator *dbChaosExperiment.Operator, clusterOperator *dbChaosInfra.Operator, chaosExperimentRunOperator *dbChaosExperimentRun.Operator, probeService probe.Service) Service {
	return &chaosExperimentService{
		chaosExperimentOperator:     chaosWorkflowOperator,
		chaosInfrastructureOperator: clusterOperator,
		chaosExperimentRunOperator:  chaosExperimentRunOperator,
		probeService:                probeService,
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
	injectExperimentContextArgs(workflowManifest.Spec.Templates)

	if workflowManifest.Labels == nil {
		workflowManifest.Labels = map[string]string{
			"workflow_id": *workflow.ExperimentID,
			"infra_id":    workflow.InfraID,
			"workflows.argoproj.io/controller-instanceid": workflow.InfraID,
			"revision_id": revID,
		}
	} else {
		workflowManifest.Labels["workflow_id"] = *workflow.ExperimentID
		workflowManifest.Labels["infra_id"] = workflow.InfraID
		workflowManifest.Labels["workflows.argoproj.io/controller-instanceid"] = workflow.InfraID
		workflowManifest.Labels["revision_id"] = revID
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

	// Apply workload-discovery RBAC patch automatically after install-application.
	err = c.applyRBACPatch(&workflowManifest)
	if err != nil {
		logrus.Errorf("Failed to apply RBAC patch: %v", err)
		// Log but don't fail - RBAC may already exist in the cluster.
	}
	
	// Apply readiness normalization patch automatically
	err = c.applyInstallApplicationReadinessPatch(&workflowManifest)
	if err != nil {
		logrus.Errorf("Failed to apply readiness patch: %v", err)
		// Log but don't fail - readiness patch is optional
	}
	
	out, err := json.Marshal(workflowManifest)
	if err != nil {
		return err
	}

	workflow.ExperimentManifest = string(out)
	return nil
}

// applyRBACPatch injects a step after install-application to grant the Litmus server
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
	for _, t := range wf.Spec.Templates {
		if t.Name == rbacStepName {
			return nil
		}
	}

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

TARGET_APP_NS="{{workflow.parameters.appNamespace}}"
if [ -z "${TARGET_APP_NS}" ]; then
	TARGET_APP_NS="{{workflow.parameters.adminModeNamespace}}"
fi

echo "[rbac-patch] ensuring namespace install RBAC in ${TARGET_APP_NS} for ${SA_NAMESPACE}/${SA_NAME}"

kubectl -n "${TARGET_APP_NS}" create role litmus-app-installer \
	--verb=get,list,watch,create,update,patch,delete \
	--resource=configmaps,secrets,serviceaccounts,services,pods,persistentvolumeclaims,jobs.batch,cronjobs.batch,deployments.apps,replicasets.apps,statefulsets.apps,daemonsets.apps,roles.rbac.authorization.k8s.io,rolebindings.rbac.authorization.k8s.io \
	--dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${TARGET_APP_NS}" create rolebinding "${APP_RB_NAME}" \
	--role=litmus-app-installer \
	--serviceaccount="${SA_NAMESPACE}:${SA_NAME}" \
	--dry-run=client -o yaml | kubectl apply -f -

echo "[rbac-patch] workload-discovery RBAC applied"`},
		},
	}

	wf.Spec.Templates = append(wf.Spec.Templates, rbacTpl)

	newStepGroup := v1alpha1.ParallelSteps{
		Steps: []v1alpha1.WorkflowStep{{
			Name:     rbacStepName,
			Template: rbacStepName,
		}},
	}

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
	injectExperimentContextArgs(cronExperimentManifest.Spec.WorkflowSpec.Templates)

	if strings.TrimSpace(cronExperimentManifest.Spec.Schedule) == "" {
		return errors.New("failed to process cron workflow, cron syntax not provided in manifest")
	}

	if cronExperimentManifest.Labels == nil {
		cronExperimentManifest.Labels = map[string]string{
			"workflow_id": *workflow.ExperimentID,
			"infra_id":    workflow.InfraID,
			"workflows.argoproj.io/controller-instanceid": workflow.InfraID,
			"revision_id": revID,
		}
	} else {
		cronExperimentManifest.Labels["workflow_id"] = *workflow.ExperimentID
		cronExperimentManifest.Labels["infra_id"] = workflow.InfraID
		cronExperimentManifest.Labels["workflows.argoproj.io/controller-instanceid"] = workflow.InfraID
		cronExperimentManifest.Labels["revision_id"] = revID
	}

	if cronExperimentManifest.Spec.WorkflowMetadata == nil {
		cronExperimentManifest.Spec.WorkflowMetadata = &v1.ObjectMeta{
			Labels: map[string]string{
				"workflow_id": *workflow.ExperimentID,
				"infra_id":    workflow.InfraID,
				"workflows.argoproj.io/controller-instanceid": workflow.InfraID,
				"revision_id": revID,
			},
		}
	} else {
		if cronExperimentManifest.Spec.WorkflowMetadata.Labels == nil {
			cronExperimentManifest.Spec.WorkflowMetadata.Labels = map[string]string{
				"workflow_id": *workflow.ExperimentID,
				"infra_id":    workflow.InfraID,
				"workflows.argoproj.io/controller-instanceid": workflow.InfraID,
				"revision_id": revID,
			}
		} else {
			cronExperimentManifest.Spec.WorkflowMetadata.Labels["workflow_id"] = *workflow.ExperimentID
			cronExperimentManifest.Spec.WorkflowMetadata.Labels["infra_id"] = workflow.InfraID
			cronExperimentManifest.Spec.WorkflowMetadata.Labels["workflows.argoproj.io/controller-instanceid"] = workflow.InfraID
			cronExperimentManifest.Spec.WorkflowMetadata.Labels["revision_id"] = revID
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

// applyInstallAgentTemplateOverrides enforces a configurable install-agent image
// and pull policy for all template-based workflow manifests.
//
// Phase 1 (metadata-driven with fallback):
//   - Try reading agent metadata from chartserviceversion.yaml
//   - If metadata exists → match templates by installTemplateName / installImage
//   - If metadata missing (CSV not synced, parse error) → fall back to hardcoded matching
func applyInstallAgentTemplateOverrides(templates []v1alpha1.Template) {
	envImage := strings.TrimSpace(os.Getenv("INSTALL_AGENT_IMAGE"))
	envPullPolicy := strings.TrimSpace(os.Getenv("INSTALL_AGENT_IMAGE_PULL_POLICY"))
	if envPullPolicy == "" {
		envPullPolicy = string(corev1.PullIfNotPresent)
	}
	switch corev1.PullPolicy(envPullPolicy) {
	case corev1.PullAlways, corev1.PullIfNotPresent, corev1.PullNever:
	default:
		envPullPolicy = string(corev1.PullIfNotPresent)
	}

	// Try metadata-driven path
	agentEntries := agenthub.GetAgentInjectionMetadata()
	if len(agentEntries) > 0 {
		applyInstallAgentTemplateOverridesFromMetadata(templates, agentEntries, envImage, envPullPolicy)
		return
	}

	// Fallback: hardcoded matching (will be removed in Phase 2)
	logrus.Warn("[Install Agent Patch] CSV metadata not available — using hardcoded fallback")
	applyInstallAgentTemplateOverridesFallback(templates, envImage, envPullPolicy)
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

// injectExperimentContextArgs appends --set flags to install-agent templates
// so that Argo Workflow template variables (experiment_id, run_id, workflow_name)
// are passed through to the agent Helm chart as ConfigMap values.
//
// Phase 1 (metadata-driven with fallback):
//   - Try building --set args from chartserviceversion contextInjection metadata
//   - If metadata missing → fall back to hardcoded agent.config.* paths
func injectExperimentContextArgs(templates []v1alpha1.Template) {
	// Try metadata-driven path
	agentEntries := agenthub.GetAgentInjectionMetadata()
	if len(agentEntries) > 0 {
		injectExperimentContextArgsFromMetadata(templates, agentEntries)
		return
	}

	// Fallback: hardcoded --set paths (will be removed in Phase 2)
	logrus.Warn("[Experiment Context] CSV metadata not available — using hardcoded fallback")
	injectExperimentContextArgsFallback(templates)
}

// injectExperimentContextArgsFromMetadata builds --set args from CSV contextInjection
// entries. Each agent declares which helmPath maps to which Argo template variable.
func injectExperimentContextArgsFromMetadata(templates []v1alpha1.Template, agents []agenthub.AgentEntry) {
	for i := range templates {
		t := &templates[i]
		if t.Container == nil {
			continue
		}

		// Find matching agent
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

		// Check idempotency — if any helmPath from this agent is already present, skip
		alreadyHas := false
		if len(matched.ContextInjection) > 0 {
			checkKey := matched.ContextInjection[0].HelmPath + "="
			for _, arg := range t.Container.Args {
				if strings.Contains(arg, checkKey) {
					alreadyHas = true
					break
				}
			}
		}
		if alreadyHas {
			continue
		}

		// Build --set args from metadata
		var setArgs []string
		for _, ci := range matched.ContextInjection {
			setArgs = append(setArgs, "--set", ci.HelmPath+"="+ci.Source)
		}

		t.Container.Args = append(t.Container.Args, setArgs...)
		logrus.WithFields(logrus.Fields{
			"template": t.Name,
			"agent":    matched.Name,
			"args":     setArgs,
			"source":   "csv-metadata",
		}).Info("[Experiment Context] Injected --set args from CSV metadata")
	}
}

// injectExperimentContextArgsFallback is the original hardcoded injection logic.
// Retained for backward compatibility when CSV metadata is not yet available.
// TODO(phase2): Remove this function once CSV metadata is confirmed stable in production.
func injectExperimentContextArgsFallback(templates []v1alpha1.Template) {
	experimentSetArgs := []string{
		"--set", "agent.config.EXPERIMENT_ID={{workflow.labels.workflow_id}}",
		"--set", "agent.config.EXPERIMENT_RUN_ID={{workflow.uid}}",
		"--set", "agent.config.WORKFLOW_NAME={{workflow.labels.subject}}",
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

		// Check if experiment context args are already present (idempotent)
		alreadyHas := false
		for _, arg := range t.Container.Args {
			if strings.Contains(arg, "EXPERIMENT_ID=") {
				alreadyHas = true
				break
			}
		}
		if alreadyHas {
			continue
		}

		t.Container.Args = append(t.Container.Args, experimentSetArgs...)
		logrus.WithFields(logrus.Fields{
			"template": t.Name,
			"args":     experimentSetArgs,
			"source":   "hardcoded-fallback",
		}).Info("[Experiment Context] Injected --set args for experiment context (fallback)")
	}
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
