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

const sessionTokenBytes = 32

type Session struct {
	Plaintext string
	Hash      []byte
	UserID    int64
	ExpiresAt time.Time
}

type SessionModel struct {
	DB *sql.DB
}

func NewSession(userID int64, ttl time.Duration) (Session, error) {
	if userID < 1 {
		return Session{}, errors.New("user ID must be positive")
	}

	if ttl <= 0 {
		return Session{}, errors.New("session TTL must be positive")
	}

	randomBytes := make([]byte, sessionTokenBytes)

	if _, err := rand.Read(randomBytes); err != nil {
		return Session{}, err
	}

	plaintext := base64.RawURLEncoding.EncodeToString(randomBytes)
	hash := sha256.Sum256([]byte(plaintext))

	return Session{
		Plaintext: plaintext,
		Hash:      hash[:],
		UserID:    userID,
		ExpiresAt: time.Now().Add(ttl).UTC(),
	}, nil
}

func (m *SessionModel) Create(ctx context.Context, session Session) error {
	const query = `
		INSERT INTO sessions (token_hash, user_id, expires_at)
		VALUES ($1, $2, $3)
	`

	_, err := m.DB.ExecContext(
		ctx,
		query,
		session.Hash,
		session.UserID,
		session.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	return nil
}

func (m *SessionModel) GetUserByToken(ctx context.Context, plaintext string) (User, error) {
	hash := sha256.Sum256([]byte(plaintext))
	const query = `
		SELECT u.id, u.email, u.name, u.email_verified_at, u.created_at
		FROM sessions AS s
		INNER JOIN users AS u ON u.id = s.user_id
		WHERE s.token_hash = $1
			AND s.expires_at > NOW()
	`

	var user User

	err := m.DB.QueryRowContext(ctx, query, hash[:]).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.EmailVerifiedAt,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNoRecord
		}

		return User{}, fmt.Errorf("get user by session token: %w", err)
	}

	return user, nil
}
