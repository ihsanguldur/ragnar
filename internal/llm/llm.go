package llm

import "context"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Messages    []Message
	Temperature float64
	MaxTokens   int
}

type ChatResponse struct {
	Content string
	Model   string
	Usage   Usage
}

type Usage struct {
	InputTokens  int
	OutputTokens int
}

type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest, onToken func(text string) error) (ChatResponse, error)
}
