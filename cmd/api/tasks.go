package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

const (
	defaultTaskPage     = 1
	defaultTaskPageSize = 20
	maximumTaskPageSize = 100
)

type taskListResponse struct {
	Tasks      []taskResponse     `json:"tasks"`
	Pagination paginationResponse `json:"pagination"`
}

type optionalInt64 struct {
	Set   bool
	Null  bool
	Value int64
}

type optionalInt struct {
	Set   bool
	Null  bool
	Value int
}

func (o *optionalInt64) UnmarshalJSON(data []byte) error {
	o.Set = true

	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		o.Null = true
		return nil
	}

	return json.Unmarshal(data, &o.Value)
}

func (o *optionalInt) UnmarshalJSON(data []byte) error {
	o.Set = true

	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		o.Null = true
		return nil
	}

	return json.Unmarshal(data, &o.Value)
}

type updateTaskRequest struct {
	Title              optionalString `json:"title"`
	Description        optionalString `json:"description"`
	AcceptanceCriteria optionalString `json:"acceptanceCriteria"`
	AssigneeID         optionalInt64  `json:"assigneeId"`
	Priority           optionalString `json:"priority"`
	DueDate            optionalString `json:"dueDate"`
	EstimateMinutes    optionalInt    `json:"estimateMinutes"`
	Status             optionalString `json:"status"`
	BlockedReason      optionalString `json:"blockedReason"`
	Version            int            `json:"version"`
}

func isValidTaskStatusValue(status models.TaskStatus) bool {
	switch status {
	case models.TaskStatusTODO,
		models.TaskStatusInProgress,
		models.TaskStatusBlocked,
		models.TaskStatusDone:
		return true
	default:
		return false
	}
}
func canTransitionTaskStatus(current models.TaskStatus, next models.TaskStatus) bool {
	if current == next {
		return true
	}

	switch current {
	case models.TaskStatusTODO:
		return next == models.TaskStatusInProgress

	case models.TaskStatusInProgress:
		return next == models.TaskStatusBlocked ||
			next == models.TaskStatusDone

	case models.TaskStatusBlocked:
		return next == models.TaskStatusInProgress ||
			next == models.TaskStatusDone

	case models.TaskStatusDone:
		return false

	default:
		return false
	}
}
func mergeTaskUpdate(task models.Task, request updateTaskRequest) (models.TaskUpdateInput, error) {
	if request.Version < 1 {
		return models.TaskUpdateInput{}, errors.New(
			"version must be a positive integer",
		)
	}

	update := models.TaskUpdateInput{
		Title:              task.Title,
		Description:        task.Description,
		AcceptanceCriteria: task.AcceptanceCriteria,
		AssigneeID:         task.AssigneeID,
		Priority:           task.Priority,
		DueDate:            task.DueDate,
		EstimateMinutes:    task.EstimateMinutes,
		Status:             task.Status,
		BlockedReason:      task.BlockedReason,
		ExpectedVersion:    request.Version,
	}

	if request.Title.Set {
		if request.Title.Null {
			return models.TaskUpdateInput{}, errors.New(
				"task title cannot be null",
			)
		}

		update.Title = strings.TrimSpace(request.Title.Value)
	}

	if update.Title == "" {
		return models.TaskUpdateInput{}, errors.New(
			"task title is required",
		)
	}

	if utf8.RuneCountInString(update.Title) > maximumTaskTitleCharacters {
		return models.TaskUpdateInput{}, errors.New(
			"task title must not exceed 150 characters",
		)
	}

	if request.Description.Set {
		if request.Description.Null {
			return models.TaskUpdateInput{}, errors.New(
				"task description cannot be null",
			)
		}

		update.Description = strings.TrimSpace(request.Description.Value)
	}

	if utf8.RuneCountInString(update.Description) >
		maximumTaskDescriptionCharacters {
		return models.TaskUpdateInput{}, errors.New(
			"task description must not exceed 5000 characters",
		)
	}

	if request.AcceptanceCriteria.Set {
		if request.AcceptanceCriteria.Null {
			return models.TaskUpdateInput{}, errors.New(
				"acceptance criteria cannot be null",
			)
		}

		update.AcceptanceCriteria = strings.TrimSpace(
			request.AcceptanceCriteria.Value,
		)
	}

	if utf8.RuneCountInString(update.AcceptanceCriteria) >
		maximumTaskAcceptanceCriteriaCharacters {
		return models.TaskUpdateInput{}, errors.New(
			"acceptance criteria must not exceed 3000 characters",
		)
	}

	if request.AssigneeID.Set {
		if request.AssigneeID.Null {
			update.AssigneeID = nil
		} else {
			if request.AssigneeID.Value < 1 {
				return models.TaskUpdateInput{}, errors.New(
					"assignee id must be a positive integer",
				)
			}

			assigneeID := request.AssigneeID.Value
			update.AssigneeID = &assigneeID
		}
	}

	if request.Priority.Set {
		if request.Priority.Null {
			return models.TaskUpdateInput{}, errors.New(
				"task priority cannot be null",
			)
		}

		priority := models.TaskPriority(request.Priority.Value)
		if !models.IsValidTaskPriority(priority) {
			return models.TaskUpdateInput{}, errors.New(
				"task priority is invalid",
			)
		}

		update.Priority = priority
	}

	if request.DueDate.Set {
		if request.DueDate.Null {
			update.DueDate = nil
		} else {
			dueDate, err := parseTargetDate(&request.DueDate.Value)
			if err != nil {
				return models.TaskUpdateInput{}, errors.New(
					"due date must use YYYY-MM-DD",
				)
			}

			update.DueDate = dueDate
		}
	}

	if request.EstimateMinutes.Set {
		if request.EstimateMinutes.Null {
			update.EstimateMinutes = nil
		} else {
			if request.EstimateMinutes.Value < 1 {
				return models.TaskUpdateInput{}, errors.New(
					"estimate minutes must be a positive integer",
				)
			}

			estimate := request.EstimateMinutes.Value
			update.EstimateMinutes = &estimate
		}
	}

	if request.Status.Set {
		if request.Status.Null {
			return models.TaskUpdateInput{}, errors.New(
				"task status cannot be null",
			)
		}

		status := models.TaskStatus(request.Status.Value)
		if !isValidTaskStatusValue(status) {
			return models.TaskUpdateInput{}, errors.New(
				"task status is invalid",
			)
		}

		if !canTransitionTaskStatus(task.Status, status) {
			return models.TaskUpdateInput{}, fmt.Errorf(
				"cannot change task status from %q to %q",
				task.Status,
				status,
			)
		}

		update.Status = status
	}

	if request.BlockedReason.Set {
		if request.BlockedReason.Null {
			update.BlockedReason = nil
		} else {
			reason := strings.TrimSpace(request.BlockedReason.Value)
			update.BlockedReason = &reason
		}
	}

	if update.Status == models.TaskStatusBlocked {
		if update.BlockedReason == nil ||
			strings.TrimSpace(*update.BlockedReason) == "" {
			return models.TaskUpdateInput{}, errors.New(
				"blocked tasks require a blocked reason",
			)
		}
	} else {
		update.BlockedReason = nil
	}

	return update, nil
}

