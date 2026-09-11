package models_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
	"github.com/SaadShakeel1/TaskFlow/internal/testutil"
)

const sessionTTL = 7 * 24 * time.Hour

func TestNewSession(t *testing.T) {
	before := time.Now().UTC()

	session, err := models.NewSession(42, sessionTTL)

	if err != nil {
		t.Fatalf("creating session: %v", err)
	}

	after := time.Now().UTC()

	if session.UserID != 42 {
		t.Errorf("UserID = %d, want %d", session.UserID, 42)
	}

	if session.Plaintext == "" {
		t.Error("Plaintext is empty")
	}

	randomBytes, err := base64.RawURLEncoding.DecodeString(session.Plaintext)

	if err != nil {
		t.Fatalf("decode plaintext token: %v", err)
	}

	if len(randomBytes) != 32 {
		t.Errorf("random byte length = %d, want %d", len(randomBytes), 32)
	}

	expectedHash := sha256.Sum256([]byte(session.Plaintext))

	if !bytes.Equal(session.Hash, expectedHash[:]) {
		t.Error("Hash does not match SHA-256 of Plaintext")
	}

	if session.ExpiresAt.Before(before.Add(sessionTTL)) || session.ExpiresAt.After(after.Add(sessionTTL)) {
		t.Errorf(
			"ExpiresAt = %v, want between %v and %v",
			session.ExpiresAt,
			before.Add(sessionTTL),
			after.Add(sessionTTL),
		)
	}
}

func TestSessionModelCreate(t *testing.T) {
	db := testutil.OpenTestDB(t)

	testutil.ResetUsers(t, db)
	t.Cleanup(func() { testutil.ResetUsers(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}
	user, err := users.Create(
		ctx,
		"saad@example.com",
		"Saad",
		"correct-password",
	)

	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	session, err := models.NewSession(user.ID, sessionTTL)
	if err != nil {
		t.Fatalf("creating session: %v", err)
	}

	model := &models.SessionModel{DB: db}
	if err := model.Create(ctx, session); err != nil {
		t.Fatalf("create user: %v", err)
	}

	var gotHash []byte
	var gotUserID int64
	var gotExpiresAt time.Time

	err = db.QueryRowContext(
		ctx,
		`SELECT token_hash, user_id, expires_at
		FROM sessions
		WHERE token_hash = $1`,
		session.Hash,
	).Scan(&gotHash, &gotUserID, &gotExpiresAt)

	if err != nil {
		t.Fatalf("get stored session: %v", err)
	}

	if !bytes.Equal(gotHash, session.Hash) {
		t.Error("stored hash does not match session hash")
	}

	if gotUserID != user.ID {
		t.Errorf("stored user ID = %d, want %d", gotUserID, user.ID)
	}

	if gotExpiresAt.Before(session.ExpiresAt.Add(-time.Second)) ||
		gotExpiresAt.After(session.ExpiresAt.Add(time.Second)) {
		t.Errorf("stored expiry = %v, want approximately %v",
			gotExpiresAt, session.ExpiresAt)
	}
}

func TestSessionModelGetUserByToken(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetUsers(t, db)

	t.Cleanup(func() {
		testutil.ResetUsers(t, db)
	})

	ctx, cancel := context.WithTimeout(
		context.Background(),
		databaseTestTimeout,
	)
	defer cancel()

	userModel := &models.UserModel{DB: db}
	createdUser, err := userModel.Create(
		ctx,
		"saad@example.com",
		"Saad",
		"correct-password",
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	session, err := models.NewSession(createdUser.ID, sessionTTL)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	sessionModel := &models.SessionModel{DB: db}
	if err := sessionModel.Create(ctx, session); err != nil {
		t.Fatalf("store session: %v", err)
	}

	user, err := sessionModel.GetUserByToken(ctx, session.Plaintext)
	if err != nil {
		t.Fatalf("get user by session token: %v", err)
	}

	if user.ID != createdUser.ID {
		t.Errorf("user ID = %d, want %d", user.ID, createdUser.ID)
	}

	if user.Email != createdUser.Email {
		t.Errorf("user email = %q, want %q", user.Email, createdUser.Email)
	}

	if user.Name != createdUser.Name {
		t.Errorf("user name = %q, want %q", user.Name, createdUser.Name)
	}
}

func TestSessionModelGetUserByTokenRejectsExpiredSession(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetUsers(t, db)

	t.Cleanup(func() {
		testutil.ResetUsers(t, db)
	})

	ctx, cancel := context.WithTimeout(
		context.Background(),
		databaseTestTimeout,
	)
	defer cancel()

	users := &models.UserModel{DB: db}
	user, err := users.Create(
		ctx,
		"saad@example.com",
		"Saad",
		"correct-password",
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	session, err := models.NewSession(user.ID, sessionTTL)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	sessions := &models.SessionModel{DB: db}
	if err := sessions.Create(ctx, session); err != nil {
		t.Fatalf("store session: %v", err)
	}

	_, err = db.ExecContext(
		ctx,
		`UPDATE sessions
			SET created_at = NOW() - INTERVAL '2 hours',
				expires_at = NOW() - INTERVAL '1 hour'
			WHERE token_hash = $1`,
		session.Hash,
	)
	if err != nil {
		t.Fatalf("expire session: %v", err)
	}

	_, err = sessions.GetUserByToken(ctx, session.Plaintext)
	if !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("error = %v, want models.ErrNoRecord", err)
	}
}

func TestSessionModelDeleteByToken(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetUsers(t, db)

	t.Cleanup(func() {
		testutil.ResetUsers(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	userModel := &models.UserModel{DB: db}
	createdUser, err := userModel.Create(ctx, "saad@example.com", "Saad", "correct-password")

	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	sessionModel := &models.SessionModel{DB: db}
	createdSession, err := models.NewSession(createdUser.ID, sessionTTL)

	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := sessionModel.Create(ctx, createdSession); err != nil {
		t.Fatalf("store session: %v", err)
	}

	if err := sessionModel.DeleteByToken(ctx, createdSession.Plaintext); err != nil {
		t.Fatalf("delete session: %v", err)
	}

	_, err = sessionModel.GetUserByToken(ctx, createdSession.Plaintext)

	if !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("error = %v, want models.ErrNoRecord", err)
	}

}
