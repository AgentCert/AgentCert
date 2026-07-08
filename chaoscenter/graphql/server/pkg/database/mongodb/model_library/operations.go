package model_library_db

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Operations struct {
	collection *mongo.Collection
}

func NewOperations(db *mongo.Database) *Operations {
	return &Operations{collection: db.Collection(CollectionName)}
}

func (o *Operations) InsertModelConfig(ctx context.Context, doc ModelConfigDocument) error {
	if doc.ID.IsZero() {
		doc.ID = primitive.NewObjectID()
	}
	now := time.Now().Unix()
	if doc.CreatedAt == 0 {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now
	if doc.AgentsUsing == nil {
		doc.AgentsUsing = []string{}
	}
	_, err := o.collection.InsertOne(ctx, doc)
	return err
}

func (o *Operations) GetModelConfigByAlias(ctx context.Context, projectID, alias string) (*ModelConfigDocument, error) {
	var doc ModelConfigDocument
	err := o.collection.FindOne(ctx, bson.M{"projectID": projectID, "alias": alias}).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (o *Operations) ListModelConfigsByProject(ctx context.Context, projectID string) ([]ModelConfigDocument, error) {
	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cur, err := o.collection.Find(ctx, bson.M{"projectID": projectID}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []ModelConfigDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

func (o *Operations) UpdateModelConfig(ctx context.Context, projectID, alias string, update bson.D) error {
	update = append(update, bson.E{Key: "updatedAt", Value: time.Now().Unix()})
	res, err := o.collection.UpdateOne(
		ctx,
		bson.M{"projectID": projectID, "alias": alias},
		bson.D{{Key: "$set", Value: update}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("model config '%s' not found for project '%s'", alias, projectID)
	}
	return nil
}

func (o *Operations) DeleteModelConfig(ctx context.Context, projectID, alias string) error {
	res, err := o.collection.DeleteOne(ctx, bson.M{"projectID": projectID, "alias": alias})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return fmt.Errorf("model config '%s' not found for project '%s'", alias, projectID)
	}
	return nil
}

func (o *Operations) AddAgentReference(ctx context.Context, projectID, alias, agentName string) error {
	_, err := o.collection.UpdateOne(
		ctx,
		bson.M{"projectID": projectID, "alias": alias},
		bson.M{"$addToSet": bson.M{"agentsUsing": agentName}},
	)
	return err
}

func (o *Operations) RemoveAgentReference(ctx context.Context, projectID, alias, agentName string) error {
	_, err := o.collection.UpdateOne(
		ctx,
		bson.M{"projectID": projectID, "alias": alias},
		bson.M{"$pull": bson.M{"agentsUsing": agentName}},
	)
	return err
}
