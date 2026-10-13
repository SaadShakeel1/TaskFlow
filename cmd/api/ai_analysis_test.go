package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SaadShakeel1/TaskFlow/internal/testutil"
)

func TestAIAnalysisRunEndpoints(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)
	owner, token := createAuthenticatedTestUser(t, app, ctx)
	project, err := app.projects.Create(ctx, owner.ID, "AI API project", "Project for AI endpoint tests", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	createRequest := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/projects/%d/analysis-runs", project.ID), nil)
	createRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	createRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(createRecorder, createRequest)

	if createRecorder.Code != http.StatusAccepted {
		t.Fatalf("create status = %d, want %d: %s", createRecorder.Code, http.StatusAccepted, createRecorder.Body.String())
	}

	var created aiAnalysisRunResponse
	if err := json.NewDecoder(createRecorder.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	if created.Status != "queued" || created.ProjectID != project.ID {
		t.Errorf("created response = %#v", created)
	}

	listRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/projects/%d/analysis-runs", project.ID), nil)
	listRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	listRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(listRecorder, listRequest)

	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRecorder.Code, http.StatusOK)
	}

	var list aiAnalysisRunListResponse
	if err := json.NewDecoder(listRecorder.Body).Decode(&list); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if list.Pagination.TotalItems != 1 || len(list.Runs) != 1 || list.Runs[0].ID != created.ID {
		t.Errorf("list response = %#v", list)
	}

	getRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/projects/%d/analysis-runs/%d", project.ID, created.ID), nil)
	getRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	getRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(getRecorder, getRequest)

	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", getRecorder.Code, http.StatusOK)
	}
}
