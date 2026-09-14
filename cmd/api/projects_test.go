package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func createAuthenticatedTestUser(t *testing.T,app *application,ctx context.Context) (models.User, string) {
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