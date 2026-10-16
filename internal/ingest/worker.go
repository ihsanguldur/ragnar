package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/ihsanguldur/ragnar/internal/config"
	"github.com/ihsanguldur/ragnar/internal/extract"
	"github.com/ihsanguldur/ragnar/internal/store"
)

type Worker struct {
	store     *store.Store
	extractor extract.Extractor
	cfg       config.IngestConfig
}

func NewWorker(st *store.Store, ex extract.Extractor, cfg config.IngestConfig) *Worker {
	return &Worker{store: st, extractor: ex, cfg: cfg}
}

func (w *Worker) Run(ctx context.Context) {
	for ctx.Err() == nil {
		processed, err := w.processNext(ctx)
		if err != nil {
			slog.Error("ingest failed", "error", err)
		}
		if processed && err == nil {
			continue
		}

		select {
		case <-ctx.Done():
		case <-time.After(w.cfg.PollInterval):
		}
	}
}

func (w *Worker) processNext(ctx context.Context) (bool, error) {
	doc, found, err := w.store.ClaimDocument(ctx)
	if err != nil || !found {
		return false, err
	}

	start := time.Now()
	path := filepath.Join(w.cfg.UploadDir, doc.SHA256+".pdf")

	extractCtx, cancel := context.WithTimeout(ctx, w.cfg.ExtractTimeout)
	defer cancel()

	pages, err := w.extractor.Extract(extractCtx, path)
	if ctx.Err() != nil {
		return true, nil
	}
	if err != nil {
		if errors.Is(extractCtx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("extraction timed out after %s", w.cfg.ExtractTimeout)
		}
		slog.Warn("document failed", "id", doc.ID, "filename", doc.Filename, "error", err)

		return true, w.store.FailDocument(ctx, doc.ID, err.Error())
	}

	if err := w.store.CompleteDocument(ctx, doc.ID, pages); err != nil {
		return true, err
	}

	slog.Info("document ready",
		"id", doc.ID,
		"filename", doc.Filename,
		"pages", len(pages),
		"duration", time.Since(start),
	)
	return true, nil
}
