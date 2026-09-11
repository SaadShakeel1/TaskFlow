package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
	"github.com/SaadShakeel1/TaskFlow/internal/testutil"
)

func TestCreateSessionEndpoint(t *testing.T) {
	db := testutil.OpenTestDB(t)

	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)

	user, err := app.users.Create(ctx, "saad@example.com", "Saad", "correct-password")

	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	const query = `
		UPDATE users
		SET email_verified_at = NOW()
		WHERE id = $1
	`

	_, err = db.ExecContext(ctx, query, user.ID)

	if err != nil {
		t.Fatalf("mark user as verified: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(`{"email":"saad@example.com","password":"correct-password"}`))
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		t.Errorf("status code = %d, want %d", response.StatusCode, http.StatusNoContent)
	}

	cookies := response.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}

	cookie := cookies[0]

	if cookie.Name != "taskflow_session" {
		t.Errorf("cookie name = %q, want %q", cookie.Name, "taskflow_session")
	}

	if cookie.Value == "" {
		t.Error("session cookie value is empty")
	}

	if !cookie.HttpOnly {
		t.Error("HttpOnly = false, want true")
	}

	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf(
			"SameSite = %d, want %d",
			cookie.SameSite,
			http.SameSiteLaxMode,
		)
	}

	gotUser, err := app.sessions.GetUserByToken(ctx, cookie.Value)

	if err != nil {
		t.Fatalf("get user by session cookie: %v", err)
	}

	if gotUser.ID != user.ID {
		t.Errorf("user ID = %d, want %d", gotUser.ID, user.ID)
	}
}

func TestCreateSessionEndpointRejectsInvalidCredentials(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)

	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)

	user, err := app.users.Create(
		ctx,
		"saad@example.com",
		"Saad",
		"correct-password",
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	_, err = db.ExecContext(
		ctx,
		`UPDATE users SET email_verified_at = NOW() WHERE id = $1`,
		user.ID,
	)
	if err != nil {
		t.Fatalf("mark user as verified: %v", err)
	}

	tests := []struct {
		name     string
		email    string
		password string
	}{
		{
			name:     "unknown email",
			email:    "unknown@example.com",
			password: "correct-password",
		},
		{
			name:     "wrong password",
			email:    "saad@example.com",
			password: "wrong-password",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPost,
				"/sessions",
				strings.NewReader(
					`{"email":"`+test.email+`","password":"`+test.password+`"}`,
				),
			)
			request.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			app.routes().ServeHTTP(recorder, request)

			if recorder.Code != http.StatusUnauthorized {
				t.Errorf(
					"status code = %d, want %d",
					recorder.Code,
					http.StatusUnauthorized,
				)
			}

			if got := recorder.Body.String(); got != "invalid email or password\n" {
				t.Errorf(
					"response body = %q, want %q",
					got,
					"invalid email or password\n",
				)
			}

			if cookies := recorder.Result().Cookies(); len(cookies) != 0 {
				t.Errorf("cookie count = %d, want 0", len(cookies))
			}
		})
	}
}

func TestCreateSessionEndpointRejectsUnverifiedUser(t *testing.T) {
	db := testutil.OpenTestDB(t)

	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)

	_, err := app.users.Create(ctx, "saad@example.com", "Saad", "correct-password")

	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(`{"email":"saad@example.com","password":"correct-password"}`))
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", response.StatusCode, http.StatusForbidden)
	}

	cookies := response.Cookies()
	if len(cookies) != 0 {
		t.Fatalf("cookie count = %d, want 0", len(cookies))
	}

	if got := recorder.Body.String(); got != "email address must be verified\n" {
		t.Errorf(
			"response body = %q, want %q",
			got,
			"email address must be verified\n",
		)
	}
}

func TestCreateSessionEndpointRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantBody string
	}{
		{
			name:     "invalid JSON",
			body:     `{"email":`,
			wantBody: "request body must be valid JSON\n",
		},
		{
			name:     "missing email",
			body:     `{"password":"correct-password"}`,
			wantBody: "email is required\n",
		},
		{
			name:     "missing password",
			body:     `{"email":"saad@example.com"}`,
			wantBody: "password is required\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPost,
				"/sessions",
				strings.NewReader(test.body),
			)
			request.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			newTestApplication(nil).routes().ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Errorf(
					"status code = %d, want %d",
					recorder.Code,
					http.StatusBadRequest,
				)
			}

			if got := recorder.Body.String(); got != test.wantBody {
				t.Errorf(
					"response body = %q, want %q",
					got,
					test.wantBody,
				)
			}
		})
	}
}

func TestDeleteCurrentSessionEndpoint(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)

	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	app := newTestApplication(db)

	user, err := app.users.Create(ctx, "saad@example.com", "Saad", "correct-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	session, err := models.NewSession(user.ID, app.config.sessionTTL)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := app.sessions.Create(ctx, session); err != nil {
		t.Fatalf("store session: %v", err)
	}

	request := httptest.NewRequest(http.MethodDelete, "/sessions/current", nil)

	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: session.Plaintext,
	})

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		t.Errorf(
			"status code = %d, want %d",
			response.StatusCode,
			http.StatusNoContent,
		)
	}

	cookies := response.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}

	if cookies[0].Name != sessionCookieName {
		t.Errorf(
			"cookie name = %q, want %q",
			cookies[0].Name,
			sessionCookieName,
		)
	}

	if cookies[0].MaxAge != -1 {
		t.Errorf("cookie MaxAge = %d, want -1", cookies[0].MaxAge)
	}

	_, err = app.sessions.GetUserByToken(ctx, session.Plaintext)
	if !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("error = %v, want models.ErrNoRecord", err)
	}
}
