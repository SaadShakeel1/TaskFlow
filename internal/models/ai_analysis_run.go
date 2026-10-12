package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type AIAnalysisRunStatus string

const (
	AIAnalysisRunStatusQueued    AIAnalysisRunStatus = "queued"
	AIAnalysisRunStatusRunning   AIAnalysisRunStatus = "running"
	AIAnalysisRunStatusSucceeded AIAnalysisRunStatus = "succeeded"
	AIAnalysisRunStatusFailed    AIAnalysisRunStatus = "failed"
	AIAnalysisRunStatusCancelled AIAnalysisRunStatus = "cancelled"
)

type AIInsightSeverity string

const (
	AIInsightSeverityLow      AIInsightSeverity = "low"
	AIInsightSeverityMedium   AIInsightSeverity = "medium"
	AIInsightSeverityHigh     AIInsightSeverity = "high"
	AIInsightSeverityCritical AIInsightSeverity = "critical"
)

type AIInsightCategory string

const (
	AIInsightCategoryBlockedTask          AIInsightCategory = "blocked_task"
	AIInsightCategoryOverdueTask          AIInsightCategory = "overdue_task"
	AIInsightCategoryDependencyBottleneck AIInsightCategory = "dependency_bottleneck"
	AIInsightCategoryUnassignedWork       AIInsightCategory = "unassigned_work"
	AIInsightCategoryInactivity           AIInsightCategory = "inactivity"
	AIInsightCategoryScopeRisk            AIInsightCategory = "scope_risk"
)

type AIAnalysisResult struct {
	Summary  string              `json:"summary"`
	Findings []AIAnalysisFinding `json:"findings"`
}

type AIAnalysisFinding struct {
	Severity       AIInsightSeverity `json:"severity"`
	Category       AIInsightCategory `json:"category"`
	Title          string            `json:"title"`
	Summary        string            `json:"summary"`
	Evidence       AIInsightEvidence `json:"evidence"`
	Recommendation string            `json:"recommendation"`
}

type AIInsightEvidence struct {
	TaskIDs     []int64 `json:"taskIDs"`
	ActivityIDs []int64 `json:"activityIDs"`
}

type AIAnalysisRun struct {
	ID                   int64
	ProjectID            int64
	RequestedByUserID    int64
	Status               AIAnalysisRunStatus
	ContextSchemaVersion int
	ContextSnapshot      json.RawMessage
	Result               json.RawMessage
	Provider             *string
	Model                *string
	AttemptCount         int
	ErrorMessage         *string
	StartedAt            *time.Time
	CompletedAt          *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type AIAnalysisRunCreateInput struct {
	ProjectID         int64
	RequestedByUserID int64
	ContextSnapshot   json.RawMessage
}

type AIAnalysisRunListOptions struct {
	Page     int
	PageSize int
}

type AIAnalysisRunList struct {
	Runs       []AIAnalysisRun
	TotalItems int
}

type AIAnalysisRunModel struct {
	DB *sql.DB
}

func (m *AIAnalysisRunModel) Create(
	ctx context.Context,
	input AIAnalysisRunCreateInput,
) (AIAnalysisRun, error) {
	if input.ProjectID < 1 {
		return AIAnalysisRun{}, fmt.Errorf("project ID must be positive")
	}

	if input.RequestedByUserID < 1 {
		return AIAnalysisRun{}, fmt.Errorf("requesting user ID must be positive")
	}

	contextSnapshot := input.ContextSnapshot
	if len(contextSnapshot) == 0 {
		contextSnapshot = json.RawMessage(`{}`)
	}

	if !isJSONObject(contextSnapshot) {
		return AIAnalysisRun{}, fmt.Errorf("context snapshot must be a JSON object")
	}

	const query = `
		INSERT INTO ai_analysis_runs (
			project_id,
			requested_by_user_id,
			context_snapshot
		)
		SELECT $1, $2, $3
		WHERE EXISTS (
			SELECT 1
			FROM projects
			WHERE projects.id = $1
				AND (
					projects.user_id = $2
					OR EXISTS (
						SELECT 1
						FROM project_members
						WHERE project_members.project_id = projects.id
							AND project_members.user_id = $2
					)
				)
		)
		RETURNING
			id,
			project_id,
			requested_by_user_id,
			status,
			context_schema_version,
			context_snapshot,
			result,
			provider,
			model,
			attempt_count,
			error_message,
			started_at,
			completed_at,
			created_at,
			updated_at
	`

	run, err := scanAIAnalysisRun(
		m.DB.QueryRowContext(
			ctx,
			query,
			input.ProjectID,
			input.RequestedByUserID,
			contextSnapshot,
		),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AIAnalysisRun{}, ErrNoRecord
		}

		return AIAnalysisRun{}, fmt.Errorf("create AI analysis run: %w", err)
	}

	return run, nil
}

