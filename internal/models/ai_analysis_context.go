package models

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const (
	aiAnalysisContextSchemaVersion = 1
	aiAnalysisActivityLimit        = 100
)

type ProjectAnalysisContext struct {
	SchemaVersion int                         `json:"schemaVersion"`
	GeneratedAt   time.Time                   `json:"generatedAt"`
	Project       ProjectAnalysisProject      `json:"project"`
	Tasks         []ProjectAnalysisTask       `json:"tasks"`
	Dependencies  []ProjectAnalysisDependency `json:"dependencies"`
	Activities    []ProjectAnalysisActivity   `json:"activities"`
}

type ProjectAnalysisProject struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Status      ProjectStatus `json:"status"`
	TargetDate  *time.Time    `json:"targetDate,omitempty"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
}

type ProjectAnalysisTask struct {
	ID                 int64        `json:"id"`
	AssigneeID         *int64       `json:"assigneeID,omitempty"`
	Title              string       `json:"title"`
	Description        string       `json:"description"`
	AcceptanceCriteria string       `json:"acceptanceCriteria"`
	Status             TaskStatus   `json:"status"`
	BlockedReason      *string      `json:"blockedReason,omitempty"`
	Priority           TaskPriority `json:"priority"`
	DueDate            *time.Time   `json:"dueDate,omitempty"`
	EstimateMinutes    *int         `json:"estimateMinutes,omitempty"`
	StartedAt          *time.Time   `json:"startedAt,omitempty"`
	CompletedAt        *time.Time   `json:"completedAt,omitempty"`
	UpdatedAt          time.Time    `json:"updatedAt"`
}

type ProjectAnalysisDependency struct {
	TaskID          int64 `json:"taskID"`
	DependsOnTaskID int64 `json:"dependsOnTaskID"`
}

type ProjectAnalysisActivity struct {
	ID         int64           `json:"id"`
	UserID     *int64          `json:"userID,omitempty"`
	Action     string          `json:"action"`
	EntityType string          `json:"entityType"`
	EntityID   *int64          `json:"entityID,omitempty"`
	Metadata   json.RawMessage `json:"metadata"`
	CreatedAt  time.Time       `json:"createdAt"`
}

func (m *AIAnalysisRunModel) BuildProjectContext(
	ctx context.Context,
	projectID int64,
	userID int64,
) (ProjectAnalysisContext, error) {
	if projectID < 1 || userID < 1 {
		return ProjectAnalysisContext{}, fmt.Errorf("project and user IDs must be positive")
	}

	project, err := (&ProjectModel{DB: m.DB}).GetByIDForUser(ctx, projectID, userID)
	if err != nil {
		return ProjectAnalysisContext{}, fmt.Errorf("get project for AI analysis: %w", err)
	}

	tasks, err := m.listProjectAnalysisTasks(ctx, projectID)
	if err != nil {
		return ProjectAnalysisContext{}, err
	}

	dependencies, err := m.listProjectAnalysisDependencies(ctx, projectID)
	if err != nil {
		return ProjectAnalysisContext{}, err
	}

	activities, err := m.listProjectAnalysisActivities(ctx, projectID)
	if err != nil {
		return ProjectAnalysisContext{}, err
	}

	return ProjectAnalysisContext{
		SchemaVersion: aiAnalysisContextSchemaVersion,
		GeneratedAt:   time.Now().UTC(),
		Project: ProjectAnalysisProject{
			ID:          project.ID,
			Name:        project.Name,
			Description: project.Description,
			Status:      project.Status,
			TargetDate:  project.TargetDate,
			CreatedAt:   project.CreatedAt,
			UpdatedAt:   project.UpdatedAt,
		},
		Tasks:        tasks,
		Dependencies: dependencies,
		Activities:   activities,
	}, nil
}

func (m *AIAnalysisRunModel) listProjectAnalysisTasks(
	ctx context.Context,
	projectID int64,
) ([]ProjectAnalysisTask, error) {
	const query = `
		SELECT
			id,
			assignee_id,
			title,
			description,
			acceptance_criteria,
			status,
			blocked_reason,
			priority,
			due_date,
			estimate_minutes,
			started_at,
			completed_at,
			updated_at
		FROM tasks
		WHERE project_id = $1
		ORDER BY id ASC
	`

	rows, err := m.DB.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list tasks for AI analysis: %w", err)
	}
	defer rows.Close()

	tasks := make([]ProjectAnalysisTask, 0)

	for rows.Next() {
		var task ProjectAnalysisTask

		if err := rows.Scan(
			&task.ID,
			&task.AssigneeID,
			&task.Title,
			&task.Description,
			&task.AcceptanceCriteria,
			&task.Status,
			&task.BlockedReason,
			&task.Priority,
			&task.DueDate,
			&task.EstimateMinutes,
			&task.StartedAt,
			&task.CompletedAt,
			&task.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan task for AI analysis: %w", err)
		}

		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks for AI analysis: %w", err)
	}

	return tasks, nil
}

func (m *AIAnalysisRunModel) listProjectAnalysisDependencies(
	ctx context.Context,
	projectID int64,
) ([]ProjectAnalysisDependency, error) {
	const query = `
		SELECT
			dependency.task_id,
			dependency.depends_on_task_id
		FROM task_dependencies AS dependency
		INNER JOIN tasks AS task ON task.id = dependency.task_id
		WHERE task.project_id = $1
		ORDER BY dependency.task_id ASC, dependency.depends_on_task_id ASC
	`

	rows, err := m.DB.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list task dependencies for AI analysis: %w", err)
	}
	defer rows.Close()

	dependencies := make([]ProjectAnalysisDependency, 0)

	for rows.Next() {
		var dependency ProjectAnalysisDependency

		if err := rows.Scan(
			&dependency.TaskID,
			&dependency.DependsOnTaskID,
		); err != nil {
			return nil, fmt.Errorf("scan task dependency for AI analysis: %w", err)
		}

		dependencies = append(dependencies, dependency)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task dependencies for AI analysis: %w", err)
	}

	return dependencies, nil
}

func (m *AIAnalysisRunModel) listProjectAnalysisActivities(
	ctx context.Context,
	projectID int64,
) ([]ProjectAnalysisActivity, error) {
	const query = `
		SELECT
			id,
			user_id,
			action,
			entity_type,
			entity_id,
			metadata,
			created_at
		FROM activity_logs
		WHERE project_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`

	rows, err := m.DB.QueryContext(
		ctx,
		query,
		projectID,
		aiAnalysisActivityLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("list activities for AI analysis: %w", err)
	}
	defer rows.Close()

	activities := make([]ProjectAnalysisActivity, 0)

	for rows.Next() {
		var activity ProjectAnalysisActivity
		var metadata []byte

		if err := rows.Scan(
			&activity.ID,
			&activity.UserID,
			&activity.Action,
			&activity.EntityType,
			&activity.EntityID,
			&metadata,
			&activity.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan activity for AI analysis: %w", err)
		}

		activity.Metadata = json.RawMessage(metadata)
		activities = append(activities, activity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activities for AI analysis: %w", err)
	}

	return activities, nil
}
