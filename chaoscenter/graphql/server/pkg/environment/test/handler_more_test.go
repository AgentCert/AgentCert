package test

import (
	"context"
	"errors"
	"testing"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/environments"
	dbOperationsEnvironment "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/environments"
	dbMocks "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/mocks"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/environment/handler"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// newEnvService returns a freshly mocked operator so expectations are isolated per test.
func newEnvService() (*dbMocks.MongoOperator, handler.EnvironmentHandler) {
	m := new(dbMocks.MongoOperator)
	op := dbOperationsEnvironment.NewEnvironmentOperator(m)
	return m, handler.NewEnvironmentService(op)
}

func strptr(s string) *string { return &s }

func TestCreateEnvironment_Behavior(t *testing.T) {
	t.Run("success returns populated model and defaults tags", func(t *testing.T) {
		m, svc := newEnvService()
		m.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("environments.Environment")).Return(nil)

		input := &model.CreateEnvironmentRequest{
			EnvironmentID: "env-1",
			Name:          "prod-env",
			Type:          model.EnvironmentTypeProd,
			Description:   strptr("my desc"),
		}
		env, err := svc.CreateEnvironment(context.Background(), "proj-1", input, "alice")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if env.EnvironmentID != "env-1" || env.Name != "prod-env" || env.ProjectID != "proj-1" {
			t.Errorf("unexpected env: %+v", env)
		}
		if env.Tags == nil || len(env.Tags) != 0 {
			t.Errorf("expected empty (non-nil) tags, got %v", env.Tags)
		}
	})

	t.Run("insert error is propagated", func(t *testing.T) {
		m, svc := newEnvService()
		m.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("insert failed"))

		_, err := svc.CreateEnvironment(context.Background(), "proj", &model.CreateEnvironmentRequest{
			EnvironmentID: "e", Name: "n", Type: model.EnvironmentTypeNonProd,
		}, "bob")
		if err == nil || err.Error() != "insert failed" {
			t.Fatalf("expected insert failed error, got %v", err)
		}
	})
}

func TestGetEnvironment_Behavior(t *testing.T) {
	t.Run("success maps db record to model", func(t *testing.T) {
		m, svc := newEnvService()
		doc := bson.D{
			{Key: "environment_id", Value: "env-9"},
			{Key: "project_id", Value: "proj-9"},
			{Key: "name", Value: "staging"},
			{Key: "type", Value: "non_prod"},
		}
		m.On("Get", mock.Anything, mock.Anything, mock.Anything).
			Return(mongo.NewSingleResultFromDocument(doc, nil, nil), nil)

		env, err := svc.GetEnvironment("proj-9", "env-9")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if env.EnvironmentID != "env-9" || env.Name != "staging" {
			t.Errorf("unexpected env: %+v", env)
		}
		if env.Type != model.EnvironmentType("non_prod") {
			t.Errorf("type = %v", env.Type)
		}
	})

	t.Run("decode error propagates", func(t *testing.T) {
		m, svc := newEnvService()
		m.On("Get", mock.Anything, mock.Anything, mock.Anything).
			Return(mongo.NewSingleResultFromDocument(bson.D{}, errors.New("no docs"), nil), nil)

		_, err := svc.GetEnvironment("p", "e")
		if err == nil {
			t.Fatal("expected decode error")
		}
	})
}

