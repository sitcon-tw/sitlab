-- +goose Up
CREATE TABLE users (
    id uuid PRIMARY KEY,
    email text NOT NULL,
    password_hash text NOT NULL,
    display_name text NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT users_email_normalized CHECK (email = lower(email))
);
CREATE UNIQUE INDEX users_email_unique ON users (email);

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

CREATE TABLE workspaces (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    created_by_user_id uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE workspace_members (
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
    joined_at timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, user_id)
);
CREATE INDEX workspace_members_user_id_idx ON workspace_members (user_id);

CREATE TABLE tasks (
    id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    status text NOT NULL CHECK (status IN ('todo', 'in_progress', 'done')),
    assignee_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    created_by_user_id uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT tasks_assignee_membership_fk
      FOREIGN KEY (workspace_id, assignee_user_id)
      REFERENCES workspace_members(workspace_id, user_id)
);
CREATE INDEX tasks_workspace_updated_idx ON tasks (workspace_id, updated_at DESC, id DESC);
CREATE INDEX tasks_assignee_idx ON tasks (assignee_user_id) WHERE assignee_user_id IS NOT NULL;

-- +goose StatementBegin
CREATE FUNCTION clear_tasks_for_removed_workspace_member() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    UPDATE tasks
    SET assignee_user_id = NULL, updated_at = now()
    WHERE workspace_id = OLD.workspace_id AND assignee_user_id = OLD.user_id;
    RETURN OLD;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER workspace_member_clear_task_assignees
BEFORE DELETE ON workspace_members
FOR EACH ROW EXECUTE FUNCTION clear_tasks_for_removed_workspace_member();

-- +goose Down
DROP TABLE IF EXISTS tasks;
DROP TRIGGER IF EXISTS workspace_member_clear_task_assignees ON workspace_members;
DROP FUNCTION IF EXISTS clear_tasks_for_removed_workspace_member();
DROP TABLE IF EXISTS workspace_members;
DROP TABLE IF EXISTS workspaces;
DROP TABLE IF EXISTS auth_sessions;
DROP TABLE IF EXISTS users;
