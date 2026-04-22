package fault_studio

import (
	"context"
	"fmt"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Operator is the database operations handler for FaultStudio
type Operator struct {
	operator mongodb.MongoOperator
}

// NewFaultStudioOperator creates a new FaultStudio operator
func NewFaultStudioOperator(mongodbOperator mongodb.MongoOperator) *Operator {
	return &Operator{
		operator: mongodbOperator,
	}
}

// CreateFaultStudio creates a new fault studio in the database
func (o *Operator) CreateFaultStudio(ctx context.Context, faultStudio *FaultStudio) error {
	err := o.operator.Create(ctx, mongodb.FaultStudioCollection, faultStudio)
	if err != nil {
		return fmt.Errorf("error creating fault studio: %v", err)
	}
	return nil
}

// GetFaultStudioByID returns a fault studio by its ID and project ID
func (o *Operator) GetFaultStudioByID(ctx context.Context, studioID string, projectID string) (*FaultStudio, error) {
	var faultStudio FaultStudio
	query := bson.D{
		{Key: "studio_id", Value: studioID},
		{Key: "project_id", Value: projectID},
		{Key: "is_removed", Value: false},
	}

	result, err := o.operator.Get(ctx, mongodb.FaultStudioCollection, query)
	if err != nil {
		return nil, fmt.Errorf("error getting fault studio: %v", err)
	}

	err = result.Decode(&faultStudio)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("fault studio not found")
		}
		return nil, fmt.Errorf("error decoding fault studio: %v", err)
	}

	return &faultStudio, nil
}

// GetFaultStudiosByProjectID returns all fault studios for a project
func (o *Operator) GetFaultStudiosByProjectID(ctx context.Context, projectID string) ([]FaultStudio, error) {
	query := bson.D{
		{Key: "project_id", Value: projectID},
		{Key: "is_removed", Value: false},
	}

	results, err := o.operator.List(ctx, mongodb.FaultStudioCollection, query)
	if err != nil {
		return nil, fmt.Errorf("error listing fault studios: %v", err)
	}

	var faultStudios []FaultStudio
	err = results.All(ctx, &faultStudios)
	if err != nil {
		return nil, fmt.Errorf("error decoding fault studios: %v", err)
	}

	return faultStudios, nil
}

// GetFaultStudiosByHubID returns all fault studios that reference a specific ChaosHub
func (o *Operator) GetFaultStudiosByHubID(ctx context.Context, hubID string, projectID string) ([]FaultStudio, error) {
	query := bson.D{
		{Key: "source_hub_id", Value: hubID},
		{Key: "project_id", Value: projectID},
		{Key: "is_removed", Value: false},
	}

	results, err := o.operator.List(ctx, mongodb.FaultStudioCollection, query)
	if err != nil {
		return nil, fmt.Errorf("error listing fault studios by hub: %v", err)
	}

	var faultStudios []FaultStudio
	err = results.All(ctx, &faultStudios)
	if err != nil {
		return nil, fmt.Errorf("error decoding fault studios: %v", err)
	}

	return faultStudios, nil
}

// UpdateFaultStudio updates an existing fault studio
func (o *Operator) UpdateFaultStudio(ctx context.Context, query bson.D, update bson.D) error {
	updateResult, err := o.operator.Update(ctx, mongodb.FaultStudioCollection, query, update)
	if err != nil {
		return fmt.Errorf("error updating fault studio: %v", err)
	}

	if updateResult.MatchedCount == 0 {
		return fmt.Errorf("fault studio not found")
	}

	return nil
}

// DeleteFaultStudio soft deletes a fault studio by setting is_removed to true
func (o *Operator) DeleteFaultStudio(ctx context.Context, studioID string, projectID string, updatedBy mongodb.UserDetailResponse) error {
	query := bson.D{
		{Key: "studio_id", Value: studioID},
		{Key: "project_id", Value: projectID},
	}

	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "is_removed", Value: true},
			{Key: "updated_by", Value: updatedBy},
		}},
	}

	updateResult, err := o.operator.Update(ctx, mongodb.FaultStudioCollection, query, update)
	if err != nil {
		return fmt.Errorf("error deleting fault studio: %v", err)
	}

	if updateResult.MatchedCount == 0 {
		return fmt.Errorf("fault studio not found")
	}

	return nil
}

