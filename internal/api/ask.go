package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ihsanguldur/ragnar/internal/rag"
	"github.com/ihsanguldur/ragnar/internal/retrieval"
)

type AskRequest struct {
	Question string `json:"question"`
}

type AskSource struct {
	Number     int     `json:"number"`
	DocumentID string  `json:"document_id"`
	Filename   string  `json:"filename"`
	PageStart  int     `json:"page_start"`
	PageEnd    int     `json:"page_end"`
	Score      float64 `json:"score"`
}

type AskSourcesEvent struct {
	Sources []AskSource `json:"sources"`
}

type AskTokenEvent struct {
	Text string `json:"text"`
}

type AskDoneEvent struct {
	Model        string `json:"model"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
}

func (h *Handler) Ask(w http.ResponseWriter, r *http.Request) {
	var req AskRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Question) == "" {
		writeError(w, http.StatusBadRequest, "question is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.chat.Timeout)
	defer cancel()

	start := time.Now()
	var ttft time.Duration

	answer, err := h.rag.Ask(ctx, req.Question, func(text string) error {
		if ttft == 0 {
			ttft = time.Since(start)
			startSSE(w)
		}
		return writeSSE(w, "token", AskTokenEvent{Text: text})
	})
	if err != nil {
		h.askFailed(w, r, err, ttft, time.Since(start))
		return
	}

	slog.Info(
		"ask",
		"model", answer.Model,
		"cited_sources", len(answer.Sources),
		"input_tokens", answer.Usage.InputTokens,
		"output_tokens", answer.Usage.OutputTokens,
		"ttft_ms", ttft.Milliseconds(),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	sources := make([]AskSource, 0, len(answer.Sources))
	for _, s := range answer.Sources {
		sources = append(sources, AskSource{
			Number:     s.Number,
			DocumentID: s.DocumentID,
			Filename:   s.Filename,
			PageStart:  s.PageStart,
			PageEnd:    s.PageEnd,
			Score:      s.Score,
		})
	}
	writeSSE(w, "sources", AskSourcesEvent{Sources: sources})
	writeSSE(w, "done", AskDoneEvent{
		Model:        answer.Model,
		InputTokens:  answer.Usage.InputTokens,
		OutputTokens: answer.Usage.OutputTokens,
	})
}

func (h *Handler) askFailed(w http.ResponseWriter, r *http.Request, err error, ttft, duration time.Duration) {
	if r.Context().Err() != nil {
		slog.Info("ask canceled by client", "ttft_ms", ttft.Milliseconds(), "duration_ms", duration.Milliseconds())
		return
	}

	slog.Error("ask failed", "error", err, "ttft_ms", ttft.Milliseconds(), "duration_ms", duration.Milliseconds())

	status, msg := http.StatusInternalServerError, "answer failed"
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		status, msg = http.StatusBadGateway, "request timeout"
	case errors.Is(err, retrieval.ErrEmbed):
		status, msg = http.StatusBadGateway, "embedding request failed"
	case errors.Is(err, rag.ErrGenerate):
		status, msg = http.StatusBadGateway, "model request failed"
	}

	if ttft == 0 {
		writeError(w, status, msg)
		return
	}

	writeSSE(w, "error", ErrorResponse{Error: msg})
}
