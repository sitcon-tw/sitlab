package httpserver

import (
	"time"

	"example.com/project-template/internal/domain/identity"
	domaintask "example.com/project-template/internal/domain/task"
	domainworkspace "example.com/project-template/internal/domain/workspace"
)

type userResponse struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type workspaceResponse struct {
	ID              string               `json:"id"`
	Name            string               `json:"name"`
	Role            domainworkspace.Role `json:"role"`
	CreatedByUserID string               `json:"createdByUserId"`
	CreatedAt       time.Time            `json:"createdAt"`
	UpdatedAt       time.Time            `json:"updatedAt"`
}

type memberResponse struct {
	UserID      string               `json:"userId"`
	Email       string               `json:"email"`
	DisplayName string               `json:"displayName"`
	Role        domainworkspace.Role `json:"role"`
	CreatedAt   time.Time            `json:"createdAt"`
}

type taskResponse struct {
	ID          string            `json:"id"`
	WorkspaceID string            `json:"workspaceId"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Status      domaintask.Status `json:"status"`
	AssigneeID  *string           `json:"assigneeId"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

func mapUser(item identity.User) userResponse {
	return userResponse{ID: item.ID, Email: item.Email, DisplayName: item.DisplayName, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func mapWorkspace(item domainworkspace.Workspace) workspaceResponse {
	return workspaceResponse{ID: item.ID, Name: item.Name, Role: item.Role, CreatedByUserID: item.CreatedByUserID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func mapMember(item domainworkspace.Member) memberResponse {
	return memberResponse{UserID: item.UserID, Email: item.Email, DisplayName: item.DisplayName, Role: item.Role, CreatedAt: item.JoinedAt}
}

func mapTask(item domaintask.Task) taskResponse {
	return taskResponse{ID: item.ID, WorkspaceID: item.WorkspaceID, Title: item.Title, Description: item.Description, Status: item.Status, AssigneeID: item.AssigneeUserID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
