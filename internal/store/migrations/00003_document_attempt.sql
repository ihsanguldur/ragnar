-- +goose Up
ALTER TABLE documents
    ADD COLUMN attempts        integer     NOT NULL DEFAULT 0,
    ADD COLUMN next_attempt_at timestamptz NOT NULL DEFAULT now();

-- +goose Down
ALTER TABLE documents
    DROP COLUMN next_attempt_at,
    DROP COLUMN attempts;