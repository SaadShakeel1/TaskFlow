package main

import (
	"bytes"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"encoding/json"

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

type optionalString struct {
	Set   bool
	Null  bool
	Value string
}

func (o *optionalString) UnmarshalJSON(data []byte) error {
	o.Set = true
	o.Null = bytes.Equal(bytes.TrimSpace(data), []byte("null"))

	if o.Null {
		o.Value = ""
		return nil
	}

	return json.Unmarshal(data, &o.Value)
}

type updateProjectRequest struct {
	Name        optionalString `json:"name"`
	Description optionalString `json:"description"`
	Status      optionalString `json:"status"`
	TargetDate  optionalString `json:"targetDate"`
}

func mergeProjectUpdate(project models.Project, request updateProjectRequest) (models.ProjectUpdate, error) {
	update := models.ProjectUpdate{
		Name:        project.Name,
		Description: project.Description,
		Status:      project.Status,
		TargetDate:  project.TargetDate,
	}

	if request.Name.Set {
		if request.Name.Null {
			return models.ProjectUpdate{}, errors.New("project name cannot be null")
		}
		update.Name = strings.TrimSpace(request.Name.Value)
	}

	if request.Description.Set {
		if request.Description.Null {
			return models.ProjectUpdate{}, errors.New("project description cannot be empty")
		}
		update.Description = strings.TrimSpace(request.Description.Value)
	}

	if request.Status.Set {
		if request.Status.Null {
			return models.ProjectUpdate{}, errors.New("project status cannot be null")
		}
		status := models.ProjectStatus(request.Status.Value)

		switch status {
		case models.ProjectStatusActive,
			models.ProjectStatusArchived,
			models.ProjectStatusCompleted:
			update.Status = status

		default:
			return models.ProjectUpdate{}, errors.New("project status is invalid")
		}
	}

	if request.TargetDate.Set {
		if request.TargetDate.Null {
			update.TargetDate = nil
		} else {
			targetDate, err := parseTargetDate(&request.TargetDate.Value)
			if err != nil {
				return models.ProjectUpdate{}, errors.New("target date must use YYYY-MM-DD")
			}
			update.TargetDate = targetDate
		}
	}

	if update.Name == "" {
		return models.ProjectUpdate{}, errors.New("project name is required")
	}

	if utf8.RuneCountInString(update.Name) > maximumProjectNameCharacters {
		return models.ProjectUpdate{}, errors.New(
			"project name must not exceed 100 characters",
		)
	}

	if utf8.RuneCountInString(update.Description) > maximumProjectDescriptionCharacters {
		return models.ProjectUpdate{}, errors.New(
			"project description must not exceed 1000 characters",
		)
	}

	return update, nil
}

func (app *application) createProjectHandler(w http.ResponseWriter, r *http.Request) {
	var request createProjectRequest

	if requestError := readJSON(w, r, &request); requestError != nil {
		app.clientError(w, requestError.status, requestError.message)
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

	app.recordActivity(r.Context(), models.ActivityCreateInput{
		ProjectID:  project.ID,
		UserID:     &user.ID,
		Action:     "created",
		EntityType: "project",
		EntityID:   &project.ID,
	})

	if err := app.writeJSON(w, http.StatusCreated, newProjectResponse(project)); err != nil {
		app.errorLog.Printf("encode project response: %v", err)
	}
}

func (app *application) updateProjectHandler(w http.ResponseWriter, r *http.Request) {
	project, user, ok := app.projectForRequest(w, r)

	if !ok {
		return
	}

	var request updateProjectRequest

	if requestError := readJSON(w, r, &request); requestError != nil {
		app.clientError(w, requestError.status, requestError.message)
		return
	}

	if !request.Name.Set && !request.Description.Set && !request.Status.Set && !request.TargetDate.Set {
		app.clientError(w, http.StatusBadRequest, "atleast one project field must be provided")
		return
	}

	update, err := mergeProjectUpdate(project, request)
	if err != nil {
		app.clientError(w, http.StatusBadRequest, err.Error())
		return
	}

	updatedProject, err := app.projects.UpdateByIDForUser(r.Context(), project.ID, user.ID, update)

	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w, "project not found")
			return
		}

		app.serverError(w, err)
		return
	}

	app.recordActivity(r.Context(), models.ActivityCreateInput{
		ProjectID:  updatedProject.ID,
		UserID:     &user.ID,
		Action:     "updated",
		EntityType: "project",
		EntityID:   &updatedProject.ID,
	})

	if err := app.writeJSON(w, http.StatusOK, newProjectResponse(updatedProject)); err != nil {
		app.errorLog.Printf("encode updated project response: %v", err)
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

func (app *application) deleteProjectHandler(w http.ResponseWriter, r *http.Request) {
	project, user, ok := app.ownedProjectForRequest(w, r)

	if !ok {
		return
	}

	err := app.projects.DeleteByIDForUser(r.Context(), project.ID, user.ID)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w, "project not found")
		}
		app.serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
