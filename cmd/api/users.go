package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/SaadShakeel1/TaskFlow/internal/mailer"
	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

const (
	minimumPasswordCharacters = 15
	maximumPasswordBytes      = 72
	emailVerificationTokenTTL = 24 * time.Hour
)

type createUserRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type verifyEmailRequest struct {
	Token string `json:"token"`
}

type verifyEmailResponse struct {
	Message string `json:"message"`
}

type userResponse struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

func (app *application) createUserHandler(w http.ResponseWriter, r *http.Request) {
	var request createUserRequest

	if requestError := readJSON(w, r, &request); requestError != nil {
		app.clientError(w, requestError.status, requestError.message)
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

	token, err := models.NewEmailVerificationToken(user.ID, emailVerificationTokenTTL)
	if err != nil {
		app.serverError(w, err)
		return
	}

	if err := app.verificationTokens.Upsert(r.Context(), token); err != nil {
		app.serverError(w, err)
		return
	}

	verificationURL := fmt.Sprintf(
		"%s/verify-email?token=%s",
		strings.TrimRight(app.config.appURL, "/"),
		url.QueryEscape(token.Plaintext),
	)

	if err := app.mailer.Send(mailer.Email{
		To:      user.Email,
		Subject: "Verify your TaskFlow email address",
		Body: fmt.Sprintf(
			"Welcome to TaskFlow!\n\nVerify your email address by opening this link:\n\n%s\n\nThis link expires in 24 hours.",
			verificationURL,
		),
	}); err != nil {
		app.serverError(w, err)
		return
	}
	app.writeUserResponse(w, user, http.StatusCreated)
}

func (app *application) verifyEmailHandler(w http.ResponseWriter, r *http.Request) {
	var request verifyEmailRequest

	if requestError := readJSON(w, r, &request); requestError != nil {
		app.clientError(w, requestError.status, requestError.message)
		return
	}

	token := strings.TrimSpace(request.Token)

	if token == "" {
		app.clientError(w, http.StatusBadRequest, "verification token is required")
		return
	}

	if err := app.verificationTokens.Verify(r.Context(), token); err != nil {
		if errors.Is(err, models.ErrInvalidEmailVerificationToken) {
			app.clientError(w, http.StatusBadRequest, "verification token is invalid or expired")
			return
		}
		app.serverError(w, err)
		return
	}

	response := verifyEmailResponse{
		Message: "email verified",
	}

	if err := app.writeJSON(w, http.StatusOK, response); err != nil {
		app.errorLog.Printf("encode verification response: %v", err)
	}
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

func (app *application) currentUserHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(userContextKey).(models.User)

	if !ok {
		app.serverError(w, errors.New("authenticated user missing from context"))
		return
	}
	app.writeUserResponse(w, user, http.StatusOK)
}
