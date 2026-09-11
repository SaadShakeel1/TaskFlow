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

const tokenTTL = 24 * time.Hour

func TestEmailVerificationModelUpsert(t *testing.T) {
	db := testutil.OpenTestDB(t)

	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	userModel := &models.UserModel{DB: db}
	createdUser, err := userModel.Create(ctx, "token@example.com", "Token User", "correct-password")

	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	tokenModel := models.EmailVerificationTokenModel{DB: db}
	firstToken, err := models.NewEmailVerificationToken(createdUser.ID, tokenTTL)

	if err != nil {
		t.Fatalf("create first token: %v", err)
	}

	if err := tokenModel.Upsert(ctx, firstToken); err != nil {
		t.Fatalf("upsert first token: %v", err)
	}

	const tokenQuery = `
		SELECT token_hash, user_id, expires_at
		FROM email_verification_tokens
		WHERE user_id = $1
	`

	var storedHash []byte
	var storedUserID int64
	var storedExpiresAt time.Time

	err = db.QueryRowContext(ctx, tokenQuery, createdUser.ID).Scan(
		&storedHash,
		&storedUserID,
		&storedExpiresAt,
	)

	if err != nil {
		t.Fatalf("query stored token: %v", err)
	}

	if !bytes.Equal(storedHash, firstToken.Hash) {
		t.Error("stored hash does not match first token hash")
	}

	if storedUserID != createdUser.ID {
		t.Errorf("stored user ID = %d, want %d", storedUserID, createdUser.ID)
	}

	if !storedExpiresAt.After(time.Now().UTC()) {
		t.Errorf("stored expiry = %v, want a future time", storedExpiresAt)
	}

	secondToken, err := models.NewEmailVerificationToken(createdUser.ID, tokenTTL)

	if err != nil {
		t.Fatalf("create second token: %v", err)
	}

	if err := tokenModel.Upsert(ctx, secondToken); err != nil {
		t.Fatalf("upsert second token: %v", err)
	}

	var tokenCount int

	err = db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM email_verification_tokens WHERE user_id = $1`,
		createdUser.ID,
	).Scan(&tokenCount)

	if err != nil {
		t.Fatalf("count stored tokens: %v", err)
	}

	if tokenCount != 1 {
		t.Errorf("stored token count = %d, want 1", tokenCount)
	}

	err = db.QueryRowContext(
		ctx,
		`SELECT token_hash FROM email_verification_tokens WHERE user_id = $1`,
		createdUser.ID,
	).Scan(&storedHash)

	if err != nil {
		t.Fatalf("query replacement token: %v", err)
	}

	if !bytes.Equal(storedHash, secondToken.Hash) {
		t.Error("stored hash does not match replacement token hash")
	}

}

func TestNewEmailVerificationToken(t *testing.T) {

	before := time.Now().UTC()

	token, err := models.NewEmailVerificationToken(42, tokenTTL)

	if err != nil {
		t.Fatalf("create email verification token: %v", err)
	}

	after := time.Now().UTC()

	if token.UserID != 42 {
		t.Errorf("UserID = %d, want %d", token.UserID, 42)
	}

	if token.Plaintext == "" {
		t.Error("Plaintext is empty")
	}

	randomBytes, err := base64.RawURLEncoding.DecodeString(token.Plaintext)

	if err != nil {
		t.Fatalf("decode plaintext token: %v", err)
	}

	if len(randomBytes) != 32 {
		t.Errorf("random byte length = %d, want %d", len(randomBytes), 32)
	}

	expectedHash := sha256.Sum256([]byte(token.Plaintext))

	if !bytes.Equal(token.Hash, expectedHash[:]) {
		t.Error("Hash does not match SHA-256 of Plaintext")
	}

	if token.ExpiresAt.Before(before.Add(tokenTTL)) || token.ExpiresAt.After(after.Add(tokenTTL)) {
		t.Errorf(
			"ExpiresAt = %v, want between %v and %v",
			token.ExpiresAt,
			before.Add(tokenTTL),
			after.Add(tokenTTL),
		)
	}
}

func TestNewEmailVerificationTokenGeneratesUniqueTokens(t *testing.T) {
	first, err := models.NewEmailVerificationToken(1, time.Hour)
	if err != nil {
		t.Fatalf("create first token: %v", err)
	}

	second, err := models.NewEmailVerificationToken(1, time.Hour)
	if err != nil {
		t.Fatalf("create second token: %v", err)
	}

	if first.Plaintext == second.Plaintext {
		t.Error("two generated tokens were identical")
	}
}

func TestNewEmailVerificationTokenRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		userID int64
		ttl    time.Duration
	}{
		{name: "zero user ID", userID: 0, ttl: time.Hour},
		{name: "negative user ID", userID: -1, ttl: time.Hour},
		{name: "zero TTL", userID: 1, ttl: 0},
		{name: "negative TTL", userID: 1, ttl: -time.Hour},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := models.NewEmailVerificationToken(test.userID, test.ttl)
			if err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

func TestEmailVerificationTokenModelVerify(t *testing.T) {
	db := testutil.OpenTestDB(t)

	testutil.ResetDatabase(t, db)
	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	userModel := &models.UserModel{DB: db}
	createdUser, err := userModel.Create(
		ctx,
		"email_verify@test.com",
		"Email Verify",
		"correct-password",
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	tokenModel := &models.EmailVerificationTokenModel{DB: db}
	token, err := models.NewEmailVerificationToken(createdUser.ID, tokenTTL)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	// Store the token before trying to verify it.
	if err := tokenModel.Upsert(ctx, token); err != nil {
		t.Fatalf("store token: %v", err)
	}

	if err := tokenModel.Verify(ctx, token.Plaintext); err != nil {
		t.Fatalf("verify token: %v", err)
	}

	// A pointer represents a nullable SQL timestamp.
	var verifiedAt *time.Time
	err = db.QueryRowContext(
		ctx,
		`SELECT email_verified_at FROM users WHERE id = $1`,
		createdUser.ID,
	).Scan(&verifiedAt)
	if err != nil {
		t.Fatalf("query verified user: %v", err)
	}

	if verifiedAt == nil {
		t.Error("email_verified_at is nil after verification")
	}

	var tokenCount int
	err = db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM email_verification_tokens WHERE user_id = $1`,
		createdUser.ID,
	).Scan(&tokenCount)
	if err != nil {
		t.Fatalf("count verification tokens: %v", err)
	}

	if tokenCount != 0 {
		t.Errorf("token count = %d, want 0", tokenCount)
	}

	err = tokenModel.Verify(ctx, token.Plaintext)
	if !errors.Is(err, models.ErrInvalidEmailVerificationToken) {
		t.Errorf(
			"second verification error = %v, want ErrInvalidEmailVerificationToken",
			err,
		)
	}
}
