package apps_registry

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Operator defines the data access layer interface for Apps Registry.
type Operator interface {
	// CreateApp inserts a new app document
	CreateApp(ctx context.Context, app *App) error

	// GetApp retrieves an app by ID
	GetApp(ctx context.Context, id string) (*App, error)

	// GetAppByProjectAndName retrieves an app by project and name (for uniqueness check)
	GetAppByProjectAndName(ctx context.Context, projectID, name string) (*App, error)

	// ListApps retrieves apps with filtering and pagination
	ListApps(ctx context.Context, filter *AppFilter, skip, limit int) ([]*App, int64, error)

	// UpdateApp updates an existing app document
	UpdateApp(ctx context.Context, app *App) error

	// DeleteApp soft-deletes an app document
	DeleteApp(ctx context.Context, id string) error
}

// operatorImpl is the concrete implementation of the Operator interface.
type operatorImpl struct {
	collection *mongo.Collection
}

// NewOperator creates a new Operator instance.
func NewOperator(database *mongo.Database) Operator {
	return &operatorImpl{
		collection: database.Collection("apps_registrations"),
	}
}

// CreateApp inserts a new app document into MongoDB.
func (o *operatorImpl) CreateApp(ctx context.Context, app *App) error {
	_, err := o.collection.InsertOne(ctx, app)
	if err != nil {
		return err
	}
	return nil
}

// GetApp retrieves an app by ID from MongoDB.
func (o *operatorImpl) GetApp(ctx context.Context, id string) (*App, error) {
	filter := bson.M{"appId": id}

	var app App
	err := o.collection.FindOne(ctx, filter).Decode(&app)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrAppNotFound
		}
		return nil, err
	}

	return &app, nil
}

// GetAppByProjectAndName retrieves an app by project ID and name.
func (o *operatorImpl) GetAppByProjectAndName(ctx context.Context, projectID, name string) (*App, error) {
	filter := bson.M{
		"projectId": projectID,
		"name":      name,
		"status":    bson.M{"$ne": AppStatusDeleted},
	}

	var app App
	err := o.collection.FindOne(ctx, filter).Decode(&app)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrAppNotFound
		}
		return nil, err
	}

	return &app, nil
}

// ListApps retrieves apps with filtering and pagination.
func (o *operatorImpl) ListApps(ctx context.Context, filter *AppFilter, skip, limit int) ([]*App, int64, error) {
	// Build MongoDB filter
	mongoFilter := bson.M{}

	if filter != nil {
		// Filter by projectId
		if filter.ProjectID != "" {
			mongoFilter["projectId"] = filter.ProjectID
		}

		// Filter by environmentId
		if filter.EnvironmentID != "" {
			mongoFilter["environmentId"] = filter.EnvironmentID
		}

		// Filter by status (exclude DELETED by default)
		if filter.Status != "" {
			mongoFilter["status"] = filter.Status
		} else {
			mongoFilter["status"] = bson.M{"$ne": AppStatusDeleted}
		}

		// Search by name
		if filter.SearchTerm != "" {
			mongoFilter["name"] = bson.M{"$regex": filter.SearchTerm, "$options": "i"}
		}
	}

	// Count total matching documents
	count, err := o.collection.CountDocuments(ctx, mongoFilter)
	if err != nil {
		return nil, 0, err
	}

	// Set options for pagination and sorting
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "auditInfo.createdAt", Value: -1}})

	cursor, err := o.collection.Find(ctx, mongoFilter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var apps []*App
	if err = cursor.All(ctx, &apps); err != nil {
		return nil, 0, err
	}

	return apps, count, nil
}

// UpdateApp updates an existing app document.
func (o *operatorImpl) UpdateApp(ctx context.Context, app *App) error {
	filter := bson.M{"appId": app.AppID}
	update := bson.M{"$set": app}

	result, err := o.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return ErrAppNotFound
	}

	return nil
}

// DeleteApp soft-deletes an app document.
func (o *operatorImpl) DeleteApp(ctx context.Context, id string) error {
	filter := bson.M{"appId": id}
	update := bson.M{
		"$set": bson.M{
			"status": AppStatusDeleted,
		},
	}

	result, err := o.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return ErrAppNotFound
	}

	return nil
}