func (m *AIAnalysisRunModel) GetByIDForUser(
	ctx context.Context,
	runID int64,
	userID int64,
) (AIAnalysisRun, error) {
	if runID < 1 {
		return AIAnalysisRun{}, fmt.Errorf("AI analysis run ID must be positive")
	}

	if userID < 1 {
		return AIAnalysisRun{}, fmt.Errorf("user ID must be positive")
	}

	const query = `
		SELECT
			id,
			project_id,
			requested_by_user_id,
			status,
			context_schema_version,
			context_snapshot,
			result,
			provider,
			model,
			attempt_count,
			error_message,
			started_at,
			completed_at,
			created_at,
			updated_at
		FROM ai_analysis_runs
		WHERE id = $1
			AND EXISTS (
				SELECT 1
				FROM projects
				WHERE projects.id = ai_analysis_runs.project_id
					AND (
						projects.user_id = $2
						OR EXISTS (
							SELECT 1
							FROM project_members
							WHERE project_members.project_id = projects.id
								AND project_members.user_id = $2
						)
					)
			)
	`

	run, err := scanAIAnalysisRun(
		m.DB.QueryRowContext(ctx, query, runID, userID),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AIAnalysisRun{}, ErrNoRecord
		}

		return AIAnalysisRun{}, fmt.Errorf("get AI analysis run by ID for user: %w", err)
	}

	return run, nil
}

func (m *AIAnalysisRunModel) ClaimNext(
	ctx context.Context,
	provider string,
	model string,
) (AIAnalysisRun, error) {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)

	if provider == "" {
		return AIAnalysisRun{}, fmt.Errorf("AI provider is required")
	}

	if model == "" {
		return AIAnalysisRun{}, fmt.Errorf("AI model is required")
	}

	const query = `
		UPDATE ai_analysis_runs
		SET
			status = 'running',
			provider = $1,
			model = $2,
			attempt_count = attempt_count + 1,
			started_at = NOW(),
			completed_at = NULL,
			error_message = NULL,
			updated_at = NOW()
		WHERE id = (
			SELECT id
			FROM ai_analysis_runs
			WHERE status = 'queued'
			ORDER BY created_at ASC, id ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING
			id,
			project_id,
			requested_by_user_id,
			status,
			context_schema_version,
			context_snapshot,
			result,
			provider,
			model,
			attempt_count,
			error_message,
			started_at,
			completed_at,
			created_at,
			updated_at
	`

	run, err := scanAIAnalysisRun(
		m.DB.QueryRowContext(ctx, query, provider, model),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AIAnalysisRun{}, ErrNoRecord
		}

		return AIAnalysisRun{}, fmt.Errorf("claim next AI analysis run: %w", err)
	}

	return run, nil
}

func (m *AIAnalysisRunModel) MarkSucceeded(
	ctx context.Context,
	runID int64,
	result AIAnalysisResult,
) (AIAnalysisRun, error) {
	if runID < 1 {
		return AIAnalysisRun{}, fmt.Errorf("AI analysis run ID must be positive")
	}

	if err := validateAIAnalysisResult(result); err != nil {
		return AIAnalysisRun{}, err
	}

	encodedResult, err := json.Marshal(result)
	if err != nil {
		return AIAnalysisRun{}, fmt.Errorf("encode AI analysis result: %w", err)
	}

	const query = `
		UPDATE ai_analysis_runs
		SET
			status = 'succeeded',
			result = $2,
			error_message = NULL,
			completed_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
			AND status = 'running'
		RETURNING
			id,
			project_id,
			requested_by_user_id,
			status,
			context_schema_version,
			context_snapshot,
			result,
			provider,
			model,
			attempt_count,
			error_message,
			started_at,
			completed_at,
			created_at,
			updated_at
	`

	run, err := scanAIAnalysisRun(
		m.DB.QueryRowContext(ctx, query, runID, encodedResult),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AIAnalysisRun{}, ErrNoRecord
		}

		return AIAnalysisRun{}, fmt.Errorf("mark AI analysis run succeeded: %w", err)
	}

	return run, nil
}

