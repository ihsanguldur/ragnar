package eval

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/ihsanguldur/ragnar/internal/llm"
)

// ErrJudgeReply means the judge answered, but not in the expected format.
// It is a problem with one verdict, not with the run.
var ErrJudgeReply = errors.New("eval: unreadable judge reply")

//go:embed prompts/judge_v1.tmpl
var judgeFS embed.FS

var judgeTmpl = template.Must(template.ParseFS(judgeFS, "prompts/judge_v1.tmpl"))

// Judge uses a chat model to decide whether a generated answer says the same
// thing as the reference answer. This scales to many questions where exact
// string matching cannot, but the judge can be wrong too, so its reasons
// should be spot-checked by hand.
type Judge struct {
	llm llm.Provider
}

func NewJudge(provider llm.Provider) *Judge {
	return &Judge{llm: provider}
}

type Verdict struct {
	Correct bool
	Reason  string
}

func (j *Judge) Grade(ctx context.Context, question, reference, answer string) (Verdict, error) {
	var system, user strings.Builder
	if err := judgeTmpl.ExecuteTemplate(&system, "system", nil); err != nil {
		return Verdict{}, fmt.Errorf("eval: render judge prompt: %w", err)
	}
	data := struct{ Question, Reference, Answer string }{question, reference, answer}
	if err := judgeTmpl.ExecuteTemplate(&user, "user", data); err != nil {
		return Verdict{}, fmt.Errorf("eval: render judge prompt: %w", err)
	}

	// Without reasoning, a small judge model often contradicts its own
	// reason, such as noting a missing fact and still saying CORRECT, so
	// thinking is on. The token limit covers the reasoning as well as the
	// two-line reply. Temperature 0 keeps verdicts as repeatable as possible.
	resp, err := j.llm.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: system.String()},
			{Role: llm.RoleUser, Content: user.String()},
		},
		Temperature: 0,
		MaxTokens:   4096,
		Think:       true,
	})
	if err != nil {
		return Verdict{}, fmt.Errorf("eval: judge: %w", err)
	}
	return parseVerdict(resp.Content)
}

// parseVerdict reads the verdict from the first line. Models sometimes wrap
// it in markdown, as in "**CORRECT**", so surrounding punctuation is ignored.
// An empty reply usually means the reasoning used up the token limit.
func parseVerdict(text string) (Verdict, error) {
	first, rest, _ := strings.Cut(strings.TrimSpace(text), "\n")
	reason := strings.TrimSpace(rest)
	switch strings.ToUpper(strings.Trim(first, " *.:")) {
	case "CORRECT":
		return Verdict{Correct: true, Reason: reason}, nil
	case "INCORRECT":
		return Verdict{Correct: false, Reason: reason}, nil
	}
	return Verdict{}, fmt.Errorf("%w: %q", ErrJudgeReply, text)
}
