package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
)

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

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "q is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.search.Timeout)
	defer cancel()

	vectors, err := h.embedder.Embed(ctx, []string{q})
	if err != nil {
		slog.Error("embed query failed", "error", err)
		writeError(w, http.StatusBadGateway, "embedding request failed")
		return
	}

	matches, err := h.store.SearchChunks(ctx, vectors[0], h.embedder.Model(), h.search.TopK)
	if err != nil {
		slog.Error("search failed", "error", err)
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}

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
