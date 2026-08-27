-- +goose Up
-- gitlab_observed_at records when a GitLab read last confirmed this card exists, as
-- distinct from gitlab_updated_at, which is GitLab's own clock. Pruning compares it
-- against the instant a full sweep began, and both sides of that comparison are then
-- taken from PostgreSQL's clock, so the decision cannot be skewed by the app
-- container's clock drifting away from GitLab's.
ALTER TABLE issue_cache ADD COLUMN gitlab_observed_at timestamptz;
UPDATE issue_cache SET gitlab_observed_at = COALESCE(gitlab_updated_at, updated_at);
CREATE INDEX issue_cache_observed_idx
    ON issue_cache (gitlab_observed_at) WHERE sync_state = 'synced';

-- An issue whose GitLab status maps to no board list used to abort the whole board
-- sync, so one mis-set issue stopped every card from updating and showed everyone an
-- offline board. Quarantining it keeps the failure to that one card. The row is keyed
-- by the GitLab timestamp we rejected, so it is retried only once a human moves the
-- issue on.
CREATE TABLE board_sync_rejects (
    issue_iid         bigint PRIMARY KEY,
    gitlab_updated_at timestamptz NOT NULL,
    reason            text NOT NULL,
    attempts          integer NOT NULL DEFAULT 1 CHECK (attempts >= 0),
    first_seen_at     timestamptz NOT NULL,
    last_seen_at      timestamptz NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS board_sync_rejects;
DROP INDEX IF EXISTS issue_cache_observed_idx;
ALTER TABLE issue_cache DROP COLUMN gitlab_observed_at;
