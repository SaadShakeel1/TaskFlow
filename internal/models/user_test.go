package models_test

import (
	"context"
	"testing"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
	"github.com/SaadShakeel1/TaskFlow/internal/testutil"
)

const databaseTestTimeout = 5 * time.Second

func TestUserModelCreate(t *testing.T) {
	db := testutil.OpenTestDB(t)

	testutil.ResetUsers(t, db)
	t.Cleanup(func() {
		testutil.ResetUsers(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	model := &models.UserModel{DB: db}
	got, err := model.Create(ctx, "saad@example.com", "Saad")
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
}
