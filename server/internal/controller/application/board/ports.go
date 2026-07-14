package board

import (
	"context"
	"time"

	domain "example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/directory"
)

type Snapshot struct {
	Lists    []domain.List
	Cards    []domain.Card
	SyncedAt time.Time
}

type Mutation struct {
	Card              domain.Card
	Operation         domain.Operation
	RequestedByUserID string
	Payload           map[string]any
}

type Result struct {
	Card      domain.Card
	Operation domain.Operation
}

type Repository interface {
	Board(context.Context) (Snapshot, error)
	Card(context.Context, int64) (domain.Card, error)
	ByOperation(context.Context, string) (Result, error)
	CreateCard(context.Context, Mutation) (Result, error)
	UpdateCard(context.Context, Mutation) (Result, error)
	RetryOperation(context.Context, string) (domain.Operation, error)
}

type Directory interface {
	Snapshot(context.Context) (directory.Snapshot, error)
}
