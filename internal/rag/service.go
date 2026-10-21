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

var ErrGenerate = errors.New("rag: generate answer")

var citationRe = regexp.MustCompile(`\[(\d+)\]`)

type Service struct {
	retriever   *retrieval.Retriever
	llm         llm.Provider
	temperature float64
	maxTokens   int
}

func NewService(retriever *retrieval.Retriever, provider llm.Provider, temperature float64, maxTokens int) *Service {
	return &Service{retriever: retriever, llm: provider, temperature: temperature, maxTokens: maxTokens}
}

type Answer struct {
	Text    string
	Model   string
	Usage   llm.Usage
	Sources []Source
}

func (s *Service) Ask(ctx context.Context, question string, onToken func(text string) error) (Answer, error) {
	matches, err := s.retriever.Retrieve(ctx, question)
	if err != nil {
		return Answer{}, err
	}
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
