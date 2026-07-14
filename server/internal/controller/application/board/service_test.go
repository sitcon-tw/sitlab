package board

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	"example.com/project-template/internal/controller/application/apperror"
	domain "example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/directory"
)

const (
	testOperationID = "10000000-0000-0000-0000-000000000001"
	testActorID     = "10000000-0000-0000-0000-000000000002"
)

type directoryFake struct{ snapshot directory.Snapshot }

func (f directoryFake) Snapshot(context.Context) (directory.Snapshot, error) { return f.snapshot, nil }

type repositoryFake struct {
	board          Snapshot
	card           domain.Card
	existing       *Result
	createMutation *Mutation
	updateMutation *Mutation
}

func (f *repositoryFake) Board(context.Context) (Snapshot, error) { return f.board, nil }
func (f *repositoryFake) Card(context.Context, int64) (domain.Card, error) {
	if f.card.IssueIID == 0 {
		return domain.Card{}, domain.ErrCardNotFound
	}
	return f.card, nil
}
func (f *repositoryFake) ByOperation(context.Context, string) (Result, error) {
	if f.existing == nil {
		return Result{}, domain.ErrOperationNotFound
	}
	return *f.existing, nil
}
func (f *repositoryFake) CreateCard(_ context.Context, mutation Mutation) (Result, error) {
	f.createMutation = &mutation
	mutation.Card.IssueIID = -1
	return Result{Card: mutation.Card, Operation: mutation.Operation}, nil
}
func (f *repositoryFake) UpdateCard(_ context.Context, mutation Mutation) (Result, error) {
	f.updateMutation = &mutation
	return Result{Card: mutation.Card, Operation: mutation.Operation}, nil
}
func (f *repositoryFake) RetryOperation(context.Context, string) (domain.Operation, error) {
	return domain.Operation{ID: testOperationID, State: domain.OperationPending}, nil
}

func testDirectory() directory.Snapshot {
	return directory.Snapshot{
		Teams: []directory.Team{
			{Key: "development", Active: true},
			{Key: "design", Active: true},
		},
		Members: []directory.Member{
			{GitLabUserID: 1, State: directory.MemberActive, TeamKeys: []string{"development"}},
			{GitLabUserID: 2, State: directory.MemberActive, TeamKeys: []string{"design"}},
		},
	}
}

func newTestService(repo *repositoryFake) *Service {
	service := NewService(repo, directoryFake{snapshot: testDirectory()}, noop.NewTracerProvider().Tracer("test"))
	service.now = func() time.Time { return time.Date(2026, time.July, 14, 8, 0, 0, 0, time.UTC) }
	return service
}

