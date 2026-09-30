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
