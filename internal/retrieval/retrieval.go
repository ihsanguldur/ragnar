package retrieval

import (
	"context"
	"errors"
	"fmt"

	"github.com/ihsanguldur/ragnar/internal/embed"
	"github.com/ihsanguldur/ragnar/internal/store"
)

var ErrEmbed = errors.New("retrieval: embed query")

type Retriever struct {
	embedder embed.Embedder
	store    *store.Store
	topK     int
}

func New(em embed.Embedder, st *store.Store, topK int) *Retriever {
	return &Retriever{embedder: em, store: st, topK: topK}
}

func (r *Retriever) Retrieve(ctx context.Context, query string) ([]store.ChunkMatch, error) {
	vectors, err := r.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEmbed, err)
	}
	return r.store.SearchChunks(ctx, vectors[0], r.embedder.Model(), r.topK)
}
