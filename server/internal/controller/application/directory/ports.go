package directory

import (
	"context"
	"time"

	domain "example.com/project-template/internal/domain/directory"
)

type Preferences = domain.Preferences

type Repository interface {
	Snapshot(context.Context) (domain.Snapshot, error)
	Preferences(context.Context, string) (domain.Preferences, error)
	SetPreferences(context.Context, string, string, time.Time) (domain.Preferences, error)
}
