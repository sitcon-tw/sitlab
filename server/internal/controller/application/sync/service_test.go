package sync

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	appboard "example.com/project-template/internal/controller/application/board"
	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/directory"
)

func TestDefaultBoardListsMatchGitLabBoard(t *testing.T) {
	t.Parallel()
	want := []board.List{
		{Key: "wating", Name: "Wating", GitLabLabel: "Status::Waiting", Position: 0, Color: "#dc2626"},
		{Key: "inbox", Name: "Inbox", GitLabLabel: "Status::Inbox", Position: 1, Color: "#64748b"},
		{Key: "todo", Name: "To Do", GitLabLabel: "Status::To Do", Position: 2, Color: "#0891b2"},
		{Key: "doing", Name: "Doing", GitLabLabel: "Status::Doing", Position: 3, Color: "#2563eb"},
		{Key: "review", Name: "Review", GitLabLabel: "Status::Review", Position: 4, Color: "#b45309"},
		{Key: "closed", Name: "Closed", Position: 5, Closed: true, Color: "#15803d"},
	}
	if !reflect.DeepEqual(DefaultBoardLists, want) {
		t.Fatalf("DefaultBoardLists = %#v", DefaultBoardLists)
	}
}

type gitLabFake struct {
	members []directory.GitLabMember
	issues  []GitLabIssue
	applied *IssueMutation
}

type directorySourceFake struct {
	revision  string
	fileCalls int
}

func (f *directorySourceFake) DirectoryRevision(context.Context) (string, error) {
	return f.revision, nil
}
func (f *directorySourceFake) DirectoryFile(context.Context) (directory.File, string, error) {
	f.fileCalls++
	return directory.File{Version: 1, Teams: []directory.TeamConfig{{
		Key: "development", Name: "開發組", TitlePrefix: "[開發組]",
		GitLabLabel: "組別::開發", Active: true, Members: []string{"alice"},
	}}}, f.revision, nil
}
func (f *gitLabFake) ProjectMembers(context.Context) ([]directory.GitLabMember, error) {
	return f.members, nil
}
func (f *gitLabFake) Issues(context.Context) ([]GitLabIssue, error) { return f.issues, nil }
func (f *gitLabFake) ApplyIssue(_ context.Context, mutation IssueMutation) (GitLabIssue, error) {
	f.applied = &mutation
	return GitLabIssue{
		IssueIID: 42, GitLabIssueID: 420, Title: mutation.Title, Description: mutation.Description,
		Labels: mutation.Labels, AssigneeGitLabUserIDs: mutation.AssigneeGitLabUserIDs,
		StartDate: mutation.StartDate, DueDate: mutation.DueDate, State: "opened",
	}, nil
}

type repoFake struct {
	directory directory.Snapshot
	board     appboard.Snapshot
	cards     []board.Card
	pending   *PendingOperation
	completed bool
}

func (f *repoFake) Snapshot(context.Context) (directory.Snapshot, error) { return f.directory, nil }
func (f *repoFake) Board(context.Context) (appboard.Snapshot, error)     { return f.board, nil }
func (f *repoFake) ReplaceDirectory(_ context.Context, snapshot directory.Snapshot) error {
	f.directory = snapshot
	return nil
}
func (f *repoFake) ReplaceBoard(_ context.Context, _ []board.List, cards []board.Card, _ string, _ time.Time) error {
	f.cards = cards
	return nil
}
func (*repoFake) RecordSyncFailure(context.Context, string, time.Time, string) error { return nil }
func (f *repoFake) ClaimOperation(context.Context, time.Time) (PendingOperation, error) {
	if f.pending == nil {
		return PendingOperation{}, board.ErrOperationNotFound
	}
	return *f.pending, nil
}
func (f *repoFake) CompleteOperation(context.Context, PendingOperation, GitLabIssue, time.Time) error {
	f.completed = true
	return nil
}
func (*repoFake) FailOperation(context.Context, PendingOperation, time.Time, string, string) error {
	return nil
}

