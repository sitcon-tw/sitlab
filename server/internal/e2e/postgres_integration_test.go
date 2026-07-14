//go:build integration

package e2e_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"example.com/project-template/internal/controller/infrastructure/postgres"
	pgauth "example.com/project-template/internal/controller/infrastructure/postgres/auth"
	pgtask "example.com/project-template/internal/controller/infrastructure/postgres/task"
	pgworkspace "example.com/project-template/internal/controller/infrastructure/postgres/workspace"
	"example.com/project-template/internal/domain/identity"
	domaintask "example.com/project-template/internal/domain/task"
	domainworkspace "example.com/project-template/internal/domain/workspace"
)

func TestPostgresMigrationsRepositoriesAndTransactions(t *testing.T) {
	databaseURL := os.Getenv("PROJECT_TEMPLATE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PROJECT_TEMPLATE_TEST_DATABASE_URL is not set")
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
	if _, err := db.ExecContext(ctx, "TRUNCATE tasks, workspace_members, workspaces, auth_sessions, users CASCADE"); err != nil {
		t.Fatal(err)
	}

	pool, err := postgres.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	authRepo := pgauth.New(pool)
	workspaceRepo := pgworkspace.New(pool)
	taskRepo := pgtask.New(pool)
	tx := postgres.NewTransactor(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)
	owner := createIntegrationUser(t, ctx, authRepo, "owner@example.com", now)
	nonMember := createIntegrationUser(t, ctx, authRepo, "outsider@example.com", now)
	workspaceID := uuid.NewString()
	err = tx.WithinTx(ctx, func(txCtx context.Context) error {
		_, err := workspaceRepo.Create(txCtx, domainworkspace.Workspace{ID: workspaceID, Name: "Engineering", Role: domainworkspace.RoleOwner, CreatedByUserID: owner.ID, CreatedAt: now, UpdatedAt: now})
		if err != nil {
			return err
		}
		if err := workspaceRepo.CreateMember(txCtx, domainworkspace.Member{WorkspaceID: workspaceID, UserID: owner.ID, Role: domainworkspace.RoleOwner, JoinedAt: now}); err != nil {
			return err
		}
		_, err = workspaceRepo.GetForUser(txCtx, workspaceID, owner.ID)
		return err
	})
	if err != nil {
		t.Fatalf("committed workspace transaction: %v", err)
	}
	if _, err := workspaceRepo.GetForUser(ctx, workspaceID, owner.ID); err != nil {
		t.Fatalf("committed data not visible: %v", err)
	}

	rollbackID := uuid.NewString()
	rollbackSentinel := errors.New("force rollback")
	err = tx.WithinTx(ctx, func(txCtx context.Context) error {
		_, createErr := workspaceRepo.Create(txCtx, domainworkspace.Workspace{ID: rollbackID, Name: "Rollback", CreatedByUserID: owner.ID, CreatedAt: now, UpdatedAt: now})
		if createErr != nil {
			return createErr
		}
		return rollbackSentinel
	})
	if !errors.Is(err, rollbackSentinel) {
		t.Fatalf("rollback result: %v", err)
	}
	if _, err := workspaceRepo.GetForUser(ctx, rollbackID, owner.ID); !errors.Is(err, domainworkspace.ErrNotFound) {
		t.Fatalf("rolled back workspace visible: %v", err)
	}

	_, err = taskRepo.Create(ctx, domaintask.Task{ID: uuid.NewString(), WorkspaceID: workspaceID, Title: "Invalid assignment", Status: domaintask.StatusTodo, AssigneeUserID: &nonMember.ID, CreatedByUserID: owner.ID, CreatedAt: now, UpdatedAt: now})
	if !errors.Is(err, domaintask.ErrAssigneeNotMember) {
		t.Fatalf("assignee constraint mapping: %v", err)
	}
	validTask, err := taskRepo.Create(ctx, domaintask.Task{ID: uuid.NewString(), WorkspaceID: workspaceID, Title: "Valid assignment", Status: domaintask.StatusTodo, AssigneeUserID: &owner.ID, CreatedByUserID: owner.ID, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if err := workspaceRepo.DeleteMember(ctx, workspaceID, owner.ID); err != nil {
		t.Fatal(err)
	}
	validTask, err = taskRepo.Get(ctx, workspaceID, validTask.ID)
	if err != nil || validTask.AssigneeUserID != nil {
		t.Fatalf("member removal did not clear task assignee: task=%#v err=%v", validTask, err)
	}
}

func createIntegrationUser(t *testing.T, ctx context.Context, repo *pgauth.Repository, email string, now time.Time) identity.User {
	t.Helper()
	user, err := repo.CreateUser(ctx, identity.User{ID: uuid.NewString(), Email: email, PasswordHash: "integration-only", DisplayName: email, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func migrationDirectory(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test path")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "db", "migrations")
}
