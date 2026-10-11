package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ihsanguldur/ragnar/internal/llm"
)

type ChatRequest struct {
	Message string `json:"message"`
}

type ChatTokenEvent struct {
	Text string `json:"text"`
}

type ChatDoneEvent struct {
	Model        string `json:"model"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
}

func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.chat.Timeout)
	defer cancel()

	start := time.Now()
	var ttft time.Duration

	resp, err := h.llm.ChatStream(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: h.chat.SystemPrompt},
			{Role: llm.RoleUser, Content: req.Message},
		},
		Temperature: h.chat.Temperature,
		MaxTokens:   h.chat.MaxTokens,
	}, func(text string) error {
		if ttft == 0 {
			ttft = time.Since(start)
			startSSE(w)
		}
		return writeSSE(w, "token", ChatTokenEvent{Text: text})
	})
	if err != nil {
		h.chatFailed(w, r, err, ttft, time.Since(start))
		return
	}

	slog.Info(
		"chat",
		"model", resp.Model,
		"input_tokens", resp.Usage.InputTokens,
		"output_tokens", resp.Usage.OutputTokens,
		"ttft_ms", ttft.Milliseconds(),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	writeSSE(w, "done", ChatDoneEvent{
		Model:        resp.Model,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
	})
}

func (h *Handler) chatFailed(w http.ResponseWriter, r *http.Request, err error, ttft, duration time.Duration) {
	if r.Context().Err() != nil {
		slog.Info("chat canceled by client", "ttft_ms", ttft.Milliseconds(), "duration_ms", duration.Milliseconds())
		return
	}

	slog.Error("chat failed", "error", err, "ttft_ms", ttft.Milliseconds(), "duration_ms", duration.Milliseconds())

	msg := "model request failed"
	if errors.Is(err, context.DeadlineExceeded) {
		msg = "model request timeout"
	}

	if ttft == 0 {
		writeError(w, http.StatusBadGateway, msg)
		return
	}

	writeSSE(w, "error", ErrorResponse{Error: msg})
}
