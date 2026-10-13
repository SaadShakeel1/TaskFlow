package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestGroqAnalyzerAnalyze(t *testing.T) {
	resultJSON, err := json.Marshal(models.AIAnalysisResult{
		Summary: "One blocked task needs attention.",
		Findings: []models.AIAnalysisFinding{{
			Severity:       models.AIInsightSeverityHigh,
			Category:       models.AIInsightCategoryBlockedTask,
			Title:          "Blocked task",
			Summary:        "Task 12 is blocked.",
			Evidence:       models.AIInsightEvidence{TaskIDs: []int64{12}},
			Recommendation: "Resolve the blocker.",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	completionJSON, err := json.Marshal(map[string]any{
		"choices": []any{map[string]any{
			"message": map[string]any{"content": string(resultJSON)},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	analyzer, err := NewGroqAnalyzer(GroqAnalyzerConfig{
		APIKey: "test-key",
		Model:  "openai/gpt-oss-20b",
		Client: &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			if request.Header.Get("Authorization") != "Bearer test-key" {
				t.Errorf("authorization = %q", request.Header.Get("Authorization"))
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(string(completionJSON))),
				Header:     make(http.Header),
			}, nil
		})},
	})
	if err != nil {
		t.Fatalf("create analyzer: %v", err)
	}

	result, err := analyzer.Analyze(context.Background(), models.ProjectAnalysisContext{SchemaVersion: 1})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	if result.Summary != "One blocked task needs attention." {
		t.Errorf("summary = %q", result.Summary)
	}

	if len(result.Findings) != 1 || result.Findings[0].Evidence.TaskIDs[0] != 12 {
		t.Errorf("unexpected findings: %#v", result.Findings)
	}
}

func TestGroqAnalyzerLive(t *testing.T) {
	if os.Getenv("RUN_GROQ_LIVE_TEST") != "1" {
		t.Skip("set RUN_GROQ_LIVE_TEST=1 to call Groq")
	}

	apiKey := strings.TrimSpace(os.Getenv("GROQ_API_KEY"))
	if apiKey == "" {
		t.Fatal("GROQ_API_KEY must be set for the live test")
	}

	model := strings.TrimSpace(os.Getenv("GROQ_MODEL"))
	if model == "" {
		model = "openai/gpt-oss-20b"
	}

	analyzer, err := NewGroqAnalyzer(GroqAnalyzerConfig{
		APIKey: apiKey,
		Model:  model,
	})
	if err != nil {
		t.Fatalf("create Groq analyzer: %v", err)
	}

	result, err := analyzer.Analyze(context.Background(), models.ProjectAnalysisContext{
		SchemaVersion: 1,
		Project: models.ProjectAnalysisProject{
			ID:   1,
			Name: "Synthetic live-test project",
		},
		Tasks: []models.ProjectAnalysisTask{{
			ID:            10,
			Title:         "Blocked deployment",
			Status:        models.TaskStatusBlocked,
			BlockedReason: pointerTo("Waiting for production credentials"),
			Priority:      models.TaskPriorityHigh,
		}},
	})
	if err != nil {
		t.Fatalf("analyze with Groq: %v", err)
	}

	if strings.TrimSpace(result.Summary) == "" {
		t.Fatal("Groq result summary is empty")
	}
}

func pointerTo(value string) *string {
	return &value
}
