package workspace

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"example.com/project-template/internal/controller/application/apperror"
	"example.com/project-template/internal/domain/identity"
	domain "example.com/project-template/internal/domain/workspace"
)

type Service struct {
	repo   Repository
	users  UserLookup
	tx     Transactor
	now    func() time.Time
	tracer trace.Tracer
}

func NewService(repo Repository, users UserLookup, tx Transactor, tracer trace.Tracer) *Service {
	return &Service{repo: repo, users: users, tx: tx, now: time.Now, tracer: tracer}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (domain.Workspace, error) {
	ctx, span := s.tracer.Start(ctx, "workspace.create")
	defer span.End()
	name, err := validateName(input.Name)
	if err != nil {
		return domain.Workspace{}, err
	}
	now := s.now().UTC()
	item := domain.Workspace{ID: uuid.NewString(), Name: name, CreatedByUserID: input.ActorUserID, CreatedAt: now, UpdatedAt: now}
	item.Role = domain.RoleOwner
	err = s.tx.WithinTx(ctx, func(txCtx context.Context) error {
		var createErr error
		item, createErr = s.repo.Create(txCtx, item)
		if createErr != nil {
			return createErr
		}
		return s.repo.CreateMember(txCtx, domain.Member{WorkspaceID: item.ID, UserID: input.ActorUserID, Role: domain.RoleOwner, JoinedAt: now})
	})
	if err != nil {
		return domain.Workspace{}, technical(span, "create workspace with owner", err)
	}
	return item, nil
}

func (s *Service) List(ctx context.Context, actorUserID string) ([]domain.Workspace, error) {
	ctx, span := s.tracer.Start(ctx, "workspace.list")
	defer span.End()
	items, err := s.repo.ListForUser(ctx, actorUserID)
	if err != nil {
		return nil, technical(span, "list workspaces", err)
	}
	return items, nil
}

func (s *Service) Get(ctx context.Context, workspaceID, actorUserID string) (domain.Workspace, error) {
	ctx, span := s.tracer.Start(ctx, "workspace.get")
	defer span.End()
	if err := validateID("workspaceId", workspaceID); err != nil {
		return domain.Workspace{}, err
	}
	item, err := s.repo.GetForUser(ctx, workspaceID, actorUserID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Workspace{}, apperror.NotFound("workspace")
	}
	if err != nil {
		return domain.Workspace{}, technical(span, "get workspace", err)
	}
	return item, nil
}

func (s *Service) Update(ctx context.Context, input UpdateInput) (domain.Workspace, error) {
	ctx, span := s.tracer.Start(ctx, "workspace.update")
	defer span.End()
	if err := validateID("workspaceId", input.WorkspaceID); err != nil {
		return domain.Workspace{}, err
	}
	if err := s.requireOwner(ctx, input.WorkspaceID, input.ActorUserID); err != nil {
		return domain.Workspace{}, err
	}
	item, err := s.repo.GetForUser(ctx, input.WorkspaceID, input.ActorUserID)
	if err != nil {
		return domain.Workspace{}, mapLookup(span, "get workspace for update", err)
	}
	item.Name, err = validateName(input.Name)
	if err != nil {
		return domain.Workspace{}, err
	}
	item.UpdatedAt = s.now().UTC()
	item, err = s.repo.Update(ctx, item)
	if err != nil {
		return domain.Workspace{}, technical(span, "update workspace", err)
	}
	return item, nil
}

func (s *Service) Delete(ctx context.Context, workspaceID, actorUserID string) error {
	ctx, span := s.tracer.Start(ctx, "workspace.delete")
	defer span.End()
	if err := validateID("workspaceId", workspaceID); err != nil {
		return err
	}
	if err := s.requireOwner(ctx, workspaceID, actorUserID); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, workspaceID); err != nil {
		return technical(span, "delete workspace", err)
	}
	return nil
}

