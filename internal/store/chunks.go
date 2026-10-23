package store

import (
	"context"
	"fmt"

	"github.com/pgvector/pgvector-go"
)

// ChunkMatch is a chunk returned by a similarity search, with the document
// it belongs to.
type ChunkMatch struct {
	DocumentID string
	Filename   string
	ChunkIndex int
	PageStart  int
	PageEnd    int
	Content    string
	Score      float64
}

// SearchChunks returns the limit chunks closest to embedding, best first.
// Only chunks embedded with the same model are compared.
//
// <=> is pgvector's cosine distance operator, from 0 for vectors pointing the
// same way to 2 for opposite ones. The score is reported as 1 - distance
// (cosine similarity), so higher means more similar. The ORDER BY uses the
// distance expression itself so the HNSW index can serve the query.
func (s *Store) SearchChunks(ctx context.Context, embedding []float32, model string, limit int) ([]ChunkMatch, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.document_id, d.filename, c.chunk_index, c.page_start, c.page_end, c.content, 1 - (c.embedding <=> $1) AS score
		FROM chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE c.embedding_model = $2
		ORDER BY c.embedding <=> $1
		LIMIT $3`,
		pgvector.NewVector(embedding), model, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("store: search chunks: %w", err)
	}
	defer rows.Close()

	matches := make([]ChunkMatch, 0, limit)
	for rows.Next() {
		var m ChunkMatch
		if err := rows.Scan(&m.DocumentID, &m.Filename, &m.ChunkIndex, &m.PageStart, &m.PageEnd, &m.Content, &m.Score); err != nil {
			return nil, fmt.Errorf("store: scan chunk: %w", err)
		}
		matches = append(matches, m)
	}
	// rows.Next returns false on both the end of the results and an error;
	// rows.Err tells them apart.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: search chunks: %w", err)
	}
	return matches, nil
}
