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
	verifyUnaffectedLanesAreNotRewritten(t, ctx, pool, store, now)
	verifySweepKeepsCardsObservedMidFetch(t, ctx, pool, store, now)
	verifyRepeatedObservationIsQuiet(t, ctx, pool, store, now)
	verifySyncActionsAreGaplessAndCommitOrdered(t, ctx, pool, store, now)
	verifyPruneKeepsWhatIsStillNeeded(t, ctx, pool, store, now)

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
	if err != nil || labelsChanged.Card.TeamKey != "administration" || labelsChanged.Card.ListKey != "inbox" ||
		len(labelsChanged.Card.AssigneeGitLabUserIDs) != 0 || labelsChanged.Operation.Kind != domainboard.OperationUpdateLabels {
		t.Fatalf("label mutation = %#v, err = %v", labelsChanged, err)
	}
	var storedKind string
	if err := pool.QueryRow(ctx, `SELECT kind FROM durable_operations WHERE id = $1`, labelOperationID).Scan(&storedKind); err != nil || storedKind != "update_labels" {
		t.Fatalf("stored label operation kind = %q, err = %v", storedKind, err)
	}
	processed, err = syncService.ProcessOne(ctx)
	if err != nil || !processed || gitlab.lastMutation == nil ||
		!contains(gitlab.lastMutation.Labels, "Team::行政組") || contains(gitlab.lastMutation.Labels, "Status::Doing") || !contains(gitlab.lastMutation.Labels, "Backend") || gitlab.lastMutation.GitLabStatusName != "Inbox" {
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

	// The read has to mention some other card. A complete read that mentions nothing
	// at all is refused as a prune source, because that is what a lost-access GraphQL
	// response looks like and manual lane order lives only in PostgreSQL.
	bystanderID := int64(990)
	bystander := domainboard.Card{
		IssueIID: 99, GitLabIssueID: &bystanderID, Title: "旁觀卡", ListKey: "wating", TeamKey: "development",
		GitLabStatusName: "Waiting", SyncState: domainboard.OperationSynced,
		CreatedAt: now, UpdatedAt: now.Add(2 * time.Minute),
	}
	// StartedAt comes from the real clock here, not the fixture's. The card being
	// pruned was written by ProcessOne, which stamps gitlab_observed_at from the
	// service's own clock, and pruning compares the two. A sweep dated 2026-07-14
	// would sit before that stamp and correctly decline to prune.
	sweepStart := time.Now().UTC().Add(time.Millisecond)
	if err := store.ApplyBoardObservation(ctx, domainboard.BoardObservation{
		Cards: []domainboard.Card{bystander}, Complete: true,
		StartedAt: sweepStart, SyncedAt: now.Add(2 * time.Minute),
	}); err != nil {
		t.Fatalf("replace board without completed card: %v", err)
	}
	if _, err := store.Card(ctx, bystander.IssueIID); err != nil {
		t.Fatalf("a card the read did mention was pruned: %v", err)
	}
	defer func() {
		if _, err := pool.Exec(ctx, `DELETE FROM issue_cache WHERE issue_iid = $1`, bystander.IssueIID); err != nil {
			t.Fatalf("clean bystander card: %v", err)
		}
	}()
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
func (*operationGitLabFake) Issues(context.Context, appsync.IssueFilter) ([]appsync.GitLabIssue, error) {
	return nil, nil
}
func (*operationGitLabFake) IssueDigests(context.Context) ([]domainboard.IssueDigest, error) {
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

// verifySyncActionsAreGaplessAndCommitOrdered pins the property the whole delta
// protocol rests on.
//
// The classic way to get this wrong is a bigserial: it hands out its number before the
// transaction commits, so a reader can see id 5 while id 4 is still in flight, advance
// its cursor past it, and lose id 4 permanently once it lands. Taking the id from a
// single-row counter updated inside the transaction makes that impossible, because the
// row lock is held until commit -- which also means writers serialize here, and this
// test is what says that is deliberate rather than accidental.
func verifySyncActionsAreGaplessAndCommitOrdered(t *testing.T, ctx context.Context, pool *pgxpool.Pool, store *pgsitcon.Repository, now time.Time) {
	t.Helper()
	defer func() {
		if _, err := pool.Exec(ctx, `DELETE FROM issue_cache WHERE issue_iid = $1`, int64(1002)); err != nil {
			t.Fatalf("clean ordering fixtures: %v", err)
		}
	}()
	gitLabIssueID := int64(10002)
	fast := domainboard.Card{
		IssueIID: 1002, GitLabIssueID: &gitLabIssueID, Title: "Fast writer", ListKey: "wating", TeamKey: "development",
		GitLabStatusName: "Waiting", SyncState: domainboard.OperationSynced,
		CreatedAt: now, UpdatedAt: now.Add(30 * time.Minute),
	}

	// Stand in for a slow writer by taking a sync id the way flush does and holding the
	// transaction open. The UPDATE is the whole mechanism: it locks the single counter
	// row until commit.
	held, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin held transaction: %v", err)
	}
	var heldID int64
	if err := held.QueryRow(ctx, `
		UPDATE realtime_state SET revision = revision + 1, updated_at = $1
		WHERE topic = 'bootstrap' RETURNING revision
	`, now.Add(30*time.Minute)).Scan(&heldID); err != nil {
		_ = held.Rollback(ctx)
		t.Fatalf("take held sync id: %v", err)
	}
	if _, err := held.Exec(ctx, `
		INSERT INTO sync_actions (sync_id, seq, entity, entity_id, op, payload, created_at)
		VALUES ($1, 0, 'card', '1001', 'upsert', '{}'::jsonb, $2)
	`, heldID, now.Add(30*time.Minute)); err != nil {
		_ = held.Rollback(ctx)
		t.Fatalf("record held action: %v", err)
	}

	// A second writer must block rather than take a number ahead of the first.
	blocked := make(chan error, 1)
	go func() {
		_, blockedErr := store.ReconcileIssue(ctx, fast.IssueIID, &fast, now.Add(31*time.Minute))
		blocked <- blockedErr
	}()
	select {
	case err := <-blocked:
		_ = held.Rollback(ctx)
		t.Fatalf("second writer completed while the first still held its sync id: %v", err)
	case <-time.After(300 * time.Millisecond):
	}

	if err := held.Commit(ctx); err != nil {
		t.Fatalf("commit held transaction: %v", err)
	}
	if err := <-blocked; err != nil {
		t.Fatalf("second writer: %v", err)
	}

	var fastID int64
	if err := pool.QueryRow(ctx, `
		SELECT sync_id FROM sync_actions WHERE entity = 'card' AND entity_id = '1002' ORDER BY sync_id DESC LIMIT 1
	`).Scan(&fastID); err != nil {
		t.Fatalf("read second writer sync id: %v", err)
	}
	if heldID >= fastID {
		t.Fatalf("sync ids %d and %d were not issued in commit order", heldID, fastID)
	}

	// No gaps: every id from the floor up to the current revision has at least one
	// action, which is also a mechanical check that no write path forgot to record.
	var missing int64
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM realtime_state state,
		     generate_series(state.action_floor + 1, state.revision) AS candidate(sync_id)
		WHERE state.topic = 'bootstrap'
		  AND NOT EXISTS (SELECT 1 FROM sync_actions WHERE sync_actions.sync_id = candidate.sync_id)
	`).Scan(&missing); err != nil {
		t.Fatalf("check for gaps: %v", err)
	}
	if missing != 0 {
		t.Fatalf("%d sync ids advanced the revision without recording an action", missing)
	}
}

// verifyPruneKeepsWhatIsStillNeeded pins retention. A failed operation is still offered
// for retry in the UI, and one a card still points at is still live, so neither may be
// swept up with the merely old.
func verifyPruneKeepsWhatIsStillNeeded(t *testing.T, ctx context.Context, pool *pgxpool.Pool, store *pgsitcon.Repository, now time.Time) {
	t.Helper()
	stale := time.Now().UTC().Add(-30 * 24 * time.Hour)
	var userID string
	if err := pool.QueryRow(ctx, `SELECT id FROM users LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("load a user: %v", err)
	}
	seed := func(kind, state string, updatedAt time.Time) string {
		id := uuid.NewString()
		if _, err := pool.Exec(ctx, `
			INSERT INTO durable_operations
			    (id, kind, issue_iid, requested_by_user_id, payload, state, attempts, available_at, created_at, updated_at)
			VALUES ($1, $2, NULL, $3, '{}'::jsonb, $4, 0, $5, $5, $5)
		`, uuid.MustParse(id), kind, uuid.MustParse(userID), state, updatedAt); err != nil {
			t.Fatalf("seed durable operation: %v", err)
		}
		return id
	}
	oldSynced := seed("update_details", "synced", stale)
	oldFailed := seed("update_details", "failed", stale)
	defer func() {
		if _, err := pool.Exec(ctx, `DELETE FROM durable_operations WHERE id = ANY($1::uuid[])`,
			[]string{oldSynced, oldFailed}); err != nil {
			t.Fatalf("clean retention fixtures: %v", err)
		}
	}()

	if err := store.Prune(ctx, time.Now().UTC()); err != nil {
		t.Fatalf("Prune() error = %v", err)
	}
	exists := func(id string) bool {
		var found bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM durable_operations WHERE id = $1)`, uuid.MustParse(id)).Scan(&found); err != nil {
			t.Fatalf("check operation %s: %v", id, err)
		}
		return found
	}
	if exists(oldSynced) {
		t.Fatal("an old completed operation survived pruning")
	}
	if !exists(oldFailed) {
		t.Fatal("an old failed operation was pruned; the board still offers it for retry")
	}
}

// verifyRepeatedObservationIsQuiet pins the guard that keeps a re-read of unchanged
// data from looking like a change. The timestamp guard alone still rewrites a row whose
// gitlab_updated_at merely matches, so without the value comparison every poll would
// touch every recently-changed card, bump the bootstrap revision, and wake every
// connected browser for nothing.
func verifyRepeatedObservationIsQuiet(t *testing.T, ctx context.Context, pool *pgxpool.Pool, store *pgsitcon.Repository, now time.Time) {
	t.Helper()
	defer func() {
		if _, err := pool.Exec(ctx, `DELETE FROM issue_cache WHERE issue_iid = $1`, int64(901)); err != nil {
			t.Fatalf("clean repeat fixture: %v", err)
		}
	}()
	gitLabIssueID := int64(9001)
	card := domainboard.Card{
		IssueIID: 901, GitLabIssueID: &gitLabIssueID, Title: "重複觀測", ListKey: "wating", TeamKey: "development",
		GitLabStatusName: "Waiting", SyncState: domainboard.OperationSynced,
		CreatedAt: now, UpdatedAt: now.Add(20 * time.Minute),
	}
	if changed, err := store.ReconcileIssue(ctx, card.IssueIID, &card, now.Add(20*time.Minute)); err != nil || !changed {
		t.Fatalf("first reconcile = changed %v, error %v", changed, err)
	}

	revisionBefore, err := store.Revision(ctx)
	if err != nil {
		t.Fatalf("load revision before repeat: %v", err)
	}
	var writtenBefore string
	if err := pool.QueryRow(ctx, `SELECT xmin::text FROM issue_cache WHERE issue_iid = $1`, card.IssueIID).Scan(&writtenBefore); err != nil {
		t.Fatalf("read row version before repeat: %v", err)
	}

	if changed, err := store.ReconcileIssue(ctx, card.IssueIID, &card, now.Add(21*time.Minute)); err != nil || changed {
		t.Fatalf("repeat reconcile = changed %v, error %v, want no change", changed, err)
	}
	revisionAfter, err := store.Revision(ctx)
	if err != nil {
		t.Fatalf("load revision after repeat: %v", err)
	}
	if revisionAfter != revisionBefore {
		t.Fatalf("repeat reconcile bumped the revision %q -> %q", revisionBefore, revisionAfter)
	}

	// gitlab_observed_at must still move, or a sweep would later mistake this card for
	// one GitLab has stopped reporting.
	var observedAt time.Time
	if err := pool.QueryRow(ctx, `SELECT gitlab_observed_at FROM issue_cache WHERE issue_iid = $1`, card.IssueIID).Scan(&observedAt); err != nil {
		t.Fatalf("read observed timestamp: %v", err)
	}
	if !observedAt.Equal(now.Add(21 * time.Minute)) {
		t.Fatalf("gitlab_observed_at = %v, want the repeat read at %v", observedAt, now.Add(21*time.Minute))
	}
}

// verifySweepKeepsCardsObservedMidFetch pins the clock-domain rule that makes pruning
// safe. A full read of GitLab takes time; a webhook can reconcile a card while it is in
// flight, and that card will be missing from the pages the read already fetched.
// Deciding by gitlab_observed_at against the instant the read began -- both PostgreSQL's
// clock -- keeps it, while a card nothing has confirmed since before the read began is
// genuinely gone.
func verifySweepKeepsCardsObservedMidFetch(t *testing.T, ctx context.Context, pool *pgxpool.Pool, store *pgsitcon.Repository, now time.Time) {
	t.Helper()
	defer func() {
		if _, err := pool.Exec(ctx, `DELETE FROM issue_cache WHERE issue_iid = ANY($1::bigint[])`, []int64{801, 802, 803}); err != nil {
			t.Fatalf("clean sweep fixtures: %v", err)
		}
	}()
	card := func(issueIID, gitLabIssueID int64, title string, updatedAt time.Time) domainboard.Card {
		return domainboard.Card{
			IssueIID: issueIID, GitLabIssueID: &gitLabIssueID, Title: title, ListKey: "wating", TeamKey: "development",
			GitLabStatusName: "Waiting", SyncState: domainboard.OperationSynced, CreatedAt: now, UpdatedAt: updatedAt,
		}
	}
	beforeSweep := now.Add(9 * time.Minute)
	for _, seed := range []domainboard.Card{
		card(801, 8001, "Reconciled mid fetch", beforeSweep),
		card(802, 8002, "Genuinely gone", beforeSweep),
		card(803, 8003, "Still reported", beforeSweep),
	} {
		if _, err := store.ReconcileIssue(ctx, seed.IssueIID, &seed, beforeSweep); err != nil {
			t.Fatalf("seed sweep fixture %d: %v", seed.IssueIID, err)
		}
	}

	sweepStartedAt := now.Add(10 * time.Minute)
	// The webhook that lands while the read is paging.
	midFetch := card(801, 8001, "Reconciled mid fetch, renamed", now.Add(11*time.Minute))
	if changed, err := store.ReconcileIssue(ctx, 801, &midFetch, now.Add(11*time.Minute)); err != nil || !changed {
		t.Fatalf("reconcile mid fetch = changed %v, error %v", changed, err)
	}

	// The read only ever saw 803: it was paging before the webhook landed.
	stillReported := card(803, 8003, "Still reported", now.Add(12*time.Minute))
	if err := store.ApplyBoardObservation(ctx, domainboard.BoardObservation{
		Cards: []domainboard.Card{stillReported}, Complete: true,
		StartedAt: sweepStartedAt, SyncedAt: now.Add(12 * time.Minute),
	}); err != nil {
		t.Fatalf("apply sweep: %v", err)
	}

	if _, err := store.Card(ctx, 801); err != nil {
		t.Fatalf("a card reconciled while the read was in flight was pruned: %v", err)
	}
	if _, err := store.Card(ctx, 802); err == nil {
		t.Fatal("a card nothing confirmed since before the read began survived the sweep")
	}
	if _, err := store.Card(ctx, 803); err != nil {
		t.Fatalf("a card the read reported was pruned: %v", err)
	}
}

// verifyUnaffectedLanesAreNotRewritten pins the guarantee that a merge renumbers only
// the lanes whose order actually moved. xmin changes on any row rewrite, so comparing
// it before and after proves the untouched lane was never written, not merely that it
// ended up with the same values.
func verifyUnaffectedLanesAreNotRewritten(t *testing.T, ctx context.Context, pool *pgxpool.Pool, store *pgsitcon.Repository, now time.Time) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO issue_cache
		    (issue_iid, gitlab_issue_id, title, description, list_key, position, team_key,
		     labels, gitlab_status_name, sync_state, gitlab_updated_at, created_at, updated_at)
		VALUES
		    (701, 7001, 'Touched lane card', '', 'doing', 0, 'development', '{}', 'Doing', 'synced', $1, $1, $1),
		    (702, 7002, 'Quiet lane card one', '', 'review', 0, 'development', '{}', 'Review', 'synced', $1, $1, $1),
		    (703, 7003, 'Quiet lane card two', '', 'review', 1, 'development', '{}', 'Review', 'synced', $1, $1, $1)
	`, now); err != nil {
		t.Fatalf("seed lane isolation fixtures: %v", err)
	}
	defer func() {
		if _, err := pool.Exec(ctx, `DELETE FROM issue_cache WHERE issue_iid = ANY($1::bigint[])`, []int64{701, 702, 703}); err != nil {
			t.Fatalf("clean lane isolation fixtures: %v", err)
		}
	}()

	quietBefore := laneRowVersions(t, ctx, pool, "review")
	card := func(issueIID, gitLabIssueID int64, title, listKey, status string, updatedAt time.Time) domainboard.Card {
		return domainboard.Card{
			IssueIID: issueIID, GitLabIssueID: &gitLabIssueID, Title: title, ListKey: listKey, TeamKey: "development",
			GitLabStatusName: status, SyncState: domainboard.OperationSynced, CreatedAt: now, UpdatedAt: updatedAt,
		}
	}
	renamed := card(701, 7001, "Touched lane card renamed", "doing", "Doing", now.Add(3*time.Minute))
	if _, err := store.ReconcileIssue(ctx, 701, &renamed, now.Add(3*time.Minute)); err != nil {
		t.Fatalf("reconcile issue in the touched lane: %v", err)
	}

	quietAfter := laneRowVersions(t, ctx, pool, "review")
	for issueIID, before := range quietBefore {
		if quietAfter[issueIID] != before {
			t.Fatalf("card %d in the untouched review lane was rewritten (xmin %s -> %s)", issueIID, before, quietAfter[issueIID])
		}
	}
	assertStoredOrder(t, ctx, pool, "review", []int64{702, 703})
	assertStoredOrder(t, ctx, pool, "doing", []int64{701})
}

