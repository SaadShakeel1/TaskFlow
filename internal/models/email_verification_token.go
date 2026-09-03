package models

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

const emailVerificationTokenBytes = 32

type EmailVerificationTokenModel struct {
	DB *sql.DB
}

func (m *EmailVerificationTokenModel) Upsert(ctx context.Context, token EmailVerificationToken) error {
	const query = `
		INSERT INTO email_verification_tokens (
			token_hash, user_id, expires_at
		)
		VALUES ($1,$2,$3)
		ON CONFLICT (user_id) DO UPDATE 
		SET
			token_hash = EXCLUDED.token_hash,
			expires_at = EXCLUDED.expires_at,
			created_at = NOW()
	`

	_, err := m.DB.ExecContext(ctx, query, token.Hash, token.UserID, token.ExpiresAt)

	if err != nil {
		return fmt.Errorf("upsert email verification token: %w", err)
	}
	return nil
}

type EmailVerificationToken struct {
	Plaintext string
	Hash      []byte
	UserID    int64
	ExpiresAt time.Time
}

func NewEmailVerificationToken(userID int64, ttl time.Duration) (EmailVerificationToken, error) {
	if userID < 1 {
		return EmailVerificationToken{}, errors.New("user ID must be positive")
	}

	if ttl <= 0 {
		return EmailVerificationToken{}, errors.New("token TTL must be positive")
	}

	randomBytes := make([]byte, emailVerificationTokenBytes)

	if _, err := rand.Read(randomBytes); err != nil {
		return EmailVerificationToken{}, err
	}

	plainText := base64.RawURLEncoding.EncodeToString(randomBytes)

	hash := sha256.Sum256([]byte(plainText))

	return EmailVerificationToken{
		Plaintext: plainText,
		Hash:      hash[:],
		UserID:    userID,
		ExpiresAt: time.Now().Add(ttl).UTC(),
	}, nil
}
