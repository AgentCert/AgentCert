package fault_studio

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/authorization"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
	dbSchemaChaosHub "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_hub"
	dbSchemaFaultStudio "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/fault_studio"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
)

// DefaultHubID is the constant ID for the default Litmus ChaosHub
const DefaultHubID = "6f39cea9-6264-4951-83a8-29976b614289"

// DefaultHubName is the name of the default Litmus ChaosHub
const DefaultHubName = "Litmus ChaosHub"

// Service defines the interface for fault studio operations
type Service interface {
	// CreateFaultStudio creates a new fault studio
	CreateFaultStudio(ctx context.Context, projectID string, request model.CreateFaultStudioRequest) (*model.FaultStudio, error)

	// GetFaultStudio retrieves a fault studio by ID
	GetFaultStudio(ctx context.Context, projectID string, studioID string) (*model.FaultStudio, error)

	// ListFaultStudios returns a paginated list of fault studios
	ListFaultStudios(ctx context.Context, projectID string, request *model.ListFaultStudioRequest) (*model.ListFaultStudioResponse, error)

	// UpdateFaultStudio updates an existing fault studio
	UpdateFaultStudio(ctx context.Context, projectID string, studioID string, request model.UpdateFaultStudioRequest) (*model.FaultStudio, error)

	// DeleteFaultStudio soft deletes a fault studio
	DeleteFaultStudio(ctx context.Context, projectID string, studioID string) (bool, error)

	// ToggleFaultInStudio enables or disables a specific fault in a studio
	ToggleFaultInStudio(ctx context.Context, projectID string, studioID string, faultName string, enabled bool) (*model.ToggleFaultResponse, error)

	// SetFaultStudioActive activates or deactivates a fault studio
	SetFaultStudioActive(ctx context.Context, projectID string, studioID string, isActive bool) (*model.FaultStudio, error)

	// AddFaultToStudio adds a new fault to an existing studio
	AddFaultToStudio(ctx context.Context, projectID string, studioID string, fault model.FaultSelectionInput) (*model.FaultStudio, error)

	// RemoveFaultFromStudio removes a fault from a studio
	RemoveFaultFromStudio(ctx context.Context, projectID string, studioID string, faultName string) (*model.FaultStudio, error)

	// UpdateFaultInStudio updates a specific fault's configuration in a studio
	UpdateFaultInStudio(ctx context.Context, projectID string, studioID string, fault model.FaultSelectionInput) (*model.FaultStudio, error)

	// GetFaultStudioStats returns statistics about fault studios in a project
	GetFaultStudioStats(ctx context.Context, projectID string) (*model.FaultStudioStatsResponse, error)

	// IsFaultStudioNameAvailable checks if a name is available for use
	IsFaultStudioNameAvailable(ctx context.Context, projectID string, name string) (bool, error)

	// ListAvailableFaultsForStudio returns available faults from the source ChaosHub
	ListAvailableFaultsForStudio(ctx context.Context, projectID string, hubID string) ([]*model.FaultList, error)
}

// faultStudioService implements the Service interface
type faultStudioService struct {
	faultStudioOperator *dbSchemaFaultStudio.Operator
	chaosHubOperator    *dbSchemaChaosHub.Operator
}

// NewService creates a new fault studio service instance
func NewService(faultStudioOperator *dbSchemaFaultStudio.Operator, chaosHubOperator *dbSchemaChaosHub.Operator) Service {
	return &faultStudioService{
		faultStudioOperator: faultStudioOperator,
		chaosHubOperator:    chaosHubOperator,
	}
}

