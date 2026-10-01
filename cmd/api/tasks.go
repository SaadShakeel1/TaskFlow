package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

const (
	maximumTaskTitleCharacters              = 150
	maximumTaskDescriptionCharacters        = 5000
	maximumTaskAcceptanceCriteriaCharacters = 3000
)

type createTaskRequest struct {
	AssigneeID         *int64               `json:"assigneeId"`
	Title              string               `json:"title"`
	Description        string               `json:"description"`
	AcceptanceCriteria string               `json:"acceptanceCriteria"`
	Priority           *models.TaskPriority `json:"priority"`
	DueDate            *string              `json:"dueDate"`
	EstimateMinutes    *int                 `json:"estimateMinutes"`
}

type taskResponse struct {
	ID                 int64               `json:"id"`
	ProjectID          int64               `json:"projectId"`
	CreatedByUserID    int64               `json:"createdByUserId"`
	AssigneeID         *int64              `json:"assigneeId"`
	Title              string              `json:"title"`
	Description        string              `json:"description"`
	AcceptanceCriteria string              `json:"acceptanceCriteria"`
	Status             models.TaskStatus   `json:"status"`
	Priority           models.TaskPriority `json:"priority"`
	DueDate            *string             `json:"dueDate"`
	EstimateMinutes    *int                `json:"estimateMinutes"`
	BlockedReason      *string             `json:"blockedReason"`
	StartedAt          *time.Time          `json:"startedAt"`
	CompletedAt        *time.Time          `json:"completedAt"`
	Version            int                 `json:"version"`
	CreatedAt          time.Time           `json:"createdAt"`
	UpdatedAt          time.Time           `json:"updatedAt"`
}

func (app *application) createTaskHandler(w http.ResponseWriter, r *http.Request) {
	project, user, ok := app.projectForRequest(w, r)
	if !ok {
		return
	}

	var request createTaskRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		app.clientError(w, http.StatusBadRequest, "request body must be valid JSON")
		return
	}

	title := strings.TrimSpace(request.Title)
	description := strings.TrimSpace(request.Description)
	acceptanceCriteria := strings.TrimSpace(request.AcceptanceCriteria)

	if title == "" {
		app.clientError(w, http.StatusBadRequest, "task title is required")
		return
	}

	if utf8.RuneCountInString(title) > maximumTaskTitleCharacters {
		app.clientError(w, http.StatusBadRequest, "task title must not exceed 150 characters")
		return
	}

	if utf8.RuneCountInString(description) > maximumTaskDescriptionCharacters {
		app.clientError(w, http.StatusBadRequest, "task description must not exceed 5000 characters")
		return
	}

	if utf8.RuneCountInString(acceptanceCriteria) > maximumTaskAcceptanceCriteriaCharacters {
		app.clientError(w, http.StatusBadRequest, "task acceptance criteria must not exceed 3000 characters")
		return
	}

	if request.AssigneeID != nil && *request.AssigneeID < 1 {
		app.clientError(w, http.StatusBadRequest, "assignee id must be a positive integer")
		return
	}

	if request.EstimateMinutes != nil && *request.EstimateMinutes < 1 {
		app.clientError(w, http.StatusBadRequest, "estimate minutes must be a positive integer")
		return
	}

	// Omitted priority uses the business default from the database schema.
	priority := models.TaskPriorityMedium
	if request.Priority != nil {
		priority = *request.Priority
	}

	if !models.IsValidTaskPriority(priority) {
		app.clientError(w, http.StatusBadRequest, "task priority is invalid")
		return
	}

	dueDate, err := parseTargetDate(request.DueDate)
	if err != nil {
		app.clientError(w, http.StatusBadRequest, "due date must use YYYY-MM-DD")
		return
	}

	task, err := app.tasks.Create(
		r.Context(),
		project.ID,
		user.ID,
		models.TaskCreateInput{
			AssigneeID:         request.AssigneeID,
			Title:              title,
			Description:        description,
			AcceptanceCriteria: acceptanceCriteria,
			Priority:           priority,
			DueDate:            dueDate,
			EstimateMinutes:    request.EstimateMinutes,
		},
	)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.clientError(w, http.StatusForbidden, "task creation is not permitted")
			return
		}

		app.serverError(w, err)
		return
	}

	if err := app.writeJSON(w, http.StatusCreated, newTaskResponse(task)); err != nil {
		app.errorLog.Printf("encode task response: %v", err)
	}
}

func newTaskResponse(task models.Task) taskResponse {
	response := taskResponse{
		ID:                 task.ID,
		ProjectID:          task.ProjectID,
		CreatedByUserID:    task.CreatedByUserID,
		AssigneeID:         task.AssigneeID,
		Title:              task.Title,
		Description:        task.Description,
		AcceptanceCriteria: task.AcceptanceCriteria,
		Status:             task.Status,
		Priority:           task.Priority,
		EstimateMinutes:    task.EstimateMinutes,
		BlockedReason:      task.BlockedReason,
		StartedAt:          task.StartedAt,
		CompletedAt:        task.CompletedAt,
		Version:            task.Version,
		CreatedAt:          task.CreatedAt,
		UpdatedAt:          task.UpdatedAt,
	}

	// PostgreSQL DATE is returned as YYYY-MM-DD, not a timestamp.
	if task.DueDate != nil {
		dueDate := task.DueDate.Format(time.DateOnly)
		response.DueDate = &dueDate
	}

	return response
}

func (app *application) getTaskHandler(w http.ResponseWriter, r *http.Request) {
	project, user, ok := app.projectForRequest(w, r)

	if !ok {
		return
	}

	taskID, err := strconv.ParseInt(r.PathValue("taskID"), 10, 64)
	if err != nil || taskID < 1 {
		app.clientError(w, http.StatusBadRequest, "task id must be positive a integer")
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

	if err := app.writeJSON(w, http.StatusOK, newTaskResponse(task)); err != nil {
		app.errorLog.Printf("encode task response: %v", err)
	}
}
