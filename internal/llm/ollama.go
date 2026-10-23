package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Ollama is a Provider backed by a local Ollama server's /api/chat endpoint.
type Ollama struct {
	baseURL string
	model   string
	numCtx  int
	client  *http.Client
}

// NewOllama returns a client for model. numCtx is the context window in
// tokens; the prompt and the reply together must fit in it, and a larger
// window needs more GPU memory.
func NewOllama(baseURL, model string, numCtx int) *Ollama {
	return &Ollama{
		baseURL: baseURL,
		model:   model,
		numCtx:  numCtx,
		client:  &http.Client{},
	}
}

type ollamaChatRequest struct {
	Model    string        `json:"model"`
	Messages []Message     `json:"messages"`
	Stream   bool          `json:"stream"`
	Think    bool          `json:"think"`
	Options  ollamaOptions `json:"options"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature"`
	// NumPredict is Ollama's name for the maximum number of reply tokens.
	NumPredict int `json:"num_predict,omitempty"`
	NumCtx     int `json:"num_ctx,omitempty"`
}

// ollamaChatResponse is the whole reply when streaming is off, and one line
// of the reply when it is on. Token counts are only set on the final line.
type ollamaChatResponse struct {
	Model           string  `json:"model"`
	Message         Message `json:"message"`
	Done            bool    `json:"done"`
	Error           string  `json:"error"`
	PromptEvalCount int     `json:"prompt_eval_count"`
	EvalCount       int     `json:"eval_count"`
}

func (o *Ollama) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	resp, err := o.send(ctx, req, false)
	if err != nil {
		return ChatResponse{}, err
	}
	defer resp.Body.Close()

	var out ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ChatResponse{}, fmt.Errorf("ollama: decode response: %w", err)
	}

	return ChatResponse{
		Content: out.Message.Content,
		Model:   out.Model,
		Usage: Usage{
			InputTokens:  out.PromptEvalCount,
			OutputTokens: out.EvalCount,
		},
	}, nil
}

func (o *Ollama) ChatStream(ctx context.Context, req ChatRequest, onToken func(text string) error) (ChatResponse, error) {
	resp, err := o.send(ctx, req, true)
	if err != nil {
		return ChatResponse{}, err
	}
	defer resp.Body.Close()

	// A streamed reply is newline-delimited JSON: one object per generated
	// piece of text, ending with an object that has done set to true.
	var content strings.Builder
	dec := json.NewDecoder(resp.Body)
	for {
		var line ollamaChatResponse
		if err := dec.Decode(&line); err != nil {
			// EOF before a done line means the server dropped the stream,
			// so the reply is incomplete.
			if errors.Is(err, io.EOF) {
				return ChatResponse{}, errors.New("ollama: stream ended before done")
			}
			return ChatResponse{}, fmt.Errorf("ollama: read stream: %w", err)
		}
		if line.Error != "" {
			return ChatResponse{}, fmt.Errorf("ollama: %s", line.Error)
		}

		if text := line.Message.Content; text != "" {
			content.WriteString(text)
			if err := onToken(text); err != nil {
				return ChatResponse{}, err
			}
		}

		if line.Done {
			return ChatResponse{
				Content: content.String(),
				Model:   line.Model,
				Usage: Usage{
					InputTokens:  line.PromptEvalCount,
					OutputTokens: line.EvalCount,
				},
			}, nil
		}
	}
}

func (o *Ollama) send(ctx context.Context, req ChatRequest, stream bool) (*http.Response, error) {
	body, err := json.Marshal(ollamaChatRequest{
		Model:    o.model,
		Messages: req.Messages,
		Stream:   stream,
		// Reasoning models such as qwen3 otherwise write a long hidden
		// reasoning section before answering, which adds latency and tokens.
		Think: false,
		Options: ollamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
			NumCtx:      o.numCtx,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ollama: encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: send request: %w", err)
	}

	// On success the caller reads and closes the body; on failure it is read
	// here, capped at 4 KB, so the error message includes Ollama's reason.
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("ollama: status %d: %s", resp.StatusCode, bytes.TrimSpace(msg))
	}

	return resp, nil
}
