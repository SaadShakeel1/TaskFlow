package testutil

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/SaadShakeel1/TaskFlow/internal/database"
)

const (
	testDatabaseName         = "taskflow_test"
	testDatabaseLockID int64 = 748392615
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

	lockConnection, err := db.Conn(context.Background())
	if err != nil {
		db.Close()
		t.Fatalf("open test database lock connection: %v", err)
	}

	if _, err := lockConnection.ExecContext(
		context.Background(),
		"SELECT pg_advisory_lock($1)",
		testDatabaseLockID,
	); err != nil {
		lockConnection.Close()
		db.Close()
		t.Fatalf("lock test database: %v", err)
	}

	t.Cleanup(func() {
		if _, err := lockConnection.ExecContext(
			context.Background(),
			"SELECT pg_advisory_unlock($1)",
			testDatabaseLockID,
		); err != nil {
			t.Errorf("unlock test database: %v", err)
		}

		if err := lockConnection.Close(); err != nil {
			t.Errorf("close test database lock connection: %v", err)
		}

		if err := db.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})

	return db
}

func ResetUsers(t testing.TB, db *sql.DB) {
	t.Helper()

	if _, err := db.Exec("TRUNCATE TABLE email_verification_tokens, users RESTART IDENTITY"); err != nil {
		t.Fatalf("reset users table: %v", err)
	}
}
