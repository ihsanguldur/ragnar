package eval

import (
	"strings"

	"github.com/ihsanguldur/ragnar/internal/rag"
	"github.com/ihsanguldur/ragnar/internal/store"
)

// GenerationResult is how one generated answer was graded.
type GenerationResult struct {
	// Correct is the judge's verdict for answerable questions, and whether
	// the model refused for unanswerable ones.
	Correct bool
	Refused bool
	Reason  string
	// Citations is how well the cited sources cover the expected pages.
	// Unanswerable questions expect none, so it stays zero for them.
	Citations        RetrievalResult
	InvalidCitations int
	// JudgeFailed means the judge gave no usable verdict. The answer then
	// counts as incorrect, and the summary reports how often it happened.
	JudgeFailed bool
}

// IsRefusal reports whether the model gave the no-answer sentence. The
// prompt asks for that exact sentence, so anything else counts as an answer.
func IsRefusal(text string) bool {
	return strings.Contains(text, rag.NoAnswer)
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
	Questions int
	Accuracy  float64
	// Refused is the share of questions the model refused. For answerable
	// questions it should be close to 0, for unanswerable ones close to 1.
	Refused float64
	// CitationRecall is the average share of expected pages the answer
	// cites, over questions that expect any.
	CitationRecall   float64
	InvalidCitations int
	JudgeErrors      int
}

func SummarizeGeneration(results []GenerationResult) GenerationSummary {
	s := GenerationSummary{Questions: len(results)}
	if len(results) == 0 {
		return s
	}
	cited := 0
	for _, r := range results {
		if r.Correct {
			s.Accuracy++
		}
		if r.Refused {
			s.Refused++
		}
		if r.Citations.Expected > 0 {
			s.CitationRecall += float64(r.Citations.Found) / float64(r.Citations.Expected)
			cited++
		}
		s.InvalidCitations += r.InvalidCitations
		if r.JudgeFailed {
			s.JudgeErrors++
		}
	}
	s.Accuracy /= float64(len(results))
	s.Refused /= float64(len(results))
	if cited > 0 {
		s.CitationRecall /= float64(cited)
	}
	return s
}
