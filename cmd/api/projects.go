package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

const (
	maximumProjectNameCharacters        = 100
	maximumProjectDescriptionCharacters = 1000
)

type createProjectRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	TargetDate  *string `json:"targetDate"`
}

type projectResponse struct {
	ID          int64                `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Status      models.ProjectStatus `json:"status"`
	TargetDate  *string              `json:"targetDate"`
	CreatedAt   time.Time            `json:"createdAt"`
	UpdatedAt   time.Time            `json:"updatedAt"`
}

func (app *application) createProjectHandler(w http.ResponseWriter, r *http.Request) {
	var request createProjectRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		app.clientError(w, http.StatusBadRequest, "request body must be valid JSON")
		return
	}

	name := strings.TrimSpace(request.Name)
	description := strings.TrimSpace(request.Description)

	if name == "" {
		app.clientError(w, http.StatusBadRequest, "project name is required")
		return
	}

	if utf8.RuneCountInString(name) > maximumProjectNameCharacters {
		app.clientError(w, http.StatusBadRequest, "project name must not exceed 100 characters")
		return
	}

	if utf8.RuneCountInString(description) > maximumProjectDescriptionCharacters {
		app.clientError(w, http.StatusBadRequest, "project description must not exceed 1000 characters")
		return
	}

	targetDate, err := parseTargetDate(request.TargetDate)
	if err != nil {
		app.clientError(w, http.StatusBadRequest, "target date must use YYYY-MM-DD")
		return
	}

	user, ok := r.Context().Value(userContextKey).(models.User)

	if !ok {
		app.serverError(w, errors.New("authenticated user missing from context"))
		return
	}

	project, err := app.projects.Create(r.Context(), user.ID, name, description, targetDate)

	if err != nil {
		app.serverError(w, err)
		return
	}
	app.writeProjectResponse(w, project, http.StatusCreated)

}

func parseTargetDate(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}

	targetDate, err := time.Parse(time.DateOnly, *value)
	if err != nil {
		return nil, err
	}

	return &targetDate, nil
}

func (app *application) writeProjectResponse(w http.ResponseWriter, project models.Project, status int) {
	response := projectResponse{
		ID:          project.ID,
		Name:        project.Name,
		Description: project.Description,
		Status:      project.Status,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}

	if project.TargetDate != nil {
		targetDate := project.TargetDate.Format(time.DateOnly)
		response.TargetDate = &targetDate
	}

	if err := app.writeJSON(w, status, response); err != nil {
		app.errorLog.Printf("encode project response: %v", err)
	}
}
