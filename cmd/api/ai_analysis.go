package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

const (
	defaultAIAnalysisRunPage     = 1
	defaultAIAnalysisRunPageSize = 20
	maximumAIAnalysisRunPageSize = 100
)

type aiAnalysisRunResponse struct {
	ID                int64                      `json:"id"`
	ProjectID         int64                      `json:"projectId"`
	RequestedByUserID int64                      `json:"requestedByUserId"`
	Status            models.AIAnalysisRunStatus `json:"status"`
	Result            json.RawMessage            `json:"result,omitempty"`
	Provider          *string                    `json:"provider,omitempty"`
	Model             *string                    `json:"model,omitempty"`
	AttemptCount      int                        `json:"attemptCount"`
	ErrorMessage      *string                    `json:"errorMessage,omitempty"`
	StartedAt         *time.Time                 `json:"startedAt,omitempty"`
	CompletedAt       *time.Time                 `json:"completedAt,omitempty"`
	CreatedAt         time.Time                  `json:"createdAt"`
	UpdatedAt         time.Time                  `json:"updatedAt"`
}

type aiAnalysisRunListResponse struct {
	Runs       []aiAnalysisRunResponse `json:"runs"`
	Pagination paginationResponse      `json:"pagination"`
}

func newAIAnalysisRunResponse(run models.AIAnalysisRun) aiAnalysisRunResponse {
	return aiAnalysisRunResponse{
		ID:                run.ID,
		ProjectID:         run.ProjectID,
		RequestedByUserID: run.RequestedByUserID,
		Status:            run.Status,
		Result:            run.Result,
		Provider:          run.Provider,
		Model:             run.Model,
		AttemptCount:      run.AttemptCount,
		ErrorMessage:      run.ErrorMessage,
		StartedAt:         run.StartedAt,
		CompletedAt:       run.CompletedAt,
		CreatedAt:         run.CreatedAt,
		UpdatedAt:         run.UpdatedAt,
	}
}

func parseAIAnalysisRunListOptions(r *http.Request) (models.AIAnalysisRunListOptions, error) {
	options := models.AIAnalysisRunListOptions{
		Page:     defaultAIAnalysisRunPage,
		PageSize: defaultAIAnalysisRunPageSize,
	}

	query := r.URL.Query()

	if value := query.Get("page"); value != "" {
		page, err := strconv.Atoi(value)
		if err != nil || page < 1 {
			return models.AIAnalysisRunListOptions{}, errors.New("page must be a positive integer")
		}

		options.Page = page
	}

	if value := query.Get("pageSize"); value != "" {
		pageSize, err := strconv.Atoi(value)
		if err != nil || pageSize < 1 || pageSize > maximumAIAnalysisRunPageSize {
			return models.AIAnalysisRunListOptions{}, errors.New("page size must be between 1 and 100")
		}

		options.PageSize = pageSize
	}

	return options, nil
}

func (app *application) createAIAnalysisRunHandler(w http.ResponseWriter, r *http.Request) {
	project, user, ok := app.projectForRequest(w, r)
	if !ok {
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

	analysisContext, err := app.aiAnalysisRuns.BuildProjectContext(r.Context(), project.ID, user.ID)
	if err != nil {
		app.serverError(w, err)
		return
	}

	contextSnapshot, err := json.Marshal(analysisContext)
	if err != nil {
		app.serverError(w, fmt.Errorf("encode AI analysis context: %w", err))
		return
	}

	run, err := app.aiAnalysisRuns.Create(r.Context(), models.AIAnalysisRunCreateInput{
		ProjectID:         project.ID,
		RequestedByUserID: user.ID,
		ContextSnapshot:   contextSnapshot,
	})
	if err != nil {
		app.serverError(w, err)
		return
	}

	runID := run.ID
	app.recordActivity(r.Context(), models.ActivityCreateInput{
		ProjectID:  project.ID,
		UserID:     &user.ID,
		Action:     "requested",
		EntityType: "ai_analysis_run",
		EntityID:   &runID,
	})

	if err := app.writeJSON(w, http.StatusAccepted, newAIAnalysisRunResponse(run)); err != nil {
		app.errorLog.Printf("encode AI analysis run response: %v", err)
	}
}

func (app *application) listAIAnalysisRunsHandler(w http.ResponseWriter, r *http.Request) {
	project, user, ok := app.projectForRequest(w, r)
	if !ok {
		return
	}

	options, err := parseAIAnalysisRunListOptions(r)
	if err != nil {
		app.clientError(w, http.StatusBadRequest, err.Error())
		return
	}

	runList, err := app.aiAnalysisRuns.ListByProjectForUser(r.Context(), project.ID, user.ID, options)
	if err != nil {
		app.serverError(w, err)
		return
	}

	runs := make([]aiAnalysisRunResponse, 0, len(runList.Runs))
	for _, run := range runList.Runs {
		runs = append(runs, newAIAnalysisRunResponse(run))
	}

	response := aiAnalysisRunListResponse{
		Runs: runs,
		Pagination: paginationResponse{
			Page:       options.Page,
			PageSize:   options.PageSize,
			TotalItems: runList.TotalItems,
			TotalPages: (runList.TotalItems + options.PageSize - 1) / options.PageSize,
		},
	}

	if err := app.writeJSON(w, http.StatusOK, response); err != nil {
		app.errorLog.Printf("encode AI analysis run list response: %v", err)
	}
}

func (app *application) getAIAnalysisRunHandler(w http.ResponseWriter, r *http.Request) {
	project, user, ok := app.projectForRequest(w, r)
	if !ok {
		return
	}

	runID, err := strconv.ParseInt(r.PathValue("runID"), 10, 64)
	if err != nil || runID < 1 {
		app.clientError(w, http.StatusBadRequest, "AI analysis run id must be a positive integer")
		return
	}

	run, err := app.aiAnalysisRuns.GetByIDForUser(r.Context(), runID, user.ID)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w, "AI analysis run not found")
			return
		}

		app.serverError(w, err)
		return
	}

	if run.ProjectID != project.ID {
		app.notFound(w, "AI analysis run not found")
		return
	}

	if err := app.writeJSON(w, http.StatusOK, newAIAnalysisRunResponse(run)); err != nil {
		app.errorLog.Printf("encode AI analysis run response: %v", err)
	}
}
