package models_test

import (
	"context"
	"testing"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
	"github.com/SaadShakeel1/TaskFlow/internal/testutil"
)

func TestProjectModelCreate(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)

	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}
	user, err := users.Create(ctx, "saad@example.com", "Saad", "correct-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	projects := &models.ProjectModel{DB: db}
	got, err := projects.Create(
		ctx,
		user.ID,
		"Launch TaskFlow",
		"Plan the first public release",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if got.ID < 1 {
		t.Errorf("ID = %d, want a positive value", got.ID)
	}

	if got.UserID != user.ID {
		t.Errorf("UserID = %d, want %d", got.UserID, user.ID)
	}

	if got.Name != "Launch TaskFlow" {
		t.Errorf("Name = %q, want %q", got.Name, "Launch TaskFlow")
	}

	if got.Description != "Plan the first public release" {
		t.Errorf("Description = %q, want %q", got.Description, "Plan the first public release")
	}

	if got.Status != models.ProjectStatusActive {
		t.Errorf("Status = %q, want %q", got.Status, models.ProjectStatusActive)
	}

	if got.TargetDate != nil {
		t.Errorf("TargetDate = %v, want nil", got.TargetDate)
	}

	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero, want a PostgreSQL-generated timestamp")
	}

	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt is zero, want a PostgreSQL-generated timestamp")
	}
}

func TestProjectModelCreateWithTargetDate(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)

	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}
	user, err := users.Create(ctx, "saad@example.com", "Saad", "correct-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	targetDate := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)

	projects := &models.ProjectModel{DB: db}
	got, err := projects.Create(ctx, user.ID, "Launch TaskFlow", "Plan the first public release", &targetDate)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if got.TargetDate == nil {
		t.Fatal("TargetDate = nil, want the stored target date")
	}

	if got.TargetDate.Format(time.DateOnly) != targetDate.Format(time.DateOnly) {
		t.Errorf("TargetDate = %s, want %s", got.TargetDate.Format(time.DateOnly), targetDate.Format(time.DateOnly))
	}
}