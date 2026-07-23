//go:build integration

package e2e_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
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
		TRUNCATE durable_operations, issue_cache, board_lists, user_preferences,
		         directory_team_memberships, directory_members, directory_teams,
		         sync_snapshots, oauth_states, auth_sessions, users
		RESTART IDENTITY CASCADE
	`); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, time.July, 14, 8, 0, 0, 0, time.UTC)
	oauthRepo := pgoauth.New(pool)
	store := pgsitcon.New(pool)
	user, err := oauthRepo.UpsertUser(ctx, identity.User{
		ID: uuid.NewString(), GitLabUserID: 101, Username: "alice", DisplayName: "Alice",
		ProfileURL: "https://gitlab.com/alice", AccessLevel: 40, CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	seedSnapshots(t, ctx, pool, now)

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
		Description: "詳細規劃", TeamKey: "development", AssigneeGitLabUserIDs: []int64{101}, StartDate: &startDate, DueDate: &dueDate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Card.IssueIID >= 0 || created.Card.PendingOperationID != operationID {
		t.Fatalf("optimistic card = %#v", created.Card)
	}
	idempotent, err := boardService.Create(ctx, appboard.CreateInput{OperationID: operationID})
	if err != nil || idempotent.Card.IssueIID != created.Card.IssueIID {
		t.Fatalf("idempotent create = %#v, err = %v", idempotent, err)
	}
	gitlab := &operationGitLabFake{now: now.Add(time.Minute)}
	syncService := appsync.NewService(gitlab, operationDirectoryFake{}, store, nil, noop.NewTracerProvider().Tracer("test"))
	processed, err := syncService.ProcessOne(ctx)
	if err != nil || !processed || gitlab.lastMutation == nil || !gitlab.lastMutation.Create {
		t.Fatalf("process create = %v, %v, mutation=%#v", processed, err, gitlab.lastMutation)
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
}

type operationGitLabFake struct {
	now          time.Time
	lastMutation *appsync.IssueMutation
}

type operationDirectoryFake struct{}

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
func (f *operationGitLabFake) ApplyIssue(_ context.Context, mutation appsync.IssueMutation) (appsync.GitLabIssue, error) {
	f.lastMutation = &mutation
	return appsync.GitLabIssue{
		IssueIID: 42, GitLabIssueID: 420, Title: mutation.Title, Description: mutation.Description,
		WebURL: "https://gitlab.example/issues/42", Labels: mutation.Labels,
		AssigneeGitLabUserIDs: mutation.AssigneeGitLabUserIDs,
		StartDate:             mutation.StartDate, DueDate: mutation.DueDate, State: "opened", CreatedAt: f.now, UpdatedAt: f.now,
	}, nil
}

func seedSnapshots(t *testing.T, ctx context.Context, pool *pgxpool.Pool, now time.Time) {
	t.Helper()
	statements := []string{
		`INSERT INTO directory_teams
		    (key, display_name, title_prefix, gitlab_label, sort_order, active, source_revision, updated_at)
		VALUES
		    ('development', '開發組', '[開發組]', '組別::開發', 0, true, 'revision-1', $1),
		    ('design', '設計組', '[設計組]', '組別::設計', 1, true, 'revision-1', $1),
		    ('administration', '行政組', '[行政組]', '組別::行政', 2, true, 'revision-1', $1)`,
		`INSERT INTO directory_members
		    (gitlab_user_id, username, display_name, profile_url, access_level, state, last_synced_at)
		VALUES
		    (101, 'alice', 'Alice', 'https://gitlab.com/alice', 40, 'active', $1),
		    (202, 'bob', 'Bob', 'https://gitlab.com/bob', 30, 'active', $1)`,
		`INSERT INTO directory_team_memberships (team_key, gitlab_user_id, source, updated_at)
		VALUES
		    ('development', 101, 'gitlab_directory', $1),
		    ('design', 202, 'gitlab_directory', $1)`,
		`INSERT INTO board_lists (key, display_name, gitlab_label, position, closed, color, updated_at)
		VALUES
		    ('wating', 'Wating', 'Status::Waiting', 0, false, '#dc2626', $1),
		    ('inbox', 'Inbox', 'Status::Inbox', 1, false, '#64748b', $1),
		    ('todo', 'To Do', 'Status::To Do', 2, false, '#0891b2', $1),
		    ('doing', 'Doing', 'Status::Doing', 3, false, '#2563eb', $1),
		    ('review', 'Review', 'Status::Review', 4, false, '#b45309', $1),
		    ('closed', 'Closed', '', 5, true, '#15803d', $1)`,
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

func migrationDirectory(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test path")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "db", "migrations")
}
