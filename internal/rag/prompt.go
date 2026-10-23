package rag

import (
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/ihsanguldur/ragnar/internal/llm"
	"github.com/ihsanguldur/ragnar/internal/store"
)

// The prompt lives in a template file rather than in Go strings, so it can be
// read and changed as plain text. The version in the file name lets
// different wordings be compared side by side.
//
//go:embed prompts/answer_v1.tmpl
var promptFS embed.FS

// NoAnswer is the exact sentence the model is told to reply with when the
// sources do not answer the question. It is also returned without calling the
// model when retrieval finds nothing.
const NoAnswer = "There is no information in documents."

// answerTmpl is parsed once at startup; Must panics on a broken template, so
// a mistake in the file stops the program immediately instead of failing on
// the first request. text/template is used rather than html/template because
// the output is a prompt, not HTML, and must not be escaped.
var answerTmpl = template.Must(template.ParseFS(promptFS, "prompts/answer_v1.tmpl"))

// Source is a retrieved chunk as shown to the model. Number is its position
// in the prompt, starting at 1, which the model cites as [Number].
type Source struct {
	Number     int
	DocumentID string
	Filename   string
	PageStart  int
	PageEnd    int
	Score      float64
	Content    string
}

// Prompt is the chat messages to send and the numbered sources they contain,
// kept so that citations in the reply can be mapped back.
type Prompt struct {
	Messages []llm.Message
	Sources  []Source
}

// BuildPrompt renders the system message, which holds the answering rules,
// and the user message, which holds the numbered sources and the question.
// Sources are numbered in retrieval order, so [1] is the best match.
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
