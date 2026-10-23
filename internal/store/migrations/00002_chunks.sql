-- +goose Up
-- pgvector adds the vector column type and distance operators. The database
-- container's init script creates it too; IF NOT EXISTS makes this a no-op
-- there and lets the migration work on any Postgres with pgvector installed.
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE chunks (
    document_id     uuid NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    chunk_index     integer NOT NULL,
    -- A chunk can span pages; these are the first and last of them.
    page_start      integer NOT NULL,
    page_end        integer NOT NULL,
    content         text NOT NULL,
    -- 1024 is the output size of bge-m3. A model with a different size needs
    -- a new column type.
    embedding       vector(1024) NOT NULL,
    -- Searches only compare vectors produced by the same model.
    embedding_model text NOT NULL,
    PRIMARY KEY (document_id, chunk_index)
);

-- HNSW is an approximate nearest-neighbour index: it finds the closest
-- vectors without comparing the query against every row. vector_cosine_ops
-- builds it for cosine distance (<=>), the operator the search query uses.
CREATE INDEX chunks_embedding_idx ON chunks USING hnsw (embedding vector_cosine_ops);

-- +goose Down
DROP TABLE chunks;
