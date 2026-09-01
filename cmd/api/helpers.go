package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
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
