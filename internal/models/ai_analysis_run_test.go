package models_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
	"github.com/SaadShakeel1/TaskFlow/internal/testutil"
)

func TestAIAnalysisRunModelLifecycle(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}
	projects := &models.ProjectModel{DB: db}
	runs := &models.AIAnalysisRunModel{DB: db}

	user, err := users.Create(ctx, "ai-run-owner@example.com", "AI Run Owner", "correct-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	project, err := projects.Create(ctx, user.ID, "AI run project", "Project for AI analysis runs", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	snapshot, err := json.Marshal(models.ProjectAnalysisContext{SchemaVersion: 1})
	if err != nil {
		t.Fatalf("encode snapshot: %v", err)
	}

	created, err := runs.Create(ctx, models.AIAnalysisRunCreateInput{
		ProjectID: project.ID, RequestedByUserID: user.ID, ContextSnapshot: snapshot,
	})
	if err != nil {
		t.Fatalf("create analysis run: %v", err)
	}

	if created.Status != models.AIAnalysisRunStatusQueued {
		t.Errorf("status = %q, want queued", created.Status)
	}

	claimed, err := runs.ClaimNext(ctx, "groq", "openai/gpt-oss-20b")
	if err != nil {
		t.Fatalf("claim analysis run: %v", err)
	}

	if claimed.ID != created.ID || claimed.Status != models.AIAnalysisRunStatusRunning {
		t.Errorf("claimed run = %#v", claimed)
	}

	completed, err := runs.MarkSucceeded(ctx, claimed.ID, models.AIAnalysisResult{
		Summary: "No material project risks found.",
	})
	if err != nil {
		t.Fatalf("mark analysis run succeeded: %v", err)
	}

	if completed.Status != models.AIAnalysisRunStatusSucceeded {
		t.Errorf("status = %q, want succeeded", completed.Status)
	}

	list, err := runs.ListByProjectForUser(ctx, project.ID, user.ID, models.AIAnalysisRunListOptions{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list analysis runs: %v", err)
	}

	if list.TotalItems != 1 || len(list.Runs) != 1 || list.Runs[0].ID != created.ID {
		t.Errorf("list = %#v", list)
	}
}

func TestAIAnalysisRunModelRejectsOtherProjectMember(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}
	projects := &models.ProjectModel{DB: db}
	runs := &models.AIAnalysisRunModel{DB: db}

	owner, err := users.Create(ctx, "ai-owner@example.com", "AI Owner", "correct-password")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	outsider, err := users.Create(ctx, "ai-outsider@example.com", "AI Outsider", "correct-password")
	if err != nil {
		t.Fatalf("create outsider: %v", err)
	}
	project, err := projects.Create(ctx, owner.ID, "Private AI project", "Private", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if _, err := runs.Create(ctx, models.AIAnalysisRunCreateInput{ProjectID: project.ID, RequestedByUserID: outsider.ID, ContextSnapshot: json.RawMessage(`{}`)}); !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("create as outsider error = %v, want ErrNoRecord", err)
	}
}
