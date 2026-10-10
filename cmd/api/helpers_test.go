package main

import (
	"context"
	"database/sql"
	"io"
	"log"
	"sync"
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
		rateLimiter:        newRateLimiter(),
	}
}
