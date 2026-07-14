package oauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/project-template/internal/controller/infrastructure/postgres"
	"example.com/project-template/internal/domain/identity"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) StoreOAuthState(ctx context.Context, state identity.OAuthState) error {
	_, err := postgres.Executor(ctx, r.pool).Exec(ctx, `
		INSERT INTO oauth_states (state_hash, verifier_ciphertext, return_path, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, state.StateHash, state.VerifierCiphertext, state.ReturnPath, state.ExpiresAt, state.CreatedAt)
	if err != nil {
		return fmt.Errorf("store oauth state: %w", err)
	}
	return nil
}

func (r *Repository) ConsumeOAuthState(ctx context.Context, stateHash []byte) (identity.OAuthState, error) {
	var state identity.OAuthState
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, `
		DELETE FROM oauth_states
		WHERE state_hash = $1
		RETURNING state_hash, verifier_ciphertext, return_path, expires_at, created_at
	`, stateHash).Scan(&state.StateHash, &state.VerifierCiphertext, &state.ReturnPath, &state.ExpiresAt, &state.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.OAuthState{}, identity.ErrOAuthStateNotFound
	}
	if err != nil {
		return identity.OAuthState{}, fmt.Errorf("consume oauth state: %w", err)
	}
	return state, nil
}

func (r *Repository) UpsertUser(ctx context.Context, user identity.User) (identity.User, error) {
	row := postgres.Executor(ctx, r.pool).QueryRow(ctx, `
		INSERT INTO users
		    (id, gitlab_user_id, username, display_name, avatar_url, profile_url,
		     access_level, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
		ON CONFLICT (gitlab_user_id) DO UPDATE
		SET username = EXCLUDED.username,
		    display_name = EXCLUDED.display_name,
		    avatar_url = EXCLUDED.avatar_url,
		    profile_url = EXCLUDED.profile_url,
		    access_level = EXCLUDED.access_level,
		    updated_at = EXCLUDED.updated_at
		RETURNING id, gitlab_user_id, username, display_name, avatar_url,
		          profile_url, access_level, created_at, updated_at
	`, uuid.MustParse(user.ID), user.GitLabUserID, user.Username, user.DisplayName,
		nullableString(user.AvatarURL), user.ProfileURL, user.AccessLevel, user.CreatedAt)
	result, err := scanUser(row)
	if err != nil {
		return identity.User{}, fmt.Errorf("upsert GitLab user: %w", err)
	}
	return result, nil
}

func (r *Repository) GetUserByID(ctx context.Context, userID string) (identity.User, error) {
	user, err := scanUser(postgres.Executor(ctx, r.pool).QueryRow(ctx, `
		SELECT id, gitlab_user_id, username, display_name, avatar_url,
		       profile_url, access_level, created_at, updated_at
		FROM users
		WHERE id = $1
	`, uuid.MustParse(userID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.User{}, identity.ErrUserNotFound
	}
	if err != nil {
		return identity.User{}, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}

func (r *Repository) CreateSession(ctx context.Context, session identity.Session) (identity.Session, error) {
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, `
		INSERT INTO auth_sessions
		    (id, user_id, token_hash, expires_at, created_at, last_used_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, token_hash, csrf_token_hash, expires_at, created_at, last_used_at
	`, uuid.MustParse(session.ID), uuid.MustParse(session.UserID), session.TokenHash,
		session.ExpiresAt, session.CreatedAt, session.LastUsedAt).Scan(
		&session.ID, &session.UserID, &session.TokenHash, &session.CSRFTokenHash,
		&session.ExpiresAt, &session.CreatedAt, &session.LastUsedAt,
	)
	if err != nil {
		return identity.Session{}, fmt.Errorf("create auth session: %w", err)
	}
	return session, nil
}

func (r *Repository) GetSessionByTokenHash(ctx context.Context, digest []byte) (identity.Session, error) {
	var session identity.Session
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, `
		SELECT id, user_id, token_hash, csrf_token_hash, expires_at, created_at, last_used_at
		FROM auth_sessions
		WHERE token_hash = $1
	`, digest).Scan(
		&session.ID, &session.UserID, &session.TokenHash, &session.CSRFTokenHash,
		&session.ExpiresAt, &session.CreatedAt, &session.LastUsedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.Session{}, identity.ErrSessionNotFound
	}
	if err != nil {
		return identity.Session{}, fmt.Errorf("get auth session: %w", err)
	}
	return session, nil
}

func (r *Repository) SetSessionCSRFHash(ctx context.Context, sessionID string, digest []byte) error {
	_, err := postgres.Executor(ctx, r.pool).Exec(ctx, `UPDATE auth_sessions SET csrf_token_hash = $2 WHERE id = $1`, uuid.MustParse(sessionID), digest)
	return err
}

func (r *Repository) TouchSession(ctx context.Context, sessionID string, session identity.Session) error {
	command, err := postgres.Executor(ctx, r.pool).Exec(ctx, `
		UPDATE auth_sessions
		SET last_used_at = $2, expires_at = $3
		WHERE id = $1 AND expires_at > $2
	`, uuid.MustParse(sessionID), session.LastUsedAt, session.ExpiresAt)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return identity.ErrSessionNotFound
	}
	return nil
}

func (r *Repository) DeleteSessionByTokenHash(ctx context.Context, digest []byte) error {
	_, err := postgres.Executor(ctx, r.pool).Exec(ctx, `DELETE FROM auth_sessions WHERE token_hash = $1`, digest)
	return err
}

func (r *Repository) DeleteExpiredSession(ctx context.Context, sessionID string) error {
	_, err := postgres.Executor(ctx, r.pool).Exec(ctx, `
		DELETE FROM auth_sessions WHERE id = $1 AND expires_at <= $2
	`, uuid.MustParse(sessionID), time.Now().UTC())
	return err
}

type rowScanner interface {
	Scan(...any) error
}

func scanUser(row rowScanner) (identity.User, error) {
	var user identity.User
	var avatarURL *string
	err := row.Scan(
		&user.ID, &user.GitLabUserID, &user.Username, &user.DisplayName, &avatarURL,
		&user.ProfileURL, &user.AccessLevel, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return identity.User{}, err
	}
	if avatarURL != nil {
		user.AvatarURL = *avatarURL
	}
	return user, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
