-- +goose Up
CREATE TABLE realtime_state (
    topic text PRIMARY KEY CHECK (topic = 'bootstrap'),
    revision bigint NOT NULL CHECK (revision > 0),
    updated_at timestamptz NOT NULL
);

INSERT INTO realtime_state (topic, revision, updated_at)
VALUES ('bootstrap', 1, now());

CREATE TABLE gitlab_webhook_deliveries (
    id text PRIMARY KEY,
    scope text NOT NULL CHECK (scope IN ('project', 'group')),
    event_kind text NOT NULL CHECK (event_kind IN ('issue', 'member')),
    event_name text NOT NULL,
    issue_iid bigint,
    state text NOT NULL CHECK (state IN ('pending', 'processing', 'completed', 'dead')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at timestamptz NOT NULL,
    last_error text,
    received_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK ((event_kind = 'issue' AND issue_iid IS NOT NULL) OR (event_kind = 'member' AND issue_iid IS NULL))
);

CREATE INDEX gitlab_webhook_deliveries_pending_idx
    ON gitlab_webhook_deliveries (available_at, received_at)
    WHERE state IN ('pending', 'processing');

-- +goose Down
DROP TABLE IF EXISTS gitlab_webhook_deliveries;
DROP TABLE IF EXISTS realtime_state;
