package graph

import (
	"context"
	"os"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/authConfig"

	chaos_experiment2 "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaos_experiment/ops"

	"github.com/99designs/gqlgen/graphql"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/generated"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/agent_registry"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/authorization"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/experiment_definition"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/fault_catalog"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaos_experiment/handler"
	chaos_experiment_run2 "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaos_experiment_run"
	runHandler "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaos_experiment_run/handler"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaos_infrastructure"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/agenthub"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/apphub"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/catalog"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/certification"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaoshub"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment_run"
	dbSchemaChaosHub "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_hub"
	dbChaosInfra "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_infrastructure"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/environments"
	dbSchemaFaultStudio "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/fault_studio"
	gitops2 "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/gitops"
	image_registry2 "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/image_registry"
	model_library_db "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/model_library"
	dbSchemaProbe "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/probe"
	envHandler "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/environment/handler"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/fault_studio"
	gitops3 "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/gitops"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/image_registry"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/model_library"
	probe "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/probe/handler"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	chaosHubService            chaoshub.Service
	imageRegistryService       image_registry.Service
	chaosInfrastructureService chaos_infrastructure.Service
	chaosExperimentService     chaos_experiment2.Service
	choasExperimentRunService  chaos_experiment_run2.Service
	gitopsService              gitops3.Service
	chaosExperimentHandler     handler.ChaosExperimentHandler
	chaosExperimentRunHandler  runHandler.ChaosExperimentRunHandler
	environmentService         envHandler.EnvironmentHandler
	probeService               probe.Service
	agentRegistryService       agent_registry.Service
	faultStudioService         fault_studio.Service
	agentHubService            agenthub.Service
	appHubService              apphub.Service
	certificationService       *certification.Service
	catalogService              catalog.Service
	modelLibraryService         model_library.ModelLibraryService
	faultCatalogService         fault_catalog.Service
	experimentDefinitionService experiment_definition.Service
	runRepository               experiment_definition.RunRepository
	kubeClient                  kubernetes.Interface
}

