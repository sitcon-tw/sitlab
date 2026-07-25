-- +goose Up
ALTER TABLE directory_team_memberships
    ADD COLUMN is_leader boolean NOT NULL DEFAULT false;

CREATE TABLE gitlab_oauth_credentials (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    access_token_ciphertext bytea NOT NULL,
    refresh_token_ciphertext bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS gitlab_oauth_credentials;
ALTER TABLE directory_team_memberships DROP COLUMN IF EXISTS is_leader;
