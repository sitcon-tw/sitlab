package cardactivity

import (
	"context"
	"errors"

	"example.com/project-template/internal/domain/board"
)

var ErrGitLabForbidden = errors.New("GitLab access is forbidden")

type CardRepository interface {
	Card(context.Context, int64) (board.Card, error)
}

type GitLab interface {
	ProjectLabels(context.Context) ([]ProjectLabel, error)
	Comments(context.Context, int64, string) ([]Comment, error)
	CreateComment(context.Context, int64, string, string) (Comment, error)
}

type ActorTokens interface {
	AccessToken(context.Context, string) (string, error)
}
