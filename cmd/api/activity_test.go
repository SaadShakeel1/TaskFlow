package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
	"github.com/SaadShakeel1/TaskFlow/internal/testutil"
)

func TestListActivityEndpoint(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)
	owner, ownerToken := createAuthenticatedTestUser(t, app, ctx)

	project, err := app.projects.Create(
		ctx,
		owner.ID,
		"Activity API project",
		"Project for activity endpoint testing",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if _, err := app.activity.Create(ctx, models.ActivityCreateInput{
		ProjectID:  project.ID,
		UserID:     &owner.ID,
		Action:     "created",
		EntityType: "project",
	}); err != nil {
		t.Fatalf("create created activity: %v", err)
	}

	updatedActivity, err := app.activity.Create(ctx, models.ActivityCreateInput{
		ProjectID:  project.ID,
		UserID:     &owner.ID,
		Action:     "updated",
		EntityType: "project",
	})
	if err != nil {
		t.Fatalf("create updated activity: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf(
			"/projects/%d/activity?page=1&pageSize=1&action=updated",
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

	var response activityListResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Pagination.TotalItems != 1 {
		t.Errorf("total items = %d, want 1", response.Pagination.TotalItems)
	}

	if response.Pagination.TotalPages != 1 {
		t.Errorf("total pages = %d, want 1", response.Pagination.TotalPages)
	}

	if len(response.Activities) != 1 {
		t.Fatalf("activity count = %d, want 1", len(response.Activities))
	}

	if response.Activities[0].ID != updatedActivity.ID {
		t.Errorf(
			"activity ID = %d, want %d",
			response.Activities[0].ID,
			updatedActivity.ID,
		)
	}

	if response.Activities[0].Action != "updated" {
		t.Errorf(
			"activity action = %q, want %q",
			response.Activities[0].Action,
			"updated",
		)
	}
}

func TestListActivityEndpointRejectsInaccessibleProject(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)
	owner, _ := createAuthenticatedTestUser(t, app, ctx)
	_, outsiderToken := createProjectMemberTestUser(
		t,
		app,
		ctx,
		"activity-outsider@example.com",
		"Activity Outsider",
	)

	project, err := app.projects.Create(
		ctx,
		owner.ID,
		"Private activity project",
		"Private project",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/projects/%d/activity", project.ID),
		nil,
	)
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: outsiderToken,
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
