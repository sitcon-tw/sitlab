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
	{Key: "wating", Name: "Waiting", GitLabStatusName: "Waiting", Position: 0, Color: "#d2ad46"},
	{Key: "inbox", Name: "Inbox", GitLabStatusName: "Inbox", Position: 1, Color: "#6699cc"},
	{Key: "todo", Name: "To do", GitLabStatusName: "To do", Position: 2, Color: "#ed9121"},
	{Key: "doing", Name: "Doing", GitLabStatusName: "Doing", Position: 3, Color: "#1f75cb"},
	{Key: "review", Name: "Review", GitLabStatusName: "Review", Position: 4, Color: "#7a07ab"},
	{Key: "closed", Name: "Done", GitLabStatusName: "Done", Position: 5, Closed: true, Color: "#108548"},
}

type Service struct {
	gitlab    GitLab
	directory DirectorySource
	repo      Repository
	actors    ActorTokens
	log       MissingMemberLogger
	now       func() time.Time
	tracer    trace.Tracer
	refresh   chan struct{}
	webhook   chan struct{}
	observer  WebhookObserver

	// deltaOverlap widens each incremental read backwards so a row that committed
	// before the watermark but only became visible after it is not lost. Run overrides
	// the default from configuration.
	deltaOverlap time.Duration
}

// Intervals paces the background reads. The three board tiers answer different
// questions and so run at different rates: the delta keeps content fresh, the presence
// sweep is the only thing that can notice a deletion, and the deep sweep is the
// backstop for drift that changes nothing observable.
type Intervals struct {
	Directory     time.Duration
	BoardDelta    time.Duration
	BoardPresence time.Duration
	BoardDeep     time.Duration
	DeltaOverlap  time.Duration
	MaxBackoff    time.Duration
}

// backoff holds a failing read off the wire instead of hammering GitLab every tick.
type backoff struct {
	failures int
	until    time.Time
	max      time.Duration
}

func (b *backoff) ready(now time.Time) bool { return !now.Before(b.until) }

func (b *backoff) record(now time.Time, err error) {
	if err == nil {
		b.failures, b.until = 0, time.Time{}
		return
	}
	b.failures++
	delay := time.Second << min(b.failures-1, 8)
	b.until = now.Add(min(delay, b.max))
}

type WebhookObserver interface {
	WebhookProcessed(kind, result string, duration time.Duration)
}

func NewService(gitlab GitLab, directory DirectorySource, repo Repository, actors ActorTokens, log MissingMemberLogger, tracer trace.Tracer) *Service {
	return &Service{
		gitlab: gitlab, directory: directory, repo: repo, actors: actors, log: log, now: time.Now, tracer: tracer,
		refresh: make(chan struct{}, 1), webhook: make(chan struct{}, 1),
		deltaOverlap: 2 * time.Minute,
	}
}

func (s *Service) SetWebhookObserver(observer WebhookObserver) { s.observer = observer }

