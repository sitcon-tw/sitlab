-- +goose Up
CREATE TABLE users (
    id uuid PRIMARY KEY,
    gitlab_user_id bigint NOT NULL UNIQUE,
    username text NOT NULL,
    display_name text NOT NULL,
    avatar_url text,
    profile_url text NOT NULL,
    access_level integer NOT NULL CHECK (access_level BETWEEN 0 AND 100),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX users_username_unique ON users (lower(username));

CREATE TABLE auth_sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    csrf_token_hash bytea,
    idle_expires_at timestamptz NOT NULL,
    absolute_expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL
);
CREATE INDEX auth_sessions_user_id_idx ON auth_sessions (user_id);
CREATE INDEX auth_sessions_expiry_idx ON auth_sessions (idle_expires_at, absolute_expires_at);

CREATE TABLE oauth_states (
    state_hash bytea PRIMARY KEY,
    verifier_ciphertext bytea NOT NULL,
    return_path text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL
);
CREATE INDEX oauth_states_expiry_idx ON oauth_states (expires_at);

CREATE TABLE directory_teams (
    key text PRIMARY KEY,
    display_name text NOT NULL,
    title_prefix text NOT NULL,
    gitlab_label text NOT NULL,
    sort_order integer NOT NULL,
    active boolean NOT NULL,
    source_revision text NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX directory_teams_sort_order_unique ON directory_teams (sort_order);

CREATE TABLE directory_members (
    gitlab_user_id bigint PRIMARY KEY,
    username text NOT NULL,
    display_name text NOT NULL,
    avatar_url text,
    profile_url text NOT NULL,
    access_level integer NOT NULL CHECK (access_level BETWEEN 0 AND 100),
    state text NOT NULL CHECK (state IN ('active', 'blocked', 'deactivated')),
    last_synced_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX directory_members_username_unique ON directory_members (lower(username));

CREATE TABLE directory_team_memberships (
    team_key text NOT NULL REFERENCES directory_teams(key) ON DELETE CASCADE,
    gitlab_user_id bigint NOT NULL REFERENCES directory_members(gitlab_user_id) ON DELETE CASCADE,
    source text NOT NULL CHECK (source IN ('gitlab_directory', 'self_selected')),
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (team_key, gitlab_user_id, source)
);
CREATE INDEX directory_team_memberships_member_idx ON directory_team_memberships (gitlab_user_id);

CREATE TABLE user_preferences (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    default_team_key text NOT NULL REFERENCES directory_teams(key),
    confirmed_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE board_lists (
    key text PRIMARY KEY,
    display_name text NOT NULL,
    gitlab_label text NOT NULL,
    position integer NOT NULL,
    closed boolean NOT NULL,
    color text NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX board_lists_position_unique ON board_lists (position);

CREATE SEQUENCE local_issue_iid_seq AS bigint
    START WITH -1
    INCREMENT BY -1
    MINVALUE -9223372036854775808
    MAXVALUE -1
    NO CYCLE;

CREATE TABLE issue_cache (
    issue_iid bigint PRIMARY KEY DEFAULT nextval('local_issue_iid_seq'),
    gitlab_issue_id bigint UNIQUE,
    title text NOT NULL,
    web_url text,
    list_key text NOT NULL REFERENCES board_lists(key),
    position integer NOT NULL CHECK (position >= 0),
    team_key text NOT NULL REFERENCES directory_teams(key),
    assignee_gitlab_user_id bigint REFERENCES directory_members(gitlab_user_id) ON DELETE SET NULL,
    due_date date,
    labels text[] NOT NULL DEFAULT '{}',
    sync_state text NOT NULL CHECK (sync_state IN ('pending', 'processing', 'synced', 'failed')),
    sync_error text,
    pending_operation_id uuid,
    gitlab_updated_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
ALTER SEQUENCE local_issue_iid_seq OWNED BY issue_cache.issue_iid;
CREATE INDEX issue_cache_list_position_idx ON issue_cache (list_key, position, issue_iid);
CREATE INDEX issue_cache_assignee_idx ON issue_cache (assignee_gitlab_user_id) WHERE assignee_gitlab_user_id IS NOT NULL;

CREATE TABLE durable_operations (
    id uuid PRIMARY KEY,
    kind text NOT NULL CHECK (kind IN ('create_card', 'update_team', 'update_assignee', 'update_due_date', 'move_card')),
    issue_iid bigint,
    requested_by_user_id uuid NOT NULL REFERENCES users(id),
    payload jsonb NOT NULL,
    state text NOT NULL CHECK (state IN ('pending', 'processing', 'synced', 'failed')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at timestamptz NOT NULL,
    last_error_code text,
    last_error_detail text,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX durable_operations_pending_idx ON durable_operations (available_at, created_at) WHERE state IN ('pending', 'processing');
CREATE INDEX durable_operations_issue_idx ON durable_operations (issue_iid, created_at DESC) WHERE issue_iid IS NOT NULL;
ALTER TABLE durable_operations
    ADD CONSTRAINT durable_operations_issue_fk
    FOREIGN KEY (issue_iid) REFERENCES issue_cache(issue_iid)
    ON UPDATE CASCADE DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE issue_cache
    ADD CONSTRAINT issue_cache_pending_operation_fk
    FOREIGN KEY (pending_operation_id) REFERENCES durable_operations(id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE sync_snapshots (
    resource text PRIMARY KEY CHECK (resource IN ('directory', 'members', 'board')),
    source_revision text NOT NULL,
    last_success_at timestamptz NOT NULL,
    last_attempt_at timestamptz NOT NULL,
    last_error text,
    updated_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS sync_snapshots;
ALTER TABLE IF EXISTS issue_cache DROP CONSTRAINT IF EXISTS issue_cache_pending_operation_fk;
DROP TABLE IF EXISTS durable_operations;
DROP TABLE IF EXISTS issue_cache;
DROP TABLE IF EXISTS board_lists;
DROP TABLE IF EXISTS user_preferences;
DROP TABLE IF EXISTS directory_team_memberships;
DROP TABLE IF EXISTS directory_members;
DROP TABLE IF EXISTS directory_teams;
DROP TABLE IF EXISTS oauth_states;
DROP TABLE IF EXISTS auth_sessions;
DROP TABLE IF EXISTS users;