// CreateFaultStudio creates a new fault studio
func (s *faultStudioService) CreateFaultStudio(ctx context.Context, projectID string, request model.CreateFaultStudioRequest) (*model.FaultStudio, error) {
	// Check if name is unique
	isUnique, err := s.faultStudioOperator.IsFaultStudioNameUnique(ctx, request.Name, projectID, "")
	if err != nil {
		log.Error("error checking name uniqueness: ", err)
		return nil, err
	}
	if !isUnique {
		return nil, errors.New("fault studio name already exists in this project")
	}

	// Get the source ChaosHub name
	var sourceHubName string
	
	// Check if this is the default hub (not stored in DB, generated dynamically)
	if request.SourceHubID == DefaultHubID {
		sourceHubName = DefaultHubName
		log.Info("Using default Litmus ChaosHub as source")
	} else {
		// Try to get from database - first by project_id
		chaosHub, err := s.chaosHubOperator.GetHubByID(ctx, request.SourceHubID, projectID)
		if err != nil {
			// If not found, try to get by hub_id only
			log.Info("Hub not found by project_id, trying by hub_id only...")
			chaosHub, err = s.chaosHubOperator.GetHubByIDOnly(ctx, request.SourceHubID)
			if err != nil {
				log.Error("error getting source chaos hub: ", err)
				return nil, errors.New("source ChaosHub not found")
			}
		}
		sourceHubName = chaosHub.Name
	}

	// Get username from context
	tkn := ctx.Value(authorization.AuthKey).(string)
	username, err := authorization.GetUsername(tkn)
	if err != nil {
		log.Error("error getting username: ", err)
		return nil, err
	}

	currentTime := time.Now()

	// Convert input faults to database schema
	selectedFaults := make([]dbSchemaFaultStudio.FaultSelection, len(request.SelectedFaults))
	for i, fault := range request.SelectedFaults {
		selectedFaults[i] = convertInputToDBFaultSelection(fault)
	}

	// Calculate fault counts
	totalFaults := len(selectedFaults)
	enabledFaults := countEnabledFaults(selectedFaults)

	// Determine initial active state
	isActive := false
	if request.IsActive != nil {
		isActive = *request.IsActive
	}

	// Get description
	description := ""
	if request.Description != nil {
		description = *request.Description
	}

	// Create the fault studio
	newStudio := &dbSchemaFaultStudio.FaultStudio{
		ID:        uuid.New().String(),
		ProjectID: projectID,
		ResourceDetails: mongodb.ResourceDetails{
			Name:        request.Name,
			Description: description,
			Tags:        request.Tags,
		},
		SourceHubID:    request.SourceHubID,
		SourceHubName:  sourceHubName,
		SelectedFaults: selectedFaults,
		IsActive:       isActive,
		TotalFaults:    totalFaults,
		EnabledFaults:  enabledFaults,
		Audit: mongodb.Audit{
			CreatedAt: currentTime.UnixMilli(),
			UpdatedAt: currentTime.UnixMilli(),
			IsRemoved: false,
			CreatedBy: mongodb.UserDetailResponse{
				Username: username,
			},
			UpdatedBy: mongodb.UserDetailResponse{
				Username: username,
			},
		},
	}

	// Save to database
	if err := s.faultStudioOperator.CreateFaultStudio(ctx, newStudio); err != nil {
		log.Error("error creating fault studio: ", err)
		return nil, err
	}

	log.Info("Successfully created fault studio: ", newStudio.ID)
	return convertDBToModelFaultStudio(newStudio), nil
}

// GetFaultStudio retrieves a fault studio by ID
func (s *faultStudioService) GetFaultStudio(ctx context.Context, projectID string, studioID string) (*model.FaultStudio, error) {
	faultStudio, err := s.faultStudioOperator.GetFaultStudioByID(ctx, studioID, projectID)
	if err != nil {
		log.Error("error getting fault studio: ", err)
		return nil, err
	}

	return convertDBToModelFaultStudio(faultStudio), nil
}

// ListFaultStudios returns a paginated list of fault studios
func (s *faultStudioService) ListFaultStudios(ctx context.Context, projectID string, request *model.ListFaultStudioRequest) (*model.ListFaultStudioResponse, error) {
	// Default pagination values
	limit := 15
	offset := 0

	if request != nil {
		if request.Limit != nil {
			limit = *request.Limit
		}
		if request.Offset != nil {
			offset = *request.Offset
		}
	}

	// Get paginated results
	faultStudios, totalCount, err := s.faultStudioOperator.ListFaultStudiosWithPagination(ctx, projectID, int64(limit), int64(offset))
	if err != nil {
		log.Error("error listing fault studios: ", err)
		return nil, err
	}

	// Convert to summary models
	summaries := make([]*model.FaultStudioSummary, len(faultStudios))
	for i, studio := range faultStudios {
		summaries[i] = convertDBToModelFaultStudioSummary(&studio)
	}

	return &model.ListFaultStudioResponse{
		FaultStudios: summaries,
		TotalCount:   int(totalCount),
	}, nil
}