// NewConfig constructs the gqlgen generated.Config and also returns the
// catalog.Service so that callers (e.g. server.go) can share the single
// instance with REST handlers without creating a duplicate background goroutine.
func NewConfig(mongodbOperator mongodb.MongoOperator) (generated.Config, catalog.Service) {
	//operator
	chaosHubOperator := dbSchemaChaosHub.NewChaosHubOperator(mongodbOperator)
	chaosInfraOperator := dbChaosInfra.NewInfrastructureOperator(mongodbOperator)
	chaosExperimentOperator := chaos_experiment.NewChaosExperimentOperator(mongodbOperator)
	chaosExperimentRunOperator := chaos_experiment_run.NewChaosExperimentRunOperator(mongodbOperator)
	gitopsOperator := gitops2.NewGitOpsOperator(mongodbOperator)
	imageRegistryOperator := image_registry2.NewImageRegistryOperator(mongodbOperator)
	EnvironmentOperator := environments.NewEnvironmentOperator(mongodbOperator)
	probeOperator := dbSchemaProbe.NewChaosProbeOperator(mongodbOperator)
	agentRegistryOperator := agent_registry.NewOperator(mongodbOperator.(*mongodb.MongoOperations).MongoClient.Database)

	// Initialize Model Library dependencies
	modelLibraryDB := model_library_db.NewOperations(mongodbOperator.(*mongodb.MongoOperations).MongoClient.Database)
	litellmClient := model_library.NewLiteLLMClient()
	k8sClient := model_library.GetK8sClient()
	modelLibrarySvc := model_library.NewService(modelLibraryDB, litellmClient, k8sClient)

	//service
	probeService := probe.NewProbeService(probeOperator)
	chaosHubService := chaoshub.NewService(chaosHubOperator)
	chaosInfrastructureService := chaos_infrastructure.NewChaosInfrastructureService(chaosInfraOperator, EnvironmentOperator)
	chaosExperimentService := chaos_experiment2.NewChaosExperimentService(chaosExperimentOperator, chaosInfraOperator, chaosExperimentRunOperator, probeService, agentRegistryOperator)
	chaosExperimentRunService := chaos_experiment_run2.NewChaosExperimentRunService(chaosExperimentOperator, chaosInfraOperator, chaosExperimentRunOperator)
	gitOpsService := gitops3.NewGitOpsService(gitopsOperator, chaosExperimentService, *chaosExperimentOperator)
	imageRegistryService := image_registry.NewImageRegistryService(imageRegistryOperator)
	environmentService := envHandler.NewEnvironmentService(EnvironmentOperator)

	// Initialize Fault Studio dependencies
	faultStudioOperator := dbSchemaFaultStudio.NewFaultStudioOperator(mongodbOperator)
	faultStudioService := fault_studio.NewService(faultStudioOperator, chaosHubOperator)

	// Load capabilities vocabulary from the catalog directory (permissive when unavailable).
	capDir := os.Getenv("CATALOG_CAPABILITIES_DIR")
	if capDir == "" {
		capDir = "../../../../catalog/capabilities"
	}
	var capVocab *agent_registry.CapabilityVocab
	if vocab, err := agent_registry.LoadCapabilitiesFromDir(capDir); err != nil {
		log.WithError(err).Warn("capabilities vocab not loaded — using permissive validation")
	} else {
		capVocab = vocab
	}

	// Initialize Agent Registry dependencies
	agentRegistryValidator := agent_registry.NewValidator(agentRegistryOperator, capVocab)
	langfuseClient := agent_registry.NewLangfuseClient("", "", "") // Empty config - disabled
	agentRegistryService := agent_registry.NewService(agentRegistryOperator, agentRegistryValidator, langfuseClient, nil, capVocab)

	// Initialize AgentHub and AppsHub services
	agentHubService := agenthub.NewService(agentRegistryService)
	appHubService := apphub.NewService()

	// Initialize Catalog service (warn on error; resolvers nil-guard gracefully)
	var catalogSvc catalog.Service
	if svc, err := catalog.NewService(utils.Config.CatalogDir); err != nil {
		log.WithError(err).Warn("catalog service unavailable — listApplications/getApplication will return empty results")
	} else {
		catalogSvc = svc
	}

	// Initialize Fault Catalog service (in-memory, loaded from ACE_CATALOG_ROOT)
	catalogRoot := os.Getenv("ACE_CATALOG_ROOT")
	if catalogRoot == "" {
		catalogRoot = utils.Config.CatalogDir
		if catalogRoot == "" {
			catalogRoot = "/catalog"
		}
	}
	var faultCatalogSvc fault_catalog.Service
	if err := fault_catalog.LoadCatalog(catalogRoot); err != nil {
		log.WithError(err).Warn("fault catalog load failed (non-fatal)")
	} else {
		faultCatalogSvc = fault_catalog.NewService(catalogSvc)
	}

	// Initialize Experiment Definition service (MongoDB-backed)
	expDefRepo := experiment_definition.NewRepository(mongodbOperator.(*mongodb.MongoOperations).MongoClient.Database)
	expDefSvc := experiment_definition.NewService(expDefRepo, faultCatalogSvc)

	// Initialize Experiment Run repository (MongoDB-backed)
	runRepo := experiment_definition.NewRunRepository(mongodbOperator.(*mongodb.MongoOperations).MongoClient.Database)

	// Initialize Certification orchestrator (poller-based workflow that
	// drives the four certifier APIs and persists state in MongoDB).
	certificationOperator := certification.NewOperator(
		mongodbOperator,
		mongodb.CertificateExperimentsCollection,
		mongodb.CertificateRunWorkflowsCollection,
		mongodb.CertificateAggregationWorkflowsCollection,
	)
	certificationService := certification.NewService(
		certificationOperator,
		certification.NewClient(utils.Config.CertifierBaseURL),
	)

	//handler
	chaosExperimentHandler := handler.NewChaosExperimentHandler(chaosExperimentService, chaosExperimentRunService, chaosInfrastructureService, gitOpsService, chaosExperimentOperator, chaosExperimentRunOperator, probeService, mongodbOperator)
	choasExperimentRunHandler := runHandler.NewChaosExperimentRunHandler(chaosExperimentRunService, chaosInfrastructureService, gitOpsService, chaosExperimentOperator, chaosExperimentRunOperator, probeService, mongodbOperator, agentRegistryOperator)
	// Wire optional certification orchestrator so terminal experiment runs
	// auto-trigger the bucketing-extraction pipeline (no UI call required).
	choasExperimentRunHandler.SetCertificationService(certificationService)

	config := generated.Config{
		Resolvers: &Resolver{
			chaosHubService:             chaosHubService,
			certificationService:        certificationService,
			chaosInfrastructureService:  chaosInfrastructureService,
			chaosExperimentService:      chaosExperimentService,
			choasExperimentRunService:   chaosExperimentRunService,
			imageRegistryService:        imageRegistryService,
			environmentService:          environmentService,
			gitopsService:               gitOpsService,
			chaosExperimentHandler:      *chaosExperimentHandler,
			chaosExperimentRunHandler:   *choasExperimentRunHandler,
			probeService:                probeService,
			agentRegistryService:        agentRegistryService,
			faultStudioService:          faultStudioService,
			agentHubService:             agentHubService,
			appHubService:               appHubService,
			catalogService:              catalogSvc,
			modelLibraryService:         modelLibrarySvc,
			faultCatalogService:         faultCatalogSvc,
			experimentDefinitionService: expDefSvc,
			runRepository:               runRepo,
			kubeClient:                  k8sClient,
		}}

	config.Directives.Authorized = func(ctx context.Context, obj interface{}, next graphql.Resolver) (interface{}, error) {
		token := ctx.Value(authorization.AuthKey).(string)
		salt, err := authConfig.NewAuthConfigOperator(mongodb.Operator).GetAuthConfig(context.Background())
		if err != nil {
			return "", err
		}
		user, err := authorization.UserValidateJWT(token, salt.Value)
		if err != nil {
			return nil, err
		}

		newCtx := context.WithValue(ctx, authorization.UserClaim, user)

		return next(newCtx)
	}
	return config, catalogSvc
}
