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

type ProjectSort string

const (
	ProjectSortUpdatedAt  ProjectSort = "updatedAt"
	ProjectSortName       ProjectSort = "name"
	ProjectSortTargetDate ProjectSort = "targetDate"
)

type SortOrder string

const (
	SortOrderAscending  SortOrder = "asc"
	SortOrderDescending SortOrder = "desc"
)

type ProjectListOptions struct {
	Page     int
	PageSize int
	Status   *ProjectStatus
	Sort     ProjectSort
	Order    SortOrder
}

type ProjectList struct {
	Projects   []Project
	TotalItems int
}

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

func isValidProjectStatus(status ProjectStatus) bool {
	switch status {
	case ProjectStatusActive, ProjectStatusCompleted, ProjectStatusArchived:
		return true
	default:
		return false
	}

}

func projectListOrderBy(sort ProjectSort, order SortOrder) (string, error) {
	if order != SortOrderAscending && order != SortOrderDescending {
		return "", fmt.Errorf("invalid sort order: %q", order)
	}
	switch sort {
	case ProjectSortUpdatedAt:
		if order == SortOrderAscending {
			return "updated_at ASC, id ASC", nil
		}
		return "updated_at DESC, id DESC", nil

	case ProjectSortName:
		if order == SortOrderAscending {
			return "name ASC, id ASC", nil
		}
		return "name DESC, id DESC", nil

	case ProjectSortTargetDate:
		if order == SortOrderAscending {
			return "target_date ASC NULLS LAST, id ASC", nil
		}
		return "target_date DESC NULLS LAST, id DESC", nil

	default:
		return "", fmt.Errorf("invalid project sort: %q", sort)
	}
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
		WHERE id = $1 AND (
			user_id = $2 OR EXISTS (
				SELECT 1
				FROM project_members
				WHERE project_members.project_id = projects.id
					AND project_members.user_id = $2
			)
		)
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
		WHERE id = $5 AND (
			user_id = $6 OR EXISTS (
				SELECT 1
				FROM project_members
				WHERE project_members.project_id = projects.id
					AND project_members.user_id = $6
					AND project_members.role = 'editor'
			)
		)
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
func (m *ProjectModel) ListByUser(ctx context.Context, userID int64, options ProjectListOptions) (ProjectList, error) {
	if options.Page < 1 {
		return ProjectList{}, fmt.Errorf("page must be at least 1")
	}

	if options.PageSize < 1 {
		return ProjectList{}, fmt.Errorf("page size must be at least 1")
	}

	orderBy, err := projectListOrderBy(options.Sort, options.Order)
	if err != nil {
		return ProjectList{}, err
	}

	whereClause := `WHERE (
		user_id = $1 OR EXISTS (
			SELECT 1
			FROM project_members
			WHERE project_members.project_id = projects.id
				AND project_members.user_id = $1
		)
	)`
	whereArgs := []any{userID}

	if options.Status != nil {
		if !isValidProjectStatus(*options.Status) {
			return ProjectList{}, fmt.Errorf("invalid project status: %q", *options.Status)
		}

		whereClause += ` AND status = $2`
		whereArgs = append(whereArgs, string(*options.Status))
	}

	countQuery := `SELECT COUNT(*) FROM projects ` + whereClause

	var totalItems int
	if err := m.DB.QueryRowContext(ctx, countQuery, whereArgs...).Scan(&totalItems); err != nil {
		return ProjectList{}, fmt.Errorf("count projects by user: %w", err)
	}

	offset := (options.Page - 1) * options.PageSize
	limitPosition := len(whereArgs) + 1
	offsetPosition := len(whereArgs) + 2

	query := fmt.Sprintf(`
		SELECT id, user_id, name, description, status,
		       target_date, created_at, updated_at
		FROM projects %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d`,
		whereClause, orderBy, limitPosition, offsetPosition,
	)

	queryArgs := append([]any{}, whereArgs...)
	queryArgs = append(queryArgs, options.PageSize, offset)

	rows, err := m.DB.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return ProjectList{}, fmt.Errorf("list projects by user: %w", err)
	}
	defer rows.Close()

	projects := make([]Project, 0)

	for rows.Next() {
		var project Project

		if err := rows.Scan(
			&project.ID,
			&project.UserID,
			&project.Name,
			&project.Description,
			&project.Status,
			&project.TargetDate,
			&project.CreatedAt,
			&project.UpdatedAt,
		); err != nil {
			return ProjectList{}, fmt.Errorf("scan project: %w", err)
		}

		projects = append(projects, project)
	}

	if err := rows.Err(); err != nil {
		return ProjectList{}, fmt.Errorf("iterate projects: %w", err)
	}

	return ProjectList{Projects: projects, TotalItems: totalItems}, nil
}

