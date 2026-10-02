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

type TaskListOptions struct {
	Page     int
	PageSize int
	Status   *TaskStatus
	Priority *TaskPriority
	Sort     TaskSort
	Order    SortOrder
}

type TaskList struct {
	Tasks      []Task
	TotalItems int
}

type TaskUpdateInput struct {
	Title              string
	Description        string
	AcceptanceCriteria string
	AssigneeID         *int64
	Priority           TaskPriority
	DueDate            *time.Time
	EstimateMinutes    *int
	Status             TaskStatus
	BlockedReason      *string
	ExpectedVersion    int
}

type TaskSort string

const (
	TaskSortUpdatedAt TaskSort = "updatedAt"
	TaskSortCreatedAt TaskSort = "createdAt"
	TaskSortDueDate   TaskSort = "dueDate"
	TaskSortPriority  TaskSort = "priority"
)

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

func isValidStatus(status TaskStatus) bool {
	switch status {
	case TaskStatusTODO, TaskStatusBlocked, TaskStatusDone, TaskStatusInProgress:
		return true
	default:
		return false
	}
}

func taskListOrderBy(sort TaskSort, order SortOrder) (string, error) {
	if order != SortOrderAscending && order != SortOrderDescending {
		return "", fmt.Errorf("invalid task sort order: %q", order)
	}
	switch sort {
	case TaskSortUpdatedAt:
		if order == SortOrderAscending {
			return "t.updated_at ASC", nil
		}
		return "t.updated_at DESC", nil
	case TaskSortCreatedAt:
		if order == SortOrderAscending {
			return "t.created_at ASC", nil
		}
		return "t.created_at DESC", nil
	case TaskSortDueDate:
		if order == SortOrderAscending {
			return "t.due_date ASC NULL LAST, t.id ASC", nil
		}
		return "t.due_date DESC NULL LAST, t.id DESC", nil
	case TaskSortPriority:
		if order == SortOrderAscending {
			return `CASE t.priority
				WHEN 'urgent' THEN 1
				WHEN 'high' THEN 2
				WHEN 'medium' THEN 3
				WHEN 'low' THEN 4
			END ASC,
			t.id ASC`, nil
		}
		return `CASE t.priority
				WHEN 'urgent' THEN 1
				WHEN 'high' THEN 2
				WHEN 'medium' THEN 3
				WHEN 'low' THEN 4
			END DESC,
			t.id DESC`, nil
	default:
		return "", fmt.Errorf("invalid task sort: %q", sort)
	}

}

func (m *TaskModel) ListByProjectForUser(ctx context.Context, projectID int64, userID int64, options TaskListOptions) (TaskList, error) {
	if options.Page < 1 {
		return TaskList{}, fmt.Errorf("page must be at least 1")
	}

	if options.PageSize < 1 {
		return TaskList{}, fmt.Errorf("page size must be at least 1")
	}

	orderBy, err := taskListOrderBy(options.Sort, options.Order)

	if err != nil {
		return TaskList{}, err
	}

	whereClause := `
		WHERE t.project_id = $1
		AND EXISTS (
			SELECT 1
			FROM projects p
			WHERE p.id = t.project_id
			AND (
				p.user_id = $2
					OR EXISTS (
							SELECT 1
							FROM project_members pm
							WHERE pm.project_id = p.id
							AND pm.user_id = $2
					)
			)
	)`

	whereArgs := []any{projectID, userID}

	if options.Status != nil {
		if !isValidStatus(*options.Status) {
			return TaskList{}, fmt.Errorf("invalid task status: %q", *options.Status)
		}
		position := len(whereArgs) + 1
		whereClause += fmt.Sprintf(" AND t.status = $%d", position)
		whereArgs = append(whereArgs, string(*options.Status))
	}

	if options.Priority != nil {
		if !IsValidTaskPriority(*options.Priority) {
			return TaskList{}, fmt.Errorf(
				"invalid task priority: %q",
				*options.Priority,
			)
		}

		position := len(whereArgs) + 1
		whereClause += fmt.Sprintf(
			" AND t.priority = $%d",
			position,
		)
		whereArgs = append(whereArgs, string(*options.Priority))
	}

	countQuery := `SELECT COUNT(*) FROM tasks t` + whereClause
	var totalItems int
	if err := m.DB.QueryRowContext(ctx, countQuery, whereArgs...).Scan(&totalItems); err != nil {
		return TaskList{}, fmt.Errorf("count tasks by project: %w", err)
	}

	limitPosition := len(whereArgs) + 1
	offsetPosition := len(whereArgs) + 2
	offset := (options.Page - 1) * options.PageSize

	query := fmt.Sprintf(`
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
		%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
		`, whereClause, orderBy, limitPosition, offsetPosition)

	queryArgs := append([]any{}, whereArgs)
	queryArgs = append(whereArgs, options.PageSize, offset)

	rows, err := m.DB.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return TaskList{}, fmt.Errorf("list tasks by project: %w", err)
	}
	defer rows.Close()

	tasks := make([]Task, 0)

	for rows.Next() {
		var task Task

		if err := rows.Scan(
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
		); err != nil {
			return TaskList{}, fmt.Errorf("scan task: %w", err)
		}

		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return TaskList{}, fmt.Errorf("iterate tasks: %w", err)
	}

	return TaskList{
		Tasks:      tasks,
		TotalItems: totalItems,
	}, nil
}

func (m *TaskModel) UpdateByIDForUser(ctx context.Context, taskID int64, userID int64, input TaskUpdateInput) (Task, error) {
	if !IsValidTaskPriority(input.Priority) {
		return Task{}, fmt.Errorf("invalid task priority: %q", input.Priority)
	}

	if !isValidStatus(input.Status) {
		return Task{}, fmt.Errorf("invalid task status: %q", input.Status)
	}

	const query = `
		UPDATE tasks t
		SET 
		title = $1,                                                      
		description = $2,                                                
		acceptance_criteria = $3,                                        
		assignee_id = $4,                                                
		priority = $5,                                                   
		due_date = $6,                                                   
		estimate_minutes = $7,                                           
		status = $8,       
		blocked_reason = CASE 
			WHEN $8 = 'blocked' THEN $9
			ELSE NULL
		END,
		started_at = CASE
			WHEN $8 in ('in_progress','blocked','done') 
			AND started_at IS NULL THEN NOW()
			ELSE t.started_at
		END,
		completed_at = CASE                                              
			WHEN $8 = 'done'                                         
			THEN COALESCE(t.completed_at, NOW())                     
			ELSE NULL                                                
		END,                                                             
		version = t.version + 1,                                         
		updated_at = NOW()                                               
		WHERE t.id = $10
		AND t.version = $11
		AND EXISTS (                                                             
			SELECT 1                                                         
			FROM projects p                                                  
			WHERE p.id = t.project_id                                        
			AND (                                                            
				p.user_id = $12                                          
				OR EXISTS (                                              
					SELECT 1                                         
					FROM project_members pm                          
					WHERE pm.project_id = p.id                       
					AND pm.user_id = $12                             
					AND pm.role = 'editor'                           
			)                                                        
		)                                                                
	)     
	RETURNING                                                                
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

	`
	var task Task
	err := m.DB.QueryRowContext(
		ctx,
		query,
		input.Title,
		input.Description,
		input.AcceptanceCriteria,
		input.AssigneeID,
		string(input.Priority),
		input.DueDate,
		input.EstimateMinutes,
		string(input.Status),
		input.BlockedReason,
		taskID,
		input.ExpectedVersion,
		userID,
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
		return Task{}, fmt.Errorf("update task by ID for user: %w", err)
	}

	return task, nil
}
