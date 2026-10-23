package eval

import "github.com/ihsanguldur/ragnar/internal/store"

// RetrievalResult is how well the retrieved chunks cover the expected pages
// of one question.
type RetrievalResult struct {
	// Rank is the position, starting at 1, of the first chunk that contains
	// an expected page, or 0 if none does.
	Rank int
	// Found is how many expected pages are contained in at least one
	// retrieved chunk, out of Expected.
	Found    int
	Expected int
}

// ScoreRetrieval checks the retrieved chunks against the expected sources. A
// chunk contains a page when it comes from the same document and its page
// range includes the page; the right page number in the wrong document does
// not count. Each expected page is counted once, however many chunks contain
// it.
func ScoreRetrieval(sources []Source, matches []store.ChunkMatch) RetrievalResult {
	var r RetrievalResult
	for _, src := range sources {
		for _, page := range src.Pages {
			r.Expected++
			for i, m := range matches {
				if m.Filename == src.Document && m.PageStart <= page && page <= m.PageEnd {
					r.Found++
					if r.Rank == 0 || i+1 < r.Rank {
						r.Rank = i + 1
					}
					break
				}
			}
		}
	}
	return r
}

// RetrievalSummary aggregates RetrievalResults over a set of questions.
type RetrievalSummary struct {
	Questions int
	// Recall is the average share of expected pages found: whether the
	// model was given the information it needs.
	Recall float64
	// MRR (mean reciprocal rank) is the average of 1/Rank, counting 0 for
	// misses: how close to the top the first relevant chunk was.
	MRR float64
}

// SummarizeRetrieval averages per question, so every question weighs the
// same no matter how many pages it expects. An empty input gives zero
// values instead of dividing by zero.
func SummarizeRetrieval(results []RetrievalResult) RetrievalSummary {
	s := RetrievalSummary{Questions: len(results)}
	if len(results) == 0 {
		return s
	}
	for _, r := range results {
		s.Recall += float64(r.Found) / float64(r.Expected)
		if r.Rank > 0 {
			s.MRR += 1 / float64(r.Rank)
		}
	}
	s.Recall /= float64(len(results))
	s.MRR /= float64(len(results))
	return s
}
