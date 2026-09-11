package models

import (
	"context"
	"database/sql"
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
