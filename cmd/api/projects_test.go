package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
	"github.com/SaadShakeel1/TaskFlow/internal/testutil"
)

func TestCreateProjectEndpoint(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)
	user, sessionToken := createAuthenticatedTestUser(t, app, ctx)

	request := httptest.NewRequest(
		http.MethodPost,
		"/projects",
		strings.NewReader(`{"name":"Launch TaskFlow","description":"Plan the first public release","targetDate":"2026-12-31"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionToken})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		t.Errorf("status code = %d, want %d", response.StatusCode, http.StatusCreated)
	}

	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	var body struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Status      string `json:"status"`
		TargetDate  string `json:"targetDate"`
	}

	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.ID < 1 {
		t.Errorf("ID = %d, want a positive value", body.ID)
	}
	if body.Name != "Launch TaskFlow" {
		t.Errorf("Name = %q, want %q", body.Name, "Launch TaskFlow")
	}
	if body.Description != "Plan the first public release" {
		t.Errorf("Description = %q, want %q", body.Description, "Plan the first public release")
	}
	if body.Status != string(models.ProjectStatusActive) {
		t.Errorf("Status = %q, want %q", body.Status, models.ProjectStatusActive)
	}
	if body.TargetDate != "2026-12-31" {
		t.Errorf("TargetDate = %q, want %q", body.TargetDate, "2026-12-31")
	}

	stored, err := app.projects.GetByIDForUser(ctx, body.ID, user.ID)
	if err != nil {
		t.Fatalf("get stored project: %v", err)
	}

	if stored.Name != body.Name {
		t.Errorf("stored Name = %q, want %q", stored.Name, body.Name)
	}
}

func TestCreateProjectEndpointRejectsBlankName(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)
	_, sessionToken := createAuthenticatedTestUser(t, app, ctx)

	request := httptest.NewRequest(
		http.MethodPost,
		"/projects",
		strings.NewReader(`{"name":"   ","description":"A description"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionToken})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status code = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	if got := recorder.Body.String(); got != "project name is required\n" {
		t.Errorf("response body = %q, want %q", got, "project name is required\n")
	}
}

func createAuthenticatedTestUser(t *testing.T, app *application, ctx context.Context) (models.User, string) {
	t.Helper()

	user, err := app.users.Create(ctx, "saad@example.com", "Saad", "correct-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	session, err := models.NewSession(user.ID, app.config.sessionTTL)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := app.sessions.Create(ctx, session); err != nil {
		t.Fatalf("store session: %v", err)
	}

	return user, session.Plaintext
}

func TestListProjectsEndpointReturnsOnlyAuthenticatedUsersProjects(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)

	user, sessionToken := createAuthenticatedTestUser(t, app, ctx)

	if _, err := app.projects.Create(ctx, user.ID, "First project", "Created first", nil); err != nil {
		t.Fatalf("create first project: %v", err)
	}

	secondProject, err := app.projects.Create(ctx, user.ID, "Second project", "Created second", nil)
	if err != nil {
		t.Fatalf("create second project: %v", err)
	}

	otherUser, err := app.users.Create(ctx, "other@example.com", "Other User", "correct-password")
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}

	otherProject, err := app.projects.Create(ctx, otherUser.ID, "Private project", "Must not be listed", nil)
	if err != nil {
		t.Fatalf("create other project: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/projects?page=1&pageSize=1", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionToken})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Errorf("status code = %d, want %d", response.StatusCode, http.StatusOK)
	}

	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	var body struct {
		Projects []struct {
			ID     int64                `json:"id"`
			Name   string               `json:"name"`
			Status models.ProjectStatus `json:"status"`
		} `json:"projects"`

		Pagination struct {
			Page       int `json:"page"`
			PageSize   int `json:"pageSize"`
			TotalItems int `json:"totalItems"`
			TotalPages int `json:"totalPages"`
		} `json:"pagination"`
	}

	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(body.Projects) != 1 {
		t.Fatalf("project count = %d, want 1", len(body.Projects))
	}

	if body.Projects[0].ID != secondProject.ID {
		t.Errorf("first project ID = %d, want %d", body.Projects[0].ID, secondProject.ID)
	}

	if body.Projects[0].ID == otherProject.ID {
		t.Errorf("response included a project owned by another user")
	}

	if body.Pagination.Page != 1 {
		t.Errorf("Page = %d, want 1", body.Pagination.Page)
	}

	if body.Pagination.PageSize != 1 {
		t.Errorf("PageSize = %d, want 1", body.Pagination.PageSize)
	}

	if body.Pagination.TotalItems != 2 {
		t.Errorf("TotalItems = %d, want 2", body.Pagination.TotalItems)
	}

	if body.Pagination.TotalPages != 2 {
		t.Errorf("TotalPages = %d, want 2", body.Pagination.TotalPages)
	}
}

