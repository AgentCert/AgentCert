package mongodb

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"
)

// Enum for Database collections
const (
	ChaosInfraCollection = iota
	ChaosExperimentCollection
	ChaosExperimentRunsCollection
	ChaosHubCollection
	ImageRegistryCollection
	ServerConfigCollection
	GitOpsCollection
	UserCollection
	ProjectCollection
	EnvironmentCollection
	ChaosProbeCollection
	AgentRegistryCollection
	FaultStudioCollection
	CertificateExperimentsCollection
	CertificateRunWorkflowsCollection
	CertificateAggregationWorkflowsCollection
)

// MongoInterface requires a MongoClient that implements the Initialize method to create the Mongo DB client
// and a initAllCollection method to initialize all DB Collections
type MongoInterface interface {
	Initialize(client *mongo.Client) *MongoClient
	initAllCollection()
}

// MongoClient structure contains all the Database collections and the instance of the Database
type MongoClient struct {
	Database                      *mongo.Database
	ChaosInfraCollection          *mongo.Collection
	ChaosExperimentCollection     *mongo.Collection
	ChaosExperimentRunsCollection *mongo.Collection
	ChaosHubCollection            *mongo.Collection
	ChaosServerConfigCollection   *mongo.Collection
	ImageRegistryCollection       *mongo.Collection
	ServerConfigCollection        *mongo.Collection
	GitOpsCollection              *mongo.Collection
	UserCollection                *mongo.Collection
	ProjectCollection             *mongo.Collection
	EnvironmentCollection         *mongo.Collection
	ChaosProbeCollection          *mongo.Collection
	AgentRegistryCollection       *mongo.Collection
	FaultStudioCollection         *mongo.Collection
	CertificateExperimentsCollection          *mongo.Collection
	CertificateRunWorkflowsCollection         *mongo.Collection
	CertificateAggregationWorkflowsCollection *mongo.Collection
}

var (
	Client      MongoInterface = &MongoClient{}
	MgoClient   *mongo.Client
	Collections = map[int]string{
		ChaosInfraCollection:          "chaosInfrastructures",
		ChaosExperimentCollection:     "chaosExperiments",
		ChaosExperimentRunsCollection: "chaosExperimentRuns",
		ChaosProbeCollection:          "chaosProbes",
		ChaosHubCollection:            "chaosHubs",
		ImageRegistryCollection:       "imageRegistry",
		ServerConfigCollection:        "serverConfig",
		GitOpsCollection:              "gitops",
		UserCollection:                "user",
		ProjectCollection:             "project",
		EnvironmentCollection:         "environment",
		AgentRegistryCollection:       "agentRegistry",
		FaultStudioCollection:         "faultStudios",
		CertificateExperimentsCollection:          "certificate_experiments",
		CertificateRunWorkflowsCollection:         "certificate_run_workflows",
		CertificateAggregationWorkflowsCollection: "certificate_aggregation_workflows",
	}

	DbName            = "litmus"
	ConnectionTimeout = 20 * time.Second
	backgroundContext = context.Background()
)

func MongoConnection() (*mongo.Client, error) {
	var (
		dbServer   = utils.Config.DbServer
		dbUser     = utils.Config.DbUser
		dbPassword = utils.Config.DbPassword
	)

	if dbServer == "" {
		return nil, errors.New("DB_SERVER configuration is required")
	}

	clientOptions := options.Client().ApplyURI(dbServer)
	
	// Only set auth if credentials are provided
	if dbUser != "" && dbPassword != "" {
		credential := options.Credential{
			Username: dbUser,
			Password: dbPassword,
		}
		clientOptions = clientOptions.SetAuth(credential)
	}

	client, err := mongo.Connect(backgroundContext, clientOptions)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(backgroundContext, ConnectionTimeout)
	defer cancel()

	// Check the connection
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	logrus.Infof("connected to mongo")

	return client, nil
}

// Initialize initializes database connection
func (m *MongoClient) Initialize(client *mongo.Client) *MongoClient {
	m.Database = client.Database(DbName)
	m.initAllCollection()

	return m
}

