package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

const (
	groqChatCompletionsURL = "https://api.groq.com/openai/v1/chat/completions"
	defaultGroqTimeout     = 60 * time.Second
	maxGroqErrorBytes      = 4 << 10
)

type GroqAnalyzerConfig struct {
	APIKey string
	Model  string
	Client *http.Client
}

type GroqAnalyzer struct {
	apiKey string
	model  string
	client *http.Client
}

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqResponseFormat struct {
	Type       string         `json:"type"`
	JSONSchema groqJSONSchema `json:"json_schema"`
}

type groqJSONSchema struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type groqChatCompletionRequest struct {
	Model          string             `json:"model"`
	Messages       []groqMessage      `json:"messages"`
	Temperature    float64            `json:"temperature"`
	ResponseFormat groqResponseFormat `json:"response_format"`
}

type groqChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func NewGroqAnalyzer(config GroqAnalyzerConfig) (*GroqAnalyzer, error) {
	apiKey := strings.TrimSpace(config.APIKey)
	if apiKey == "" {
		return nil, errors.New("Groq API key is required")
	}

	model := strings.TrimSpace(config.Model)
	if model == "" {
		return nil, errors.New("Groq model is required")
	}

	client := config.Client
	if client == nil {
		client = &http.Client{Timeout: defaultGroqTimeout}
	}

	return &GroqAnalyzer{apiKey: apiKey, model: model, client: client}, nil
}

func (a *GroqAnalyzer) Analyze(ctx context.Context, analysisContext models.ProjectAnalysisContext) (models.AIAnalysisResult, error) {
	contextJSON, err := json.Marshal(analysisContext)
	if err != nil {
		return models.AIAnalysisResult{}, fmt.Errorf("encode project analysis context: %w", err)
	}

	requestBody, err := json.Marshal(groqChatCompletionRequest{
		Model: a.model,
		Messages: []groqMessage{
			{
				Role: "system",
				Content: `You are TaskFlow's project-risk analyst. Use only the supplied project snapshot as evidence.

Identify meaningful delivery risks, blockers, overdue work, dependency bottlenecks, unassigned work, inactivity, or scope risk.

Do not invent data. Every finding must cite one or more relevant task IDs or activity IDs. Return an empty findings array when the snapshot has no meaningful risk.`,
			},
			{Role: "user", Content: string(contextJSON)},
		},
		Temperature: 0.2,
		ResponseFormat: groqResponseFormat{
			Type: "json_schema",
			JSONSchema: groqJSONSchema{
				Name:   "taskflow_project_analysis",
				Strict: true,
				Schema: projectAnalysisResultSchema(),
			},
		},
	})
	if err != nil {
		return models.AIAnalysisResult{}, fmt.Errorf("encode Groq request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, groqChatCompletionsURL, bytes.NewReader(requestBody))
	if err != nil {
		return models.AIAnalysisResult{}, fmt.Errorf("create Groq request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+a.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := a.client.Do(request)
	if err != nil {
		return models.AIAnalysisResult{}, fmt.Errorf("call Groq: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxGroqErrorBytes))
		if readErr != nil {
			return models.AIAnalysisResult{}, fmt.Errorf("Groq returned HTTP %d", response.StatusCode)
		}

		message := strings.TrimSpace(string(body))
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}

		return models.AIAnalysisResult{}, fmt.Errorf("Groq returned HTTP %d: %s", response.StatusCode, message)
	}

	var completion groqChatCompletionResponse
	if err := json.NewDecoder(response.Body).Decode(&completion); err != nil {
		return models.AIAnalysisResult{}, fmt.Errorf("decode Groq response: %w", err)
	}

	if len(completion.Choices) != 1 {
		return models.AIAnalysisResult{}, errors.New("Groq response must contain exactly one choice")
	}

	content := strings.TrimSpace(completion.Choices[0].Message.Content)
	if content == "" {
		return models.AIAnalysisResult{}, errors.New("Groq response content is empty")
	}

	var result models.AIAnalysisResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return models.AIAnalysisResult{}, fmt.Errorf("decode Groq analysis result: %w", err)
	}

	return result, nil
}

func projectAnalysisResultSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"summary": map[string]any{"type": "string"},
			"findings": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"severity": map[string]any{"type": "string", "enum": []string{"low", "medium", "high", "critical"}},
						"category": map[string]any{"type": "string", "enum": []string{"blocked_task", "overdue_task", "dependency_bottleneck", "unassigned_work", "inactivity", "scope_risk"}},
						"title":    map[string]any{"type": "string"},
						"summary":  map[string]any{"type": "string"},
						"evidence": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"taskIDs":     map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
								"activityIDs": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
							},
							"required":             []string{"taskIDs", "activityIDs"},
							"additionalProperties": false,
						},
						"recommendation": map[string]any{"type": "string"},
					},
					"required":             []string{"severity", "category", "title", "summary", "evidence", "recommendation"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"summary", "findings"},
		"additionalProperties": false,
	}
}
