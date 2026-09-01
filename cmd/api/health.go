package main

import (
	"net/http"
)

type healthResponse struct {
	Status string `json:"status"`
}

func (app *application) healthHandler(w http.ResponseWriter, _ *http.Request) {
	if err := app.writeJSON(w, http.StatusOK, healthResponse{Status: "ok"}); err != nil {
		app.errorLog.Printf("encode health response: %v", err)
	}
}
