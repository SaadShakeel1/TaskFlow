package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/users"
	"github.com/jackc/pgx/v5/pgconn"
)

type createUserRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type userResponse struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s Server) createUserHandler(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	user, err := users.Create(r.Context(), s.db, email, name)
	if err != nil {
		if isUniqueViolation(err) {
			http.Error(w, "email is already registered", http.StatusConflict)
			return
		}

		log.Printf("create user: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeUserResponse(w, user, http.StatusCreated)
}

func (s Server) getUserHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.Error(w, "user id must be positive integer", http.StatusBadRequest)
		return
	}

	user, err := users.GetByID(r.Context(), s.db, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		log.Printf("get user by ID: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeUserResponse(w, user, http.StatusOK)
}

func writeUserResponse(w http.ResponseWriter, user users.User, status int) {
	response := userResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("encode user response: %v", err)
	}
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError

	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}