func TestRefreshDirectoryUsesRevisionAndRefreshesMembers(t *testing.T) {
	t.Parallel()
	gitlab := &gitLabFake{
		members: []directory.GitLabMember{{GitLabUserID: 1, Username: "alice", DisplayName: "Alice", State: directory.MemberActive}},
	}
	directorySource := &directorySourceFake{revision: "revision-1"}
	repo := &repoFake{}
	service := NewService(gitlab, directorySource, repo, nil, noop.NewTracerProvider().Tracer("test"))
	if err := service.RefreshDirectory(context.Background()); err != nil {
		t.Fatal(err)
	}
	if directorySource.fileCalls != 1 || len(repo.directory.Members) != 1 {
		t.Fatalf("first refresh: files=%d snapshot=%#v", directorySource.fileCalls, repo.directory)
	}
	gitlab.members[0].DisplayName = "Alice Updated"
	if err := service.RefreshDirectory(context.Background()); err != nil {
		t.Fatal(err)
	}
	if directorySource.fileCalls != 1 || repo.directory.Members[0].DisplayName != "Alice Updated" {
		t.Fatalf("unchanged revision downloaded again or member stale: files=%d snapshot=%#v", directorySource.fileCalls, repo.directory)
	}
}

func TestRefreshBoardMapsLabelsAndSkipsUnknownTeams(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 14, 8, 0, 0, 0, time.UTC)
	gitlab := &gitLabFake{issues: []GitLabIssue{
		{IssueIID: 1, GitLabIssueID: 10, Title: "[開發組] 修正流程", Labels: []string{"組別::開發", "Inbox", "Todo", "Status::Doing"}, StartDate: "2026-07-17", State: "opened", UpdatedAt: now},
		{IssueIID: 2, GitLabIssueID: 20, Title: "[開發組] 舊版狀態", Labels: []string{"組別::開發", "Todo"}, State: "opened", UpdatedAt: now},
		{IssueIID: 3, GitLabIssueID: 30, Title: "[開發組] 等待中", Labels: []string{"組別::開發", "Inbox", "Status::Waiting"}, State: "opened", UpdatedAt: now},
		{IssueIID: 4, GitLabIssueID: 40, Title: "[開發組] 已關閉", Labels: []string{"組別::開發", "Status::Doing"}, State: "closed", UpdatedAt: now},
		{IssueIID: 5, GitLabIssueID: 50, Title: "無組別", Labels: []string{"Todo"}, State: "opened", UpdatedAt: now},
	}}
	repo := &repoFake{directory: directory.Snapshot{Teams: []directory.Team{{Key: "development", TitlePrefix: "[開發組]", GitLabLabel: "組別::開發", Active: true}}}}
	service := NewService(gitlab, &directorySourceFake{}, repo, nil, noop.NewTracerProvider().Tracer("test"))
	if err := service.RefreshBoard(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.cards) != 4 || repo.cards[0].ListKey != "doing" || repo.cards[0].Title != "修正流程" || repo.cards[0].StartDate != "2026-07-17" ||
		repo.cards[1].ListKey != "todo" || repo.cards[2].ListKey != "wating" || repo.cards[3].ListKey != "closed" {
		t.Fatalf("cards = %#v", repo.cards)
	}
}

func TestProcessOneBuildsCanonicalIssueMutation(t *testing.T) {
	t.Parallel()
	gitlab := &gitLabFake{}
	repo := &repoFake{
		directory: directory.Snapshot{Teams: []directory.Team{
			{Key: "development", TitlePrefix: "[開發組]", GitLabLabel: "組別::開發", Active: true},
			{Key: "design", TitlePrefix: "[設計組]", GitLabLabel: "組別::設計", Active: true},
		}},
		board: appboard.Snapshot{Lists: DefaultBoardLists},
		pending: &PendingOperation{
			Operation: board.Operation{ID: "operation", Kind: board.OperationUpdateTeam},
			Card: board.Card{
				IssueIID: 42, Title: "修正流程", Description: "詳細規劃", TeamKey: "development", ListKey: "doing",
				AssigneeGitLabUserIDs: []int64{1, 2}, StartDate: "2026-07-17", DueDate: "2026-07-21", Labels: []string{"組別::設計", "Inbox", "Todo", "Status::Review", "security"},
			},
		},
	}
	service := NewService(gitlab, &directorySourceFake{}, repo, nil, noop.NewTracerProvider().Tracer("test"))
	processed, err := service.ProcessOne(context.Background())
	if err != nil || !processed || !repo.completed {
		t.Fatalf("ProcessOne() = %v, %v, completed=%v", processed, err, repo.completed)
	}
	if gitlab.applied == nil || gitlab.applied.Title != "[開發組] 修正流程" || gitlab.applied.Description != "詳細規劃" ||
		gitlab.applied.StartDate != "2026-07-17" || gitlab.applied.DueDate != "2026-07-21" ||
		!slices.Equal(gitlab.applied.AssigneeGitLabUserIDs, []int64{1, 2}) || !slices.Equal(gitlab.applied.Labels, []string{"security", "組別::開發", "Status::Doing"}) {
		t.Fatalf("mutation = %#v", gitlab.applied)
	}
}
