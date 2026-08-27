package cardactivity

import (
	"context"
	"errors"

	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/directory"
)

var (
	ErrGitLabForbidden = errors.New("GitLab access is forbidden")
	// ErrLabelConflict is a name collision reported by GitLab.
	ErrLabelConflict = errors.New("GitLab project label already exists")
	// ErrLabelRejected is a payload GitLab refused for a reason we did not catch.
	ErrLabelRejected = errors.New("GitLab rejected the project label")
)

type CardRepository interface {
	Card(context.Context, int64) (board.Card, error)
}

type GitLab interface {
	ProjectLabels(context.Context) ([]ProjectLabel, error)
	// Label writes run as the acting user so GitLab's own project role decides
	// who may manage a project-wide resource. The trailing string is the actor
	// access token, matching Comments and CreateComment.
	CreateProjectLabel(context.Context, ProjectLabelWrite, string) (ProjectLabel, error)
	UpdateProjectLabel(context.Context, int64, ProjectLabelWrite, string) (ProjectLabel, error)
	DeleteProjectLabel(context.Context, int64, string) error
	Comments(context.Context, int64, string) ([]Comment, error)
	CreateComment(context.Context, int64, string, string) (Comment, error)
}

// Directory supplies the configured team labels the reserved-label rule needs.
type Directory interface {
	Snapshot(context.Context) (directory.Snapshot, error)
}

type ActorTokens interface {
	AccessToken(context.Context, string) (string, error)
}
