package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/pgconn"
)

type user struct {
	ID        int64
	Email     string
	Name      string
	CreatedAt time.Time
}

type userResponse struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type createUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func createUser(ctx context.Context, db *sql.DB, email, name string) (user, error) {
	const query = `
		INSERT INTO users (email,name)
		VALUES ($1,$2)
		RETURNING id,email,name,created_at
	`

	var createdUser user

	err := db.QueryRowContext(ctx, query, email, name).Scan(
		&createdUser.ID,
		&createdUser.Email,
		&createdUser.Name,
		&createdUser.CreatedAt,
	)

	if err != nil {
		return user{}, fmt.Errorf("create user: %w", err)
	}

	return createdUser, nil
}

func createUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request createUserRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "request body must be valid JSON", http.StatusBadRequest)
			return
		}

		email := strings.TrimSpace(request.Email)
		name := strings.TrimSpace(request.Name)

		if email == "" {
			http.Error(w, "email is required", http.StatusBadRequest)
			return
		}

		if name == "" {
			http.Error(w, "name` is required", http.StatusBadRequest)
			return
		}

		createdUser, err := createUser(r.Context(), db, email, name)

		if err != nil {
			if isUniqueViolation(err) {
				http.Error(w, "email is already registered", http.StatusConflict)
				return
			}

			log.Printf("create user: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		response := userResponse{
			ID:        createdUser.ID,
			Email:     createdUser.Email,
			Name:      createdUser.Name,
			CreatedAt: createdUser.CreatedAt,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("encode create user response: %v", err)
		}

	}

}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError

	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}

func getUserByID(ctx context.Context, db *sql.DB, id int64) (user, error) {
	const query = `
		SELECT id,email,name,created_at
		FROM users
		where id = $1
	`

	var foundUser user

	err := db.QueryRowContext(ctx, query, id).Scan(
		&foundUser.ID,
		&foundUser.Email,
		&foundUser.Name,
		&foundUser.CreatedAt,
	)

	if err != nil {
		return user{}, fmt.Errorf("get user by ID: %w", err)
	}

	return foundUser, nil
}

func getUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idText := r.PathValue("id")

		id, err := strconv.ParseInt(idText, 10, 64)
		if err != nil || id < 1 {
			http.Error(w, "user id must be positive integer", http.StatusBadRequest)
		}

		foundUser, err := getUserByID(r.Context(), db, id)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "user not found", http.StatusNotFound)
				return
			}

			log.Printf("get user by ID: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		response := userResponse{
			ID:        foundUser.ID,
			Email:     foundUser.Email,
			Name:      foundUser.Name,
			CreatedAt: foundUser.CreatedAt,
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("encode get user response: %v", err)
		}
	}
}