// UpdateFaultStudio updates an existing fault studio
func (s *faultStudioService) UpdateFaultStudio(ctx context.Context, projectID string, studioID string, request model.UpdateFaultStudioRequest) (*model.FaultStudio, error) {
	// Verify studio exists
	existingStudio, err := s.faultStudioOperator.GetFaultStudioByID(ctx, studioID, projectID)
	if err != nil {
		log.Error("error getting fault studio for update: ", err)
		return nil, err
	}

	// Get username from context
	tkn := ctx.Value(authorization.AuthKey).(string)
	username, err := authorization.GetUsername(tkn)
	if err != nil {
		log.Error("error getting username: ", err)
		return nil, err
	}

	// Build update document
	updateFields := bson.D{
		{Key: "updated_at", Value: time.Now().UnixMilli()},
		{Key: "updated_by", Value: mongodb.UserDetailResponse{Username: username}},
	}

	// Update name if provided (and check uniqueness)
	if request.Name != nil && *request.Name != existingStudio.Name {
		isUnique, err := s.faultStudioOperator.IsFaultStudioNameUnique(ctx, *request.Name, projectID, studioID)
		if err != nil {
			return nil, err
		}
		if !isUnique {
			return nil, errors.New("fault studio name already exists")
		}
		updateFields = append(updateFields, bson.E{Key: "name", Value: *request.Name})
	}

	// Update description if provided
	if request.Description != nil {
		updateFields = append(updateFields, bson.E{Key: "description", Value: *request.Description})
	}

	// Update tags if provided
	if request.Tags != nil {
		updateFields = append(updateFields, bson.E{Key: "tags", Value: request.Tags})
	}

	// Update selected faults if provided
	if request.SelectedFaults != nil {
		selectedFaults := make([]dbSchemaFaultStudio.FaultSelection, len(request.SelectedFaults))
		for i, fault := range request.SelectedFaults {
			selectedFaults[i] = convertInputToDBFaultSelection(fault)
		}
		updateFields = append(updateFields, bson.E{Key: "selected_faults", Value: selectedFaults})
		updateFields = append(updateFields, bson.E{Key: "total_faults", Value: len(selectedFaults)})
		updateFields = append(updateFields, bson.E{Key: "enabled_faults", Value: countEnabledFaults(selectedFaults)})
	}

	// Update active status if provided
	if request.IsActive != nil {
		updateFields = append(updateFields, bson.E{Key: "is_active", Value: *request.IsActive})
	}

	// Execute update
	query := bson.D{
		{Key: "studio_id", Value: studioID},
		{Key: "project_id", Value: projectID},
		{Key: "is_removed", Value: false},
	}

	update := bson.D{{Key: "$set", Value: updateFields}}

	if err := s.faultStudioOperator.UpdateFaultStudio(ctx, query, update); err != nil {
		log.Error("error updating fault studio: ", err)
		return nil, err
	}

	// Fetch and return updated studio
	return s.GetFaultStudio(ctx, projectID, studioID)
}

// DeleteFaultStudio soft deletes a fault studio
func (s *faultStudioService) DeleteFaultStudio(ctx context.Context, projectID string, studioID string) (bool, error) {
	// Get username from context
	tkn := ctx.Value(authorization.AuthKey).(string)
	username, err := authorization.GetUsername(tkn)
	if err != nil {
		log.Error("error getting username: ", err)
		return false, err
	}

	updatedBy := mongodb.UserDetailResponse{Username: username}

	if err := s.faultStudioOperator.DeleteFaultStudio(ctx, studioID, projectID, updatedBy); err != nil {
		log.Error("error deleting fault studio: ", err)
		return false, err
	}

	log.Info("Successfully deleted fault studio: ", studioID)
	return true, nil
}

