package llm

import "context"

// Role tells the model who wrote a message: system messages carry
// instructions, user messages the input, assistant messages earlier replies.
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
	Messages []Message
	// Temperature controls randomness; values near 0 give focused, repeatable
	// answers.
	Temperature float64
	// MaxTokens caps the length of the reply, so a runaway generation cannot
	// hold the model indefinitely. For reasoning models it also counts the
	// thinking tokens.
	MaxTokens int
	// Think lets a reasoning model such as qwen3 reason before it replies.
	// It is slower but more accurate on judgement tasks; the reasoning is not
	// part of the reply content.
	Think bool
}

type ChatResponse struct {
	Content string
	Model   string
	Usage   Usage
}

// Usage is the token count of a request, as reported by the provider.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// Provider is a chat model backend. Callers depend on this interface rather
// than on a specific provider, so models can be swapped through config.
type Provider interface {
	// Chat waits for the full reply.
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	// ChatStream calls onToken for each piece of text as it is generated and
	// returns the full reply at the end. If onToken returns an error, the
	// stream stops and that error is returned.
	ChatStream(ctx context.Context, req ChatRequest, onToken func(text string) error) (ChatResponse, error)
}
