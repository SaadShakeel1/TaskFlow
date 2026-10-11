package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

type createProjectMemberRequest struct {
	Email string                   `json:"email"`
	Role  models.ProjectMemberRole `json:"role"`
}

type updateProjectMemberRequest struct {
	Role models.ProjectMemberRole `json:"role"`
}

type projectMemberResponse struct {
	UserID    int64                    `json:"userId"`
	Email     string                   `json:"email"`
	Name      string                   `json:"name"`
	Role      models.ProjectMemberRole `json:"role"`
	CreatedAt time.Time                `json:"createdAt"`
}

type projectMemberListResponse struct {
	Members []projectMemberResponse `json:"members"`
}

func (app *application) createProjectMemberHandler(w http.ResponseWriter, r *http.Request) {
	project, owner, ok := app.ownedProjectForRequest(w, r)
	if !ok {
		return
	}

	var request createProjectMemberRequest
	if requestError := readJSON(w, r, &request); requestError != nil {
		app.clientError(w, requestError.status, requestError.message)
		return
	}

	email := strings.TrimSpace(request.Email)
	if email == "" {
		app.clientError(w, http.StatusBadRequest, "member email is required")
		return
	}

	if !models.IsValidProjectMemberRole(request.Role) {
		app.clientError(w, http.StatusBadRequest, "member role must be editor or viewer")
		return
	}

	memberUser, err := app.users.GetByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w, "user not found")
			return
		}
		app.serverError(w, err)
		return
	}

	if memberUser.ID == owner.ID {
		app.clientError(w, http.StatusBadRequest, "project owner cannot be added as a member")
		return
	}

	member, err := app.projectMembers.Create(r.Context(), project.ID, memberUser.ID, request.Role)
	if err != nil {
		if errors.Is(err, models.ErrDuplicateProjectMember) {
			app.clientError(w, http.StatusConflict, "user is already a project member")
			return
		}
		app.serverError(w, err)
		return
	}

	member.Name = memberUser.Name
	member.Email = memberUser.Email

	memberID := member.UserID
	app.recordActivity(r.Context(), models.ActivityCreateInput{
		ProjectID:  project.ID,
		UserID:     &owner.ID,
		Action:     "added",
		EntityType: "member",
		EntityID:   &memberID,
	})

	if err := app.writeJSON(w, http.StatusCreated, newProjectMemberResponse(member)); err != nil {
		app.errorLog.Printf("encode project member response: %v", err)
	}
}

func (app *application) listProjectMembersHandler(w http.ResponseWriter, r *http.Request) {
	project, _, ok := app.projectForRequest(w, r)
	if !ok {
		return
	}

	members, err := app.projectMembers.ListByProject(r.Context(), project.ID)
	if err != nil {
		app.serverError(w, err)
		return
	}

	response := projectMemberListResponse{
		Members: make([]projectMemberResponse, 0, len(members)),
	}
	for _, member := range members {
		response.Members = append(response.Members, newProjectMemberResponse(member))
	}

	if err := app.writeJSON(w, http.StatusOK, response); err != nil {
		app.errorLog.Printf("encode project members response: %v", err)
	}
}

func (app *application) updateProjectMemberHandler(w http.ResponseWriter, r *http.Request) {
	project, _, ok := app.ownedProjectForRequest(w, r)
	if !ok {
		return
	}

	memberUserID, ok := projectMemberUserIDFromRequest(w, r)
	if !ok {
		return
	}

	if memberUserID == project.UserID {
		app.clientError(w, http.StatusBadRequest, "project owner role cannot be changed")
		return
	}

	var request updateProjectMemberRequest
	if requestError := readJSON(w, r, &request); requestError != nil {
		app.clientError(w, requestError.status, requestError.message)
		return
	}

	if !models.IsValidProjectMemberRole(request.Role) {
		app.clientError(w, http.StatusBadRequest, "member role must be editor or viewer")
		return
	}

	member, err := app.projectMembers.UpdateRole(r.Context(), project.ID, memberUserID, request.Role)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w, "project member not found")
			return
		}
		app.serverError(w, err)
		return
	}

	memberUser, err := app.users.GetByID(r.Context(), member.UserID)
	if err != nil {
		app.serverError(w, err)
		return
	}

	member.Name = memberUser.Name
	member.Email = memberUser.Email

	memberID := member.UserID
	app.recordActivity(r.Context(), models.ActivityCreateInput{
		ProjectID:  project.ID,
		UserID:     &project.UserID,
		Action:     "role_changed",
		EntityType: "member",
		EntityID:   &memberID,
	})

	if err := app.writeJSON(w, http.StatusOK, newProjectMemberResponse(member)); err != nil {
		app.errorLog.Printf("encode project member response: %v", err)
	}
}

func (app *application) deleteProjectMemberHandler(w http.ResponseWriter, r *http.Request) {
	project, _, ok := app.ownedProjectForRequest(w, r)
	if !ok {
		return
	}

	memberUserID, ok := projectMemberUserIDFromRequest(w, r)
	if !ok {
		return
	}

	if memberUserID == project.UserID {
		app.clientError(w, http.StatusBadRequest, "project owner cannot be removed")
		return
	}

	if err := app.projectMembers.Delete(r.Context(), project.ID, memberUserID); err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w, "project member not found")
			return
		}
		app.serverError(w, err)
		return
	}

	app.recordActivity(r.Context(), models.ActivityCreateInput{
		ProjectID:  project.ID,
		UserID:     &project.UserID,
		Action:     "removed",
		EntityType: "member",
		EntityID:   &memberUserID,
	})

	w.WriteHeader(http.StatusNoContent)
}

func (app *application) projectForRequest(w http.ResponseWriter, r *http.Request) (models.Project, models.User, bool) {
	projectID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || projectID < 1 {
		app.clientError(w, http.StatusBadRequest, "project id must be a positive integer")
		return models.Project{}, models.User{}, false
	}

	user, ok := r.Context().Value(userContextKey).(models.User)
	if !ok {
		app.serverError(w, errors.New("authenticated user missing from context"))
		return models.Project{}, models.User{}, false
	}

	project, err := app.projects.GetByIDForUser(r.Context(), projectID, user.ID)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w, "project not found")
			return models.Project{}, models.User{}, false
		}
		app.serverError(w, err)
		return models.Project{}, models.User{}, false
	}

	return project, user, true
}

func (app *application) ownedProjectForRequest(w http.ResponseWriter, r *http.Request) (models.Project, models.User, bool) {
	project, user, ok := app.projectForRequest(w, r)
	if !ok {
		return models.Project{}, models.User{}, false
	}

	if project.UserID != user.ID {
		app.clientError(w, http.StatusForbidden, "project owner permission required")
		return models.Project{}, models.User{}, false
	}

	return project, user, true
}

func projectMemberUserIDFromRequest(w http.ResponseWriter, r *http.Request) (int64, bool) {
	userID, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	if err != nil || userID < 1 {
		http.Error(w, "member user id must be a positive integer", http.StatusBadRequest)
		return 0, false
	}

	return userID, true
}

func newProjectMemberResponse(member models.ProjectMember) projectMemberResponse {
	return projectMemberResponse{
		UserID:    member.UserID,
		Email:     member.Email,
		Name:      member.Name,
		Role:      member.Role,
		CreatedAt: member.CreatedAt,
	}
}
