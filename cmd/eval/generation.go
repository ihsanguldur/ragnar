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

func runGeneration(ctx context.Context, questions []eval.Question, svc *rag.Service, judge *eval.Judge) error {
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tTYPE\tCORRECT\tCITED\tANSWER\tREASON")
	byType := make(map[eval.QuestionType][]eval.GenerationResult)
	for _, q := range questions {
		answer, err := svc.Ask(ctx, q.Question, func(string) error { return nil })
		if err != nil {
			return fmt.Errorf("question %s: %w", q.ID, err)
		}

		r := eval.GenerationResult{
			Refused:          eval.IsRefusal(answer.Text),
			InvalidCitations: answer.InvalidCitations,
		}
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
				return fmt.Errorf("question %s: %w", q.ID, err)
			default:
				r.Correct, r.Reason = v.Correct, v.Reason
			}
		}
		byType[q.Type] = append(byType[q.Type], r)

		fmt.Fprintf(tw, "%s\t%s\t%t\t%d/%d\t%s\t%s\n", q.ID, q.Type, r.Correct,
			r.Citations.Found, r.Citations.Expected, shorten(answer.Text, 60),
			shorten(r.Reason, 60))
	}
	tw.Flush()

	fmt.Println()
	tw = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TYPE\tQUESTIONS\tACCURACY\tREFUSED\tCITATION RECALL\tINVALID CITATIONS\tJUDGE ERRORS")
	for _, t := range []eval.QuestionType{eval.TypeFact, eval.TypeMulti, eval.TypeUnanswerable} {
		s := eval.SummarizeGeneration(byType[t])
		fmt.Fprintf(tw, "%s\t%d\t%.3f\t%.3f\t%.3f\t%d\t%d\n", t, s.Questions, s.Accuracy, s.Refused, s.CitationRecall, s.InvalidCitations, s.JudgeErrors)
	}
	return tw.Flush()
}

func shorten(text string, n int) string {
	text = strings.Join(strings.Fields(text), " ")
	if r := []rune(text); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return text
}
