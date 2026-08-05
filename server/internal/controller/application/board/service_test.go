package board

import (
	"context"
	"errors"
	"reflect"
	"slices"
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
			{Key: "development", GitLabLabel: "組別::開發", Active: true},
			{Key: "design", GitLabLabel: "組別::設計", Active: true},
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
	repo := &repositoryFake{board: Snapshot{Lists: []domain.List{{Key: "wating"}, {Key: "todo"}}}}
	service := newTestService(repo)
	assignees := []int64{1, 1}
	startDate := "2026-07-17"
	dueDate := "2026-07-21"

	result, err := service.Create(context.Background(), CreateInput{
		OperationID: testOperationID, ActorUserID: testActorID, Title: "修正  報名流程",
		Description: "詳細規劃", TeamKey: "development", ListKey: "todo", AssigneeGitLabUserIDs: assignees, Labels: []string{"Backend", "Backend"}, StartDate: &startDate, DueDate: &dueDate,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.Card.IssueIID != -1 || result.Card.SyncState != domain.OperationPending || result.Card.Position != 0 || result.Card.ListKey != "todo" {
		t.Fatalf("Create() card = %#v", result.Card)
	}
	if repo.createMutation == nil || repo.createMutation.Card.Title != "修正 報名流程" || repo.createMutation.Operation.Kind != domain.OperationCreateCard {
		t.Fatalf("stored mutation = %#v", repo.createMutation)
	}
	if got := repo.createMutation.Payload["assigneeGitLabUserIds"]; !reflect.DeepEqual(got, []int64{1}) {
		t.Fatalf("assignee payload = %#v", got)
	}
	if got := repo.createMutation.Payload["listKey"]; got != "todo" {
		t.Fatalf("list payload = %#v", got)
	}
	if got := repo.createMutation.Payload["labels"]; !reflect.DeepEqual(got, []string{"Backend", "組別::開發"}) {
		t.Fatalf("labels payload = %#v", got)
	}
	if result.Card.Description != "詳細規劃" || len(result.Card.AssigneeGitLabUserIDs) != 1 || result.Card.StartDate != startDate {
		t.Fatalf("Create() card details = %#v", result.Card)
	}
}

func TestCreateRejectsEmptyLabelBeforePersistence(t *testing.T) {
	t.Parallel()
	repo := &repositoryFake{board: Snapshot{Lists: []domain.List{{Key: "inbox"}}}}
	service := newTestService(repo)
	_, err := service.Create(context.Background(), CreateInput{
		OperationID: testOperationID, ActorUserID: testActorID, Title: "修正流程", TeamKey: "development", ListKey: "inbox", Labels: []string{" "},
	})
	assertAppError(t, err, apperror.KindInvalid, "VALIDATION_FAILED")
	if repo.createMutation != nil {
		t.Fatal("card with an empty label was persisted")
	}
}

func TestCreateRejectsUnknownListBeforePersistence(t *testing.T) {
	t.Parallel()
	repo := &repositoryFake{board: Snapshot{Lists: []domain.List{{Key: "inbox"}}}}
	service := newTestService(repo)
	_, err := service.Create(context.Background(), CreateInput{
		OperationID: testOperationID, ActorUserID: testActorID, Title: "修正流程", TeamKey: "development", ListKey: "unknown",
	})
	assertAppError(t, err, apperror.KindInvalid, "VALIDATION_FAILED")
	if repo.createMutation != nil {
		t.Fatal("card with an unknown list was persisted")
	}
}

func TestUpdateStartDateValidatesAndStoresDate(t *testing.T) {
	t.Parallel()
	repo := &repositoryFake{
		board: Snapshot{Lists: []domain.List{{Key: "todo"}}},
		card:  domain.Card{IssueIID: 42, TeamKey: "development", Title: "卡片"},
	}
	service := newTestService(repo)
	startDate := "2026-07-18"
	result, err := service.UpdateStartDate(context.Background(), UpdateStartDateInput{
		OperationID: testOperationID, ActorUserID: testActorID, IssueIID: 42, StartDate: &startDate,
	})
	if err != nil || result.Card.StartDate != startDate || result.Operation.Kind != domain.OperationUpdateStartDate {
		t.Fatalf("UpdateStartDate() = %#v, %v", result, err)
	}
	invalid := "07/18/2026"
	_, err = service.UpdateStartDate(context.Background(), UpdateStartDateInput{
		OperationID: "20000000-0000-0000-0000-000000000001", ActorUserID: testActorID, IssueIID: 42, StartDate: &invalid,
	})
	assertAppError(t, err, apperror.KindInvalid, "VALIDATION_FAILED")
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
		board: Snapshot{Lists: []domain.List{{Key: "todo", GitLabLabel: "Status::To Do"}}},
		card:  domain.Card{IssueIID: 42, TeamKey: "development", ListKey: "todo", AssigneeGitLabUserIDs: []int64{1}},
	}
	service := newTestService(repo)
	result, err := service.UpdateTeam(context.Background(), UpdateTeamInput{
		OperationID: testOperationID, ActorUserID: testActorID, IssueIID: 42, TeamKey: "design",
	})
	if err != nil {
		t.Fatalf("UpdateTeam() error = %v", err)
	}
	if len(result.Card.AssigneeGitLabUserIDs) != 0 || result.Card.TeamKey != "design" ||
		!slices.Equal(result.Card.Labels, []string{"組別::設計", "Status::To Do"}) {
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

func TestUpdateLabelsNormalizesScopedLabels(t *testing.T) {
	t.Parallel()
	directorySnapshot := directory.Snapshot{
		Teams: []directory.Team{
			{Key: "development", GitLabLabel: "組別::開發", Active: true},
			{Key: "design", GitLabLabel: "組別::設計", Active: true},
		},
		Members: []directory.Member{
			{GitLabUserID: 1, State: directory.MemberActive, TeamKeys: []string{"development"}},
			{GitLabUserID: 2, State: directory.MemberActive, TeamKeys: []string{"design"}},
		},
	}
	lists := []domain.List{
		{Key: "inbox", GitLabLabel: "Status::Inbox"},
		{Key: "todo", GitLabLabel: "Status::To Do"},
		{Key: "closed", Closed: true},
	}
	tests := []struct {
		name               string
		card               domain.Card
		labels             []string
		wantTeam, wantList string
		wantLabels         []string
		wantAssignees      []int64
		wantInvalid        bool
	}{
		{
			name:   "removing open status moves to Inbox",
			card:   domain.Card{IssueIID: 42, TeamKey: "development", ListKey: "todo", Labels: []string{"組別::開發", "Status::To Do", "Backend"}, AssigneeGitLabUserIDs: []int64{1}},
			labels: []string{"組別::開發", "Backend"}, wantTeam: "development", wantList: "inbox",
			wantLabels: []string{"Backend", "組別::開發", "Status::Inbox"}, wantAssignees: []int64{1},
		},
		{
			name:   "new scopes replace team and close card",
			card:   domain.Card{IssueIID: 42, TeamKey: "development", ListKey: "todo", AssigneeGitLabUserIDs: []int64{1}},
			labels: []string{"組別::設計", "Closed", "Backend"}, wantTeam: "design", wantList: "closed",
			wantLabels: []string{"Backend", "組別::設計"}, wantAssignees: []int64{},
		},
		{
			name:   "closed card stays closed without a status label",
			card:   domain.Card{IssueIID: 42, TeamKey: "development", ListKey: "closed"},
			labels: []string{"組別::開發", "Backend"}, wantTeam: "development", wantList: "closed",
			wantLabels: []string{"Backend", "組別::開發"}, wantAssignees: []int64{},
		},
		{
			name: "team cannot be empty", card: domain.Card{IssueIID: 42, TeamKey: "development", ListKey: "todo"},
			labels: []string{"Backend"}, wantInvalid: true,
		},
		{
			name: "team scope cannot contain multiple labels", card: domain.Card{IssueIID: 42, TeamKey: "development", ListKey: "todo"},
			labels: []string{"組別::開發", "組別::設計", "Status::To Do"}, wantInvalid: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &repositoryFake{board: Snapshot{Lists: lists, Cards: []domain.Card{tt.card}}, card: tt.card}
			service := NewService(repo, directoryFake{snapshot: directorySnapshot}, noop.NewTracerProvider().Tracer("test"))
			result, err := service.UpdateLabels(context.Background(), UpdateLabelsInput{
				OperationID: testOperationID, ActorUserID: testActorID, IssueIID: 42, Labels: tt.labels,
			})
			if tt.wantInvalid {
				assertAppError(t, err, apperror.KindInvalid, "VALIDATION_FAILED")
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.Card.TeamKey != tt.wantTeam || result.Card.ListKey != tt.wantList ||
				!reflect.DeepEqual(result.Card.Labels, tt.wantLabels) || !slices.Equal(result.Card.AssigneeGitLabUserIDs, tt.wantAssignees) ||
				result.Operation.Kind != domain.OperationUpdateLabels {
				t.Fatalf("UpdateLabels() card = %#v operation = %#v", result.Card, result.Operation)
			}
		})
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
