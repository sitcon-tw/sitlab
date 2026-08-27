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

// IssueOrder is the page order a read walks GitLab in. It is not cosmetic: the two
// orders have different safety properties under concurrent edits.
type IssueOrder int

const (
	// IssueOrderUpdatedAsc suits an incremental read. updated_at only moves forward,
	// so a row edited mid-pagination can only jump past the cursor, never behind it.
	// Rows can repeat, which is harmless, but none can be skipped.
	IssueOrderUpdatedAsc IssueOrder = iota
	// IssueOrderCreatedAsc suits a full enumeration. created_at never changes, so
	// paging by it under concurrent edits yields neither skips nor duplicates.
	IssueOrderCreatedAsc
)

// IssueFilter narrows a GitLab issue read. The zero value reads every issue.
type IssueFilter struct {
	// UpdatedAfter is inclusive on GitLab's side, so a row exactly on the boundary is
	// re-read rather than skipped.
	UpdatedAfter *time.Time
	IIDs         []int64
	Order        IssueOrder
}

type GitLab interface {
	ProjectMembers(context.Context) ([]directory.GitLabMember, error)
	Issues(context.Context, IssueFilter) ([]board.CanonicalIssue, error)
	// IssueDigests reads iid and updated_at for every issue, without the widgets that
	// make a full read expensive.
	IssueDigests(context.Context) ([]board.IssueDigest, error)
	Issue(context.Context, int64) (board.CanonicalIssue, error)
	ApplyIssue(context.Context, board.IssueMutation, string) (board.CanonicalIssue, error)
}

type ActorTokens interface {
	AccessToken(context.Context, string) (string, error)
}

type Repository interface {
	Snapshot(context.Context) (directory.Snapshot, error)
	Board(context.Context) (board.Snapshot, error)
	ReplaceDirectory(context.Context, directory.Snapshot) error
	ApplyBoardObservation(context.Context, board.BoardObservation) error
	EnsureBoardLists(context.Context, []board.List, time.Time) error
	BoardCursor(context.Context) (board.SyncCursor, error)
	// SweepStartedAt reads PostgreSQL's clock. A full-board read must take it before
	// its first GitLab page so that concurrent webhook reconciles are not mistaken for
	// cards GitLab has dropped.
	SweepStartedAt(context.Context) (time.Time, error)
	QuarantineCard(ctx context.Context, issueIID int64, gitLabUpdatedAt time.Time, reason string, at time.Time) error
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
