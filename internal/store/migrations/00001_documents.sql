-- +goose Up
CREATE TABLE documents (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    filename   text NOT NULL,
    sha256     text NOT NULL UNIQUE,
    size_bytes bigint NOT NULL,
    status     text NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'processing', 'ready', 'failed')),
    error      text,
    page_count integer,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE pages (
    document_id uuid NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    page_number integer NOT NULL,
    content     text NOT NULL,
    PRIMARY KEY (document_id, page_number)
);

-- +goose Down
DROP TABLE pages;
DROP TABLE documents;