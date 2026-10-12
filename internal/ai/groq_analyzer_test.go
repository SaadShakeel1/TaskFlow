package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
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
