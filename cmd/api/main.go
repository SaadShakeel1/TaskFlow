package main

import (
	"database/sql"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"

	"github.com/SaadShakeel1/TaskFlow/internal/database"
	"github.com/SaadShakeel1/TaskFlow/internal/mailer"
	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

type mailerConfig struct {
	smtpAddress  string
	smtpUsername string
	smtpPassword string
	from         string
}

type config struct {
	addr                string
	appURL              string
	databaseURL         string
	mailer              mailerConfig
	sessionTTL          time.Duration
	sessionCookieSecure bool
}

type application struct {
	config             config
	db                 *sql.DB
	errorLog           *log.Logger
	infoLog            *log.Logger
	users              *models.UserModel
	verificationTokens *models.EmailVerificationTokenModel
	sessions           *models.SessionModel
	projects           *models.ProjectModel
	projectMembers     *models.ProjectMemberModel
	tasks              *models.TaskModel
	mailer             mailer.Sender
	taskDependencies *models.TaskDependencyModel
}

func main() {
	var cfg config

	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		errorLog.Fatalf("load .env: %v", err)
	}

	flag.StringVar(&cfg.addr, "addr", ":8081", "HTTP network address")
	flag.StringVar(&cfg.appURL, "app-url", os.Getenv("APP_URL"), "Public application URL")
	flag.StringVar(&cfg.databaseURL, "db-dsn", os.Getenv("DATABASE_URL"), "PostgreSQL data source name")
	flag.StringVar(&cfg.mailer.smtpAddress, "smtp-address", os.Getenv("SMTP_ADDRESS"), "SMTP server address")
	flag.StringVar(&cfg.mailer.smtpUsername, "smtp-username", os.Getenv("SMTP_USERNAME"), "SMTP username")
	flag.StringVar(&cfg.mailer.smtpPassword, "smtp-password", os.Getenv("SMTP_PASSWORD"), "SMTP password")
	flag.StringVar(&cfg.mailer.from, "mail-from", os.Getenv("MAIL_FROM"), "Email sender address")
	flag.DurationVar(&cfg.sessionTTL, "session-ttl", 7*24*time.Hour, "Session lifetime")
	flag.BoolVar(&cfg.sessionCookieSecure, "session-cookie-secure", false, "Require HTTPS for session cookie")
	flag.Parse()

	if cfg.databaseURL == "" {
		errorLog.Fatal("DATABASE_URL must be set")
	}

	if cfg.appURL == "" {
		errorLog.Fatal("APP_URL must be set")
	}

	if cfg.mailer.smtpAddress == "" {
		errorLog.Fatal("SMTP_ADDRESS must be set")
	}

	if cfg.mailer.from == "" {
		errorLog.Fatal("MAIL_FROM must be set")
	}

	db, err := database.Open(cfg.databaseURL)
	if err != nil {
		errorLog.Fatal(err)
	}
	defer db.Close()

	app := &application{
		config:             cfg,
		db:                 db,
		errorLog:           errorLog,
		infoLog:            infoLog,
		users:              &models.UserModel{DB: db},
		verificationTokens: &models.EmailVerificationTokenModel{DB: db},
		sessions:           &models.SessionModel{DB: db},
		projects:           &models.ProjectModel{DB: db},
		projectMembers:     &models.ProjectMemberModel{DB: db},
		tasks:              &models.TaskModel{DB: db},
		taskDependencies: &models.TaskDependencyModel{DB: db},
		mailer: mailer.New(mailer.Config{
			SMTPAddress:  cfg.mailer.smtpAddress,
			SMTPUsername: cfg.mailer.smtpUsername,
			SMTPPassword: cfg.mailer.smtpPassword,
			From:         cfg.mailer.from,
		}),
	}

	server := &http.Server{
		Addr:              cfg.addr,
		ErrorLog:          errorLog,
		Handler:           app.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       time.Minute,
	}

	infoLog.Printf("starting server on %s", cfg.addr)

	if err := server.ListenAndServe(); err != nil {
		errorLog.Fatal(err)
	}
}
