package ops

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math/rand"
	"os"
	"reflect"
	"testing"

	dbSchemaProbe "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/probe"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	store "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/data-store"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
	dbChaosExperiment "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment"
	dbChaosExperimentRun "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment_run"
	dbChaosInfra "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_infrastructure"
	dbMocks "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/mocks"
	probe "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/probe/handler"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"sigs.k8s.io/yaml"
)

var (
	mongodbMockOperator        = new(dbMocks.MongoOperator)
	probeOperator              = dbSchemaProbe.NewChaosProbeOperator(mongodbMockOperator)
	infraOperator              = dbChaosInfra.NewInfrastructureOperator(mongodbMockOperator)
	chaosExperimentOperator    = dbChaosExperiment.NewChaosExperimentOperator(mongodbMockOperator)
	chaosExperimentRunOperator = dbChaosExperimentRun.NewChaosExperimentRunOperator(mongodbMockOperator)
	probeService               = probe.NewProbeService(probeOperator)
)

var chaosExperimentRunTestService = NewChaosExperimentService(chaosExperimentOperator, infraOperator, chaosExperimentRunOperator, probeService, nil)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

func loadYAMLData(path string) (string, error) {
	YAMLData, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	jsonData, err := yaml.YAMLToJSON(YAMLData)
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

func TestNewChaosExperimentService(t *testing.T) {
	type args struct {
		chaosWorkflowOperator      *dbChaosExperiment.Operator
		clusterOperator            *dbChaosInfra.Operator
		chaosExperimentRunOperator *dbChaosExperimentRun.Operator
		probeService               probe.Service
	}
	tests := []struct {
		name string
		args args
		want Service
	}{
		{
			name: "NewChaosExperimentService",
			args: args{
				chaosWorkflowOperator:      chaosExperimentOperator,
				clusterOperator:            infraOperator,
				chaosExperimentRunOperator: chaosExperimentRunOperator,
				probeService:               probeService,
			},
			want: &chaosExperimentService{
				chaosExperimentOperator:     chaosExperimentOperator,
				chaosInfrastructureOperator: infraOperator,
				chaosExperimentRunOperator:  chaosExperimentRunOperator,
				probeService:                probeService,
				agentRegistryOperator:       nil,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NewChaosExperimentService(tc.args.chaosWorkflowOperator, tc.args.clusterOperator, tc.args.chaosExperimentRunOperator, tc.args.probeService, nil); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("NewChaosExperimentService() = %v, want %v", got, tc.want)
			}
		})
	}
}

