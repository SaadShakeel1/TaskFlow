package api

import (
	"database/sql"
	"net/http"
)

type Server struct {
	db *sql.DB
}

func NewRouter(db *sql.DB) *http.ServeMux {
	server := Server{db: db}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", server.healthHandler)
	mux.HandleFunc("POST /greet", server.greetingHandler)
	mux.HandleFunc("POST /users", server.createUserHandler)
	mux.HandleFunc("GET /users/{id}", server.getUserHandler)

	return mux
}
