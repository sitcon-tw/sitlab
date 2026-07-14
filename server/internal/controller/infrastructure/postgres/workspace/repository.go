package workspace

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/project-template/internal/controller/infrastructure/postgres"
	"example.com/project-template/internal/controller/infrastructure/postgres/sqlc"
	domain "example.com/project-template/internal/domain/workspace"
)

type Repository struct{ base *sqlc.Queries }

func New(pool *pgxpool.Pool) *Repository { return &Repository{base: sqlc.New(pool)} }

func (r *Repository) q(ctx context.Context) *sqlc.Queries { return postgres.Queries(ctx, r.base) }

func (r *Repository) Create(ctx context.Context, input domain.Workspace) (domain.Workspace, error) {
	row, err := r.q(ctx).CreateWorkspace(ctx, sqlc.CreateWorkspaceParams{ID: uuid.MustParse(input.ID), Name: input.Name, CreatedByUserID: uuid.MustParse(input.CreatedByUserID), CreatedAt: input.CreatedAt})
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("create workspace: %w", err)
	}
	result := mapWorkspace(row)
	result.Role = input.Role
	return result, nil
}

func (r *Repository) CreateMember(ctx context.Context, input domain.Member) error {
	err := r.q(ctx).CreateWorkspaceMember(ctx, sqlc.CreateWorkspaceMemberParams{WorkspaceID: uuid.MustParse(input.WorkspaceID), UserID: uuid.MustParse(input.UserID), Role: string(input.Role), JoinedAt: input.JoinedAt})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrMemberExists
	}
	if err != nil {
		return fmt.Errorf("create workspace member: %w", err)
	}
	return nil
}

func (r *Repository) GetForUser(ctx context.Context, workspaceID, userID string) (domain.Workspace, error) {
	row, err := r.q(ctx).GetWorkspaceForUser(ctx, sqlc.GetWorkspaceForUserParams{ID: uuid.MustParse(workspaceID), UserID: uuid.MustParse(userID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Workspace{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("get workspace: %w", err)
	}
	return domain.Workspace{ID: row.ID.String(), Name: row.Name, Role: domain.Role(row.Role), CreatedByUserID: row.CreatedByUserID.String(), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

func (r *Repository) ListForUser(ctx context.Context, userID string) ([]domain.Workspace, error) {
	rows, err := r.q(ctx).ListWorkspacesForUser(ctx, uuid.MustParse(userID))
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	result := make([]domain.Workspace, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.Workspace{ID: row.ID.String(), Name: row.Name, Role: domain.Role(row.Role), CreatedByUserID: row.CreatedByUserID.String(), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt})
	}
	return result, nil
}

func (r *Repository) Update(ctx context.Context, input domain.Workspace) (domain.Workspace, error) {
	row, err := r.q(ctx).UpdateWorkspace(ctx, sqlc.UpdateWorkspaceParams{ID: uuid.MustParse(input.ID), Name: input.Name, UpdatedAt: input.UpdatedAt})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Workspace{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("update workspace: %w", err)
	}
	result := mapWorkspace(row)
	result.Role = input.Role
	return result, nil
}

func (r *Repository) Delete(ctx context.Context, workspaceID string) error {
	return r.q(ctx).DeleteWorkspace(ctx, uuid.MustParse(workspaceID))
}

func (r *Repository) LockMembership(ctx context.Context, workspaceID string) error {
	_, err := r.q(ctx).LockWorkspaceMembership(ctx, uuid.MustParse(workspaceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock workspace membership: %w", err)
	}
	return nil
}

func (r *Repository) GetMemberRole(ctx context.Context, workspaceID, userID string) (domain.Role, error) {
	role, err := r.q(ctx).GetWorkspaceMemberRole(ctx, sqlc.GetWorkspaceMemberRoleParams{WorkspaceID: uuid.MustParse(workspaceID), UserID: uuid.MustParse(userID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrMemberNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get workspace member role: %w", err)
	}
	return domain.Role(role), nil
}

func (r *Repository) ListMembers(ctx context.Context, workspaceID string) ([]domain.Member, error) {
	rows, err := r.q(ctx).ListWorkspaceMembers(ctx, uuid.MustParse(workspaceID))
	if err != nil {
		return nil, fmt.Errorf("list workspace members: %w", err)
	}
	result := make([]domain.Member, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.Member{WorkspaceID: row.WorkspaceID.String(), UserID: row.UserID.String(), Email: row.Email, DisplayName: row.DisplayName, Role: domain.Role(row.Role), JoinedAt: row.JoinedAt})
	}
	return result, nil
}

func (r *Repository) GetMember(ctx context.Context, workspaceID, userID string) (domain.Member, error) {
	row, err := r.q(ctx).GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{WorkspaceID: uuid.MustParse(workspaceID), UserID: uuid.MustParse(userID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Member{}, domain.ErrMemberNotFound
	}
	if err != nil {
		return domain.Member{}, fmt.Errorf("get workspace member: %w", err)
	}
	return domain.Member{WorkspaceID: row.WorkspaceID.String(), UserID: row.UserID.String(), Email: row.Email, DisplayName: row.DisplayName, Role: domain.Role(row.Role), JoinedAt: row.JoinedAt}, nil
}

func (r *Repository) CountOwners(ctx context.Context, workspaceID string) (int, error) {
	count, err := r.q(ctx).CountWorkspaceOwners(ctx, uuid.MustParse(workspaceID))
	return int(count), err
}

func (r *Repository) UpdateMemberRole(ctx context.Context, workspaceID, userID string, role domain.Role) error {
	return r.q(ctx).UpdateWorkspaceMemberRole(ctx, sqlc.UpdateWorkspaceMemberRoleParams{WorkspaceID: uuid.MustParse(workspaceID), UserID: uuid.MustParse(userID), Role: string(role)})
}

func (r *Repository) DeleteMember(ctx context.Context, workspaceID, userID string) error {
	return r.q(ctx).DeleteWorkspaceMember(ctx, sqlc.DeleteWorkspaceMemberParams{WorkspaceID: uuid.MustParse(workspaceID), UserID: uuid.MustParse(userID)})
}

func mapWorkspace(row sqlc.Workspace) domain.Workspace {
	return domain.Workspace{ID: row.ID.String(), Name: row.Name, CreatedByUserID: row.CreatedByUserID.String(), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}