func (s *Service) RefreshDirectory(ctx context.Context) error {
	ctx, span := s.tracer.Start(ctx, "sync.directory")
	defer span.End()
	now := s.now().UTC()
	revision, err := s.directory.DirectoryRevision(ctx)
	if err != nil {
		s.recordFailure(ctx, "directory", now, err)
		return technical(span, "load directory revision", err)
	}
	current, currentErr := s.repo.Snapshot(ctx)
	var file directory.File
	if currentErr == nil && current.SourceRevision == revision {
		file = directoryFileFromSnapshot(current)
	} else {
		file, revision, err = s.directory.DirectoryFile(ctx)
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

// RefreshBoardDelta reads only what GitLab changed since the last successful read.
//
// It walks UPDATED_ASC: updated_at only moves forward, so an issue edited mid-
// pagination can only jump past the cursor, never behind it. Rows can repeat, which is
// harmless because the merge no-ops on unchanged content.
func (s *Service) RefreshBoardDelta(ctx context.Context) error {
	ctx, span := s.tracer.Start(ctx, "sync.board.delta")
	defer span.End()
	cursor, err := s.repo.BoardCursor(ctx)
	if err != nil {
		return technical(span, "load board cursor", err)
	}
	if cursor.Watermark == nil {
		// Nothing has been read yet, so there is no boundary to read forward from.
		return s.RefreshBoardDeep(ctx)
	}
	// Overlap, even though updatedAfter is inclusive. Inclusivity only covers rows we
	// could see; it does nothing for a row that committed at T but did not become
	// visible until T+n, behind a long transaction or a lagging read replica. Without
	// the window that row falls behind the watermark and is lost for good.
	since := cursor.Watermark.Add(-s.deltaOverlap)
	now := s.now().UTC()
	issues, err := s.gitlab.Issues(ctx, IssueFilter{UpdatedAfter: &since, Order: IssueOrderUpdatedAsc})
	if err != nil {
		s.recordFailure(ctx, "board", now, err)
		return technical(span, "read GitLab board delta", err)
	}
	watermark := *cursor.Watermark
	for _, issue := range issues {
		if issue.UpdatedAt.After(watermark) {
			watermark = issue.UpdatedAt.UTC()
		}
	}
	return s.applyIssues(ctx, span, boardRead{
		issues: issues, removeUnmapped: true, watermark: &watermark, now: now,
	})
}

// RefreshBoardPresence enumerates which issues still exist, without paying for their
// content. It is the only thing that can notice a deletion: GitLab has no webhook for
// one, and an incremental read can never observe an absence.
func (s *Service) RefreshBoardPresence(ctx context.Context) error {
	ctx, span := s.tracer.Start(ctx, "sync.board.presence")
	defer span.End()
	now := s.now().UTC()
	startedAt, err := s.repo.SweepStartedAt(ctx)
	if err != nil {
		return technical(span, "read presence sweep start", err)
	}
	digests, err := s.gitlab.IssueDigests(ctx)
	if err != nil {
		s.recordFailure(ctx, "board", now, err)
		return technical(span, "read GitLab issue digests", err)
	}
	present := make([]int64, 0, len(digests))
	for _, digest := range digests {
		present = append(present, digest.IssueIID)
	}
	// Every issue is Retained: this read confirms existence and nothing else, so it
	// prunes what GitLab no longer lists and writes nothing. Content drift is the
	// delta read's job, and the deep sweep's.
	if err := s.repo.ApplyBoardObservation(ctx, board.BoardObservation{
		Retained: present, Complete: true, StartedAt: startedAt, SyncedAt: now,
	}); err != nil {
		return technical(span, "apply board presence sweep", err)
	}
	return nil
}

// RefreshBoardDeep re-reads every issue in full. It is the backstop for drift that
// changes nothing observable: a renamed Team:: label, a reconfigured lifecycle, a
// webhook that was dead-lettered.
//
// It walks CREATED_ASC because created_at never changes, so a full enumeration under
// concurrent edits yields neither skips nor duplicates.
func (s *Service) RefreshBoardDeep(ctx context.Context) error {
	ctx, span := s.tracer.Start(ctx, "sync.board.deep")
	defer span.End()
	now := s.now().UTC()
	startedAt, err := s.repo.SweepStartedAt(ctx)
	if err != nil {
		return technical(span, "read deep sweep start", err)
	}
	issues, err := s.gitlab.Issues(ctx, IssueFilter{Order: IssueOrderCreatedAsc})
	if err != nil {
		s.recordFailure(ctx, "board", now, err)
		return technical(span, "read GitLab board", err)
	}
	watermark := time.Time{}
	for _, issue := range issues {
		if issue.UpdatedAt.After(watermark) {
			watermark = issue.UpdatedAt.UTC()
		}
	}
	read := boardRead{issues: issues, complete: true, startedAt: startedAt, now: now}
	if !watermark.IsZero() {
		read.watermark = &watermark
	}
	return s.applyIssues(ctx, span, read)
}

type boardRead struct {
	issues []GitLabIssue
	// complete marks a read that enumerated the whole project.
	complete bool
	// removeUnmapped turns an issue that no longer carries an active Team:: label into
	// an explicit deletion. A complete read does not need it -- omission already means
	// gone -- but an incremental one does, where omission means unchanged.
	removeUnmapped bool
	watermark      *time.Time
	startedAt      time.Time
	now            time.Time
}

func (s *Service) applyIssues(ctx context.Context, span trace.Span, read boardRead) error {
	directorySnapshot, err := s.repo.Snapshot(ctx)
	if err != nil {
		return technical(span, "load directory for board", err)
	}
	cards := make([]board.Card, 0, len(read.issues))
	retained := make([]int64, 0)
	removed := make([]int64, 0)
	positions := make(map[string]int32)
	for _, issue := range read.issues {
		card, included, mapErr := mapIssue(issue, directorySnapshot, DefaultBoardLists, positions)
		if mapErr != nil {
			// One issue set to a status no lane maps to used to abort the whole
			// refresh, so a single mis-set issue froze every card and showed the
			// entire team an offline board. Quarantine it instead and keep going;
			// the card stays as we last knew it until someone moves the issue on.
			// A lifecycle genuinely missing a required status is a different
			// failure and still fails the GitLab read outright.
			retained = append(retained, issue.IssueIID)
			if quarantineErr := s.repo.QuarantineCard(ctx, issue.IssueIID, issue.UpdatedAt.UTC(), mapErr.Error(), read.now); quarantineErr != nil {
				return technical(span, "quarantine unmapped GitLab issue", quarantineErr)
			}
			span.RecordError(mapErr)
			continue
		}
		if !included {
			if read.removeUnmapped {
				removed = append(removed, issue.IssueIID)
			}
			continue
		}
		cards = append(cards, card)
	}
	if err := s.repo.ApplyBoardObservation(ctx, board.BoardObservation{
		Cards: cards, Retained: retained, Removed: removed, Complete: read.complete,
		Watermark: read.watermark, StartedAt: read.startedAt, SyncedAt: read.now,
	}); err != nil {
		return technical(span, "apply board observation", err)
	}
	return nil
}

func (s *Service) InitialSync(ctx context.Context) error {
	if err := s.RefreshDirectory(ctx); err != nil {
		return err
	}
	if err := s.repo.EnsureBoardLists(ctx, DefaultBoardLists, s.now().UTC()); err != nil {
		return err
	}
	return s.RefreshBoardDeep(ctx)
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
		Create:                pending.Operation.Kind == board.OperationCreateCard,
		IssueIID:              pending.Card.IssueIID,
		Title:                 board.ComposeGitLabTitle(team.TitlePrefix, pending.Card.Title),
		Description:           pending.Card.Description,
		Labels:                canonicalLabels(pending.Card.Labels, team, list, directorySnapshot.Teams, boardSnapshot.Lists),
		AssigneeGitLabUserIDs: append([]int64(nil), pending.Card.AssigneeGitLabUserIDs...),
		StartDate:             pending.Card.StartDate, DueDate: pending.Card.DueDate, GitLabStatusName: list.GitLabStatusName,
	}
	if s.actors == nil {
		err := errors.New("GitLab authorization is unavailable; sign out and sign in again")
		s.failOperation(ctx, pending, now, "GITLAB_REAUTH_REQUIRED", err)
		return true, technical(span, "load actor GitLab authorization", err)
	}
	actorAccessToken, err := s.actors.AccessToken(ctx, pending.RequestedByUserID)
	if err != nil {
		reauthErr := fmt.Errorf("GitLab authorization is unavailable; sign out and sign in again: %w", err)
		s.failOperation(ctx, pending, now, "GITLAB_REAUTH_REQUIRED", reauthErr)
		return true, technical(span, "load actor GitLab authorization", reauthErr)
	}
	issue, err := s.gitlab.ApplyIssue(ctx, mutation, actorAccessToken)
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

func (s *Service) EnqueueWebhook(ctx context.Context, delivery board.WebhookDelivery) (bool, error) {
	duplicate, err := s.repo.EnqueueWebhook(ctx, delivery)
	if err != nil {
		return false, err
	}
	if !duplicate {
		select {
		case s.webhook <- struct{}{}:
		default:
		}
	}
	return duplicate, nil
}

func (s *Service) ProcessWebhookOne(ctx context.Context) (bool, error) {
	ctx, span := s.tracer.Start(ctx, "sync.webhook")
	defer span.End()
	now := s.now().UTC()
	delivery, err := s.repo.ClaimWebhook(ctx, now)
	if errors.Is(err, board.ErrOperationNotFound) {
		return false, nil
	}
	if err != nil {
		return false, technical(span, "claim GitLab webhook", err)
	}
	startedAt := time.Now()
	metricResult := "failed"
	defer func() {
		if s.observer != nil {
			s.observer.WebhookProcessed(delivery.EventKind, metricResult, time.Since(startedAt))
		}
	}()
	if delivery.EventKind == "member" {
		err = s.RefreshDirectory(ctx)
	} else if delivery.EventKind == "issue" && delivery.IssueIID != nil {
		err = s.reconcileWebhookIssue(ctx, *delivery.IssueIID, now)
	} else {
		err = fmt.Errorf("unsupported webhook delivery kind %q", delivery.EventKind)
	}
	if err != nil {
		_ = s.repo.FailWebhook(ctx, delivery, now, err.Error())
		if delivery.Attempts >= 10 {
			metricResult = "dead"
		} else {
			metricResult = "retry"
		}
		return true, technical(span, "process GitLab webhook", err)
	}
	if err := s.repo.CompleteWebhook(ctx, delivery.ID, now); err != nil {
		return true, technical(span, "complete GitLab webhook", err)
	}
	metricResult = "completed"
	return true, nil
}

func (s *Service) reconcileWebhookIssue(ctx context.Context, issueIID int64, now time.Time) error {
	issue, err := s.gitlab.Issue(ctx, issueIID)
	if errors.Is(err, board.ErrCardNotFound) {
		_, reconcileErr := s.repo.ReconcileIssue(ctx, issueIID, nil, now)
		return reconcileErr
	}
	if err != nil {
		return err
	}
	directorySnapshot, err := s.repo.Snapshot(ctx)
	if err != nil {
		return err
	}
	missingAssignee := false
	for _, assigneeID := range issue.AssigneeGitLabUserIDs {
		if !directorySnapshot.IsAssignable(assigneeID) {
			missingAssignee = true
			break
		}
	}
	if missingAssignee {
		if err := s.RefreshDirectory(ctx); err != nil {
			return err
		}
		directorySnapshot, err = s.repo.Snapshot(ctx)
		if err != nil {
			return err
		}
	}
	card, included, mapErr := mapIssue(issue, directorySnapshot, DefaultBoardLists, make(map[string]int32))
	if mapErr != nil {
		return mapErr
	}
	if !included {
		_, err = s.repo.ReconcileIssue(ctx, issueIID, nil, now)
		return err
	}
	_, err = s.repo.ReconcileIssue(ctx, issueIID, &card, now)
	return err
}

func (s *Service) RunWebhooks(ctx context.Context, pollInterval time.Duration) {
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		processed, _ := s.ProcessWebhookOne(ctx)
		if processed {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-s.webhook:
		case <-ticker.C:
		}
	}
}

func (s *Service) Run(ctx context.Context, intervals Intervals) {
	if intervals.Directory <= 0 {
		intervals.Directory = 5 * time.Minute
	}
	if intervals.BoardDelta <= 0 {
		intervals.BoardDelta = 10 * time.Second
	}
	if intervals.BoardPresence <= 0 {
		intervals.BoardPresence = 5 * time.Minute
	}
	if intervals.BoardDeep <= 0 {
		intervals.BoardDeep = time.Hour
	}
	if intervals.MaxBackoff <= 0 {
		intervals.MaxBackoff = 5 * time.Minute
	}
	if intervals.DeltaOverlap > 0 {
		s.deltaOverlap = intervals.DeltaOverlap
	}
	directoryTicker := time.NewTicker(intervals.Directory)
	deltaTicker := time.NewTicker(intervals.BoardDelta)
	presenceTicker := time.NewTicker(intervals.BoardPresence)
	deepTicker := time.NewTicker(intervals.BoardDeep)
	defer directoryTicker.Stop()
	defer deltaTicker.Stop()
	defer presenceTicker.Stop()
	defer deepTicker.Stop()
	// One gate for all three board tiers. They read the same GitLab project, so a
	// failing delta should hold the sweeps back too rather than each tier discovering
	// the outage on its own schedule.
	board := &backoff{max: intervals.MaxBackoff}
	run := func(read func(context.Context) error) {
		now := s.now().UTC()
		if !board.ready(now) {
			return
		}
		board.record(now, read(ctx))
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.refresh:
			_ = s.InitialSync(ctx)
		case <-directoryTicker.C:
			_ = s.RefreshDirectory(ctx)
		case <-deltaTicker.C:
			run(s.RefreshBoardDelta)
		case <-presenceTicker.C:
			run(s.RefreshBoardPresence)
		case <-deepTicker.C:
			run(s.RefreshBoardDeep)
		}
	}
}