func Test_chaosExperimentService_ProcessExperiment(t *testing.T) {
	projectID := uuid.NewString()
	revID := uuid.NewString()

	commonPath := "../model/mocks/"
	yamlTypeMap := map[string]string{
		"workflow":       commonPath + "workflow.yaml",
		"cron_workflow":  commonPath + "cron_workflow.yaml",
		"chaos_engine":   commonPath + "chaos_engine.yaml",
		"chaos_schedule": commonPath + "chaos_schedule.yaml",
		"wrong_type":     commonPath + "wrong_type.yaml",
	}
	experimentID := uuid.NewString()
	infraID := uuid.NewString()
	projectID = uuid.NewString()

	tests := []struct {
		experiment *model.ChaosExperimentRequest
		name       string
		given      func(experiment *model.ChaosExperimentRequest)
		wantErr    bool
	}{
		{
			name: "success: Process Experiment (type-workflow)",
			experiment: &model.ChaosExperimentRequest{
				ExperimentID:   &experimentID,
				InfraID:        infraID,
				ExperimentName: "test-podtato-head-1682669740",
			},
			given: func(experiment *model.ChaosExperimentRequest) {
				findResult := bson.D{
					{Key: "infra_id", Value: infraID},
					{Key: "project_id", Value: projectID},
					{Key: "is_active", Value: true},
					{Key: "is_registered", Value: true},
				}
				singleResult := mongo.NewSingleResultFromDocument(findResult, nil, nil)
				mongodbMockOperator.On("Get", mock.Anything, mongodb.ChaosInfraCollection, mock.Anything).Return(singleResult, nil).Once()

				yaml, err := loadYAMLData(yamlTypeMap["workflow"])
				if (err != nil) != false {
					t.Errorf("chaosExperimentService.ProcessExperiment() error = %v, wantErr %v", err, false)
					return
				}
				experiment.ExperimentManifest = yaml
			},
		},
		{
			name: "success: Process Experiment (type-cron_workflow)",
			experiment: &model.ChaosExperimentRequest{
				ExperimentID:   &experimentID,
				InfraID:        infraID,
				ExperimentName: "test-podtato-head-1682669740",
			},
			given: func(experiment *model.ChaosExperimentRequest) {
				findResult := bson.D{
					{Key: "infra_id", Value: infraID},
					{Key: "project_id", Value: projectID},
					{Key: "is_active", Value: true},
					{Key: "is_registered", Value: true},
				}
				singleResult := mongo.NewSingleResultFromDocument(findResult, nil, nil)
				mongodbMockOperator.On("Get", mock.Anything, mongodb.ChaosInfraCollection, mock.Anything).Return(singleResult, nil).Once()

				yaml, err := loadYAMLData(yamlTypeMap["cron_workflow"])
				if (err != nil) != false {
					t.Errorf("chaosExperimentService.ProcessExperiment() error = %v, wantErr %v", err, false)
					return
				}
				experiment.ExperimentManifest = yaml
			},
		},
		{
			name: "success: Process Experiment (type-chaos_engine)",
			experiment: &model.ChaosExperimentRequest{
				ExperimentID:   &experimentID,
				InfraID:        infraID,
				ExperimentName: "nginx-chaos",
			},
			given: func(experiment *model.ChaosExperimentRequest) {
				findResult := bson.D{
					{Key: "infra_id", Value: infraID},
					{Key: "project_id", Value: projectID},
					{Key: "is_active", Value: true},
					{Key: "is_registered", Value: true},
				}
				singleResult := mongo.NewSingleResultFromDocument(findResult, nil, nil)
				mongodbMockOperator.On("Get", mock.Anything, mongodb.ChaosInfraCollection, mock.Anything).Return(singleResult, nil).Once()

				yaml, err := loadYAMLData(yamlTypeMap["chaos_engine"])
				if (err != nil) != false {
					t.Errorf("chaosExperimentService.ProcessExperiment() error = %v, wantErr %v", err, false)
					return
				}
				experiment.ExperimentManifest = yaml
			},
		},
		{
			name: "success: Process Experiment (type-chaos_schedule)",
			experiment: &model.ChaosExperimentRequest{
				ExperimentID:   &experimentID,
				InfraID:        infraID,
				ExperimentName: "schedule-nginx",
			},
			given: func(experiment *model.ChaosExperimentRequest) {
				findResult := bson.D{
					{Key: "infra_id", Value: infraID},
					{Key: "project_id", Value: projectID},
					{Key: "is_active", Value: true},
					{Key: "is_registered", Value: true},
				}
				singleResult := mongo.NewSingleResultFromDocument(findResult, nil, nil)
				mongodbMockOperator.On("Get", mock.Anything, mongodb.ChaosInfraCollection, mock.Anything).Return(singleResult, nil).Once()

				yaml, err := loadYAMLData(yamlTypeMap["chaos_schedule"])
				if (err != nil) != false {
					t.Errorf("chaosExperimentService.ProcessExperiment() error = %v, wantErr %v", err, false)
					return
				}
				experiment.ExperimentManifest = yaml
			},
		},
		{
			name: "failure: Process Experiment (type-random(incorrect))",
			experiment: &model.ChaosExperimentRequest{
				ExperimentID:   &experimentID,
				InfraID:        infraID,
				ExperimentName: "schedule-nginx",
			},
			given: func(experiment *model.ChaosExperimentRequest) {
				findResult := bson.D{
					{Key: "infra_id", Value: infraID},
					{Key: "project_id", Value: projectID},
					{Key: "is_active", Value: true},
					{Key: "is_registered", Value: true},
				}
				singleResult := mongo.NewSingleResultFromDocument(findResult, nil, nil)
				mongodbMockOperator.On("Get", mock.Anything, mongodb.ChaosInfraCollection, mock.Anything).Return(singleResult, nil).Once()

				yaml, err := loadYAMLData(yamlTypeMap["wrong_type"])
				if (err != nil) != false {
					t.Errorf("chaosExperimentService.ProcessExperiment() error = %v, wantErr %v", err, false)
					return
				}
				experiment.ExperimentManifest = yaml
			},
			wantErr: true,
		},
		{
			name: "failure: incorrect experiment name",
			experiment: &model.ChaosExperimentRequest{
				ExperimentID:   &experimentID,
				InfraID:        infraID,
				ExperimentName: "some_random_name",
			},
			given: func(experiment *model.ChaosExperimentRequest) {
				findResult := bson.D{
					{Key: "infra_id", Value: infraID},
					{Key: "project_id", Value: projectID},
					{Key: "is_active", Value: true},
					{Key: "is_registered", Value: true},
				}
				singleResult := mongo.NewSingleResultFromDocument(findResult, nil, nil)
				mongodbMockOperator.On("Get", mock.Anything, mongodb.ChaosInfraCollection, mock.Anything).Return(singleResult, nil).Once()

				yaml, err := loadYAMLData(yamlTypeMap["workflow"])
				if (err != nil) != false {
					t.Errorf("chaosExperimentService.ProcessExperiment() error = %v, wantErr %v", err, false)
					return
				}
				experiment.ExperimentManifest = yaml
			},
			wantErr: true,
		},
		{
			name: "failure: unable to unmarshal experiment manifest",
			experiment: &model.ChaosExperimentRequest{
				ExperimentID:   &experimentID,
				InfraID:        infraID,
				ExperimentName: "some_name",
			},
			given: func(experiment *model.ChaosExperimentRequest) {
				findResult := bson.D{
					{Key: "infra_id", Value: infraID},
					{Key: "project_id", Value: projectID},
					{Key: "is_active", Value: true},
					{Key: "is_registered", Value: true},
				}
				singleResult := mongo.NewSingleResultFromDocument(findResult, nil, nil)
				mongodbMockOperator.On("Get", mock.Anything, mongodb.ChaosInfraCollection, mock.Anything).Return(singleResult, nil).Once()

				yaml := "{\"kind\": \"SomeKubernetesKind\", \"apiVersion\": \"v1\", \"metadata\": {\"name\": \"some-name\"}"
				experiment.ExperimentManifest = yaml
			},
			wantErr: true,
		},
		{
			name: "failure: inactive infra",
			experiment: &model.ChaosExperimentRequest{
				ExperimentID:   &experimentID,
				InfraID:        infraID,
				ExperimentName: "test-podtato-head-1682669740",
			},
			given: func(_ *model.ChaosExperimentRequest) {
				findResult := bson.D{
					{Key: "infra_id", Value: infraID},
					{Key: "project_id", Value: projectID},
				}
				singleResult := mongo.NewSingleResultFromDocument(findResult, nil, nil)
				mongodbMockOperator.On("Get", mock.Anything, mongodb.ChaosInfraCollection, mock.Anything).Return(singleResult, nil).Once()
			},
			wantErr: true,
		},
		{
			name: "failure: incorrect project ID",
			experiment: &model.ChaosExperimentRequest{
				ExperimentID:   &experimentID,
				InfraID:        infraID,
				ExperimentName: "test-podtato-head-1682669740",
			},
			given: func(_ *model.ChaosExperimentRequest) {
				findResult := bson.D{
					{Key: "infra_id", Value: infraID},
					{Key: "project_id", Value: uuid.NewString()},
					{Key: "is_active", Value: true},
				}
				singleResult := mongo.NewSingleResultFromDocument(findResult, nil, nil)
				mongodbMockOperator.On("Get", mock.Anything, mongodb.ChaosInfraCollection, mock.Anything).Return(singleResult, nil).Once()
			},
			wantErr: true,
		},
		{
			name: "failure: mongo returns empty result",
			experiment: &model.ChaosExperimentRequest{
				ExperimentID:   &experimentID,
				InfraID:        infraID,
				ExperimentName: "test-podtato-head-1682669740",
			},
			given: func(_ *model.ChaosExperimentRequest) {
				singleResult := mongo.NewSingleResultFromDocument(nil, nil, nil)
				mongodbMockOperator.On("Get", mock.Anything, mongodb.ChaosInfraCollection, mock.Anything).Return(singleResult, errors.New("nil single result returned")).Once()
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.given(tc.experiment)
			_, _, err := chaosExperimentRunTestService.ProcessExperiment(context.Background(), tc.experiment, projectID, revID)
			if (err != nil) != tc.wantErr {
				t.Errorf("chaosExperimentService.ProcessExperiment() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
		})
	}
}

func Test_applyRBACPatch_InsertsBeforeInstallApplication(t *testing.T) {
	workflowJSON, err := loadYAMLData("../model/mocks/workflow.yaml")
	if err != nil {
		t.Fatalf("failed to load workflow mock: %v", err)
	}

	var wf v1alpha1.Workflow
	if err := json.Unmarshal([]byte(workflowJSON), &wf); err != nil {
		t.Fatalf("failed to unmarshal workflow: %v", err)
	}

	svc, ok := chaosExperimentRunTestService.(*chaosExperimentService)
	if !ok {
		t.Fatal("expected *chaosExperimentService implementation")
	}

	if err := svc.applyRBACPatch(&wf); err != nil {
		t.Fatalf("applyRBACPatch() returned error: %v", err)
	}

	var root *v1alpha1.Template
	for i := range wf.Spec.Templates {
		if wf.Spec.Templates[i].Name == wf.Spec.Entrypoint {
			root = &wf.Spec.Templates[i]
			break
		}
	}
	if root == nil {
		t.Fatal("entrypoint template not found")
	}

	rbacIndex := -1
	installIndex := -1
	for i, stepGroup := range root.Steps {
		for _, step := range stepGroup.Steps {
			switch step.Name {
			case "apply-workload-rbac":
				rbacIndex = i
			case "install-application":
				installIndex = i
			}
		}
	}

	if rbacIndex == -1 {
		t.Fatal("apply-workload-rbac step was not injected")
	}
	if installIndex == -1 {
		t.Fatal("install-application step not found in mock workflow")
	}
	if rbacIndex >= installIndex {
		t.Fatalf("expected apply-workload-rbac before install-application, got rbac index %d and install index %d", rbacIndex, installIndex)
	}
}

func Test_applyRBACPatch_ReordersExistingStepBeforeInstallApplication(t *testing.T) {
	workflowJSON, err := loadYAMLData("../model/mocks/workflow.yaml")
	if err != nil {
		t.Fatalf("failed to load workflow mock: %v", err)
	}

	var wf v1alpha1.Workflow
	if err := json.Unmarshal([]byte(workflowJSON), &wf); err != nil {
		t.Fatalf("failed to unmarshal workflow: %v", err)
	}

	svc, ok := chaosExperimentRunTestService.(*chaosExperimentService)
	if !ok {
		t.Fatal("expected *chaosExperimentService implementation")
	}

	if err := svc.applyRBACPatch(&wf); err != nil {
		t.Fatalf("initial applyRBACPatch() returned error: %v", err)
	}

	var root *v1alpha1.Template
	for i := range wf.Spec.Templates {
		if wf.Spec.Templates[i].Name == wf.Spec.Entrypoint {
			root = &wf.Spec.Templates[i]
			break
		}
	}
	if root == nil {
		t.Fatal("entrypoint template not found")
	}

	// Simulate stale ordering from older manifests by moving apply-workload-rbac to the end.
	filteredSteps := make([]v1alpha1.ParallelSteps, 0, len(root.Steps))
	for _, stepGroup := range root.Steps {
		newGroup := v1alpha1.ParallelSteps{Steps: make([]v1alpha1.WorkflowStep, 0, len(stepGroup.Steps))}
		for _, step := range stepGroup.Steps {
			if step.Name != "apply-workload-rbac" {
				newGroup.Steps = append(newGroup.Steps, step)
			}
		}
		if len(newGroup.Steps) > 0 {
			filteredSteps = append(filteredSteps, newGroup)
		}
	}
	filteredSteps = append(filteredSteps, v1alpha1.ParallelSteps{Steps: []v1alpha1.WorkflowStep{{
		Name:     "apply-workload-rbac",
		Template: "apply-workload-rbac",
	}}})
	root.Steps = filteredSteps

	if err := svc.applyRBACPatch(&wf); err != nil {
		t.Fatalf("reordering applyRBACPatch() returned error: %v", err)
	}

	rbacIndex := -1
	installIndex := -1
	rbacCount := 0
	for i, stepGroup := range root.Steps {
		for _, step := range stepGroup.Steps {
			switch step.Name {
			case "apply-workload-rbac":
				rbacCount++
				rbacIndex = i
			case "install-application":
				installIndex = i
			}
		}
	}

	if rbacCount != 1 {
		t.Fatalf("expected exactly one apply-workload-rbac step, got %d", rbacCount)
	}
	if installIndex == -1 || rbacIndex == -1 {
		t.Fatalf("expected both install-application and apply-workload-rbac steps, got install=%d rbac=%d", installIndex, rbacIndex)
	}
	if rbacIndex >= installIndex {
		t.Fatalf("expected apply-workload-rbac before install-application, got rbac index %d and install index %d", rbacIndex, installIndex)
	}
}

func Test_chaosExperimentService_ProcessExperimentCreation(t *testing.T) {
	type args struct {
		input *model.ChaosExperimentRequest
	}
	ctx := context.Background()
	store := store.NewStore()
	experimentID := uuid.NewString()
	projectID := uuid.NewString()
	revisionID := uuid.NewString()
	infraID := uuid.NewString()
	username := "test"
	wfType := dbChaosExperiment.NonCronExperiment

	tests := []struct {
		name    string
		args    args
		given   func()
		wantErr bool
	}{
		{
			name: "success: Process Experiment Creation",
			args: args{
				input: &model.ChaosExperimentRequest{
					ExperimentID: &experimentID,
					InfraID:      infraID,
				},
			},
			given: func() {
				mongodbMockOperator.On("Create", mock.Anything, mongodb.ChaosExperimentCollection, mock.Anything).Return(nil)
			},
		},
		{
			name: "success: Process Experiment Creation with weights",
			args: args{
				input: &model.ChaosExperimentRequest{
					ExperimentID: &experimentID,
					InfraID:      infraID,
					Weightages: []*model.WeightagesInput{
						{
							Weightage: rand.Int(),
						},
					},
				},
			},
			given: func() {
				mongodbMockOperator.On("Create", mock.Anything, mongodb.ChaosExperimentCollection, mock.Anything).Return(nil)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.given()
			if err := chaosExperimentRunTestService.ProcessExperimentCreation(ctx, tc.args.input, username, projectID, &wfType, revisionID, store); (err != nil) != tc.wantErr {
				t.Errorf("chaosExperimentService.ProcessExperimentCreation() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func Test_chaosExperimentService_ProcessExperimentUpdate(t *testing.T) {
	type args struct {
		workflow       *model.ChaosExperimentRequest
		updateRevision bool
	}
	username := "test"
	wfType := dbChaosExperiment.NonCronExperiment
	revisionID := uuid.NewString()
	infraID := uuid.NewString()
	projectID := uuid.NewString()
	store := store.NewStore()
	tests := []struct {
		name    string
		args    args
		given   func()
		wantErr bool
	}{
		{
			name: "success: Process Experiment Update",
			args: args{
				workflow: &model.ChaosExperimentRequest{
					Weightages: []*model.WeightagesInput{
						{
							FaultName: "pod-delete",
						},
					},
					InfraID:            infraID,
					ExperimentManifest: "{\"kind\": \"SomeKubernetesKind\", \"apiVersion\": \"v1\", \"metadata\": {\"name\": \"some-name\"}}",
				},
				updateRevision: true,
			},
			given: func() {
				updateResult := &mongo.UpdateResult{
					MatchedCount: 1,
				}
				mongodbMockOperator.On("Update", mock.Anything, mongodb.ChaosExperimentCollection, mock.Anything, mock.Anything, mock.Anything).Return(updateResult, nil).Once()
			},
			wantErr: false,
		},
		{
			name: "failure: incorrect experiment manifest",
			args: args{
				workflow: &model.ChaosExperimentRequest{
					Weightages: []*model.WeightagesInput{
						{
							FaultName: "pod-delete",
						},
					},
					InfraID:            infraID,
					ExperimentManifest: "{\"test\": \"name\"}",
				},
				updateRevision: true,
			},
			given: func() {
				updateResult := &mongo.UpdateResult{
					MatchedCount: 1,
				}
				mongodbMockOperator.On("Update", mock.Anything, mongodb.ChaosExperimentCollection, mock.Anything, mock.Anything, mock.Anything).Return(updateResult, nil).Once()
			},
			wantErr: true,
		},
		{
			name: "failure: failed to update experiment",
			args: args{
				workflow: &model.ChaosExperimentRequest{
					Weightages: []*model.WeightagesInput{
						{
							FaultName: "pod-delete",
						},
					},
					InfraID:            infraID,
					ExperimentManifest: "{\"kind\": \"SomeKubernetesKind\", \"apiVersion\": \"v1\", \"metadata\": {\"name\": \"some-name\"}}",
				},
				updateRevision: true,
			},
			given: func() {
				updateResult := &mongo.UpdateResult{
					MatchedCount: 1,
				}
				mongodbMockOperator.On("Update", mock.Anything, mongodb.ChaosExperimentCollection, mock.Anything, mock.Anything, mock.Anything).Return(updateResult, errors.New("error while updating")).Once()
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.given()
			if err := chaosExperimentRunTestService.ProcessExperimentUpdate(tc.args.workflow, username, &wfType, revisionID, tc.args.updateRevision, projectID, store); (err != nil) != tc.wantErr {
				t.Errorf("chaosExperimentService.ProcessExperimentUpdate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
