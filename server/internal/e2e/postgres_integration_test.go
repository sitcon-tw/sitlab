//go:build integration

package e2e_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"go.opentelemetry.io/otel/trace/noop"

	appboard "example.com/project-template/internal/controller/application/board"
	appdirectory "example.com/project-template/internal/controller/application/directory"
	appsync "example.com/project-template/internal/controller/application/sync"
	"example.com/project-template/internal/controller/infrastructure/postgres"
	pgoauth "example.com/project-template/internal/controller/infrastructure/postgres/oauth"
	pgsitcon "example.com/project-template/internal/controller/infrastructure/postgres/sitcon"
	domainboard "example.com/project-template/internal/domain/board"
	domaindirectory "example.com/project-template/internal/domain/directory"
	"example.com/project-template/internal/domain/identity"
)

func TestPostgresSnapshotsOperationsAndRollingSessions(t *testing.T) {
	databaseURL := os.Getenv("SITCON_BOARD_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SITCON_BOARD_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(db, migrationDirectory(t)); err != nil {
		t.Fatalf("migrate empty database: %v", err)
	}

	pool, err := postgres.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `
		TRUNCATE gitlab_webhook_deliveries, durable_operations, issue_cache, board_lists, user_preferences,
		         directory_team_memberships, directory_members, directory_teams,
		         sync_snapshots, oauth_states, auth_sessions, users
		RESTART IDENTITY CASCADE
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE realtime_state SET revision = 1, updated_at = now() WHERE topic = 'bootstrap'`); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, time.July, 14, 8, 0, 0, 0, time.UTC)
	oauthRepo := pgoauth.New(pool)
	store := pgsitcon.New(pool)
	webhookIssueIID := int64(42)
	delivery := domainboard.WebhookDelivery{
		ID: "integration-delivery", Scope: "project", EventKind: "issue", EventName: "Issue Hook",
		IssueIID: &webhookIssueIID, ReceivedAt: now,
	}
	duplicate, err := store.EnqueueWebhook(ctx, delivery)
	if err != nil || duplicate {
		t.Fatalf("enqueue webhook = duplicate %v, error %v", duplicate, err)
	}
	duplicate, err = store.EnqueueWebhook(ctx, delivery)
	if err != nil || !duplicate {
		t.Fatalf("duplicate webhook = duplicate %v, error %v", duplicate, err)
	}
	claimed, err := store.ClaimWebhook(ctx, now)
	if err != nil || claimed.ID != delivery.ID || claimed.Attempts != 1 {
		t.Fatalf("claim webhook = %#v, error %v", claimed, err)
	}
	if err := store.CompleteWebhook(ctx, delivery.ID, now); err != nil {
		t.Fatalf("complete webhook: %v", err)
	}
	user, err := oauthRepo.UpsertUser(ctx, identity.User{
		ID: uuid.NewString(), GitLabUserID: 101, Username: "alice", DisplayName: "Alice",
		ProfileURL: "https://gitlab.com/alice", AccessLevel: 40, CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	credential := identity.OAuthCredential{
		UserID: user.ID, AccessTokenCiphertext: []byte("sealed-access"), RefreshTokenCiphertext: []byte("sealed-refresh"),
		ExpiresAt: now.Add(time.Hour), UpdatedAt: now,
	}
	if err := oauthRepo.UpsertOAuthCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}
	storedCredential, err := oauthRepo.OAuthCredential(ctx, user.ID)
	if err != nil || string(storedCredential.AccessTokenCiphertext) != "sealed-access" {
		t.Fatalf("OAuth credential = %#v, error %v", storedCredential, err)
	}
	seedSnapshots(t, ctx, pool, now)
	verifyCardReordering(t, ctx, pool, store, user.ID, now)
	verifySnapshotMerge(t, ctx, pool, store, now)

	listenerCtx, stopListener := context.WithCancel(ctx)
	updates, unsubscribe := store.SubscribeRevisions()
	go store.RunRevisionListener(listenerCtx)
	select {
	case <-updates:
	case <-time.After(5 * time.Second):
		t.Fatal("revision listener did not become ready")
	}
	externalIssueID := int64(770)
	external := domainboard.Card{
		IssueIID: 77, GitLabIssueID: &externalIssueID, Title: "外部更新", Description: "GitLab canonical",
		WebURL: "https://gitlab.com/sitcon-tw/2027/-/issues/77", ListKey: "doing", TeamKey: "development",
		AssigneeGitLabUserIDs: []int64{101}, Labels: []string{"Team::開發組"}, GitLabStatusName: "Doing",
		SyncState: domainboard.OperationSynced, CreatedAt: now, UpdatedAt: now.Add(time.Minute),
	}
	realtimeChanged, err := store.ReconcileIssue(ctx, external.IssueIID, &external, now.Add(time.Minute))
	if err != nil || !realtimeChanged {
		t.Fatalf("reconcile external issue = changed %v, error %v", realtimeChanged, err)
	}
	select {
	case revision := <-updates:
		if revision == "1" {
			t.Fatalf("revision did not advance: %s", revision)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reconcile did not publish a revision")
	}
	if card, err := store.Card(ctx, external.IssueIID); err != nil || card.Description != "GitLab canonical" || len(card.AssigneeGitLabUserIDs) != 1 {
		t.Fatalf("external card = %#v, error %v", card, err)
	}
	realtimeChanged, err = store.ReconcileIssue(ctx, external.IssueIID, nil, now.Add(2*time.Minute))
	if err != nil || !realtimeChanged {
		t.Fatalf("remove external issue = changed %v, error %v", realtimeChanged, err)
	}
	unsubscribe()
	stopListener()

	session, err := oauthRepo.CreateSession(ctx, identity.Session{
		ID: uuid.NewString(), UserID: user.ID, TokenHash: []byte("session-hash"),
		ExpiresAt: now.Add(14 * 24 * time.Hour), CreatedAt: now, LastUsedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	session.LastUsedAt = now.Add(time.Hour)
	session.ExpiresAt = session.LastUsedAt.Add(14 * 24 * time.Hour)
	if err := oauthRepo.TouchSession(ctx, session.ID, session); err != nil {
		t.Fatal(err)
	}
	renewed, err := oauthRepo.GetSessionByTokenHash(ctx, []byte("session-hash"))
	if err != nil || !renewed.ExpiresAt.Equal(session.ExpiresAt) {
		t.Fatalf("rolling session = %#v, err = %v", renewed, err)
	}

	directoryService := appdirectory.NewService(store, noop.NewTracerProvider().Tracer("test"))
	preferences, err := directoryService.Update(ctx, user.ID, "design")
	if err != nil {
		t.Fatal(err)
	}
	if preferences.DefaultTeamKey == nil || *preferences.DefaultTeamKey != "design" || len(preferences.DirectoryTeamKeys) != 1 || preferences.DirectoryTeamKeys[0] != "development" {
		t.Fatalf("preferences = %#v", preferences)
	}

	boardService := appboard.NewService(store, directoryService, noop.NewTracerProvider().Tracer("test"))
	operationID := uuid.NewString()
	startDate := "2026-07-17"
	dueDate := "2026-07-21"
	created, err := boardService.Create(ctx, appboard.CreateInput{
		OperationID: operationID, ActorUserID: user.ID, Title: "修正報名流程",
		Description: "詳細規劃", TeamKey: "development", ListKey: "inbox", AssigneeGitLabUserIDs: []int64{101}, StartDate: &startDate, DueDate: &dueDate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Card.IssueIID >= 0 || created.Card.PendingOperationID != operationID || created.Card.ListKey != "inbox" {
		t.Fatalf("optimistic card = %#v", created.Card)
	}
	idempotent, err := boardService.Create(ctx, appboard.CreateInput{OperationID: operationID})
	if err != nil || idempotent.Card.IssueIID != created.Card.IssueIID {
		t.Fatalf("idempotent create = %#v, err = %v", idempotent, err)
	}
	gitlab := &operationGitLabFake{now: now.Add(time.Minute)}
	actorTokens := &operationActorTokensFake{}
	syncService := appsync.NewService(gitlab, operationDirectoryFake{}, store, actorTokens, nil, noop.NewTracerProvider().Tracer("test"))
	processed, err := syncService.ProcessOne(ctx)
	if err != nil || !processed || gitlab.lastMutation == nil || !gitlab.lastMutation.Create {
		t.Fatalf("process create = %v, %v, mutation=%#v", processed, err, gitlab.lastMutation)
	}
	if actorTokens.userID != user.ID {
		t.Fatalf("operation actor = %q, want %q", actorTokens.userID, user.ID)
	}
	canonical, err := store.ByOperation(ctx, operationID)
	if err != nil || canonical.Card.IssueIID != 42 || canonical.Card.StartDate != startDate {
		t.Fatalf("canonical create = %#v, err = %v", canonical, err)
	}

	updatedStartDate := "2026-07-18"
	startChanged, err := boardService.UpdateStartDate(ctx, appboard.UpdateStartDateInput{
		OperationID: uuid.NewString(), ActorUserID: user.ID,
		IssueIID: canonical.Card.IssueIID, StartDate: &updatedStartDate,
	})
	if err != nil || startChanged.Card.StartDate != updatedStartDate {
		t.Fatalf("start date mutation = %#v, err = %v", startChanged.Card, err)
	}
	processed, err = syncService.ProcessOne(ctx)
	if err != nil || !processed || gitlab.lastMutation == nil || gitlab.lastMutation.StartDate != updatedStartDate {
		t.Fatalf("process start date = %v, %v, mutation=%#v", processed, err, gitlab.lastMutation)
	}

	labelOperationID := uuid.NewString()
	labelsChanged, err := boardService.UpdateLabels(ctx, appboard.UpdateLabelsInput{
		OperationID: labelOperationID, ActorUserID: user.ID, IssueIID: canonical.Card.IssueIID,
		Labels: []string{"Team::行政組", "Backend"},
	})
	if err != nil || labelsChanged.Card.TeamKey != "administration" || labelsChanged.Card.ListKey != "doing" ||
		len(labelsChanged.Card.AssigneeGitLabUserIDs) != 0 || labelsChanged.Operation.Kind != domainboard.OperationUpdateLabels {
		t.Fatalf("label mutation = %#v, err = %v", labelsChanged, err)
	}
	var storedKind string
	if err := pool.QueryRow(ctx, `SELECT kind FROM durable_operations WHERE id = $1`, labelOperationID).Scan(&storedKind); err != nil || storedKind != "update_labels" {
		t.Fatalf("stored label operation kind = %q, err = %v", storedKind, err)
	}
	processed, err = syncService.ProcessOne(ctx)
	if err != nil || !processed || gitlab.lastMutation == nil ||
		!contains(gitlab.lastMutation.Labels, "Team::行政組") || contains(gitlab.lastMutation.Labels, "Status::Doing") || !contains(gitlab.lastMutation.Labels, "Backend") || gitlab.lastMutation.GitLabStatusName != "Doing" {
		t.Fatalf("process labels = %v, %v, mutation=%#v", processed, err, gitlab.lastMutation)
	}

	changed, err := boardService.UpdateTeam(ctx, appboard.UpdateTeamInput{
		OperationID: uuid.NewString(), ActorUserID: user.ID,
		IssueIID: canonical.Card.IssueIID, TeamKey: "administration",
	})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Card.TeamKey != "administration" || len(changed.Card.AssigneeGitLabUserIDs) != 0 {
		t.Fatalf("team mutation = %#v", changed.Card)
	}
	processed, err = syncService.ProcessOne(ctx)
	if err != nil || !processed || gitlab.lastMutation == nil || gitlab.lastMutation.Create {
		t.Fatalf("process update = %v, %v, mutation=%#v", processed, err, gitlab.lastMutation)
	}

	if err := store.ReplaceBoard(ctx, appsync.DefaultBoardLists, nil, "board-2", now.Add(2*time.Minute)); err != nil {
		t.Fatalf("replace board without completed card: %v", err)
	}
	var cardCount, attachedOperationCount int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM issue_cache WHERE issue_iid = 42),
			(SELECT COUNT(*) FROM durable_operations WHERE issue_iid = 42)
	`).Scan(&cardCount, &attachedOperationCount); err != nil {
		t.Fatal(err)
	}
	if cardCount != 0 || attachedOperationCount != 0 {
		t.Fatalf("removed card references: cards=%d attached_operations=%d", cardCount, attachedOperationCount)
	}
	detached, err := store.ByOperation(ctx, operationID)
	if err != nil || detached.Operation.IssueIID != nil || detached.Operation.State != "synced" {
		t.Fatalf("detached completed operation = %#v, err = %v", detached.Operation, err)
	}

	revokedDirectory := domaindirectory.Snapshot{
		Teams: []domaindirectory.Team{{
			Key: "development", Name: "Development", TitlePrefix: "[Development]", GitLabLabel: "team::development",
			Active: true, MemberGitLabUserIDs: []int64{202},
		}},
		Members: []domaindirectory.Member{{
			GitLabUserID: 202, Username: "bob", DisplayName: "Bob", ProfileURL: "https://gitlab.com/bob",
			AccessLevel: 30, State: domaindirectory.MemberActive, TeamKeys: []string{"development"},
		}},
		SourceRevision: "member-revocation", SyncedAt: now.Add(3 * time.Minute),
	}
	if err := store.ReplaceDirectory(ctx, revokedDirectory); err != nil {
		t.Fatalf("replace directory after member revocation: %v", err)
	}
	if _, err := oauthRepo.GetSessionByTokenHash(ctx, []byte("session-hash")); !errors.Is(err, identity.ErrSessionNotFound) {
		t.Fatalf("revoked member session error = %v", err)
	}
	if _, err := oauthRepo.OAuthCredential(ctx, user.ID); !errors.Is(err, identity.ErrOAuthCredentialNotFound) {
		t.Fatalf("revoked member OAuth credential error = %v", err)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

type operationGitLabFake struct {
	now          time.Time
	lastMutation *appsync.IssueMutation
}

type operationDirectoryFake struct{}

type operationActorTokensFake struct{ userID string }

func (f *operationActorTokensFake) AccessToken(_ context.Context, userID string) (string, error) {
	f.userID = userID
	return "actor-token", nil
}

func (operationDirectoryFake) DirectoryRevision(context.Context) (string, error) {
	return "revision", nil
}
func (operationDirectoryFake) DirectoryFile(context.Context) (domaindirectory.File, string, error) {
	return domaindirectory.File{}, "revision", nil
}
func (*operationGitLabFake) ProjectMembers(context.Context) ([]domaindirectory.GitLabMember, error) {
	return nil, nil
}
func (*operationGitLabFake) Issues(context.Context) ([]appsync.GitLabIssue, error) {
	return nil, nil
}
func (*operationGitLabFake) Issue(context.Context, int64) (appsync.GitLabIssue, error) {
	return appsync.GitLabIssue{}, domainboard.ErrCardNotFound
}
func (f *operationGitLabFake) ApplyIssue(_ context.Context, mutation appsync.IssueMutation, _ string) (appsync.GitLabIssue, error) {
	f.lastMutation = &mutation
	return appsync.GitLabIssue{
		IssueIID: 42, GitLabIssueID: 420, Title: mutation.Title, Description: mutation.Description,
		WebURL: "https://gitlab.example/issues/42", Labels: mutation.Labels,
		AssigneeGitLabUserIDs: mutation.AssigneeGitLabUserIDs,
		StartDate:             mutation.StartDate, DueDate: mutation.DueDate, State: "opened", CreatedAt: f.now, UpdatedAt: f.now,
		GitLabStatusName: mutation.GitLabStatusName,
	}, nil
}

func seedSnapshots(t *testing.T, ctx context.Context, pool *pgxpool.Pool, now time.Time) {
	t.Helper()
	statements := []string{
		`INSERT INTO directory_teams
		    (key, display_name, title_prefix, gitlab_label, sort_order, active, source_revision, updated_at)
		VALUES
		    ('development', '開發組', '[開發組]', 'Team::開發組', 0, true, 'revision-1', $1),
		    ('design', '設計組', '[設計組]', 'Team::設計組', 1, true, 'revision-1', $1),
		    ('administration', '行政組', '[行政組]', 'Team::行政組', 2, true, 'revision-1', $1)`,
		`INSERT INTO directory_members
		    (gitlab_user_id, username, display_name, profile_url, access_level, state, last_synced_at)
		VALUES
		    (101, 'alice', 'Alice', 'https://gitlab.com/alice', 40, 'active', $1),
		    (202, 'bob', 'Bob', 'https://gitlab.com/bob', 30, 'active', $1)`,
		`INSERT INTO directory_team_memberships (team_key, gitlab_user_id, source, updated_at)
		VALUES
		    ('development', 101, 'gitlab_directory', $1),
		    ('design', 202, 'gitlab_directory', $1)`,
		`INSERT INTO board_lists (key, display_name, gitlab_status_name, position, closed, color, updated_at)
		VALUES
		    ('wating', 'Waiting', 'Waiting', 0, false, '#d2ad46', $1),
		    ('inbox', 'Inbox', 'Inbox', 1, false, '#6699cc', $1),
		    ('todo', 'To do', 'To do', 2, false, '#ed9121', $1),
		    ('doing', 'Doing', 'Doing', 3, false, '#1f75cb', $1),
		    ('review', 'Review', 'Review', 4, false, '#7a07ab', $1),
		    ('closed', 'Done', 'Done', 5, true, '#108548', $1)`,
		`INSERT INTO sync_snapshots
		    (resource, source_revision, last_success_at, last_attempt_at, updated_at)
		VALUES
		    ('directory', 'revision-1', $1, $1, $1),
		    ('members', 'members-1', $1, $1, $1),
		    ('board', 'board-1', $1, $1, $1)`,
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement, now); err != nil {
			t.Fatal(err)
		}
	}
}

func verifyCardReordering(t *testing.T, ctx context.Context, pool *pgxpool.Pool, store *pgsitcon.Repository, userID string, now time.Time) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO issue_cache
		    (issue_iid, title, description, list_key, position, team_key, labels, sync_state, created_at, updated_at)
		VALUES
		    (501, 'First', '', 'doing', 0, 'development', '{}', 'synced', $1, $1),
		    (502, 'Second', '', 'doing', 1, 'development', '{}', 'synced', $1, $1),
		    (503, 'Third', '', 'doing', 2, 'development', '{}', 'synced', $1, $1)
	`, now); err != nil {
		t.Fatalf("seed card reorder fixture: %v", err)
	}

	card, err := store.Card(ctx, 503)
	if err != nil {
		t.Fatalf("load card reorder fixture: %v", err)
	}
	operationID := uuid.NewString()
	issueIID := card.IssueIID
	card.Position = 0
	card.SyncState = domainboard.OperationPending
	card.PendingOperationID = operationID
	card.UpdatedAt = now.Add(time.Second)
	result, updateErr := store.UpdateCard(ctx, domainboard.Mutation{
		Card: card,
		Operation: domainboard.Operation{
			ID: operationID, Kind: domainboard.OperationMoveCard, IssueIID: &issueIID,
			State: domainboard.OperationPending, CreatedAt: card.UpdatedAt, UpdatedAt: card.UpdatedAt,
		},
		RequestedByUserID: userID,
		Payload:           map[string]any{"listKey": "doing", "position": 0},
	})

	rows, queryErr := pool.Query(ctx, `SELECT issue_iid, position FROM issue_cache WHERE list_key = 'doing' ORDER BY position, issue_iid`)
	var issueIIDs []int64
	var positions []int32
	if queryErr == nil {
		for rows.Next() {
			var currentIssueIID int64
			var position int32
			if scanErr := rows.Scan(&currentIssueIID, &position); scanErr != nil {
				queryErr = scanErr
				break
			}
			issueIIDs = append(issueIIDs, currentIssueIID)
			positions = append(positions, position)
		}
		if rowsErr := rows.Err(); queryErr == nil && rowsErr != nil {
			queryErr = rowsErr
		}
		rows.Close()
	}
	if _, err := pool.Exec(ctx, `DELETE FROM issue_cache WHERE issue_iid = ANY($1::bigint[])`, []int64{501, 502, 503}); err != nil {
		t.Fatalf("clean card reorder fixtures: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM durable_operations WHERE id = $1`, uuid.MustParse(operationID)); err != nil {
		t.Fatalf("clean card reorder operation: %v", err)
	}

	if updateErr != nil {
		t.Fatalf("move card in repository: %v", updateErr)
	}
	if queryErr != nil {
		t.Fatalf("read reordered cards: %v", queryErr)
	}
	if result.Card.Position != 0 || !slices.Equal(issueIIDs, []int64{503, 501, 502}) || !slices.Equal(positions, []int32{0, 1, 2}) {
		t.Fatalf("reordered cards = ids %v positions %v result %#v", issueIIDs, positions, result.Card)
	}
}

func verifySnapshotMerge(t *testing.T, ctx context.Context, pool *pgxpool.Pool, store *pgsitcon.Repository, now time.Time) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO issue_cache
		    (issue_iid, gitlab_issue_id, title, description, list_key, position, team_key,
		     labels, gitlab_status_name, sync_state, gitlab_updated_at, created_at, updated_at)
		VALUES
		    (601, 6001, 'First snapshot card', '', 'doing', 0, 'development', '{}', 'Doing', 'synced', $1, $1, $1),
		    (602, 6002, 'Second snapshot card', '', 'doing', 1, 'development', '{}', 'Doing', 'synced', $1, $1, $1)
	`, now); err != nil {
		t.Fatalf("seed snapshot merge fixtures: %v", err)
	}
	defer func() {
		if _, err := pool.Exec(ctx, `DELETE FROM issue_cache WHERE issue_iid = ANY($1::bigint[])`, []int64{601, 602}); err != nil {
			t.Fatalf("clean snapshot merge fixtures: %v", err)
		}
	}()

	card := func(issueIID, gitLabIssueID int64, title, listKey, status string, updatedAt time.Time) domainboard.Card {
		return domainboard.Card{
			IssueIID: issueIID, GitLabIssueID: &gitLabIssueID, Title: title, ListKey: listKey, TeamKey: "development",
			GitLabStatusName: status, SyncState: domainboard.OperationSynced, CreatedAt: now, UpdatedAt: updatedAt,
		}
	}
	first := card(601, 6001, "First snapshot card", "doing", "Doing", now.Add(time.Minute))
	second := card(602, 6002, "Second snapshot card", "doing", "Doing", now.Add(time.Minute))

	revisionBefore, err := store.Revision(ctx)
	if err != nil {
		t.Fatalf("load revision before identical snapshot: %v", err)
	}
	if err := store.ReplaceBoard(ctx, appsync.DefaultBoardLists, []domainboard.Card{second, first}, "board-1", now.Add(time.Minute)); err != nil {
		t.Fatalf("replace identical board snapshot: %v", err)
	}
	revisionAfter, err := store.Revision(ctx)
	if err != nil || revisionAfter != revisionBefore {
		t.Fatalf("identical snapshot revision = %q, error %v, want %q", revisionAfter, err, revisionBefore)
	}
	assertStoredOrder(t, ctx, pool, "doing", []int64{601, 602})

	if err := store.ReplaceBoard(ctx, appsync.DefaultBoardLists, []domainboard.Card{second, first}, "board-merge-2", now.Add(2*time.Minute)); err != nil {
		t.Fatalf("merge changed board snapshot: %v", err)
	}
	assertStoredOrder(t, ctx, pool, "doing", []int64{601, 602})

	if _, err := pool.Exec(ctx, `
		UPDATE issue_cache
		SET list_key = 'review', position = 0, gitlab_status_name = 'Review', gitlab_updated_at = $2, updated_at = $2
		WHERE issue_iid = $1
	`, int64(601), now.Add(4*time.Minute)); err != nil {
		t.Fatalf("seed completed local move: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE issue_cache SET position = 0 WHERE issue_iid = 602`); err != nil {
		t.Fatalf("compact local move source: %v", err)
	}
	first.UpdatedAt = now.Add(2 * time.Minute)
	second.UpdatedAt = now.Add(2 * time.Minute)
	var revisionBeforeStaleMerge string
	if err := pool.QueryRow(ctx, `SELECT source_revision FROM sync_snapshots WHERE resource = 'board'`).Scan(&revisionBeforeStaleMerge); err != nil {
		t.Fatalf("load revision before stale merge: %v", err)
	}
	if err := store.ReplaceBoard(ctx, appsync.DefaultBoardLists, []domainboard.Card{second, first}, "board-merge-3", now.Add(3*time.Minute)); err != nil {
		t.Fatalf("merge stale board snapshot: %v", err)
	}
	if stored, err := store.Card(ctx, 601); err != nil || stored.ListKey != "review" || stored.Position != 0 {
		t.Fatalf("stale snapshot overwrote completed move: card %#v, error %v", stored, err)
	}

	if err := store.ReplaceBoard(ctx, appsync.DefaultBoardLists, []domainboard.Card{second}, "board-merge-4", now.Add(3*time.Minute)); err != nil {
		t.Fatalf("merge stale snapshot missing a recently updated card: %v", err)
	}
	if _, err := store.Card(ctx, 601); err != nil {
		t.Fatalf("stale snapshot deleted a recently updated card: %v", err)
	}
	var revisionAfterStaleMerge string
	if err := pool.QueryRow(ctx, `SELECT source_revision FROM sync_snapshots WHERE resource = 'board'`).Scan(&revisionAfterStaleMerge); err != nil {
		t.Fatalf("load revision after stale merge: %v", err)
	}
	if revisionAfterStaleMerge != revisionBeforeStaleMerge {
		t.Fatalf("stale merge revision = %q, want %q so the next refresh retries", revisionAfterStaleMerge, revisionBeforeStaleMerge)
	}

	second.UpdatedAt = now.Add(5 * time.Minute)
	if changed, err := store.ReconcileIssue(ctx, second.IssueIID, &second, now.Add(5*time.Minute)); err != nil || !changed {
		t.Fatalf("reconcile same-list issue = changed %v, error %v", changed, err)
	}
	assertStoredOrder(t, ctx, pool, "doing", []int64{602})
	second.ListKey, second.GitLabStatusName, second.UpdatedAt = "review", "Review", now.Add(6*time.Minute)
	if changed, err := store.ReconcileIssue(ctx, second.IssueIID, &second, now.Add(6*time.Minute)); err != nil || !changed {
		t.Fatalf("reconcile external list move = changed %v, error %v", changed, err)
	}
	assertStoredOrder(t, ctx, pool, "review", []int64{602, 601})
}

func assertStoredOrder(t *testing.T, ctx context.Context, pool *pgxpool.Pool, listKey string, want []int64) {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT issue_iid FROM issue_cache WHERE list_key = $1 ORDER BY position, issue_iid`, listKey)
	if err != nil {
		t.Fatalf("query %s card order: %v", listKey, err)
	}
	defer rows.Close()
	var got []int64
	for rows.Next() {
		var issueIID int64
		if err := rows.Scan(&issueIID); err != nil {
			t.Fatalf("scan %s card order: %v", listKey, err)
		}
		got = append(got, issueIID)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s card order: %v", listKey, err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("%s card order = %v, want %v", listKey, got, want)
	}
}

func migrationDirectory(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test path")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "db", "migrations")
}
