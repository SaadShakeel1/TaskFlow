package main

import (
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/database"
	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

type config struct {
	addr        string
	databaseURL string
}

type application struct {
	config   config
	db       *sql.DB
	errorLog *log.Logger
	infoLog  *log.Logger
	users    *models.UserModel
}

func main() {
	var cfg config

	flag.StringVar(&cfg.addr, "addr", ":8081", "HTTP network address")
	flag.StringVar(&cfg.databaseURL, "db-dsn", os.Getenv("DATABASE_URL"), "PostgreSQL data source name")
	flag.Parse()

	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	if cfg.databaseURL == "" {
		errorLog.Fatal("DATABASE_URL must be set")
	}

	db, err := database.Open(cfg.databaseURL)
	if err != nil {
		errorLog.Fatal(err)
	}
	defer db.Close()

	app := &application{
		config:   cfg,
		db:       db,
		errorLog: errorLog,
		infoLog:  infoLog,
		users:    &models.UserModel{DB: db},
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
