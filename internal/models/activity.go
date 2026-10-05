package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Activity struct {
	ID         int64
	ProjectID  int64
	UserID     *int64
	Action     string
	EntityType string
	EntityID   *int64
	Metadata   json.RawMessage
	CreatedAt  time.Time
}

type ActivityCreateInput struct {
	ProjectID  int64
	UserID     *int64
	Action     string
	EntityType string
	EntityID   *int64
	Metadata   json.RawMessage
}

type ActivityModel struct {
	DB *sql.DB
}

func (m *ActivityModel) Create(
	ctx context.Context,
	input ActivityCreateInput,
) (Activity, error) {
	if input.ProjectID < 1 {
		return Activity{}, fmt.Errorf("project ID must be positive")
	}

	if input.UserID != nil && *input.UserID < 1 {
		return Activity{}, fmt.Errorf("user ID must be positive")
	}

	if input.EntityID != nil && *input.EntityID < 1 {
		return Activity{}, fmt.Errorf("entity ID must be positive")
	}

	action := strings.TrimSpace(input.Action)
	entityType := strings.TrimSpace(input.EntityType)

	if action == "" {
		return Activity{}, fmt.Errorf("activity action is required")
	}

	if entityType == "" {
		return Activity{}, fmt.Errorf("activity entity type is required")
	}

	metadata := input.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}

	if !json.Valid(metadata) {
		return Activity{}, fmt.Errorf("activity metadata must be valid JSON")
	}

	var activity Activity
	var userID sql.NullInt64
	var entityID sql.NullInt64
	var storedMetadata []byte

	err := m.DB.QueryRowContext(
		ctx,
		`
		INSERT INTO activity_logs (
			project_id,
			user_id,
			action,
			entity_type,
			entity_id,
			metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING
			id,
			project_id,
			user_id,
			action,
			entity_type,
			entity_id,
			metadata,
			created_at
		`,
		input.ProjectID,
		input.UserID,
		action,
		entityType,
		input.EntityID,
		metadata,
	).Scan(
		&activity.ID,
		&activity.ProjectID,
		&userID,
		&activity.Action,
		&activity.EntityType,
		&entityID,
		&storedMetadata,
		&activity.CreatedAt,
	)
	if err != nil {
		return Activity{}, fmt.Errorf("create activity log: %w", err)
	}

	if userID.Valid {
		value := userID.Int64
		activity.UserID = &value
	}

	if entityID.Valid {
		value := entityID.Int64
		activity.EntityID = &value
	}

	activity.Metadata = json.RawMessage(storedMetadata)

	return activity, nil
}
