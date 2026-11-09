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
//go:embed prompts/answer_v3.tmpl
var promptFS embed.FS

// NoAnswer is the exact sentence the model is told to reply with when the
// sources do not answer the question. It is also returned without calling the
// model when retrieval finds nothing.
const NoAnswer = "There is no information in documents."

// PromptVersion names the answer template in use; eval reports record it.
// The go:embed line above must name the same file, since it cannot read a
// constant.
const PromptVersion = "answer_v3"

// answerTmpl is parsed once at startup; Must panics on a broken template, so
// a mistake in the file stops the program immediately instead of failing on
// the first request. text/template is used rather than html/template because
// the output is a prompt, not HTML, and must not be escaped.
var answerTmpl = template.Must(template.ParseFS(promptFS, "prompts/"+PromptVersion+".tmpl"))

// Source is a retrieved chunk as shown to the model. Label is a letter for
// its position in the prompt, starting at "A", which the model cites as
// [Label]. Letters are used because the documents have numbered headings
// such as "Article 7", and the model cited those numbers when sources were
// labeled [1] or even [S1].
type Source struct {
	Label      string
	DocumentID string
	Filename   string
	PageStart  int
	PageEnd    int
	Score      float64
	Content    string
}

// Prompt is the chat messages to send and the labeled sources they contain,
// kept so that citations in the reply can be mapped back.
type Prompt struct {
	Messages []llm.Message
	Sources  []Source
}

// BuildPrompt renders the system message, which holds the answering rules,
// and the user message, which holds the labeled sources and the question.
// Sources are labeled in retrieval order, so [A] is the best match. There
// are only 26 letters, so more matches than that is an error; top_k is far
// below it.
func BuildPrompt(question string, matches []store.ChunkMatch) (Prompt, error) {
	if len(matches) > 26 {
		return Prompt{}, fmt.Errorf("rag: %d sources, at most 26 can be labeled", len(matches))
	}
	sources := make([]Source, len(matches))
	for i, m := range matches {
		sources[i] = Source{
			Label:      string(rune('A' + i)),
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
