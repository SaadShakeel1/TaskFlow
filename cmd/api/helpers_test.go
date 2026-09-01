package main

import (
	"database/sql"
	"io"
	"log"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

func newTestApplication(db *sql.DB) *application {
	return &application{
		db:       db,
		errorLog: log.New(io.Discard, "", 0),
		infoLog:  log.New(io.Discard, "", 0),
		users:    &models.UserModel{DB: db},
	}
}
