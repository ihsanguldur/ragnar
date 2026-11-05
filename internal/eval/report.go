package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config is the setup a run was made with, so two reports can be compared
// knowing what changed between them.
type Config struct {
	ChatModel    string  `json:"chat_model"`
	Temperature  float64 `json:"temperature"`
	AnswerPrompt string  `json:"answer_prompt"`
	EmbedModel   string  `json:"embed_model"`
	TopK         int     `json:"top_k"`
	ChunkSize    int     `json:"chunk_size"`
	ChunkOverlap int     `json:"chunk_overlap"`
	JudgeModel   string  `json:"judge_model,omitempty"`
	JudgePrompt  string  `json:"judge_prompt,omitempty"`
}

// QuestionReport holds the results of one question. Retrieval is nil for
// unanswerable questions and Generation is nil when answers were not
// generated, so a step that did not run is left out of the JSON instead of
// showing up as zeros.
type QuestionReport struct {
	ID         string            `json:"id"`
	Type       QuestionType      `json:"type"`
	Retrieval  *RetrievalResult  `json:"retrieval,omitempty"`
	Generation *GenerationResult `json:"generation,omitempty"`
}

// Report is everything one eval run produced. The summaries are keyed by
// question type, and retrieval also has "all".
type Report struct {
	Label      string                       `json:"label"`
	CreatedAt  time.Time                    `json:"created_at"`
	Config     Config                       `json:"config"`
	Retrieval  map[string]RetrievalSummary  `json:"retrieval"`
	Generation map[string]GenerationSummary `json:"generation,omitempty"`
	Questions  []QuestionReport             `json:"questions"`
}

// SaveReport writes r as indented JSON into dir and returns the file's path.
// The name starts with the creation time, so the files sort by date.
func SaveReport(dir string, r Report) (string, error) {
	data, err := json.MarshalIndent(r, "", " ")
	if err != nil {
		return "", fmt.Errorf("eval: save report: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("eval: save report: %w", err)
	}
	path := filepath.Join(dir, r.CreatedAt.Format("20060102-150405")+"_"+r.Label+".json")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("eval: save report: %w", err)
	}
	return path, nil
}