func TestUpdateEnvironment_Behavior(t *testing.T) {
	t.Run("success with all fields", func(t *testing.T) {
		m, svc := newEnvService()
		// GetEnvironments uses List -> Aggregate? No, it uses List.
		cur, _ := mongo.NewCursorFromDocuments([]interface{}{
			bson.D{{Key: "environment_id", Value: "env-1"}},
		}, nil, nil)
		m.On("List", mock.Anything, mock.Anything, mock.Anything).Return(cur, nil)
		m.On("UpdateMany", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(&mongo.UpdateResult{}, nil)

		typ := model.EnvironmentTypeProd
		req := &model.UpdateEnvironmentRequest{
			EnvironmentID: "env-1",
			Name:          strptr("renamed"),
			Description:   strptr("new desc"),
			Tags:          []*string{strptr("a"), strptr("b")},
			Type:          &typ,
		}
		msg, err := svc.UpdateEnvironment(context.Background(), "proj-1", req, "carol")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if msg != "environment updated successfully" {
			t.Errorf("msg = %q", msg)
		}
	})

	t.Run("get environments error returns failure message", func(t *testing.T) {
		m, svc := newEnvService()
		m.On("List", mock.Anything, mock.Anything, mock.Anything).
			Return((*mongo.Cursor)(nil), errors.New("list failed"))

		msg, err := svc.UpdateEnvironment(context.Background(), "proj", &model.UpdateEnvironmentRequest{
			EnvironmentID: "x",
		}, "u")
		if err == nil {
			t.Fatal("expected error")
		}
		if msg != "couldn't update environment" {
			t.Errorf("msg = %q", msg)
		}
	})

	t.Run("update error returns failure message", func(t *testing.T) {
		m, svc := newEnvService()
		cur, _ := mongo.NewCursorFromDocuments([]interface{}{bson.D{}}, nil, nil)
		m.On("List", mock.Anything, mock.Anything, mock.Anything).Return(cur, nil)
		m.On("UpdateMany", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return((*mongo.UpdateResult)(nil), errors.New("update failed"))

		msg, err := svc.UpdateEnvironment(context.Background(), "proj", &model.UpdateEnvironmentRequest{
			EnvironmentID: "x",
		}, "u")
		if err == nil || msg != "couldn't update environment" {
			t.Fatalf("expected update failure, got msg=%q err=%v", msg, err)
		}
	})
}

func TestDeleteEnvironment_Errors(t *testing.T) {
	t.Run("get error returns fetch failure", func(t *testing.T) {
		m, svc := newEnvService()
		m.On("Get", mock.Anything, mock.Anything, mock.Anything).
			Return(mongo.NewSingleResultFromDocument(bson.D{}, errors.New("not found"), nil), nil)

		msg, err := svc.DeleteEnvironment(context.Background(), "proj", "env", "u")
		if err == nil || msg != "couldn't fetch environment details" {
			t.Fatalf("expected fetch failure, got msg=%q err=%v", msg, err)
		}
	})

	t.Run("update error returns delete failure", func(t *testing.T) {
		m, svc := newEnvService()
		m.On("Get", mock.Anything, mock.Anything, mock.Anything).
			Return(mongo.NewSingleResultFromDocument(bson.D{{Key: "environment_id", Value: "e"}}, nil, nil), nil)
		m.On("UpdateMany", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return((*mongo.UpdateResult)(nil), errors.New("update failed"))

		msg, err := svc.DeleteEnvironment(context.Background(), "proj", "e", "u")
		if err == nil || msg != "couldn't delete environment" {
			t.Fatalf("expected delete failure, got msg=%q err=%v", msg, err)
		}
	})

	t.Run("success", func(t *testing.T) {
		m, svc := newEnvService()
		m.On("Get", mock.Anything, mock.Anything, mock.Anything).
			Return(mongo.NewSingleResultFromDocument(bson.D{{Key: "environment_id", Value: "e"}}, nil, nil), nil)
		m.On("UpdateMany", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(&mongo.UpdateResult{}, nil)

		msg, err := svc.DeleteEnvironment(context.Background(), "proj", "e", "u")
		if err != nil || msg != "successfully deleted environment" {
			t.Fatalf("expected success, got msg=%q err=%v", msg, err)
		}
	})
}

func TestListEnvironments_Behavior(t *testing.T) {
	t.Run("aggregate error propagates", func(t *testing.T) {
		m, svc := newEnvService()
		m.On("Aggregate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return((*mongo.Cursor)(nil), errors.New("agg failed"))

		_, err := svc.ListEnvironments("proj", nil)
		if err == nil {
			t.Fatal("expected aggregate error")
		}
	})

	t.Run("success returns environments and count with filters and sort", func(t *testing.T) {
		m, svc := newEnvService()
		agg := environments.AggregatedEnvironments{
			TotalFilteredEnvironments: []environments.TotalFilteredData{{Count: 1}},
			Environments: []environments.Environment{
				{
					EnvironmentID: "env-1",
					ProjectID:     "proj",
					Type:          environments.Prod,
				},
			},
		}
		cur, err := mongo.NewCursorFromDocuments([]interface{}{agg}, nil, nil)
		if err != nil {
			t.Fatalf("cursor build: %v", err)
		}
		m.On("Aggregate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(cur, nil)

		asc := true
		req := &model.ListEnvironmentRequest{
			EnvironmentIDs: []string{"env-1"},
			Filter: &model.EnvironmentFilterInput{
				Name:        strptr("env"),
				Description: strptr("d"),
				Tags:        []string{"t1"},
			},
			Sort: &model.EnvironmentSortInput{
				Field:     model.EnvironmentSortingFieldName,
				Ascending: &asc,
			},
			Pagination: &model.Pagination{Page: 0, Limit: 10},
		}
		resp, err := svc.ListEnvironments("proj", req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.TotalNoOfEnvironments != 1 {
			t.Errorf("total = %d, want 1", resp.TotalNoOfEnvironments)
		}
		if len(resp.Environments) != 1 || resp.Environments[0].EnvironmentID != "env-1" {
			t.Errorf("unexpected environments: %+v", resp.Environments)
		}
	})
}
