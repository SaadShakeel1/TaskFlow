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

func TestGetTaskEndpointReturnsTask(t *testing.T) {
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
		"Task project",
		"Project for retrieving a task",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	task, err := app.tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{
		Title:              "Review API design",
		Description:        "Review the current API design",
		AcceptanceCriteria: "All endpoints are documented",
		Priority:           models.TaskPriorityHigh,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/projects/%d/tasks/%d", project.ID, task.ID),
		nil,
	)
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: ownerToken,
	})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response taskResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.ID != task.ID {
		t.Errorf("task ID = %d, want %d", response.ID, task.ID)
	}

	if response.ProjectID != project.ID {
		t.Errorf("project ID = %d, want %d", response.ProjectID, project.ID)
	}

	if response.Title != "Review API design" {
		t.Errorf("title = %q, want %q", response.Title, "Review API design")
	}

	if response.Priority != models.TaskPriorityHigh {
		t.Errorf(
			"priority = %q, want %q",
			response.Priority,
			models.TaskPriorityHigh,
		)
	}
}

func TestGetTaskEndpointAllowsViewer(t *testing.T) {
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
		"task-viewer@example.com",
		"Task Viewer",
	)

	project, err := app.projects.Create(
		ctx,
		owner.ID,
		"Shared task project",
		"Project visible to viewer",
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

	task, err := app.tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{
		Title:    "Visible task",
		Priority: models.TaskPriorityMedium,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/projects/%d/tasks/%d", project.ID, task.ID),
		nil,
	)
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: viewerToken,
	})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestGetTaskEndpointRejectsTaskFromAnotherProject(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)

	owner, ownerToken := createAuthenticatedTestUser(t, app, ctx)

	firstProject, err := app.projects.Create(
		ctx,
		owner.ID,
		"First project",
		"First project",
		nil,
	)
	if err != nil {
		t.Fatalf("create first project: %v", err)
	}

	secondProject, err := app.projects.Create(
		ctx,
		owner.ID,
		"Second project",
		"Second project",
		nil,
	)
	if err != nil {
		t.Fatalf("create second project: %v", err)
	}

	task, err := app.tasks.Create(ctx, firstProject.ID, owner.ID, models.TaskCreateInput{
		Title:    "First project task",
		Priority: models.TaskPriorityMedium,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/projects/%d/tasks/%d", secondProject.ID, task.ID),
		nil,
	)
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: ownerToken,
	})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Errorf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusNotFound,
		)
	}
}

func TestGetTaskEndpointRejectsInvalidIDs(t *testing.T) {
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
		"Task project",
		"Project for invalid ID testing",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	tests := []struct {
		name string
		path string
		want int
	}{
		{
			name: "invalid task ID",
			path: fmt.Sprintf(
				"/projects/%d/tasks/not-a-number",
				project.ID,
			),
			want: http.StatusBadRequest,
		},
		{
			name: "missing task",
			path: fmt.Sprintf(
				"/projects/%d/tasks/999999",
				project.ID,
			),
			want: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodGet,
				test.path,
				nil,
			)
			request.AddCookie(&http.Cookie{
				Name:  sessionCookieName,
				Value: ownerToken,
			})

			recorder := httptest.NewRecorder()
			app.routes().ServeHTTP(recorder, request)

			if recorder.Code != test.want {
				t.Errorf(
					"status code = %d, want %d",
					recorder.Code,
					test.want,
				)
			}
		})
	}
}

func TestListTasksEndpoint(t *testing.T) {
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
		"Task list API project",
		"Test task listing",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	urgentTask, err := app.tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{
		Title:    "Urgent task",
		Priority: models.TaskPriorityUrgent,
	})
	if err != nil {
		t.Fatalf("create urgent task: %v", err)
	}

	_, err = app.tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{
		Title:    "High task",
		Priority: models.TaskPriorityHigh,
	})
	if err != nil {
		t.Fatalf("create high task: %v", err)
	}

	_, err = app.tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{
		Title:    "Low task",
		Priority: models.TaskPriorityLow,
	})
	if err != nil {
		t.Fatalf("create low task: %v", err)
	}

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

	request := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf(
			"/projects/%d/tasks?page=1&pageSize=2&sort=priority&order=asc",
			project.ID,
		),
		nil,
	)
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: ownerToken,
	})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response taskListResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Pagination.Page != 1 {
		t.Errorf("page = %d, want 1", response.Pagination.Page)
	}

	if response.Pagination.PageSize != 2 {
		t.Errorf("page size = %d, want 2", response.Pagination.PageSize)
	}

	if response.Pagination.TotalItems != 3 {
		t.Errorf("total items = %d, want 3", response.Pagination.TotalItems)
	}

	if response.Pagination.TotalPages != 2 {
		t.Errorf("total pages = %d, want 2", response.Pagination.TotalPages)
	}

	if len(response.Tasks) != 2 {
		t.Fatalf("returned tasks = %d, want 2", len(response.Tasks))
	}

	if response.Tasks[0].Priority != models.TaskPriorityUrgent {
		t.Errorf(
			"first priority = %q, want %q",
			response.Tasks[0].Priority,
			models.TaskPriorityUrgent,
		)
	}

	filterRequest := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf(
			"/projects/%d/tasks?status=blocked",
			project.ID,
		),
		nil,
	)
	filterRequest.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: ownerToken,
	})

	filterRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(filterRecorder, filterRequest)

	if filterRecorder.Code != http.StatusOK {
		t.Fatalf(
			"filtered status code = %d, want %d",
			filterRecorder.Code,
			http.StatusOK,
		)
	}

	var filteredResponse taskListResponse
	if err := json.NewDecoder(filterRecorder.Body).Decode(&filteredResponse); err != nil {
		t.Fatalf("decode filtered response: %v", err)
	}

	if filteredResponse.Pagination.TotalItems != 1 {
		t.Errorf(
			"filtered total items = %d, want 1",
			filteredResponse.Pagination.TotalItems,
		)
	}

	if len(filteredResponse.Tasks) != 1 {
		t.Fatalf("filtered tasks = %d, want 1", len(filteredResponse.Tasks))
	}

	if filteredResponse.Tasks[0].Status != models.TaskStatusBlocked {
		t.Errorf(
			"filtered status = %q, want %q",
			filteredResponse.Tasks[0].Status,
			models.TaskStatusBlocked,
		)
	}
}