func TestGetProjectEndpointReturnsOwnedProject(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)

	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)

	user, sessionToken := createAuthenticatedTestUser(t, app, ctx)

	targetDate := time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)

	project, err := app.projects.Create(ctx, user.ID, "Launch Taskflow", "Plan the first public release", &targetDate)

	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/projects/%d", project.ID), nil)

	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionToken})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Errorf("status code = %d, want %d", response.StatusCode, http.StatusOK)
	}

	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	var body struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Status      string `json:"status"`
		TargetDate  string `json:"targetDate"`
	}

	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.ID != project.ID {
		t.Errorf("ID = %d, want %d", body.ID, project.ID)
	}
	if body.Name != project.Name {
		t.Errorf("Name = %q, want %q", body.Name, project.Name)
	}
	if body.Description != project.Description {
		t.Errorf("Description = %q, want %q", body.Description, project.Description)
	}
	if body.Status != string(models.ProjectStatusActive) {
		t.Errorf("Status = %q, want %q", body.Status, models.ProjectStatusActive)
	}
	if body.TargetDate != "2026-12-31" {
		t.Errorf("TargetDate = %q, want %q", body.TargetDate, "2026-12-31")
	}
}

func TestGetProjectEndpointDoesNotReturnOtherUsersProject(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)

	_, sessionToken := createAuthenticatedTestUser(t, app, ctx)

	otherUser, err := app.users.Create(
		ctx,
		"other@example.com",
		"Other User",
		"correct-password",
	)
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}

	otherProject, err := app.projects.Create(
		ctx,
		otherUser.ID,
		"Private project",
		"This must not be visible to the requesting user",
		nil,
	)
	if err != nil {
		t.Fatalf("create other project: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/projects/%d", otherProject.ID),
		nil,
	)
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: sessionToken,
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

	if got := recorder.Body.String(); got != "project not found\n" {
		t.Errorf(
			"response body = %q, want %q",
			got,
			"project not found\n",
		)
	}
}

func TestListProjectsEndpointUsesDefaultPagination(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)

	user, sessionToken := createAuthenticatedTestUser(t, app, ctx)

	for _, name := range []string{"First project", "Second project"} {
		if _, err := app.projects.Create(ctx, user.ID, name, "Test project", nil); err != nil {
			t.Fatalf("create %q:%v", name, err)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/projects", nil)
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: sessionToken,
	})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Errorf("status code = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body struct {
		Projects []struct {
			ID int64 `json:"id"`
		} `json:"projects"`
		Pagination struct {
			Page       int `json:"page"`
			PageSize   int `json:"pageSize"`
			TotalItems int `json:"totalItems"`
			TotalPages int `json:"totalPages"`
		} `json:"pagination"`
	}

	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(body.Projects) != 2 {
		t.Errorf("project count = %d, want 2", len(body.Projects))
	}

	if body.Pagination.Page != 1 {
		t.Errorf("Page = %d, want 1", body.Pagination.Page)
	}

	if body.Pagination.PageSize != 20 {
		t.Errorf("PageSize = %d, want 20", body.Pagination.PageSize)
	}

	if body.Pagination.TotalItems != 2 {
		t.Errorf("TotalItems = %d, want 2", body.Pagination.TotalItems)
	}

	if body.Pagination.TotalPages != 1 {
		t.Errorf("TotalPages = %d, want 1", body.Pagination.TotalPages)
	}
}