// GetActiveFaultStudios returns all active fault studios for a project
func (o *Operator) GetActiveFaultStudios(ctx context.Context, projectID string) ([]FaultStudio, error) {
	query := bson.D{
		{Key: "project_id", Value: projectID},
		{Key: "is_active", Value: true},
		{Key: "is_removed", Value: false},
	}

	results, err := o.operator.List(ctx, mongodb.FaultStudioCollection, query)
	if err != nil {
		return nil, fmt.Errorf("error listing active fault studios: %v", err)
	}

	var faultStudios []FaultStudio
	err = results.All(ctx, &faultStudios)
	if err != nil {
		return nil, fmt.Errorf("error decoding fault studios: %v", err)
	}

	return faultStudios, nil
}

// IsFaultStudioNameUnique checks if a fault studio name is unique within a project
func (o *Operator) IsFaultStudioNameUnique(ctx context.Context, name string, projectID string, excludeStudioID string) (bool, error) {
	query := bson.D{
		{Key: "name", Value: name},
		{Key: "project_id", Value: projectID},
		{Key: "is_removed", Value: false},
	}

	// Exclude the current studio if updating
	if excludeStudioID != "" {
		query = append(query, bson.E{Key: "studio_id", Value: bson.D{{Key: "$ne", Value: excludeStudioID}}})
	}

	results, err := o.operator.List(ctx, mongodb.FaultStudioCollection, query)
	if err != nil {
		return false, fmt.Errorf("error checking name uniqueness: %v", err)
	}

	var existingStudios []FaultStudio
	err = results.All(ctx, &existingStudios)
	if err != nil {
		return false, fmt.Errorf("error decoding results: %v", err)
	}

	return len(existingStudios) == 0, nil
}

// GetAggregateFaultStudios runs an aggregation pipeline on the fault studio collection
func (o *Operator) GetAggregateFaultStudios(ctx context.Context, pipeline mongo.Pipeline) (*mongo.Cursor, error) {
	results, err := mongodb.Operator.Aggregate(ctx, mongodb.FaultStudioCollection, pipeline)
	if err != nil {
		return nil, fmt.Errorf("error aggregating fault studios: %v", err)
	}
	return results, nil
}

// GetFaultStudioStats returns statistics about fault studios for a project
func (o *Operator) GetFaultStudioStats(ctx context.Context, projectID string) (*AggregatedFaultStudioStats, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{
			{Key: "project_id", Value: projectID},
			{Key: "is_removed", Value: false},
		}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
	}

	cursor, err := o.GetAggregateFaultStudios(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var stats AggregatedFaultStudioStats
	var results []TotalCount
	if err = cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("error decoding stats: %v", err)
	}

	stats.TotalFaultStudios = results
	return &stats, nil
}

// ListFaultStudiosWithPagination returns fault studios with pagination support
func (o *Operator) ListFaultStudiosWithPagination(ctx context.Context, projectID string, limit int64, offset int64) ([]FaultStudio, int64, error) {
	query := bson.D{
		{Key: "project_id", Value: projectID},
		{Key: "is_removed", Value: false},
	}

	// Get total count
	count, err := o.operator.CountDocuments(ctx, mongodb.FaultStudioCollection, query)
	if err != nil {
		return nil, 0, fmt.Errorf("error counting fault studios: %v", err)
	}

	// Build find options with pagination
	findOptions := options.Find()
	findOptions.SetLimit(limit)
	findOptions.SetSkip(offset)
	findOptions.SetSort(bson.D{{Key: "updated_at", Value: -1}}) // Sort by most recently updated

	results, err := o.operator.List(ctx, mongodb.FaultStudioCollection, query)
	if err != nil {
		return nil, 0, fmt.Errorf("error listing fault studios: %v", err)
	}

	var faultStudios []FaultStudio
	err = results.All(ctx, &faultStudios)
	if err != nil {
		return nil, 0, fmt.Errorf("error decoding fault studios: %v", err)
	}

	return faultStudios, count, nil
}
