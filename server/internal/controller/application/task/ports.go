package task

import (
	"context"

	domain "example.com/project-template/internal/domain/task"
	"example.com/project-template/internal/domain/workspace"
)

type Repository interface {
	Create(context.Context, domain.Task) (domain.Task, error)
	Get(context.Context, string, string) (domain.Task, error)
	List(context.Context, string, *domain.Status) ([]domain.Task, error)
	Update(context.Context, domain.Task) (domain.Task, error)
	Delete(context.Context, string, string) error
}

type PermissionReader interface {
	GetMemberRole(context.Context, string, string) (workspace.Role, error)
}
