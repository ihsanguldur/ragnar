package rag

import (
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/ihsanguldur/ragnar/internal/llm"
	"github.com/ihsanguldur/ragnar/internal/store"
)

//go:embed prompts/answer_v1.tmpl
var promptFS embed.FS

const NoAnswer = "There is no information in documents."

var answerTmpl = template.Must(template.ParseFS(promptFS, "prompts/answer_v1.tmpl"))

type Source struct {
	Number     int
	DocumentID string
	Filename   string
	PageStart  int
	PageEnd    int
	Score      float64
	Content    string
}

type Prompt struct {
	Messages []llm.Message
	Sources  []Source
}

func BuildPrompt(question string, matches []store.ChunkMatch) (Prompt, error) {
	sources := make([]Source, len(matches))
	for i, m := range matches {
		sources[i] = Source{
			Number:     i + 1,
			DocumentID: m.DocumentID,
			Filename:   m.Filename,
			PageStart:  m.PageStart,
			PageEnd:    m.PageEnd,
			Score:      m.Score,
			Content:    m.Content,
		}
	}

	var system, user strings.Builder
	if err := answerTmpl.ExecuteTemplate(&system, "system", struct{ NoAnswer string }{NoAnswer}); err != nil {
		return Prompt{}, fmt.Errorf("rag: render system prompt: %w", err)
	}
	data := struct {
		Question string
		Sources  []Source
	}{question, sources}
	if err := answerTmpl.ExecuteTemplate(&user, "user", data); err != nil {
		return Prompt{}, fmt.Errorf("rag: render user prompt: %w", err)
	}

	return Prompt{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: system.String()},
			{Role: llm.RoleUser, Content: user.String()},
		},
		Sources: sources,
	}, nil
}
