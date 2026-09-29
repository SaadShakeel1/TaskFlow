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

func TestProjectMemberEndpoints(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)
	owner, ownerToken := createAuthenticatedTestUser(t, app, ctx)
	editor, editorToken := createProjectMemberTestUser(t, app, ctx, "editor@example.com", "Editor")

	project, err := app.projects.Create(ctx, owner.ID, "Shared project", "A project with a collaborator", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	createRequest := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/projects/%d/members", project.ID),
		strings.NewReader(`{"email":"editor@example.com","role":"editor"}`),
	)
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: ownerToken})

	createRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(createRecorder, createRequest)

	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status code = %d, want %d", createRecorder.Code, http.StatusCreated)
	}

	var created projectMemberResponse
	if err := json.NewDecoder(createRecorder.Body).Decode(&created); err != nil {
		t.Fatalf("decode created member: %v", err)
	}
	if created.UserID != editor.ID {
		t.Errorf("created UserID = %d, want %d", created.UserID, editor.ID)
	}
	if created.Role != models.ProjectMemberRoleEditor {
		t.Errorf("created Role = %q, want %q", created.Role, models.ProjectMemberRoleEditor)
	}

	getProjectRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/projects/%d", project.ID), nil)
	getProjectRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: editorToken})
	getProjectRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(getProjectRecorder, getProjectRequest)
	if getProjectRecorder.Code != http.StatusOK {
		t.Errorf("editor get project status code = %d, want %d", getProjectRecorder.Code, http.StatusOK)
	}

	listRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/projects/%d/members", project.ID), nil)
	listRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: editorToken})
	listRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(listRecorder, listRequest)

	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status code = %d, want %d", listRecorder.Code, http.StatusOK)
	}

	var listed projectMemberListResponse
	if err := json.NewDecoder(listRecorder.Body).Decode(&listed); err != nil {
		t.Fatalf("decode member list: %v", err)
	}
	if len(listed.Members) != 2 {
		t.Fatalf("member count = %d, want 2", len(listed.Members))
	}
	if listed.Members[0].UserID != owner.ID || listed.Members[0].Role != models.ProjectMemberRoleOwner {
		t.Errorf("first member = %#v, want project owner", listed.Members[0])
	}

	updateRequest := httptest.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("/projects/%d/members/%d", project.ID, editor.ID),
		strings.NewReader(`{"role":"viewer"}`),
	)
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: ownerToken})
	updateRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(updateRecorder, updateRequest)

	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update status code = %d, want %d", updateRecorder.Code, http.StatusOK)
	}

	var updated projectMemberResponse
	if err := json.NewDecoder(updateRecorder.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated member: %v", err)
	}
	if updated.Role != models.ProjectMemberRoleViewer {
		t.Errorf("updated Role = %q, want %q", updated.Role, models.ProjectMemberRoleViewer)
	}

	forbiddenRequest := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/projects/%d/members", project.ID),
		strings.NewReader(`{"email":"another@example.com","role":"viewer"}`),
	)
	forbiddenRequest.Header.Set("Content-Type", "application/json")
	forbiddenRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: editorToken})
	forbiddenRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(forbiddenRecorder, forbiddenRequest)
	if forbiddenRecorder.Code != http.StatusForbidden {
		t.Errorf("member create status code = %d, want %d", forbiddenRecorder.Code, http.StatusForbidden)
	}

	deleteRequest := httptest.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("/projects/%d/members/%d", project.ID, editor.ID),
		nil,
	)
	deleteRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: ownerToken})
	deleteRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("delete status code = %d, want %d", deleteRecorder.Code, http.StatusNoContent)
	}

	getRemovedMemberRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/projects/%d", project.ID), nil)
	getRemovedMemberRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: editorToken})
	getRemovedMemberRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(getRemovedMemberRecorder, getRemovedMemberRequest)
	if getRemovedMemberRecorder.Code != http.StatusNotFound {
		t.Errorf("removed member get status code = %d, want %d", getRemovedMemberRecorder.Code, http.StatusNotFound)
	}
}

func TestCreateProjectMemberEndpointRejectsInvalidInput(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)
	owner, ownerToken := createAuthenticatedTestUser(t, app, ctx)
	project, err := app.projects.Create(ctx, owner.ID, "Shared project", "A project with members", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "missing email",
			body:       `{"email":"   ","role":"editor"}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "member email is required\n",
		},
		{
			name:       "invalid role",
			body:       `{"email":"person@example.com","role":"owner"}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "member role must be editor or viewer\n",
		},
		{
			name:       "owner as member",
			body:       `{"email":"saad@example.com","role":"editor"}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "project owner cannot be added as a member\n",
		},
		{
			name:       "unknown user",
			body:       `{"email":"unknown@example.com","role":"viewer"}`,
			wantStatus: http.StatusNotFound,
			wantBody:   "user not found\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPost,
				fmt.Sprintf("/projects/%d/members", project.ID),
				strings.NewReader(test.body),
			)
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: ownerToken})

			recorder := httptest.NewRecorder()
			app.routes().ServeHTTP(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Errorf("status code = %d, want %d", recorder.Code, test.wantStatus)
			}
			if got := recorder.Body.String(); got != test.wantBody {
				t.Errorf("response body = %q, want %q", got, test.wantBody)
			}
		})
	}
}

func TestCreateProjectMemberEndpointRejectsDuplicateMember(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)
	owner, ownerToken := createAuthenticatedTestUser(t, app, ctx)
	editor, _ := createProjectMemberTestUser(t, app, ctx, "editor@example.com", "Editor")
	project, err := app.projects.Create(ctx, owner.ID, "Shared project", "A project with members", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if _, err := app.projectMembers.Create(ctx, project.ID, editor.ID, models.ProjectMemberRoleEditor); err != nil {
		t.Fatalf("add initial member: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/projects/%d/members", project.ID),
		strings.NewReader(`{"email":"editor@example.com","role":"editor"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: ownerToken})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Errorf("status code = %d, want %d", recorder.Code, http.StatusConflict)
	}
	if got := recorder.Body.String(); got != "user is already a project member\n" {
		t.Errorf("response body = %q, want duplicate-member error", got)
	}
}

func createProjectMemberTestUser(t *testing.T, app *application, ctx context.Context, email, name string) (models.User, string) {
	t.Helper()

	user, err := app.users.Create(ctx, email, name, "correct-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	session, err := models.NewSession(user.ID, app.config.sessionTTL)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := app.sessions.Create(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	return user, session.Plaintext
}