func (s *Service) ListMembers(ctx context.Context, workspaceID, actorUserID string) ([]domain.Member, error) {
	ctx, span := s.tracer.Start(ctx, "workspace.members.list")
	defer span.End()
	if _, err := s.memberRole(ctx, workspaceID, actorUserID); err != nil {
		return nil, err
	}
	members, err := s.repo.ListMembers(ctx, workspaceID)
	if err != nil {
		return nil, technical(span, "list workspace members", err)
	}
	return members, nil
}

func (s *Service) AddMember(ctx context.Context, input AddMemberInput) (domain.Member, error) {
	ctx, span := s.tracer.Start(ctx, "workspace.members.add")
	defer span.End()
	if err := s.requireOwner(ctx, input.WorkspaceID, input.ActorUserID); err != nil {
		return domain.Member{}, err
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	parsed, parseErr := mail.ParseAddress(email)
	if parseErr != nil || parsed.Address != email || len(email) > 254 {
		return domain.Member{}, apperror.Invalid("VALIDATION_FAILED", "member input is invalid", apperror.Field{Name: "email", Code: "INVALID_FORMAT", Message: "must be a valid email address"})
	}
	role, ok := domain.ParseRole(input.Role)
	if !ok {
		return domain.Member{}, invalidRole()
	}
	user, err := s.users.FindUserByEmail(ctx, email)
	if errors.Is(err, identity.ErrUserNotFound) {
		return domain.Member{}, apperror.NotFound("user")
	}
	if err != nil {
		return domain.Member{}, technical(span, "look up workspace member user", err)
	}
	err = s.repo.CreateMember(ctx, domain.Member{WorkspaceID: input.WorkspaceID, UserID: user.ID, Role: role, JoinedAt: s.now().UTC()})
	if errors.Is(err, domain.ErrMemberExists) {
		return domain.Member{}, apperror.Conflict("WORKSPACE_MEMBER_ALREADY_EXISTS", "user is already a workspace member")
	}
	if err != nil {
		return domain.Member{}, technical(span, "add workspace member", err)
	}
	member, err := s.repo.GetMember(ctx, input.WorkspaceID, user.ID)
	if err != nil {
		return domain.Member{}, technical(span, "load added workspace member", err)
	}
	return member, nil
}

func (s *Service) UpdateMember(ctx context.Context, input UpdateMemberInput) (domain.Member, error) {
	ctx, span := s.tracer.Start(ctx, "workspace.members.update")
	defer span.End()
	if err := validateID("workspaceId", input.WorkspaceID); err != nil {
		return domain.Member{}, err
	}
	if err := validateID("userId", input.UserID); err != nil {
		return domain.Member{}, err
	}
	role, ok := domain.ParseRole(input.Role)
	if !ok {
		return domain.Member{}, invalidRole()
	}
	var member domain.Member
	err := s.tx.WithinTx(ctx, func(txCtx context.Context) error {
		if lockErr := s.repo.LockMembership(txCtx, input.WorkspaceID); lockErr != nil {
			return lockErr
		}
		if authErr := s.requireOwner(txCtx, input.WorkspaceID, input.ActorUserID); authErr != nil {
			return authErr
		}
		var loadErr error
		member, loadErr = s.repo.GetMember(txCtx, input.WorkspaceID, input.UserID)
		if loadErr != nil {
			return loadErr
		}
		if member.Role == domain.RoleOwner && role != domain.RoleOwner {
			if ownerErr := s.ensureAnotherOwner(txCtx, input.WorkspaceID); ownerErr != nil {
				return ownerErr
			}
		}
		return s.repo.UpdateMemberRole(txCtx, input.WorkspaceID, input.UserID, role)
	})
	if errors.Is(err, domain.ErrMemberNotFound) {
		return domain.Member{}, apperror.NotFound("workspace member")
	}
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Member{}, apperror.NotFound("workspace")
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return domain.Member{}, appErr
	}
	if err != nil {
		return domain.Member{}, technical(span, "update workspace member", err)
	}
	member.Role = role
	return member, nil
}

