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

func TestMutationEndpointsRecordActivity(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)
	owner, ownerToken := createAuthenticatedTestUser(t, app, ctx)

	projectRequest := authenticatedRequest(
		http.MethodPost,
		"/projects",
		`{"name":"Activity mutation project","description":"Mutation testing"}`,
		ownerToken,
	)
	projectRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(projectRecorder, projectRequest)

	if projectRecorder.Code != http.StatusCreated {
		t.Fatalf("create project status = %d, want %d", projectRecorder.Code, http.StatusCreated)
	}

	var projectResponseValue projectResponse
	if err := json.NewDecoder(projectRecorder.Body).Decode(&projectResponseValue); err != nil {
		t.Fatalf("decode project response: %v", err)
	}

	projectID := projectResponseValue.ID
	assertLatestActivity(t, app, ctx, projectID, owner.ID, "created", "project")

	updateProjectRequest := authenticatedRequest(
		http.MethodPatch,
		fmt.Sprintf("/projects/%d", projectID),
		`{"name":"Renamed activity project"}`,
		ownerToken,
	)
	updateProjectRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(updateProjectRecorder, updateProjectRequest)

	if updateProjectRecorder.Code != http.StatusOK {
		t.Fatalf("update project status = %d, want %d", updateProjectRecorder.Code, http.StatusOK)
	}

	assertLatestActivity(t, app, ctx, projectID, owner.ID, "updated", "project")

	createTaskRequest := authenticatedRequest(
		http.MethodPost,
		fmt.Sprintf("/projects/%d/tasks", projectID),
		`{"title":"Activity task"}`,
		ownerToken,
	)
	createTaskRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(createTaskRecorder, createTaskRequest)

	if createTaskRecorder.Code != http.StatusCreated {
		t.Fatalf("create task status = %d, want %d", createTaskRecorder.Code, http.StatusCreated)
	}

	var taskResponseValue taskResponse
	if err := json.NewDecoder(createTaskRecorder.Body).Decode(&taskResponseValue); err != nil {
		t.Fatalf("decode task response: %v", err)
	}

	taskID := taskResponseValue.ID
	assertLatestActivity(t, app, ctx, projectID, owner.ID, "created", "task")

	updateTaskRequest := authenticatedRequest(
		http.MethodPatch,
		fmt.Sprintf("/projects/%d/tasks/%d", projectID, taskID),
		`{"title":"Updated activity task","version":1}`,
		ownerToken,
	)
	updateTaskRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(updateTaskRecorder, updateTaskRequest)

	if updateTaskRecorder.Code != http.StatusOK {
		t.Fatalf("update task status = %d, want %d", updateTaskRecorder.Code, http.StatusOK)
	}

	var updatedTaskResponseValue taskResponse
	if err := json.NewDecoder(updateTaskRecorder.Body).Decode(&updatedTaskResponseValue); err != nil {
		t.Fatalf("decode updated task response: %v", err)
	}

	assertLatestActivity(t, app, ctx, projectID, owner.ID, "updated", "task")

	member, err := app.users.Create(
		ctx,
		"activity-member@example.com",
		"Activity Member",
		"correct-password",
	)
	if err != nil {
		t.Fatalf("create member user: %v", err)
	}

	createMemberRequest := authenticatedRequest(
		http.MethodPost,
		fmt.Sprintf("/projects/%d/members", projectID),
		`{"email":"activity-member@example.com","role":"viewer"}`,
		ownerToken,
	)
	createMemberRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(createMemberRecorder, createMemberRequest)

	if createMemberRecorder.Code != http.StatusCreated {
		t.Fatalf("create member status = %d, want %d", createMemberRecorder.Code, http.StatusCreated)
	}

	assertLatestActivity(t, app, ctx, projectID, owner.ID, "added", "member")

	updateMemberRequest := authenticatedRequest(
		http.MethodPatch,
		fmt.Sprintf("/projects/%d/members/%d", projectID, member.ID),
		`{"role":"editor"}`,
		ownerToken,
	)
	updateMemberRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(updateMemberRecorder, updateMemberRequest)

	if updateMemberRecorder.Code != http.StatusOK {
		t.Fatalf("update member status = %d, want %d", updateMemberRecorder.Code, http.StatusOK)
	}

	assertLatestActivity(t, app, ctx, projectID, owner.ID, "role_changed", "member")

	deleteMemberRequest := authenticatedRequest(
		http.MethodDelete,
		fmt.Sprintf("/projects/%d/members/%d", projectID, member.ID),
		"",
		ownerToken,
	)
	deleteMemberRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(deleteMemberRecorder, deleteMemberRequest)

	if deleteMemberRecorder.Code != http.StatusNoContent {
		t.Fatalf("delete member status = %d, want %d", deleteMemberRecorder.Code, http.StatusNoContent)
	}

	assertLatestActivity(t, app, ctx, projectID, owner.ID, "removed", "member")

	prerequisite, err := app.tasks.Create(
		ctx,
		projectID,
		owner.ID,
		models.TaskCreateInput{
			Title:    "Prerequisite task",
			Priority: models.TaskPriorityMedium,
		},
	)
	if err != nil {
		t.Fatalf("create prerequisite task: %v", err)
	}

	createDependencyRequest := authenticatedRequest(
		http.MethodPost,
		fmt.Sprintf("/projects/%d/tasks/%d/dependencies", projectID, taskID),
		fmt.Sprintf(`{"dependsOnTaskId":%d}`, prerequisite.ID),
		ownerToken,
	)
	createDependencyRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(createDependencyRecorder, createDependencyRequest)

	if createDependencyRecorder.Code != http.StatusCreated {
		t.Fatalf("create dependency status = %d, want %d", createDependencyRecorder.Code, http.StatusCreated)
	}

	assertLatestActivity(t, app, ctx, projectID, owner.ID, "created", "dependency")

	deleteDependencyRequest := authenticatedRequest(
		http.MethodDelete,
		fmt.Sprintf("/projects/%d/tasks/%d/dependencies/%d", projectID, taskID, prerequisite.ID),
		"",
		ownerToken,
	)
	deleteDependencyRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(deleteDependencyRecorder, deleteDependencyRequest)

	if deleteDependencyRecorder.Code != http.StatusNoContent {
		t.Fatalf("delete dependency status = %d, want %d", deleteDependencyRecorder.Code, http.StatusNoContent)
	}

	assertLatestActivity(t, app, ctx, projectID, owner.ID, "deleted", "dependency")

	deleteTaskRequest := authenticatedRequest(
		http.MethodDelete,
		fmt.Sprintf(
			"/projects/%d/tasks/%d?version=%d",
			projectID,
			taskID,
			updatedTaskResponseValue.Version,
		),
		"",
		ownerToken,
	)
	deleteTaskRecorder := httptest.NewRecorder()
	app.routes().ServeHTTP(deleteTaskRecorder, deleteTaskRequest)

	if deleteTaskRecorder.Code != http.StatusNoContent {
		t.Fatalf("delete task status = %d, want %d", deleteTaskRecorder.Code, http.StatusNoContent)
	}

	assertLatestActivity(t, app, ctx, projectID, owner.ID, "deleted", "task")
}

func authenticatedRequest(method, path, body, token string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: token,
	})

	return request
}

func assertLatestActivity(
	t *testing.T,
	app *application,
	ctx context.Context,
	projectID int64,
	userID int64,
	wantAction string,
	wantEntityType string,
) {
	t.Helper()

	list, err := app.activity.ListByProjectForUser(
		ctx,
		projectID,
		userID,
		models.ActivityListOptions{Page: 1, PageSize: 100},
	)
	if err != nil {
		t.Fatalf("list recorded activities: %v", err)
	}

	if len(list.Activities) == 0 {
		t.Fatalf("expected activity %q for %s, got no activities", wantAction, wantEntityType)
	}

	latest := list.Activities[0]
	if latest.Action != wantAction {
		t.Errorf("latest activity action = %q, want %q", latest.Action, wantAction)
	}

	if latest.EntityType != wantEntityType {
		t.Errorf("latest activity entity type = %q, want %q", latest.EntityType, wantEntityType)
	}
}
