package sync

import (
	"context"
	"errors"
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
		{Key: "wating", Name: "Waiting", GitLabStatusName: "Waiting", Position: 0, Color: "#d2ad46"},
		{Key: "inbox", Name: "Inbox", GitLabStatusName: "Inbox", Position: 1, Color: "#6699cc"},
		{Key: "todo", Name: "To do", GitLabStatusName: "To do", Position: 2, Color: "#ed9121"},
		{Key: "doing", Name: "Doing", GitLabStatusName: "Doing", Position: 3, Color: "#1f75cb"},
		{Key: "review", Name: "Review", GitLabStatusName: "Review", Position: 4, Color: "#7a07ab"},
		{Key: "closed", Name: "Done", GitLabStatusName: "Done", Position: 5, Closed: true, Color: "#108548"},
	}
	if !reflect.DeepEqual(DefaultBoardLists, want) {
		t.Fatalf("DefaultBoardLists = %#v", DefaultBoardLists)
	}
}

func developmentDirectory() directory.Snapshot {
	return directory.Snapshot{Teams: []directory.Team{{
		Key: "development", TitlePrefix: "[開發組]", GitLabLabel: "Team::開發組", Active: true,
	}}}
}

func TestDeltaReadsForwardFromGitLabsClockNotTheLocalOne(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, time.August, 18, 8, 0, 0, 0, time.UTC)
	newest := watermark.Add(7 * time.Minute)
	gitlab := &gitLabFake{issues: []GitLabIssue{{
		IssueIID: 11, GitLabIssueID: 110, Title: "[開發組] 工作",
		Labels: []string{"Team::開發組"}, GitLabStatusName: "Doing", UpdatedAt: newest,
	}}}
	repo := &repoFake{directory: developmentDirectory(), watermark: &watermark}
	service := NewService(gitlab, &directorySourceFake{}, repo, actorTokensFake{}, nil, noop.NewTracerProvider().Tracer("test"))
	service.deltaOverlap = 2 * time.Minute
	// A local clock hours away from GitLab's must not reach the boundary arithmetic.
	service.now = func() time.Time { return watermark.Add(-9 * time.Hour) }

	if err := service.RefreshBoardDelta(context.Background()); err != nil {
		t.Fatalf("RefreshBoardDelta() error = %v", err)
	}
	if gitlab.lastFilter.UpdatedAfter == nil {
		t.Fatal("delta read sent no lower bound; it would re-read the whole project every tick")
	}
	if want := watermark.Add(-2 * time.Minute); !gitlab.lastFilter.UpdatedAfter.Equal(want) {
		t.Fatalf("updatedAfter = %v, want the stored watermark less the overlap %v", *gitlab.lastFilter.UpdatedAfter, want)
	}
	if gitlab.lastFilter.Order != IssueOrderUpdatedAsc {
		t.Fatalf("delta order = %v, want UPDATED_ASC so a row edited mid-pagination cannot be skipped", gitlab.lastFilter.Order)
	}
	if repo.observation.Complete {
		t.Fatal("a delta read must never be Complete, or it would prune every card it did not mention")
	}
	if repo.observation.Watermark == nil || !repo.observation.Watermark.Equal(newest) {
		t.Fatalf("watermark = %v, want the newest GitLab timestamp observed %v", repo.observation.Watermark, newest)
	}
}

func TestDeltaTreatsALostTeamLabelAsARemoval(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, time.August, 18, 8, 0, 0, 0, time.UTC)
	updated := watermark.Add(time.Minute)
	gitlab := &gitLabFake{issues: []GitLabIssue{
		// No active Team:: label: this issue is no longer a board card.
		{IssueIID: 12, GitLabIssueID: 120, Title: "無組別", GitLabStatusName: "Doing", UpdatedAt: updated},
		{IssueIID: 13, GitLabIssueID: 130, Title: "[開發組] 工作", Labels: []string{"Team::開發組"}, GitLabStatusName: "Doing", UpdatedAt: updated},
	}}
	repo := &repoFake{directory: developmentDirectory(), watermark: &watermark}
	service := NewService(gitlab, &directorySourceFake{}, repo, actorTokensFake{}, nil, noop.NewTracerProvider().Tracer("test"))

	if err := service.RefreshBoardDelta(context.Background()); err != nil {
		t.Fatalf("RefreshBoardDelta() error = %v", err)
	}
	// A full read expresses "gone" by omission, but in a delta omission means
	// "unchanged", so the removal has to be stated outright.
	if len(repo.observation.Removed) != 1 || repo.observation.Removed[0] != 12 {
		t.Fatalf("removed = %v, want [12]", repo.observation.Removed)
	}
	if len(repo.observation.Cards) != 1 || repo.observation.Cards[0].IssueIID != 13 {
		t.Fatalf("cards = %#v, want only the issue that still carries a team label", repo.observation.Cards)
	}
}

