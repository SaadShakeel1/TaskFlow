package models_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
	"github.com/SaadShakeel1/TaskFlow/internal/testutil"
)

func TestTaskModelCreate(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}

	user, err := users.Create(ctx, "task-creator@example.com", "Task Creator", "correct-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	projects := &models.ProjectModel{DB: db}

	project, err := projects.Create(ctx, user.ID, "TaskFlow API", "Build the TaskFlow backend", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	dueDate := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)
	estimateMinutes := 120

	tasks := &models.TaskModel{DB: db}

	got, err := tasks.Create(ctx, project.ID, user.ID, models.TaskCreateInput{
		AssigneeID:         &user.ID,
		Title:              "Build task creation",
		Description:        "Add the first Task model method",
		AcceptanceCriteria: "A Task can be stored and returned from PostgreSQL",
		Priority:           models.TaskPriorityHigh,
		DueDate:            &dueDate,
		EstimateMinutes:    &estimateMinutes,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	if got.ID < 1 {
		t.Errorf("ID = %d, want a positive value", got.ID)
	}
	if got.ProjectID != project.ID {
		t.Errorf("ProjectID = %d, want %d", got.ProjectID, project.ID)
	}
	if got.CreatedByUserID != user.ID {
		t.Errorf("CreatedByUserID = %d, want %d", got.CreatedByUserID, user.ID)
	}
	if got.AssigneeID == nil || *got.AssigneeID != user.ID {
		t.Errorf("AssigneeID = %v, want %d", got.AssigneeID, user.ID)
	}
	if got.Title != "Build task creation" {
		t.Errorf("Title = %q, want task title", got.Title)
	}
	if got.Description != "Add the first Task model method" {
		t.Errorf("Description = %q, want task description", got.Description)
	}
	if got.AcceptanceCriteria != "A Task can be stored and returned from PostgreSQL" {
		t.Errorf("AcceptanceCriteria = %q, want acceptance criteria", got.AcceptanceCriteria)
	}
	if got.Status != models.TaskStatusTODO {
		t.Errorf("Status = %q, want %q", got.Status, models.TaskStatusTODO)
	}
	if got.Priority != models.TaskPriorityHigh {
		t.Errorf("Priority = %q, want %q", got.Priority, models.TaskPriorityHigh)
	}
	if got.DueDate == nil || got.DueDate.Format(time.DateOnly) != dueDate.Format(time.DateOnly) {
		t.Errorf("DueDate = %v, want %s", got.DueDate, dueDate.Format(time.DateOnly))
	}
	if got.EstimateMinutes == nil || *got.EstimateMinutes != estimateMinutes {
		t.Errorf("EstimateMinutes = %v, want %d", got.EstimateMinutes, estimateMinutes)
	}
	if got.BlockedReason != nil {
		t.Errorf("BlockedReason = %v, want nil for a new todo Task", got.BlockedReason)
	}
	if got.StartedAt != nil {
		t.Errorf("StartedAt = %v, want nil for a new todo Task", got.StartedAt)
	}
	if got.CompletedAt != nil {
		t.Errorf("CompletedAt = %v, want nil for a new todo Task", got.CompletedAt)
	}
	if got.Version != 1 {
		t.Errorf("Version = %d, want 1", got.Version)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero, want a PostgreSQL-generated timestamp")
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt is zero, want a PostgreSQL-generated timestamp")
	}
}

func TestTaskModelCreateAllowsEditorsButRejectsViewers(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}
	projects := &models.ProjectModel{DB: db}
	members := &models.ProjectMemberModel{DB: db}
	tasks := &models.TaskModel{DB: db}

	owner, err := users.Create(ctx, "task-owner@example.com", "Task Owner", "correct-password")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	editor, err := users.Create(ctx, "task-editor@example.com", "Task Editor", "correct-password")
	if err != nil {
		t.Fatalf("create editor: %v", err)
	}

	viewer, err := users.Create(ctx, "task-viewer@example.com", "Task Viewer", "correct-password")
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}

	project, err := projects.Create(ctx, owner.ID, "Shared TaskFlow project", "Test Task permissions", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	_, err = members.Create(ctx, project.ID, editor.ID, models.ProjectMemberRoleEditor)
	if err != nil {
		t.Fatalf("add editor: %v", err)
	}

	_, err = members.Create(ctx, project.ID, viewer.ID, models.ProjectMemberRoleViewer)
	if err != nil {
		t.Fatalf("add viewer: %v", err)
	}

	editorTask, err := tasks.Create(ctx, project.ID, editor.ID, models.TaskCreateInput{
		Title:    "Editor-created Task",
		Priority: models.TaskPriorityMedium,
	})
	if err != nil {
		t.Fatalf("editor create task: %v", err)
	}

	if editorTask.CreatedByUserID != editor.ID {
		t.Errorf("editor-created Task CreatedByUserID = %d, want %d", editorTask.CreatedByUserID, editor.ID)
	}

	_, err = tasks.Create(ctx, project.ID, viewer.ID, models.TaskCreateInput{
		Title:    "Viewer must not create",
		Priority: models.TaskPriorityLow,
	})
	if !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("viewer create error = %v, want models.ErrNoRecord", err)
	}
}

