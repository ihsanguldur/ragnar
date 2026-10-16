package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ihsanguldur/ragnar/internal/api"
	"github.com/ihsanguldur/ragnar/internal/config"
	"github.com/ihsanguldur/ragnar/internal/extract"
	"github.com/ihsanguldur/ragnar/internal/ingest"
	"github.com/ihsanguldur/ragnar/internal/llm"
	"github.com/ihsanguldur/ragnar/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.Database.URL)
	if err != nil {
		return err
	}
	defer st.Close()

	if err := os.MkdirAll(cfg.Ingest.UploadDir, 0o755); err != nil {
		return err
	}

	n, err := st.RequeueProcessing(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		slog.Info("requeued interrupted documents", "count", n)
	}
	workerCtx, stopWorkers := context.WithCancel(ctx)
	var wg sync.WaitGroup
	defer func() {
		stopWorkers()
		wg.Wait()
	}()

	worker := ingest.NewWorker(st, extract.NewPDFToText(), cfg.Ingest)
	for range cfg.Ingest.Workers {
		wg.Go(func() { worker.Run(workerCtx) })
	}

	provider := llm.NewOllama(cfg.Ollama.URL, cfg.Ollama.ChatModel)

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           api.NewRouter(api.NewHandler(provider, st, cfg.Chat, cfg.Ingest)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.HTTP.Addr, "chat_model", cfg.Ollama.ChatModel)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
