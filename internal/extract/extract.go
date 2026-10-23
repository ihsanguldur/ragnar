package extract

import (
	"context"
	"errors"
)

// ErrNoText is returned for documents without a text layer, such as scanned
// PDFs. Retrying them will not help, so they are marked as failed.
var ErrNoText = errors.New("document has no extractable text")

// Page is the text of one page. Number is the page number in the original
// document, starting at 1.
type Page struct {
	Number int
	Text   string
}

// Extractor reads a document from disk and returns its text page by page.
type Extractor interface {
	Extract(ctx context.Context, path string) ([]Page, error)
}