func TestCreateStoresOptimisticCardAndOperation(t *testing.T) {
	t.Parallel()
	repo := &repositoryFake{board: Snapshot{Lists: []domain.List{{Key: "todo"}}}}
	service := newTestService(repo)
	assignees := []int64{1, 1}
	dueDate := "2026-07-21"

	result, err := service.Create(context.Background(), CreateInput{
		OperationID: testOperationID, ActorUserID: testActorID, Title: "修正  報名流程",
		Description: "詳細規劃", TeamKey: "development", AssigneeGitLabUserIDs: assignees, DueDate: &dueDate,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.Card.IssueIID != -1 || result.Card.SyncState != domain.OperationPending {
		t.Fatalf("Create() card = %#v", result.Card)
	}
	if repo.createMutation == nil || repo.createMutation.Card.Title != "修正 報名流程" || repo.createMutation.Operation.Kind != domain.OperationCreateCard {
		t.Fatalf("stored mutation = %#v", repo.createMutation)
	}
	if got := repo.createMutation.Payload["assigneeGitLabUserIds"]; !reflect.DeepEqual(got, []int64{1}) {
		t.Fatalf("assignee payload = %#v", got)
	}
	if result.Card.Description != "詳細規劃" || len(result.Card.AssigneeGitLabUserIDs) != 1 {
		t.Fatalf("Create() card details = %#v", result.Card)
	}
}

func TestCreateRejectsInactiveAssigneeBeforePersistence(t *testing.T) {
	t.Parallel()
	repo := &repositoryFake{board: Snapshot{Lists: []domain.List{{Key: "todo"}}}}
	service := newTestService(repo)
	_, err := service.Create(context.Background(), CreateInput{
		OperationID: testOperationID, ActorUserID: testActorID, Title: "修正流程",
		TeamKey: "development", AssigneeGitLabUserIDs: []int64{99},
	})
	assertAppError(t, err, apperror.KindInvalid, "MEMBER_NOT_ASSIGNABLE")
	if repo.createMutation != nil {
		t.Fatal("invalid card was persisted")
	}
}

func TestChangingTeamClearsIncompatibleAssignee(t *testing.T) {
	t.Parallel()
	repo := &repositoryFake{
		board: Snapshot{Lists: []domain.List{{Key: "todo"}}},
		card:  domain.Card{IssueIID: 42, TeamKey: "development", AssigneeGitLabUserIDs: []int64{1}},
	}
	service := newTestService(repo)
	result, err := service.UpdateTeam(context.Background(), UpdateTeamInput{
		OperationID: testOperationID, ActorUserID: testActorID, IssueIID: 42, TeamKey: "design",
	})
	if err != nil {
		t.Fatalf("UpdateTeam() error = %v", err)
	}
	if len(result.Card.AssigneeGitLabUserIDs) != 0 || result.Card.TeamKey != "design" {
		t.Fatalf("UpdateTeam() card = %#v", result.Card)
	}
}

func TestUpdateDetailsNormalizesTitle(t *testing.T) {
	t.Parallel()
	repo := &repositoryFake{
		board: Snapshot{Lists: []domain.List{{Key: "todo"}}},
		card:  domain.Card{IssueIID: 42, TeamKey: "development", Title: "舊標題"},
	}
	service := newTestService(repo)
	result, err := service.UpdateDetails(context.Background(), UpdateDetailsInput{
		OperationID: testOperationID, ActorUserID: testActorID, IssueIID: 42,
		Title: "新  標題", Description: "工作拆解",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Card.Title != "新 標題" || result.Card.Description != "工作拆解" {
		t.Fatalf("UpdateDetails() = %#v", result.Card)
	}
}

func TestIdempotentOperationReturnsExistingResult(t *testing.T) {
	t.Parallel()
	existing := Result{Card: domain.Card{IssueIID: 42}, Operation: domain.Operation{ID: testOperationID, Kind: domain.OperationMoveCard}}
	repo := &repositoryFake{existing: &existing}
	service := newTestService(repo)
	got, err := service.Move(context.Background(), MoveInput{OperationID: testOperationID, ActorUserID: testActorID, IssueIID: 42, ListKey: "closed"})
	if err != nil || !reflect.DeepEqual(got, existing) {
		t.Fatalf("Move() = %#v, %v", got, err)
	}
	if repo.updateMutation != nil {
		t.Fatal("idempotent mutation was persisted twice")
	}
}

func TestOperationIDCannotBeReusedForAnotherKind(t *testing.T) {
	t.Parallel()
	existing := Result{Operation: domain.Operation{ID: testOperationID, Kind: domain.OperationCreateCard}}
	repo := &repositoryFake{existing: &existing}
	service := newTestService(repo)
	_, err := service.Move(context.Background(), MoveInput{OperationID: testOperationID, ActorUserID: testActorID})
	assertAppError(t, err, apperror.KindConflict, "OPERATION_CONFLICT")
}

func TestMoveRejectsUnknownList(t *testing.T) {
	t.Parallel()
	repo := &repositoryFake{board: Snapshot{Lists: []domain.List{{Key: "todo"}}}, card: domain.Card{IssueIID: 42}}
	service := newTestService(repo)
	_, err := service.Move(context.Background(), MoveInput{
		OperationID: testOperationID, ActorUserID: testActorID, IssueIID: 42, ListKey: "unknown", Position: 0,
	})
	assertAppError(t, err, apperror.KindInvalid, "VALIDATION_FAILED")
}

func TestRetryMapsRepositoryErrors(t *testing.T) {
	t.Parallel()
	repo := &retryErrorRepo{repositoryFake: repositoryFake{}, err: domain.ErrOperationConflict}
	service := NewService(repo, directoryFake{}, noop.NewTracerProvider().Tracer("test"))
	_, err := service.Retry(context.Background(), testOperationID)
	assertAppError(t, err, apperror.KindConflict, "OPERATION_CONFLICT")
}

type retryErrorRepo struct {
	repositoryFake
	err error
}

func (f *retryErrorRepo) RetryOperation(context.Context, string) (domain.Operation, error) {
	return domain.Operation{}, f.err
}

func assertAppError(t *testing.T, err error, kind apperror.Kind, code string) {
	t.Helper()
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Kind != kind || appErr.Code != code {
		t.Fatalf("error = %#v, want kind %s code %s", err, kind, code)
	}
}
