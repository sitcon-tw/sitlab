package sync

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/directory"
)

var DefaultBoardLists = []board.List{
	{Key: "todo", Name: "待處理", GitLabLabel: "Todo", Position: 0, Color: "#64748b"},
	{Key: "doing", Name: "進行中", GitLabLabel: "Doing", Position: 1, Color: "#2563eb"},
	{Key: "review", Name: "待確認", GitLabLabel: "Review", Position: 2, Color: "#b45309"},
	{Key: "closed", Name: "已完成", GitLabLabel: "Closed", Position: 3, Closed: true, Color: "#15803d"},
}

type Service struct {
	gitlab  GitLab
	repo    Repository
	log     MissingMemberLogger
	now     func() time.Time
	tracer  trace.Tracer
	refresh chan struct{}
}

func NewService(gitlab GitLab, repo Repository, log MissingMemberLogger, tracer trace.Tracer) *Service {
	return &Service{gitlab: gitlab, repo: repo, log: log, now: time.Now, tracer: tracer, refresh: make(chan struct{}, 1)}
}

func (s *Service) RefreshDirectory(ctx context.Context) error {
	ctx, span := s.tracer.Start(ctx, "sync.directory")
	defer span.End()
	now := s.now().UTC()
	revision, err := s.gitlab.DirectoryRevision(ctx)
	if err != nil {
		s.recordFailure(ctx, "directory", now, err)
		return technical(span, "load directory revision", err)
	}
	current, currentErr := s.repo.Snapshot(ctx)
	var file directory.File
	if currentErr == nil && current.SourceRevision == revision {
		file = directoryFileFromSnapshot(current)
	} else {
		file, revision, err = s.gitlab.DirectoryFile(ctx)
		if err != nil {
			s.recordFailure(ctx, "directory", now, err)
			return technical(span, "load directory file", err)
		}
	}
	members, err := s.gitlab.ProjectMembers(ctx)
	if err != nil {
		s.recordFailure(ctx, "members", now, err)
		return technical(span, "load project members", err)
	}
	snapshot, missing, err := directory.Normalize(file, members, revision, now)
	if err != nil {
		s.recordFailure(ctx, "directory", now, err)
		return technical(span, "normalize directory", err)
	}
	for _, member := range missing {
		if s.log != nil {
			s.log.DirectoryMemberMissing(member.TeamKey, member.Username)
		}
	}
	if err := s.repo.ReplaceDirectory(ctx, snapshot); err != nil {
		return technical(span, "replace directory snapshot", err)
	}
	return nil
}

func (s *Service) RefreshBoard(ctx context.Context) error {
	ctx, span := s.tracer.Start(ctx, "sync.board")
	defer span.End()
	now := s.now().UTC()
	directorySnapshot, err := s.repo.Snapshot(ctx)
	if err != nil {
		return technical(span, "load directory for board", err)
	}
	issues, err := s.gitlab.Issues(ctx)
	if err != nil {
		s.recordFailure(ctx, "board", now, err)
		return technical(span, "load GitLab issues", err)
	}
	cards := make([]board.Card, 0, len(issues))
	positions := make(map[string]int32)
	for _, issue := range issues {
		card, ok := mapIssue(issue, directorySnapshot, DefaultBoardLists, positions)
		if !ok {
			continue
		}
		cards = append(cards, card)
	}
	revision := boardRevision(issues, now)
	if err := s.repo.ReplaceBoard(ctx, DefaultBoardLists, cards, revision, now); err != nil {
		return technical(span, "replace board snapshot", err)
	}
	return nil
}

func (s *Service) InitialSync(ctx context.Context) error {
	if err := s.RefreshDirectory(ctx); err != nil {
		return err
	}
	return s.RefreshBoard(ctx)
}

func (s *Service) ProcessOne(ctx context.Context) (bool, error) {
	ctx, span := s.tracer.Start(ctx, "sync.operation")
	defer span.End()
	now := s.now().UTC()
	pending, err := s.repo.ClaimOperation(ctx, now)
	if errors.Is(err, board.ErrOperationNotFound) {
		return false, nil
	}
	if err != nil {
		return false, technical(span, "claim durable operation", err)
	}
	directorySnapshot, err := s.repo.Snapshot(ctx)
	if err != nil {
		s.failOperation(ctx, pending, now, "SNAPSHOT_NOT_READY", err)
		return true, technical(span, "load operation directory", err)
	}
	boardSnapshot, err := s.repo.Board(ctx)
	if err != nil {
		s.failOperation(ctx, pending, now, "SNAPSHOT_NOT_READY", err)
		return true, technical(span, "load operation board", err)
	}
	team, ok := directorySnapshot.Team(pending.Card.TeamKey)
	if !ok {
		err := board.ErrTeamNotFound
		s.failOperation(ctx, pending, now, "TEAM_NOT_FOUND", err)
		return true, err
	}
	list, ok := boardList(boardSnapshot.Lists, pending.Card.ListKey)
	if !ok {
		err := board.ErrListNotFound
		s.failOperation(ctx, pending, now, "LIST_NOT_FOUND", err)
		return true, err
	}
	mutation := IssueMutation{
		Create:               pending.Operation.Kind == board.OperationCreateCard,
		IssueIID:             pending.Card.IssueIID,
		Title:                board.ComposeGitLabTitle(team.TitlePrefix, pending.Card.Title),
		Labels:               canonicalLabels(pending.Card.Labels, team, list, directorySnapshot.Teams, boardSnapshot.Lists),
		AssigneeGitLabUserID: pending.Card.AssigneeGitLabUserID,
		DueDate:              pending.Card.DueDate, Closed: list.Closed,
	}
	issue, err := s.gitlab.ApplyIssue(ctx, mutation)
	if err != nil {
		s.failOperation(ctx, pending, now, "GITLAB_SYNC_FAILED", err)
		return true, technical(span, "apply GitLab issue mutation", err)
	}
	if err := s.repo.CompleteOperation(ctx, pending, issue, now); err != nil {
		return true, technical(span, "complete durable operation", err)
	}
	return true, nil
}

