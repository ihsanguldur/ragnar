package eval

import (
	"errors"
	"testing"

	"github.com/ihsanguldur/ragnar/internal/rag"
)

func TestParseVerdict(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    Verdict
		wantErr bool
	}{
		{name: "correct", text: "CORRECT\nAll facts match.", want: Verdict{Correct: true,
			Reason: "All facts match."}},
		{name: "incorrect", text: "INCORRECT\nThe number differs.", want: Verdict{Correct: false, Reason: "The number differs."}},
		{name: "markdown and spaces", text: "  **Correct**  \nFine.", want: Verdict{Correct: true, Reason: "Fine."}},
		{name: "no reason", text: "INCORRECT", want: Verdict{Correct: false}},
		{name: "empty", text: "", wantErr: true},
		{name: "other word", text: "MAYBE\nNot sure.", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVerdict(tt.text)
			if tt.wantErr {
				if !errors.Is(err, ErrJudgeReply) {
					t.Fatalf("got error %v, want ErrJudgeReply", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSummarizeGeneration(t *testing.T) {
	got := SummarizeGeneration([]GenerationResult{
		{Correct: true, Citations: RetrievalResult{Found: 1, Expected: 1}},
		{Correct: false, Citations: RetrievalResult{Found: 1, Expected: 2},
			InvalidCitations: 2},
		{Correct: false, Refused: true},
		{Correct: false, JudgeFailed: true, Citations: RetrievalResult{Found: 0,
			Expected: 1}},
	})
	want := GenerationSummary{
		Questions:         4,
		Accuracy:          0.25,
		Refused:           0.25,
		CitationRecall:    0.5,
		CitationQuestions: 3,
		InvalidCitations:  2,
		JudgeErrors:       1,
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// TestIsRefusal covers surrounding whitespace, a normal answer, and the case
// seen with a Turkish question: a made-up answer that ends with the
// no-answer sentence, which must not count as a refusal.
func TestIsRefusal(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{rag.NoAnswer, true},
		{"  " + rag.NoAnswer + "\n", true},
		{"Employees get 18 days of sick leave [A]. " + rag.NoAnswer, false},
		{"Rent is 32,500 TL [A].", false},
	}
	for _, tt := range tests {
		if got := IsRefusal(tt.text); got != tt.want {
			t.Errorf("IsRefusal(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}
