package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ihsanguldur/ragnar/internal/retrieval"
)

// SearchResult is one retrieved chunk. Score is the cosine similarity to the
// query: higher is closer.
type SearchResult struct {
	DocumentID string  `json:"document_id"`
	Filename   string  `json:"filename"`
	ChunkIndex int     `json:"chunk_index"`
	PageStart  int     `json:"page_start"`
	PageEnd    int     `json:"page_end"`
	Score      float64 `json:"score"`
	Content    string  `json:"content"`
}

type SearchResponse struct {
	Query   string         `json:"query"`
	Results []SearchResult `json:"results"`
}

// Search handles GET /search?q=. It returns the chunks most similar to q,
// best first.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "q is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.search.Timeout)
	defer cancel()

	matches, err := h.retriever.Retrieve(ctx, q)
	if err != nil {
		slog.Error("search failed", "error", err)
		// 502 means an upstream service (Ollama) failed, not ragnar itself.
		if errors.Is(err, retrieval.ErrEmbed) {
			writeError(w, http.StatusBadGateway, "embedding request failed")
			return
		}
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}

	// A non-nil empty slice encodes as [] rather than null.
	resp := SearchResponse{Query: q, Results: make([]SearchResult, 0, len(matches))}
	for _, m := range matches {
		resp.Results = append(resp.Results, SearchResult{
			DocumentID: m.DocumentID,
			Filename:   m.Filename,
			ChunkIndex: m.ChunkIndex,
			PageStart:  m.PageStart,
			PageEnd:    m.PageEnd,
			Score:      m.Score,
			Content:    m.Content,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}
