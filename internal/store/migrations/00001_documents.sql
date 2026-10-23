-- +goose Up
-- documents is both the list of uploaded files and the ingestion job queue:
-- workers claim rows with status 'pending' and move them through
-- 'processing' to 'ready' or 'failed'.
CREATE TABLE documents (
    -- uuidv7 ids start with a timestamp, so new rows land at the end of the
    -- primary key index instead of at random positions.
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    filename   text NOT NULL,
    -- Hash of the file contents. UNIQUE turns a repeated upload of the same
    -- file into the existing row instead of a second copy.
    sha256     text NOT NULL UNIQUE,
    size_bytes bigint NOT NULL,
    status     text NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'processing', 'ready', 'failed')),
    -- Last failure message, kept while a retry is pending.
    error      text,
    page_count integer,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Extracted text of each page, as returned by the extractor. Page numbers
-- match the PDF; pages without text have no row.
CREATE TABLE pages (
    document_id uuid NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    page_number integer NOT NULL,
    content     text NOT NULL,
    PRIMARY KEY (document_id, page_number)
);

-- +goose Down
DROP TABLE pages;
DROP TABLE documents;
