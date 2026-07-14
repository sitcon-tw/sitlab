package directory

import (
	"context"
	"time"

	domain "example.com/project-template/internal/domain/directory"
)

type Preferences struct {
	DefaultTeamKey    *string
	ConfirmedAt       *time.Time
	DirectoryTeamKeys []string
}

type Repository interface {
	Snapshot(context.Context) (domain.Snapshot, error)
	Preferences(context.Context, string) (Preferences, error)
	SetPreferences(context.Context, string, string, time.Time) (Preferences, error)
}
