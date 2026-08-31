-- +goose Up
-- Done is a lifecycle status on an open GitLab issue. Close is the issue's
-- top-level terminal state, so they need separate board lanes.
INSERT INTO board_lists
    (key, display_name, gitlab_status_name, position, closed, color, updated_at)
SELECT 'done', 'Done', 'Done', 6, false, color, updated_at
FROM board_lists
WHERE key = 'closed'
ON CONFLICT (key) DO NOTHING;

UPDATE issue_cache
SET list_key = 'done', gitlab_status_name = 'Done'
WHERE list_key = 'closed';

UPDATE durable_operations
SET payload = jsonb_set(payload, '{listKey}', '"done"')
WHERE payload ->> 'listKey' = 'closed';

DELETE FROM board_lists WHERE key = 'closed';

UPDATE board_lists
SET display_name = 'Done', gitlab_status_name = 'Done', position = 5, closed = false
WHERE key = 'done';

INSERT INTO board_lists
    (key, display_name, gitlab_status_name, position, closed, color, updated_at)
SELECT 'closed', 'Close', '', 6, true, '#626168', updated_at
FROM board_lists
WHERE key = 'done'
ON CONFLICT (key) DO UPDATE
SET display_name = EXCLUDED.display_name,
    gitlab_status_name = EXCLUDED.gitlab_status_name,
    position = EXCLUDED.position,
    closed = EXCLUDED.closed,
    color = EXCLUDED.color,
    updated_at = EXCLUDED.updated_at;

-- Lane identity changed outside the normal action writer. Advance the checkpoint
-- floor so every connected browser reloads one canonical bootstrap instead of
-- replaying stale actions that still name the legacy closed-as-Done lane.
UPDATE realtime_state
SET revision = revision + 1,
    action_floor = revision + 1,
    updated_at = now()
WHERE topic = 'bootstrap';

-- +goose Down
-- A rollback has only one terminal lane, so cards in either Done or Close are
-- folded back into the legacy Done lane.
UPDATE issue_cache
SET list_key = 'done', gitlab_status_name = 'Done'
WHERE list_key = 'closed';

UPDATE durable_operations
SET payload = jsonb_set(payload, '{listKey}', '"closed"')
WHERE payload ->> 'listKey' IN ('done', 'closed');

DELETE FROM board_lists WHERE key = 'closed';

INSERT INTO board_lists
    (key, display_name, gitlab_status_name, position, closed, color, updated_at)
SELECT 'closed', 'Done', 'Done', 6, true, color, updated_at
FROM board_lists
WHERE key = 'done'
ON CONFLICT (key) DO NOTHING;

UPDATE issue_cache
SET list_key = 'closed', gitlab_status_name = 'Done'
WHERE list_key = 'done';

DELETE FROM board_lists WHERE key = 'done';

UPDATE board_lists
SET display_name = 'Done', gitlab_status_name = 'Done', position = 5, closed = true
WHERE key = 'closed';

UPDATE realtime_state
SET revision = revision + 1,
    action_floor = revision + 1,
    updated_at = now()
WHERE topic = 'bootstrap';
