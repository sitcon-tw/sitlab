package board

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"example.com/project-template/internal/controller/application/apperror"
	domain "example.com/project-template/internal/domain/board"
	domaindirectory "example.com/project-template/internal/domain/directory"
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
	team, found := directory.Team(input.TeamKey)
	if !found {
		return Result{}, unknownTeam("teamKey")
	}
	assigneeIDs := domain.NormalizeAssigneeIDs(input.AssigneeGitLabUserIDs)
	if err := domain.ValidateAssignees(directory, assigneeIDs); err != nil {
		return Result{}, unknownAssignee()
	}
	startDate, err := normalizeDate(input.StartDate, "startDate")
	if err != nil {
		return Result{}, err
	}
	dueDate, err := normalizeDate(input.DueDate, "dueDate")
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
	list, found := boardListByKey(boardSnapshot.Lists, input.ListKey)
	if !found {
		return Result{}, invalidField("listKey", "INVALID_VALUE", "must identify an active board list")
	}

	now := s.now().UTC()
	operation := newOperation(input.OperationID, domain.OperationCreateCard, now)
	card := domain.Card{
		Title: title, Description: input.Description, ListKey: input.ListKey, TeamKey: input.TeamKey,
		AssigneeGitLabUserIDs: append([]int64(nil), assigneeIDs...), StartDate: startDate, DueDate: dueDate,
		Labels:    canonicalCardLabels(nil, team.GitLabLabel, list, directory.Teams, boardSnapshot.Lists),
		SyncState: domain.OperationPending, PendingOperationID: input.OperationID, CreatedAt: now, UpdatedAt: now,
	}
	result, err := s.repo.CreateCard(ctx, Mutation{
		Card: card, Operation: operation, RequestedByUserID: input.ActorUserID,
		Payload: map[string]any{"title": title, "description": input.Description, "teamKey": input.TeamKey, "listKey": input.ListKey, "assigneeGitLabUserIds": assigneeIDs, "startDate": nullableDate(startDate), "dueDate": nullableDate(dueDate)},
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
	return s.update(ctx, input.OperationID, input.ActorUserID, input.IssueIID, domain.OperationUpdateDetails, func(card *domain.Card, _ domaindirectory.Snapshot, _ Snapshot) (map[string]any, error) {
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
	return s.update(ctx, input.OperationID, input.ActorUserID, input.IssueIID, domain.OperationUpdateTeam, func(card *domain.Card, directorySnapshot domaindirectory.Snapshot, boardSnapshot Snapshot) (map[string]any, error) {
		assigneeIDs, _, err := domain.ReconcileAssignees(directorySnapshot, input.TeamKey, card.AssigneeGitLabUserIDs)
		if errors.Is(err, domain.ErrTeamNotFound) {
			return nil, unknownTeam("teamKey")
		}
		if err != nil {
			return nil, err
		}
		team, found := directorySnapshot.Team(input.TeamKey)
		if !found {
			return nil, unknownTeam("teamKey")
		}
		list, found := boardListByKey(boardSnapshot.Lists, card.ListKey)
		if !found {
			return nil, invalidField("teamKey", "INVALID_VALUE", "card list is unavailable")
		}
		card.TeamKey = input.TeamKey
		card.AssigneeGitLabUserIDs = append([]int64(nil), assigneeIDs...)
		card.Labels = canonicalCardLabels(card.Labels, team.GitLabLabel, list, directorySnapshot.Teams, boardSnapshot.Lists)
		return map[string]any{"teamKey": input.TeamKey, "assigneeGitLabUserIds": assigneeIDs, "labels": card.Labels}, nil
	})
}

func (s *Service) UpdateAssignee(ctx context.Context, input UpdateAssigneeInput) (Result, error) {
	return s.update(ctx, input.OperationID, input.ActorUserID, input.IssueIID, domain.OperationUpdateAssignee, func(card *domain.Card, directorySnapshot domaindirectory.Snapshot, _ Snapshot) (map[string]any, error) {
		assigneeIDs := domain.NormalizeAssigneeIDs(input.AssigneeGitLabUserIDs)
		if err := domain.ValidateAssignees(directorySnapshot, assigneeIDs); err != nil {
			return nil, unknownAssignee()
		}
		card.AssigneeGitLabUserIDs = append([]int64(nil), assigneeIDs...)
		return map[string]any{"assigneeGitLabUserIds": assigneeIDs}, nil
	})
}

func (s *Service) UpdateDueDate(ctx context.Context, input UpdateDueDateInput) (Result, error) {
	return s.update(ctx, input.OperationID, input.ActorUserID, input.IssueIID, domain.OperationUpdateDueDate, func(card *domain.Card, _ domaindirectory.Snapshot, _ Snapshot) (map[string]any, error) {
		dueDate, err := normalizeDate(input.DueDate, "dueDate")
		if err != nil {
			return nil, err
		}
		card.DueDate = dueDate
		return map[string]any{"dueDate": nullableDate(dueDate)}, nil
	})
}

func (s *Service) UpdateStartDate(ctx context.Context, input UpdateStartDateInput) (Result, error) {
	return s.update(ctx, input.OperationID, input.ActorUserID, input.IssueIID, domain.OperationUpdateStartDate, func(card *domain.Card, _ domaindirectory.Snapshot, _ Snapshot) (map[string]any, error) {
		startDate, err := normalizeDate(input.StartDate, "startDate")
		if err != nil {
			return nil, err
		}
		card.StartDate = startDate
		return map[string]any{"startDate": nullableDate(startDate)}, nil
	})
}

func (s *Service) UpdateLabels(ctx context.Context, input UpdateLabelsInput) (Result, error) {
	return s.update(ctx, input.OperationID, input.ActorUserID, input.IssueIID, domain.OperationUpdateLabels, func(card *domain.Card, directorySnapshot domaindirectory.Snapshot, boardSnapshot Snapshot) (map[string]any, error) {
		labels, valid := domain.NormalizeLabels(input.Labels)
		if !valid {
			return nil, invalidField("labels", "INVALID_VALUE", "must contain non-empty label names")
		}

		team, teamCount := selectedTeam(labels, directorySnapshot.Teams)
		if teamCount != 1 {
			return nil, invalidField("labels", "INVALID_VALUE", "must contain exactly one active team label")
		}
		list, listCount, unsupportedStatus := selectedList(labels, boardSnapshot.Lists)
		if unsupportedStatus || listCount > 1 {
			return nil, invalidField("labels", "INVALID_VALUE", "must contain at most one supported status label")
		}
		if listCount == 0 {
			current, found := boardListByKey(boardSnapshot.Lists, card.ListKey)
			if !found {
				return nil, invalidField("labels", "INVALID_VALUE", "card list is unavailable")
			}
			if current.Closed {
				list = current
			} else {
				list, found = boardListByKey(boardSnapshot.Lists, "inbox")
				if !found {
					return nil, invalidField("labels", "INVALID_VALUE", "Inbox board list is unavailable")
				}
			}
		}

		assigneeIDs, _, err := domain.ReconcileAssignees(directorySnapshot, team.Key, card.AssigneeGitLabUserIDs)
		if err != nil {
			return nil, unknownAssignee()
		}
		if card.ListKey != list.Key {
			card.Position = nextListPosition(boardSnapshot.Cards, list.Key, card.IssueIID)
		}
		card.TeamKey = team.Key
		card.ListKey = list.Key
		card.AssigneeGitLabUserIDs = append([]int64(nil), assigneeIDs...)
		card.Labels = canonicalCardLabels(labels, team.GitLabLabel, list, directorySnapshot.Teams, boardSnapshot.Lists)
		return map[string]any{
			"labels": card.Labels, "teamKey": card.TeamKey, "listKey": card.ListKey,
			"position": card.Position, "assigneeGitLabUserIds": card.AssigneeGitLabUserIDs,
		}, nil
	})
}

func (s *Service) Move(ctx context.Context, input MoveInput) (Result, error) {
	return s.update(ctx, input.OperationID, input.ActorUserID, input.IssueIID, domain.OperationMoveCard, func(card *domain.Card, directorySnapshot domaindirectory.Snapshot, boardSnapshot Snapshot) (map[string]any, error) {
		if input.Position < 0 {
			return nil, invalidField("position", "INVALID_VALUE", "must be zero or greater")
		}
		list, found := boardListByKey(boardSnapshot.Lists, input.ListKey)
		if !found {
			return nil, invalidField("listKey", "INVALID_VALUE", "must identify an active board list")
		}
		team, found := directorySnapshot.Team(card.TeamKey)
		if !found {
			return nil, unknownTeam("teamKey")
		}
		card.ListKey, card.Position = input.ListKey, input.Position
		card.Labels = canonicalCardLabels(card.Labels, team.GitLabLabel, list, directorySnapshot.Teams, boardSnapshot.Lists)
		return map[string]any{"listKey": input.ListKey, "position": input.Position, "labels": card.Labels}, nil
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

type cardChange func(*domain.Card, domaindirectory.Snapshot, Snapshot) (map[string]any, error)

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

func selectedTeam(labels []string, teams []domaindirectory.Team) (domaindirectory.Team, int) {
	var selected domaindirectory.Team
	count := 0
	for _, team := range teams {
		if !team.Active {
			continue
		}
		for _, label := range labels {
			if label == team.GitLabLabel {
				selected = team
				count++
				break
			}
		}
	}
	return selected, count
}

func selectedList(labels []string, lists []domain.List) (domain.List, int, bool) {
	selectedKeys := make(map[string]struct{})
	for _, label := range labels {
		matched := false
		for _, list := range lists {
			if list.GitLabLabel != "" && label == list.GitLabLabel {
				selectedKeys[list.Key] = struct{}{}
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if key, legacy := domain.LegacyStatusListKey(label); legacy {
			selectedKeys[key] = struct{}{}
			continue
		}
		if strings.HasPrefix(label, "Status::") {
			return domain.List{}, 0, true
		}
	}
	for _, list := range lists {
		if _, exists := selectedKeys[list.Key]; exists {
			return list, len(selectedKeys), false
		}
	}
	return domain.List{}, len(selectedKeys), false
}

func boardListByKey(lists []domain.List, key string) (domain.List, bool) {
	for _, list := range lists {
		if list.Key == key {
			return list, true
		}
	}
	return domain.List{}, false
}

func nextListPosition(cards []domain.Card, listKey string, issueIID int64) int32 {
	var position int32
	for _, card := range cards {
		if card.ListKey == listKey && card.IssueIID != issueIID {
			position++
		}
	}
	return position
}

func canonicalCardLabels(labels []string, teamLabel string, list domain.List, teams []domaindirectory.Team, lists []domain.List) []string {
	teamLabels := make([]string, 0, len(teams))
	for _, team := range teams {
		teamLabels = append(teamLabels, team.GitLabLabel)
	}
	listLabels := make([]string, 0, len(lists))
	for _, candidate := range lists {
		if candidate.GitLabLabel != "" {
			listLabels = append(listLabels, candidate.GitLabLabel)
		}
	}
	listLabel := ""
	if !list.Closed {
		listLabel = list.GitLabLabel
	}
	return domain.CanonicalLabels(labels, teamLabel, listLabel, teamLabels, listLabels)
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

func normalizeDate(value *string, field string) (string, error) {
	if value == nil || *value == "" {
		return "", nil
	}
	parsed, err := time.Parse(time.DateOnly, *value)
	if err != nil || parsed.Format(time.DateOnly) != *value {
		return "", invalidField(field, "INVALID_FORMAT", "must use YYYY-MM-DD")
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
