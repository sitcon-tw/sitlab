-- name: CreateAuthSession :one
INSERT INTO auth_sessions (
  id, user_id, token_hash, idle_expires_at, absolute_expires_at, created_at, last_seen_at
) VALUES ($1, $2, $3, $4, $5, $6, $6)
RETURNING *;

-- name: GetAuthSessionByTokenHash :one
SELECT * FROM auth_sessions WHERE token_hash = $1;

-- name: SetAuthSessionCSRFHash :exec
UPDATE auth_sessions SET csrf_token_hash = $2 WHERE id = $1;

-- name: TouchAuthSession :exec
UPDATE auth_sessions SET last_seen_at = $2, idle_expires_at = $3 WHERE id = $1;

-- name: DeleteAuthSessionByTokenHash :exec
DELETE FROM auth_sessions WHERE token_hash = $1;

-- name: DeleteExpiredAuthSession :exec
DELETE FROM auth_sessions WHERE id = $1 AND (idle_expires_at <= $2 OR absolute_expires_at <= $2);