func (s *Service) RemoveMember(ctx context.Context, workspaceID, userID, actorUserID string) error {
	ctx, span := s.tracer.Start(ctx, "workspace.members.remove")
	defer span.End()
	if err := validateID("workspaceId", workspaceID); err != nil {
		return err
	}
	if err := validateID("userId", userID); err != nil {
		return err
	}
	err := s.tx.WithinTx(ctx, func(txCtx context.Context) error {
		if lockErr := s.repo.LockMembership(txCtx, workspaceID); lockErr != nil {
			return lockErr
		}
		if authErr := s.requireOwner(txCtx, workspaceID, actorUserID); authErr != nil {
			return authErr
		}
		member, loadErr := s.repo.GetMember(txCtx, workspaceID, userID)
		if loadErr != nil {
			return loadErr
		}
		if member.Role == domain.RoleOwner {
			if ownerErr := s.ensureAnotherOwner(txCtx, workspaceID); ownerErr != nil {
				return ownerErr
			}
		}
		return s.repo.DeleteMember(txCtx, workspaceID, userID)
	})
	if errors.Is(err, domain.ErrMemberNotFound) {
		return apperror.NotFound("workspace member")
	}
	if errors.Is(err, domain.ErrNotFound) {
		return apperror.NotFound("workspace")
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if err != nil {
		return technical(span, "remove workspace member", err)
	}
	return nil
}

func (s *Service) requireOwner(ctx context.Context, workspaceID, userID string) error {
	role, err := s.memberRole(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	if !role.CanManageWorkspace() {
		return apperror.Forbidden("INSUFFICIENT_ROLE", "workspace owner role is required")
	}
	return nil
}

func (s *Service) memberRole(ctx context.Context, workspaceID, userID string) (domain.Role, error) {
	if err := validateID("workspaceId", workspaceID); err != nil {
		return "", err
	}
	role, err := s.repo.GetMemberRole(ctx, workspaceID, userID)
	if errors.Is(err, domain.ErrMemberNotFound) {
		return "", apperror.NotFound("workspace")
	}
	if err != nil {
		return "", fmt.Errorf("get workspace role: %w", err)
	}
	return role, nil
}

func (s *Service) ensureAnotherOwner(ctx context.Context, workspaceID string) error {
	count, err := s.repo.CountOwners(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("count workspace owners: %w", err)
	}
	if count <= 1 {
		return apperror.Conflict("WORKSPACE_LAST_OWNER", "the workspace must retain at least one owner")
	}
	return nil
}

func validateID(name, value string) error {
	if _, err := uuid.Parse(value); err != nil {
		return apperror.Invalid("VALIDATION_FAILED", "identifier is invalid", apperror.Field{Name: "path." + name, Code: "INVALID_FORMAT", Message: "must be a UUID"})
	}
	return nil
}

func validateName(value string) (string, error) {
	value = domain.NormalizeName(value)
	if !domain.ValidName(value) {
		return "", apperror.Invalid("VALIDATION_FAILED", "workspace input is invalid", apperror.Field{Name: "name", Code: "INVALID_LENGTH", Message: "must be between 1 and 80 characters"})
	}
	return value, nil
}

func invalidRole() error {
	return apperror.Invalid("VALIDATION_FAILED", "role is invalid", apperror.Field{Name: "role", Code: "INVALID_ENUM", Message: "must be owner, editor, or viewer"})
}

func mapLookup(span trace.Span, action string, err error) error {
	if errors.Is(err, domain.ErrNotFound) {
		return apperror.NotFound("workspace")
	}
	return technical(span, action, err)
}

func technical(span trace.Span, action string, err error) error {
	span.RecordError(err)
	span.SetStatus(codes.Error, action)
	return fmt.Errorf("%s: %w", strings.TrimSpace(action), err)
}
