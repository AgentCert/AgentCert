package experiment_definition

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// RunRepository defines CRUD for experiment run records.
type RunRepository interface {
	Create(ctx context.Context, doc *AceExperimentRunDoc) error
	GetByID(ctx context.Context, runID string) (*AceExperimentRunDoc, error)
	UpdateStatus(ctx context.Context, runID string, status RunStatus, reason string) error
	UpdateCertifierFields(ctx context.Context, runID string, langfuseTraceID, certifierReportID string) error
	List(ctx context.Context, filter RunListFilter) ([]*AceExperimentRunDoc, error)
}

// RunListFilter filters the List query.
type RunListFilter struct {
	ProjectID      string
	DefinitionName string
	AgentName      string
	Status         RunStatus
}

type mongoRunRepository struct {
	collection *mongo.Collection
}

// NewRunRepository returns a MongoDB-backed RunRepository.
func NewRunRepository(db *mongo.Database) RunRepository {
	coll := db.Collection(RunCollectionName)

	// Create indexes
	_, _ = coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "runID", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("runID_unique"),
		},
		{
			Keys: bson.D{
				{Key: "definitionName", Value: 1},
				{Key: "status", Value: 1},
			},
			Options: options.Index().SetName("definitionName_status"),
		},
		{
			Keys:    bson.D{{Key: "agentName", Value: 1}},
			Options: options.Index().SetName("agentName"),
		},
	})

	return &mongoRunRepository{collection: coll}
}

func (r *mongoRunRepository) Create(ctx context.Context, doc *AceExperimentRunDoc) error {
	doc.ID = primitive.NewObjectID()
	doc.CreatedAt = time.Now()
	if doc.Status == "" {
		doc.Status = RunStatusQueued
	}
	doc.StatusHistory = append(doc.StatusHistory, StatusEvent{
		Status:    doc.Status,
		Timestamp: doc.CreatedAt,
	})
	_, err := r.collection.InsertOne(ctx, doc)
	return err
}

func (r *mongoRunRepository) GetByID(ctx context.Context, runID string) (*AceExperimentRunDoc, error) {
	var doc AceExperimentRunDoc
	err := r.collection.FindOne(ctx, bson.M{"runID": runID}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, ErrRunNotFound{RunID: runID}
	}
	return &doc, err
}

func (r *mongoRunRepository) UpdateStatus(ctx context.Context, runID string, status RunStatus, reason string) error {
	now := time.Now()
	setFields := bson.M{"status": status}
	switch status {
	case RunStatusRunning:
		setFields["startedAt"] = now
	case RunStatusCompleted, RunStatusFailed, RunStatusAborted:
		setFields["completedAt"] = now
	}
	update := bson.M{
		"$set": setFields,
		"$push": bson.M{
			"statusHistory": StatusEvent{
				Status:    status,
				Timestamp: now,
				Reason:    reason,
			},
		},
	}
	_, err := r.collection.UpdateOne(ctx, bson.M{"runID": runID}, update)
	return err
}

func (r *mongoRunRepository) UpdateCertifierFields(ctx context.Context, runID, langfuseTraceID, certifierReportID string) error {
	_, err := r.collection.UpdateOne(ctx,
		bson.M{"runID": runID},
		bson.M{"$set": bson.M{
			"langfuseTraceId":   langfuseTraceID,
			"certifierReportId": certifierReportID,
		}},
	)
	return err
}

func (r *mongoRunRepository) List(ctx context.Context, f RunListFilter) ([]*AceExperimentRunDoc, error) {
	filter := bson.M{}
	if f.ProjectID != "" {
		filter["projectID"] = f.ProjectID
	}
	if f.DefinitionName != "" {
		filter["definitionName"] = f.DefinitionName
	}
	if f.AgentName != "" {
		filter["agentName"] = f.AgentName
	}
	if f.Status != "" {
		filter["status"] = f.Status
	}

	cursor, err := r.collection.Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(100))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []*AceExperimentRunDoc
	return docs, cursor.All(ctx, &docs)
}
