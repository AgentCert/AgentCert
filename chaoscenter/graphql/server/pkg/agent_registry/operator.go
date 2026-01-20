package agent_registry

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Operator handles MongoDB operations for agents
type Operator struct {
	collection *mongo.Collection
}

// NewOperator creates a new Operator with the given MongoDB database
func NewOperator(db *mongo.Database) *Operator {
	return &Operator{
		collection: db.Collection(AgentRegistryCollection),
	}
}

// CreateIndexes creates the necessary indexes for the agent registry collection
func (o *Operator) CreateIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "agentId", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "projectId", Value: 1}, {Key: "name", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}, {Key: "auditInfo.createdAt", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "capabilities", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "projectId", Value: 1}, {Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "tags", Value: 1}},
		},
	}

	_, err := o.collection.Indexes().CreateMany(ctx, indexes)
	return err
}

// CreateAgent creates a new agent in the database
func (o *Operator) CreateAgent(ctx context.Context, agent *Agent) (*Agent, error) {
	agent.ID = primitive.NewObjectID()
	agent.AuditInfo.CreatedAt = time.Now().UTC()
	agent.AuditInfo.UpdatedAt = time.Now().UTC()

	_, err := o.collection.InsertOne(ctx, agent)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, ErrAgentAlreadyExists
		}
		return nil, err
	}

	return agent, nil
}

// GetAgentByID retrieves an agent by its AgentID
func (o *Operator) GetAgentByID(ctx context.Context, agentID string) (*Agent, error) {
	filter := bson.M{
		"agentId": agentID,
		"status":  bson.M{"$ne": AgentStatusDeleted},
	}

	var agent Agent
	err := o.collection.FindOne(ctx, filter).Decode(&agent)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}

	return &agent, nil
}

// GetAgentByProjectAndName retrieves an agent by project ID and name
func (o *Operator) GetAgentByProjectAndName(ctx context.Context, projectID, name string) (*Agent, error) {
	filter := bson.M{
		"projectId": projectID,
		"name":      name,
		"status":    bson.M{"$ne": AgentStatusDeleted},
	}

	var agent Agent
	err := o.collection.FindOne(ctx, filter).Decode(&agent)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}

	return &agent, nil
}

// UpdateAgent updates an existing agent
func (o *Operator) UpdateAgent(ctx context.Context, agentID string, update bson.M) (*Agent, error) {
	update["auditInfo.updatedAt"] = time.Now().UTC()

	filter := bson.M{
		"agentId": agentID,
		"status":  bson.M{"$ne": AgentStatusDeleted},
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var agent Agent
	err := o.collection.FindOneAndUpdate(ctx, filter, bson.M{"$set": update}, opts).Decode(&agent)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}

	return &agent, nil
}

// UpdateAgentStatus updates the status of an agent
func (o *Operator) UpdateAgentStatus(ctx context.Context, agentID string, status AgentStatus, updatedBy string) (*Agent, error) {
	update := bson.M{
		"status":            status,
		"auditInfo.updatedAt": time.Now().UTC(),
	}
	if updatedBy != "" {
		update["auditInfo.updatedBy"] = updatedBy
	}

	return o.UpdateAgent(ctx, agentID, update)
}

// UpdateHealthCheckResult updates the last health check result for an agent
func (o *Operator) UpdateHealthCheckResult(ctx context.Context, agentID string, result *HealthCheckResult, newStatus *AgentStatus) (*Agent, error) {
	update := bson.M{
		"lastHealthCheck":     result,
		"auditInfo.updatedAt": time.Now().UTC(),
	}
	if newStatus != nil {
		update["status"] = *newStatus
	}

	return o.UpdateAgent(ctx, agentID, update)
}

// UpdateLangfuseConfig updates the Langfuse configuration for an agent
func (o *Operator) UpdateLangfuseConfig(ctx context.Context, agentID string, config *LangfuseConfig) (*Agent, error) {
	update := bson.M{
		"langfuseConfig":      config,
		"auditInfo.updatedAt": time.Now().UTC(),
	}

	return o.UpdateAgent(ctx, agentID, update)
}

