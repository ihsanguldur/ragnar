package eval

import (
	"bytes"
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

// QuestionType groups questions by what they test, so metrics can be
// reported per group.
type QuestionType string

const (
	// TypeFact questions are answered in a single place.
	TypeFact QuestionType = "fact"
	// TypeMulti questions need two or more places, such as two pages.
	TypeMulti QuestionType = "multi"
	// TypeUnanswerable questions are not answered by the documents; the
	// model must refuse instead of guessing.
	TypeUnanswerable QuestionType = "unanswerable"
)

// Source is where the answer to a question is written: a document's file
// name and the pages that contain it.
type Source struct {
	Document string `yaml:"document"`
	Pages    []int  `yaml:"pages"`
}

// Question is one entry of the golden set. Answer is a short reference
// answer to compare generated answers against; it is not expected word for
// word.
type Question struct {
	ID       string       `yaml:"id"`
	Type     QuestionType `yaml:"type"`
	Question string       `yaml:"question"`
	Answer   string       `yaml:"answer"`
	Sources  []Source     `yaml:"sources"`
}

// LoadGolden reads and validates the golden set at path. It is strict on
// purpose: a mistake in hand-written data would otherwise skew the metrics
// without any visible error.
func LoadGolden(path string) ([]Question, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("eval: read golden set: %w", err)
	}

	var golden struct {
		Questions []Question `yaml:"questions"`
	}
	// By default unknown keys are silently ignored, so a typo such as
	// "page:" for "pages:" would leave a question without sources. With
	// KnownFields it is an error instead.
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&golden); err != nil {
		return nil, fmt.Errorf("eval: parse golden set: %w", err)
	}

	seen := make(map[string]bool)
	for _, q := range golden.Questions {
		if q.ID == "" || q.Question == "" {
			return nil, fmt.Errorf("eval: question %q: id and question are required", q.ID)
		}
		if seen[q.ID] {
			return nil, fmt.Errorf("eval: question %q: duplicate id", q.ID)
		}
		seen[q.ID] = true

		switch q.Type {
		case TypeFact, TypeMulti:
			if q.Answer == "" || len(q.Sources) == 0 {
				return nil, fmt.Errorf("eval: question %q: answer and sources are required", q.ID)
			}
		case TypeUnanswerable:
			if len(q.Sources) != 0 {
				return nil, fmt.Errorf("eval: question %q: unanswerable question must not have sources", q.ID)
			}
		default:
			return nil, fmt.Errorf("eval: question %q: unknown type %q", q.ID, q.Type)
		}
	}
	return golden.Questions, nil
}