func TestListProjectsEndpointFiltersAndSortsProjects(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)
	user, sessionToken := createAuthenticatedTestUser(t, app, ctx)

	earlyDate := time.Date(2026, time.January, 10, 0, 0, 0, 0, time.UTC)
	lateDate := time.Date(2026, time.December, 20, 0, 0, 0, 0, time.UTC)

	if _, err := app.projects.Create(ctx, user.ID, "Active late project", "Has a later target date", &lateDate); err != nil {
		t.Fatalf("create active late project: %v", err)
	}

	completedProject, err := app.projects.Create(ctx, user.ID, "Completed project", "Must not be returned by the active filter", &earlyDate)
	if err != nil {
		t.Fatalf("create completed project: %v", err)
	}

	update := models.ProjectUpdate{
		Name:        completedProject.Name,
		Description: completedProject.Description,
		Status:      models.ProjectStatusCompleted,
		TargetDate:  completedProject.TargetDate,
	}

	if _, err := app.projects.UpdateByIDForUser(ctx, completedProject.ID, user.ID, update); err != nil {
		t.Fatalf("complete project: %v", err)
	}

	if _, err := app.projects.Create(ctx, user.ID, "Active early project", "Has an earlier target date", &earlyDate); err != nil {
		t.Fatalf("create active early project: %v", err)
	}

	if _, err := app.projects.Create(ctx, user.ID, "Active project without date", "Must sort last", nil); err != nil {
		t.Fatalf("create active project without date: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/projects?status=active&sort=targetDate&order=asc", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionToken})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Errorf("status code = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body struct {
		Projects []struct {
			Name       string               `json:"name"`
			Status     models.ProjectStatus `json:"status"`
			TargetDate *string              `json:"targetDate"`
		} `json:"projects"`
		Pagination struct {
			TotalItems int `json:"totalItems"`
			TotalPages int `json:"totalPages"`
		} `json:"pagination"`
	}

	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(body.Projects) != 3 {
		t.Fatalf("project count = %d, want 3", len(body.Projects))
	}

	wantNames := []string{
		"Active early project",
		"Active late project",
		"Active project without date",
	}

	for i, wantName := range wantNames {
		if got := body.Projects[i].Name; got != wantName {
			t.Errorf("project %d Name = %q, want %q", i, got, wantName)
		}
		if got := body.Projects[i].Status; got != models.ProjectStatusActive {
			t.Errorf("project %d Status = %q, want %q", i, got, models.ProjectStatusActive)
		}
	}

	if body.Projects[2].TargetDate != nil {
		t.Errorf("last project TargetDate = %q, want nil", *body.Projects[2].TargetDate)
	}
	if body.Pagination.TotalItems != 3 {
		t.Errorf("TotalItems = %d, want 3", body.Pagination.TotalItems)
	}
	if body.Pagination.TotalPages != 1 {
		t.Errorf("TotalPages = %d, want 1", body.Pagination.TotalPages)
	}
}

func TestListProjectsEndpointRejectsInvalidQueryParameters(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)
	_, sessionToken := createAuthenticatedTestUser(t, app, ctx)

	tests := []struct {
		name     string
		query    string
		wantBody string
	}{
		{name: "invalid order", query: "?order=random", wantBody: "sort order is invalid\n"},
		{name: "invalid sort", query: "?sort=ownerID", wantBody: "project sort is invalid\n"},
		{name: "invalid status", query: "?status=waiting", wantBody: "project status is invalid\n"},
		{name: "negative page", query: "?page=-1", wantBody: "page must be a positive integer\n"},
		{name: "non-numeric page", query: "?page=one", wantBody: "page must be a positive integer\n"},
		{name: "page size too large", query: "?pageSize=101", wantBody: "page size must be between 1 and 100\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/projects"+test.query, nil)
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionToken})

			recorder := httptest.NewRecorder()
			app.routes().ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Errorf("status code = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
			if got := recorder.Body.String(); got != test.wantBody {
				t.Errorf("response body = %q, want %q", got, test.wantBody)
			}
		})
	}
}

