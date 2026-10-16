package extract

import (
	"context"
	"errors"
)

var ErrNoText = errors.New("document has no extractable text")

type Page struct {
	Number int
	Text   string
}

type Extractor interface {
	Extract(ctx context.Context, path string) ([]Page, error)
}
