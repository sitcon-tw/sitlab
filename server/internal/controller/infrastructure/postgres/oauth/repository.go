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

func (r *Repository) UpsertOAuthCredential(ctx context.Context, credential identity.OAuthCredential) error {
	_, err := postgres.Executor(ctx, r.pool).Exec(ctx, `
		INSERT INTO gitlab_oauth_credentials
		    (user_id, access_token_ciphertext, refresh_token_ciphertext, expires_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
		SET access_token_ciphertext = EXCLUDED.access_token_ciphertext,
		    refresh_token_ciphertext = EXCLUDED.refresh_token_ciphertext,
		    expires_at = EXCLUDED.expires_at,
		    updated_at = EXCLUDED.updated_at
	`, uuid.MustParse(credential.UserID), credential.AccessTokenCiphertext,
		credential.RefreshTokenCiphertext, credential.ExpiresAt, credential.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert GitLab OAuth credential: %w", err)
	}
	return nil
}

func (r *Repository) ReplaceOAuthCredential(ctx context.Context, credential identity.OAuthCredential, now time.Time) error {
	if err := r.QueueAndDeleteOAuthCredential(ctx, credential.UserID, now); err != nil {
		return err
	}
	return r.UpsertOAuthCredential(ctx, credential)
}

func (r *Repository) OAuthCredential(ctx context.Context, userID string) (identity.OAuthCredential, error) {
	return r.oauthCredential(ctx, userID, false)
}

func (r *Repository) OAuthCredentialForUpdate(ctx context.Context, userID string) (identity.OAuthCredential, error) {
	return r.oauthCredential(ctx, userID, true)
}

func (r *Repository) oauthCredential(ctx context.Context, userID string, forUpdate bool) (identity.OAuthCredential, error) {
	var credential identity.OAuthCredential
	query := `
		SELECT user_id, access_token_ciphertext, refresh_token_ciphertext, expires_at, updated_at
		FROM gitlab_oauth_credentials
		WHERE user_id = $1
	`
	if forUpdate {
		query += " FOR UPDATE"
	}
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, query, uuid.MustParse(userID)).Scan(&credential.UserID, &credential.AccessTokenCiphertext,
		&credential.RefreshTokenCiphertext, &credential.ExpiresAt, &credential.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.OAuthCredential{}, identity.ErrOAuthCredentialNotFound
	}
	if err != nil {
		return identity.OAuthCredential{}, fmt.Errorf("get GitLab OAuth credential: %w", err)
	}
	return credential, nil
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

func (r *Repository) DeleteSessionByTokenHash(ctx context.Context, digest []byte) (string, bool, error) {
	var userID string
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, `
		DELETE FROM auth_sessions WHERE token_hash = $1 RETURNING user_id
	`, digest).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("delete auth session: %w", err)
	}
	return userID, true, nil
}

func (r *Repository) DeleteExpiredSession(ctx context.Context, sessionID string) error {
	_, err := postgres.Executor(ctx, r.pool).Exec(ctx, `
		DELETE FROM auth_sessions WHERE id = $1 AND expires_at <= $2
	`, uuid.MustParse(sessionID), time.Now().UTC())
	return err
}

