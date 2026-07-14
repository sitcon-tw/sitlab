package bootstrap

import (
	"context"

	appboard "example.com/project-template/internal/controller/application/board"
	appdirectory "example.com/project-template/internal/controller/application/directory"
	"example.com/project-template/internal/domain/board"
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

type SyncStatus = board.SyncStatus

type Sync interface {
	Status(context.Context) (SyncStatus, error)
}
