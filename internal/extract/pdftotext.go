package extract

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type PDFToText struct{}

func NewPDFToText() *PDFToText {
	return &PDFToText{}
}

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
