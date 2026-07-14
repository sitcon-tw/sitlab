package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/project-template/internal/controller/infrastructure/postgres"
	"example.com/project-template/internal/controller/infrastructure/postgres/sqlc"
	"example.com/project-template/internal/domain/identity"
)

type Repository struct{ base *sqlc.Queries }

func New(pool *pgxpool.Pool) *Repository { return &Repository{base: sqlc.New(pool)} }

func (r *Repository) q(ctx context.Context) *sqlc.Queries { return postgres.Queries(ctx, r.base) }

func (r *Repository) CreateUser(ctx context.Context, input identity.User) (identity.User, error) {
	row, err := r.q(ctx).CreateUser(ctx, sqlc.CreateUserParams{ID: uuid.MustParse(input.ID), Email: input.Email, PasswordHash: input.PasswordHash, DisplayName: input.DisplayName, CreatedAt: input.CreatedAt})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return identity.User{}, identity.ErrEmailInUse
	}
	if err != nil {
		return identity.User{}, fmt.Errorf("create user: %w", err)
	}
	return mapUser(row), nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (identity.User, error) {
	row, err := r.q(ctx).GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.User{}, identity.ErrUserNotFound
	}
	if err != nil {
		return identity.User{}, fmt.Errorf("get user by email: %w", err)
	}
	return mapUser(row), nil
}

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (identity.User, error) {
	return r.GetUserByEmail(ctx, email)
}

func (r *Repository) GetUserByID(ctx context.Context, userID string) (identity.User, error) {
	row, err := r.q(ctx).GetUserByID(ctx, uuid.MustParse(userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.User{}, identity.ErrUserNotFound
	}
	if err != nil {
		return identity.User{}, fmt.Errorf("get user by id: %w", err)
	}
	return mapUser(row), nil
}

func (r *Repository) CreateSession(ctx context.Context, input identity.Session) (identity.Session, error) {
	row, err := r.q(ctx).CreateAuthSession(ctx, sqlc.CreateAuthSessionParams{ID: uuid.MustParse(input.ID), UserID: uuid.MustParse(input.UserID), TokenHash: input.TokenHash, IdleExpiresAt: input.IdleExpiresAt, AbsoluteExpiresAt: input.AbsoluteExpiresAt, CreatedAt: input.CreatedAt})
	if err != nil {
		return identity.Session{}, fmt.Errorf("create auth session: %w", err)
	}
	return mapSession(row), nil
}

func (r *Repository) GetSessionByTokenHash(ctx context.Context, digest []byte) (identity.Session, error) {
	row, err := r.q(ctx).GetAuthSessionByTokenHash(ctx, digest)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.Session{}, identity.ErrSessionNotFound
	}
	if err != nil {
		return identity.Session{}, fmt.Errorf("get auth session: %w", err)
	}
	return mapSession(row), nil
}

func (r *Repository) SetSessionCSRFHash(ctx context.Context, sessionID string, digest []byte) error {
	return r.q(ctx).SetAuthSessionCSRFHash(ctx, sqlc.SetAuthSessionCSRFHashParams{ID: uuid.MustParse(sessionID), CsrfTokenHash: digest})
}

func (r *Repository) TouchSession(ctx context.Context, sessionID string, input identity.Session) error {
	return r.q(ctx).TouchAuthSession(ctx, sqlc.TouchAuthSessionParams{ID: uuid.MustParse(sessionID), LastSeenAt: input.LastSeenAt, IdleExpiresAt: input.IdleExpiresAt})
}

func (r *Repository) DeleteSessionByTokenHash(ctx context.Context, digest []byte) error {
	return r.q(ctx).DeleteAuthSessionByTokenHash(ctx, digest)
}

func (r *Repository) DeleteExpiredSession(ctx context.Context, sessionID string) error {
	return r.q(ctx).DeleteExpiredAuthSession(ctx, sqlc.DeleteExpiredAuthSessionParams{ID: uuid.MustParse(sessionID), IdleExpiresAt: time.Now().UTC()})
}

func mapUser(row sqlc.User) identity.User {
	return identity.User{ID: row.ID.String(), Email: row.Email, PasswordHash: row.PasswordHash, DisplayName: row.DisplayName, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func mapSession(row sqlc.AuthSession) identity.Session {
	return identity.Session{ID: row.ID.String(), UserID: row.UserID.String(), TokenHash: row.TokenHash, CSRFTokenHash: row.CsrfTokenHash, IdleExpiresAt: row.IdleExpiresAt, AbsoluteExpiresAt: row.AbsoluteExpiresAt, CreatedAt: row.CreatedAt, LastSeenAt: row.LastSeenAt}
}
