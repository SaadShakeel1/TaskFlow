package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type ProjectStatus string

const (
	ProjectStatusActive    ProjectStatus = "active"
	ProjectStatusCompleted ProjectStatus = "completed"
	ProjectStatusArchived  ProjectStatus = "archived"
)

type Project struct {
	ID          int64
	UserID      int64
	Name        string
	Description string
	Status      ProjectStatus
	TargetDate  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ProjectUpdate struct {
	Name        string
	Description string
	Status      ProjectStatus
	TargetDate  *time.Time
}

type ProjectModel struct {
	DB *sql.DB
}

func (m *ProjectModel) Create(ctx context.Context, userID int64, name string, description string, targetDate *time.Time) (Project, error) {
	const query = `
		INSERT INTO projects (user_id, name, description, target_date)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, name, description, status, target_date, created_at, updated_at
	`

	var project Project

	err := m.DB.QueryRowContext(ctx, query, userID, name, description, targetDate).Scan(
		&project.ID,
		&project.UserID,
		&project.Name,
		&project.Description,
		&project.Status,
		&project.TargetDate,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if err != nil {
		return Project{}, fmt.Errorf("create project: %w", err)
	}

	return project, nil
}

func (m *ProjectModel) GetByIDForUser(ctx context.Context, projectID, userID int64) (Project, error) {
	const query = `
		SELECT id, user_id, name, description, status,
			target_date, created_at, updated_at
		FROM projects
		WHERE id = $1 AND user_id = $2
	`

	var project Project

	err := m.DB.QueryRowContext(ctx, query, projectID, userID).Scan(
		&project.ID,
		&project.UserID,
		&project.Name,
		&project.Description,
		&project.Status,
		&project.TargetDate,
		&project.CreatedAt,
		&project.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrNoRecord
		}
		return Project{}, fmt.Errorf("get project by ID for user: %w", err)
	}

	return project, nil
}

func (m *ProjectModel) ListByUser(ctx context.Context, userID int64) ([]Project, error) {
	const query = `
		SELECT id, user_id, name, description, status,
			target_date, created_at, updated_at
		FROM projects
		WHERE user_id = $1
		ORDER BY updated_at DESC, id DESC
	`

	rows, err := m.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list projects by user: %w", err)
	}
	defer rows.Close()

	projects := make([]Project, 0)

	for rows.Next() {
		var project Project

		err := rows.Scan(
			&project.ID,
			&project.UserID,
			&project.Name,
			&project.Description,
			&project.Status,
			&project.TargetDate,
			&project.CreatedAt,
			&project.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}

		projects = append(projects, project)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}

	return projects, nil
}

func (m *ProjectModel) DeleteByIDForUser(ctx context.Context, projectID, userID int64) error {
	const query = `
		DELETE FROM projects
		WHERE user_id = $1 AND id = $2
	`

	result, err := m.DB.ExecContext(ctx, query, userID, projectID)

	if err != nil {
		return fmt.Errorf("delete project by ID for user: %w", err)
	}

	rowAffected, err := result.RowsAffected()

	if err != nil {
		return fmt.Errorf("get deleted project row count: %w", err)
	}

	if rowAffected == 0 {
		return ErrNoRecord
	}

	return nil
}

func (m *ProjectModel) UpdateByIDForUser(ctx context.Context, projectID int64, userID int64, update ProjectUpdate) (Project, error) {
	const query = `
		UPDATE projects
		SET name = $1,
			description = $2,
			status = $3,
			target_date = $4,
			updated_at = NOW()
		WHERE id = $5 AND user_id = $6
		RETURNING id, user_id, name, description, status,
			target_date, created_at, updated_at
	`

	var project Project

	err := m.DB.QueryRowContext(ctx, query, update.Name, update.Description, update.Status, update.TargetDate, projectID, userID).Scan(
		&project.ID, &project.UserID, &project.Name, &project.Description, &project.Status, &project.TargetDate, &project.CreatedAt, &project.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrNoRecord
		}
		return Project{}, fmt.Errorf("update project by ID for user: %w", err)
	}

	return project, nil
}
