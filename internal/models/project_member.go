package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type ProjectMemberRole string

const (
	ProjectMemberRoleOwner  ProjectMemberRole = "owner"
	ProjectMemberRoleEditor ProjectMemberRole = "editor"
	ProjectMemberRoleViewer ProjectMemberRole = "viewer"
)

type ProjectMember struct {
	ProjectID int64
	UserID    int64
	Name      string
	Email     string
	Role      ProjectMemberRole
	CreatedAt time.Time
}

type ProjectMemberModel struct {
	DB *sql.DB
}

func IsValidProjectMemberRole(role ProjectMemberRole) bool {
	return role == ProjectMemberRoleEditor || role == ProjectMemberRoleViewer
}

func (m *ProjectMemberModel) Create(ctx context.Context, projectID, userID int64, role ProjectMemberRole) (ProjectMember, error) {
	if !IsValidProjectMemberRole(role) {
		return ProjectMember{}, fmt.Errorf("invalid project member role: %q", role)
	}

	const query = `
		INSERT INTO project_members (project_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (project_id, user_id) DO NOTHING
		RETURNING project_id, user_id, role, created_at
	`

	var member ProjectMember
	var storedRole string

	err := m.DB.QueryRowContext(ctx, query, projectID, userID, role).Scan(
		&member.ProjectID,
		&member.UserID,
		&storedRole,
		&member.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProjectMember{}, ErrDuplicateProjectMember
		}
		return ProjectMember{}, fmt.Errorf("create project member: %w", err)
	}

	member.Role = ProjectMemberRole(storedRole)
	return member, nil
}

func (m *ProjectMemberModel) ListByProject(ctx context.Context, projectID int64) ([]ProjectMember, error) {
	const query = `
		SELECT project_id, user_id, name, email, role, created_at
		FROM (
			SELECT p.id AS project_id, u.id AS user_id, u.name, u.email,
				'owner'::text AS role, p.created_at
			FROM projects p
			JOIN users u ON u.id = p.user_id
			WHERE p.id = $1

			UNION ALL

			SELECT pm.project_id, u.id, u.name, u.email, pm.role, pm.created_at
			FROM project_members pm
			JOIN users u ON u.id = pm.user_id
			WHERE pm.project_id = $1
		) AS members
		ORDER BY
			CASE role
				WHEN 'owner' THEN 0
				WHEN 'editor' THEN 1
				ELSE 2
			END,
			name ASC,
			user_id ASC
	`

	rows, err := m.DB.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project members: %w", err)
	}
	defer rows.Close()

	members := make([]ProjectMember, 0)
	for rows.Next() {
		var member ProjectMember
		var role string

		if err := rows.Scan(
			&member.ProjectID,
			&member.UserID,
			&member.Name,
			&member.Email,
			&role,
			&member.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan project member: %w", err)
		}

		member.Role = ProjectMemberRole(role)
		members = append(members, member)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project members: %w", err)
	}

	return members, nil
}

func (m *ProjectMemberModel) UpdateRole(ctx context.Context, projectID, userID int64, role ProjectMemberRole) (ProjectMember, error) {
	if !IsValidProjectMemberRole(role) {
		return ProjectMember{}, fmt.Errorf("invalid project member role: %q", role)
	}

	const query = `
		UPDATE project_members
		SET role = $1
		WHERE project_id = $2 AND user_id = $3
		RETURNING project_id, user_id, role, created_at
	`

	var member ProjectMember
	var storedRole string

	err := m.DB.QueryRowContext(ctx, query, role, projectID, userID).Scan(
		&member.ProjectID,
		&member.UserID,
		&storedRole,
		&member.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProjectMember{}, ErrNoRecord
		}
		return ProjectMember{}, fmt.Errorf("update project member role: %w", err)
	}

	member.Role = ProjectMemberRole(storedRole)
	return member, nil
}

func (m *ProjectMemberModel) Delete(ctx context.Context, projectID, userID int64) error {
	const query = `
		DELETE FROM project_members
		WHERE project_id = $1 AND user_id = $2
	`

	result, err := m.DB.ExecContext(ctx, query, projectID, userID)
	if err != nil {
		return fmt.Errorf("delete project member: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get deleted project member row count: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNoRecord
	}

	return nil
}
