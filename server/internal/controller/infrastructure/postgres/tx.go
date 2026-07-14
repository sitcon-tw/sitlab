package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/project-template/internal/controller/infrastructure/postgres/sqlc"
)

type txKey struct{}

type Transactor struct{ pool *pgxpool.Pool }

func NewTransactor(pool *pgxpool.Pool) *Transactor { return &Transactor{pool: pool} }

func (t *Transactor) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := txFromContext(ctx); ok {
		return fn(ctx)
	}
	return pgx.BeginFunc(ctx, t.pool, func(tx pgx.Tx) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

func Queries(ctx context.Context, base *sqlc.Queries) *sqlc.Queries {
	if tx, ok := txFromContext(ctx); ok {
		return base.WithTx(tx)
	}
	return base
}

type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func Executor(ctx context.Context, pool *pgxpool.Pool) DBTX {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return pool
}

func txFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}
