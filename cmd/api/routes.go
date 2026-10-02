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
	mux.Handle("GET /me", app.requireAuthentication(http.HandlerFunc(app.currentUserHandler)))
	mux.Handle("DELETE /sessions/current", app.requireAuthentication(http.HandlerFunc(app.deleteCurrentSessionHandler)))
	mux.Handle("POST /projects", app.requireAuthentication(http.HandlerFunc(app.createProjectHandler)))
	mux.Handle("GET /projects", app.requireAuthentication(http.HandlerFunc(app.listProjectsHandler)))
	mux.Handle("GET /projects/{id}", app.requireAuthentication(http.HandlerFunc(app.getProjectHandler)))
	mux.Handle("POST /projects/{id}/members", app.requireAuthentication(http.HandlerFunc(app.createProjectMemberHandler)))
	mux.Handle("GET /projects/{id}/members", app.requireAuthentication(http.HandlerFunc(app.listProjectMembersHandler)))
	mux.Handle("PATCH /projects/{id}/members/{userID}", app.requireAuthentication(http.HandlerFunc(app.updateProjectMemberHandler)))
	mux.Handle("DELETE /projects/{id}/members/{userID}", app.requireAuthentication(http.HandlerFunc(app.deleteProjectMemberHandler)))
	mux.Handle("PATCH /projects/{id}", app.requireAuthentication(http.HandlerFunc(app.updateProjectHandler)))
	mux.Handle("DELETE /projects/{id}", app.requireAuthentication(http.HandlerFunc(app.deleteProjectHandler)))
	mux.Handle("POST /projects/{id}/tasks", app.requireAuthentication(http.HandlerFunc(app.createTaskHandler)))
	mux.Handle("GET /projects/{id}/tasks/{taskID}", app.requireAuthentication(http.HandlerFunc(app.getTaskHandler)))
	mux.Handle("GET /projects/{id}/tasks", app.requireAuthentication(http.HandlerFunc(app.listTasksHandler)))
	mux.Handle("PATCH /projects/{id}/tasks/{taskID}", app.requireAuthentication(http.HandlerFunc(app.updateTaskHandler)))
	return app.recoverPanic(app.logRequest(secureHeaders(mux)))
}
