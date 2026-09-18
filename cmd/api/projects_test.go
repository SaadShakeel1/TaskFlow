package main

import (
	"context"
	"encoding/json"
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
