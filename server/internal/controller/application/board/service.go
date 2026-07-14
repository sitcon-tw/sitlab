package board

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"example.com/project-template/internal/controller/application/apperror"
	domain "example.com/project-template/internal/domain/board"
)

type Service struct {
	repo      Repository
	directory Directory
	now       func() time.Time
	tracer    trace.Tracer
}

func NewService(repo Repository, directory Directory, tracer trace.Tracer) *Service {
	return &Service{repo: repo, directory: directory, now: time.Now, tracer: tracer}
}

func (s *Service) Board(ctx context.Context) (Snapshot, error) {
	ctx, span := s.tracer.Start(ctx, "board.snapshot")
	defer span.End()
	snapshot, err := s.repo.Board(ctx)
	if errors.Is(err, domain.ErrSnapshotNotFound) {
		return Snapshot{}, apperror.Unavailable("board snapshot is not ready")
	}
	if err != nil {
		return Snapshot{}, technical(span, "load board snapshot", err)
	}
	return snapshot, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Result, error) {
	ctx, span := s.tracer.Start(ctx, "board.create_card")
	defer span.End()
	if result, done, err := s.idempotent(ctx, input.OperationID, domain.OperationCreateCard); done {
		return result, err
	}
	if err := validateMutationIdentity(input.OperationID, input.ActorUserID); err != nil {
		return Result{}, err
	}
	title := domain.NormalizeTitle(input.Title)
	if !domain.ValidTitle(title) {
		return Result{}, invalidField("title", "INVALID_LENGTH", "must be between 1 and 255 characters")
	}
	directory, err := s.directory.Snapshot(ctx)
	if err != nil {
		return Result{}, technical(span, "load directory snapshot", err)
	}
	if !directory.TeamExists(input.TeamKey) {
		return Result{}, unknownTeam("teamKey")
	}
	assigneeIDs := domain.NormalizeAssigneeIDs(input.AssigneeGitLabUserIDs)
	if err := domain.ValidateAssignees(directory, assigneeIDs); err != nil {
		return Result{}, unknownAssignee()
	}
	dueDate, err := normalizeDueDate(input.DueDate)
	if err != nil {
		return Result{}, err
	}
	boardSnapshot, err := s.repo.Board(ctx)
	if errors.Is(err, domain.ErrSnapshotNotFound) {
		return Result{}, apperror.Unavailable("board snapshot is not ready")
	}
	if err != nil {
		return Result{}, technical(span, "load board lists", err)
	}
	if len(boardSnapshot.Lists) == 0 {
		return Result{}, apperror.Unavailable("board snapshot is not ready")
	}

	now := s.now().UTC()
	operation := newOperation(input.OperationID, domain.OperationCreateCard, now)
	card := domain.Card{
		Title: title, Description: input.Description, ListKey: boardSnapshot.Lists[0].Key, TeamKey: input.TeamKey,
		AssigneeGitLabUserIDs: append([]int64(nil), assigneeIDs...), DueDate: dueDate,
		SyncState: domain.OperationPending, PendingOperationID: input.OperationID, CreatedAt: now, UpdatedAt: now,
	}
	result, err := s.repo.CreateCard(ctx, Mutation{
		Card: card, Operation: operation, RequestedByUserID: input.ActorUserID,
		Payload: map[string]any{"title": title, "description": input.Description, "teamKey": input.TeamKey, "assigneeGitLabUserIds": assigneeIDs, "dueDate": nullableDate(dueDate)},
	})
	if errors.Is(err, domain.ErrOperationConflict) {
		return Result{}, operationConflict()
	}
	if err != nil {
		return Result{}, technical(span, "create optimistic card", err)
	}
	return result, nil
}

func (s *Service) UpdateDetails(ctx context.Context, input UpdateDetailsInput) (Result, error) {
	return s.update(ctx, input.OperationID, input.ActorUserID, input.IssueIID, domain.OperationUpdateDetails, func(card *domain.Card, _ domain.AssignmentDirectory, _ Snapshot) (map[string]any, error) {
		title := domain.NormalizeTitle(input.Title)
		if !domain.ValidTitle(title) {
			return nil, invalidField("title", "INVALID_LENGTH", "must be between 1 and 255 characters")
		}
		card.Title = title
		card.Description = input.Description
		return map[string]any{"title": title, "description": input.Description}, nil
	})
}

