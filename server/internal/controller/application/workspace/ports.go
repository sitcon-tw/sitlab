package workspace

import (
	"context"

	"example.com/project-template/internal/domain/identity"
	domain "example.com/project-template/internal/domain/workspace"
)

type Repository interface {
	Create(context.Context, domain.Workspace) (domain.Workspace, error)
	CreateMember(context.Context, domain.Member) error
	GetForUser(context.Context, string, string) (domain.Workspace, error)
	ListForUser(context.Context, string) ([]domain.Workspace, error)
	Update(context.Context, domain.Workspace) (domain.Workspace, error)
	Delete(context.Context, string) error
	LockMembership(context.Context, string) error
	GetMemberRole(context.Context, string, string) (domain.Role, error)
	ListMembers(context.Context, string) ([]domain.Member, error)
	GetMember(context.Context, string, string) (domain.Member, error)
	CountOwners(context.Context, string) (int, error)
	UpdateMemberRole(context.Context, string, string, domain.Role) error
	DeleteMember(context.Context, string, string) error
}

type UserLookup interface {
	FindUserByEmail(context.Context, string) (identity.User, error)
}

type Transactor interface {
	WithinTx(context.Context, func(context.Context) error) error
}
