-- +goose Up
ALTER TABLE issue_cache
    ADD COLUMN description text NOT NULL DEFAULT '';

CREATE TABLE issue_cache_assignees (
    issue_iid bigint NOT NULL,
    gitlab_user_id bigint NOT NULL REFERENCES directory_members(gitlab_user_id) ON DELETE CASCADE,
    PRIMARY KEY (issue_iid, gitlab_user_id),
    CONSTRAINT issue_cache_assignees_issue_fk
        FOREIGN KEY (issue_iid) REFERENCES issue_cache(issue_iid)
        ON UPDATE CASCADE ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX issue_cache_assignees_member_idx ON issue_cache_assignees (gitlab_user_id, issue_iid);

INSERT INTO issue_cache_assignees (issue_iid, gitlab_user_id)
SELECT issue_iid, assignee_gitlab_user_id
FROM issue_cache
WHERE assignee_gitlab_user_id IS NOT NULL;

DROP INDEX issue_cache_assignee_idx;
ALTER TABLE issue_cache DROP COLUMN assignee_gitlab_user_id;

ALTER TABLE durable_operations DROP CONSTRAINT durable_operations_kind_check;
ALTER TABLE durable_operations
    ADD CONSTRAINT durable_operations_kind_check
    CHECK (kind IN ('create_card', 'update_details', 'update_team', 'update_assignee', 'update_due_date', 'move_card'));

-- +goose Down
UPDATE issue_cache
SET pending_operation_id = NULL, sync_state = 'synced', sync_error = NULL
WHERE pending_operation_id IN (
    SELECT id FROM durable_operations WHERE kind = 'update_details'
);
DELETE FROM durable_operations WHERE kind = 'update_details';

ALTER TABLE durable_operations DROP CONSTRAINT durable_operations_kind_check;
ALTER TABLE durable_operations
    ADD CONSTRAINT durable_operations_kind_check
    CHECK (kind IN ('create_card', 'update_team', 'update_assignee', 'update_due_date', 'move_card'));

ALTER TABLE issue_cache
    ADD COLUMN assignee_gitlab_user_id bigint REFERENCES directory_members(gitlab_user_id) ON DELETE SET NULL;
UPDATE issue_cache card
SET assignee_gitlab_user_id = (
    SELECT MIN(assignee.gitlab_user_id)
    FROM issue_cache_assignees assignee
    WHERE assignee.issue_iid = card.issue_iid
);
CREATE INDEX issue_cache_assignee_idx ON issue_cache (assignee_gitlab_user_id)
WHERE assignee_gitlab_user_id IS NOT NULL;

DROP TABLE issue_cache_assignees;
ALTER TABLE issue_cache DROP COLUMN description;
