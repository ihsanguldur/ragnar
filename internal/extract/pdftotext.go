package extract

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// PDFToText extracts text with the pdftotext command from poppler-utils,
// which must be installed on the host.
type PDFToText struct{}

func NewPDFToText() *PDFToText {
	return &PDFToText{}
}

// Extract runs pdftotext as a child process; cancelling ctx kills it, so a
// malformed PDF cannot hang a worker.
func (p *PDFToText) Extract(ctx context.Context, path string) ([]Page, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "pdftotext", "-enc", "UTF-8", path, "-")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdftotext: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return splitPages(stdout.String())
}

// splitPages splits pdftotext output into pages. pdftotext ends every page
// with a form feed (\f), so the last element after splitting is always empty
// and is dropped. Blank pages are skipped but keep their original numbers, so
// page numbers still match the PDF.
func splitPages(out string) ([]Page, error) {
	raw := strings.Split(out, "\f")
	raw = raw[:len(raw)-1]

	var pages []Page
	for i, text := range raw {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		pages = append(pages, Page{Number: i + 1, Text: text})
	}

	if len(pages) == 0 {
		return nil, ErrNoText
	}
	return pages, nil
}
