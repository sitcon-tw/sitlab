package bootstrap

import (
	"context"
	"time"

	appboard "example.com/project-template/internal/controller/application/board"
	appdirectory "example.com/project-template/internal/controller/application/directory"
	"example.com/project-template/internal/domain/directory"
	"example.com/project-template/internal/domain/identity"
)

type Auth interface {
	IssueCSRF(context.Context, identity.SessionClaims) (string, error)
	Me(context.Context, string) (identity.User, error)
}

type Directory interface {
	Snapshot(context.Context) (directory.Snapshot, error)
	Preferences(context.Context, string) (appdirectory.Preferences, error)
}

type Board interface {
	Board(context.Context) (appboard.Snapshot, error)
}

type SyncStatus struct {
	State         string
	LastSuccessAt time.Time
	Message       string
}

type Sync interface {
	Status(context.Context) (SyncStatus, error)
}
