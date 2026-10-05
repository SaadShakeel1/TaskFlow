package models_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
	"github.com/SaadShakeel1/TaskFlow/internal/testutil"
)

func TestActivityModelCreate(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}
	projects := &models.ProjectModel{DB: db}
	activities := &models.ActivityModel{DB: db}

	user, err := users.Create(
		ctx,
		"activity-owner@example.com",
		"Activity Owner",
		"correct-password",
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	project, err := projects.Create(
		ctx,
		user.ID,
		"Activity project",
		"Project for activity testing",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	entityID := int64(42)
	metadata := json.RawMessage(`{
		"oldStatus": "todo",
		"newStatus": "done"
	}`)

	activity, err := activities.Create(ctx, models.ActivityCreateInput{
		ProjectID:  project.ID,
		UserID:     &user.ID,
		Action:     " status_changed ",
		EntityType: " task ",
		EntityID:   &entityID,
		Metadata:   metadata,
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	if activity.ID < 1 {
		t.Errorf("activity ID = %d, want a positive value", activity.ID)
	}

	if activity.ProjectID != project.ID {
		t.Errorf("project ID = %d, want %d", activity.ProjectID, project.ID)
	}

	if activity.UserID == nil || *activity.UserID != user.ID {
		t.Errorf("user ID = %v, want %d", activity.UserID, user.ID)
	}

	if activity.Action != "status_changed" {
		t.Errorf("action = %q, want %q", activity.Action, "status_changed")
	}

	if activity.EntityType != "task" {
		t.Errorf("entity type = %q, want %q", activity.EntityType, "task")
	}

	if activity.EntityID == nil || *activity.EntityID != entityID {
		t.Errorf("entity ID = %v, want %d", activity.EntityID, entityID)
	}

	var storedMetadata map[string]string
	if err := json.Unmarshal(activity.Metadata, &storedMetadata); err != nil {
		t.Fatalf("decode stored metadata: %v", err)
	}

	if storedMetadata["oldStatus"] != "todo" {
		t.Errorf(
			"old status = %q, want %q",
			storedMetadata["oldStatus"],
			"todo",
		)
	}

	if storedMetadata["newStatus"] != "done" {
		t.Errorf(
			"new status = %q, want %q",
			storedMetadata["newStatus"],
			"done",
		)
	}
}

func TestActivityModelCreateUsesDefaultMetadata(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}
	projects := &models.ProjectModel{DB: db}
	activities := &models.ActivityModel{DB: db}

	user, err := users.Create(
		ctx,
		"system-activity-owner@example.com",
		"System Activity Owner",
		"correct-password",
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	project, err := projects.Create(
		ctx,
		user.ID,
		"System activity project",
		"Project for default metadata testing",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	activity, err := activities.Create(ctx, models.ActivityCreateInput{
		ProjectID:  project.ID,
		Action:     "system_check",
		EntityType: "project",
	})
	if err != nil {
		t.Fatalf("create system activity: %v", err)
	}

	if activity.UserID != nil {
		t.Errorf("user ID = %v, want nil", activity.UserID)
	}

	if activity.EntityID != nil {
		t.Errorf("entity ID = %v, want nil", activity.EntityID)
	}

	var metadata map[string]any
	if err := json.Unmarshal(activity.Metadata, &metadata); err != nil {
		t.Fatalf("decode default metadata: %v", err)
	}

	if len(metadata) != 0 {
		t.Errorf("default metadata = %v, want an empty object", metadata)
	}
}

func TestActivityModelCreateRejectsInvalidInput(t *testing.T) {
	activities := &models.ActivityModel{}

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	_, err := activities.Create(ctx, models.ActivityCreateInput{
		ProjectID:  1,
		Action:     "created",
		EntityType: "task",
		Metadata:   json.RawMessage(`{"invalid":`),
	})
	if err == nil {
		t.Fatal("expected invalid metadata error, got nil")
	}
}