package models_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
	"github.com/SaadShakeel1/TaskFlow/internal/testutil"
)

const databaseTestTimeout = 5 * time.Second

func TestUserModelCreate(t *testing.T) {
	db := testutil.OpenTestDB(t)

	testutil.ResetDatabase(t, db)
	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	model := &models.UserModel{DB: db}
	got, err := model.Create(ctx, "saad@example.com", "Saad", "correct-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if got.ID < 1 {
		t.Errorf("ID = %d, want a positive value", got.ID)
	}

	if got.Email != "saad@example.com" {
		t.Errorf("Email = %q, want %q", got.Email, "saad@example.com")
	}

	if got.Name != "Saad" {
		t.Errorf("Name = %q, want %q", got.Name, "Saad")
	}

	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero, want a PostgreSQL-generated timestamp")
	}

	matches, err := got.Password.Matches("correct-password")

	if err != nil {
		t.Fatalf("match created user's password: %v", err)
	}

	if !matches {
		t.Error("created user's password did not match")
	}

	if got.EmailVerifiedAt != nil {
		t.Errorf(
			"EmailVerifiedAt = %v, want nil for a new user",
			got.EmailVerifiedAt,
		)
	}
}

func TestUserGetByEmail(t *testing.T) {
	db := testutil.OpenTestDB(t)

	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)

	defer cancel()

	model := &models.UserModel{DB: db}

	created, err := model.Create(ctx, "saad@example.com", "Saad", "correct-password")

	if err != nil {
		t.Fatalf("creating user: %v", err)
	}

	got, err := model.GetByEmail(ctx, "saad@example.com")

	if err != nil {
		t.Fatalf("get user by email: %v", err)
	}

	if got.ID != created.ID {
		t.Errorf("ID = %d, want %d", got.ID, created.ID)
	}

	if got.Email != created.Email {
		t.Errorf("Email = %q, want %q", got.Email, created.Email)
	}

	if got.Name != created.Name {
		t.Errorf("Name = %q, want %q", got.Name, created.Name)
	}

	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}

	matches, err := got.Password.Matches("correct-password")

	if err != nil {
		t.Fatalf("match retrieved password: %v", err)
	}

	if !matches {
		t.Error("retrieved user's password did not match")
	}

	if got.EmailVerifiedAt != nil {
		t.Errorf("EmailVerifiedAt = %v, want nil", got.EmailVerifiedAt)
	}
}

func TestUserGetByEmailNotFound(t *testing.T) {
	db := testutil.OpenTestDB(t)

	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)

	defer cancel()

	model := &models.UserModel{DB: db}

	_, err := model.GetByEmail(ctx, "saad@example.com")

	if !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("error = %v, want models.ErrNoRecord", err)
	}

}
