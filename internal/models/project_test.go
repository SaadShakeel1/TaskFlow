package models_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
	"github.com/SaadShakeel1/TaskFlow/internal/testutil"
)

func TestProjectModelCreate(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)

	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}
	user, err := users.Create(ctx, "saad@example.com", "Saad", "correct-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	projects := &models.ProjectModel{DB: db}
	got, err := projects.Create(
		ctx,
		user.ID,
		"Launch TaskFlow",
		"Plan the first public release",
		nil,
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if got.ID < 1 {
		t.Errorf("ID = %d, want a positive value", got.ID)
	}

	if got.UserID != user.ID {
		t.Errorf("UserID = %d, want %d", got.UserID, user.ID)
	}

	if got.Name != "Launch TaskFlow" {
		t.Errorf("Name = %q, want %q", got.Name, "Launch TaskFlow")
	}

	if got.Description != "Plan the first public release" {
		t.Errorf("Description = %q, want %q", got.Description, "Plan the first public release")
	}

	if got.Status != models.ProjectStatusActive {
		t.Errorf("Status = %q, want %q", got.Status, models.ProjectStatusActive)
	}

	if got.TargetDate != nil {
		t.Errorf("TargetDate = %v, want nil", got.TargetDate)
	}

	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero, want a PostgreSQL-generated timestamp")
	}

	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt is zero, want a PostgreSQL-generated timestamp")
	}
}

func TestProjectModelCreateWithTargetDate(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)

	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	users := &models.UserModel{DB: db}
	user, err := users.Create(ctx, "saad@example.com", "Saad", "correct-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	targetDate := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)

	projects := &models.ProjectModel{DB: db}
	got, err := projects.Create(ctx, user.ID, "Launch TaskFlow", "Plan the first public release", &targetDate)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if got.TargetDate == nil {
		t.Fatal("TargetDate = nil, want the stored target date")
	}

	if got.TargetDate.Format(time.DateOnly) != targetDate.Format(time.DateOnly) {
		t.Errorf("TargetDate = %s, want %s", got.TargetDate.Format(time.DateOnly), targetDate.Format(time.DateOnly))
	}
}

func TestProjectModelGetByIDForUser(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)

	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	userModel := &models.UserModel{DB: db}

	createdUser, err := userModel.Create(ctx, "test@example.com", "Example", "correct-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	projectModel := &models.ProjectModel{DB: db}

	createdProject, err := projectModel.Create(ctx, createdUser.ID, "Test Project", "Test project description", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	got, err := projectModel.GetByIDForUser(ctx, createdProject.ID, createdUser.ID)
	if err != nil {
		t.Fatalf("get project by ID for user: %v", err)
	}

	if got.ID != createdProject.ID {
		t.Errorf("ID = %d, want %d", got.ID, createdProject.ID)
	}

	if got.UserID != createdUser.ID {
		t.Errorf("UserID = %d, want %d", got.UserID, createdUser.ID)
	}

	if got.Name != createdProject.Name {
		t.Errorf("Name = %q, want %q", got.Name, createdProject.Name)
	}

	if got.Description != createdProject.Description {
		t.Errorf("Description = %q, want %q", got.Description, createdProject.Description)
	}

	if got.Status != createdProject.Status {
		t.Errorf("Status = %q, want %q", got.Status, createdProject.Status)
	}

	if got.TargetDate != nil {
		t.Errorf("TargetDate = %v, want nil", got.TargetDate)
	}

	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}

	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt is zero")
	}
}

func TestProjectModelGetByIDForUserRejectsOtherUser(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)

	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	userModel := &models.UserModel{DB: db}

	owner, err := userModel.Create(ctx, "owner@example.com", "Owner", "correct-password")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	projectModel := &models.ProjectModel{DB: db}

	project, err := projectModel.Create(ctx, owner.ID, "Private Project", "Only the owner can read this project", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	otherUser, err := userModel.Create(ctx, "other@example.com", "Other User", "correct-password")
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}

	_, err = projectModel.GetByIDForUser(ctx, project.ID, otherUser.ID)
	if !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("error = %v, want models.ErrNoRecord", err)
	}
}

