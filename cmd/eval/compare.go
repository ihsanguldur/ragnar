package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"text/tabwriter"
	"time"

	"github.com/ihsanguldur/ragnar/internal/eval"
)

// runCompare implements "eval compare <a.json> <b.json>". It prints three
// tables: the config fields that differ, every metric side by side with its
// change from A to B, and the questions whose outcome changed.
func runCompare(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: eval compare <a.json> <b.json>")
	}
	a, err := eval.LoadReport(args[0])
	if err != nil {
		return err
	}
	b, err := eval.LoadReport(args[1])
	if err != nil {
		return err
	}
	fmt.Printf("A: %s (%s)\nB: %s (%s)\n\n", a.Label, a.CreatedAt.Format(time.DateTime), b.Label, b.CreatedAt.Format(time.DateTime))

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "CONFIG\tA\tB")
	changes := eval.ConfigChanges(a.Config, b.Config)
	if len(changes) == 0 {
		fmt.Fprintln(tw, "(same)\t\t")
	}
	for _, c := range changes {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", c.Field, orDash(c.A), orDash(c.B))
	}
	tw.Flush()

	fmt.Println()
	tw = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TYPE\tMETRIC\tA\tB\tDELTA")
	// The summaries are keyed by type name, plus "all" for retrieval. A type
	// that is in neither report gets no rows; one that is in only one of
	// them still shows, with "-" on the other side.
	keys := make([]string, 0, len(eval.Types)+1)
	for _, t := range eval.Types {
		keys = append(keys, string(t))
	}
	for _, t := range append(slices.Clone(keys), "all") {
		ra, okA := a.Retrieval[t]
		rb, okB := b.Retrieval[t]
		if !okA && !okB {
			continue
		}
		metricRow(tw, t, "recall", "%.3f", ra.Recall, rb.Recall, okA, okB)
		metricRow(tw, t, "mrr", "%.3f", ra.MRR, rb.MRR, okA, okB)
	}
	// Reading a nil map is safe, so a retrieval-only report simply shows "-"
	// in its generation column.
	if a.Generation != nil || b.Generation != nil {
		for _, t := range keys {
			ga, okA := a.Generation[t]
			gb, okB := b.Generation[t]
			if !okA && !okB {
				continue
			}
			metricRow(tw, t, "accuracy", "%.3f", ga.Accuracy, gb.Accuracy, okA, okB)
			metricRow(tw, t, "refused", "%.3f", ga.Refused, gb.Refused, okA, okB)
			if ga.CitationQuestions > 0 || gb.CitationQuestions > 0 {
				metricRow(tw, t, "citation_recall", "%.3f", ga.CitationRecall, gb.CitationRecall, okA, okB)
			}
			metricRow(tw, t, "invalid_citations", "%.0f", float64(ga.InvalidCitations), float64(gb.InvalidCitations), okA, okB)
			metricRow(tw, t, "judge_errors", "%.0f", float64(ga.JudgeErrors), float64(gb.JudgeErrors), okA, okB)
		}
	}
	tw.Flush()

	fmt.Println()
	tw = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "QUESTION\tCHANGE\tA\tB")
	qchanges := eval.QuestionChanges(a, b)
	if len(qchanges) == 0 {
		fmt.Fprintln(tw, "(none)\t\t\t")
	}
	for _, c := range qchanges {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", c.ID, c.Field, c.A, c.B)
	}
	return tw.Flush()
}

// metricRow prints one metric for both reports. okA and okB tell whether
// the metric exists in that report; a missing one is shown as "-", which is
// different from a measured zero. The delta is printed only when the values
// differ as displayed, so rounding noise does not show up as "+0.000".
func metricRow(w io.Writer, typ, name, format string, a, b float64, okA, okB bool) {
	va, vb, delta := "-", "-", ""
	if okA {
		va = fmt.Sprintf(format, a)
	}
	if okB {
		vb = fmt.Sprintf(format, b)
	}
	if okA && okB && va != vb {
		// "%.3f" becomes "%+.3f" so positive deltas get a sign too.
		delta = fmt.Sprintf("%+"+format[1:], b-a)
	}
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", typ, name, va, vb, delta)
}

// orDash shows an absent value as "-".
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