func (s *Service) UpdateTeam(ctx context.Context, input UpdateTeamInput) (Result, error) {
	return s.update(ctx, input.OperationID, input.ActorUserID, input.IssueIID, domain.OperationUpdateTeam, func(card *domain.Card, directorySnapshot domain.AssignmentDirectory, _ Snapshot) (map[string]any, error) {
		assigneeIDs, _, err := domain.ReconcileAssignees(directorySnapshot, input.TeamKey, card.AssigneeGitLabUserIDs)
		if errors.Is(err, domain.ErrTeamNotFound) {
			return nil, unknownTeam("teamKey")
		}
		if err != nil {
			return nil, err
		}
		card.TeamKey = input.TeamKey
		card.AssigneeGitLabUserIDs = append([]int64(nil), assigneeIDs...)
		return map[string]any{"teamKey": input.TeamKey, "assigneeGitLabUserIds": assigneeIDs}, nil
	})
}

func (s *Service) UpdateAssignee(ctx context.Context, input UpdateAssigneeInput) (Result, error) {
	return s.update(ctx, input.OperationID, input.ActorUserID, input.IssueIID, domain.OperationUpdateAssignee, func(card *domain.Card, directorySnapshot domain.AssignmentDirectory, _ Snapshot) (map[string]any, error) {
		assigneeIDs := domain.NormalizeAssigneeIDs(input.AssigneeGitLabUserIDs)
		if err := domain.ValidateAssignees(directorySnapshot, assigneeIDs); err != nil {
			return nil, unknownAssignee()
		}
		card.AssigneeGitLabUserIDs = append([]int64(nil), assigneeIDs...)
		return map[string]any{"assigneeGitLabUserIds": assigneeIDs}, nil
	})
}

func (s *Service) UpdateDueDate(ctx context.Context, input UpdateDueDateInput) (Result, error) {
	return s.update(ctx, input.OperationID, input.ActorUserID, input.IssueIID, domain.OperationUpdateDueDate, func(card *domain.Card, _ domain.AssignmentDirectory, _ Snapshot) (map[string]any, error) {
		dueDate, err := normalizeDueDate(input.DueDate)
		if err != nil {
			return nil, err
		}
		card.DueDate = dueDate
		return map[string]any{"dueDate": nullableDate(dueDate)}, nil
	})
}

func (s *Service) Move(ctx context.Context, input MoveInput) (Result, error) {
	return s.update(ctx, input.OperationID, input.ActorUserID, input.IssueIID, domain.OperationMoveCard, func(card *domain.Card, _ domain.AssignmentDirectory, boardSnapshot Snapshot) (map[string]any, error) {
		if input.Position < 0 {
			return nil, invalidField("position", "INVALID_VALUE", "must be zero or greater")
		}
		found := false
		for _, list := range boardSnapshot.Lists {
			if list.Key == input.ListKey {
				found = true
				break
			}
		}
		if !found {
			return nil, invalidField("listKey", "INVALID_VALUE", "must identify an active board list")
		}
		card.ListKey, card.Position = input.ListKey, input.Position
		return map[string]any{"listKey": input.ListKey, "position": input.Position}, nil
	})
}

func (s *Service) Retry(ctx context.Context, operationID string) (domain.Operation, error) {
	ctx, span := s.tracer.Start(ctx, "board.retry_operation")
	defer span.End()
	if _, err := uuid.Parse(operationID); err != nil {
		return domain.Operation{}, invalidField("path.operationId", "INVALID_FORMAT", "must be a UUID")
	}
	operation, err := s.repo.RetryOperation(ctx, operationID)
	if errors.Is(err, domain.ErrOperationNotFound) {
		return domain.Operation{}, apperror.NotFound("operation")
	}
	if errors.Is(err, domain.ErrOperationConflict) {
		return domain.Operation{}, apperror.Conflict("OPERATION_CONFLICT", "only failed operations can be retried")
	}
	if err != nil {
		return domain.Operation{}, technical(span, "retry durable operation", err)
	}
	return operation, nil
}

type cardChange func(*domain.Card, domain.AssignmentDirectory, Snapshot) (map[string]any, error)