func TestDeepReadPrunesByOmissionRatherThanStatement(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 18, 8, 0, 0, 0, time.UTC)
	gitlab := &gitLabFake{issues: []GitLabIssue{
		{IssueIID: 14, GitLabIssueID: 140, Title: "無組別", GitLabStatusName: "Doing", UpdatedAt: now},
	}}
	repo := &repoFake{directory: developmentDirectory()}
	service := NewService(gitlab, &directorySourceFake{}, repo, actorTokensFake{}, nil, noop.NewTracerProvider().Tracer("test"))

	if err := service.RefreshBoardDeep(context.Background()); err != nil {
		t.Fatalf("RefreshBoardDeep() error = %v", err)
	}
	if !repo.observation.Complete {
		t.Fatal("a deep read must be Complete, or nothing would ever prune")
	}
	if len(repo.observation.Removed) != 0 {
		t.Fatalf("removed = %v, want none: a complete read prunes by omission", repo.observation.Removed)
	}
	if gitlab.lastFilter.Order != IssueOrderCreatedAsc {
		t.Fatalf("deep order = %v, want CREATED_ASC so a full pass cannot skip or repeat", gitlab.lastFilter.Order)
	}
}

func TestPresenceSweepConfirmsExistenceWithoutWritingContent(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 18, 8, 0, 0, 0, time.UTC)
	gitlab := &gitLabFake{issues: []GitLabIssue{
		{IssueIID: 15, GitLabIssueID: 150, Title: "[開發組] 一", Labels: []string{"Team::開發組"}, GitLabStatusName: "Doing", UpdatedAt: now},
		{IssueIID: 16, GitLabIssueID: 160, Title: "[開發組] 二", Labels: []string{"Team::開發組"}, GitLabStatusName: "Doing", UpdatedAt: now},
	}}
	repo := &repoFake{directory: developmentDirectory()}
	service := NewService(gitlab, &directorySourceFake{}, repo, actorTokensFake{}, nil, noop.NewTracerProvider().Tracer("test"))

	if err := service.RefreshBoardPresence(context.Background()); err != nil {
		t.Fatalf("RefreshBoardPresence() error = %v", err)
	}
	if !repo.observation.Complete {
		t.Fatal("a presence sweep must be Complete: noticing deletions is the whole point of it")
	}
	if len(repo.observation.Cards) != 0 {
		t.Fatalf("cards = %#v, want none: a presence sweep confirms existence and nothing else", repo.observation.Cards)
	}
	if len(repo.observation.Retained) != 2 {
		t.Fatalf("retained = %v, want both issues", repo.observation.Retained)
	}
	if repo.observation.Watermark != nil {
		t.Fatal("a presence sweep read no content, so it must not move the incremental boundary")
	}
	if repo.observation.StartedAt.IsZero() {
		t.Fatal("a pruning read needs the instant it began, or a concurrent reconcile looks prunable")
	}
}

func TestBoardBackoffHoldsOffAfterFailureAndResetsOnSuccess(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 18, 8, 0, 0, 0, time.UTC)
	gate := &backoff{max: time.Minute}
	if !gate.ready(now) {
		t.Fatal("a fresh gate must allow the first read")
	}
	gate.record(now, errors.New("gitlab unavailable"))
	if gate.ready(now.Add(500 * time.Millisecond)) {
		t.Fatal("gate opened immediately after a failure")
	}
	if !gate.ready(now.Add(2 * time.Second)) {
		t.Fatal("gate stayed shut past its first delay")
	}
	gate.record(now, errors.New("still down"))
	gate.record(now, errors.New("still down"))
	first := gate.until
	gate.record(now, errors.New("still down"))
	if !gate.until.After(first) {
		t.Fatal("repeated failures must widen the delay")
	}
	if gate.until.Sub(now) > time.Minute {
		t.Fatalf("delay %v exceeded the configured maximum", gate.until.Sub(now))
	}
	gate.record(now, nil)
	if !gate.ready(now) || gate.failures != 0 {
		t.Fatal("a success must clear the gate")
	}
}

