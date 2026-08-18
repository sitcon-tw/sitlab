-- +goose Up
ALTER TABLE board_lists RENAME COLUMN gitlab_label TO gitlab_status_name;

UPDATE board_lists
SET gitlab_status_name = CASE key
    WHEN 'wating' THEN 'Waiting'
    WHEN 'inbox' THEN 'Inbox'
    WHEN 'todo' THEN 'To do'
    WHEN 'doing' THEN 'Doing'
    WHEN 'review' THEN 'Review'
    WHEN 'closed' THEN 'Done'
    ELSE gitlab_status_name
END;

ALTER TABLE issue_cache
    ADD COLUMN gitlab_status_name text NOT NULL DEFAULT '';

UPDATE issue_cache
SET gitlab_status_name = CASE list_key
    WHEN 'wating' THEN 'Waiting'
    WHEN 'inbox' THEN 'Inbox'
    WHEN 'todo' THEN 'To do'
    WHEN 'doing' THEN 'Doing'
    WHEN 'review' THEN 'Review'
    WHEN 'closed' THEN 'Done'
    ELSE ''
END;

-- +goose Down
ALTER TABLE issue_cache DROP COLUMN gitlab_status_name;

UPDATE board_lists
SET gitlab_status_name = CASE key
    WHEN 'wating' THEN 'Status::Waiting'
    WHEN 'inbox' THEN 'Status::Inbox'
    WHEN 'todo' THEN 'Status::To Do'
    WHEN 'doing' THEN 'Status::Doing'
    WHEN 'review' THEN 'Status::Review'
    WHEN 'closed' THEN ''
    ELSE gitlab_status_name
END;

ALTER TABLE board_lists RENAME COLUMN gitlab_status_name TO gitlab_label;
