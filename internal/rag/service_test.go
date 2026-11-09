package rag

import (
	"slices"
	"testing"
)

// TestCitedSources covers how [X] markers in an answer are mapped back to
// the prompt's sources: duplicates, the order of first citation, letters
// outside the sources, and bracketed text that is not a citation.
func TestCitedSources(t *testing.T) {
	sources := []Source{{Label: "A"}, {Label: "B"}, {Label: "C"}}
	tests := []struct {
		name        string
		text        string
		wantLabels  []string
		wantInvalid int
	}{
		{"single", "Rent is 32,500 TL [B].", []string{"B"}, 0},
		{"several, in first-cited order", "One [C]. Two [A][C].", []string{"C", "A"}, 0},
		{"out of range", "One [A]. Two [D].", []string{"A"}, 1},
		// Heading numbers copied from a document, and the older [S1]
		// labels, are neither citations nor invalid ones.
		{"numbers are not citations", "See Article 7 [7] and [S1].", []string{}, 0},
		// 'a' - 'A' is 32, so a lowercase letter must never reach the
		// index arithmetic.
		{"lowercase is not a citation", "One [a].", []string{}, 0},
		{"no citations", "There is no information in documents.", []string{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cited, invalid := citedSources(tt.text, sources)
			labels := []string{}
			for _, s := range cited {
				labels = append(labels, s.Label)
			}
			if !slices.Equal(labels, tt.wantLabels) {
				t.Errorf("labels = %v, want %v", labels, tt.wantLabels)
			}
			if invalid != tt.wantInvalid {
				t.Errorf("invalid = %d, want %d", invalid, tt.wantInvalid)
			}
		})
	}
}
