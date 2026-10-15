package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Document struct {
	ID        string
	Filename  string
	SHA256    string
	SizeBytes int64
	Status    string
	Error     *string
	PageCount *int
	CreatedAt time.Time
	UpdatedAt time.Time
}

const documentColumns = `id, filename, sha256, size_bytes, status, error, page_count, created_at, updated_at`

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
		&d.CreatedAt,
		&d.UpdatedAt,
	)
	return d, err
}

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
