-- +goose Up
-- Season calendar (籌會 / 站立會議 / 年會) sourced from .sitcon/board-directory.yml.
-- Persisted alongside the directory tables because the unchanged-revision refresh
-- path rebuilds the directory file from this snapshot rather than re-reading disk.
CREATE TABLE directory_milestones (
    date            date        NOT NULL,
    name            text        NOT NULL,
    kind            text        NOT NULL CHECK (kind IN ('organizing', 'standup', 'conference')),
    source_revision text        NOT NULL,
    updated_at      timestamptz NOT NULL,
    PRIMARY KEY (date, name)
);

ALTER TABLE sync_actions DROP CONSTRAINT sync_actions_entity_check;
ALTER TABLE sync_actions
    ADD CONSTRAINT sync_actions_entity_check
    CHECK (entity IN ('card', 'card_order', 'list', 'team', 'member', 'preference', 'sync_status', 'milestone'));

-- +goose Down
ALTER TABLE sync_actions DROP CONSTRAINT sync_actions_entity_check;
DELETE FROM sync_actions WHERE entity = 'milestone';
ALTER TABLE sync_actions
    ADD CONSTRAINT sync_actions_entity_check
    CHECK (entity IN ('card', 'card_order', 'list', 'team', 'member', 'preference', 'sync_status'));
DROP TABLE IF EXISTS directory_milestones;
