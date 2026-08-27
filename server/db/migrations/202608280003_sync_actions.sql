-- +goose Up
-- An append-only log of everything a browser would need to learn about. Clients hold
-- the last sync_id they applied and ask for what came after, instead of refetching the
-- entire bootstrap payload every time anything anywhere changes.
--
-- sync_id is realtime_state.revision, not a sequence of its own. That matters: a
-- bigserial hands out its number before the transaction commits, so a reader can
-- observe id 5 while id 4 is still uncommitted, advance past it, and lose id 4 forever
-- once it lands. realtime_state.revision is assigned by an UPDATE that holds a row lock
-- until commit, so numbers are issued in commit order with no gaps, and a reader that
-- can see N can see everything below it.
CREATE TABLE sync_actions (
    sync_id          bigint  NOT NULL,
    seq              integer NOT NULL CHECK (seq >= 0),
    entity           text    NOT NULL CHECK (entity IN
                         ('card', 'card_order', 'list', 'team', 'member', 'preference', 'sync_status')),
    entity_id        text    NOT NULL,
    op               text    NOT NULL CHECK (op IN ('upsert', 'delete')),
    -- NULL broadcasts to everyone. Set for rows only one user may see: user
    -- preferences are per-user and today only stay private because each browser
    -- refetches its own bootstrap.
    audience_user_id uuid REFERENCES users(id) ON DELETE CASCADE,
    payload          jsonb,
    actor_user_id    uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at       timestamptz NOT NULL,
    PRIMARY KEY (sync_id, seq),
    CHECK ((op = 'delete' AND payload IS NULL) OR (op = 'upsert' AND payload IS NOT NULL))
);
CREATE INDEX sync_actions_created_at_idx ON sync_actions (created_at);

-- The highest sync_id that has been pruned. A client asking for anything at or below
-- it cannot be served incrementally and has to start over from a full bootstrap.
ALTER TABLE realtime_state ADD COLUMN action_floor bigint NOT NULL DEFAULT 0;
-- Every checkpoint handed out before this migration predates the log, so each existing
-- client resets exactly once and then syncs incrementally forever.
UPDATE realtime_state SET action_floor = revision WHERE topic = 'bootstrap';

-- +goose Down
ALTER TABLE realtime_state DROP COLUMN action_floor;
DROP TABLE IF EXISTS sync_actions;
