package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type User struct {
	ID              int64
	Email           string
	Name            string
	Password        Password
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
}

type UserModel struct {
	DB *sql.DB
}

func (m *UserModel) Create(ctx context.Context, email, name, plaintextPassword string) (User, error) {
	var user User

	if err := user.Password.Set(plaintextPassword); err != nil {
		return User{}, fmt.Errorf("set user password: %w", err)
	}

	const query = `
		INSERT INTO users (email, name,password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, email, name, email_verified_at, created_at
	`

	err := m.DB.QueryRowContext(ctx, query, email, name, user.Password.hash).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.EmailVerifiedAt,
		&user.CreatedAt,
	)

	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return User{}, ErrDuplicateEmail
		}

		return User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

func (m *UserModel) GetByID(ctx context.Context, id int64) (User, error) {
	const query = `
		SELECT id, email, name, created_at
		FROM users
		WHERE id = $1
	`

	var user User

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNoRecord
		}

		return User{}, fmt.Errorf("get user by ID: %w", err)
	}

	return user, nil
}