func TestTaskModelCreateRejectsViewerAssignee(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}
	projects := &models.ProjectModel{DB: db}
	members := &models.ProjectMemberModel{DB: db}
	tasks := &models.TaskModel{DB: db}

	owner, err := users.Create(ctx, "assignee-owner@example.com", "Assignee Owner", "correct-password")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	viewer, err := users.Create(ctx, "assignee-viewer@example.com", "Assignee Viewer", "correct-password")
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}

	project, err := projects.Create(ctx, owner.ID, "Assignment rules", "Only contributors can receive Tasks", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	_, err = members.Create(ctx, project.ID, viewer.ID, models.ProjectMemberRoleViewer)
	if err != nil {
		t.Fatalf("add viewer: %v", err)
	}

	_, err = tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{
		AssigneeID: &viewer.ID,
		Title:      "Viewer must not be assigned",
		Priority:   models.TaskPriorityMedium,
	})
	if !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("viewer-assignee create error = %v, want models.ErrNoRecord", err)
	}
}

func TestTaskModelListByProjectForUser(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)

	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}
	projects := &models.ProjectModel{DB: db}
	members := &models.ProjectMemberModel{DB: db}
	tasks := &models.TaskModel{DB: db}

	owner, err := users.Create(
		ctx,
		"list-owner@example.com",
		"List Owner",
		"correct-password",
	)
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	viewer, err := users.Create(
		ctx,
		"list-viewer@example.com",
		"List Viewer",
		"correct-password",
	)
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}

	outsider, err := users.Create(
		ctx,
		"list-outsider@example.com",
		"List Outsider",
		"correct-password",
	)
	if err != nil {
		t.Fatalf("create outsider: %v", err)
	}

	project, err := projects.Create(
		ctx,
		owner.ID,
		"Task list project",
		"Project for list testing",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if _, err := members.Create(
		ctx,
		project.ID,
		viewer.ID,
		models.ProjectMemberRoleViewer,
	); err != nil {
		t.Fatalf("create viewer membership: %v", err)
	}

	urgentTask, err := tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{
		Title:    "Urgent task",
		Priority: models.TaskPriorityUrgent,
	})
	if err != nil {
		t.Fatalf("create urgent task: %v", err)
	}

	highTask, err := tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{
		Title:    "High task",
		Priority: models.TaskPriorityHigh,
	})
	if err != nil {
		t.Fatalf("create high task: %v", err)
	}

	_, err = tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{
		Title:    "Low task",
		Priority: models.TaskPriorityLow,
	})
	if err != nil {
		t.Fatalf("create low task: %v", err)
	}

	// Blocked tasks must include a blocked reason because of the database constraint.
	if _, err := db.ExecContext(
		ctx,
		`UPDATE tasks
		 SET status = 'blocked',
		     blocked_reason = 'Waiting for deployment access'
		 WHERE id = $1`,
		urgentTask.ID,
	); err != nil {
		t.Fatalf("mark task blocked: %v", err)
	}

	list, err := tasks.ListByProjectForUser(
		ctx,
		project.ID,
		owner.ID,
		models.TaskListOptions{
			Page:     1,
			PageSize: 2,
			Sort:     models.TaskSortPriority,
			Order:    models.SortOrderAscending,
		},
	)
	if err != nil {
		t.Fatalf("list tasks for owner: %v", err)
	}

	if list.TotalItems != 3 {
		t.Errorf("total items = %d, want 3", list.TotalItems)
	}

	if len(list.Tasks) != 2 {
		t.Fatalf("returned tasks = %d, want 2", len(list.Tasks))
	}

	if list.Tasks[0].ID != urgentTask.ID {
		t.Errorf(
			"first task ID = %d, want urgent task %d",
			list.Tasks[0].ID,
			urgentTask.ID,
		)
	}

	if list.Tasks[1].ID != highTask.ID {
		t.Errorf(
			"second task ID = %d, want high task %d",
			list.Tasks[1].ID,
			highTask.ID,
		)
	}

	filtered, err := tasks.ListByProjectForUser(
		ctx,
		project.ID,
		owner.ID,
		models.TaskListOptions{
			Page:     1,
			PageSize: 20,
			Status:   taskStatusPointer(models.TaskStatusBlocked),
			Sort:     models.TaskSortUpdatedAt,
			Order:    models.SortOrderDescending,
		},
	)
	if err != nil {
		t.Fatalf("list blocked tasks: %v", err)
	}

	if filtered.TotalItems != 1 {
		t.Errorf("blocked total items = %d, want 1", filtered.TotalItems)
	}

	if len(filtered.Tasks) != 1 || filtered.Tasks[0].ID != urgentTask.ID {
		t.Errorf("blocked tasks = %#v, want urgent task", filtered.Tasks)
	}

	viewerList, err := tasks.ListByProjectForUser(
		ctx,
		project.ID,
		viewer.ID,
		models.TaskListOptions{
			Page:     1,
			PageSize: 20,
			Sort:     models.TaskSortCreatedAt,
			Order:    models.SortOrderAscending,
		},
	)
	if err != nil {
		t.Fatalf("list tasks for viewer: %v", err)
	}

	if viewerList.TotalItems != 3 {
		t.Errorf("viewer total items = %d, want 3", viewerList.TotalItems)
	}

	outsideList, err := tasks.ListByProjectForUser(
		ctx,
		project.ID,
		outsider.ID,
		models.TaskListOptions{
			Page:     1,
			PageSize: 20,
			Sort:     models.TaskSortUpdatedAt,
			Order:    models.SortOrderDescending,
		},
	)
	if err != nil {
		t.Fatalf("list tasks for outsider: %v", err)
	}

	if outsideList.TotalItems != 0 {
		t.Errorf("outsider total items = %d, want 0", outsideList.TotalItems)
	}

	if len(outsideList.Tasks) != 0 {
		t.Errorf("outsider tasks = %d, want 0", len(outsideList.Tasks))
	}
}

