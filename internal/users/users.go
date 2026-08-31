package users

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type User struct {
	ID        int64
	Email     string
	Name      string
	CreatedAt time.Time
}

func Create(ctx context.Context, db *sql.DB, email, name string) (User, error) {
	const query = `
		INSERT INTO users (email, name)
		VALUES ($1, $2)
		RETURNING id, email, name, created_at
	`

	var user User

	err := db.QueryRowContext(ctx, query, email, name).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.CreatedAt,
	)
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

func GetByID(ctx context.Context, db *sql.DB, id int64) (User, error) {
	const query = `
		SELECT id, email, name, created_at
		FROM users
		WHERE id = $1
	`

	var user User

	err := db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.CreatedAt,
	)
	if err != nil {
		return User{}, fmt.Errorf("get user by ID: %w", err)
	}

	return user, nil
}