// initAllCollection initializes all the database collections
func (m *MongoClient) initAllCollection() {
	m.UserCollection = m.Database.Collection(Collections[UserCollection])
	m.ProjectCollection = m.Database.Collection(Collections[ProjectCollection])

	// Initialize chaos infra collection
	err := m.Database.CreateCollection(context.TODO(), Collections[ChaosInfraCollection], nil)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logrus.Info(Collections[ChaosInfraCollection] + "'s collection already exists, continuing with the existing mongo collection")
		} else {
			logrus.WithError(err).Error("failed to create chaosInfrastructures collection")
		}
	}

	m.ChaosInfraCollection = m.Database.Collection(Collections[ChaosInfraCollection])
	_, err = m.ChaosInfraCollection.Indexes().CreateMany(backgroundContext, []mongo.IndexModel{
		{
			Keys: bson.M{
				"infra_id": 1,
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.M{
				"name": 1,
			},
		},
	})
	if err != nil {
		logrus.WithError(err).Error("failed to create indexes for chaosInfrastructures collection")
	}

	// Initialize chaos experiment collection
	err = m.Database.CreateCollection(context.TODO(), Collections[ChaosExperimentCollection], nil)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logrus.Info(Collections[ChaosExperimentCollection] + "'s collection already exists, continuing with the existing mongo collection")
		} else {
			logrus.WithError(err).Error("failed to create chaosExperiments collection")
		}
	}

	m.ChaosExperimentCollection = m.Database.Collection(Collections[ChaosExperimentCollection])
	_, err = m.ChaosExperimentCollection.Indexes().CreateMany(backgroundContext, []mongo.IndexModel{
		{
			Keys: bson.M{
				"experiment_id": 1,
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.M{
				"name": 1,
			},
		},
	})
	if err != nil {
		logrus.WithError(err).Error("failed to create indexes for chaosExperiments collection")
	}

	// Initialize chaos experiment runs collection
	err = m.Database.CreateCollection(context.TODO(), Collections[ChaosExperimentRunsCollection], nil)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logrus.Info(Collections[ChaosExperimentRunsCollection] + "'s collection already exists, continuing with the existing mongo collection")
		} else {
			logrus.WithError(err).Error("failed to create chaosExperimentRuns collection")
		}
	}

	m.ChaosExperimentRunsCollection = m.Database.Collection(Collections[ChaosExperimentRunsCollection])
	_, err = m.ChaosExperimentRunsCollection.Indexes().CreateMany(backgroundContext, []mongo.IndexModel{
		{
			Keys: bson.M{
				"experiment_run_id": 1,
			},
		},
	})
	if err != nil {
		logrus.WithError(err).Fatal("failed to create indexes for chaosExperimentRuns collection")
	}

	// Initialize chaos hubs collection
	err = m.Database.CreateCollection(context.TODO(), Collections[ChaosHubCollection], nil)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logrus.Info(Collections[ChaosHubCollection] + "'s collection already exists, continuing with the existing mongo collection")
		} else {
			logrus.WithError(err).Error("failed to create chaosHubs collection")
		}
	}

	m.ChaosHubCollection = m.Database.Collection(Collections[ChaosHubCollection])
	_, err = m.ChaosHubCollection.Indexes().CreateMany(backgroundContext, []mongo.IndexModel{
		{
			Keys: bson.M{
				"hub_id": 1,
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.M{
				"name": 1,
			},
		},
	})
	if err != nil {
		logrus.WithError(err).Fatal("failed to create indexes for chaosHubs collection")
	}

	m.GitOpsCollection = m.Database.Collection(Collections[GitOpsCollection])
	_, err = m.GitOpsCollection.Indexes().CreateMany(backgroundContext, []mongo.IndexModel{
		{
			Keys: bson.M{
				"project_id": 1,
			},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		logrus.WithError(err).Fatal("Error Creating Index for GitOps Collection")
	}
	m.ImageRegistryCollection = m.Database.Collection(Collections[ImageRegistryCollection])
	_, err = m.ImageRegistryCollection.Indexes().CreateMany(backgroundContext, []mongo.IndexModel{
		{
			Keys: bson.M{
				"project_id": 1,
			},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		logrus.WithError(err).Fatal("Error Creating Index for Image Registry Collection")
	}
	m.ServerConfigCollection = m.Database.Collection(Collections[ServerConfigCollection])
	_, err = m.ServerConfigCollection.Indexes().CreateMany(backgroundContext, []mongo.IndexModel{
		{
			Keys: bson.M{
				"key": 1,
			},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		logrus.WithError(err).Fatal("Error Creating Index for Server Config Collection")
	}
	m.EnvironmentCollection = m.Database.Collection(Collections[EnvironmentCollection])
	_, err = m.EnvironmentCollection.Indexes().CreateMany(backgroundContext, []mongo.IndexModel{
		{
			Keys: bson.M{
				"environment_id": 1,
			},
			Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.D{{
				"is_removed", false,
			}}),
		},
		{
			Keys: bson.M{
				"name": 1,
			},
		},
	})
	if err != nil {
		logrus.WithError(err).Fatal("failed to create indexes for environments collection")
	}
	// Initialize chaos probes collection
	err = m.Database.CreateCollection(context.TODO(), Collections[ChaosProbeCollection], nil)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logrus.Info(Collections[ChaosProbeCollection] + "'s collection already exists, continuing with the existing mongo collection")
		} else {
			logrus.WithError(err).Error("failed to create chaosProbes collection")
		}
	}

	m.ChaosProbeCollection = m.Database.Collection(Collections[ChaosProbeCollection])
	_, err = m.ChaosProbeCollection.Indexes().CreateMany(backgroundContext, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "name", Value: 1},
				{Key: "project_id", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.D{{
				Key: "is_removed", Value: false,
			}}),
		},
	})
	if err != nil {
		logrus.WithError(err).Error("failed to create indexes for chaosProbes collection")
	}

	// Agent Registry Collection
	err = m.Database.CreateCollection(backgroundContext, Collections[AgentRegistryCollection], nil)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logrus.Info(Collections[AgentRegistryCollection] + "'s collection already exists, continuing with the existing mongo collection")
		} else {
			logrus.WithError(err).Error("failed to create agentRegistry collection")
		}
	}

	m.AgentRegistryCollection = m.Database.Collection(Collections[AgentRegistryCollection])
	_, err = m.AgentRegistryCollection.Indexes().CreateMany(backgroundContext, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "agentId", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "projectId", Value: 1},
				{Key: "name", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.D{{
				Key: "status", Value: "REGISTERED",
			}}),
		},
		{
			Keys: bson.D{
				{Key: "projectId", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
			},
		},
	})
	if err != nil {
		logrus.WithError(err).Error("failed to create indexes for agentRegistry collection")
	}

	// Fault Studio Collection
	err = m.Database.CreateCollection(backgroundContext, Collections[FaultStudioCollection], nil)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logrus.Info(Collections[FaultStudioCollection] + "'s collection already exists, continuing with the existing mongo collection")
		} else {
			logrus.WithError(err).Error("failed to create faultStudios collection")
		}
	}

	m.FaultStudioCollection = m.Database.Collection(Collections[FaultStudioCollection])
	_, err = m.FaultStudioCollection.Indexes().CreateMany(backgroundContext, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "studio_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "project_id", Value: 1},
			},
		},
	})
	if err != nil {
		logrus.WithError(err).Error("failed to create indexes for faultStudios collection")
	}

	// Certification workflow collections (poller-based orchestrator).
	// See docs/mongo-collection-certiifcation.md for the data model.
	m.initCertificationCollections()
}