func (app *application) updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	project, user, ok := app.projectForRequest(w, r)
	if !ok {
		return
	}

	taskID, err := strconv.ParseInt(r.PathValue("taskID"), 10, 64)
	if err != nil || taskID < 1 {
		app.clientError(
			w,
			http.StatusBadRequest,
			"task id must be a positive integer",
		)
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

	canEdit, err := app.userCanEditProject(
		r.Context(),
		project,
		user.ID,
	)
	if err != nil {
		app.serverError(w, err)
		return
	}

	if !canEdit {
		app.clientError(
			w,
			http.StatusForbidden,
			"project editor permission required",
		)
		return
	}

	var request updateTaskRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		app.clientError(
			w,
			http.StatusBadRequest,
			"request body must be valid JSON",
		)
		return
	}

	update, err := mergeTaskUpdate(task, request)
	if err != nil {
		app.clientError(w, http.StatusBadRequest, err.Error())
		return
	}

	updatedTask, err := app.tasks.UpdateByIDForUser(
		r.Context(),
		task.ID,
		user.ID,
		update,
	)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			currentTask, currentErr := app.tasks.GetByIDForUser(
				r.Context(),
				task.ID,
				user.ID,
			)

			if currentErr == nil &&
				currentTask.Version != request.Version {
				app.clientError(
					w,
					http.StatusConflict,
					"task was changed; fetch the latest version and try again",
				)
				return
			}

			app.notFound(w, "task not found")
			return
		}

		app.serverError(w, err)
		return
	}

	if err := app.writeJSON(w, http.StatusOK, newTaskResponse(updatedTask)); err != nil {
		app.errorLog.Printf(
			"encode updated task response: %v",
			err,
		)
	}
}

