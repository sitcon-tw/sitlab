package board

import (
	"context"

	domain "example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/directory"
)

type Snapshot = domain.Snapshot
type Mutation = domain.Mutation
type Result = domain.Result

type Repository interface {
	Board(context.Context) (domain.Snapshot, error)
	Card(context.Context, int64) (domain.Card, error)
	ByOperation(context.Context, string) (domain.Result, error)
	CreateCard(context.Context, domain.Mutation) (domain.Result, error)
	UpdateCard(context.Context, domain.Mutation) (domain.Result, error)
	RetryOperation(context.Context, string) (domain.Operation, error)
}

type Directory interface {
	Snapshot(context.Context) (directory.Snapshot, error)
}
