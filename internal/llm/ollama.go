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

type Ollama struct {
	baseURL string
	model   string
	client  *http.Client
}

func NewOllama(baseURL, model string) *Ollama {
	return &Ollama{
		baseURL: baseURL,
		model:   model,
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
	NumPredict  int     `json:"num_predict,omitempty"`
}

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

	var content strings.Builder
	dec := json.NewDecoder(resp.Body)
	for {
		var line ollamaChatResponse
		if err := dec.Decode(&line); err != nil {
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
		Think:    false,
		Options: ollamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
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

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("ollama: status %d: %s", resp.StatusCode, bytes.TrimSpace(msg))
	}

	return resp, nil
}