func TestUpdateProjectEndpointUpdatesOnlyProvidedFields(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)
	user, sessionToken := createAuthenticatedTestUser(t, app, ctx)

	targetDate := time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)

	project, err := app.projects.Create(
		ctx,
		user.ID,
		"Original project name",
		"Original project description",
		&targetDate,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("/projects/%d", project.ID),
		strings.NewReader(`{"name":"Updated project name"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: sessionToken,
	})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	var body struct {
		ID          int64                `json:"id"`
		Name        string               `json:"name"`
		Description string               `json:"description"`
		Status      models.ProjectStatus `json:"status"`
		TargetDate  *string              `json:"targetDate"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.ID != project.ID {
		t.Errorf("ID = %d, want %d", body.ID, project.ID)
	}

	if body.Name != "Updated project name" {
		t.Errorf("Name = %q, want %q", body.Name, "Updated project name")
	}

	if body.Description != project.Description {
		t.Errorf("Description = %q, want %q", body.Description, project.Description)
	}

	if body.Status != project.Status {
		t.Errorf("Status = %q, want %q", body.Status, project.Status)
	}

	if body.TargetDate == nil {
		t.Fatal("TargetDate = nil, want the original target date")
	}

	if got := *body.TargetDate; got != targetDate.Format(time.DateOnly) {
		t.Errorf("TargetDate = %q, want %q", got, targetDate.Format(time.DateOnly))
	}

	stored, err := app.projects.GetByIDForUser(ctx, project.ID, user.ID)
	if err != nil {
		t.Fatalf("get updated project: %v", err)
	}

	if stored.Name != "Updated project name" {
		t.Errorf("stored Name = %q, want %q", stored.Name, "Updated project name")
	}
}

func TestUpdateProjectEndpointClearsTargetDate(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)
	user, sessionToken := createAuthenticatedTestUser(t, app, ctx)

	targetDate := time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)

	project, err := app.projects.Create(
		ctx,
		user.ID,
		"Project with a target date",
		"A project whose date will be cleared",
		&targetDate,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("/projects/%d", project.ID),
		strings.NewReader(`{"targetDate":null}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: sessionToken,
	})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	var body struct {
		TargetDate *string `json:"targetDate"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.TargetDate != nil {
		t.Errorf("response TargetDate = %q, want nil", *body.TargetDate)
	}

	stored, err := app.projects.GetByIDForUser(ctx, project.ID, user.ID)
	if err != nil {
		t.Fatalf("get updated project: %v", err)
	}

	if stored.TargetDate != nil {
		t.Errorf(
			"stored TargetDate = %s, want nil",
			stored.TargetDate.Format(time.DateOnly),
		)
	}
}

func TestUpdateProjectEndpointHonorsMemberRoles(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)

	owner, _ := createAuthenticatedTestUser(t, app, ctx)
	editor, editorToken := createProjectMemberTestUser(t, app, ctx, "editor@example.com", "Editor")
	viewer, viewerToken := createProjectMemberTestUser(t, app, ctx, "viewer@example.com", "Viewer")

	project, err := app.projects.Create(
		ctx, owner.ID, "Shared project", "A project with collaborators", nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Add editor.
	if _, err := app.projectMembers.Create(
		ctx, project.ID, editor.ID, models.ProjectMemberRoleEditor,
	); err != nil {
		t.Fatalf("add editor: %v", err)
	}

	// Add viewer.
	if _, err := app.projectMembers.Create(
		ctx, project.ID, viewer.ID, models.ProjectMemberRoleViewer,
	); err != nil {
		t.Fatalf("add viewer: %v", err)
	}

	// Test 1: Editor should be able to update the project.
	editorRequest := httptest.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("/projects/%d", project.ID),
		strings.NewReader(`{"status":"completed"}`),
	)
	editorRequest.Header.Set("Content-Type", "application/json")
	editorRequest.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: editorToken,
	})

	editorRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(editorRecorder, editorRequest)

	if editorRecorder.Code != http.StatusOK {
		t.Fatalf("editor status code = %d, want %d", editorRecorder.Code, http.StatusOK)
	}

	var editorBody struct {
		Status models.ProjectStatus `json:"status"`
	}

	if err := json.NewDecoder(editorRecorder.Body).Decode(&editorBody); err != nil {
		t.Fatalf("decode editor response: %v", err)
	}

	if editorBody.Status != models.ProjectStatusCompleted {
		t.Errorf(
			"editor updated Status = %q, want %q",
			editorBody.Status,
			models.ProjectStatusCompleted,
		)
	}

	// Test 2: Viewer should not be able to update the project.
	viewerRequest := httptest.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("/projects/%d", project.ID),
		strings.NewReader(`{"name":"Viewer must not update"}`),
	)
	viewerRequest.Header.Set("Content-Type", "application/json")
	viewerRequest.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: viewerToken,
	})

	viewerRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(viewerRecorder, viewerRequest)

	if viewerRecorder.Code != http.StatusNotFound {
		t.Errorf(
			"viewer status code = %d, want %d",
			viewerRecorder.Code,
			http.StatusNotFound,
		)
	}

	if got := viewerRecorder.Body.String(); got != "project not found\n" {
		t.Errorf("viewer response body = %q, want not-found error", got)
	}
}

func TestUpdateProjectEndpointRejectsInvalidRequests(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)
	user, sessionToken := createAuthenticatedTestUser(t, app, ctx)

	project, err := app.projects.Create(
		ctx, user.ID, "Original project", "Original description", nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	tests := []struct {
		name     string
		body     string
		wantBody string
	}{
		{
			name:     "empty update",
			body:     `{}`,
			wantBody: "atleast one project field must be provided\n",
		},
		{
			name:     "blank name",
			body:     `{"name":"   "}`,
			wantBody: "project name is required\n",
		},
		{
			name:     "null name",
			body:     `{"name":null}`,
			wantBody: "project name cannot be null\n",
		},
		{
			name:     "invalid status",
			body:     `{"status":"paused"}`,
			wantBody: "project status is invalid\n",
		},
		{
			name:     "invalid target date",
			body:     `{"targetDate":"31-12-2026"}`,
			wantBody: "target date must use YYYY-MM-DD\n",
		},
		{
			name:     "unknown field",
			body:     `{"ownerId":99}`,
			wantBody: "request body must be valid JSON\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPatch,
				fmt.Sprintf("/projects/%d", project.ID),
				strings.NewReader(test.body),
			)

			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(&http.Cookie{
				Name:  sessionCookieName,
				Value: sessionToken,
			})

			recorder := httptest.NewRecorder()
			app.routes().ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Errorf(
					"status code = %d, want %d",
					recorder.Code, http.StatusBadRequest,
				)
			}

			if got := recorder.Body.String(); got != test.wantBody {
				t.Errorf("response body = %q, want %q", got, test.wantBody)
			}
		})
	}

	// Invalid requests must not partially change stored data.
	stored, err := app.projects.GetByIDForUser(ctx, project.ID, user.ID)
	if err != nil {
		t.Fatalf("get project after invalid updates: %v", err)
	}

	if stored.Name != "Original project" {
		t.Errorf("stored Name = %q, want original value", stored.Name)
	}
}

