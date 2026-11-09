package eval

import (
	"strings"

	"github.com/ihsanguldur/ragnar/internal/rag"
	"github.com/ihsanguldur/ragnar/internal/store"
)

// GenerationResult is how one generated answer was graded.
type GenerationResult struct {
	Answer string `json:"answer"`
	// Correct is the judge's verdict for answerable questions, and whether
	// the model refused for unanswerable ones.
	Correct bool   `json:"correct"`
	Refused bool   `json:"refused"`
	Reason  string `json:"reason,omitempty"`
	// Citations is how well the cited sources cover the expected pages.
	// Unanswerable questions expect none, so it stays zero for them.
	Citations        RetrievalResult `json:"citations"`
	InvalidCitations int             `json:"invalid_citations"`
	// JudgeFailed means the judge gave no usable verdict. The answer then
	// counts as incorrect, and the summary reports how often it happened.
	JudgeFailed bool `json:"judge_failed"`
}

// IsRefusal reports whether the model gave the no-answer sentence. The
// prompt asks for that exact sentence, so anything else counts as an answer.
// The reply must start with it: a made-up answer that ends with the sentence
// is still an answer, and has to be graded as one rather than hidden as a
// refusal.
func IsRefusal(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), rag.NoAnswer)
}

// ScoreCitations checks the sources an answer cites against the expected
// sources, with the same page rules as ScoreRetrieval.
func ScoreCitations(sources []Source, cited []rag.Source) RetrievalResult {
	matches := make([]store.ChunkMatch, 0, len(cited))
	for _, c := range cited {
		matches = append(matches, store.ChunkMatch{Filename: c.Filename, PageStart: c.PageStart, PageEnd: c.PageEnd})
	}
	return ScoreRetrieval(sources, matches)
}

type GenerationSummary struct {
	Questions int     `json:"questions"`
	Accuracy  float64 `json:"accuracy"`
	// Refused is the share of questions the model refused. For answerable
	// questions it should be close to 0, for unanswerable ones close to 1.
	Refused float64 `json:"refused"`
	// CitationRecall is the average share of expected pages the answer
	// cites, over the CitationQuestions questions that expect any. With none,
	// both are 0 and the recall means "not applicable", not "all missed".
	CitationRecall    float64 `json:"citation_recall"`
	CitationQuestions int     `json:"citation_questions"`
	InvalidCitations  int     `json:"invalid_citations"`
	JudgeErrors       int     `json:"judge_errors"`
}

func SummarizeGeneration(results []GenerationResult) GenerationSummary {
	s := GenerationSummary{Questions: len(results)}
	if len(results) == 0 {
		return s
	}
	for _, r := range results {
		if r.Correct {
			s.Accuracy++
		}
		if r.Refused {
			s.Refused++
		}
		if r.Citations.Expected > 0 {
			s.CitationRecall += float64(r.Citations.Found) / float64(r.Citations.Expected)
			s.CitationQuestions++
		}
		s.InvalidCitations += r.InvalidCitations
		if r.JudgeFailed {
			s.JudgeErrors++
		}
	}
	s.Accuracy /= float64(len(results))
	s.Refused /= float64(len(results))
	if s.CitationQuestions > 0 {
		s.CitationRecall /= float64(s.CitationQuestions)
	}
	return s
}
