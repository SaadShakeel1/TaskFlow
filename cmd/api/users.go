package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

const (
	minimumPasswordCharacters = 15
	maximumPasswordBytes      = 72
)

type createUserRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type userResponse struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

func (app *application) createUserHandler(w http.ResponseWriter, r *http.Request) {
	var request createUserRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		app.clientError(w, http.StatusBadRequest, "request body must be valid JSON")
		return
	}

	email := strings.TrimSpace(request.Email)
	name := strings.TrimSpace(request.Name)
	password := request.Password

	if email == "" {
		app.clientError(w, http.StatusBadRequest, "email is required")
		return
	}

	if name == "" {
		app.clientError(w, http.StatusBadRequest, "name is required")
		return
	}

	if password == "" {
		app.clientError(w, http.StatusBadRequest, "password is required")
		return
	}

	if utf8.RuneCountInString(password) < minimumPasswordCharacters {
		app.clientError(w, http.StatusBadRequest, "password must contain at least 15 characters")
		return
	}

	if len(password) > maximumPasswordBytes {
		app.clientError(w, http.StatusBadRequest, "password must not exceed 72 bytes")
		return
	}

	user, err := app.users.Create(r.Context(), email, name, password)
	if err != nil {
		if errors.Is(err, models.ErrDuplicateEmail) {
			app.clientError(w, http.StatusConflict, "email is already registered")
			return
		}

		app.serverError(w, err)
		return
	}

	app.writeUserResponse(w, user, http.StatusCreated)
}

func (app *application) getUserHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		app.clientError(w, http.StatusBadRequest, "user id must be positive integer")
		return
	}

	user, err := app.users.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w, "user not found")
			return
		}

		app.serverError(w, err)
		return
	}

	app.writeUserResponse(w, user, http.StatusOK)
}

func (app *application) writeUserResponse(w http.ResponseWriter, user models.User, status int) {
	response := userResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt,
	}

	if err := app.writeJSON(w, status, response); err != nil {
		app.errorLog.Printf("encode user response: %v", err)
	}
}