func laneRowVersions(t *testing.T, ctx context.Context, pool *pgxpool.Pool, listKey string) map[int64]string {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT issue_iid, xmin::text FROM issue_cache WHERE list_key = $1`, listKey)
	if err != nil {
		t.Fatalf("read lane row versions: %v", err)
	}
	defer rows.Close()
	versions := make(map[int64]string)
	for rows.Next() {
		var issueIID int64
		var version string
		if err := rows.Scan(&issueIID, &version); err != nil {
			t.Fatalf("scan lane row version: %v", err)
		}
		versions[issueIID] = version
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate lane row versions: %v", err)
	}
	return versions
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

	// A read that reports exactly what the cache already holds must stay silent. This
	// used to be satisfied by a content hash of the whole board short-circuiting the
	// merge before it began; now the merge runs and the per-row value comparison is
	// what keeps it quiet, so the fixture has to match the seeded rows value for
	// value rather than merely hash to the same thing.
	unchangedFirst := card(601, 6001, "First snapshot card", "doing", "Doing", now)
	unchangedSecond := card(602, 6002, "Second snapshot card", "doing", "Doing", now)
	revisionBefore, err := store.Revision(ctx)
	if err != nil {
		t.Fatalf("load revision before identical snapshot: %v", err)
	}
	if err := replaceBoard(t, ctx, store, []domainboard.Card{unchangedSecond, unchangedFirst}, now.Add(time.Minute)); err != nil {
		t.Fatalf("replace identical board snapshot: %v", err)
	}
	revisionAfter, err := store.Revision(ctx)
	if err != nil || revisionAfter != revisionBefore {
		t.Fatalf("identical snapshot revision = %q, error %v, want %q", revisionAfter, err, revisionBefore)
	}
	assertStoredOrder(t, ctx, pool, "doing", []int64{601, 602})

	if err := replaceBoard(t, ctx, store, []domainboard.Card{second, first}, now.Add(2*time.Minute)); err != nil {
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
	watermarkBeforeStaleMerge := boardWatermark(t, ctx, pool)
	if err := replaceBoard(t, ctx, store, []domainboard.Card{second, first}, now.Add(3*time.Minute)); err != nil {
		t.Fatalf("merge stale board snapshot: %v", err)
	}
	if stored, err := store.Card(ctx, 601); err != nil || stored.ListKey != "review" || stored.Position != 0 {
		t.Fatalf("stale snapshot overwrote completed move: card %#v, error %v", stored, err)
	}

	if err := replaceBoard(t, ctx, store, []domainboard.Card{second}, now.Add(3*time.Minute)); err != nil {
		t.Fatalf("merge stale snapshot missing a recently updated card: %v", err)
	}
	if _, err := store.Card(ctx, 601); err != nil {
		t.Fatalf("stale snapshot deleted a recently updated card: %v", err)
	}
	watermarkAfterStaleMerge := boardWatermark(t, ctx, pool)
	// This assertion used to demand the opposite: a merge that skipped any row stored
	// the *old* revision so the next poll would redo the whole board. Because the
	// revision was a content hash of the whole board, one skipped row made the hash a
	// lie and the only repair available was a board-wide retry -- which, paired with a
	// row whose GitLab timestamp stayed ahead, re-merged every five seconds forever.
	//
	// Every skip is self-resolving instead. A row with a local mutation in flight is
	// superseded when the operation worker completes it, and a row we already hold
	// newer data for needs no repair at all. So the cursor advances and the treadmill
	// is gone.
	if watermarkAfterStaleMerge == watermarkBeforeStaleMerge {
		t.Fatalf("cursor stayed at %q; a row skipped as stale must not hold it back", watermarkAfterStaleMerge)
	}
	if parsed, err := time.Parse(time.RFC3339Nano, watermarkAfterStaleMerge); err != nil || !parsed.Equal(now.Add(3*time.Minute)) {
		t.Fatalf("cursor = %q (%v), want the boundary this read reached", watermarkAfterStaleMerge, err)
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

// replaceBoard hands the store a full-board read. StartedAt is synthetic here rather
// than PostgreSQL's real clock because these fixtures live at a fixed date in the
// past; a real now() would sit after every seeded gitlab_observed_at and make every
// card look prunable.
func replaceBoard(t *testing.T, ctx context.Context, store *pgsitcon.Repository, cards []domainboard.Card, syncedAt time.Time) error {
	t.Helper()
	return store.ApplyBoardObservation(ctx, domainboard.BoardObservation{
		Cards: cards, Complete: true, Watermark: &syncedAt,
		StartedAt: syncedAt.Add(-time.Second), SyncedAt: syncedAt,
	})
}

// boardWatermark reads the incremental cursor, which is stored as RFC3339Nano where a
// content hash of the whole board used to live.
func boardWatermark(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()
	var raw string
	if err := pool.QueryRow(ctx, `SELECT source_revision FROM sync_snapshots WHERE resource = 'board'`).Scan(&raw); err != nil {
		t.Fatalf("load board watermark: %v", err)
	}
	return raw
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