func (app *application) userCanEditProject(ctx context.Context, project models.Project, userID int64) (bool, error) {
	if project.UserID == userID {
		return true, nil
	}

	members, err := app.projectMembers.ListByProject(
		ctx,
		project.ID,
	)
	if err != nil {
		return false, err
	}

	for _, member := range members {
		if member.UserID == userID &&
			member.Role == models.ProjectMemberRoleEditor {
			return true, nil
		}
	}

	return false, nil
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

func parseTaskListOptions(r *http.Request) (models.TaskListOptions, error) {
	options := models.TaskListOptions{
		Page:     defaultTaskPage,
		PageSize: defaultTaskPageSize,
		Sort:     models.TaskSortUpdatedAt,
		Order:    models.SortOrderDescending,
	}

	query := r.URL.Query()

	if value := query.Get("page"); value != "" {
		page, err := strconv.Atoi(value)
		if err != nil || page < 1 {
			return models.TaskListOptions{}, errors.New(
				"page must be a positive integer",
			)
		}

		options.Page = page
	}

	if value := query.Get("pageSize"); value != "" {
		pageSize, err := strconv.Atoi(value)
		if err != nil || pageSize < 1 || pageSize > maximumTaskPageSize {
			return models.TaskListOptions{}, errors.New(
				"page size must be between 1 and 100",
			)
		}

		options.PageSize = pageSize
	}

	if value := query.Get("status"); value != "" {
		status := models.TaskStatus(value)

		switch status {
		case models.TaskStatusTODO,
			models.TaskStatusInProgress,
			models.TaskStatusBlocked,
			models.TaskStatusDone:
			options.Status = &status
		default:
			return models.TaskListOptions{}, errors.New(
				"task status is invalid",
			)
		}
	}

	if value := query.Get("priority"); value != "" {
		priority := models.TaskPriority(value)

		if !models.IsValidTaskPriority(priority) {
			return models.TaskListOptions{}, errors.New(
				"task priority is invalid",
			)
		}

		options.Priority = &priority
	}

	if value := query.Get("sort"); value != "" {
		sort := models.TaskSort(value)

		switch sort {
		case models.TaskSortUpdatedAt,
			models.TaskSortCreatedAt,
			models.TaskSortDueDate,
			models.TaskSortPriority:
			options.Sort = sort
		default:
			return models.TaskListOptions{}, errors.New(
				"task sort is invalid",
			)
		}
	}

	if value := query.Get("order"); value != "" {
		order := models.SortOrder(value)

		switch order {
		case models.SortOrderAscending,
			models.SortOrderDescending:
			options.Order = order
		default:
			return models.TaskListOptions{}, errors.New(
				"sort order is invalid",
			)
		}
	}

	return options, nil
}

func (app *application) listTasksHandler(w http.ResponseWriter, r *http.Request) {
	project, user, ok := app.projectForRequest(w, r)
	if !ok {
		return
	}

	options, err := parseTaskListOptions(r)

	if err != nil {
		app.clientError(w, http.StatusBadRequest, err.Error())
		return
	}

	taskList, err := app.tasks.ListByProjectForUser(r.Context(), project.ID, user.ID, options)

	if err != nil {
		app.serverError(w, err)
	}

	tasks := make([]taskResponse, 0, len(taskList.Tasks))

	for _, task := range taskList.Tasks {
		tasks = append(tasks, newTaskResponse(task))
	}

	response := taskListResponse{
		Tasks: tasks,
		Pagination: paginationResponse{
			Page:       options.Page,
			PageSize:   options.PageSize,
			TotalItems: taskList.TotalItems,
			TotalPages: (taskList.TotalItems + options.PageSize - 1) / options.PageSize,
		},
	}

	if err := app.writeJSON(w, http.StatusOK, response); err != nil {
		app.errorLog.Printf("encode task list response: %v", err)
	}
}

func (app *application) deleteTaskHandler(w http.ResponseWriter, r *http.Request) {
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

	versionValue := r.URL.Query().Get("version")

	if versionValue == "" {
		app.clientError(w, http.StatusBadRequest, "version is required")
		return
	}

	expectedVersion, err := strconv.Atoi(versionValue)
	if err != nil || expectedVersion < 1 {
		app.clientError(w, http.StatusBadRequest, "version must be positive integer")
		return
	}

	err = app.tasks.DeleteByIDForUser(r.Context(), task.ID, user.ID, expectedVersion)

	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			currentTask, currentErr := app.tasks.GetByIDForUser(r.Context(), task.ID, user.ID)
			if currentErr == nil && currentTask.Version != expectedVersion {
				app.clientError(w, http.StatusConflict, "task was changed; fetch the latest version and try again")
				return
			}

			app.notFound(w, "task not found")
			return
		}
		app.serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
