package experiment_definition

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Repository defines CRUD operations for experiment definitions.
type Repository interface {
	Create(ctx context.Context, doc *ExperimentDefinitionDoc) error
	GetByName(ctx context.Context, projectID, name string) (*ExperimentDefinitionDoc, error)
	List(ctx context.Context, projectID string, filter ListFilter) ([]*ExperimentDefinitionDoc, error)
	Update(ctx context.Context, projectID, name string, update *ExperimentDefinitionDoc) error
	Delete(ctx context.Context, projectID, name string) error
}

// ListFilter allows filtering the List query.
type ListFilter struct {
	Tags      []string
	TargetApp string
	Status    string
}

type mongoRepository struct {
	collection *mongo.Collection
}

// NewRepository returns a new MongoDB-backed repository.
func NewRepository(db *mongo.Database) Repository {
	coll := db.Collection(CollectionName)

	// Ensure unique index on (name, projectID)
	_, _ = coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{
			{Key: "name", Value: 1},
			{Key: "projectID", Value: 1},
		},
		Options: options.Index().SetUnique(true).SetName("name_projectID_unique"),
	})

	return &mongoRepository{collection: coll}
}

func (r *mongoRepository) Create(ctx context.Context, doc *ExperimentDefinitionDoc) error {
	doc.ID = primitive.NewObjectID()
	doc.CreatedAt = time.Now()
	doc.UpdatedAt = time.Now()
	if doc.Version == "" {
		doc.Version = "1.0.0"
	}
	if doc.Status == "" {
		doc.Status = "DRAFT"
	}
	_, err := r.collection.InsertOne(ctx, doc)
	return err
}

func (r *mongoRepository) GetByName(ctx context.Context, projectID, name string) (*ExperimentDefinitionDoc, error) {
	var doc ExperimentDefinitionDoc
	err := r.collection.FindOne(ctx, bson.M{
		"name":      name,
		"projectID": projectID,
	}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, ErrExperimentNotFound{Name: name}
	}
	return &doc, err
}

func (r *mongoRepository) List(ctx context.Context, projectID string, f ListFilter) ([]*ExperimentDefinitionDoc, error) {
	filter := bson.M{"projectID": projectID}
	if f.TargetApp != "" {
		filter["targetApp.name"] = f.TargetApp
	}
	if f.Status != "" {
		filter["status"] = f.Status
	}

	cursor, err := r.collection.Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []*ExperimentDefinitionDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

func (r *mongoRepository) Update(ctx context.Context, projectID, name string, update *ExperimentDefinitionDoc) error {
	update.UpdatedAt = time.Now()
	_, err := r.collection.ReplaceOne(ctx,
		bson.M{"name": name, "projectID": projectID},
		update,
	)
	return err
}

func (r *mongoRepository) Delete(ctx context.Context, projectID, name string) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"name": name, "projectID": projectID})
	return err
}
