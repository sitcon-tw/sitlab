-- +goose Up
-- GitLab has one top-level closed state and several granular closed statuses.
-- Keep one Close lane, preserve each cached status value, and remove the
-- intermediate Done lane introduced by 202608310002.
UPDATE issue_cache
SET list_key = 'closed'
WHERE list_key = 'done';

UPDATE durable_operations
SET payload = jsonb_set(payload, '{listKey}', '"closed"')
WHERE payload ->> 'listKey' = 'done';

DELETE FROM board_lists WHERE key = 'done';

UPDATE board_lists
SET display_name = 'Close',
    gitlab_status_name = '',
    position = 5,
    closed = true,
    color = '#108548',
    updated_at = now()
WHERE key = 'closed';

-- Lane identity changed outside the normal action writer. Force connected
-- browsers to replace the intermediate Done/Close mapping with a bootstrap.
UPDATE realtime_state
SET revision = revision + 1,
    action_floor = revision + 1,
    updated_at = now()
WHERE topic = 'bootstrap';

-- +goose Down
-- Restore the intermediate two-lane mapping created by 202608310002.
INSERT INTO board_lists
    (key, display_name, gitlab_status_name, position, closed, color, updated_at)
SELECT 'done', 'Done', 'Done', 5, false, '#108548', updated_at
FROM board_lists
WHERE key = 'closed'
ON CONFLICT (key) DO UPDATE
SET display_name = EXCLUDED.display_name,
    gitlab_status_name = EXCLUDED.gitlab_status_name,
    position = EXCLUDED.position,
    closed = EXCLUDED.closed,
    color = EXCLUDED.color,
    updated_at = EXCLUDED.updated_at;

UPDATE issue_cache
SET list_key = 'done'
WHERE list_key = 'closed'
  AND gitlab_status_name = 'Done';

UPDATE durable_operations
SET payload = jsonb_set(payload, '{listKey}', '"done"')
WHERE payload ->> 'listKey' = 'closed'
  AND payload ->> 'gitLabStatusName' = 'Done';

UPDATE board_lists
SET display_name = 'Close',
    gitlab_status_name = '',
    position = 6,
    closed = true,
    color = '#626168',
    updated_at = now()
WHERE key = 'closed';

UPDATE realtime_state
SET revision = revision + 1,
    action_floor = revision + 1,
    updated_at = now()
WHERE topic = 'bootstrap';