func (m *AIAnalysisRunModel) MarkFailed(
	ctx context.Context,
	runID int64,
	failureMessage string,
) (AIAnalysisRun, error) {
	if runID < 1 {
		return AIAnalysisRun{}, fmt.Errorf("AI analysis run ID must be positive")
	}

	failureMessage = strings.TrimSpace(failureMessage)
	if failureMessage == "" {
		return AIAnalysisRun{}, fmt.Errorf("AI analysis failure message is required")
	}

	if len(failureMessage) > 2000 {
		return AIAnalysisRun{}, fmt.Errorf("AI analysis failure message must not exceed 2000 characters")
	}

	const query = `
		UPDATE ai_analysis_runs
		SET
			status = 'failed',
			result = NULL,
			error_message = $2,
			completed_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
			AND status = 'running'
		RETURNING
			id,
			project_id,
			requested_by_user_id,
			status,
			context_schema_version,
			context_snapshot,
			result,
			provider,
			model,
			attempt_count,
			error_message,
			started_at,
			completed_at,
			created_at,
			updated_at
	`

	run, err := scanAIAnalysisRun(
		m.DB.QueryRowContext(ctx, query, runID, failureMessage),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AIAnalysisRun{}, ErrNoRecord
		}

		return AIAnalysisRun{}, fmt.Errorf("mark AI analysis run failed: %w", err)
	}

	return run, nil
}

