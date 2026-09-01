package main

import (
	"encoding/json"
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

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		app.clientError(w, http.StatusBadRequest, "request body must be valid JSON")
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