// ToggleFaultInStudio enables or disables a specific fault in a studio
func (s *faultStudioService) ToggleFaultInStudio(ctx context.Context, projectID string, studioID string, faultName string, enabled bool) (*model.ToggleFaultResponse, error) {
	// Get existing studio
	studio, err := s.faultStudioOperator.GetFaultStudioByID(ctx, studioID, projectID)
	if err != nil {
		log.Error("error getting fault studio: ", err)
		return nil, err
	}

	// Find and toggle the fault
	found := false
	for i := range studio.SelectedFaults {
		if studio.SelectedFaults[i].FaultName == faultName {
			studio.SelectedFaults[i].Enabled = enabled
			found = true
			break
		}
	}

	if !found {
		return nil, errors.New("fault not found in studio")
	}

	// Recalculate enabled count
	enabledCount := countEnabledFaults(studio.SelectedFaults)

	// Get username from context
	tkn := ctx.Value(authorization.AuthKey).(string)
	username, err := authorization.GetUsername(tkn)
	if err != nil {
		return nil, err
	}

	// Update the studio
	query := bson.D{
		{Key: "studio_id", Value: studioID},
		{Key: "project_id", Value: projectID},
	}

	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "selected_faults", Value: studio.SelectedFaults},
		{Key: "enabled_faults", Value: enabledCount},
		{Key: "updated_at", Value: time.Now().UnixMilli()},
		{Key: "updated_by", Value: mongodb.UserDetailResponse{Username: username}},
	}}}

	if err := s.faultStudioOperator.UpdateFaultStudio(ctx, query, update); err != nil {
		return nil, err
	}

	// Get updated studio for response
	updatedStudio, err := s.GetFaultStudio(ctx, projectID, studioID)
	if err != nil {
		return nil, err
	}

	message := "Fault toggled successfully"
	return &model.ToggleFaultResponse{
		FaultStudio: updatedStudio,
		Success:     true,
		Message:     &message,
	}, nil
}

// SetFaultStudioActive activates or deactivates a fault studio
func (s *faultStudioService) SetFaultStudioActive(ctx context.Context, projectID string, studioID string, isActive bool) (*model.FaultStudio, error) {
	// Get username from context
	tkn := ctx.Value(authorization.AuthKey).(string)
	username, err := authorization.GetUsername(tkn)
	if err != nil {
		return nil, err
	}

	query := bson.D{
		{Key: "studio_id", Value: studioID},
		{Key: "project_id", Value: projectID},
		{Key: "is_removed", Value: false},
	}

	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "is_active", Value: isActive},
		{Key: "updated_at", Value: time.Now().UnixMilli()},
		{Key: "updated_by", Value: mongodb.UserDetailResponse{Username: username}},
	}}}

	if err := s.faultStudioOperator.UpdateFaultStudio(ctx, query, update); err != nil {
		return nil, err
	}

	return s.GetFaultStudio(ctx, projectID, studioID)
}

// AddFaultToStudio adds a new fault to an existing studio
func (s *faultStudioService) AddFaultToStudio(ctx context.Context, projectID string, studioID string, fault model.FaultSelectionInput) (*model.FaultStudio, error) {
	// Get existing studio
	studio, err := s.faultStudioOperator.GetFaultStudioByID(ctx, studioID, projectID)
	if err != nil {
		return nil, err
	}

	// Check if fault already exists
	for _, existingFault := range studio.SelectedFaults {
		if existingFault.FaultName == fault.FaultName {
			return nil, errors.New("fault already exists in studio")
		}
	}

	// Add the new fault
	newFault := convertInputToDBFaultSelection(&fault)
	studio.SelectedFaults = append(studio.SelectedFaults, newFault)

	// Recalculate counts
	totalFaults := len(studio.SelectedFaults)
	enabledFaults := countEnabledFaults(studio.SelectedFaults)

	// Get username from context
	tkn := ctx.Value(authorization.AuthKey).(string)
	username, err := authorization.GetUsername(tkn)
	if err != nil {
		return nil, err
	}

	// Update the studio
	query := bson.D{
		{Key: "studio_id", Value: studioID},
		{Key: "project_id", Value: projectID},
	}

	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "selected_faults", Value: studio.SelectedFaults},
		{Key: "total_faults", Value: totalFaults},
		{Key: "enabled_faults", Value: enabledFaults},
		{Key: "updated_at", Value: time.Now().UnixMilli()},
		{Key: "updated_by", Value: mongodb.UserDetailResponse{Username: username}},
	}}}

	if err := s.faultStudioOperator.UpdateFaultStudio(ctx, query, update); err != nil {
		return nil, err
	}

	return s.GetFaultStudio(ctx, projectID, studioID)
}

