package rag

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/ihsanguldur/ragnar/internal/llm"
	"github.com/ihsanguldur/ragnar/internal/retrieval"
)

// ErrGenerate wraps failures of the chat model call.
var ErrGenerate = errors.New("rag: generate answer")

// citationRe matches citation markers such as [2] and captures the number.
var citationRe = regexp.MustCompile(`\[(\d+)\]`)

// Service answers questions from the ingested documents:
// retrieve, build the prompt, generate, then map citations.
type Service struct {
	retriever   *retrieval.Retriever
	llm         llm.Provider
	temperature float64
	maxTokens   int
}

func NewService(retriever *retrieval.Retriever, provider llm.Provider, temperature float64, maxTokens int) *Service {
	return &Service{retriever: retriever, llm: provider, temperature: temperature, maxTokens: maxTokens}
}

// Answer is a generated answer. Sources holds only the sources the text
// cites, in the order they are first cited.
type Answer struct {
	Text    string
	Model   string
	Usage   llm.Usage
	Sources []Source
}

// Ask answers question, passing each piece of generated text to onToken as it
// arrives. Retrieval errors are returned as they are (see retrieval.ErrEmbed);
// model errors are wrapped in ErrGenerate.
func (s *Service) Ask(ctx context.Context, question string, onToken func(text string) error) (Answer, error) {
	matches, err := s.retriever.Retrieve(ctx, question)
	if err != nil {
		return Answer{}, err
	}
	// With nothing retrieved there is nothing to ground an answer in, so
	// the model is not called at all. This only happens when no documents
	// are ingested, since the search always returns the nearest chunks.
	if len(matches) == 0 {
		if err := onToken(NoAnswer); err != nil {
			return Answer{}, err
		}
		return Answer{Text: NoAnswer, Sources: []Source{}}, nil
	}

	prompt, err := BuildPrompt(question, matches)
	if err != nil {
		return Answer{}, err
	}

	resp, err := s.llm.ChatStream(ctx, llm.ChatRequest{
		Messages:    prompt.Messages,
		Temperature: s.temperature,
		MaxTokens:   s.maxTokens,
	}, onToken)
	if err != nil {
		return Answer{}, fmt.Errorf("%w: %w", ErrGenerate, err)
	}

	return Answer{
		Text:    resp.Content,
		Sources: citedSources(resp.Content, prompt.Sources),
		Model:   resp.Model,
		Usage:   resp.Usage,
	}, nil
}

// citedSources returns the sources text cites with [n] markers, each once.
// Numbers outside the prompt's sources are invented by the model and are
// dropped. The result is never nil, so it encodes as [] in JSON.
func citedSources(text string, sources []Source) []Source {
	cited := []Source{}
	seen := make(map[int]bool)
	for _, m := range citationRe.FindAllStringSubmatch(text, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil || n < 1 || n > len(sources) || seen[n] {
			continue
		}
		seen[n] = true
		cited = append(cited, sources[n-1])
	}
	return cited
}
