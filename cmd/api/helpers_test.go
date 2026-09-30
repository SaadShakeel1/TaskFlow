package main

import (
	"database/sql"
	"io"
	"log"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/mailer"
	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

type discardMailer struct{}

func (discardMailer) Send(mailer.Email) error {
	return nil
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
		tasks: &models.TaskModel{DB: db},
	}
}
