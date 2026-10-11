package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

const sessionCookieName = "taskflow_session"

type createSessionRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (app *application) createSessionHandler(w http.ResponseWriter, r *http.Request) {
	var request createSessionRequest

	if requestError := readJSON(w, r, &request); requestError != nil {
		app.clientError(w, requestError.status, requestError.message)
		return
	}

	email := strings.TrimSpace(request.Email)
	password := request.Password

	if email == "" {
		app.clientError(w, http.StatusBadRequest, "email is required")
		return
	}

	if password == "" {
		app.clientError(w, http.StatusBadRequest, "password is required")
		return
	}

	user, err := app.users.GetByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.clientError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		app.serverError(w, err)
		return
	}

	matches, err := user.Password.Matches(password)

	if err != nil {
		app.serverError(w, err)
		return
	}

	if !matches {
		app.clientError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if user.EmailVerifiedAt == nil {
		app.clientError(w, http.StatusForbidden, "email address must be verified")
		return
	}

	session, err := models.NewSession(user.ID, app.config.sessionTTL)

	if err != nil {
		app.serverError(w, err)
		return
	}

	if err := app.sessions.Create(r.Context(), session); err != nil {
		app.serverError(w, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.Plaintext,
		Path:     "/",
		Expires:  time.Now().Add(app.config.sessionTTL),
		MaxAge:   int(app.config.sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   app.config.sessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
	})

	w.WriteHeader(http.StatusNoContent)
}

func (app *application) deleteCurrentSessionHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)

	if err != nil {
		app.clientError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	if err := app.sessions.DeleteByToken(r.Context(), cookie.Value); err != nil {
		app.serverError(w, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   app.config.sessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}
