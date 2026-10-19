package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/ihsanguldur/ragnar/internal/chunk"
	"github.com/ihsanguldur/ragnar/internal/config"
	"github.com/ihsanguldur/ragnar/internal/embed"
	"github.com/ihsanguldur/ragnar/internal/extract"
	"github.com/ihsanguldur/ragnar/internal/store"
)

type Worker struct {
	store     *store.Store
	extractor extract.Extractor
	embedder  embed.Embedder
	cfg       config.IngestConfig
}

func NewWorker(st *store.Store, ex extract.Extractor, em embed.Embedder, cfg config.IngestConfig) *Worker {
	return &Worker{store: st, extractor: ex, embedder: em, cfg: cfg}
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
		return true, w.fail(ctx, doc, err)
	}

	chunks := chunk.Split(pages, w.cfg.ChunkSize, w.cfg.ChunkOverlap)
	vectors, err := w.embed(ctx, chunks)
	if ctx.Err() != nil {
		return true, nil
	}
	if err != nil {
		return true, w.retry(ctx, doc, err)
	}

	if err := w.store.CompleteDocument(ctx, doc.ID, pages, chunks, vectors, w.embedder.Model()); err != nil {
		return true, err
	}

	slog.Info("document ready",
		"id", doc.ID,
		"filename", doc.Filename,
		"pages", len(pages),
		"chunks", len(chunks),
		"duration", time.Since(start),
	)
	return true, nil
}

func (w *Worker) embed(ctx context.Context, chunks []chunk.Chunk) ([][]float32, error) {
	vectors := make([][]float32, 0, len(chunks))
	for i := 0; i < len(chunks); i += w.cfg.EmbedBatchSize {
		end := min(i+w.cfg.EmbedBatchSize, len(chunks))
		texts := make([]string, 0, end-i)
		for _, c := range chunks[i:end] {
			texts = append(texts, c.Content)
		}

		batchCtx, cancel := context.WithTimeout(ctx, w.cfg.EmbedTimeout)
		vecs, err := w.embedder.Embed(batchCtx, texts)
		cancel()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, fmt.Errorf("embedding timed out after %s", w.cfg.EmbedTimeout)
			}
			return nil, err
		}
		vectors = append(vectors, vecs...)
	}
	return vectors, nil
}

func (w *Worker) retry(ctx context.Context, doc store.Document, err error) error {
	attempt := doc.Attempts + 1
	if attempt >= w.cfg.MaxAttempts {
		return w.fail(ctx, doc, fmt.Errorf("giving up after %d attempts: %w", attempt, err))
	}

	delay := w.cfg.RetryBackoff << time.Duration(doc.Attempts)

	slog.Info("document retrying",
		"id", doc.ID,
		"filename", doc.Filename,
		"attempt", attempt,
		"retry_in", delay,
		"error", err,
	)
	return w.store.RetryDocument(ctx, doc.ID, err.Error(), delay)
}

func (w *Worker) fail(ctx context.Context, doc store.Document, err error) error {
	slog.Warn("document failed", "id", doc.ID, "filename", doc.Filename, "error", err)
	return w.store.FailDocument(ctx, doc.ID, err.Error())
}