// DeltaQuery asks for everything recorded after a checkpoint that this user may see.
type DeltaQuery struct {
	Since          string
	AudienceUserID string
	Limit          int
}

// Delta serves both the catch-up endpoint and the event stream, so the checkpoint
// guards cannot diverge between them.
func (s *Service) Delta(ctx context.Context, query DeltaQuery) (board.SyncDelta, error) {
	return s.repo.SyncActions(ctx, query.Since, query.AudienceUserID, query.Limit)
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
			Leaders: append([]string(nil), team.DirectoryLeaderUsernames...),
		})
	}
	return file
}

func mapIssue(issue GitLabIssue, directorySnapshot directory.Snapshot, lists []board.List, positions map[string]int32) (board.Card, bool, error) {
	team, ok := issueTeam(issue.Labels, directorySnapshot.Teams)
	if !ok {
		return board.Card{}, false, nil
	}
	list, ok := boardListByStatus(lists, issue.GitLabStatusName)
	if !ok {
		return board.Card{}, false, fmt.Errorf("issue !%d uses unmapped GitLab status %q", issue.IssueIID, issue.GitLabStatusName)
	}
	title := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(issue.Title), team.TitlePrefix))
	position := positions[list.Key]
	positions[list.Key]++
	return board.Card{
		IssueIID: issue.IssueIID, GitLabIssueID: &issue.GitLabIssueID,
		Title: title, Description: issue.Description, WebURL: issue.WebURL, ListKey: list.Key, Position: position,
		TeamKey: team.Key, AssigneeGitLabUserIDs: append([]int64(nil), issue.AssigneeGitLabUserIDs...),
		StartDate: issue.StartDate, DueDate: issue.DueDate, Labels: append([]string(nil), issue.Labels...), GitLabStatusName: issue.GitLabStatusName,
		SyncState: board.OperationSynced, CreatedAt: issue.CreatedAt.UTC(), UpdatedAt: issue.UpdatedAt.UTC(),
	}, true, nil
}

func issueTeam(labels []string, teams []directory.Team) (directory.Team, bool) {
	for _, team := range teams {
		if team.Active && slices.Contains(labels, team.GitLabLabel) {
			return team, true
		}
	}
	return directory.Team{}, false
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

func boardListByStatus(lists []board.List, statusName string) (board.List, bool) {
	for _, list := range lists {
		if strings.EqualFold(list.GitLabStatusName, statusName) {
			return list, true
		}
	}
	return board.List{}, false
}

func canonicalLabels(existing []string, team directory.Team, _ board.List, teams []directory.Team, _ []board.List) []string {
	teamLabels := make([]string, 0, len(teams))
	for _, candidate := range teams {
		teamLabels = append(teamLabels, candidate.GitLabLabel)
	}
	return board.CanonicalLabels(existing, team.GitLabLabel, teamLabels)
}

func technical(span trace.Span, action string, err error) error {
	span.RecordError(err)
	span.SetStatus(codes.Error, action)
	return fmt.Errorf("%s: %w", action, err)
}
