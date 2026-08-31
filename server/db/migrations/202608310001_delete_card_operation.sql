-- +goose Up
ALTER TABLE durable_operations DROP CONSTRAINT durable_operations_kind_check;
ALTER TABLE durable_operations
    ADD CONSTRAINT durable_operations_kind_check
    CHECK (kind IN ('create_card', 'update_details', 'update_team', 'update_assignee', 'update_start_date', 'update_due_date', 'update_labels', 'move_card', 'delete_card'));

-- +goose Down
UPDATE issue_cache
SET pending_operation_id = NULL, sync_state = 'synced', sync_error = NULL
WHERE pending_operation_id IN (
    SELECT id FROM durable_operations WHERE kind = 'delete_card'
);
DELETE FROM durable_operations WHERE kind = 'delete_card';

ALTER TABLE durable_operations DROP CONSTRAINT durable_operations_kind_check;
ALTER TABLE durable_operations
    ADD CONSTRAINT durable_operations_kind_check
    CHECK (kind IN ('create_card', 'update_details', 'update_team', 'update_assignee', 'update_start_date', 'update_due_date', 'update_labels', 'move_card'));
