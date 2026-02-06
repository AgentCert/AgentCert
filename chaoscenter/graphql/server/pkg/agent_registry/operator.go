package agent_registry

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Operator defines the data access layer interface for Agent Registry.
// It provides methods for CRUD operations on agent documents in MongoDB.
type Operator interface {
	// CreateAgent inserts a new agent document
	CreateAgent(ctx context.Context, agent *Agent) error

	// GetAgent retrieves an agent by ID
	GetAgent(ctx context.Context, id string) (*Agent, error)

	// GetAgentByProjectAndName retrieves an agent by project and name (for uniqueness check)
	GetAgentByProjectAndName(ctx context.Context, projectID, name string) (*Agent, error)

	// ListAgents retrieves agents with filtering and pagination
	ListAgents(ctx context.Context, filter *AgentFilter, skip, limit int) ([]*Agent, int64, error)

	// UpdateAgent updates an existing agent document
	UpdateAgent(ctx context.Context, agent *Agent) error

	// DeleteAgent removes an agent document
	DeleteAgent(ctx context.Context, id string) error

	// GetAgentsByCapabilities retrieves agents that have all specified capabilities
	GetAgentsByCapabilities(ctx context.Context, projectID string, capabilities []string) ([]*Agent, error)
}

// operatorImpl is the concrete implementation of the Operator interface.
type operatorImpl struct {
	collection *mongo.Collection
}

// NewOperator creates a new Operator instance.
func NewOperator(database *mongo.Database) Operator {
	return &operatorImpl{
		collection: database.Collection("agentRegistry"),
	}
}

// CreateAgent inserts a new agent document into MongoDB.
func (o *operatorImpl) CreateAgent(ctx context.Context, agent *Agent) error {
	_, err := o.collection.InsertOne(ctx, agent)
	if err != nil {
		return err
	}
	return nil
}

// GetAgent retrieves an agent by ID from MongoDB.
func (o *operatorImpl) GetAgent(ctx context.Context, id string) (*Agent, error) {
	filter := bson.M{"agentId": id}

	var agent Agent
	err := o.collection.FindOne(ctx, filter).Decode(&agent)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}

	return &agent, nil
}

// GetAgentByProjectAndName retrieves an agent by project ID and name.
func (o *operatorImpl) GetAgentByProjectAndName(ctx context.Context, projectID, name string) (*Agent, error) {
	filter := bson.M{
		"projectId": projectID,
		"name":      name,
	}

	var agent Agent
	err := o.collection.FindOne(ctx, filter).Decode(&agent)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}

	return &agent, nil
}

// ListAgents retrieves agents with filtering and pagination.
func (o *operatorImpl) ListAgents(ctx context.Context, filter *AgentFilter, skip, limit int) ([]*Agent, int64, error) {
	// Build MongoDB filter
	mongoFilter := bson.M{}

	if filter != nil {
		// Filter by projectId
		if filter.ProjectID != "" {
			mongoFilter["projectId"] = filter.ProjectID
		}

		// Filter by status (single)
		if filter.Status != nil {
			mongoFilter["status"] = *filter.Status
		}

		// Filter by statuses (multiple) - takes precedence over single status
		if len(filter.Statuses) > 0 {
			mongoFilter["status"] = bson.M{"$in": filter.Statuses}
		}

		// Filter by capabilities (must have ALL capabilities)
		if len(filter.Capabilities) > 0 {
			mongoFilter["capabilities"] = bson.M{"$all": filter.Capabilities}
		}

		// Filter by searchTerm (searches in name and vendor)
		if filter.SearchTerm != nil && *filter.SearchTerm != "" {
			searchTerm := *filter.SearchTerm
			mongoFilter["$or"] = []bson.M{
				{"name": bson.M{"$regex": searchTerm, "$options": "i"}},
				{"vendor": bson.M{"$regex": searchTerm, "$options": "i"}},
			}
		}
	}

	// Get total count
	totalCount, err := o.collection.CountDocuments(ctx, mongoFilter)
	if err != nil {
		return nil, 0, err
	}

	// Apply pagination
	findOptions := options.Find()
	findOptions.SetSkip(int64(skip))
	findOptions.SetLimit(int64(limit))
	findOptions.SetSort(bson.D{{Key: "auditInfo.createdAt", Value: -1}})

	// Execute query
	cursor, err := o.collection.Find(ctx, mongoFilter, findOptions)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	// Decode results
	var agents []*Agent
	if err := cursor.All(ctx, &agents); err != nil {
		return nil, 0, err
	}

	return agents, totalCount, nil
}

// UpdateAgent updates an existing agent document.
func (o *operatorImpl) UpdateAgent(ctx context.Context, agent *Agent) error {
	filter := bson.M{"agentId": agent.AgentID}

	result, err := o.collection.ReplaceOne(ctx, filter, agent)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return ErrAgentNotFound
	}

	return nil
}

// DeleteAgent removes an agent document from MongoDB.
func (o *operatorImpl) DeleteAgent(ctx context.Context, id string) error {
	filter := bson.M{"agentId": id}

	result, err := o.collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return ErrAgentNotFound
	}

	return nil
}

// GetAgentsByCapabilities retrieves agents that have all specified capabilities.
func (o *operatorImpl) GetAgentsByCapabilities(ctx context.Context, projectID string, capabilities []string) ([]*Agent, error) {
	// Build filter with $all operator for AND logic
	filter := bson.M{
		"projectId":    projectID,
		"capabilities": bson.M{"$all": capabilities},
	}

	// Use index hint to optimize query
	findOptions := options.Find()
	findOptions.SetHint(bson.D{{Key: "capabilities", Value: 1}})
	findOptions.SetSort(bson.D{{Key: "name", Value: 1}})

	cursor, err := o.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var agents []*Agent
	if err := cursor.All(ctx, &agents); err != nil {
		return nil, err
	}

	return agents, nil
}