type gitLabFake struct {
	members    []directory.GitLabMember
	issues     []GitLabIssue
	applied    *IssueMutation
	token      string
	lastFilter IssueFilter
	filters    []IssueFilter
}

type actorTokensFake struct{}

func (actorTokensFake) AccessToken(context.Context, string) (string, error) {
	return "actor-token", nil
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
func (f *gitLabFake) Issues(_ context.Context, filter IssueFilter) ([]GitLabIssue, error) {
	f.lastFilter = filter
	f.filters = append(f.filters, filter)
	if filter.UpdatedAfter == nil {
		return f.issues, nil
	}
	// GitLab treats updatedAfter as inclusive, so a row exactly on the boundary comes
	// back rather than being skipped.
	matched := make([]GitLabIssue, 0, len(f.issues))
	for _, issue := range f.issues {
		if !issue.UpdatedAt.Before(*filter.UpdatedAfter) {
			matched = append(matched, issue)
		}
	}
	return matched, nil
}

func (f *gitLabFake) IssueDigests(context.Context) ([]board.IssueDigest, error) {
	digests := make([]board.IssueDigest, 0, len(f.issues))
	for _, issue := range f.issues {
		digests = append(digests, board.IssueDigest{IssueIID: issue.IssueIID, UpdatedAt: issue.UpdatedAt})
	}
	return digests, nil
}
func (f *gitLabFake) Issue(_ context.Context, issueIID int64) (GitLabIssue, error) {
	for _, issue := range f.issues {
		if issue.IssueIID == issueIID {
			return issue, nil
		}
	}
	return GitLabIssue{}, board.ErrCardNotFound
}
func (f *gitLabFake) ApplyIssue(_ context.Context, mutation IssueMutation, token string) (GitLabIssue, error) {
	f.applied = &mutation
	f.token = token
	return GitLabIssue{
		IssueIID: 42, GitLabIssueID: 420, Title: mutation.Title, Description: mutation.Description,
		Labels: mutation.Labels, AssigneeGitLabUserIDs: mutation.AssigneeGitLabUserIDs,
		StartDate: mutation.StartDate, DueDate: mutation.DueDate, State: "opened", GitLabStatusName: mutation.GitLabStatusName,
	}, nil
}

type repoFake struct {
	directory        directory.Snapshot
	board            appboard.Snapshot
	cards            []board.Card
	pending          *PendingOperation
	completed        bool
	webhook          *board.WebhookDelivery
	webhookCompleted bool
	reconciled       *board.Card
	observation      board.BoardObservation
	observations     []board.BoardObservation
	quarantined      []int64
	sweepStartedAt   time.Time
	watermark        *time.Time
}

func (f *repoFake) Snapshot(context.Context) (directory.Snapshot, error) { return f.directory, nil }
func (f *repoFake) Board(context.Context) (appboard.Snapshot, error)     { return f.board, nil }
func (f *repoFake) ReplaceDirectory(_ context.Context, snapshot directory.Snapshot) error {
	f.directory = snapshot
	return nil
}
func (f *repoFake) ApplyBoardObservation(_ context.Context, observation board.BoardObservation) error {
	f.cards = observation.Cards
	f.observation = observation
	f.observations = append(f.observations, observation)
	return nil
}
func (f *repoFake) EnsureBoardLists(context.Context, []board.List, time.Time) error { return nil }
func (f *repoFake) BoardCursor(context.Context) (board.SyncCursor, error) {
	return board.SyncCursor{Watermark: f.watermark}, nil
}
func (f *repoFake) SweepStartedAt(context.Context) (time.Time, error) {
	if f.sweepStartedAt.IsZero() {
		f.sweepStartedAt = time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	}
	return f.sweepStartedAt, nil
}
func (f *repoFake) QuarantineCard(_ context.Context, issueIID int64, _ time.Time, _ string, _ time.Time) error {
	f.quarantined = append(f.quarantined, issueIID)
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
func (*repoFake) EnqueueWebhook(context.Context, board.WebhookDelivery) (bool, error) {
	return false, nil
}
func (f *repoFake) ClaimWebhook(context.Context, time.Time) (board.WebhookDelivery, error) {
	if f.webhook == nil {
		return board.WebhookDelivery{}, board.ErrOperationNotFound
	}
	delivery := *f.webhook
	f.webhook = nil
	return delivery, nil
}
func (f *repoFake) CompleteWebhook(context.Context, string, time.Time) error {
	f.webhookCompleted = true
	return nil
}
func (*repoFake) FailWebhook(context.Context, board.WebhookDelivery, time.Time, string) error {
	return nil
}
func (f *repoFake) ReconcileIssue(_ context.Context, _ int64, card *board.Card, _ time.Time) (bool, error) {
	if card != nil {
		copy := *card
		f.reconciled = &copy
	}
	return true, nil
}

func TestRefreshDirectoryUsesRevisionAndRefreshesMembers(t *testing.T) {
	t.Parallel()
	gitlab := &gitLabFake{
		members: []directory.GitLabMember{{GitLabUserID: 1, Username: "alice", DisplayName: "Alice", State: directory.MemberActive}},
	}
	directorySource := &directorySourceFake{revision: "revision-1"}
	repo := &repoFake{}
	service := NewService(gitlab, directorySource, repo, actorTokensFake{}, nil, noop.NewTracerProvider().Tracer("test"))
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

func TestRefreshBoardMapsNativeStatusesAndSkipsUnknownTeams(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 14, 8, 0, 0, 0, time.UTC)
	gitlab := &gitLabFake{issues: []GitLabIssue{
		{IssueIID: 1, GitLabIssueID: 10, Title: "[開發組] 修正流程", Labels: []string{"Team::開發組"}, GitLabStatusName: "Doing", StartDate: "2026-07-17", State: "opened", UpdatedAt: now},
		{IssueIID: 2, GitLabIssueID: 20, Title: "[開發組] 待辦", Labels: []string{"Team::開發組"}, GitLabStatusName: "To do", State: "opened", UpdatedAt: now},
		{IssueIID: 3, GitLabIssueID: 30, Title: "[開發組] 等待中", Labels: []string{"Team::開發組"}, GitLabStatusName: "Waiting", State: "opened", UpdatedAt: now},
		{IssueIID: 4, GitLabIssueID: 40, Title: "[開發組] 已完成", Labels: []string{"Team::開發組"}, GitLabStatusName: "Done", State: "closed", UpdatedAt: now},
		{IssueIID: 5, GitLabIssueID: 50, Title: "無組別", Labels: nil, GitLabStatusName: "Inbox", State: "opened", UpdatedAt: now},
	}}
	repo := &repoFake{directory: directory.Snapshot{Teams: []directory.Team{{Key: "development", TitlePrefix: "[開發組]", GitLabLabel: "Team::開發組", Active: true}}}}
	service := NewService(gitlab, &directorySourceFake{}, repo, actorTokensFake{}, nil, noop.NewTracerProvider().Tracer("test"))
	if err := service.RefreshBoardDeep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.cards) != 4 || repo.cards[0].ListKey != "doing" || repo.cards[0].Title != "修正流程" || repo.cards[0].StartDate != "2026-07-17" ||
		repo.cards[1].ListKey != "todo" || repo.cards[2].ListKey != "wating" || repo.cards[3].ListKey != "closed" {
		t.Fatalf("cards = %#v", repo.cards)
	}
}

func TestUnmappedStatusQuarantinesOneCardWithoutFailingTheBatch(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 18, 8, 0, 0, 0, time.UTC)
	gitlab := &gitLabFake{issues: []GitLabIssue{
		{
			IssueIID: 9, GitLabIssueID: 90, Title: "[開發組] 取消工作",
			Labels: []string{"Team::開發組"}, GitLabStatusName: "Won't do", UpdatedAt: now,
		},
		{
			IssueIID: 10, GitLabIssueID: 100, Title: "[開發組] 正常工作",
			Labels: []string{"Team::開發組"}, GitLabStatusName: "Doing", UpdatedAt: now,
		},
	}}
	repo := &repoFake{directory: directory.Snapshot{Teams: []directory.Team{{
		Key: "development", TitlePrefix: "[開發組]", GitLabLabel: "Team::開發組", Active: true,
	}}}}
	service := NewService(gitlab, &directorySourceFake{}, repo, actorTokensFake{}, nil, noop.NewTracerProvider().Tracer("test"))

	if err := service.RefreshBoardDeep(context.Background()); err != nil {
		t.Fatalf("RefreshBoard() error = %v, want one bad issue not to fail the whole board", err)
	}
	if len(repo.cards) != 1 || repo.cards[0].IssueIID != 10 {
		t.Fatalf("merged cards = %#v, want only the mappable issue", repo.cards)
	}
	if len(repo.quarantined) != 1 || repo.quarantined[0] != 9 {
		t.Fatalf("quarantined = %v, want [9]", repo.quarantined)
	}
	// Retained keeps the unmappable card out of the prune set, so it stays on the
	// board as last known instead of vanishing while someone fixes GitLab.
	if len(repo.observation.Retained) != 1 || repo.observation.Retained[0] != 9 {
		t.Fatalf("retained = %v, want [9]", repo.observation.Retained)
	}
}

