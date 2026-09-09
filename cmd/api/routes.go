package main

import "net/http"

func (app *application) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", app.healthHandler)
	mux.HandleFunc("POST /greet", app.greetingHandler)
	mux.HandleFunc("POST /users", app.createUserHandler)
	mux.HandleFunc("GET /users/{id}", app.getUserHandler)
	mux.HandleFunc("POST /users/verify-email", app.verifyEmailHandler)
	mux.HandleFunc("POST /sessions", app.createSessionHandler)
	mux.HandleFunc("GET /verify-email", app.verifyEmailPageHandler)

	return app.recoverPanic(app.logRequest(secureHeaders(mux)))
}
