package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

const (
	defaultActivityPage     = 1
	defaultActivityPageSize = 20
	maximumActivityPageSize = 100
)

type activityResponse struct {
	ID         int64     `json:"id"`
	ProjectID  int64     `json:"projectId"`
	UserID     *int64    `json:"userId"`
	Action     string    `json:"action"`
	EntityType string    `json:"entityType"`
	EntityID   *int64    `json:"entityId"`
	Metadata   any       `json:"metadata"`
	CreatedAt  time.Time `json:"createdAt"`
}

type activityListResponse struct {
	Activities []activityResponse `json:"activities"`
	Pagination paginationResponse `json:"pagination"`
}

func newActivityResponse(activity models.Activity) activityResponse {
	return activityResponse{
		ID:         activity.ID,
		ProjectID:  activity.ProjectID,
		UserID:     activity.UserID,
		Action:     activity.Action,
		EntityType: activity.EntityType,
		EntityID:   activity.EntityID,
		Metadata:   activity.Metadata,
		CreatedAt:  activity.CreatedAt,
	}
}

func parseActivityListOptions(r *http.Request) (models.ActivityListOptions, error) {
	options := models.ActivityListOptions{
		Page:     defaultActivityPage,
		PageSize: defaultActivityPageSize,
	}

	query := r.URL.Query()

	if value := query.Get("page"); value != "" {
		page, err := strconv.Atoi(value)
		if err != nil || page < 1 {
			return models.ActivityListOptions{}, errors.New(
				"page must be a positive integer",
			)
		}

		options.Page = page
	}

	if value := query.Get("pageSize"); value != "" {
		pageSize, err := strconv.Atoi(value)
		if err != nil || pageSize < 1 || pageSize > maximumActivityPageSize {
			return models.ActivityListOptions{}, errors.New(
				"page size must be between 1 and 100",
			)
		}

		options.PageSize = pageSize
	}

	if value := strings.TrimSpace(query.Get("action")); value != "" {
		options.Action = &value
	}

	if value := strings.TrimSpace(query.Get("entityType")); value != "" {
		options.EntityType = &value
	}

	if value := query.Get("entityId"); value != "" {
		entityID, err := strconv.ParseInt(value, 10, 64)
		if err != nil || entityID < 1 {
			return models.ActivityListOptions{}, errors.New(
				"entity id must be a positive integer",
			)
		}

		options.EntityID = &entityID
	}

	return options, nil
}

func (app *application) listActivityHandler(w http.ResponseWriter, r *http.Request) {
	// Confirm that the authenticated user can access the project.
	project, user, ok := app.projectForRequest(w, r)
	if !ok {
		return
	}

	options, err := parseActivityListOptions(r)
	if err != nil {
		app.clientError(w, http.StatusBadRequest, err.Error())
		return
	}

	activityList, err := app.activity.ListByProjectForUser(
		r.Context(),
		project.ID,
		user.ID,
		options,
	)
	if err != nil {
		app.serverError(w, err)
		return
	}

	activities := make([]activityResponse, 0, len(activityList.Activities))

	for _, activity := range activityList.Activities {
		activities = append(activities, newActivityResponse(activity))
	}

	response := activityListResponse{
		Activities: activities,
		Pagination: paginationResponse{
			Page:       options.Page,
			PageSize:   options.PageSize,
			TotalItems: activityList.TotalItems,
			TotalPages: (activityList.TotalItems + options.PageSize - 1) / options.PageSize,
		},
	}

	if err := app.writeJSON(w, http.StatusOK, response); err != nil {
		app.errorLog.Printf("encode activity list response: %v", err)
	}
}