func (r *Repository) HasActiveSessions(ctx context.Context, userID string, now time.Time) (bool, error) {
	var active bool
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM auth_sessions WHERE user_id = $1 AND expires_at > $2
		)
	`, uuid.MustParse(userID), now).Scan(&active)
	return active, err
}

func (r *Repository) QueueAndDeleteOAuthCredential(ctx context.Context, userID string, now time.Time) error {
	_, err := postgres.Executor(ctx, r.pool).Exec(ctx, `
		INSERT INTO gitlab_oauth_token_revocations
		    (id, user_id, access_token_ciphertext, refresh_token_ciphertext,
		     attempts, available_at, created_at, updated_at)
		SELECT $1, user_id, access_token_ciphertext, refresh_token_ciphertext,
		       0, $3, $3, $3
		FROM gitlab_oauth_credentials
		WHERE user_id = $2
	`, uuid.New(), uuid.MustParse(userID), now)
	if err != nil {
		return fmt.Errorf("queue GitLab OAuth token revocation: %w", err)
	}
	if _, err := postgres.Executor(ctx, r.pool).Exec(ctx, `
		DELETE FROM gitlab_oauth_credentials WHERE user_id = $1
	`, uuid.MustParse(userID)); err != nil {
		return fmt.Errorf("delete GitLab OAuth credential: %w", err)
	}
	return nil
}

func (r *Repository) QueueOrphanedOAuthCredentials(ctx context.Context, now time.Time) error {
	if _, err := postgres.Executor(ctx, r.pool).Exec(ctx, `DELETE FROM auth_sessions WHERE expires_at <= $1`, now); err != nil {
		return fmt.Errorf("delete expired auth sessions: %w", err)
	}
	rows, err := postgres.Executor(ctx, r.pool).Query(ctx, `
		SELECT credential.user_id
		FROM gitlab_oauth_credentials AS credential
		WHERE NOT EXISTS (
			SELECT 1 FROM auth_sessions AS session
			WHERE session.user_id = credential.user_id AND session.expires_at > $1
		)
		FOR UPDATE OF credential
	`, now)
	if err != nil {
		return fmt.Errorf("find orphaned GitLab OAuth credentials: %w", err)
	}
	userIDs := make([]string, 0)
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			rows.Close()
			return fmt.Errorf("scan orphaned GitLab OAuth credential: %w", err)
		}
		userIDs = append(userIDs, userID)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		return fmt.Errorf("iterate orphaned GitLab OAuth credentials: %w", rowsErr)
	}
	for _, userID := range userIDs {
		if err := r.QueueAndDeleteOAuthCredential(ctx, userID, now); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) ClaimOAuthRevocation(ctx context.Context, now, leaseUntil time.Time) (identity.OAuthTokenRevocation, error) {
	var item identity.OAuthTokenRevocation
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, `
		WITH candidate AS (
			SELECT id
			FROM gitlab_oauth_token_revocations
			WHERE available_at <= $1
			ORDER BY available_at, created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE gitlab_oauth_token_revocations AS revocation
		SET attempts = revocation.attempts + 1,
		    available_at = $2,
		    updated_at = $1
		FROM candidate
		WHERE revocation.id = candidate.id
		RETURNING revocation.id, revocation.user_id,
		          revocation.access_token_ciphertext, revocation.refresh_token_ciphertext,
		          revocation.attempts, revocation.available_at,
		          revocation.created_at, revocation.updated_at
	`, now, leaseUntil).Scan(
		&item.ID, &item.UserID, &item.AccessTokenCiphertext, &item.RefreshTokenCiphertext,
		&item.Attempts, &item.AvailableAt, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.OAuthTokenRevocation{}, identity.ErrOAuthRevocationNotFound
	}
	if err != nil {
		return identity.OAuthTokenRevocation{}, fmt.Errorf("claim GitLab OAuth token revocation: %w", err)
	}
	return item, nil
}

func (r *Repository) CompleteOAuthRevocation(ctx context.Context, id string) error {
	_, err := postgres.Executor(ctx, r.pool).Exec(ctx, `DELETE FROM gitlab_oauth_token_revocations WHERE id = $1`, uuid.MustParse(id))
	return err
}

func (r *Repository) RetryOAuthRevocation(ctx context.Context, id, code string, availableAt, now time.Time) error {
	_, err := postgres.Executor(ctx, r.pool).Exec(ctx, `
		UPDATE gitlab_oauth_token_revocations
		SET available_at = $2, last_error_code = $3, updated_at = $4
		WHERE id = $1
	`, uuid.MustParse(id), availableAt, code, now)
	return err
}

func (r *Repository) OAuthRevocationQueueStats(ctx context.Context, now time.Time) (int64, float64, error) {
	var pending int64
	var oldest float64
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, `
		SELECT count(*),
		       GREATEST(COALESCE(EXTRACT(EPOCH FROM ($1 - min(created_at))), 0), 0)
		FROM gitlab_oauth_token_revocations
	`, now).Scan(&pending, &oldest)
	return pending, oldest, err
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
