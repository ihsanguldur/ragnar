package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
)

// LoadReport reads a report written by SaveReport.
func LoadReport(path string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, fmt.Errorf("eval: read report: %w", err)
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return Report{}, fmt.Errorf("eval: parse report %s: %w", path, err)
	}
	return r, nil
}

// Change is one difference between two reports: a config field, or one
// aspect of one question (ID set). A and B are the values as they should be
// displayed; an empty string means the value is absent on that side.
type Change struct {
	ID    string
	Field string
	A, B  string
}

// ConfigChanges lists the config fields that differ, sorted by their JSON
// name. The fields are found through JSON instead of being listed by hand,
// so a field added to Config later is compared without touching this code.
func ConfigChanges(a, b Config) []Change {
	am, bm := configFields(a), configFields(b)
	// Union of both key sets: a field left out by omitempty on one side must
	// still show up as a change.
	var keys []string
	for k := range am {
		keys = append(keys, k)
	}
	for k := range bm {
		if _, ok := am[k]; !ok {
			keys = append(keys, k)
		}
	}
	// Map iteration order is random; sorting keeps the output stable.
	slices.Sort(keys)

	var changes []Change
	for _, k := range keys {
		if am[k] != bm[k] {
			changes = append(changes, Change{Field: k, A: am[k], B: bm[k]})
		}
	}
	return changes
}

// configFields flattens a Config into its JSON field names and values as
// strings. Config holds only strings and numbers, so neither Marshal nor
// Unmarshal can fail here.
func configFields(c Config) map[string]string {
	data, _ := json.Marshal(c)
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	fields := make(map[string]string, len(m))
	for k, v := range m {
		// JSON numbers decode as float64; Sprint prints 5.0 as "5".
		fields[k] = fmt.Sprint(v)
	}
	return fields
}

// QuestionChanges lists, per question, what turned out differently in b
// compared to a: the rank of the first relevant page, how many expected
// pages were retrieved, whether the answer was judged correct, and how many
// expected pages were cited. Questions are matched by ID, not by position,
// so editing the golden set does not shift the comparison. Steps that did
// not run in one of the reports are skipped.
func QuestionChanges(a, b Report) []Change {
	inB := make(map[string]QuestionReport, len(b.Questions))
	for _, q := range b.Questions {
		inB[q.ID] = q
	}
	inA := make(map[string]bool, len(a.Questions))

	var changes []Change
	for _, qa := range a.Questions {
		inA[qa.ID] = true
		qb, ok := inB[qa.ID]
		if !ok {
			changes = append(changes, Change{ID: qa.ID, Field: "question", A: "present", B: "missing"})
			continue
		}
		if ra, rb := qa.Retrieval, qb.Retrieval; ra != nil && rb != nil {
			if ra.Rank != rb.Rank {
				changes = append(changes, Change{ID: qa.ID, Field: "rank", A: strconv.Itoa(ra.Rank), B: strconv.Itoa(rb.Rank)})
			}
			// Checked separately from rank: a multi-page question can lose
			// one of its pages while the first one keeps its rank.
			if ra.Found != rb.Found {
				changes = append(changes, Change{ID: qa.ID, Field: "pages", A: pages(*ra), B: pages(*rb)})
			}
		}
		ga, gb := qa.Generation, qb.Generation
		if ga == nil || gb == nil {
			continue
		}
		if ga.Correct != gb.Correct {
			changes = append(changes, Change{ID: qa.ID, Field: "correct", A: strconv.FormatBool(ga.Correct), B: strconv.FormatBool(gb.Correct)})
		}
		if ga.Citations.Found != gb.Citations.Found {
			changes = append(changes, Change{ID: qa.ID, Field: "cited", A: pages(ga.Citations), B: pages(gb.Citations)})
		}
	}
	for _, qb := range b.Questions {
		if !inA[qb.ID] {
			changes = append(changes, Change{ID: qb.ID, Field: "question", A: "missing", B: "present"})
		}
	}
	return changes
}

// pages formats found/expected, e.g. "1/2".
func pages(r RetrievalResult) string {
	return fmt.Sprintf("%d/%d", r.Found, r.Expected)
}
