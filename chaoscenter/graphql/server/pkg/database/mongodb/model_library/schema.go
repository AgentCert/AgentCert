package model_library_db

import "go.mongodb.org/mongo-driver/bson/primitive"

const CollectionName = "model_library_collection"

type ModelConfigDocument struct {
	ID              primitive.ObjectID `bson:"_id"`
	ProjectID       string             `bson:"projectID"`
	Alias           string             `bson:"alias"`
	Provider        string             `bson:"provider"`
	Model           string             `bson:"model"`
	BaseURL         *string            `bson:"baseURL,omitempty"`
	SecretRef       string             `bson:"secretRef"`
	LiteLLMDeployID string             `bson:"litellmDeployId"`
	Status          string             `bson:"status"`
	LastTestedAt    *int64             `bson:"lastTestedAt,omitempty"`
	AgentsUsing     []string           `bson:"agentsUsing"`
	CreatedAt       int64              `bson:"createdAt"`
	UpdatedAt       int64              `bson:"updatedAt"`
}