func TestRefreshBoardReadsTheSweepStartBeforeFetchingGitLab(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 18, 8, 0, 0, 0, time.UTC)
	gitlab := &gitLabFake{issues: []GitLabIssue{{
		IssueIID: 11, GitLabIssueID: 110, Title: "[開發組] 工作",
		Labels: []string{"Team::開發組"}, GitLabStatusName: "Doing", UpdatedAt: now,
	}}}
	repo := &repoFake{
		directory: directory.Snapshot{Teams: []directory.Team{{
			Key: "development", TitlePrefix: "[開發組]", GitLabLabel: "Team::開發組", Active: true,
		}}},
		sweepStartedAt: time.Date(2026, time.August, 18, 7, 59, 0, 0, time.UTC),
	}
	service := NewService(gitlab, &directorySourceFake{}, repo, actorTokensFake{}, nil, noop.NewTracerProvider().Tracer("test"))
	if err := service.RefreshBoardDeep(context.Background()); err != nil {
		t.Fatalf("RefreshBoard() error = %v", err)
	}
	if !repo.observation.StartedAt.Equal(repo.sweepStartedAt) {
		t.Fatalf("observation StartedAt = %v, want the PostgreSQL clock read %v", repo.observation.StartedAt, repo.sweepStartedAt)
	}
	if !repo.observation.StartedAt.Before(repo.observation.SyncedAt) {
		t.Fatalf("StartedAt %v must precede SyncedAt %v, or a concurrent webhook reconcile looks prunable",
			repo.observation.StartedAt, repo.observation.SyncedAt)
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
	service := NewService(gitlab, &directorySourceFake{}, repo, actorTokensFake{}, nil, noop.NewTracerProvider().Tracer("test"))
	processed, err := service.ProcessOne(context.Background())
	if err != nil || !processed || !repo.completed {
		t.Fatalf("ProcessOne() = %v, %v, completed=%v", processed, err, repo.completed)
	}
	if gitlab.applied == nil || gitlab.applied.Title != "[開發組] 修正流程" || gitlab.applied.Description != "詳細規劃" ||
		gitlab.applied.StartDate != "2026-07-17" || gitlab.applied.DueDate != "2026-07-21" ||
		!slices.Equal(gitlab.applied.AssigneeGitLabUserIDs, []int64{1, 2}) || !slices.Equal(gitlab.applied.Labels, []string{"security", "組別::開發"}) || gitlab.applied.GitLabStatusName != "Doing" {
		t.Fatalf("mutation = %#v", gitlab.applied)
	}
	if gitlab.token != "actor-token" {
		t.Fatalf("actor token = %q", gitlab.token)
	}
}