func (s *Service) update(ctx context.Context, operationID, actorUserID string, issueIID int64, kind domain.OperationKind, change cardChange) (Result, error) {
	ctx, span := s.tracer.Start(ctx, "board."+string(kind))
	defer span.End()
	if result, done, err := s.idempotent(ctx, operationID, kind); done {
		return result, err
	}
	if err := validateMutationIdentity(operationID, actorUserID); err != nil {
		return Result{}, err
	}
	card, err := s.repo.Card(ctx, issueIID)
	if errors.Is(err, domain.ErrCardNotFound) {
		return Result{}, apperror.NotFound("card")
	}
	if err != nil {
		return Result{}, technical(span, "load card", err)
	}
	directorySnapshot, err := s.directory.Snapshot(ctx)
	if err != nil {
		return Result{}, technical(span, "load directory snapshot", err)
	}
	boardSnapshot, err := s.repo.Board(ctx)
	if errors.Is(err, domain.ErrSnapshotNotFound) {
		return Result{}, apperror.Unavailable("board snapshot is not ready")
	}
	if err != nil {
		return Result{}, technical(span, "load board snapshot", err)
	}
	payload, err := change(&card, directorySnapshot, boardSnapshot)
	if err != nil {
		return Result{}, err
	}
	now := s.now().UTC()
	card.SyncState, card.SyncError, card.PendingOperationID, card.UpdatedAt = domain.OperationPending, "", operationID, now
	iid := issueIID
	operation := newOperation(operationID, kind, now)
	operation.IssueIID = &iid
	result, err := s.repo.UpdateCard(ctx, Mutation{Card: card, Operation: operation, RequestedByUserID: actorUserID, Payload: payload})
	if errors.Is(err, domain.ErrOperationConflict) {
		return Result{}, operationConflict()
	}
	if err != nil {
		return Result{}, technical(span, "store optimistic mutation", err)
	}
	return result, nil
}

func (s *Service) idempotent(ctx context.Context, operationID string, kind domain.OperationKind) (Result, bool, error) {
	if _, err := uuid.Parse(operationID); err != nil {
		return Result{}, true, invalidField("operationId", "INVALID_FORMAT", "must be a UUID")
	}
	result, err := s.repo.ByOperation(ctx, operationID)
	if errors.Is(err, domain.ErrOperationNotFound) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, true, fmt.Errorf("load operation for idempotency: %w", err)
	}
	if result.Operation.Kind != kind {
		return Result{}, true, operationConflict()
	}
	return result, true, nil
}

func validateMutationIdentity(operationID, actorUserID string) error {
	if _, err := uuid.Parse(operationID); err != nil {
		return invalidField("operationId", "INVALID_FORMAT", "must be a UUID")
	}
	if _, err := uuid.Parse(actorUserID); err != nil {
		return apperror.Unauthorized("AUTH_INVALID_SESSION", "session user is invalid")
	}
	return nil
}

func normalizeDueDate(value *string) (string, error) {
	if value == nil || *value == "" {
		return "", nil
	}
	parsed, err := time.Parse(time.DateOnly, *value)
	if err != nil || parsed.Format(time.DateOnly) != *value {
		return "", invalidField("dueDate", "INVALID_FORMAT", "must use YYYY-MM-DD")
	}
	return *value, nil
}

func nullableDate(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func newOperation(id string, kind domain.OperationKind, now time.Time) domain.Operation {
	return domain.Operation{ID: id, Kind: kind, State: domain.OperationPending, CreatedAt: now, UpdatedAt: now}
}

func unknownTeam(name string) error {
	return apperror.Invalid("TEAM_NOT_FOUND", "team does not exist or is inactive", apperror.Field{Name: name, Code: "UNKNOWN_TEAM", Message: "must identify an active team"})
}

func unknownAssignee() error {
	return apperror.Invalid("MEMBER_NOT_ASSIGNABLE", "an assignee is not an active GitLab project member", apperror.Field{Name: "assigneeGitLabUserIds", Code: "UNKNOWN_MEMBER", Message: "must contain only active project members"})
}

func invalidField(name, code, message string) error {
	return apperror.Invalid("VALIDATION_FAILED", "card input is invalid", apperror.Field{Name: name, Code: code, Message: message})
}

func operationConflict() error {
	return apperror.Conflict("OPERATION_CONFLICT", "operationId was already used for another mutation")
}

func technical(span trace.Span, action string, err error) error {
	span.RecordError(err)
	span.SetStatus(codes.Error, action)
	return fmt.Errorf("%s: %w", action, err)
}
