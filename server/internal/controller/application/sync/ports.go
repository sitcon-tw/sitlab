package sync

import (
	"context"
	"time"

	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/directory"
)

type GitLabIssue = board.CanonicalIssue
type IssueMutation = board.IssueMutation
type PendingOperation = board.PendingOperation

type DirectorySource interface {
	DirectoryRevision(context.Context) (string, error)
	DirectoryFile(context.Context) (directory.File, string, error)
}

type GitLab interface {
	ProjectMembers(context.Context) ([]directory.GitLabMember, error)
	Issues(context.Context) ([]board.CanonicalIssue, error)
	Issue(context.Context, int64) (board.CanonicalIssue, error)
	ApplyIssue(context.Context, board.IssueMutation) (board.CanonicalIssue, error)
}

type Repository interface {
	Snapshot(context.Context) (directory.Snapshot, error)
	Board(context.Context) (board.Snapshot, error)
	ReplaceDirectory(context.Context, directory.Snapshot) error
	ReplaceBoard(context.Context, []board.List, []board.Card, string, time.Time) error
	RecordSyncFailure(context.Context, string, time.Time, string) error
	ClaimOperation(context.Context, time.Time) (board.PendingOperation, error)
	CompleteOperation(context.Context, board.PendingOperation, board.CanonicalIssue, time.Time) error
	FailOperation(context.Context, board.PendingOperation, time.Time, string, string) error
	EnqueueWebhook(context.Context, board.WebhookDelivery) (bool, error)
	ClaimWebhook(context.Context, time.Time) (board.WebhookDelivery, error)
	CompleteWebhook(context.Context, string, time.Time) error
	FailWebhook(context.Context, board.WebhookDelivery, time.Time, string) error
	ReconcileIssue(context.Context, int64, *board.Card, time.Time) (bool, error)
}

type MissingMemberLogger interface {
	DirectoryMemberMissing(teamKey, username string)
}
