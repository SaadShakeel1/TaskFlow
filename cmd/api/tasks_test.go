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

func TestCreateTaskEndpoint(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)

	owner, ownerToken := createAuthenticatedTestUser(t, app, ctx)

	project, err := app.projects.Create(
		ctx,
		owner.ID,
		"TaskFlow launch",
		"Prepare the first release",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	body := fmt.Sprintf(`{
		"assigneeId": %d,
		"title": "Prepare release checklist",
		"description": "Document all release steps",
		"acceptanceCriteria": "Every release step is documented",
		"priority": "high",
		"dueDate": "2026-11-15",
		"estimateMinutes": 120
	}`, owner.ID)

	request := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/projects/%d/tasks", project.ID),
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: ownerToken,
	})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusCreated)
	}

	var response taskResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.ID < 1 {
		t.Errorf("task ID = %d, want positive value", response.ID)
	}

	if response.ProjectID != project.ID {
		t.Errorf("project ID = %d, want %d", response.ProjectID, project.ID)
	}

	if response.CreatedByUserID != owner.ID {
		t.Errorf(
			"created by user ID = %d, want %d",
			response.CreatedByUserID,
			owner.ID,
		)
	}

	if response.AssigneeID == nil || *response.AssigneeID != owner.ID {
		t.Errorf("assignee ID = %v, want %d", response.AssigneeID, owner.ID)
	}

	if response.Title != "Prepare release checklist" {
		t.Errorf(
			"title = %q, want %q",
			response.Title,
			"Prepare release checklist",
		)
	}

	if response.Description != "Document all release steps" {
		t.Errorf(
			"description = %q, want %q",
			response.Description,
			"Document all release steps",
		)
	}

	if response.AcceptanceCriteria != "Every release step is documented" {
		t.Errorf(
			"acceptance criteria = %q, want expected value",
			response.AcceptanceCriteria,
		)
	}

	if response.Status != models.TaskStatusTODO {
		t.Errorf(
			"status = %q, want %q",
			response.Status,
			models.TaskStatusTODO,
		)
	}

	if response.Priority != models.TaskPriorityHigh {
		t.Errorf(
			"priority = %q, want %q",
			response.Priority,
			models.TaskPriorityHigh,
		)
	}

	if response.DueDate == nil || *response.DueDate != "2026-11-15" {
		t.Errorf(
			"due date = %v, want %q",
			response.DueDate,
			"2026-11-15",
		)
	}

	if response.EstimateMinutes == nil || *response.EstimateMinutes != 120 {
		t.Errorf(
			"estimate minutes = %v, want 120",
			response.EstimateMinutes,
		)
	}

	if response.Version != 1 {
		t.Errorf("version = %d, want 1", response.Version)
	}
}

func TestCreateTaskEndpointAllowsEditor(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)

	owner, _ := createAuthenticatedTestUser(t, app, ctx)
	editor, editorToken := createProjectMemberTestUser(
		t,
		app,
		ctx,
		"editor-task@example.com",
		"Task Editor",
	)

	project, err := app.projects.Create(
		ctx,
		owner.ID,
		"Shared task project",
		"Project for editor testing",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if _, err := app.projectMembers.Create(
		ctx,
		project.ID,
		editor.ID,
		models.ProjectMemberRoleEditor,
	); err != nil {
		t.Fatalf("create editor membership: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/projects/%d/tasks", project.ID),
		strings.NewReader(`{"title":"Editor task"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: editorToken,
	})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusCreated,
		)
	}

	var response taskResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Title != "Editor task" {
		t.Errorf("title = %q, want %q", response.Title, "Editor task")
	}

	if response.Priority != models.TaskPriorityMedium {
		t.Errorf(
			"priority = %q, want default %q",
			response.Priority,
			models.TaskPriorityMedium,
		)
	}
}

func TestCreateTaskEndpointRejectsViewer(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)

	owner, _ := createAuthenticatedTestUser(t, app, ctx)
	viewer, viewerToken := createProjectMemberTestUser(
		t,
		app,
		ctx,
		"viewer-task@example.com",
		"Task Viewer",
	)

	project, err := app.projects.Create(
		ctx,
		owner.ID,
		"Viewer project",
		"Project for permission testing",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if _, err := app.projectMembers.Create(
		ctx,
		project.ID,
		viewer.ID,
		models.ProjectMemberRoleViewer,
	); err != nil {
		t.Fatalf("create viewer membership: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/projects/%d/tasks", project.ID),
		strings.NewReader(`{"title":"Viewer task"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: viewerToken,
	})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Errorf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusForbidden,
		)
	}
}

func TestCreateTaskEndpointRejectsInvalidInput(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)

	owner, ownerToken := createAuthenticatedTestUser(t, app, ctx)

	project, err := app.projects.Create(
		ctx,
		owner.ID,
		"Validation project",
		"Project for task validation",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "blank title",
			body: `{"title":"   "}`,
			want: "task title is required\n",
		},
		{
			name: "invalid priority",
			body: `{"title":"Valid title","priority":"critical"}`,
			want: "task priority is invalid\n",
		},
		{
			name: "invalid due date",
			body: `{"title":"Valid title","dueDate":"15-11-2026"}`,
			want: "due date must use YYYY-MM-DD\n",
		},
		{
			name: "invalid estimate",
			body: `{"title":"Valid title","estimateMinutes":0}`,
			want: "estimate minutes must be a positive integer\n",
		},
		{
			name: "unknown field",
			body: `{"title":"Valid title","unknown":"value"}`,
			want: "request body must be valid JSON\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPost,
				fmt.Sprintf("/projects/%d/tasks", project.ID),
				strings.NewReader(test.body),
			)
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(&http.Cookie{
				Name:  sessionCookieName,
				Value: ownerToken,
			})

			recorder := httptest.NewRecorder()
			app.routes().ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Errorf(
					"status code = %d, want %d",
					recorder.Code,
					http.StatusBadRequest,
				)
			}

			if got := recorder.Body.String(); got != test.want {
				t.Errorf(
					"response body = %q, want %q",
					got,
					test.want,
				)
			}
		})
	}
}

func TestCreateTaskEndpointRejectsIneligibleAssignee(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)

	owner, ownerToken := createAuthenticatedTestUser(t, app, ctx)
	otherUser, _ := createProjectMemberTestUser(
		t,
		app,
		ctx,
		"outside-task@example.com",
		"Outside User",
	)

	project, err := app.projects.Create(
		ctx,
		owner.ID,
		"Assignment project",
		"Project for assignment testing",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	body := fmt.Sprintf(`{
		"title": "Invalid assignment",
		"assigneeId": %d
	}`, otherUser.ID)

	request := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/projects/%d/tasks", project.ID),
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: ownerToken,
	})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Errorf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusForbidden,
		)
	}
}
