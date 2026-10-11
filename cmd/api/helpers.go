package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

const maxJSONBodyBytes int64 = 1 << 20 // 1 MB

type jsonRequestError struct {
	status  int
	message string
}

func readJSON(w http.ResponseWriter, r *http.Request, destination any) *jsonRequestError {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		var syntaxError *json.SyntaxError
		var typeError *json.UnmarshalTypeError
		var maxBytesError *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxError):
			return &jsonRequestError{
				status: http.StatusBadRequest,
				message: fmt.Sprintf(
					"request body contains badly-formed JSON at character %d",
					syntaxError.Offset,
				),
			}

		case errors.Is(err, io.ErrUnexpectedEOF):
			return &jsonRequestError{
				status:  http.StatusBadRequest,
				message: "request body contains badly-formed JSON",
			}

		case errors.As(err, &typeError):
			if typeError.Field != "" {
				return &jsonRequestError{
					status: http.StatusBadRequest,
					message: fmt.Sprintf(
						"request body contains an invalid value for field %q",
						typeError.Field,
					),
				}
			}

			return &jsonRequestError{
				status:  http.StatusBadRequest,
				message: "request body contains an invalid value",
			}

		case errors.Is(err, io.EOF):
			return &jsonRequestError{
				status:  http.StatusBadRequest,
				message: "request body must not be empty",
			}

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			field := strings.TrimPrefix(err.Error(), "json: unknown field ")

			return &jsonRequestError{
				status: http.StatusBadRequest,
				message: fmt.Sprintf(
					"request body contains unknown field %s",
					field,
				),
			}

		case errors.As(err, &maxBytesError):
			return &jsonRequestError{
				status: http.StatusRequestEntityTooLarge,
				message: fmt.Sprintf(
					"request body must not be larger than %d bytes",
					maxJSONBodyBytes,
				),
			}

		default:
			return &jsonRequestError{
				status:  http.StatusBadRequest,
				message: "request body contains invalid JSON",
			}
		}
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return &jsonRequestError{
			status:  http.StatusBadRequest,
			message: "request body must contain a single JSON value",
		}
	}

	return nil
}
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
