-- +goose Up
-- Retry bookkeeping for ingestion. Storing the schedule in the table, rather
-- than in worker memory, means pending retries survive a restart.
ALTER TABLE documents
    -- Failed attempts so far; the document fails for good at the limit.
    ADD COLUMN attempts        integer     NOT NULL DEFAULT 0,
    -- Workers only claim a pending document once this time has passed.
    ADD COLUMN next_attempt_at timestamptz NOT NULL DEFAULT now();

-- +goose Down
ALTER TABLE documents
    DROP COLUMN next_attempt_at,
    DROP COLUMN attempts;
