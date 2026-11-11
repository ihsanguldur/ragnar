-- +goose Up
-- Full-text search over chunk content, for keyword search next to the vector
-- search. to_tsvector('english', ...) splits the text into words, drops stop
-- words and reduces the rest to their stems ("employees" -> "employe"), so a
-- query matches other forms of the same word. The column is generated, so
-- inserts still write only content; Postgres fills it in, and computes it
-- for existing rows when the column is added. Generated columns need an
-- immutable expression, which is why the 'english' config is written out
-- rather than passed in.
ALTER TABLE chunks
ADD COLUMN content_tsv tsvector GENERATED ALWAYS AS (to_tsvector('english', content)) STORED;

-- GIN is an inverted index: for each stem it lists the rows that contain
-- it, so a text search does not have to read every chunk.
CREATE INDEX chunks_content_tsv_idx ON chunks USING gin (content_tsv);

-- +goose Down
DROP INDEX chunks_content_tsv_idx;
ALTER TABLE chunks DROP COLUMN content_tsv;
