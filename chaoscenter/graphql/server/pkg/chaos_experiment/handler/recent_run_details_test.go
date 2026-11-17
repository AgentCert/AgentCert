package handler

import (
	"testing"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
	dbExperiment "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestRefreshRecentRunDetails(t *testing.T) {
	services := NewMockServices()
	score := 100.0
	cursor, err := mongo.NewCursorFromDocuments([]interface{}{
		bson.D{{"experiment_run_id", "successful"}, {"phase", "Completed"}, {"resiliency_score", score}},
		bson.D{{"experiment_run_id", "failed"}, {"phase", "Error"}},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	services.MongodbOperator.On("Aggregate", mock.Anything, mongodb.ChaosExperimentRunsCollection, mock.Anything, mock.Anything).Return(cursor, nil).Once()

	first := []dbExperiment.ExperimentRunDetail{
		{ExperimentRunID: "successful", Phase: "Error"},
		{ExperimentRunID: "missing", Phase: "Error"},
	}
	second := []dbExperiment.ExperimentRunDetail{{ExperimentRunID: "failed", Phase: "Completed"}}
	if err := services.ChaosExperimentHandler.refreshRecentRunDetails("project", first, second); err != nil {
		t.Fatal(err)
	}
	if first[0].Phase != "Completed" || first[0].ResiliencyScore == nil || *first[0].ResiliencyScore != score {
		t.Fatalf("successful run was not restored: %+v", first[0])
	}
	if first[1].Phase != "Error" {
		t.Fatalf("missing run should keep its cached status: %+v", first[1])
	}
	if second[0].Phase != "Error" {
		t.Fatalf("failed run must retain the canonical failure: %+v", second[0])
	}
	services.MongodbOperator.AssertExpectations(t)
}
