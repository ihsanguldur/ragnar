package embed

import "context"

// Embedder turns text into vectors whose distance reflects how close their
// meanings are.
type Embedder interface {
	// Embed returns one vector per input text, in the same order.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Model names the embedding model. Vectors from different models are not
	// comparable, so the name is stored with every vector and used to filter
	// searches.
	Model() string
}
