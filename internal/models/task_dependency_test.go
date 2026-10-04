package models_test

import (
	"context"
	"errors"
	"testing"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
	"github.com/SaadShakeel1/TaskFlow/internal/testutil"
)

func TestTaskDependencyModelCreate(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}
	projects := &models.ProjectModel{DB: db}
	tasks := &models.TaskModel{DB: db}
	dependencies := &models.TaskDependencyModel{DB: db}

	owner, err := users.Create(ctx, "dependency-owner@example.com", "Dependency Owner", "correct-password")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	project, err := projects.Create(ctx, owner.ID, "Dependency project", "Project for dependency testing", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	taskA, err := tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{Title: "Task A", Priority: models.TaskPriorityMedium})
	if err != nil {
		t.Fatalf("create task A: %v", err)
	}

	taskB, err := tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{Title: "Task B", Priority: models.TaskPriorityMedium})
	if err != nil {
		t.Fatalf("create task B: %v", err)
	}

	taskC, err := tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{Title: "Task C", Priority: models.TaskPriorityMedium})
	if err != nil {
		t.Fatalf("create task C: %v", err)
	}

	dependency, err := dependencies.Create(ctx, taskB.ID, taskA.ID, owner.ID)
	if err != nil {
		t.Fatalf("create dependency B -> A: %v", err)
	}

	if dependency.TaskID != taskB.ID {
		t.Errorf("task ID = %d, want %d", dependency.TaskID, taskB.ID)
	}

	if dependency.DependsOnTaskID != taskA.ID {
		t.Errorf("depends-on task ID = %d, want %d", dependency.DependsOnTaskID, taskA.ID)
	}

	if _, err = dependencies.Create(ctx, taskB.ID, taskA.ID, owner.ID); !errors.Is(err, models.ErrDuplicateTaskDependency) {
		t.Errorf("duplicate dependency error = %v, want ErrDuplicateTaskDependency", err)
	}

	if _, err = dependencies.Create(ctx, taskA.ID, taskA.ID, owner.ID); !errors.Is(err, models.ErrTaskDependencyCycle) {
		t.Errorf("self dependency error = %v, want ErrTaskDependencyCycle", err)
	}

	if _, err = dependencies.Create(ctx, taskC.ID, taskB.ID, owner.ID); err != nil {
		t.Fatalf("create dependency C -> B: %v", err)
	}

	// Existing chain: C -> B -> A. Adding A -> C would create a cycle.
	if _, err = dependencies.Create(ctx, taskA.ID, taskC.ID, owner.ID); !errors.Is(err, models.ErrTaskDependencyCycle) {
		t.Errorf("circular dependency error = %v, want ErrTaskDependencyCycle", err)
	}
}

func TestTaskDependencyModelRejectsCrossProjectDependency(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}
	projects := &models.ProjectModel{DB: db}
	tasks := &models.TaskModel{DB: db}
	dependencies := &models.TaskDependencyModel{DB: db}

	owner, err := users.Create(ctx, "cross-project-owner@example.com", "Cross Project Owner", "correct-password")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	firstProject, err := projects.Create(ctx, owner.ID, "First project", "First project", nil)
	if err != nil {
		t.Fatalf("create first project: %v", err)
	}

	secondProject, err := projects.Create(ctx, owner.ID, "Second project", "Second project", nil)
	if err != nil {
		t.Fatalf("create second project: %v", err)
	}

	firstTask, err := tasks.Create(ctx, firstProject.ID, owner.ID, models.TaskCreateInput{Title: "First project task", Priority: models.TaskPriorityMedium})
	if err != nil {
		t.Fatalf("create first task: %v", err)
	}

	secondTask, err := tasks.Create(ctx, secondProject.ID, owner.ID, models.TaskCreateInput{Title: "Second project task", Priority: models.TaskPriorityMedium})
	if err != nil {
		t.Fatalf("create second task: %v", err)
	}

	if _, err = dependencies.Create(ctx, firstTask.ID, secondTask.ID, owner.ID); !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("cross-project error = %v, want ErrNoRecord", err)
	}
}
