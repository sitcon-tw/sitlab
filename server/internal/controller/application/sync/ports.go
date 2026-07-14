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
}

type Repository interface {
	Snapshot(context.Context) (directory.Snapshot, error)
	Board(context.Context) (appboard.Snapshot, error)
	ReplaceDirectory(context.Context, directory.Snapshot) error
	ReplaceBoard(context.Context, []board.List, []board.Card, string, time.Time) error
	RecordSyncFailure(context.Context, string, time.Time, string) error
}

type MissingMemberLogger interface {
	DirectoryMemberMissing(teamKey, username string)
}
