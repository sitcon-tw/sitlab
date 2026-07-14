package sync

import (
	"context"
	"slices"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	appboard "example.com/project-template/internal/controller/application/board"
	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/directory"
)

type gitLabFake struct {
	revision  string
	fileCalls int
	members   []directory.GitLabMember
	issues    []GitLabIssue
	applied   *IssueMutation
}

func (f *gitLabFake) DirectoryRevision(context.Context) (string, error) { return f.revision, nil }
func (f *gitLabFake) DirectoryFile(context.Context) (directory.File, string, error) {
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
	return GitLabIssue{IssueIID: 42, GitLabIssueID: 420, Title: mutation.Title, Labels: mutation.Labels, State: "opened"}, nil
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
		revision: "revision-1",
		members:  []directory.GitLabMember{{GitLabUserID: 1, Username: "alice", DisplayName: "Alice", State: directory.MemberActive}},
	}
	repo := &repoFake{}
	service := NewService(gitlab, repo, nil, noop.NewTracerProvider().Tracer("test"))
	if err := service.RefreshDirectory(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gitlab.fileCalls != 1 || len(repo.directory.Members) != 1 {
		t.Fatalf("first refresh: files=%d snapshot=%#v", gitlab.fileCalls, repo.directory)
	}
	gitlab.members[0].DisplayName = "Alice Updated"
	if err := service.RefreshDirectory(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gitlab.fileCalls != 1 || repo.directory.Members[0].DisplayName != "Alice Updated" {
		t.Fatalf("unchanged revision downloaded again or member stale: files=%d snapshot=%#v", gitlab.fileCalls, repo.directory)
	}
}

func TestRefreshBoardMapsLabelsAndSkipsUnknownTeams(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 14, 8, 0, 0, 0, time.UTC)
	gitlab := &gitLabFake{issues: []GitLabIssue{
		{IssueIID: 1, GitLabIssueID: 10, Title: "[開發組] 修正流程", Labels: []string{"組別::開發", "Doing"}, State: "opened", UpdatedAt: now},
		{IssueIID: 2, GitLabIssueID: 20, Title: "無組別", Labels: []string{"Todo"}, State: "opened", UpdatedAt: now},
	}}
	repo := &repoFake{directory: directory.Snapshot{Teams: []directory.Team{{Key: "development", TitlePrefix: "[開發組]", GitLabLabel: "組別::開發", Active: true}}}}
	service := NewService(gitlab, repo, nil, noop.NewTracerProvider().Tracer("test"))
	if err := service.RefreshBoard(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.cards) != 1 || repo.cards[0].ListKey != "doing" || repo.cards[0].Title != "修正流程" {
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
			Card:      board.Card{IssueIID: 42, Title: "修正流程", TeamKey: "development", ListKey: "doing", Labels: []string{"組別::設計", "Todo", "security"}},
		},
	}
	service := NewService(gitlab, repo, nil, noop.NewTracerProvider().Tracer("test"))
	processed, err := service.ProcessOne(context.Background())
	if err != nil || !processed || !repo.completed {
		t.Fatalf("ProcessOne() = %v, %v, completed=%v", processed, err, repo.completed)
	}
	if gitlab.applied == nil || gitlab.applied.Title != "[開發組] 修正流程" || !slices.Equal(gitlab.applied.Labels, []string{"security", "組別::開發", "Doing"}) {
		t.Fatalf("mutation = %#v", gitlab.applied)
	}
}