func TestListTasksEndpointAllowsViewer(t *testing.T) {
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
		"list-viewer-api@example.com",
		"List Viewer API",
	)

	project, err := app.projects.Create(
		ctx,
		owner.ID,
		"Viewer task project",
		"Viewer can list tasks",
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

	_, err = app.tasks.Create(ctx, project.ID, owner.ID, models.TaskCreateInput{
		Title:    "Visible task",
		Priority: models.TaskPriorityMedium,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/projects/%d/tasks", project.ID),
		nil,
	)
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: viewerToken,
	})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response taskListResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Pagination.TotalItems != 1 {
		t.Errorf("total items = %d, want 1", response.Pagination.TotalItems)
	}
}

func TestListTasksEndpointRejectsInvalidQueryParameters(t *testing.T) {
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
		"Invalid list project",
		"Test invalid task list parameters",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	tests := []struct {
		name  string
		query string
	}{
		{name: "invalid page", query: "?page=0"},
		{name: "invalid page size", query: "?pageSize=101"},
		{name: "invalid status", query: "?status=unknown"},
		{name: "invalid priority", query: "?priority=critical"},
		{name: "invalid sort", query: "?sort=unknown"},
		{name: "invalid order", query: "?order=random"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodGet,
				fmt.Sprintf("/projects/%d/tasks%s", project.ID, test.query),
				nil,
			)
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
		})
	}
}

func TestUpdateTaskEndpoint(t *testing.T) {
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
		"Update API project",
		"Project for update endpoint testing",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	task, err := app.tasks.Create(
		ctx,
		project.ID,
		owner.ID,
		models.TaskCreateInput{
			Title:    "Original task",
			Priority: models.TaskPriorityMedium,
		},
	)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("/projects/%d/tasks/%d", project.ID, task.ID),
		strings.NewReader(`{
			"title": "Updated task",
			"description": "Updated description",
			"priority": "high",
			"status": "in_progress",
			"version": 1
		}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: ownerToken,
	})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}

	var response taskResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Title != "Updated task" {
		t.Errorf(
			"title = %q, want %q",
			response.Title,
			"Updated task",
		)
	}

	if response.Status != models.TaskStatusInProgress {
		t.Errorf(
			"status = %q, want %q",
			response.Status,
			models.TaskStatusInProgress,
		)
	}

	if response.Priority != models.TaskPriorityHigh {
		t.Errorf(
			"priority = %q, want %q",
			response.Priority,
			models.TaskPriorityHigh,
		)
	}

	if response.StartedAt == nil {
		t.Error("startedAt is nil, want timestamp")
	}

	if response.Version != 2 {
		t.Errorf("version = %d, want 2", response.Version)
	}

	staleRequest := httptest.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("/projects/%d/tasks/%d", project.ID, task.ID),
		strings.NewReader(`{
			"title": "Stale update",
			"version": 1
		}`),
	)
	staleRequest.Header.Set("Content-Type", "application/json")
	staleRequest.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: ownerToken,
	})

	staleRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(staleRecorder, staleRequest)

	if staleRecorder.Code != http.StatusConflict {
		t.Errorf(
			"stale status code = %d, want %d",
			staleRecorder.Code,
			http.StatusConflict,
		)
	}
}

func TestUpdateTaskEndpointRejectsViewer(t *testing.T) {
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
		"update-viewer@example.com",
		"Update Viewer",
	)

	project, err := app.projects.Create(
		ctx,
		owner.ID,
		"Viewer update project",
		"Viewer must not update tasks",
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

	task, err := app.tasks.Create(
		ctx,
		project.ID,
		owner.ID,
		models.TaskCreateInput{
			Title:    "Protected task",
			Priority: models.TaskPriorityMedium,
		},
	)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("/projects/%d/tasks/%d", project.ID, task.ID),
		strings.NewReader(`{
			"title": "Viewer update",
			"version": 1
		}`),
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

func TestUpdateTaskEndpointRejectsInvalidWorkflow(t *testing.T) {
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
		"Workflow project",
		"Project for workflow validation",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	task, err := app.tasks.Create(
		ctx,
		project.ID,
		owner.ID,
		models.TaskCreateInput{
			Title:    "Workflow task",
			Priority: models.TaskPriorityMedium,
		},
	)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("/projects/%d/tasks/%d", project.ID, task.ID),
		strings.NewReader(`{
			"status": "blocked",
			"version": 1
		}`),
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
}
