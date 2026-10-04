package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

type createTaskDependencyRequest struct {
	DependsOnTaskID int64 `json:"dependsOnTaskID"`
}

type taskDependencyResponse struct {
	TaskID          int64     `json:"taskId"`
	DependsOnTaskID int64     `json:"dependsOnTaskId"`
	CreatedAt       time.Time `json:"createdAt"`
}

func newTaskDependencyResponse(dependency models.TaskDependency) taskDependencyResponse {
	return taskDependencyResponse{
		TaskID:          dependency.TaskID,
		DependsOnTaskID: dependency.DependsOnTaskID,
		CreatedAt:       dependency.CreatedAt,
	}
}

func (app *application) createTaskDependencyHandler(w http.ResponseWriter, r *http.Request) {
	project, user, ok := app.projectForRequest(w, r)
	if !ok {
		return
	}

	taskID, err := strconv.ParseInt(r.PathValue("taskID"), 10, 64)
	if err != nil || taskID < 1 {
		app.clientError(w, http.StatusBadRequest, "task id must be a positive integer")
		return
	}

	task, err := app.tasks.GetByIDForUser(r.Context(), taskID, user.ID)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w, "task not found")
			return
		}
		app.serverError(w, err)
		return
	}

	if task.ProjectID != project.ID {
		app.notFound(w, "task not found")
		return
	}

	canEdit, err := app.userCanEditProject(r.Context(), project, user.ID)
	if err != nil {
		app.serverError(w, err)
		return
	}
	if !canEdit {
		app.clientError(w, http.StatusForbidden, "project editor permission required")
		return
	}

	var request createTaskDependencyRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		app.clientError(w, http.StatusBadRequest, "request body must be valid JSON")
		return
	}

	if request.DependsOnTaskID < 1 {
		app.clientError(w, http.StatusBadRequest, "dependsOnTaskId must be a positive integer")
		return
	}

	dependency, err := app.taskDependencies.Create(r.Context(), task.ID, request.DependsOnTaskID, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrDuplicateTaskDependency):
			app.clientError(w, http.StatusConflict, "task dependency already exists")
		case errors.Is(err, models.ErrTaskDependencyCycle):
			app.clientError(w, http.StatusBadRequest, "task dependency would create a cycle")
		case errors.Is(err, models.ErrNoRecord):
			app.notFound(w, "dependency task not found")
		default:
			app.serverError(w, err)
		}
		return
	}

	if err := app.writeJSON(w, http.StatusCreated, newTaskDependencyResponse(dependency)); err != nil {
		app.errorLog.Printf("encode task dependency response: %v", err)
	}
}


type taskDependencyListResponse struct {
	Dependencies []taskDependencyResponse `json:"dependencies"`
}

func (app *application) listTaskDependenciesHandler(w http.ResponseWriter, r *http.Request) {
	project, user, ok := app.projectForRequest(w, r)
	if !ok {
		return
	}

	taskID, err := strconv.ParseInt(r.PathValue("taskID"), 10, 64)
	if err != nil || taskID < 1 {
		app.clientError(w, http.StatusBadRequest, "task id must be a positive integer")
		return
	}

	task, err := app.tasks.GetByIDForUser(r.Context(), taskID, user.ID)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w, "task not found")
			return
		}

		app.serverError(w, err)
		return
	}

	if task.ProjectID != project.ID {
		app.notFound(w, "task not found")
		return
	}

	dependencies, err := app.taskDependencies.ListByTask(
		r.Context(),
		task.ID,
		user.ID,
	)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w, "task not found")
			return
		}

		app.serverError(w, err)
		return
	}

	response := taskDependencyListResponse{
		Dependencies: make(
			[]taskDependencyResponse,
			0,
			len(dependencies),
		),
	}

	for _, dependency := range dependencies {
		response.Dependencies = append(
			response.Dependencies,
			newTaskDependencyResponse(dependency),
		)
	}

	if err := app.writeJSON(w, http.StatusOK, response); err != nil {
		app.errorLog.Printf("encode task dependency list response: %v", err)
	}
}