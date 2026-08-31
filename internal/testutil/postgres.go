package testutil

import (
	"database/sql"
	"os"
	"testing"

	"github.com/SaadShakeel1/TaskFlow/internal/database"
)

func OpenTestDB(t testing.TB) *sql.DB {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	db, err := database.Open(databaseURL)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func ResetUsers(t testing.TB, db *sql.DB) {
	t.Helper()

	if _, err := db.Exec("TRUNCATE TABLE users RESTART IDENTITY"); err != nil {
		t.Fatalf("reset users table: %v", err)
	}
}
