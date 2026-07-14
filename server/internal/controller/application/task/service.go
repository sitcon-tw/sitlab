package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"example.com/project-template/internal/controller/application/apperror"
	domain "example.com/project-template/internal/domain/task"
	"example.com/project-template/internal/domain/workspace"
)

type Service struct {
	repo        Repository
	permissions PermissionReader
	now         func() time.Time
	tracer      trace.Tracer
}

func NewService(repo Repository, permissions PermissionReader, tracer trace.Tracer) *Service {
	return &Service{repo: repo, permissions: permissions, now: time.Now, tracer: tracer}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (domain.Task, error) {
	ctx, span := s.tracer.Start(ctx, "task.create")
	defer span.End()
	if err := s.requireWrite(ctx, input.WorkspaceID, input.ActorUserID); err != nil {
		return domain.Task{}, err
	}
	title, err := validateTitle(input.Title)
	if err != nil {
		return domain.Task{}, err
	}
	if !domain.ValidDescription(input.Description) {
		return domain.Task{}, invalidDescription()
	}
	status := domain.StatusTodo
	if input.Status != "" {
		var ok bool
		status, ok = domain.ParseStatus(input.Status)
		if !ok {
			return domain.Task{}, invalidStatus("status")
		}
	}
	if err := validateOptionalUserID(input.AssigneeUserID); err != nil {
		return domain.Task{}, err
	}
	if err := s.requireAssigneeMember(ctx, input.WorkspaceID, input.AssigneeUserID); err != nil {
		return domain.Task{}, err
	}
	now := s.now().UTC()
	item := domain.Task{ID: uuid.NewString(), WorkspaceID: input.WorkspaceID, Title: title, Description: input.Description, Status: status, AssigneeUserID: input.AssigneeUserID, CreatedByUserID: input.ActorUserID, CreatedAt: now, UpdatedAt: now}
	item, err = s.repo.Create(ctx, item)
	if errors.Is(err, domain.ErrAssigneeNotMember) {
		return domain.Task{}, assigneeNotMember()
	}
	if err != nil {
		return domain.Task{}, technical(span, "create task", err)
	}
	return item, nil
}

func (s *Service) List(ctx context.Context, workspaceID, actorUserID, statusValue string) ([]domain.Task, error) {
	ctx, span := s.tracer.Start(ctx, "task.list")
	defer span.End()
	if err := s.requireRead(ctx, workspaceID, actorUserID); err != nil {
		return nil, err
	}
	var status *domain.Status
	if statusValue != "" {
		parsed, ok := domain.ParseStatus(statusValue)
		if !ok {
			return nil, invalidStatus("query.status")
		}
		status = &parsed
	}
	items, err := s.repo.List(ctx, workspaceID, status)
	if err != nil {
		return nil, technical(span, "list tasks", err)
	}
	return items, nil
}

func (s *Service) Get(ctx context.Context, workspaceID, taskID, actorUserID string) (domain.Task, error) {
	ctx, span := s.tracer.Start(ctx, "task.get")
	defer span.End()
	if err := s.requireRead(ctx, workspaceID, actorUserID); err != nil {
		return domain.Task{}, err
	}
	if err := validateID("taskId", taskID); err != nil {
		return domain.Task{}, err
	}
	item, err := s.repo.Get(ctx, workspaceID, taskID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Task{}, apperror.NotFound("task")
	}
	if err != nil {
		return domain.Task{}, technical(span, "get task", err)
	}
	return item, nil
}

func (s *Service) Update(ctx context.Context, input UpdateInput) (domain.Task, error) {
	ctx, span := s.tracer.Start(ctx, "task.update")
	defer span.End()
	if err := s.requireWrite(ctx, input.WorkspaceID, input.ActorUserID); err != nil {
		return domain.Task{}, err
	}
	if err := validateID("taskId", input.TaskID); err != nil {
		return domain.Task{}, err
	}
	item, err := s.repo.Get(ctx, input.WorkspaceID, input.TaskID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Task{}, apperror.NotFound("task")
	}
	if err != nil {
		return domain.Task{}, technical(span, "get task for update", err)
	}
	if input.Title != nil {
		item.Title, err = validateTitle(*input.Title)
		if err != nil {
			return domain.Task{}, err
		}
	}
	if input.Description != nil {
		if !domain.ValidDescription(*input.Description) {
			return domain.Task{}, invalidDescription()
		}
		item.Description = *input.Description
	}
	if input.Status != nil {
		var ok bool
		item.Status, ok = domain.ParseStatus(*input.Status)
		if !ok {
			return domain.Task{}, invalidStatus("status")
		}
	}
	if input.AssigneeUserID != nil {
		if err := validateOptionalUserID(*input.AssigneeUserID); err != nil {
			return domain.Task{}, err
		}
		item.AssigneeUserID = *input.AssigneeUserID
		if err := s.requireAssigneeMember(ctx, input.WorkspaceID, item.AssigneeUserID); err != nil {
			return domain.Task{}, err
		}
	}
	item.UpdatedAt = s.now().UTC()
	item, err = s.repo.Update(ctx, item)
	if errors.Is(err, domain.ErrAssigneeNotMember) {
		return domain.Task{}, assigneeNotMember()
	}
	if err != nil {
		return domain.Task{}, technical(span, "update task", err)
	}
	return item, nil
}

func (s *Service) Delete(ctx context.Context, workspaceID, taskID, actorUserID string) error {
	ctx, span := s.tracer.Start(ctx, "task.delete")
	defer span.End()
	if err := s.requireWrite(ctx, workspaceID, actorUserID); err != nil {
		return err
	}
	if err := validateID("taskId", taskID); err != nil {
		return err
	}
	if _, err := s.repo.Get(ctx, workspaceID, taskID); errors.Is(err, domain.ErrNotFound) {
		return apperror.NotFound("task")
	} else if err != nil {
		return technical(span, "get task for delete", err)
	}
	if err := s.repo.Delete(ctx, workspaceID, taskID); err != nil {
		return technical(span, "delete task", err)
	}
	return nil
}

func (s *Service) requireRead(ctx context.Context, workspaceID, userID string) error {
	if err := validateID("workspaceId", workspaceID); err != nil {
		return err
	}
	role, err := s.permissions.GetMemberRole(ctx, workspaceID, userID)
	if errors.Is(err, workspace.ErrMemberNotFound) {
		return apperror.NotFound("workspace")
	}
	if err != nil {
		return fmt.Errorf("get task workspace role: %w", err)
	}
	if !role.CanRead() {
		return apperror.Forbidden("INSUFFICIENT_ROLE", "workspace membership is required")
	}
	return nil
}

func (s *Service) requireWrite(ctx context.Context, workspaceID, userID string) error {
	if err := validateID("workspaceId", workspaceID); err != nil {
		return err
	}
	role, err := s.permissions.GetMemberRole(ctx, workspaceID, userID)
	if errors.Is(err, workspace.ErrMemberNotFound) {
		return apperror.NotFound("workspace")
	}
	if err != nil {
		return fmt.Errorf("get task workspace role: %w", err)
	}
	if !role.CanWriteTasks() {
		return apperror.Forbidden("INSUFFICIENT_ROLE", "owner or editor role is required")
	}
	return nil
}

func (s *Service) requireAssigneeMember(ctx context.Context, workspaceID string, assigneeUserID *string) error {
	if assigneeUserID == nil {
		return nil
	}
	_, err := s.permissions.GetMemberRole(ctx, workspaceID, *assigneeUserID)
	if errors.Is(err, workspace.ErrMemberNotFound) {
		return assigneeNotMember()
	}
	if err != nil {
		return fmt.Errorf("verify task assignee membership: %w", err)
	}
	return nil
}

func assigneeNotMember() error {
	return apperror.Invalid("TASK_ASSIGNEE_NOT_MEMBER", "assignee must be a workspace member", apperror.Field{Name: "assigneeId", Code: "NOT_A_WORKSPACE_MEMBER", Message: "must identify a workspace member"})
}

func validateID(name, value string) error {
	if _, err := uuid.Parse(value); err != nil {
		return apperror.Invalid("VALIDATION_FAILED", "identifier is invalid", apperror.Field{Name: "path." + name, Code: "INVALID_FORMAT", Message: "must be a UUID"})
	}
	return nil
}

func validateOptionalUserID(value *string) error {
	if value == nil {
		return nil
	}
	if _, err := uuid.Parse(*value); err != nil {
		return apperror.Invalid("VALIDATION_FAILED", "identifier is invalid", apperror.Field{Name: "assigneeId", Code: "INVALID_FORMAT", Message: "must be a UUID"})
	}
	return nil
}

func validateTitle(value string) (string, error) {
	value = domain.NormalizeTitle(value)
	if !domain.ValidTitle(value) {
		return "", apperror.Invalid("VALIDATION_FAILED", "task input is invalid", apperror.Field{Name: "title", Code: "INVALID_LENGTH", Message: "must be between 1 and 160 characters"})
	}
	return value, nil
}

func invalidDescription() error {
	return apperror.Invalid("VALIDATION_FAILED", "task input is invalid", apperror.Field{Name: "description", Code: "VALUE_TOO_LONG", Message: "must be at most 4000 characters"})
}

func invalidStatus(location string) error {
	return apperror.Invalid("VALIDATION_FAILED", "status is invalid", apperror.Field{Name: location, Code: "INVALID_ENUM", Message: "must be todo, in_progress, or done"})
}

func technical(span trace.Span, action string, err error) error {
	span.RecordError(err)
	span.SetStatus(codes.Error, action)
	return fmt.Errorf("%s: %w", action, err)
}
