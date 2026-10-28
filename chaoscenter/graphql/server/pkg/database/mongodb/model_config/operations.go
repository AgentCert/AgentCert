package model_config

import (
	"context"
	"errors"
	"time"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ErrDuplicateAlias is returned by Insert when the alias is already recorded,
// including by a deleted model: aliases are never reused.
var ErrDuplicateAlias = errors.New("model alias already exists")

// Guard is the state a row must still be in for an update to apply, so a
// caller that lost a race to a concurrent change updates nothing.
type Guard struct {
	// Statuses the row may have; empty allows any.
	Statuses []SyncStatus
	// UpdatedAt the row must still carry; 0 skips the check.
	UpdatedAt int64
}

// Changes are the fields an update sets. Zero values leave a field alone.
type Changes struct {
	Status    SyncStatus
	LastError *string
	KeyHint   *string
	// Removed soft-deletes the row. Its alias stays reserved.
	Removed bool
	// By is the username recorded as updated_by.
	By string
}

// Operator is used to perform operations on the modelConfig collection
type Operator struct {
	operator mongodb.MongoOperator
}

// NewModelConfigOperator returns a new instance of Operator
func NewModelConfigOperator(mongodbOperator mongodb.MongoOperator) *Operator {
	return &Operator{
		operator: mongodbOperator,
	}
}

// Insert records a new model. A taken alias returns ErrDuplicateAlias.
func (o *Operator) Insert(ctx context.Context, modelConfig ModelConfig) error {
	err := o.operator.Create(ctx, mongodb.ModelConfigCollection, modelConfig)
	if mongo.IsDuplicateKeyError(err) {
		return ErrDuplicateAlias
	}
	return err
}

// Get returns the live (not deleted) model with this ID, or nil.
func (o *Operator) Get(ctx context.Context, modelID string) (*ModelConfig, error) {
	return o.getOne(ctx, bson.D{{"model_id", modelID}, {"is_removed", false}})
}

// GetByAlias returns the model recorded under alias, deleted or not, or nil.
func (o *Operator) GetByAlias(ctx context.Context, alias string) (*ModelConfig, error) {
	return o.getOne(ctx, bson.D{{"alias", alias}})
}

// ListByProject returns a project's live models, oldest first, optionally
// only those in one of statuses.
func (o *Operator) ListByProject(ctx context.Context, projectID string, statuses ...SyncStatus) ([]ModelConfig, error) {
	query := bson.D{{"project_id", projectID}, {"is_removed", false}}
	if len(statuses) > 0 {
		query = append(query, bson.E{Key: "sync_status", Value: bson.D{{"$in", statuses}}})
	}
	return o.list(ctx, query)
}

// ListLive returns every live model of every project.
func (o *Operator) ListLive(ctx context.Context) ([]ModelConfig, error) {
	return o.list(ctx, bson.D{{"is_removed", false}})
}

// Update applies changes to the live model with this ID while guard holds,
// and reports whether it did.
func (o *Operator) Update(ctx context.Context, modelID string, guard Guard, changes Changes) (bool, error) {
	set := bson.D{{"updated_at", time.Now().UnixMilli()}}
	if changes.Status != "" {
		set = append(set, bson.E{Key: "sync_status", Value: changes.Status})
	}
	if changes.LastError != nil {
		set = append(set, bson.E{Key: "last_error", Value: *changes.LastError})
	}
	if changes.KeyHint != nil {
		set = append(set, bson.E{Key: "api_key_hint", Value: *changes.KeyHint})
	}
	if changes.Removed {
		set = append(set, bson.E{Key: "is_removed", Value: true})
	}
	if changes.By != "" {
		set = append(set, bson.E{Key: "updated_by", Value: mongodb.UserDetailResponse{Username: changes.By}})
	}
	result, err := o.operator.Update(ctx, mongodb.ModelConfigCollection, guard.filter(modelID), bson.D{{"$set", set}})
	if err != nil {
		return false, err
	}
	return result.MatchedCount > 0, nil
}

// Remove hard-deletes the live model with this ID while guard holds. Only a
// model LiteLLM never served is removed this way, which frees its alias.
func (o *Operator) Remove(ctx context.Context, modelID string, guard Guard) (bool, error) {
	result, err := o.operator.Delete(ctx, mongodb.ModelConfigCollection, guard.filter(modelID))
	if err != nil {
		return false, err
	}
	return result.DeletedCount > 0, nil
}

func (g Guard) filter(modelID string) bson.D {
	filter := bson.D{{"model_id", modelID}, {"is_removed", false}}
	if len(g.Statuses) > 0 {
		filter = append(filter, bson.E{Key: "sync_status", Value: bson.D{{"$in", g.Statuses}}})
	}
	if g.UpdatedAt != 0 {
		filter = append(filter, bson.E{Key: "updated_at", Value: g.UpdatedAt})
	}
	return filter
}

func (o *Operator) getOne(ctx context.Context, query bson.D) (*ModelConfig, error) {
	result, err := o.operator.Get(ctx, mongodb.ModelConfigCollection, query)
	if err != nil {
		return nil, err
	}
	var modelConfig ModelConfig
	if err := result.Decode(&modelConfig); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &modelConfig, nil
}

func (o *Operator) list(ctx context.Context, query bson.D) ([]ModelConfig, error) {
	cursor, err := o.operator.List(ctx, mongodb.ModelConfigCollection, query, options.Find().SetSort(bson.D{{"created_at", 1}}))
	if err != nil {
		return nil, err
	}
	modelConfigs := []ModelConfig{}
	if err := cursor.All(ctx, &modelConfigs); err != nil {
		return nil, err
	}
	return modelConfigs, nil
}
