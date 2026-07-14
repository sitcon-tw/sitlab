-- +goose Up
ALTER TABLE issue_cache ADD COLUMN start_date date;

ALTER TABLE durable_operations DROP CONSTRAINT durable_operations_kind_check;
ALTER TABLE durable_operations
    ADD CONSTRAINT durable_operations_kind_check
    CHECK (kind IN ('create_card', 'update_details', 'update_team', 'update_assignee', 'update_start_date', 'update_due_date', 'move_card'));

-- +goose Down
UPDATE issue_cache
SET pending_operation_id = NULL, sync_state = 'synced', sync_error = NULL
WHERE pending_operation_id IN (
    SELECT id FROM durable_operations WHERE kind = 'update_start_date'
);
DELETE FROM durable_operations WHERE kind = 'update_start_date';

ALTER TABLE durable_operations DROP CONSTRAINT durable_operations_kind_check;
ALTER TABLE durable_operations
    ADD CONSTRAINT durable_operations_kind_check
    CHECK (kind IN ('create_card', 'update_details', 'update_team', 'update_assignee', 'update_due_date', 'move_card'));

ALTER TABLE issue_cache DROP COLUMN start_date;
