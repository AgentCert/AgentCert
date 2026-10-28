package model_config

import (
	"strconv"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
	dbModelConfig "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/model_config"
)

func toModelConfig(row dbModelConfig.ModelConfig) *model.ModelConfig {
	createdAt := strconv.FormatInt(row.CreatedAt, 10)
	updatedAt := strconv.FormatInt(row.UpdatedAt, 10)
	return &model.ModelConfig{
		ProjectID:  row.ProjectID,
		ModelID:    row.ModelID,
		Alias:      row.Alias,
		Label:      row.Label,
		Provider:   model.ModelProvider(row.Provider),
		Model:      row.Model,
		APIBase:    optional(row.APIBase),
		APIVersion: optional(row.APIVersion),
		APIKeyHint: optional(row.APIKeyHint),
		SyncStatus: model.ModelSyncStatus(row.SyncStatus),
		LastError:  optional(row.LastError),
		CreatedAt:  &createdAt,
		UpdatedAt:  &updatedAt,
		CreatedBy:  toUserDetails(row.CreatedBy),
		UpdatedBy:  toUserDetails(row.UpdatedBy),
	}
}

func toUserDetails(user mongodb.UserDetailResponse) *model.UserDetails {
	return &model.UserDetails{UserID: user.UserID, Username: user.Username, Email: user.Email}
}

func toTestResult(result TestResult) *model.ModelTestResult {
	latency := result.LatencyMs
	return &model.ModelTestResult{Ok: result.OK, LatencyMs: &latency, Error: optional(result.Error)}
}

func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