// DeleteAgent soft-deletes an agent
func (o *Operator) DeleteAgent(ctx context.Context, agentID string, deletedBy string) error {
	update := bson.M{
		"status":              AgentStatusDeleted,
		"auditInfo.updatedAt": time.Now().UTC(),
	}
	if deletedBy != "" {
		update["auditInfo.updatedBy"] = deletedBy
	}

	filter := bson.M{
		"agentId": agentID,
		"status":  bson.M{"$ne": AgentStatusDeleted},
	}

	result, err := o.collection.UpdateOne(ctx, filter, bson.M{"$set": update})
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return ErrAgentNotFound
	}

	return nil
}

// ListAgents lists agents with optional filtering and pagination
func (o *Operator) ListAgents(ctx context.Context, filter ListAgentsFilter, pagination ListAgentsPagination) (*ListAgentsResponse, error) {
	query := bson.M{
		"status": bson.M{"$ne": AgentStatusDeleted},
	}

	if filter.ProjectID != "" {
		query["projectId"] = filter.ProjectID
	}
	if filter.Status != nil {
		query["status"] = *filter.Status
	}
	if len(filter.Capabilities) > 0 {
		query["capabilities"] = bson.M{"$all": filter.Capabilities}
	}
	if filter.Vendor != "" {
		query["vendor"] = filter.Vendor
	}
	if len(filter.Tags) > 0 {
		query["tags"] = bson.M{"$all": filter.Tags}
	}
	if filter.Search != "" {
		query["$or"] = []bson.M{
			{"name": bson.M{"$regex": filter.Search, "$options": "i"}},
			{"description": bson.M{"$regex": filter.Search, "$options": "i"}},
		}
	}

	// Count total
	totalCount, err := o.collection.CountDocuments(ctx, query)
	if err != nil {
		return nil, err
	}

	// Apply pagination
	opts := options.Find().
		SetSort(bson.D{{Key: "auditInfo.createdAt", Value: -1}}).
		SetSkip(int64(pagination.Offset)).
		SetLimit(int64(pagination.Limit))

	cursor, err := o.collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var agents []*Agent
	if err := cursor.All(ctx, &agents); err != nil {
		return nil, err
	}

	return &ListAgentsResponse{
		Agents:     agents,
		TotalCount: totalCount,
	}, nil
}

// GetAgentsByCapabilities finds agents that have all the specified capabilities
func (o *Operator) GetAgentsByCapabilities(ctx context.Context, projectID string, capabilities []string, activeOnly bool) ([]*Agent, error) {
	query := bson.M{
		"projectId":    projectID,
		"capabilities": bson.M{"$all": capabilities},
		"status":       bson.M{"$ne": AgentStatusDeleted},
	}

	if activeOnly {
		query["status"] = AgentStatusActive
	}

	cursor, err := o.collection.Find(ctx, query)
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

// GetAgentsForHealthCheck returns agents that need health checking
func (o *Operator) GetAgentsForHealthCheck(ctx context.Context, olderThan time.Duration) ([]*Agent, error) {
	cutoff := time.Now().UTC().Add(-olderThan)

	query := bson.M{
		"status":              bson.M{"$in": []AgentStatus{AgentStatusActive, AgentStatusValidating, AgentStatusRegistered}},
		"healthCheck.enabled": true,
		"$or": []bson.M{
			{"lastHealthCheck": nil},
			{"lastHealthCheck.timestamp": bson.M{"$lt": cutoff}},
		},
	}

	cursor, err := o.collection.Find(ctx, query)
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

// GetAgentsForLangfuseSync returns agents that need Langfuse sync
func (o *Operator) GetAgentsForLangfuseSync(ctx context.Context) ([]*Agent, error) {
	query := bson.M{
		"status":                bson.M{"$ne": AgentStatusDeleted},
		"langfuseConfig.enabled": true,
		"$or": []bson.M{
			{"langfuseConfig.syncStatus": "PENDING"},
			{"langfuseConfig.syncStatus": "FAILED"},
			{"langfuseConfig.syncStatus": nil},
		},
	}

	cursor, err := o.collection.Find(ctx, query)
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
