package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
	"github.com/SaadShakeel1/TaskFlow/internal/testutil"
)

func TestCreateTaskDependencyEndpoint(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)
	owner, ownerToken := createAuthenticatedTestUser(t, app, ctx)

	project, err := app.projects.Create(ctx, owner.ID, "Dependency API project", "Project for dependency endpoint testing", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	taskA, err := app.tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{Title: "Task A", Priority: models.TaskPriorityMedium})
	if err != nil {
		t.Fatalf("create task A: %v", err)
	}

	taskB, err := app.tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{Title: "Task B", Priority: models.TaskPriorityMedium})
	if err != nil {
		t.Fatalf("create task B: %v", err)
	}

	url := fmt.Sprintf("/projects/%d/tasks/%d/dependencies", project.ID, taskB.ID)
	body := strings.NewReader(fmt.Sprintf(`{"dependsOnTaskId":%d}`, taskA.ID))

	request := httptest.NewRequest(http.MethodPost, url, body)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: ownerToken})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusCreated)
	}

	var response taskDependencyResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.TaskID != taskB.ID {
		t.Errorf("task ID = %d, want %d", response.TaskID, taskB.ID)
	}
	if response.DependsOnTaskID != taskA.ID {
		t.Errorf("depends-on task ID = %d, want %d", response.DependsOnTaskID, taskA.ID)
	}

	duplicateRequest := httptest.NewRequest(http.MethodPost, url, strings.NewReader(fmt.Sprintf(`{"dependsOnTaskId":%d}`, taskA.ID)))
	duplicateRequest.Header.Set("Content-Type", "application/json")
	duplicateRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: ownerToken})

	duplicateRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(duplicateRecorder, duplicateRequest)

	if duplicateRecorder.Code != http.StatusConflict {
		t.Errorf("duplicate status code = %d, want %d", duplicateRecorder.Code, http.StatusConflict)
	}
}

func TestCreateTaskDependencyEndpointRejectsCycle(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)
	owner, ownerToken := createAuthenticatedTestUser(t, app, ctx)

	project, err := app.projects.Create(ctx, owner.ID, "Cycle API project", "Project for cycle testing", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	taskA, err := app.tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{Title: "Task A", Priority: models.TaskPriorityMedium})
	if err != nil {
		t.Fatalf("create task A: %v", err)
	}

	taskB, err := app.tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{Title: "Task B", Priority: models.TaskPriorityMedium})
	if err != nil {
		t.Fatalf("create task B: %v", err)
	}

	if _, err := app.taskDependencies.Create(ctx, taskB.ID, taskA.ID, owner.ID); err != nil {
		t.Fatalf("create initial dependency: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/projects/%d/tasks/%d/dependencies", project.ID, taskA.ID),
		strings.NewReader(fmt.Sprintf(`{"dependsOnTaskId":%d}`, taskB.ID)),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: ownerToken})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status code = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestCreateTaskDependencyEndpointRejectsViewer(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)
	owner, _ := createAuthenticatedTestUser(t, app, ctx)
	viewer, viewerToken := createProjectMemberTestUser(t, app, ctx, "dependency-viewer@example.com", "Dependency Viewer")

	project, err := app.projects.Create(ctx, owner.ID, "Viewer dependency project", "Viewer cannot create dependencies", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if _, err := app.projectMembers.Create(ctx, project.ID, viewer.ID, models.ProjectMemberRoleViewer); err != nil {
		t.Fatalf("create viewer membership: %v", err)
	}

	taskA, err := app.tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{Title: "Task A", Priority: models.TaskPriorityMedium})
	if err != nil {
		t.Fatalf("create task A: %v", err)
	}

	taskB, err := app.tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{Title: "Task B", Priority: models.TaskPriorityMedium})
	if err != nil {
		t.Fatalf("create task B: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/projects/%d/tasks/%d/dependencies", project.ID, taskB.ID),
		strings.NewReader(fmt.Sprintf(`{"dependsOnTaskId":%d}`, taskA.ID)),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: viewerToken})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}
