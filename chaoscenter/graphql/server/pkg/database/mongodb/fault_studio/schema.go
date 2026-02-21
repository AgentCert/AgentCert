package fault_studio

import (
	"strconv"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
)

// InjectionType represents the type of fault injection
type InjectionType string

const (
	InjectionTypeScheduled  InjectionType = "SCHEDULED"
	InjectionTypeOnDemand   InjectionType = "ON_DEMAND"
	InjectionTypeContinuous InjectionType = "CONTINUOUS"
)

// FaultInjectionConfig stores configuration for how a fault should be injected
type FaultInjectionConfig struct {
	InjectionType  InjectionType `bson:"injection_type" json:"injectionType"`
	Schedule       string        `bson:"schedule,omitempty" json:"schedule,omitempty"`             // Cron expression for scheduled injection
	Duration       string        `bson:"duration,omitempty" json:"duration,omitempty"`             // Duration of fault injection
	TargetSelector string        `bson:"target_selector,omitempty" json:"targetSelector,omitempty"` // Pod/Node selector
	Interval       string        `bson:"interval,omitempty" json:"interval,omitempty"`             // Interval between injections
}

// FaultSelection represents a single fault selected from a ChaosHub
type FaultSelection struct {
	FaultCategory    string                 `bson:"fault_category" json:"faultCategory"`
	FaultName        string                 `bson:"fault_name" json:"faultName"`
	DisplayName      string                 `bson:"display_name" json:"displayName"`
	Description      string                 `bson:"description,omitempty" json:"description,omitempty"`
	Enabled          bool                   `bson:"enabled" json:"enabled"`
	InjectionConfig  *FaultInjectionConfig  `bson:"injection_config,omitempty" json:"injectionConfig,omitempty"`
	CustomParameters map[string]interface{} `bson:"custom_parameters,omitempty" json:"customParameters,omitempty"`
	Weight           int                    `bson:"weight" json:"weight"` // Priority/weight for fault selection
}

// FaultStudio represents a collection of faults configured for agent testing
type FaultStudio struct {
	ID                      string           `bson:"studio_id" json:"id"`
	ProjectID               string           `bson:"project_id" json:"projectId"`
	mongodb.ResourceDetails `bson:",inline"`
	mongodb.Audit           `bson:",inline"`
	SourceHubID             string           `bson:"source_hub_id" json:"sourceHubId"`       // Reference to ChaosHub
	SourceHubName           string           `bson:"source_hub_name" json:"sourceHubName"`   // Cached hub name for display
	SelectedFaults          []FaultSelection `bson:"selected_faults" json:"selectedFaults"`
	IsActive                bool             `bson:"is_active" json:"isActive"`
	TotalFaults             int              `bson:"total_faults" json:"totalFaults"`
	EnabledFaults           int              `bson:"enabled_faults" json:"enabledFaults"`
}

// FaultStudioSummary is a lightweight version for list views
type FaultStudioSummary struct {
	ID            string `bson:"studio_id" json:"id"`
	Name          string `bson:"name" json:"name"`
	Description   string `bson:"description" json:"description"`
	ProjectID     string `bson:"project_id" json:"projectId"`
	SourceHubID   string `bson:"source_hub_id" json:"sourceHubId"`
	SourceHubName string `bson:"source_hub_name" json:"sourceHubName"`
	TotalFaults   int    `bson:"total_faults" json:"totalFaults"`
	EnabledFaults int    `bson:"enabled_faults" json:"enabledFaults"`
	IsActive      bool   `bson:"is_active" json:"isActive"`
	CreatedAt     int64  `bson:"created_at" json:"createdAt"`
	UpdatedAt     int64  `bson:"updated_at" json:"updatedAt"`
}

// TotalCount is used for aggregation queries
type TotalCount struct {
	Count int `bson:"count"`
}

// AggregatedFaultStudioStats stores aggregated statistics
type AggregatedFaultStudioStats struct {
	TotalFaultStudios []TotalCount `bson:"total_fault_studios"`
}

// GetOutputFaultStudio converts the database model to output model
func (f *FaultStudio) GetOutputFaultStudio() map[string]interface{} {
	return map[string]interface{}{
		"id":             f.ID,
		"name":           f.Name,
		"description":    f.Description,
		"tags":           f.Tags,
		"projectId":      f.ProjectID,
		"sourceHubId":    f.SourceHubID,
		"sourceHubName":  f.SourceHubName,
		"selectedFaults": f.SelectedFaults,
		"isActive":       f.IsActive,
		"totalFaults":    f.TotalFaults,
		"enabledFaults":  f.EnabledFaults,
		"isRemoved":      f.IsRemoved,
		"createdAt":      strconv.FormatInt(f.CreatedAt, 10),
		"updatedAt":      strconv.FormatInt(f.UpdatedAt, 10),
		"createdBy":      f.CreatedBy,
		"updatedBy":      f.UpdatedBy,
	}
}

// CountEnabledFaults returns the count of enabled faults
func (f *FaultStudio) CountEnabledFaults() int {
	count := 0
	for _, fault := range f.SelectedFaults {
		if fault.Enabled {
			count++
		}
	}
	return count
}

// UpdateFaultCounts updates the total and enabled fault counts
func (f *FaultStudio) UpdateFaultCounts() {
	f.TotalFaults = len(f.SelectedFaults)
	f.EnabledFaults = f.CountEnabledFaults()
}
