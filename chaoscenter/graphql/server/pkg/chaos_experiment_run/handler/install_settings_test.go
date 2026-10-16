package handler

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/golang-jwt/jwt/v4"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/authorization"
	store "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/data-store"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
	experiments "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment"
	dbMocks "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/mocks"
	probeMocks "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/probe/model/mocks"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func TestCronDispatchRefreshesConfiguration(t *testing.T) {
	root := t.TempDir()
	previousAgentHub, previousAppHub, previousKubeconfig := utils.Config.DefaultAgentHubPath, utils.Config.DefaultAppHubPath, utils.Config.KubeConfigFilePath
	previousMongo := mongodb.Operator
	utils.Config.DefaultAgentHubPath, utils.Config.DefaultAppHubPath = root, root
	t.Cleanup(func() {
		utils.Config.DefaultAgentHubPath, utils.Config.DefaultAppHubPath, utils.Config.KubeConfigFilePath = previousAgentHub, previousAppHub, previousKubeconfig
		mongodb.Operator = previousMongo
	})
	for chart, values := range map[string]string{
		"agent": "agent:\n  config:\n    SCAN_INTERVAL: \"60\"\n    MODEL_ALIAS: default\nconfigurations:\n  - key: agent.config.SCAN_INTERVAL\n    type: integer\n    min: 1\n  - key: agent.config.MODEL_ALIAS\n    type: model\n",
		"app":   "app:\n  replicas: 1\nconfigurations:\n  - key: app.replicas\n    type: integer\n    min: 1\n",
	} {
		dir := filepath.Join(root, "charts", chart)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "values.yaml"), []byte(values), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Runtime discovery uses an unreachable local endpoint, never a real cluster.
	utils.Config.KubeConfigFilePath = filepath.Join(root, "kubeconfig")
	if err := os.WriteFile(utils.Config.KubeConfigFilePath, []byte("apiVersion: v1\nkind: Config\nclusters:\n- name: test\n  cluster:\n    server: http://127.0.0.1:1\ncontexts:\n- name: test\n  context:\n    cluster: test\ncurrent-context: test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FLASH_AGENT_MODEL", "fresh-platform-model")
	const agentValues = `{"agent":{"config":{"SCAN_INTERVAL":"90","MODEL_ALIAS":"experiment-model"}}}`
	const appValues = `{"app":{"replicas":3}}`
	for _, override := range []string{"", "api-model"} {
		t.Run("override="+override, func(t *testing.T) {
			cron := v1alpha1.CronWorkflow{
				TypeMeta:   metav1.TypeMeta{APIVersion: "argoproj.io/v1alpha1", Kind: "CronWorkflow"},
				ObjectMeta: metav1.ObjectMeta{Name: "configured", Namespace: "chaos"},
				Spec: v1alpha1.CronWorkflowSpec{Schedule: "0 * * * *", WorkflowSpec: v1alpha1.WorkflowSpec{
					Entrypoint: "main",
					Arguments: v1alpha1.Arguments{Parameters: []v1alpha1.Parameter{
						{Name: "appNamespace", Value: v1alpha1.AnyStringPtr("target")},
						{Name: "agentFolder", Value: v1alpha1.AnyStringPtr("agent")},
					}},
					Templates: []v1alpha1.Template{
						{Name: "main", Steps: []v1alpha1.ParallelSteps{
							{Steps: []v1alpha1.WorkflowStep{{Name: "install-application", Template: "install-application"}}},
							{Steps: []v1alpha1.WorkflowStep{{Name: "install-agent", Template: "install-agent"}}},
						}},
						{Name: "install-application", Container: &corev1.Container{Image: "agentcert/agentcert-install-app", Args: []string{"-folder=app", "-namespace=target", "-values-json=" + appValues, "--set=app.replicas=1"}}},
						{Name: "install-agent", Container: &corev1.Container{Image: "agentcert/agentcert-install-agent", Args: []string{"-folder=agent", "-namespace=target", "-values-json=" + agentValues, "--set=agent.config.MODEL_ALIAS=stale"}}},
					},
				}},
			}
			cron.Spec.WorkflowMetadata = &metav1.ObjectMeta{Labels: map[string]string{"workflow_id": "experiment", "experiment_name": "configured"}}
			data, err := json.Marshal(cron)
			if err != nil {
				t.Fatal(err)
			}
			workflow := experiments.ChaosExperimentRequest{ProjectID: "project", InfraID: "infra", ExperimentID: "experiment", Revision: []experiments.ExperimentRevision{{ExperimentManifest: string(data)}}}
			mongoMock := new(dbMocks.MongoOperator)
			mongodb.Operator = mongoMock
			mongoMock.On("GetAuthConfig", mock.Anything, "salt").Return(&mongodb.AuthConfig{Value: "test-salt"}, nil).Once()
			infra := mongo.NewSingleResultFromDocument(bson.M{"infra_id": "infra", "infra_namespace": "chaos"}, nil, nil)
			mongoMock.On("Get", mock.Anything, mongodb.ChaosInfraCollection, mock.Anything).Return(infra, nil).Once()
			if override != "" {
				mongoMock.On("Update", mock.Anything, mongodb.ChaosExperimentCollection, mock.Anything, mock.Anything, mock.Anything).Return(&mongo.UpdateResult{MatchedCount: 1}, nil).Once()
			}
			probes := new(probeMocks.ProbeService)
			probes.On("GenerateCronExperimentManifestWithProbes", string(data), "project").Return(cron, nil).Once()
			handler := &ChaosExperimentRunHandler{probeService: probes, mongodbOperator: mongoMock, chaosExperimentOperator: experiments.NewChaosExperimentOperator(mongoMock)}
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"username": "tester"}).SignedString([]byte("test-salt"))
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.WithValue(context.Background(), authorization.AuthKey, token)
			state := store.NewStore()
			messages := make(chan *model.InfraActionResponse, 1)
			state.ConnectedInfra["infra"] = messages
			if err := handler.runCronExperiment(ctx, "project", workflow, state, override); err != nil {
				t.Fatal(err)
			}
			var sent v1alpha1.CronWorkflow
			select {
			case action := <-messages:
				if err := yaml.Unmarshal([]byte(action.Action.K8sManifest), &sent); err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatal("no workflow was dispatched")
			}
			parameters := buildWorkflowParameterMap(sent.Spec.WorkflowSpec.Arguments)
			parameterRef := regexp.MustCompile(`\{\{workflow\.parameters\.([A-Za-z0-9_-]+)\}\}`)
			wantModel := "experiment-model"
			if override != "" {
				wantModel = override
			}
			for _, template := range sent.Spec.WorkflowSpec.Templates {
				if template.Container == nil {
					continue
				}
				args := strings.Join(template.Container.Args, "\n")
				for _, match := range parameterRef.FindAllStringSubmatch(args, -1) {
					if _, exists := parameters[match[1]]; !exists {
						t.Errorf("%s references missing parameter %s", template.Name, match[1])
					}
				}
				switch template.Name {
				case "install-application":
					if !strings.Contains(args, "-values-json="+appValues) || strings.Contains(args, "--set=app.replicas=1") {
						t.Fatalf("app args: %s", args)
					}
				case "install-agent":
					if strings.Contains(args, "{{workflow.labels.notify_id}}") {
						t.Fatal("scheduled runs must use the workflow UID for trace correlation")
					}
					if !strings.Contains(args, "-values-json="+agentValues) || !strings.Contains(args, "agent.config.MODEL_ALIAS="+wantModel) || strings.Contains(args, "MODEL_ALIAS=stale") {
						t.Fatalf("agent args: %s", args)
					}
				}
			}
			mongoMock.AssertExpectations(t)
			probes.AssertExpectations(t)
		})
	}
}
