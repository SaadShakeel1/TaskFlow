package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

func (app *application) serverError(w http.ResponseWriter, err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	app.errorLog.Output(2, trace)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func (app *application) clientError(w http.ResponseWriter, status int, message string) {
	http.Error(w, message, status)
}

func (app *application) notFound(w http.ResponseWriter, message string) {
	app.clientError(w, http.StatusNotFound, message)
}

func (app *application) writeJSON(w http.ResponseWriter, status int, data any) error {
	response, err := json.Marshal(data)
	if err != nil {
		return err
	}

	response = append(response, '\n')
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_, err = w.Write(response)
	return err
}

func (app *application) recordActivity(
	ctx context.Context,
	input models.ActivityCreateInput,
) {
	_, err := app.activity.Create(ctx, input)
	if err != nil {
		// The main mutation already succeeded, so log the audit failure.
		app.errorLog.Printf("record activity: %v", err)
	}
}