func TestProcessWebhookFetchesCanonicalIssueAndReconcilesCard(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 24, 8, 0, 0, 0, time.UTC)
	iid := int64(42)
	gitlab := &gitLabFake{issues: []GitLabIssue{{
		IssueIID: iid, GitLabIssueID: 420, Title: "[開發組] 即時同步", Description: "canonical",
		Labels: []string{"組別::開發"}, GitLabStatusName: "Doing", State: "opened", CreatedAt: now, UpdatedAt: now,
	}}}
	repo := &repoFake{
		directory: directory.Snapshot{Teams: []directory.Team{{Key: "development", TitlePrefix: "[開發組]", GitLabLabel: "組別::開發", Active: true}}},
		webhook:   &board.WebhookDelivery{ID: "delivery-42", EventKind: "issue", IssueIID: &iid},
	}
	service := NewService(gitlab, &directorySourceFake{}, repo, actorTokensFake{}, nil, noop.NewTracerProvider().Tracer("test"))
	processed, err := service.ProcessWebhookOne(context.Background())
	if err != nil || !processed || !repo.webhookCompleted || repo.reconciled == nil {
		t.Fatalf("ProcessWebhookOne() = %v, %v, completed=%v card=%#v", processed, err, repo.webhookCompleted, repo.reconciled)
	}
	if repo.reconciled.Title != "即時同步" || repo.reconciled.ListKey != "doing" || repo.reconciled.Description != "canonical" {
		t.Fatalf("reconciled card = %#v", repo.reconciled)
	}
}