// RemoveFaultFromStudio removes a fault from a studio
func (s *faultStudioService) RemoveFaultFromStudio(ctx context.Context, projectID string, studioID string, faultName string) (*model.FaultStudio, error) {
	// Get existing studio
	studio, err := s.faultStudioOperator.GetFaultStudioByID(ctx, studioID, projectID)
	if err != nil {
		return nil, err
	}

	// Find and remove the fault
	found := false
	newFaults := make([]dbSchemaFaultStudio.FaultSelection, 0, len(studio.SelectedFaults)-1)
	for _, fault := range studio.SelectedFaults {
		if fault.FaultName == faultName {
			found = true
			continue
		}
		newFaults = append(newFaults, fault)
	}

	if !found {
		return nil, errors.New("fault not found in studio")
	}

	// Recalculate counts
	totalFaults := len(newFaults)
	enabledFaults := countEnabledFaults(newFaults)

	// Get username from context
	tkn := ctx.Value(authorization.AuthKey).(string)
	username, err := authorization.GetUsername(tkn)
	if err != nil {
		return nil, err
	}

	// Update the studio
	query := bson.D{
		{Key: "studio_id", Value: studioID},
		{Key: "project_id", Value: projectID},
	}

	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "selected_faults", Value: newFaults},
		{Key: "total_faults", Value: totalFaults},
		{Key: "enabled_faults", Value: enabledFaults},
		{Key: "updated_at", Value: time.Now().UnixMilli()},
		{Key: "updated_by", Value: mongodb.UserDetailResponse{Username: username}},
	}}}

	if err := s.faultStudioOperator.UpdateFaultStudio(ctx, query, update); err != nil {
		return nil, err
	}

	return s.GetFaultStudio(ctx, projectID, studioID)
}

// UpdateFaultInStudio updates a specific fault's configuration in a studio
func (s *faultStudioService) UpdateFaultInStudio(ctx context.Context, projectID string, studioID string, fault model.FaultSelectionInput) (*model.FaultStudio, error) {
	// Get existing studio
	studio, err := s.faultStudioOperator.GetFaultStudioByID(ctx, studioID, projectID)
	if err != nil {
		return nil, err
	}

	// Find and update the fault
	found := false
	for i := range studio.SelectedFaults {
		if studio.SelectedFaults[i].FaultName == fault.FaultName {
			studio.SelectedFaults[i] = convertInputToDBFaultSelection(&fault)
			found = true
			break
		}
	}

	if !found {
		return nil, errors.New("fault not found in studio")
	}

	// Recalculate enabled count
	enabledFaults := countEnabledFaults(studio.SelectedFaults)

	// Get username from context
	tkn := ctx.Value(authorization.AuthKey).(string)
	username, err := authorization.GetUsername(tkn)
	if err != nil {
		return nil, err
	}

	// Update the studio
	query := bson.D{
		{Key: "studio_id", Value: studioID},
		{Key: "project_id", Value: projectID},
	}

	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "selected_faults", Value: studio.SelectedFaults},
		{Key: "enabled_faults", Value: enabledFaults},
		{Key: "updated_at", Value: time.Now().UnixMilli()},
		{Key: "updated_by", Value: mongodb.UserDetailResponse{Username: username}},
	}}}

	if err := s.faultStudioOperator.UpdateFaultStudio(ctx, query, update); err != nil {
		return nil, err
	}

	return s.GetFaultStudio(ctx, projectID, studioID)
}

