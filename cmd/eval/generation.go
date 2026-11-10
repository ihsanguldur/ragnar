package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/ihsanguldur/ragnar/internal/eval"
	"github.com/ihsanguldur/ragnar/internal/rag"
)

// runGeneration asks every question through the same service /ask uses,
// grades the answers, and prints a per-question table and a summary per
// question type. Each result is stored in the matching entry of reports, and
// the summaries are returned keyed by question type.
func runGeneration(ctx context.Context, questions []eval.Question, reports []eval.QuestionReport, svc *rag.Service, judge *eval.Judge) (map[string]eval.GenerationSummary, error) {
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tTYPE\tCORRECT\tCITED\tANSWER\tREASON")
	byType := make(map[eval.QuestionType][]eval.GenerationResult)
	for i, q := range questions {
		answer, err := svc.Ask(ctx, q.Question, func(string) error { return nil })
		if err != nil {
			return nil, fmt.Errorf("question %s: %w", q.ID, err)
		}

		r := eval.GenerationResult{
			Answer:           answer.Text,
			Refused:          eval.IsRefusal(answer.Text),
			InvalidCitations: answer.InvalidCitations,
		}
		// Refusals are graded without the judge: for unanswerable questions
		// refusing is the right answer, for the others it is always wrong.
		switch {
		case q.Type == eval.TypeUnanswerable:
			r.Correct = r.Refused
		case r.Refused:
			r.Reason = "refused"
		default:
			r.Citations = eval.ScoreCitations(q.Sources, answer.Sources)
			v, err := judge.Grade(ctx, q.Question, q.Answer, answer.Text)
			switch {
			case errors.Is(err, eval.ErrJudgeReply):
				r.JudgeFailed, r.Reason = true, err.Error()
			case err != nil:
				return nil, fmt.Errorf("question %s: %w", q.ID, err)
			default:
				r.Correct, r.Reason = v.Correct, v.Reason
			}
		}
		reports[i].Generation = &r
		byType[q.Type] = append(byType[q.Type], r)

		fmt.Fprintf(tw, "%s\t%s\t%t\t%d/%d\t%s\t%s\n", q.ID, q.Type, r.Correct,
			r.Citations.Found, r.Citations.Expected, shorten(answer.Text, 60),
			shorten(r.Reason, 60))
	}
	tw.Flush()

	fmt.Println()
	tw = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TYPE\tQUESTIONS\tACCURACY\tREFUSED\tCITATION RECALL\tINVALID CITATIONS\tJUDGE ERRORS")
	summaries := make(map[string]eval.GenerationSummary)
	for _, t := range eval.Types {
		// A type with no questions in this golden set gets no row, rather
		// than a row of zeros that would read as all answers being wrong.
		if len(byType[t]) == 0 {
			continue
		}
		s := eval.SummarizeGeneration(byType[t])
		summaries[string(t)] = s
		// "-" rather than 0.000 when no question of this type expects a
		// source, so it does not read as every citation being missed.
		citation := "-"
		if s.CitationQuestions > 0 {
			citation = fmt.Sprintf("%.3f", s.CitationRecall)
		}
		fmt.Fprintf(tw, "%s\t%d\t%.3f\t%.3f\t%s\t%d\t%d\n", t, s.Questions, s.Accuracy, s.Refused, citation, s.InvalidCitations, s.JudgeErrors)
	}
	return summaries, tw.Flush()
}

// shorten puts text on one line and cuts it to at most n characters, so it
// fits in a table cell. It counts runes rather than bytes, so a multi-byte
// character is never cut in half.
func shorten(text string, n int) string {
	text = strings.Join(strings.Fields(text), " ")
	if r := []rune(text); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return text
}