func (s *Service) RunOperations(ctx context.Context, pollInterval time.Duration) {
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		processed, _ := s.ProcessOne(ctx)
		if processed {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) Run(ctx context.Context, directoryInterval, boardInterval time.Duration) {
	if directoryInterval <= 0 {
		directoryInterval = 5 * time.Minute
	}
	if boardInterval <= 0 {
		boardInterval = 30 * time.Second
	}
	directoryTicker := time.NewTicker(directoryInterval)
	boardTicker := time.NewTicker(boardInterval)
	defer directoryTicker.Stop()
	defer boardTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.refresh:
			_ = s.InitialSync(ctx)
		case <-directoryTicker.C:
			_ = s.RefreshDirectory(ctx)
		case <-boardTicker.C:
			_ = s.RefreshBoard(ctx)
		}
	}
}

func (s *Service) RequestRefresh() time.Time {
	requestedAt := s.now().UTC()
	select {
	case s.refresh <- struct{}{}:
	default:
	}
	return requestedAt
}

func directoryFileFromSnapshot(snapshot directory.Snapshot) directory.File {
	file := directory.File{Version: 1, Teams: make([]directory.TeamConfig, 0, len(snapshot.Teams))}
	for _, team := range snapshot.Teams {
		file.Teams = append(file.Teams, directory.TeamConfig{
			Key: team.Key, Name: team.Name, TitlePrefix: team.TitlePrefix,
			GitLabLabel: team.GitLabLabel, Active: team.Active,
			Members: append([]string(nil), team.DirectoryMemberUsernames...),
		})
	}
	return file
}

func mapIssue(issue GitLabIssue, directorySnapshot directory.Snapshot, lists []board.List, positions map[string]int32) (board.Card, bool) {
	team, ok := issueTeam(issue.Labels, directorySnapshot.Teams)
	if !ok {
		return board.Card{}, false
	}
	list := lists[0]
	if issue.State == "closed" {
		for _, candidate := range lists {
			if candidate.Closed {
				list = candidate
				break
			}
		}
	} else {
		for _, candidate := range lists {
			if !candidate.Closed && slices.Contains(issue.Labels, candidate.GitLabLabel) {
				list = candidate
				break
			}
		}
	}
	title := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(issue.Title), team.TitlePrefix))
	position := positions[list.Key]
	positions[list.Key]++
	return board.Card{
		IssueIID: issue.IssueIID, GitLabIssueID: &issue.GitLabIssueID,
		Title: title, WebURL: issue.WebURL, ListKey: list.Key, Position: position,
		TeamKey: team.Key, AssigneeGitLabUserID: issue.AssigneeGitLabUserID,
		DueDate: issue.DueDate, Labels: append([]string(nil), issue.Labels...),
		SyncState: board.OperationSynced, UpdatedAt: issue.UpdatedAt.UTC(),
	}, true
}

func issueTeam(labels []string, teams []directory.Team) (directory.Team, bool) {
	for _, team := range teams {
		if team.Active && slices.Contains(labels, team.GitLabLabel) {
			return team, true
		}
	}
	return directory.Team{}, false
}

func boardRevision(issues []GitLabIssue, now time.Time) string {
	latest := time.Time{}
	for _, issue := range issues {
		if issue.UpdatedAt.After(latest) {
			latest = issue.UpdatedAt
		}
	}
	if latest.IsZero() {
		latest = now
	}
	return fmt.Sprintf("%s:%d", latest.UTC().Format(time.RFC3339Nano), len(issues))
}

func (s *Service) recordFailure(ctx context.Context, resource string, at time.Time, err error) {
	_ = s.repo.RecordSyncFailure(ctx, resource, at, err.Error())
}

func (s *Service) failOperation(ctx context.Context, pending PendingOperation, at time.Time, code string, cause error) {
	_ = s.repo.FailOperation(ctx, pending, at, code, cause.Error())
}

func boardList(lists []board.List, key string) (board.List, bool) {
	for _, list := range lists {
		if list.Key == key {
			return list, true
		}
	}
	return board.List{}, false
}

func canonicalLabels(existing []string, team directory.Team, list board.List, teams []directory.Team, lists []board.List) []string {
	reserved := make(map[string]struct{}, len(teams)+len(lists))
	for _, candidate := range teams {
		reserved[candidate.GitLabLabel] = struct{}{}
	}
	for _, candidate := range lists {
		reserved[candidate.GitLabLabel] = struct{}{}
	}
	labels := make([]string, 0, len(existing)+2)
	for _, label := range existing {
		if _, isReserved := reserved[label]; !isReserved {
			labels = append(labels, label)
		}
	}
	labels = append(labels, team.GitLabLabel)
	if !list.Closed {
		labels = append(labels, list.GitLabLabel)
	}
	return labels
}

func technical(span trace.Span, action string, err error) error {
	span.RecordError(err)
	span.SetStatus(codes.Error, action)
	return fmt.Errorf("%s: %w", action, err)
}
