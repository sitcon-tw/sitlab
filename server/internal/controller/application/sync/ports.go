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

type GitLab interface {
	DirectoryRevision(context.Context) (string, error)
	DirectoryFile(context.Context) (directory.File, string, error)
	ProjectMembers(context.Context) ([]directory.GitLabMember, error)
	Issues(context.Context) ([]board.CanonicalIssue, error)
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
}

type MissingMemberLogger interface {
	DirectoryMemberMissing(teamKey, username string)
}
