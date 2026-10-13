package main

import (
	"context"
	"database/sql"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/mailer"
	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

type discardMailer struct{}

func (discardMailer) Send(mailer.Email) error {
	return nil
}

type recordingNotificationDispatcher struct {
	mu         sync.Mutex
	emails     []mailer.Email
	enqueueErr error
}

func (d *recordingNotificationDispatcher) Enqueue(email mailer.Email) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.enqueueErr != nil {
		return d.enqueueErr
	}

	d.emails = append(d.emails, email)

	return nil
}

func (d *recordingNotificationDispatcher) Shutdown(context.Context) error {
	return nil
}

func (d *recordingNotificationDispatcher) Emails() []mailer.Email {
	d.mu.Lock()
	defer d.mu.Unlock()

	emails := make([]mailer.Email, len(d.emails))
	copy(emails, d.emails)

	return emails
}

func newTestApplication(db *sql.DB) *application {
	return &application{
		db:                 db,
		errorLog:           log.New(io.Discard, "", 0),
		infoLog:            log.New(io.Discard, "", 0),
		users:              &models.UserModel{DB: db},
		verificationTokens: &models.EmailVerificationTokenModel{DB: db},
		sessions:           &models.SessionModel{DB: db},
		mailer:             discardMailer{},
		config:             config{sessionTTL: 7 * 24 * time.Hour},
		projects:           &models.ProjectModel{DB: db},
		projectMembers:     &models.ProjectMemberModel{DB: db},
		tasks:              &models.TaskModel{DB: db},
		taskDependencies:   &models.TaskDependencyModel{DB: db},
		activity:           &models.ActivityModel{DB: db},
		aiAnalysisRuns:     &models.AIAnalysisRunModel{DB: db},
		rateLimiter:        newRateLimiter(),
	}
}

func TestReadJSON(t *testing.T) {
	type requestBody struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name        string
		body        string
		wantName    string
		wantStatus  int
		wantMessage string
	}{
		{
			name:     "valid body",
			body:     `{"name":"TaskFlow"}`,
			wantName: "TaskFlow",
		},
		{
			name:        "empty body",
			body:        "",
			wantStatus:  http.StatusBadRequest,
			wantMessage: "request body must not be empty",
		},
		{
			name:        "malformed JSON",
			body:        `{"name":`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "request body contains badly-formed JSON",
		},
		{
			name:        "unknown field",
			body:        `{"name":"TaskFlow","admin":true}`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: `request body contains unknown field "admin"`,
		},
		{
			name:        "incorrect field type",
			body:        `{"name":123}`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: `request body contains an invalid value for field "name"`,
		},
		{
			name:        "multiple JSON values",
			body:        `{"name":"first"} {"name":"second"}`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "request body must contain a single JSON value",
		},
		{
			name:        "body exceeds limit",
			body:        `{"name":"` + strings.Repeat("a", 1<<20) + `"}`,
			wantStatus:  http.StatusRequestEntityTooLarge,
			wantMessage: "request body must not be larger than 1048576 bytes",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPost,
				"/",
				strings.NewReader(test.body),
			)

			recorder := httptest.NewRecorder()

			var body requestBody
			requestError := readJSON(recorder, request, &body)

			if test.wantStatus == 0 {
				if requestError != nil {
					t.Fatalf("readJSON returned error: %s", requestError.message)
				}

				if body.Name != test.wantName {
					t.Errorf(
						"decoded name = %q, want %q",
						body.Name,
						test.wantName,
					)
				}

				return
			}

			if requestError == nil {
				t.Fatal("readJSON returned nil error")
			}

			if requestError.status != test.wantStatus {
				t.Errorf(
					"status = %d, want %d",
					requestError.status,
					test.wantStatus,
				)
			}

			if requestError.message != test.wantMessage {
				t.Errorf(
					"message = %q, want %q",
					requestError.message,
					test.wantMessage,
				)
			}
		})
	}
}
