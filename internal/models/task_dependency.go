package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type TaskDependency struct {
	TaskID          int64
	DependsOnTaskID int64
	CreatedAt       time.Time
}

type TaskDependencyModel struct {
	DB *sql.DB
}

func (m *TaskDependencyModel) Create(
	ctx context.Context,
	taskID int64,
	dependsOnTaskID int64,
	userID int64,
) (TaskDependency, error) {
	if taskID < 1 || dependsOnTaskID < 1 || userID < 1 {
		return TaskDependency{}, fmt.Errorf("IDs must be positive integers")
	}

	if taskID == dependsOnTaskID {
		return TaskDependency{}, ErrTaskDependencyCycle
	}

	tx, err := m.DB.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return TaskDependency{}, fmt.Errorf(
			"begin dependency transaction: %w",
			err,
		)
	}
	defer tx.Rollback()

	var canEdit bool

	err = tx.QueryRowContext(
		ctx,
		`
		SELECT EXISTS (
			SELECT 1
			FROM tasks dependent
			JOIN tasks prerequisite
				ON prerequisite.id = $2
				AND prerequisite.project_id = dependent.project_id
			JOIN projects p
				ON p.id = dependent.project_id
			WHERE dependent.id = $1
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
		`,
		taskID,
		dependsOnTaskID,
		userID,
	).Scan(&canEdit)

	if err != nil {
		return TaskDependency{}, fmt.Errorf(
			"check dependency permission: %w",
			err,
		)
	}

	if !canEdit {
		return TaskDependency{}, ErrNoRecord
	}

	var createsCycle bool

	err = tx.QueryRowContext(
		ctx,
		`
		WITH RECURSIVE prerequisite_chain(task_id) AS (
			SELECT $2::BIGINT

			UNION

			SELECT td.depends_on_task_id
			FROM task_dependencies td
			JOIN prerequisite_chain pc
				ON td.task_id = pc.task_id
		)
		SELECT EXISTS (
			SELECT 1
			FROM prerequisite_chain
			WHERE task_id = $1
		)
		`,
		taskID,
		dependsOnTaskID,
	).Scan(&createsCycle)

	if err != nil {
		return TaskDependency{}, fmt.Errorf(
			"check dependency cycle: %w",
			err,
		)
	}

	if createsCycle {
		return TaskDependency{}, ErrTaskDependencyCycle
	}

	var dependency TaskDependency

	err = tx.QueryRowContext(
		ctx,
		`
		INSERT INTO task_dependencies (
			task_id,
			depends_on_task_id
		)
		VALUES ($1, $2)
		RETURNING task_id, depends_on_task_id, created_at
		`,
		taskID,
		dependsOnTaskID,
	).Scan(
		&dependency.TaskID,
		&dependency.DependsOnTaskID,
		&dependency.CreatedAt,
	)

	if err != nil {
		var postgresError *pgconn.PgError

		if errors.As(err, &postgresError) &&
			postgresError.Code == "23505" {
			return TaskDependency{}, ErrDuplicateTaskDependency
		}

		return TaskDependency{}, fmt.Errorf(
			"create task dependency: %w",
			err,
		)
	}

	if err := tx.Commit(); err != nil {
		return TaskDependency{}, fmt.Errorf(
			"commit task dependency: %w",
			err,
		)
	}

	return dependency, nil
}
