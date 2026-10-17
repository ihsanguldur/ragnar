-- +goose Up
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE chunks (
    document_id     uuid NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    chunk_index     integer NOT NULL,
    page_start      integer NOT NULL,
    page_end        integer NOT NULL,
    content         text NOT NULL,
    embedding       vector(1024) NOT NULL,
    embedding_model text NOT NULL,
    PRIMARY KEY (document_id, chunk_index)
);  

CREATE INDEX chunks_embedding_idx ON chunks USING hnsw (embedding vector_cosine_ops);

-- +goose Down
DROP TABLE chunks;