func taskStatusPointer(status models.TaskStatus) *models.TaskStatus {
	return &status
}

func TestTaskModelUpdateByIDForUser(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}
	projects := &models.ProjectModel{DB: db}
	tasks := &models.TaskModel{DB: db}

	owner, err := users.Create(
		ctx,
		"update-owner@example.com",
		"Update Owner",
		"correct-password",
	)
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	project, err := projects.Create(
		ctx,
		owner.ID,
		"Update project",
		"Project for update testing",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	task, err := tasks.Create(
		ctx,
		project.ID,
		owner.ID,
		models.TaskCreateInput{
			Title:    "Original title",
			Priority: models.TaskPriorityMedium,
		},
	)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	updated, err := tasks.UpdateByIDForUser(
		ctx,
		task.ID,
		owner.ID,
		models.TaskUpdateInput{
			Title:              "Updated title",
			Description:        "Updated description",
			AcceptanceCriteria: "Updated acceptance criteria",
			Priority:           models.TaskPriorityHigh,
			Status:             models.TaskStatusInProgress,
			ExpectedVersion:    task.Version,
		},
	)
	if err != nil {
		t.Fatalf("update task: %v", err)
	}

	if updated.Title != "Updated title" {
		t.Errorf("title = %q, want %q", updated.Title, "Updated title")
	}

	if updated.Description != "Updated description" {
		t.Errorf(
			"description = %q, want %q",
			updated.Description,
			"Updated description",
		)
	}

	if updated.Priority != models.TaskPriorityHigh {
		t.Errorf(
			"priority = %q, want %q",
			updated.Priority,
			models.TaskPriorityHigh,
		)
	}

	if updated.Status != models.TaskStatusInProgress {
		t.Errorf(
			"status = %q, want %q",
			updated.Status,
			models.TaskStatusInProgress,
		)
	}

	if updated.StartedAt == nil {
		t.Error("startedAt is nil, want timestamp for in-progress task")
	}

	if updated.Version != task.Version+1 {
		t.Errorf(
			"version = %d, want %d",
			updated.Version,
			task.Version+1,
		)
	}

	_, err = tasks.UpdateByIDForUser(
		ctx,
		task.ID,
		owner.ID,
		models.TaskUpdateInput{
			Title:           "Stale update",
			Priority:        models.TaskPriorityLow,
			Status:          models.TaskStatusInProgress,
			ExpectedVersion: task.Version,
		},
	)
	if !errors.Is(err, models.ErrNoRecord) {
		t.Errorf(
			"stale update error = %v, want models.ErrNoRecord",
			err,
		)
	}
}