func TestProjectModelListByUser(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)

	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	userModel := &models.UserModel{DB: db}

	owner, err := userModel.Create(ctx, "owner@example.com", "Owner", "correct-password")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	otherUser, err := userModel.Create(ctx, "other@example.com", "Other User", "correct-password")
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}

	projectModel := &models.ProjectModel{DB: db}

	_, err = projectModel.Create(ctx, owner.ID, "First Owner Project", "First project", nil)
	if err != nil {
		t.Fatalf("create first owner project: %v", err)
	}

	_, err = projectModel.Create(ctx, owner.ID, "Second Owner Project", "Second project", nil)
	if err != nil {
		t.Fatalf("create second owner project: %v", err)
	}

	_, err = projectModel.Create(ctx, otherUser.ID, "Other User Project", "This project must not appear", nil)
	if err != nil {
		t.Fatalf("create other user's project: %v", err)
	}

	list, err := projectModel.ListByUser(ctx, owner.ID, models.ProjectListOptions{
		Page:     1,
		PageSize: 20,
		Sort:     models.ProjectSortUpdatedAt,
		Order:    models.SortOrderDescending,
	})
	if err != nil {
		t.Fatalf("list projects by user: %v", err)
	}

	projects := list.Projects

	if list.TotalItems != 2 {
		t.Errorf("TotalItems = %d, want 2", list.TotalItems)
	}

	if len(projects) != 2 {
		t.Fatalf("project count = %d, want 2", len(projects))
	}

	gotNames := make(map[string]bool)

	for _, project := range projects {
		if project.UserID != owner.ID {
			t.Errorf("UserID = %d, want %d", project.UserID, owner.ID)
		}

		gotNames[project.Name] = true
	}

	for _, want := range []string{"First Owner Project", "Second Owner Project"} {
		if !gotNames[want] {
			t.Errorf("missing project %q", want)
		}
	}

	if gotNames["Other User Project"] {
		t.Error("returned a project owned by another user")
	}
}

func TestProjectModelDeleteByIDForUser(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	userModel := &models.UserModel{DB: db}
	user, err := userModel.Create(ctx, "owner@example.com", "Owner", "correct-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	projectModel := &models.ProjectModel{DB: db}
	project, err := projectModel.Create(ctx, user.ID, "Project To Delete", "This project will be deleted", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	err = projectModel.DeleteByIDForUser(ctx, project.ID, user.ID)
	if err != nil {
		t.Fatalf("delete project: %v", err)
	}

	_, err = projectModel.GetByIDForUser(ctx, project.ID, user.ID)
	if !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("error = %v, want models.ErrNoRecord", err)
	}
}

func TestProjectModelUpdateByIDForUser(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	userModel := &models.UserModel{DB: db}
	user, err := userModel.Create(ctx, "owner@example.com", "Owner", "correct-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	projectModel := &models.ProjectModel{DB: db}
	created, err := projectModel.Create(ctx, user.ID, "Original Project", "Original description", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	targetDate := time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)

	update := models.ProjectUpdate{
		Name:        "Updated Project",
		Description: "Updated description",
		Status:      models.ProjectStatusCompleted,
		TargetDate:  &targetDate,
	}

	got, err := projectModel.UpdateByIDForUser(ctx, created.ID, user.ID, update)
	if err != nil {
		t.Fatalf("update project: %v", err)
	}

	if got.ID != created.ID {
		t.Errorf("ID = %d, want %d", got.ID, created.ID)
	}

	if got.UserID != user.ID {
		t.Errorf("UserID = %d, want %d", got.UserID, user.ID)
	}

	if got.Name != update.Name {
		t.Errorf("Name = %q, want %q", got.Name, update.Name)
	}

	if got.Description != update.Description {
		t.Errorf("Description = %q, want %q", got.Description, update.Description)
	}

	if got.Status != update.Status {
		t.Errorf("Status = %q, want %q", got.Status, update.Status)
	}

	if got.TargetDate == nil {
		t.Fatal("TargetDate = nil, want updated target date")
	}

	if got.TargetDate.Format(time.DateOnly) != targetDate.Format(time.DateOnly) {
		t.Errorf("TargetDate = %s, want %s",
			got.TargetDate.Format(time.DateOnly),
			targetDate.Format(time.DateOnly),
		)
	}

	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}

	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt is zero")
	}
}

func TestProjectModelMutationsRejectOtherUser(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() { testutil.ResetDatabase(t, db) })

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	userModel := &models.UserModel{DB: db}

	owner, err := userModel.Create(ctx, "owner@example.com", "Owner", "correct-password")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	otherUser, err := userModel.Create(ctx, "other@example.com", "Other User", "correct-password")
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}

	projectModel := &models.ProjectModel{DB: db}
	project, err := projectModel.Create(ctx, owner.ID, "Private Project", "Only its owner may change it", nil)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	_, err = projectModel.UpdateByIDForUser(ctx, project.ID, otherUser.ID, models.ProjectUpdate{
		Name:        "Stolen Project",
		Description: "This must not be saved",
		Status:      models.ProjectStatusArchived,
		TargetDate:  nil,
	})
	if !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("update error = %v, want models.ErrNoRecord", err)
	}

	err = projectModel.DeleteByIDForUser(ctx, project.ID, otherUser.ID)
	if !errors.Is(err, models.ErrNoRecord) {
		t.Errorf("delete error = %v, want models.ErrNoRecord", err)
	}

	got, err := projectModel.GetByIDForUser(ctx, project.ID, owner.ID)
	if err != nil {
		t.Fatalf("get owner's project: %v", err)
	}

	if got.Name != project.Name {
		t.Errorf("Name = %q, want %q", got.Name, project.Name)
	}
}
