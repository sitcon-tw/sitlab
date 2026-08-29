-- +goose Up
-- Existing ciphertext has no key id or associated-data binding. The rollout
-- requires deleting the old OAuth application to revoke its remote grants, so
-- force every browser through the replacement application and envelope.
DELETE FROM auth_sessions;
DELETE FROM oauth_states;
DELETE FROM gitlab_oauth_credentials;

CREATE TABLE gitlab_oauth_token_revocations (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    access_token_ciphertext bytea NOT NULL,
    refresh_token_ciphertext bytea NOT NULL,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at timestamptz NOT NULL,
    last_error_code text,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX gitlab_oauth_token_revocations_available_idx
    ON gitlab_oauth_token_revocations (available_at, created_at);

-- +goose Down
DROP TABLE IF EXISTS gitlab_oauth_token_revocations;
