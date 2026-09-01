package testutil

import (
	"database/sql"
	"os"
	"testing"

	"github.com/SaadShakeel1/TaskFlow/internal/database"
)

const testDatabaseName = "taskflow_test"

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

	var databaseName string
	err = db.QueryRow("SELECT current_database()").Scan(&databaseName)

	if err != nil {
		db.Close()
		t.Fatalf("get current database name: %v", err)
	}

	if databaseName != testDatabaseName {
		db.Close()
		t.Fatalf("refusing to run tests against database %q; expected %q",
			databaseName,
			testDatabaseName)
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
