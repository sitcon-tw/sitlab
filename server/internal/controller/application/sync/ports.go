package sync

import (
	"context"
	"time"

	appboard "example.com/project-template/internal/controller/application/board"
	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/directory"
)

type GitLabIssue struct {
	IssueIID             int64
	GitLabIssueID        int64
	Title                string
	WebURL               string
	Labels               []string
	AssigneeGitLabUserID *int64
	DueDate              string
	State                string
	UpdatedAt            time.Time
}

type GitLab interface {
	DirectoryRevision(context.Context) (string, error)
	DirectoryFile(context.Context) (directory.File, string, error)
	ProjectMembers(context.Context) ([]directory.GitLabMember, error)
	Issues(context.Context) ([]GitLabIssue, error)
	ApplyIssue(context.Context, IssueMutation) (GitLabIssue, error)
}

type IssueMutation struct {
	Create               bool
	IssueIID             int64
	Title                string
	Labels               []string
	AssigneeGitLabUserID *int64
	DueDate              string
	Closed               bool
}

type PendingOperation struct {
	Operation board.Operation
	Card      board.Card
}

type Repository interface {
	Snapshot(context.Context) (directory.Snapshot, error)
	Board(context.Context) (appboard.Snapshot, error)
	ReplaceDirectory(context.Context, directory.Snapshot) error
	ReplaceBoard(context.Context, []board.List, []board.Card, string, time.Time) error
	RecordSyncFailure(context.Context, string, time.Time, string) error
	ClaimOperation(context.Context, time.Time) (PendingOperation, error)
	CompleteOperation(context.Context, PendingOperation, GitLabIssue, time.Time) error
	FailOperation(context.Context, PendingOperation, time.Time, string, string) error
}

type MissingMemberLogger interface {
	DirectoryMemberMissing(teamKey, username string)
}
