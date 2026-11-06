package eval

import (
	"slices"
	"testing"
)

// TestConfigChanges covers a changed field, a field present on one side
// only (omitempty), and the sort order of the result.
func TestConfigChanges(t *testing.T) {
	a := Config{ChatModel: "qwen3:8b", TopK: 5, JudgeModel: "qwen3:8b"}
	b := Config{ChatModel: "qwen3:8b", TopK: 1}
	got := ConfigChanges(a, b)
	want := []Change{
		{Field: "judge_model", A: "qwen3:8b", B: ""},
		{Field: "top_k", A: "5", B: "1"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got := ConfigChanges(a, a); len(got) != 0 {
		t.Errorf("same config: got %+v, want no changes", got)
	}
}

// TestQuestionChanges covers a generation-only change (q1), a
// retrieval-only report entry (q2), and questions present in only one of
// the reports (q3, q4).
func TestQuestionChanges(t *testing.T) {
	a := Report{Questions: []QuestionReport{
		{ID: "q1", Retrieval: &RetrievalResult{Rank: 1, Found: 1, Expected: 1},
			Generation: &GenerationResult{Correct: true, Citations: RetrievalResult{Found: 1, Expected: 1}}},
		{ID: "q2", Retrieval: &RetrievalResult{Rank: 2, Found: 1, Expected: 1}},
		{ID: "q3"},
	}}
	b := Report{Questions: []QuestionReport{
		{ID: "q1", Retrieval: &RetrievalResult{Rank: 1, Found: 1, Expected: 1},
			Generation: &GenerationResult{Correct: false, Citations: RetrievalResult{Found: 0, Expected: 1}}},
		{ID: "q2", Retrieval: &RetrievalResult{Rank: 0, Found: 0, Expected: 1}},
		{ID: "q4"},
	}}
	got := QuestionChanges(a, b)
	want := []Change{
		{ID: "q1", Field: "correct", A: "true", B: "false"},
		{ID: "q1", Field: "cited", A: "1/1", B: "0/1"},
		{ID: "q2", Field: "rank", A: "2", B: "0"},
		{ID: "q2", Field: "pages", A: "1/1", B: "0/1"},
		{ID: "q3", Field: "question", A: "present", B: "missing"},
		{ID: "q4", Field: "question", A: "missing", B: "present"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
