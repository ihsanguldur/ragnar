package eval

import (
	"testing"

	"github.com/ihsanguldur/ragnar/internal/store"
)

func TestScoreRetrieval(t *testing.T) {
	matches := []store.ChunkMatch{
		{Filename: "lease.pdf", PageStart: 1, PageEnd: 2},
		{Filename: "handbook.pdf", PageStart: 3, PageEnd: 4},
		{Filename: "handbook.pdf", PageStart: 5, PageEnd: 5},
	}

	tests := []struct {
		name    string
		sources []Source
		want    RetrievalResult
	}{
		{
			name:    "first chunk",
			sources: []Source{{Document: "lease.pdf", Pages: []int{2}}},
			want:    RetrievalResult{Rank: 1, Found: 1, Expected: 1},
		},
		{
			name:    "page inside a chunk range",
			sources: []Source{{Document: "handbook.pdf", Pages: []int{4}}},
			want:    RetrievalResult{Rank: 2, Found: 1, Expected: 1},
		},
		{
			name:    "same page in another document",
			sources: []Source{{Document: "backup-guide.pdf", Pages: []int{1}}},
			want:    RetrievalResult{Rank: 0, Found: 0, Expected: 1},
		},
		{
			name:    "one of two pages",
			sources: []Source{{Document: "handbook.pdf", Pages: []int{5, 6}}},
			want:    RetrievalResult{Rank: 3, Found: 1, Expected: 2},
		},
		{
			name:    "rank is the best page",
			sources: []Source{{Document: "handbook.pdf", Pages: []int{5, 3}}},
			want:    RetrievalResult{Rank: 2, Found: 2, Expected: 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScoreRetrieval(tt.sources, matches)
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSummarizeRetrieval(t *testing.T) {
	got := SummarizeRetrieval([]RetrievalResult{
		{Rank: 1, Found: 1, Expected: 1},
		{Rank: 2, Found: 1, Expected: 2},
		{Rank: 0, Found: 0, Expected: 1},
		{Rank: 4, Found: 1, Expected: 1},
	})
	want := RetrievalSummary{Questions: 4, Recall: 0.625, MRR: 0.4375}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
