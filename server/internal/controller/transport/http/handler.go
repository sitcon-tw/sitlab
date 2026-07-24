package httpserver

import (
	"context"
	"net/http"
	"time"

	appboard "example.com/project-template/internal/controller/application/board"
	appbootstrap "example.com/project-template/internal/controller/application/bootstrap"
	appdirectory "example.com/project-template/internal/controller/application/directory"
	appoauth "example.com/project-template/internal/controller/application/oauth"
	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/directory"
	"example.com/project-template/internal/domain/identity"
)

type AuthService interface {
	Start(context.Context) (appoauth.StartResult, error)
	Complete(context.Context, appoauth.CompleteInput) (appoauth.Authenticated, error)
	VerifySession(context.Context, string) (identity.SessionClaims, error)
	VerifyCSRFToken(context.Context, string, string) (identity.SessionClaims, error)
	IssueCSRF(context.Context, identity.SessionClaims) (string, error)
	Logout(context.Context, string) error
	Me(context.Context, string) (identity.User, error)
}

type BootstrapService interface {
	Get(context.Context, identity.SessionClaims) (appbootstrap.Result, error)
}

type DirectoryService interface {
	Snapshot(context.Context) (directory.Snapshot, error)
	Preferences(context.Context, string) (appdirectory.Preferences, error)
	Update(context.Context, string, string) (appdirectory.Preferences, error)
}

type BoardService interface {
	Create(context.Context, appboard.CreateInput) (appboard.Result, error)
	UpdateDetails(context.Context, appboard.UpdateDetailsInput) (appboard.Result, error)
	UpdateTeam(context.Context, appboard.UpdateTeamInput) (appboard.Result, error)
	UpdateAssignee(context.Context, appboard.UpdateAssigneeInput) (appboard.Result, error)
	UpdateStartDate(context.Context, appboard.UpdateStartDateInput) (appboard.Result, error)
	UpdateDueDate(context.Context, appboard.UpdateDueDateInput) (appboard.Result, error)
	Move(context.Context, appboard.MoveInput) (appboard.Result, error)
	Retry(context.Context, string) (board.Operation, error)
}

type SyncService interface {
	RequestRefresh() time.Time
	EnqueueWebhook(context.Context, board.WebhookDelivery) (bool, error)
}

type RevisionEvents interface {
	Revision(context.Context) (string, error)
	SubscribeRevisions() (<-chan string, func())
}

type RealtimeMetrics interface {
	WebhookDelivery(scope, result string)
	SSEConnected()
	SSEDisconnected()
	SSEEvent()
}

type handler struct {
	auth      AuthService
	bootstrap BootstrapService
	directory DirectoryService
	board     BoardService
	sync      SyncService
	webhooks  WebhookConfig
	events    RevisionEvents
	metrics   RealtimeMetrics
	cookie    CookieConfig
}

type WebhookConfig struct {
	ProjectSigningToken string
	GroupSigningToken   string
	ProjectPath         string
	GroupPath           string
}

type CookieConfig struct {
	Name   string
	Secure bool
	TTL    time.Duration
}

func actorID(r *http.Request) string { return claimsFromContext(r.Context()).UserID }
