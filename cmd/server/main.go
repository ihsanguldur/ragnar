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
	"github.com/ihsanguldur/ragnar/internal/embed"
	"github.com/ihsanguldur/ragnar/internal/extract"
	"github.com/ihsanguldur/ragnar/internal/ingest"
	"github.com/ihsanguldur/ragnar/internal/llm"
	"github.com/ihsanguldur/ragnar/internal/rag"
	"github.com/ihsanguldur/ragnar/internal/retrieval"
	"github.com/ihsanguldur/ragnar/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// run holds the whole program so that its deferred cleanups run before main
// exits; os.Exit skips deferred calls.
func run() error {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		return err
	}

	// ctx is cancelled on Ctrl+C or when the process is asked to stop
	// (SIGTERM, as sent by docker stop), which starts the shutdown below.
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

	// Requeue documents left half-processed by a previous run before any
	// worker starts.
	n, err := st.RequeueProcessing(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		slog.Info("requeued interrupted documents", "count", n)
	}
	// Workers get their own context so they can be stopped explicitly. The
	// deferred function waits for them to return, so the store is not
	// closed while a worker is still using it.
	workerCtx, stopWorkers := context.WithCancel(ctx)
	var wg sync.WaitGroup
	defer func() {
		stopWorkers()
		wg.Wait()
	}()

	embedder := embed.NewOllama(cfg.Ollama.URL, cfg.Ollama.EmbedModel)
	worker := ingest.NewWorker(st, extract.NewPDFToText(), embedder, cfg.Ingest)
	for range cfg.Ingest.Workers {
		wg.Go(func() { worker.Run(workerCtx) })
	}

	// Queries must be embedded with the same model as the documents, so
	// the retriever shares the workers' embedder.
	retriever := retrieval.New(embedder, st, cfg.Search.TopK)

	provider := llm.NewOllama(cfg.Ollama.URL, cfg.Ollama.ChatModel, cfg.Ollama.NumCtx)

	ragService := rag.NewService(retriever, provider, cfg.Chat.Temperature, cfg.Chat.MaxTokens)

	// There is no write timeout: /ask streams for as long as the model
	// generates, and its own timeout is set per request from config. The
	// header timeout stops clients that open a connection and never finish
	// sending the request.
	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           api.NewRouter(api.NewHandler(retriever, ragService, st, cfg.Chat, cfg.Ingest, cfg.Search)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// ListenAndServe blocks, so it runs in a goroutine while the main
	// goroutine waits for either a server error or a stop signal. The buffer
	// lets the goroutine send its result even if nobody is receiving yet.
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

	// Shutdown stops accepting connections and waits up to 10 seconds for
	// in-flight requests to finish. Its context is new because ctx is already
	// cancelled at this point.
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	// After Shutdown, ListenAndServe returns ErrServerClosed; anything else
	// is a real error.
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