// GetFaultStudioStats returns statistics about fault studios in a project
func (s *faultStudioService) GetFaultStudioStats(ctx context.Context, projectID string) (*model.FaultStudioStatsResponse, error) {
	stats, err := s.faultStudioOperator.GetFaultStudioStats(ctx, projectID)
	if err != nil {
		log.Error("error getting fault studio stats: ", err)
		return nil, err
	}

	totalStudios := 0
	if len(stats.TotalFaultStudios) > 0 {
		totalStudios = stats.TotalFaultStudios[0].Count
	}

	// Get active studios count
	activeStudios, err := s.faultStudioOperator.GetActiveFaultStudios(ctx, projectID)
	if err != nil {
		log.Error("error getting active fault studios: ", err)
		return nil, err
	}

	// Calculate total faults configured
	allStudios, err := s.faultStudioOperator.GetFaultStudiosByProjectID(ctx, projectID)
	if err != nil {
		log.Error("error getting all fault studios: ", err)
		return nil, err
	}

	totalFaultsConfigured := 0
	for _, studio := range allStudios {
		totalFaultsConfigured += len(studio.SelectedFaults)
	}

	return &model.FaultStudioStatsResponse{
		TotalFaultStudios:     totalStudios,
		ActiveFaultStudios:    len(activeStudios),
		TotalFaultsConfigured: totalFaultsConfigured,
	}, nil
}

// IsFaultStudioNameAvailable checks if a name is available for use
func (s *faultStudioService) IsFaultStudioNameAvailable(ctx context.Context, projectID string, name string) (bool, error) {
	return s.faultStudioOperator.IsFaultStudioNameUnique(ctx, name, projectID, "")
}

// ListAvailableFaultsForStudio returns available faults from the source ChaosHub
// This leverages the existing ChaosHub infrastructure to get fault lists
func (s *faultStudioService) ListAvailableFaultsForStudio(ctx context.Context, projectID string, hubID string) ([]*model.FaultList, error) {
	// Verify the hub exists
	_, err := s.chaosHubOperator.GetHubByID(ctx, hubID, projectID)
	if err != nil {
		return nil, errors.New("ChaosHub not found")
	}

	// Note: This will be implemented to leverage ChaosHub's ListChaosFaults
	// For now, return an empty list - the actual implementation will use
	// the chaoshub handler to read faults from the cloned repository
	log.Info("ListAvailableFaultsForStudio called for hub: ", hubID)

	// TODO: Integrate with chaoshub handler to get actual faults
	// This would typically call handler.GetChartsPath and parse the charts
	return []*model.FaultList{}, nil
}

// Helper functions

// countEnabledFaults counts the number of enabled faults
func countEnabledFaults(faults []dbSchemaFaultStudio.FaultSelection) int {
	count := 0
	for _, fault := range faults {
		if fault.Enabled {
			count++
		}
	}
	return count
}

// convertInputToDBFaultSelection converts GraphQL input to database schema
func convertInputToDBFaultSelection(input *model.FaultSelectionInput) dbSchemaFaultStudio.FaultSelection {
	description := ""
	if input.Description != nil {
		description = *input.Description
	}

	weight := 1
	if input.Weight != nil {
		weight = *input.Weight
	}

	var injectionConfig *dbSchemaFaultStudio.FaultInjectionConfig
	if input.InjectionConfig != nil {
		schedule := ""
		duration := ""
		targetSelector := ""
		interval := ""

		if input.InjectionConfig.Schedule != nil {
			schedule = *input.InjectionConfig.Schedule
		}
		if input.InjectionConfig.Duration != nil {
			duration = *input.InjectionConfig.Duration
		}
		if input.InjectionConfig.TargetSelector != nil {
			targetSelector = *input.InjectionConfig.TargetSelector
		}
		if input.InjectionConfig.Interval != nil {
			interval = *input.InjectionConfig.Interval
		}

		injectionConfig = &dbSchemaFaultStudio.FaultInjectionConfig{
			InjectionType:  dbSchemaFaultStudio.InjectionType(input.InjectionConfig.InjectionType),
			Schedule:       schedule,
			Duration:       duration,
			TargetSelector: targetSelector,
			Interval:       interval,
		}
	}

	// Handle custom parameters (JSON string to map)
	var customParams map[string]interface{}
	if input.CustomParameters != nil && *input.CustomParameters != "" {
		_ = json.Unmarshal([]byte(*input.CustomParameters), &customParams)
	}

	return dbSchemaFaultStudio.FaultSelection{
		FaultCategory:    input.FaultCategory,
		FaultName:        input.FaultName,
		DisplayName:      input.DisplayName,
		Description:      description,
		Enabled:          input.Enabled,
		InjectionConfig:  injectionConfig,
		CustomParameters: customParams,
		Weight:           weight,
	}
}