func TestDeleteProjectEndpoint(t *testing.T) {
    db := testutil.OpenTestDB(t)
    testutil.ResetDatabase(t, db)
    t.Cleanup(func() { testutil.ResetDatabase(t, db) })

    ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
    defer cancel()

    app := newTestApplication(db)
    owner, sessionToken := createAuthenticatedTestUser(t, app, ctx)

    project, err := app.projects.Create(
        ctx, owner.ID, "Project to delete", "This project should disappear", nil,
    )
    if err != nil {
        t.Fatalf("create project: %v", err)
    }

    request := httptest.NewRequest(
        http.MethodDelete, fmt.Sprintf("/projects/%d", project.ID), nil,
    )
    request.AddCookie(&http.Cookie{
        Name:  sessionCookieName,
        Value: sessionToken,
    })

    recorder := httptest.NewRecorder()
    app.routes().ServeHTTP(recorder, request)

    // Successful DELETE requests return 204 with no response body.
    if recorder.Code != http.StatusNoContent {
        t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusNoContent)
    }

    if recorder.Body.Len() != 0 {
        t.Errorf("response body = %q, want empty body", recorder.Body.String())
    }

    // Confirm the database record was deleted.
    _, err = app.projects.GetByIDForUser(ctx, project.ID, owner.ID)
    if !errors.Is(err, models.ErrNoRecord) {
        t.Errorf("get deleted project error = %v, want models.ErrNoRecord", err)
    }
}

func TestDeleteProjectEndpointRejectsProjectMembers(t *testing.T) {
    db := testutil.OpenTestDB(t)
    testutil.ResetDatabase(t, db)
    t.Cleanup(func() { testutil.ResetDatabase(t, db) })

    ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
    defer cancel()

    app := newTestApplication(db)
    owner, _ := createAuthenticatedTestUser(t, app, ctx)

    project, err := app.projects.Create(
        ctx, owner.ID, "Owner-only project", "Members must not delete it", nil,
    )
    if err != nil {
        t.Fatalf("create project: %v", err)
    }

    tests := []struct {
        name  string
        email string
        role  models.ProjectMemberRole
    }{
        {
            name:  "editor",
            email: "delete-editor@example.com",
            role:  models.ProjectMemberRoleEditor,
        },
        {
            name:  "viewer",
            email: "delete-viewer@example.com",
            role:  models.ProjectMemberRoleViewer,
        },
    }

    for _, test := range tests {
        t.Run(test.name, func(t *testing.T) {
            member, sessionToken := createProjectMemberTestUser(
                t, app, ctx, test.email, test.name,
            )

            _, err := app.projectMembers.Create(ctx, project.ID, member.ID, test.role)
            if err != nil {
                t.Fatalf("add %s: %v", test.name, err)
            }

            request := httptest.NewRequest(
                http.MethodDelete, fmt.Sprintf("/projects/%d", project.ID), nil,
            )
            request.AddCookie(&http.Cookie{
                Name:  sessionCookieName,
                Value: sessionToken,
            })

            recorder := httptest.NewRecorder()
            app.routes().ServeHTTP(recorder, request)

            if recorder.Code != http.StatusForbidden {
                t.Errorf(
                    "status code = %d, want %d",
                    recorder.Code, http.StatusForbidden,
                )
            }

            if got := recorder.Body.String(); got != "project owner permission required\n" {
                t.Errorf("response body = %q, want owner-permission error", got)
            }

            // Rejected deletes must leave the project intact.
            _, err = app.projects.GetByIDForUser(ctx, project.ID, owner.ID)
            if err != nil {
                t.Errorf("get project after rejected delete: %v", err)
            }
        })
    }
}