func (m *AIAnalysisRunModel) ListByProjectForUser(
	ctx context.Context,
	projectID int64,
	userID int64,
	options AIAnalysisRunListOptions,
) (AIAnalysisRunList, error) {
	if projectID < 1 || userID < 1 {
		return AIAnalysisRunList{}, fmt.Errorf("project and user IDs must be positive")
	}

	if options.Page < 1 {
		return AIAnalysisRunList{}, fmt.Errorf("page must be at least 1")
	}

	if options.PageSize < 1 || options.PageSize > 100 {
		return AIAnalysisRunList{}, fmt.Errorf("page size must be between 1 and 100")
	}

	const countQuery = `
		SELECT COUNT(*)
		FROM ai_analysis_runs AS run
		WHERE run.project_id = $1
			AND EXISTS (
				SELECT 1
				FROM projects
				WHERE id = run.project_id
					AND (
						user_id = $2
						OR EXISTS (
							SELECT 1
							FROM project_members
							WHERE project_id = run.project_id
								AND user_id = $2
						)
					)
			)
	`

	var totalItems int

	if err := m.DB.QueryRowContext(ctx, countQuery, projectID, userID).Scan(&totalItems); err != nil {
		return AIAnalysisRunList{}, fmt.Errorf("count AI analysis runs by project: %w", err)
	}

	offset := (options.Page - 1) * options.PageSize

	const query = `
		SELECT
			id,
			project_id,
			requested_by_user_id,
			status,
			context_schema_version,
			context_snapshot,
			result,
			provider,
			model,
			attempt_count,
			error_message,
			started_at,
			completed_at,
			created_at,
			updated_at
		FROM ai_analysis_runs AS run
		WHERE run.project_id = $1
			AND EXISTS (
				SELECT 1
				FROM projects
				WHERE id = run.project_id
					AND (
						user_id = $2
						OR EXISTS (
							SELECT 1
							FROM project_members
							WHERE project_id = run.project_id
								AND user_id = $2
						)
					)
			)
		ORDER BY run.created_at DESC, run.id DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := m.DB.QueryContext(ctx, query, projectID, userID, options.PageSize, offset)
	if err != nil {
		return AIAnalysisRunList{}, fmt.Errorf("list AI analysis runs by project: %w", err)
	}
	defer rows.Close()

	runs := make([]AIAnalysisRun, 0)

	for rows.Next() {
		run, err := scanAIAnalysisRun(rows)
		if err != nil {
			return AIAnalysisRunList{}, fmt.Errorf("scan AI analysis run: %w", err)
		}

		runs = append(runs, run)
	}

	if err := rows.Err(); err != nil {
		return AIAnalysisRunList{}, fmt.Errorf("iterate AI analysis runs: %w", err)
	}

	return AIAnalysisRunList{Runs: runs, TotalItems: totalItems}, nil
}

func validateAIAnalysisResult(result AIAnalysisResult) error {
	if summary := strings.TrimSpace(result.Summary); summary == "" {
		return fmt.Errorf("AI analysis result summary is required")
	} else if len(summary) > 3000 {
		return fmt.Errorf("AI analysis result summary must not exceed 3000 characters")
	}

	if len(result.Findings) > 50 {
		return fmt.Errorf("AI analysis result cannot contain more than 50 findings")
	}

	for index, finding := range result.Findings {
		if !isValidAIInsightSeverity(finding.Severity) {
			return fmt.Errorf("finding %d has invalid severity: %q", index, finding.Severity)
		}

		if !isValidAIInsightCategory(finding.Category) {
			return fmt.Errorf("finding %d has invalid category: %q", index, finding.Category)
		}

		if title := strings.TrimSpace(finding.Title); title == "" {
			return fmt.Errorf("finding %d title is required", index)
		} else if len(title) > 300 {
			return fmt.Errorf("finding %d title must not exceed 300 characters", index)
		}

		if summary := strings.TrimSpace(finding.Summary); summary == "" {
			return fmt.Errorf("finding %d summary is required", index)
		} else if len(summary) > 4000 {
			return fmt.Errorf("finding %d summary must not exceed 4000 characters", index)
		}

		if recommendation := strings.TrimSpace(finding.Recommendation); recommendation == "" {
			return fmt.Errorf("finding %d recommendation is required", index)
		} else if len(recommendation) > 2000 {
			return fmt.Errorf("finding %d recommendation must not exceed 2000 characters", index)
		}

		if len(finding.Evidence.TaskIDs) == 0 && len(finding.Evidence.ActivityIDs) == 0 {
			return fmt.Errorf("finding %d must include evidence", index)
		}

		if err := validateAIInsightIDs("task", finding.Evidence.TaskIDs); err != nil {
			return fmt.Errorf("finding %d: %w", index, err)
		}

		if err := validateAIInsightIDs("activity", finding.Evidence.ActivityIDs); err != nil {
			return fmt.Errorf("finding %d: %w", index, err)
		}
	}

	return nil
}

func isValidAIInsightSeverity(severity AIInsightSeverity) bool {
	switch severity {
	case AIInsightSeverityLow,
		AIInsightSeverityMedium,
		AIInsightSeverityHigh,
		AIInsightSeverityCritical:
		return true
	default:
		return false
	}
}

func isValidAIInsightCategory(category AIInsightCategory) bool {
	switch category {
	case AIInsightCategoryBlockedTask,
		AIInsightCategoryOverdueTask,
		AIInsightCategoryDependencyBottleneck,
		AIInsightCategoryUnassignedWork,
		AIInsightCategoryInactivity,
		AIInsightCategoryScopeRisk:
		return true
	default:
		return false
	}
}

func validateAIInsightIDs(kind string, ids []int64) error {
	seen := make(map[int64]struct{})

	for _, id := range ids {
		if id < 1 {
			return fmt.Errorf("%s evidence ID must be positive", kind)
		}

		if _, exists := seen[id]; exists {
			return fmt.Errorf("duplicate %s evidence ID: %d", kind, id)
		}

		seen[id] = struct{}{}
	}

	return nil
}

type aiAnalysisRunScanner interface {
	Scan(dest ...any) error
}

func scanAIAnalysisRun(scanner aiAnalysisRunScanner) (AIAnalysisRun, error) {
	var run AIAnalysisRun
	var contextSnapshot []byte
	var result []byte
	var provider sql.NullString
	var model sql.NullString
	var errorMessage sql.NullString
	var startedAt sql.NullTime
	var completedAt sql.NullTime

	err := scanner.Scan(
		&run.ID,
		&run.ProjectID,
		&run.RequestedByUserID,
		&run.Status,
		&run.ContextSchemaVersion,
		&contextSnapshot,
		&result,
		&provider,
		&model,
		&run.AttemptCount,
		&errorMessage,
		&startedAt,
		&completedAt,
		&run.CreatedAt,
		&run.UpdatedAt,
	)
	if err != nil {
		return AIAnalysisRun{}, err
	}

	run.ContextSnapshot = json.RawMessage(contextSnapshot)

	if result != nil {
		run.Result = json.RawMessage(result)
	}

	if provider.Valid {
		value := provider.String
		run.Provider = &value
	}

	if model.Valid {
		value := model.String
		run.Model = &value
	}

	if errorMessage.Valid {
		value := errorMessage.String
		run.ErrorMessage = &value
	}

	if startedAt.Valid {
		value := startedAt.Time
		run.StartedAt = &value
	}

	if completedAt.Valid {
		value := completedAt.Time
		run.CompletedAt = &value
	}

	return run, nil
}

func isJSONObject(value json.RawMessage) bool {
	var object map[string]json.RawMessage

	if err := json.Unmarshal(value, &object); err != nil {
		return false
	}

	return object != nil
}
