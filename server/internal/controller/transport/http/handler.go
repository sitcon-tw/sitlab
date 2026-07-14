package httpserver

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	appauth "example.com/project-template/internal/controller/application/auth"
	apptask "example.com/project-template/internal/controller/application/task"
	appworkspace "example.com/project-template/internal/controller/application/workspace"
	"example.com/project-template/internal/domain/identity"
	domaintask "example.com/project-template/internal/domain/task"
	domainworkspace "example.com/project-template/internal/domain/workspace"
)

type AuthService interface {
	Register(context.Context, appauth.RegisterInput) (appauth.Authenticated, error)
	Login(context.Context, appauth.LoginInput) (appauth.Authenticated, error)
	VerifySession(context.Context, string) (identity.SessionClaims, error)
	VerifyCSRFToken(context.Context, string, string) (identity.SessionClaims, error)
	IssueCSRF(context.Context, identity.SessionClaims) (string, error)
	Logout(context.Context, string) error
	Me(context.Context, string) (identity.User, error)
}

type WorkspaceService interface {
	Create(context.Context, appworkspace.CreateInput) (domainworkspace.Workspace, error)
	List(context.Context, string) ([]domainworkspace.Workspace, error)
	Get(context.Context, string, string) (domainworkspace.Workspace, error)
	Update(context.Context, appworkspace.UpdateInput) (domainworkspace.Workspace, error)
	Delete(context.Context, string, string) error
	ListMembers(context.Context, string, string) ([]domainworkspace.Member, error)
	AddMember(context.Context, appworkspace.AddMemberInput) (domainworkspace.Member, error)
	UpdateMember(context.Context, appworkspace.UpdateMemberInput) (domainworkspace.Member, error)
	RemoveMember(context.Context, string, string, string) error
}

type TaskService interface {
	Create(context.Context, apptask.CreateInput) (domaintask.Task, error)
	List(context.Context, string, string, string) ([]domaintask.Task, error)
	Get(context.Context, string, string, string) (domaintask.Task, error)
	Update(context.Context, apptask.UpdateInput) (domaintask.Task, error)
	Delete(context.Context, string, string, string) error
}

type handler struct {
	auth       AuthService
	workspaces WorkspaceService
	tasks      TaskService
	cookie     CookieConfig
}

type CookieConfig struct {
	Name   string
	Secure bool
	TTL    time.Duration
}

func actorID(r *http.Request) string     { return claimsFromContext(r.Context()).UserID }
func workspaceID(r *http.Request) string { return chi.URLParam(r, "workspaceId") }
func taskID(r *http.Request) string      { return chi.URLParam(r, "taskId") }
