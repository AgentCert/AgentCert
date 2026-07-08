package agent_registry_db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AgentFilter struct {
	ProjectID  string
	IsDeleted  *bool
	SearchTerm *string
}

type Pagination struct {
	Skip  int64
	Limit int64
}

type Operations struct {
	collection *mongo.Collection
}

func NewOperations(db *mongo.Database) *Operations {
	return &Operations{collection: db.Collection(CollectionName)}
}

func (o *Operations) InsertAgent(ctx context.Context, doc AgentDocument) error {
	if doc.ID.IsZero() {
		doc.ID = primitive.NewObjectID()
	}
	now := time.Now().Unix()
	if doc.CreatedAt == 0 {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now
	_, err := o.collection.InsertOne(ctx, doc)
	return err
}

func (o *Operations) GetAgentByID(ctx context.Context, agentID string) (*AgentDocument, error) {
	var doc AgentDocument
	err := o.collection.FindOne(ctx, bson.M{"agentID": agentID, "isDeleted": bson.M{"$ne": true}}).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (o *Operations) GetAgentByProjectAndName(ctx context.Context, projectID, name string) (*AgentDocument, error) {
	var doc AgentDocument
	err := o.collection.FindOne(ctx, bson.M{
		"projectID": projectID,
		"name":      name,
		"isDeleted": bson.M{"$ne": true},
	}).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (o *Operations) ListAgentsByProject(ctx context.Context, projectID string, filter AgentFilter, pg Pagination) ([]AgentDocument, int64, error) {
	query := bson.M{"projectID": projectID, "isDeleted": bson.M{"$ne": true}}
	if filter.SearchTerm != nil && *filter.SearchTerm != "" {
		query["$or"] = []bson.M{
			{"name": bson.M{"$regex": *filter.SearchTerm, "$options": "i"}},
			{"displayName": bson.M{"$regex": *filter.SearchTerm, "$options": "i"}},
		}
	}
	total, err := o.collection.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSkip(pg.Skip).SetLimit(pg.Limit).SetSort(bson.M{"createdAt": -1})
	cur, err := o.collection.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var docs []AgentDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, 0, err
	}
	return docs, total, nil
}

func (o *Operations) UpdateAgent(ctx context.Context, agentID string, update bson.D) error {
	update = append(update, bson.E{Key: "updatedAt", Value: time.Now().Unix()})
	_, err := o.collection.UpdateOne(ctx, bson.M{"agentID": agentID}, bson.D{{Key: "$set", Value: update}})
	return err
}

func (o *Operations) SoftDeleteAgent(ctx context.Context, agentID string) error {
	return o.UpdateAgent(ctx, agentID, bson.D{{Key: "isDeleted", Value: true}})
}
