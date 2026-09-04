package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/mailer"
	"github.com/SaadShakeel1/TaskFlow/internal/models"
	"github.com/SaadShakeel1/TaskFlow/internal/testutil"
)

const databaseTestTimeout = 5 * time.Second

type recordingMailer struct {
	emails []mailer.Email
}

func (m *recordingMailer) Send(email mailer.Email) error {
	// Save outgoing emails so the test can inspect them.
	m.emails = append(m.emails, email)
	return nil
}

func TestCreateUserEndpointSendsVerificationEmail(t *testing.T) {
	db := testutil.OpenTestDB(t)

	testutil.ResetUsers(t, db)
	t.Cleanup(func() {
		testutil.ResetUsers(t, db)
	})

	emailSender := &recordingMailer{}

	app := newTestApplication(db)
	app.config.appURL = "http://taskflow.test"
	app.mailer = emailSender

	request := httptest.NewRequest(
		http.MethodPost,
		"/users",
		strings.NewReader(`{"email":"saad@example.com","name":"Saad","password":"correct-password"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	app.routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Errorf("status code = %d, want %d", recorder.Code, http.StatusCreated)
	}

	if len(emailSender.emails) != 1 {
		t.Fatalf("sent email count = %d, want 1", len(emailSender.emails))
	}

	email := emailSender.emails[0]

	if email.To != "saad@example.com" {
		t.Errorf("email recipient = %q, want %q", email.To, "saad@example.com")
	}

	if !strings.Contains(email.Body, "http://taskflow.test/verify-email?token=") {
		t.Errorf("email body = %q, want a verification link", email.Body)
	}
}

func TestCreateUserEndpoint(t *testing.T) {
	db := testutil.OpenTestDB(t)

	testutil.ResetUsers(t, db)
	t.Cleanup(func() {
		testutil.ResetUsers(t, db)
	})

	request := httptest.NewRequest(
		http.MethodPost,
		"/users",
		strings.NewReader(`{"email":"saad@example.com","name":"Saad","password":"correct-password"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	newTestApplication(db).routes().ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		t.Errorf("status code = %d, want %d", response.StatusCode, http.StatusCreated)
	}

	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	var body userResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode JSON response: %v", err)
	}

	if body.ID < 1 {
		t.Errorf("id = %d, want a positive value", body.ID)
	}

	if body.Email != "saad@example.com" {
		t.Errorf("email = %q, want %q", body.Email, "saad@example.com")
	}

	if body.Name != "Saad" {
		t.Errorf("name = %q, want %q", body.Name, "Saad")
	}
}

func TestCreateUserEndpointRejectsInvalidPasswords(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantBody string
	}{
		{
			name:     "missing password",
			password: "",
			wantBody: "password is required\n",
		},
		{
			name:     "password shorter than 15 characters",
			password: "too-short",
			wantBody: "password must contain at least 15 characters\n",
		},
		{
			name:     "password longer than bcrypt limit",
			password: strings.Repeat("a", maximumPasswordBytes+1),
			wantBody: "password must not exceed 72 bytes\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body, err := json.Marshal(createUserRequest{
				Email:    "saad@example.com",
				Name:     "Saad",
				Password: test.password,
			})
			if err != nil {
				t.Fatalf("marshal request body: %v", err)
			}

			request := httptest.NewRequest(
				http.MethodPost,
				"/users",
				strings.NewReader(string(body)),
			)
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
				t.Errorf("response body = %q, want %q", got, test.wantBody)
			}
		})
	}
}

func TestGetUserEndpoint(t *testing.T) {
	db := testutil.OpenTestDB(t)

	testutil.ResetUsers(t, db)
	t.Cleanup(func() {
		testutil.ResetUsers(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	userModel := &models.UserModel{DB: db}
	createdUser, err := userModel.Create(ctx, "saad@example.com", "Saad", "correct-password")
	if err != nil {
		t.Fatalf("create user for GET test: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/users/"+strconv.FormatInt(createdUser.ID, 10),
		nil,
	)
	recorder := httptest.NewRecorder()
	newTestApplication(db).routes().ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Errorf("status code = %d, want %d", response.StatusCode, http.StatusOK)
	}

	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	var body userResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode JSON response: %v", err)
	}

	if body.ID != createdUser.ID {
		t.Errorf("id = %d, want %d", body.ID, createdUser.ID)
	}

	if body.Email != "saad@example.com" {
		t.Errorf("email = %q, want %q", body.Email, "saad@example.com")
	}

	if body.Name != "Saad" {
		t.Errorf("name = %q, want %q", body.Name, "Saad")
	}
}

func TestGetUserEndpointRejectsInvalidID(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/users/not-a-number", nil)
	recorder := httptest.NewRecorder()

	newTestApplication(nil).routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status code = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	if got := recorder.Body.String(); got != "user id must be positive integer\n" {
		t.Errorf("response body = %q, want %q", got, "user id must be positive integer\n")
	}
}

func TestVerifyEmailEndpoint(t *testing.T) {
	db := testutil.OpenTestDB(t)

	testutil.ResetUsers(t, db)
	t.Cleanup(func() {
		testutil.ResetUsers(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	userModel := &models.UserModel{DB: db}
	user, err := userModel.Create(ctx, "verify@example.com", "Verify User", "correct-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	tokenModel := &models.EmailVerificationTokenModel{DB: db}
	token, err := models.NewEmailVerificationToken(user.ID, emailVerificationTokenTTL)
	if err != nil {
		t.Fatalf("create verification token: %v", err)
	}

	if err := tokenModel.Upsert(ctx, token); err != nil {
		t.Fatalf("store verification token: %v", err)
	}

	requestBody, err := json.Marshal(verifyEmailRequest{
		Token: token.Plaintext,
	})
	if err != nil {
		t.Fatalf("marshal verification request: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/users/verify-email",
		strings.NewReader(string(requestBody)),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	newTestApplication(db).routes().ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Errorf("status code = %d, want %d", response.StatusCode, http.StatusOK)
	}

	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	var body verifyEmailResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Message != "email verified" {
		t.Errorf("message = %q, want %q", body.Message, "email verified")
	}
}
