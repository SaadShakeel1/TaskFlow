package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/ai"
	"github.com/SaadShakeel1/TaskFlow/internal/database"
	"github.com/SaadShakeel1/TaskFlow/internal/mailer"
	"github.com/SaadShakeel1/TaskFlow/internal/models"
	"github.com/SaadShakeel1/TaskFlow/internal/notifications"
	"github.com/joho/godotenv"
)

type mailerConfig struct {
	smtpAddress  string
	smtpUsername string
	smtpPassword string
	from         string
}

type aiConfig struct {
	groqAPIKey     string
	groqModel      string
	pollInterval   time.Duration
	processTimeout time.Duration
}

type config struct {
	addr                string
	appURL              string
	databaseURL         string
	mailer              mailerConfig
	ai                  aiConfig
	sessionTTL          time.Duration
	sessionCookieSecure bool
}

type notificationDispatcher interface {
	Enqueue(email mailer.Email) error
	Shutdown(ctx context.Context) error
}

type aiWorker interface {
	Run(context.Context)
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
	taskDependencies   *models.TaskDependencyModel
	activity           *models.ActivityModel
	aiAnalysisRuns     *models.AIAnalysisRunModel
	mailer             mailer.Sender
	notifications      notificationDispatcher
	rateLimiter        *rateLimiter
	aiWorker           aiWorker
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
	flag.StringVar(&cfg.ai.groqAPIKey, "groq-api-key", os.Getenv("GROQ_API_KEY"), "Groq API key")
	flag.StringVar(&cfg.ai.groqModel, "groq-model", os.Getenv("GROQ_MODEL"), "Groq model")
	flag.DurationVar(&cfg.ai.pollInterval, "ai-poll-interval", time.Second, "AI worker polling interval")
	flag.DurationVar(&cfg.ai.processTimeout, "ai-process-timeout", 60*time.Second, "Maximum duration of one AI analysis")
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

	if cfg.ai.groqAPIKey == "" {
		errorLog.Fatal("GROQ_API_KEY must be set")
	}

	if cfg.ai.groqModel == "" {
		errorLog.Fatal("GROQ_MODEL must be set")
	}

	db, err := database.Open(cfg.databaseURL)
	if err != nil {
		errorLog.Fatal(err)
	}
	defer db.Close()

	emailSender := mailer.New(mailer.Config{
		SMTPAddress:  cfg.mailer.smtpAddress,
		SMTPUsername: cfg.mailer.smtpUsername,
		SMTPPassword: cfg.mailer.smtpPassword,
		From:         cfg.mailer.from,
	})

	notificationQueue := notifications.New(emailSender, errorLog)
	aiAnalyzer, err := ai.NewGroqAnalyzer(ai.GroqAnalyzerConfig{
		APIKey: cfg.ai.groqAPIKey,
		Model:  cfg.ai.groqModel,
	})
	if err != nil {
		errorLog.Fatal(err)
	}

	aiAnalysisRuns := &models.AIAnalysisRunModel{DB: db}
	aiWorker, err := ai.NewWorker(
		aiAnalysisRuns,
		aiAnalyzer,
		errorLog,
		"groq",
		cfg.ai.groqModel,
		cfg.ai.pollInterval,
		cfg.ai.processTimeout,
	)
	if err != nil {
		errorLog.Fatal(err)
	}

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
		taskDependencies:   &models.TaskDependencyModel{DB: db},
		activity:           &models.ActivityModel{DB: db},
		aiAnalysisRuns:     aiAnalysisRuns,
		rateLimiter:        newRateLimiter(),
		mailer:             emailSender,
		notifications:      notificationQueue,
		aiWorker:           aiWorker,
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

	listener, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		errorLog.Fatal(err)
	}

	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	infoLog.Printf("starting server on %s", listener.Addr())

	if err := app.serve(shutdownContext, server, listener); err != nil {
		errorLog.Fatal(err)
	}
}
