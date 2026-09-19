package models_test

import (
	"context"
	"errors"
	"testing"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
	"github.com/SaadShakeel1/TaskFlow/internal/testutil"
)

func TestProjectMemberModelCreateListUpdateAndDelete(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}
	owner, err := users.Create(ctx, "owner@example.com", "Owner", "correct-password")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	editor, err := users.Create(ctx, "editor@example.com", "Editor", "correct-password")
	if err != nil {
		t.Fatalf("create editor: %v", err)
	}

	viewer, err := users.Create(ctx, "viewer@example.com", "Viewer", "correct-password")
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}

	projects := &models.ProjectModel{DB: db}
	project, err := projects.Create(ctx, owner.ID, "Shared project", "A project with members", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	members := &models.ProjectMemberModel{DB: db}
	if _, err := members.Create(ctx, project.ID, editor.ID, models.ProjectMemberRoleEditor); err != nil {
		t.Fatalf("add editor: %v", err)
	}

	if _, err := members.Create(ctx, project.ID, viewer.ID, models.ProjectMemberRoleViewer); err != nil {
		t.Fatalf("add viewer: %v", err)
	}

	if _, err := members.Create(ctx, project.ID, editor.ID, models.ProjectMemberRoleEditor); !errors.Is(err, models.ErrDuplicateProjectMember) {
		t.Errorf("duplicate member error = %v, want models.ErrDuplicateProjectMember", err)
	}

	listed, err := members.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("list project members: %v", err)
	}

	if len(listed) != 3 {
		t.Fatalf("member count = %d, want 3", len(listed))
	}

	rolesByUserID := make(map[int64]models.ProjectMemberRole)
	for _, member := range listed {
		rolesByUserID[member.UserID] = member.Role
	}

	if got := rolesByUserID[owner.ID]; got != models.ProjectMemberRoleOwner {
		t.Errorf("owner role = %q, want %q", got, models.ProjectMemberRoleOwner)
	}
	if got := rolesByUserID[editor.ID]; got != models.ProjectMemberRoleEditor {
		t.Errorf("editor role = %q, want %q", got, models.ProjectMemberRoleEditor)
	}
	if got := rolesByUserID[viewer.ID]; got != models.ProjectMemberRoleViewer {
		t.Errorf("viewer role = %q, want %q", got, models.ProjectMemberRoleViewer)
	}

	updated, err := members.UpdateRole(ctx, project.ID, viewer.ID, models.ProjectMemberRoleEditor)
	if err != nil {
		t.Fatalf("promote viewer: %v", err)
	}
	if updated.Role != models.ProjectMemberRoleEditor {
		t.Errorf("updated role = %q, want %q", updated.Role, models.ProjectMemberRoleEditor)
	}

	if err := members.Delete(ctx, project.ID, editor.ID); err != nil {
		t.Fatalf("remove editor: %v", err)
	}

	if err := members.Delete(ctx, project.ID, editor.ID); !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("delete missing member error = %v, want models.ErrNoRecord", err)
	}
}

func TestProjectModelAppliesMemberPermissions(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}
	owner, err := users.Create(ctx, "owner@example.com", "Owner", "correct-password")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	editor, err := users.Create(ctx, "editor@example.com", "Editor", "correct-password")
	if err != nil {
		t.Fatalf("create editor: %v", err)
	}

	viewer, err := users.Create(ctx, "viewer@example.com", "Viewer", "correct-password")
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}

	outsider, err := users.Create(ctx, "outsider@example.com", "Outsider", "correct-password")
	if err != nil {
		t.Fatalf("create outsider: %v", err)
	}

	projects := &models.ProjectModel{DB: db}
	project, err := projects.Create(ctx, owner.ID, "Shared project", "A project with members", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	members := &models.ProjectMemberModel{DB: db}
	if _, err := members.Create(ctx, project.ID, editor.ID, models.ProjectMemberRoleEditor); err != nil {
		t.Fatalf("add editor: %v", err)
	}
	if _, err := members.Create(ctx, project.ID, viewer.ID, models.ProjectMemberRoleViewer); err != nil {
		t.Fatalf("add viewer: %v", err)
	}

	if _, err := projects.GetByIDForUser(ctx, project.ID, editor.ID); err != nil {
		t.Errorf("editor get project: %v", err)
	}
	if _, err := projects.GetByIDForUser(ctx, project.ID, viewer.ID); err != nil {
		t.Errorf("viewer get project: %v", err)
	}
	if _, err := projects.GetByIDForUser(ctx, project.ID, outsider.ID); !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("outsider get project error = %v, want models.ErrNoRecord", err)
	}

	options := models.ProjectListOptions{
		Page:     1,
		PageSize: 20,
		Sort:     models.ProjectSortUpdatedAt,
		Order:    models.SortOrderDescending,
	}
	list, err := projects.ListByUser(ctx, viewer.ID, options)
	if err != nil {
		t.Fatalf("list viewer projects: %v", err)
	}
	if list.TotalItems != 1 || len(list.Projects) != 1 || list.Projects[0].ID != project.ID {
		t.Errorf("viewer project list = %#v, want shared project", list)
	}

	updated, err := projects.UpdateByIDForUser(ctx, project.ID, editor.ID, models.ProjectUpdate{
		Name:        "Updated by editor",
		Description: project.Description,
		Status:      project.Status,
		TargetDate:  project.TargetDate,
	})
	if err != nil {
		t.Fatalf("editor update project: %v", err)
	}
	if updated.Name != "Updated by editor" {
		t.Errorf("updated name = %q, want %q", updated.Name, "Updated by editor")
	}

	_, err = projects.UpdateByIDForUser(ctx, project.ID, viewer.ID, models.ProjectUpdate{
		Name:        "Viewer must not update",
		Description: project.Description,
		Status:      project.Status,
		TargetDate:  project.TargetDate,
	})
	if !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("viewer update error = %v, want models.ErrNoRecord", err)
	}

	if err := projects.DeleteByIDForUser(ctx, project.ID, editor.ID); !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("editor delete error = %v, want models.ErrNoRecord", err)
	}
}