// convertDBToModelFaultStudio converts database schema to GraphQL model
func convertDBToModelFaultStudio(studio *dbSchemaFaultStudio.FaultStudio) *model.FaultStudio {
	// Convert selected faults
	selectedFaults := make([]*model.FaultSelection, len(studio.SelectedFaults))
	for i, fault := range studio.SelectedFaults {
		selectedFaults[i] = convertDBToModelFaultSelection(&fault)
	}

	description := &studio.Description

	return &model.FaultStudio{
		ID:             studio.ID,
		Name:           studio.Name,
		Description:    description,
		Tags:           studio.Tags,
		ProjectID:      studio.ProjectID,
		SourceHubID:    studio.SourceHubID,
		SourceHubName:  studio.SourceHubName,
		SelectedFaults: selectedFaults,
		IsActive:       studio.IsActive,
		TotalFaults:    studio.TotalFaults,
		EnabledFaults:  studio.EnabledFaults,
		IsRemoved:      studio.IsRemoved,
		CreatedAt:      strconv.FormatInt(studio.CreatedAt, 10),
		UpdatedAt:      strconv.FormatInt(studio.UpdatedAt, 10),
		CreatedBy: &model.UserDetails{
			Username: studio.CreatedBy.Username,
		},
		UpdatedBy: &model.UserDetails{
			Username: studio.UpdatedBy.Username,
		},
	}
}

// convertDBToModelFaultSelection converts a database FaultSelection to GraphQL model
func convertDBToModelFaultSelection(fault *dbSchemaFaultStudio.FaultSelection) *model.FaultSelection {
	description := &fault.Description

	var injectionConfig *model.FaultInjectionConfig
	if fault.InjectionConfig != nil {
		schedule := &fault.InjectionConfig.Schedule
		duration := &fault.InjectionConfig.Duration
		targetSelector := &fault.InjectionConfig.TargetSelector
		interval := &fault.InjectionConfig.Interval

		injectionConfig = &model.FaultInjectionConfig{
			InjectionType:  model.InjectionType(fault.InjectionConfig.InjectionType),
			Schedule:       schedule,
			Duration:       duration,
			TargetSelector: targetSelector,
			Interval:       interval,
		}
	}

	// Convert custom parameters map to JSON string
	var customParams *string
	if fault.CustomParameters != nil && len(fault.CustomParameters) > 0 {
		jsonBytes, err := json.Marshal(fault.CustomParameters)
		if err == nil {
			jsonStr := string(jsonBytes)
			customParams = &jsonStr
		}
	}

	return &model.FaultSelection{
		FaultCategory:    fault.FaultCategory,
		FaultName:        fault.FaultName,
		DisplayName:      fault.DisplayName,
		Description:      description,
		Enabled:          fault.Enabled,
		InjectionConfig:  injectionConfig,
		CustomParameters: customParams,
		Weight:           fault.Weight,
	}
}

// convertDBToModelFaultStudioSummary converts database schema to summary model
func convertDBToModelFaultStudioSummary(studio *dbSchemaFaultStudio.FaultStudio) *model.FaultStudioSummary {
	description := &studio.Description

	return &model.FaultStudioSummary{
		ID:            studio.ID,
		Name:          studio.Name,
		Description:   description,
		ProjectID:     studio.ProjectID,
		SourceHubID:   studio.SourceHubID,
		SourceHubName: studio.SourceHubName,
		TotalFaults:   studio.TotalFaults,
		EnabledFaults: studio.EnabledFaults,
		IsActive:      studio.IsActive,
		CreatedAt:     strconv.FormatInt(studio.CreatedAt, 10),
		UpdatedAt:     strconv.FormatInt(studio.UpdatedAt, 10),
	}
}
