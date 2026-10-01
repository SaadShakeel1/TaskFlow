package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type TaskStatus string

const (
	TaskStatusTODO       TaskStatus = "todo"
	TaskStatusBlocked    TaskStatus = "blocked"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusDone       TaskStatus = "done"
)

type TaskPriority string

const (
	TaskPriorityLow    TaskPriority = "low"
	TaskPriorityMedium TaskPriority = "medium"
	TaskPriorityHigh   TaskPriority = "high"
	TaskPriorityUrgent TaskPriority = "urgent"
)

type Task struct {
	ID                 int64
	ProjectID          int64
	CreatedByUserID    int64
	AssigneeID         *int64
	Title              string
	Description        string
	AcceptanceCriteria string
	Status             TaskStatus
	BlockedReason      *string
	Priority           TaskPriority
	DueDate            *time.Time
	EstimateMinutes    *int
	StartedAt          *time.Time
	CompletedAt        *time.Time
	Version            int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type TaskModel struct {
	DB *sql.DB
}

type TaskCreateInput struct {
	AssigneeID         *int64
	Title              string
	Description        string
	AcceptanceCriteria string
	Priority           TaskPriority
	DueDate            *time.Time
	EstimateMinutes    *int
}

func IsValidTaskPriority(priority TaskPriority) bool {
	switch priority {
	case TaskPriorityLow, TaskPriorityMedium, TaskPriorityHigh, TaskPriorityUrgent:
		return true
	default:
		return false
	}
}

func (m *TaskModel) Create(ctx context.Context, projectID int64, createdByUserID int64, input TaskCreateInput) (Task, error) {
	if !IsValidTaskPriority(input.Priority) {
		return Task{}, fmt.Errorf("invalid task priority: %q", input.Priority)
	}
	const query = `
		INSERT INTO tasks (
			project_id,
			created_by_user_id,
			assignee_id,
			title,
			description,
			acceptance_criteria,
			priority,
			due_date,
			estimate_minutes
		)
		SELECT
			$1,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9
		WHERE EXISTS (
			SELECT 1
			FROM projects p
			WHERE p.id = $1
			AND (
				p.user_id = $2
				OR EXISTS (
					SELECT 1
					FROM project_members pm
					WHERE pm.project_id = p.id
						AND pm.user_id = $2
						AND pm.role = 'editor'
				)
			)
		)
		AND (
			$3::BIGINT IS NULL
			OR EXISTS (
				SELECT 1
				FROM projects p
				WHERE p.id = $1
				AND (
					p.user_id = $3
					OR EXISTS (
						SELECT 1
						FROM project_members pm
						WHERE pm.project_id = p.id
							AND pm.user_id = $3
							AND pm.role = 'editor'
					)
				)
			)
		)
		RETURNING
			id,
			project_id,
			created_by_user_id,
			assignee_id,
			title,
			description,
			acceptance_criteria,
			status,
			priority,
			due_date,
			estimate_minutes,
			blocked_reason,
			started_at,
			completed_at,
			version,
			created_at,
			updated_at;
	`

	var task Task

	err := m.DB.QueryRowContext(
		ctx,
		query,
		projectID,
		createdByUserID,
		input.AssigneeID,
		input.Title,
		input.Description,
		input.AcceptanceCriteria,
		string(input.Priority),
		input.DueDate,
		input.EstimateMinutes,
	).Scan(
		&task.ID,
		&task.ProjectID,
		&task.CreatedByUserID,
		&task.AssigneeID,
		&task.Title,
		&task.Description,
		&task.AcceptanceCriteria,
		&task.Status,
		&task.Priority,
		&task.DueDate,
		&task.EstimateMinutes,
		&task.BlockedReason,
		&task.StartedAt,
		&task.CompletedAt,
		&task.Version,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Task{}, ErrNoRecord
		}
		return Task{}, fmt.Errorf("create task: %w", err)
	}

	return task, nil
}

func (m *TaskModel) GetByIDForUser(ctx context.Context, taskID int64, userID int64) (Task, error) {
	const query = `
	SELECT 
		t.id,                                            
		t.project_id,                                    
		t.created_by_user_id,                            
		t.assignee_id,                                   
		t.title,                                         
		t.description,                                   
		t.acceptance_criteria,                           
		t.status,                                        
		t.priority,                                      
		t.due_date,
		t.estimate_minutes,                              
		t.blocked_reason,                                
		t.started_at,                                    
		t.completed_at,                                  
		t.version,                                       
		t.created_at,                                    
		t.updated_at    
	FROM tasks t
	WHERE t.id = $1
	AND EXISTS(
		SELECT 1 FROM
		projects p
		WHERE p.id = t.project_id
		AND(
			p.user_id = $2
			OR EXISTS (
				SELECT 1 FROM
				project_members pm
				WHERE pm.user_id = $2
				AND p.id = pm.project_id
			)
		)
	)
	`
	var task Task
	err := m.DB.QueryRowContext(ctx, query, taskID, userID).Scan(
		&task.ID,
		&task.ProjectID,
		&task.CreatedByUserID,
		&task.AssigneeID,
		&task.Title,
		&task.Description,
		&task.AcceptanceCriteria,
		&task.Status,
		&task.Priority,
		&task.DueDate,
		&task.EstimateMinutes,
		&task.BlockedReason,
		&task.StartedAt,
		&task.CompletedAt,
		&task.Version,
		&task.CreatedAt,
		&task.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Task{}, ErrNoRecord
		}
		return Task{}, fmt.Errorf("get task by ID for user: %w", err)
	}
	return task, nil
}
