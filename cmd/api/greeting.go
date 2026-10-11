package main

import (
	"net/http"
	"strings"
)

type greetingRequest struct {
	Name string `json:"name"`
}

type greetingResponse struct {
	Message string `json:"message"`
}

func (app *application) greetingHandler(w http.ResponseWriter, r *http.Request) {
	var request greetingRequest

	if requestError := readJSON(w, r, &request); requestError != nil {
		app.clientError(w, requestError.status, requestError.message)
		return
	}

	name := strings.TrimSpace(request.Name)
	if name == "" {
		app.clientError(w, http.StatusBadRequest, "name is required")
		return
	}

	if err := app.writeJSON(w, http.StatusOK, greetingResponse{Message: "Hello, " + name + "!"}); err != nil {
		app.errorLog.Printf("encode greeting response: %v", err)
	}
}
