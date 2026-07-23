-- +goose Up
ALTER TABLE durable_operations DROP CONSTRAINT durable_operations_issue_fk;
ALTER TABLE durable_operations
    ADD CONSTRAINT durable_operations_issue_fk
    FOREIGN KEY (issue_iid) REFERENCES issue_cache(issue_iid)
    ON UPDATE CASCADE ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED;

-- +goose Down
ALTER TABLE durable_operations DROP CONSTRAINT durable_operations_issue_fk;
ALTER TABLE durable_operations
    ADD CONSTRAINT durable_operations_issue_fk
    FOREIGN KEY (issue_iid) REFERENCES issue_cache(issue_iid)
    ON UPDATE CASCADE DEFERRABLE INITIALLY DEFERRED;