// initCertificationCollections creates the three certification-related
// collections and their indexes.  All operations tolerate "already exists"
// errors so the server can be restarted safely.
func (m *MongoClient) initCertificationCollections() {
	// certificate_experiments
	if err := m.Database.CreateCollection(backgroundContext, Collections[CertificateExperimentsCollection], nil); err != nil &&
		!strings.Contains(err.Error(), "already exists") {
		logrus.WithError(err).Error("failed to create certificate_experiments collection")
	}
	m.CertificateExperimentsCollection = m.Database.Collection(Collections[CertificateExperimentsCollection])
	if _, err := m.CertificateExperimentsCollection.Indexes().CreateMany(backgroundContext, []mongo.IndexModel{
		{Keys: bson.D{{Key: "projectId", Value: 1}, {Key: "experimentId", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "projectId", Value: 1}, {Key: "agentId", Value: 1}, {Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "updatedAt", Value: -1}}},
		{Keys: bson.D{{Key: "latestCertificate.status", Value: 1}, {Key: "updatedAt", Value: -1}}},
	}); err != nil {
		logrus.WithError(err).Error("failed to create indexes for certificate_experiments collection")
	}

	// certificate_run_workflows
	if err := m.Database.CreateCollection(backgroundContext, Collections[CertificateRunWorkflowsCollection], nil); err != nil &&
		!strings.Contains(err.Error(), "already exists") {
		logrus.WithError(err).Error("failed to create certificate_run_workflows collection")
	}
	m.CertificateRunWorkflowsCollection = m.Database.Collection(Collections[CertificateRunWorkflowsCollection])
	if _, err := m.CertificateRunWorkflowsCollection.Indexes().CreateMany(backgroundContext, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "projectId", Value: 1}, {Key: "agentId", Value: 1},
				{Key: "experimentId", Value: 1}, {Key: "experimentRunId", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "bucketing.taskId", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true)},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "bucketing.nextPollAt", Value: 1}}},
		{Keys: bson.D{{Key: "agentId", Value: 1}, {Key: "experimentId", Value: 1}, {Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "updatedAt", Value: -1}}},
	}); err != nil {
		logrus.WithError(err).Error("failed to create indexes for certificate_run_workflows collection")
	}

	// certificate_aggregation_workflows
	if err := m.Database.CreateCollection(backgroundContext, Collections[CertificateAggregationWorkflowsCollection], nil); err != nil &&
		!strings.Contains(err.Error(), "already exists") {
		logrus.WithError(err).Error("failed to create certificate_aggregation_workflows collection")
	}
	m.CertificateAggregationWorkflowsCollection = m.Database.Collection(Collections[CertificateAggregationWorkflowsCollection])
	if _, err := m.CertificateAggregationWorkflowsCollection.Indexes().CreateMany(backgroundContext, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "projectId", Value: 1}, {Key: "agentId", Value: 1},
				{Key: "experimentId", Value: 1}, {Key: "aggregationVersion", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "aggregation.certTaskId", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true)},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "aggregation.nextPollAt", Value: 1}}},
		{Keys: bson.D{{Key: "agentId", Value: 1}, {Key: "experimentId", Value: 1}, {Key: "createdAt", Value: -1}}},
	}); err != nil {
		logrus.WithError(err).Error("failed to create indexes for certificate_aggregation_workflows collection")
	}
}
