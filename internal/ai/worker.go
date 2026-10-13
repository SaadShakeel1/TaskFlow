package ai

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

type Analyzer interface {
	Analyze(
		ctx context.Context,
		analysisContext models.ProjectAnalysisContext,
	) (models.AIAnalysisResult, error)
}

type RunModel interface {
	ClaimNext(
		ctx context.Context,
		provider string,
		model string,
	) (models.AIAnalysisRun, error)

	MarkSucceeded(
		ctx context.Context,
		runID int64,
		result models.AIAnalysisResult,
	) (models.AIAnalysisRun, error)

	MarkFailed(
		ctx context.Context,
		runID int64,
		failureMessage string,
	) (models.AIAnalysisRun, error)
}

type Worker struct {
	runs           RunModel
	analyzer       Analyzer
	logger         *log.Logger
	provider       string
	model          string
	pollInterval   time.Duration
	processTimeout time.Duration
}

func NewWorker(
	runs RunModel,
	analyzer Analyzer,
	logger *log.Logger,
	provider string,
	model string,
	pollInterval time.Duration,
	processTimeout time.Duration,
) (*Worker, error) {
	if runs == nil {
		return nil, errors.New("AI analysis run model is required")
	}

	if analyzer == nil {
		return nil, errors.New("AI analyzer is required")
	}

	if logger == nil {
		return nil, errors.New("logger is required")
	}

	provider = strings.TrimSpace(provider)
	if provider == "" {
		return nil, errors.New("AI provider is required")
	}

	model = strings.TrimSpace(model)
	if model == "" {
		return nil, errors.New("AI model is required")
	}

	if pollInterval <= 0 {
		return nil, errors.New("AI worker poll interval must be positive")
	}

	if processTimeout <= 0 {
		return nil, errors.New("AI worker process timeout must be positive")
	}

	return &Worker{
		runs:           runs,
		analyzer:       analyzer,
		logger:         logger,
		provider:       provider,
		model:          model,
		pollInterval:   pollInterval,
		processTimeout: processTimeout,
	}, nil
}

func (w *Worker) Run(ctx context.Context) {
	w.processNext(ctx)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			w.processNext(ctx)
		}
	}
}

func (w *Worker) processNext(parentContext context.Context) {
	ctx, cancel := context.WithTimeout(parentContext, w.processTimeout)
	defer cancel()

	run, err := w.runs.ClaimNext(ctx, w.provider, w.model)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			return
		}

		w.logger.Printf("claim AI analysis run: %v", err)
		return
	}

	var analysisContext models.ProjectAnalysisContext
	if err := json.Unmarshal(run.ContextSnapshot, &analysisContext); err != nil {
		w.failRun(ctx, run.ID, "stored analysis context is invalid")
		return
	}

	if analysisContext.SchemaVersion != 1 {
		w.failRun(ctx, run.ID, "stored analysis context version is unsupported")
		return
	}

	result, err := w.analyzer.Analyze(ctx, analysisContext)
	if err != nil {
		w.failRun(ctx, run.ID, "AI analysis could not be completed")
		w.logger.Printf("analyze AI run %d: %v", run.ID, err)
		return
	}

	if _, err := w.runs.MarkSucceeded(ctx, run.ID, result); err != nil {
		w.logger.Printf("mark AI run %d succeeded: %v", run.ID, err)
	}
}

func (w *Worker) failRun(
	ctx context.Context,
	runID int64,
	failureMessage string,
) {
	if _, err := w.runs.MarkFailed(ctx, runID, failureMessage); err != nil {
		w.logger.Printf("mark AI run %d failed: %v", runID, err)
	}
}

func (w *Worker) ProcessNextForTest(ctx context.Context) {
	w.processNext(ctx)
}