func TestTaskModelDeleteByIDForUser(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}
	projects := &models.ProjectModel{DB: db}
	members := &models.ProjectMemberModel{DB: db}
	tasks := &models.TaskModel{DB: db}

	owner, err := users.Create(ctx, "delete-owner@example.com", "Delete Owner", "correct-password")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	viewer, err := users.Create(ctx, "delete-viewer@example.com", "Delete Viewer", "correct-password")
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}

	project, err := projects.Create(ctx, owner.ID, "Delete project", "Project for delete testing", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if _, err := members.Create(ctx, project.ID, viewer.ID, models.ProjectMemberRoleViewer); err != nil {
		t.Fatalf("create viewer membership: %v", err)
	}

	staleTask, err := tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{
		Title:    "Stale delete task",
		Priority: models.TaskPriorityMedium,
	})
	if err != nil {
		t.Fatalf("create stale task: %v", err)
	}

	if err := tasks.DeleteByIDForUser(ctx, staleTask.ID, owner.ID, staleTask.Version+1); !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("stale delete error = %v, want models.ErrNoRecord", err)
	}

	if _, err := tasks.GetByIDForUser(ctx, staleTask.ID, owner.ID); err != nil {
		t.Fatalf("stale task should still exist: %v", err)
	}

	viewerTask, err := tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{
		Title:    "Viewer delete task",
		Priority: models.TaskPriorityMedium,
	})
	if err != nil {
		t.Fatalf("create viewer task: %v", err)
	}

	if err := tasks.DeleteByIDForUser(ctx, viewerTask.ID, viewer.ID, viewerTask.Version); !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("viewer delete error = %v, want models.ErrNoRecord", err)
	}

	if err := tasks.DeleteByIDForUser(ctx, viewerTask.ID, owner.ID, viewerTask.Version); err != nil {
		t.Fatalf("owner delete: %v", err)
	}

	if _, err := tasks.GetByIDForUser(ctx, viewerTask.ID, owner.ID); !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("deleted task lookup error = %v, want models.ErrNoRecord", err)
	}
}
