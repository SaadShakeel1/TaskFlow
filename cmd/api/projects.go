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
	maximumProjectNameCharacters        = 100
	maximumProjectDescriptionCharacters = 1000
	defaultProjectPage                  = 1
	defaultProjectPageSize              = 20
	maximumProjectPageSize              = 100
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

type paginationResponse struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}

type projectListResponse struct {
	Projects   []projectResponse  `json:"projects"`
	Pagination paginationResponse `json:"pagination"`
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

	if err := app.writeJSON(w, http.StatusCreated, newProjectResponse(project)); err != nil {
		app.errorLog.Printf("encode project response: %v", err)
	}
}

func (app *application) listProjectsHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(userContextKey).(models.User)
	if !ok {
		app.serverError(w, errors.New("authenticated user missing from context"))
		return
	}

	options, err := parseProjectListOptions(r)
	if err != nil {
		app.clientError(w, http.StatusBadRequest, err.Error())
		return
	}

	projectList, err := app.projects.ListByUser(r.Context(), user.ID, options)
	if err != nil {
		app.serverError(w, err)
		return
	}

	projects := make([]projectResponse, 0, len(projectList.Projects))
	for _, project := range projectList.Projects {
		projects = append(projects, newProjectResponse(project))
	}

	response := projectListResponse{
		Projects: projects,
		Pagination: paginationResponse{
			Page:       options.Page,
			PageSize:   options.PageSize,
			TotalItems: projectList.TotalItems,
			TotalPages: (projectList.TotalItems + options.PageSize - 1) / options.PageSize,
		},
	}

	if err := app.writeJSON(w, http.StatusOK, response); err != nil {
		app.errorLog.Printf("encode projects response: %v", err)
	}
}

func (app *application) getProjectHandler(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || projectID < 1 {
		app.clientError(w, http.StatusBadRequest, "project id must be a positive integer")
		return
	}

	user, ok := r.Context().Value(userContextKey).(models.User)
	if !ok {
		app.serverError(w, errors.New("authenticated user missing from context"))
		return
	}

	project, err := app.projects.GetByIDForUser(r.Context(), projectID, user.ID)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w, "project not found")
			return
		}
		app.serverError(w, err)
		return
	}

	if err := app.writeJSON(w, http.StatusOK, newProjectResponse(project)); err != nil {
		app.errorLog.Printf("encode project response: %v", err)
	}
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

func newProjectResponse(project models.Project) projectResponse {
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

	return response
}

func parseProjectListOptions(r *http.Request) (models.ProjectListOptions, error) {
	options := models.ProjectListOptions{
		Page:     defaultProjectPage,
		PageSize: defaultProjectPageSize,
		Sort:     models.ProjectSortUpdatedAt,
		Order:    models.SortOrderDescending,
	}

	query := r.URL.Query()

	if value := query.Get("page"); value != "" {
		page, err := strconv.Atoi(value)
		if err != nil || page < 1 {
			return models.ProjectListOptions{}, errors.New("page must be a positive integer")
		}
		options.Page = page
	}

	if value := query.Get("pageSize"); value != "" {
		pageSize, err := strconv.Atoi(value)
		if err != nil || pageSize < 1 || pageSize > maximumProjectPageSize {
			return models.ProjectListOptions{}, errors.New("page size must be between 1 and 100")
		}
		options.PageSize = pageSize
	}

	if value := query.Get("status"); value != "" {
		status := models.ProjectStatus(value)
		switch status {
		case models.ProjectStatusActive, models.ProjectStatusCompleted, models.ProjectStatusArchived:
			options.Status = &status
		default:
			return models.ProjectListOptions{}, errors.New("project status is invalid")
		}
	}

	if value := query.Get("sort"); value != "" {
		sort := models.ProjectSort(value)
		switch sort {
		case models.ProjectSortUpdatedAt, models.ProjectSortName, models.ProjectSortTargetDate:
			options.Sort = sort
		default:
			return models.ProjectListOptions{}, errors.New("project sort is invalid")
		}
	}

	if value := query.Get("order"); value != "" {
		order := models.SortOrder(value)
		switch order {
		case models.SortOrderAscending, models.SortOrderDescending:
			options.Order = order
		default:
			return models.ProjectListOptions{}, errors.New("sort order is invalid")
		}
	}

	return options, nil
}
