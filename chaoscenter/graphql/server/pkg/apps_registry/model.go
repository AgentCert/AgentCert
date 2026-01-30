package apps_registry

// App represents an application registered in the platform.
type App struct {
	AppID         string       `bson:"appId" json:"appId"`
	ProjectID     string       `bson:"projectId" json:"projectId"`
	Name          string       `bson:"name" json:"name"`
	Version       string       `bson:"version" json:"version"`
	Description   string       `bson:"description,omitempty" json:"description,omitempty"`
	ChartName     string       `bson:"chartName,omitempty" json:"chartName,omitempty"`
	Namespace     string       `bson:"namespace" json:"namespace"`
	EnvironmentID string       `bson:"environmentId" json:"environmentId"` // Placeholder for environment relationship
	Method        string       `bson:"method" json:"method"`               // HELM_CHART, CLOUD_MANAGED, etc.
	Status        AppStatus    `bson:"status" json:"status"`
	Metadata      *AppMetadata `bson:"metadata,omitempty" json:"metadata,omitempty"`
	AuditInfo     *AuditInfo   `bson:"auditInfo" json:"auditInfo"`
}

// AppMetadata represents additional metadata for an application.
type AppMetadata struct {
	Labels       map[string]string `bson:"labels,omitempty" json:"labels,omitempty"`
	Annotations  map[string]string `bson:"annotations,omitempty" json:"annotations,omitempty"`
	ChartVersion string            `bson:"chartVersion,omitempty" json:"chartVersion,omitempty"`
	AppVersion   string            `bson:"appVersion,omitempty" json:"appVersion,omitempty"`
}

// AuditInfo represents audit information for an application.
type AuditInfo struct {
	CreatedAt int64  `bson:"createdAt" json:"createdAt"`
	CreatedBy string `bson:"createdBy" json:"createdBy"`
	UpdatedAt int64  `bson:"updatedAt" json:"updatedAt"`
	UpdatedBy string `bson:"updatedBy" json:"updatedBy"`
}

// AppStatus represents the current status of an application.
type AppStatus string

const (
	AppStatusRegistered AppStatus = "REGISTERED"
	AppStatusActive     AppStatus = "ACTIVE"
	AppStatusInactive   AppStatus = "INACTIVE"
	AppStatusDeleted    AppStatus = "DELETED"
)

// AppFilter represents filter options for listing applications.
type AppFilter struct {
	ProjectID     string
	EnvironmentID string
	Status        AppStatus
	SearchTerm    string
}
