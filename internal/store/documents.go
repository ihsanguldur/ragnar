package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ihsanguldur/ragnar/internal/chunk"
	"github.com/ihsanguldur/ragnar/internal/extract"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pgvector/pgvector-go"
)

// Document is an uploaded file and its place in the ingestion queue. Error
// holds the last failure, and PageCount is set once the document is ready;
// both are pointers because the columns can be NULL.
type Document struct {
	ID        string
	Filename  string
	SHA256    string
	SizeBytes int64
	Status    string
	Error     *string
	PageCount *int
	Attempts  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// documentColumns lists the columns in the order scanDocument reads them.
// Queries that return documents must select exactly these.
const documentColumns = `id, filename, sha256, size_bytes, status, error, page_count, attempts, created_at, updated_at`

// scanDocument reads one row selected with documentColumns. pgx.Row is
// satisfied by both QueryRow results and Rows, so it serves single and
// multi-row queries.
func scanDocument(row pgx.Row) (Document, error) {
	var d Document
	err := row.Scan(
		&d.ID,
		&d.Filename,
		&d.SHA256,
		&d.SizeBytes,
		&d.Status,
		&d.Error,
		&d.PageCount,
		&d.Attempts,
		&d.CreatedAt,
		&d.UpdatedAt,
	)
	return d, err
}

// CreateDocument queues a new document for ingestion. If a document with the
// same content hash already exists, it returns that one with created set to
// false.
//
// ON CONFLICT DO NOTHING makes the check and the insert one atomic step, so
// two simultaneous uploads of the same file cannot both create a row. When
// the insert is skipped, RETURNING yields no row and the existing document
// is fetched instead.
func (s *Store) CreateDocument(ctx context.Context, filename, sha256 string, size int64) (doc Document, created bool, err error) {
	doc, err = scanDocument(s.pool.QueryRow(ctx, `
		INSERT INTO documents (filename, sha256, size_bytes)
		VALUES ($1, $2, $3)
		ON CONFLICT (sha256) DO NOTHING
		RETURNING `+documentColumns,
		filename, sha256, size,
	))
	if err == nil {
		return doc, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Document{}, false, fmt.Errorf("store: create document: %w", err)
	}

	doc, err = scanDocument(s.pool.QueryRow(ctx,
		`SELECT `+documentColumns+` FROM documents WHERE sha256 = $1`, sha256,
	))
	if err != nil {
		return Document{}, false, fmt.Errorf("store: get document by sha256: %w", err)
	}
	return doc, false, nil
}

// ClaimDocument takes the oldest pending document that is due and marks it
// processing, so the documents table doubles as a job queue.
//
// FOR UPDATE SKIP LOCKED makes concurrent workers skip rows another worker is
// claiming at that moment instead of waiting for them, so each document is
// claimed by exactly one worker. Documents scheduled for a later retry are
// not due until next_attempt_at.
func (s *Store) ClaimDocument(ctx context.Context) (doc Document, found bool, err error) {
	doc, err = scanDocument(s.pool.QueryRow(ctx, `
		UPDATE documents
		SET status = 'processing', updated_at = now()
		WHERE id = (
			SELECT id FROM documents
			WHERE status = 'pending' AND next_attempt_at <= now()
			ORDER BY created_at
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING `+documentColumns,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, false, nil
	}
	if err != nil {
		return Document{}, false, fmt.Errorf("store: claim document: %w", err)
	}
	return doc, true, nil
}

// CompleteDocument saves the pages and the embedded chunks of a document and
// marks it ready. vectors[i] is the embedding of chunks[i].
//
// Everything happens in one transaction, so a document is never ready with
// only some of its chunks saved.
func (s *Store) CompleteDocument(ctx context.Context, id string, pages []extract.Page, chunks []chunk.Chunk, vectors [][]float32, model string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	// Rollback does nothing once Commit has succeeded, so deferring it
	// undoes the transaction on every early return.
	defer tx.Rollback(ctx)

	for _, p := range pages {
		_, err := tx.Exec(ctx,
			`INSERT INTO pages (document_id, page_number, content) VALUES ($1, $2, $3)`,
			id, p.Number, p.Text,
		)
		if err != nil {
			return fmt.Errorf("store: insert page %d: %w", p.Number, err)
		}
	}

	// COPY streams all rows in one round trip, much faster than one INSERT
	// per chunk.
	_, err = tx.CopyFrom(ctx,
		pgx.Identifier{"chunks"},
		[]string{"document_id", "chunk_index", "page_start", "page_end", "content", "embedding", "embedding_model"},
		pgx.CopyFromSlice(len(chunks), func(i int) ([]any, error) {
			c := chunks[i]
			return []any{id, c.Index, c.PageStart, c.PageEnd, c.Content, pgvector.NewVector(vectors[i]), model}, nil
		}),
	)
	if err != nil {
		return fmt.Errorf("store: insert chunks: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE documents
		SET status = 'ready', page_count = $2, error = NULL, updated_at = now()
		WHERE id = $1`,
		id, len(pages),
	)
	if err != nil {
		return fmt.Errorf("store: mark ready: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit: %w", err)
	}
	return nil
}

// FailDocument marks a document as permanently failed. It is not retried.
func (s *Store) FailDocument(ctx context.Context, id, reason string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE documents
		SET status = 'failed', error = $2, attempts = attempts + 1, updated_at = now()
		WHERE id = $1`,
		id, reason,
	)
	if err != nil {
		return fmt.Errorf("store: mark failed: %w", err)
	}
	return nil
}

// RetryDocument puts a document back in the queue, to be claimed again after
// delay. The schedule lives in the database, so it survives restarts.
func (s *Store) RetryDocument(ctx context.Context, id, reason string, delay time.Duration) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE documents
		SET status = 'pending', error = $2, attempts = attempts + 1, next_attempt_at = now() + $3, updated_at = now()
		WHERE id = $1`,
		id, reason, delay,
	)
	if err != nil {
		return fmt.Errorf("store: schedule retry: %w", err)
	}
	return nil
}

// RequeueProcessing returns documents stuck in processing to the queue and
// reports how many there were. It runs at startup: with a single server
// instance, a document can only be processing then if the previous run
// stopped in the middle of it.
func (s *Store) RequeueProcessing(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE documents
		SET status = 'pending', updated_at = now()
		WHERE status = 'processing'`,
	)
	if err != nil {
		return 0, fmt.Errorf("store: requeue processing: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ListDocuments returns all documents, newest first.
func (s *Store) ListDocuments(ctx context.Context) ([]Document, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+documentColumns+` FROM documents ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("store: list documents: %w", err)
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		d, err := scanDocument(rows)
		if err != nil {
			return nil, fmt.Errorf("store: scan document: %w", err)
		}
		docs = append(docs, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list documents: %w", err)
	}
	return docs, nil
}

func (s *Store) GetDocument(ctx context.Context, id string) (doc Document, found bool, err error) {
	doc, err = scanDocument(s.pool.QueryRow(ctx,
		`SELECT `+documentColumns+` FROM documents WHERE id = $1`, id,
	))
	// 22P02 (invalid_text_representation) means id is not a valid UUID. No
	// document can have such an id, so it is reported as not found rather
	// than as a database error.
	var pgErr *pgconn.PgError
	if errors.Is(err, pgx.ErrNoRows) || (errors.As(err, &pgErr) && pgErr.Code == "22P02") {
		return Document{}, false, nil
	}
	if err != nil {
		return Document{}, false, fmt.Errorf("store: get document: %w", err)
	}
	return doc, true, nil
}
