package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"testing"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

type runModelStub struct {
	run            models.AIAnalysisRun
	claimErr       error
	succeededRunID int64
	failedRunID    int64
	failureMessage string
}

func (s *runModelStub) ClaimNext(context.Context, string, string) (models.AIAnalysisRun, error) {
	return s.run, s.claimErr
}

func (s *runModelStub) MarkSucceeded(_ context.Context, runID int64, _ models.AIAnalysisResult) (models.AIAnalysisRun, error) {
	s.succeededRunID = runID
	return models.AIAnalysisRun{ID: runID}, nil
}

func (s *runModelStub) MarkFailed(_ context.Context, runID int64, message string) (models.AIAnalysisRun, error) {
	s.failedRunID = runID
	s.failureMessage = message
	return models.AIAnalysisRun{ID: runID}, nil
}

type analyzerStub struct {
	result models.AIAnalysisResult
	err    error
}

func (s analyzerStub) Analyze(context.Context, models.ProjectAnalysisContext) (models.AIAnalysisResult, error) {
	return s.result, s.err
}

func TestWorkerProcessNextMarksRunSucceeded(t *testing.T) {
	contextSnapshot, err := json.Marshal(models.ProjectAnalysisContext{SchemaVersion: 1})
	if err != nil {
		t.Fatal(err)
	}

	runs := &runModelStub{run: models.AIAnalysisRun{ID: 42, ContextSnapshot: contextSnapshot}}
	worker, err := NewWorker(
		runs,
		analyzerStub{result: models.AIAnalysisResult{Summary: "No material delivery risks found."}},
		log.New(io.Discard, "", 0),
		"groq",
		"test-model",
		1,
		1,
	)
	if err != nil {
		t.Fatalf("create worker: %v", err)
	}

	worker.ProcessNextForTest(context.Background())

	if runs.succeededRunID != 42 {
		t.Errorf("succeeded run ID = %d, want 42", runs.succeededRunID)
	}

	if runs.failedRunID != 0 {
		t.Errorf("failed run ID = %d, want 0", runs.failedRunID)
	}
}

func TestWorkerProcessNextMarksRunFailedWhenAnalysisFails(t *testing.T) {
	contextSnapshot, err := json.Marshal(models.ProjectAnalysisContext{SchemaVersion: 1})
	if err != nil {
		t.Fatal(err)
	}

	runs := &runModelStub{run: models.AIAnalysisRun{ID: 7, ContextSnapshot: contextSnapshot}}
	worker, err := NewWorker(
		runs,
		analyzerStub{err: errors.New("Groq unavailable")},
		log.New(io.Discard, "", 0),
		"groq",
		"test-model",
		1,
		1,
	)
	if err != nil {
		t.Fatalf("create worker: %v", err)
	}

	worker.ProcessNextForTest(context.Background())

	if runs.failedRunID != 7 {
		t.Errorf("failed run ID = %d, want 7", runs.failedRunID)
	}

	if runs.failureMessage != "AI analysis could not be completed" {
		t.Errorf("failure message = %q", runs.failureMessage)
	}
}
