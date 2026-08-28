package cardrelation

import (
	"context"
	"errors"

	"example.com/project-template/internal/domain/board"
)

var (
	ErrGitLabForbidden    = errors.New("GitLab relationship access is forbidden")
	ErrWorkItemNotFound   = errors.New("GitLab work item not found")
	ErrRelationConflict   = errors.New("GitLab work item relationship conflicts with current state")
	ErrInvalidRelation    = errors.New("GitLab work item relationship is invalid")
	ErrFeatureUnavailable = errors.New("GitLab work item relationship feature is unavailable")
)

type CardRepository interface {
	Card(context.Context, int64) (board.Card, error)
}

type RelationshipReader interface {
	ChildItems(context.Context, int64, PageQuery, string) (ChildPage, error)
	LinkedItems(context.Context, int64, PageQuery, string) (LinkedPage, error)
	RelationshipCandidates(context.Context, int64, RelationshipKind, CandidateQuery, string) ([]WorkItem, error)
}

type ChildWriter interface {
	CreateChild(context.Context, int64, string, string) (WorkItem, error)
	AttachChild(context.Context, int64, int64, string) error
	DetachChild(context.Context, int64, int64, string) error
}

type LinkWriter interface {
	AddLinks(context.Context, int64, []int64, LinkType, string) error
	RemoveLink(context.Context, int64, int64, string) error
}

type ActorTokens interface {
	AccessToken(context.Context, string) (string, error)
}
