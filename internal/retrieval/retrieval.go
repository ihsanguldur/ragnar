package retrieval

import (
	"context"
	"errors"
	"fmt"

	"github.com/ihsanguldur/ragnar/internal/embed"
	"github.com/ihsanguldur/ragnar/internal/store"
)

// ErrEmbed wraps failures to embed the query, so callers can tell an
// embedding model failure apart from a database failure.
var ErrEmbed = errors.New("retrieval: embed query")

// Retriever finds the chunks most relevant to a question.
type Retriever struct {
	embedder embed.Embedder
	store    *store.Store
	topK     int
}

// New returns a Retriever that returns up to topK chunks per query. The
// embedder must be the one the documents were ingested with, because
// vectors from different models cannot be compared.
func New(em embed.Embedder, st *store.Store, topK int) *Retriever {
	return &Retriever{embedder: em, store: st, topK: topK}
}

// Retrieve embeds query and returns the closest chunks, best first.
func (r *Retriever) Retrieve(ctx context.Context, query string) ([]store.ChunkMatch, error) {
	vectors, err := r.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEmbed, err)
	}
	return r.store.SearchChunks(ctx, vectors[0], r.embedder.Model(), r.topK)
}
