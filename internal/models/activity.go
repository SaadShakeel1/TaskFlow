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

type ActivityListOptions struct {
	Page       int
	PageSize   int
	Action     *string
	EntityType *string
	EntityID   *int64
}

type ActivityList struct {
	Activities []Activity
	TotalItems int
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

func (m *ActivityModel) ListByProjectForUser(
	ctx context.Context,
	projectID int64,
	userID int64,
	options ActivityListOptions,
) (ActivityList, error) {
	if projectID < 1 || userID < 1 {
		return ActivityList{}, fmt.Errorf("IDs must be positive integers")
	}

	if options.Page < 1 {
		return ActivityList{}, fmt.Errorf("page must be at least 1")
	}

	if options.PageSize < 1 {
		return ActivityList{}, fmt.Errorf("page size must be at least 1")
	}

	whereClause := `
		WHERE al.project_id = $1
		AND EXISTS (
			SELECT 1
			FROM projects p
			WHERE p.id = al.project_id
			AND (
				p.user_id = $2
				OR EXISTS (
					SELECT 1
					FROM project_members pm
					WHERE pm.project_id = p.id
					AND pm.user_id = $2
				)
			)
		)
	`

	whereArgs := []any{projectID, userID}

	if options.Action != nil {
		action := strings.TrimSpace(*options.Action)
		if action == "" {
			return ActivityList{}, fmt.Errorf("activity action cannot be empty")
		}

		position := len(whereArgs) + 1
		whereClause += fmt.Sprintf(" AND al.action = $%d", position)
		whereArgs = append(whereArgs, action)
	}

	if options.EntityType != nil {
		entityType := strings.TrimSpace(*options.EntityType)
		if entityType == "" {
			return ActivityList{}, fmt.Errorf("entity type cannot be empty")
		}

		position := len(whereArgs) + 1
		whereClause += fmt.Sprintf(" AND al.entity_type = $%d", position)
		whereArgs = append(whereArgs, entityType)
	}

	if options.EntityID != nil {
		if *options.EntityID < 1 {
			return ActivityList{}, fmt.Errorf("entity ID must be positive")
		}

		position := len(whereArgs) + 1
		whereClause += fmt.Sprintf(" AND al.entity_id = $%d", position)
		whereArgs = append(whereArgs, *options.EntityID)
	}

	countQuery := `SELECT COUNT(*) FROM activity_logs al ` + whereClause

	var totalItems int
	if err := m.DB.QueryRowContext(
		ctx,
		countQuery,
		whereArgs...,
	).Scan(&totalItems); err != nil {
		return ActivityList{}, fmt.Errorf("count activity logs: %w", err)
	}

	offset := (options.Page - 1) * options.PageSize
	limitPosition := len(whereArgs) + 1
	offsetPosition := len(whereArgs) + 2

	query := fmt.Sprintf(`
		SELECT
			al.id,
			al.project_id,
			al.user_id,
			al.action,
			al.entity_type,
			al.entity_id,
			al.metadata,
			al.created_at
		FROM activity_logs al
		%s
		ORDER BY al.created_at DESC, al.id DESC
		LIMIT $%d OFFSET $%d
	`,
		whereClause,
		limitPosition,
		offsetPosition,
	)

	queryArgs := append([]any{}, whereArgs...)
	queryArgs = append(queryArgs, options.PageSize, offset)

	rows, err := m.DB.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return ActivityList{}, fmt.Errorf("list activity logs: %w", err)
	}
	defer rows.Close()

	activities := make([]Activity, 0)

	for rows.Next() {
		var activity Activity
		var userIDValue sql.NullInt64
		var entityIDValue sql.NullInt64
		var storedMetadata []byte

		if err := rows.Scan(
			&activity.ID,
			&activity.ProjectID,
			&userIDValue,
			&activity.Action,
			&activity.EntityType,
			&entityIDValue,
			&storedMetadata,
			&activity.CreatedAt,
		); err != nil {
			return ActivityList{}, fmt.Errorf("scan activity log: %w", err)
		}

		if userIDValue.Valid {
			value := userIDValue.Int64
			activity.UserID = &value
		}

		if entityIDValue.Valid {
			value := entityIDValue.Int64
			activity.EntityID = &value
		}

		activity.Metadata = json.RawMessage(storedMetadata)
		activities = append(activities, activity)
	}

	if err := rows.Err(); err != nil {
		return ActivityList{}, fmt.Errorf("iterate activity logs: %w", err)
	}

	return ActivityList{
		Activities: activities,
		TotalItems: totalItems,
	}, nil
}
