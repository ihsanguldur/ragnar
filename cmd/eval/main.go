package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/ihsanguldur/ragnar/internal/config"
	"github.com/ihsanguldur/ragnar/internal/embed"
	"github.com/ihsanguldur/ragnar/internal/eval"
	"github.com/ihsanguldur/ragnar/internal/extract"
	"github.com/ihsanguldur/ragnar/internal/ingest"
	"github.com/ihsanguldur/ragnar/internal/retrieval"
	"github.com/ihsanguldur/ragnar/internal/store"
	"github.com/jackc/pgx/v5"
)

func main() {
	if err := run(); err != nil {
		slog.Error("eval failed", "error", err)
		os.Exit(1)
	}
}

// run ingests the eval documents into a throwaway database, runs every
// answerable question through retrieval, and prints a per-question table
// followed by recall and MRR per question type. Settings such as top_k or
// chunk_size are changed with the usual RAGNAR_ environment variables.
func run() error {
	goldenPath := flag.String("golden", "evals/golden.yaml", "golden question set")
	docsDir := flag.String("docs", "evals/docs", "directory with the PDFs to ingest")
	flag.Parse()

	cfg, err := config.Load("config.yaml")
	if err != nil {
		return err
	}
	questions, err := eval.LoadGolden(*goldenPath)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Deferred calls run in reverse order: the upload directory is removed,
	// then the pool is closed, then the database is dropped.
	dbURL, dropDB, err := createTempDB(ctx, cfg.Database.URL)
	if err != nil {
		return err
	}
	defer dropDB()

	st, err := store.Open(ctx, dbURL)
	if err != nil {
		return err
	}
	defer st.Close()

	uploadDir, err := os.MkdirTemp("", "ragnar-eval-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(uploadDir)
	cfg.Ingest.UploadDir = uploadDir

	embedder := embed.NewOllama(cfg.Ollama.URL, cfg.Ollama.EmbedModel)
	start := time.Now()
	if err := ingestDocs(ctx, st, embedder, cfg.Ingest, *docsDir); err != nil {
		return err
	}
	slog.Info("documents ingested", "duration", time.Since(start))

	retriever := retrieval.New(embedder, st, cfg.Search.TopK)
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tTYPE\tRANK\tPAGES\tTOP RESULT")
	byType := make(map[eval.QuestionType][]eval.RetrievalResult)
	var all []eval.RetrievalResult
	for _, q := range questions {
		// Unanswerable questions have no expected source, so retrieval
		// metrics do not apply to them.
		if q.Type == eval.TypeUnanswerable {
			continue
		}
		matches, err := retriever.Retrieve(ctx, q.Question)
		if err != nil {
			return fmt.Errorf("question %s: %w", q.ID, err)
		}
		r := eval.ScoreRetrieval(q.Sources, matches)
		byType[q.Type] = append(byType[q.Type], r)
		all = append(all, r)

		top := "-"
		if len(matches) > 0 {
			m := matches[0]
			top = fmt.Sprintf("%s p%d-%d (%.2f)", m.Filename, m.PageStart, m.PageEnd, m.Score)
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d/%d\t%s\n", q.ID, q.Type, r.Rank, r.Found, r.Expected, top)
	}
	tw.Flush()

	fmt.Printf("\nretrieval: top_k=%d chunk_size=%d chunk_overlap=%d embed_model=%s\n", cfg.Search.TopK, cfg.Ingest.ChunkSize, cfg.Ingest.ChunkOverlap, cfg.Ollama.EmbedModel)
	// The types are listed explicitly rather than read from the map, whose
	// iteration order is random, so the rows always come out in this order.
	tw = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TYPE\tQUESTIONS\tRECALL\tMRR")
	for _, t := range []eval.QuestionType{eval.TypeFact, eval.TypeMulti} {
		s := eval.SummarizeRetrieval(byType[t])
		fmt.Fprintf(tw, "%s\t%d\t%.3f\t%.3f\n", t, s.Questions, s.Recall, s.MRR)
	}
	s := eval.SummarizeRetrieval(all)
	fmt.Fprintf(tw, "all\t%d\t%.3f\t%.3f\n", s.Questions, s.Recall, s.MRR)
	return tw.Flush()
}

// createTempDB creates an empty database next to the configured one, so an
// eval run never sees documents uploaded outside of it, and every run
// re-ingests with the current chunking settings. It returns the new
// database's URL and a function that drops it.
func createTempDB(ctx context.Context, baseURL string) (string, func(), error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", nil, fmt.Errorf("parse database url: %w", err)
	}
	// Database names cannot be passed as query parameters, so the name is
	// quoted as an identifier and put into the SQL text.
	name := fmt.Sprintf("ragnar_eval_%d", time.Now().Unix())
	ident := pgx.Identifier{name}.Sanitize()

	conn, err := pgx.Connect(ctx, baseURL)
	if err != nil {
		return "", nil, fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+ident); err != nil {
		return "", nil, fmt.Errorf("create database: %w", err)
	}

	// drop uses a fresh context, because after Ctrl+C the run's context is
	// already cancelled and the cleanup must still happen. FORCE closes any
	// connections that are still open to the database.
	drop := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		conn, err := pgx.Connect(ctx, baseURL)
		if err != nil {
			slog.Error("drop eval database", "name", name, "error", err)
			return
		}
		defer conn.Close(ctx)
		if _, err := conn.Exec(ctx, "DROP DATABASE "+ident+" WITH (FORCE)"); err != nil {
			slog.Error("drop eval database", "name", name, "error", err)
		}
	}

	u.Path = "/" + name
	return u.String(), drop, nil
}

// ingestDocs runs every PDF in dir through the same worker the server uses
// and waits until all of them are ready, so the eval measures the real
// ingestion code rather than a copy of it.
func ingestDocs(ctx context.Context, st *store.Store, em embed.Embedder, cfg config.IngestConfig,
	dir string) error {
	paths, err := filepath.Glob(filepath.Join(dir, "*.pdf"))
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no PDFs in %s", dir)
	}

	// Same steps as an upload: store the file under its content hash, then
	// queue it.
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		hexSum := hex.EncodeToString(sum[:])
		if err := os.WriteFile(filepath.Join(cfg.UploadDir, hexSum+".pdf"), data, 0o644); err != nil {
			return err
		}
		if _, _, err := st.CreateDocument(ctx, filepath.Base(path), hexSum,
			int64(len(data))); err != nil {
			return err
		}
	}

	workerCtx, stopWorker := context.WithCancel(ctx)
	var wg sync.WaitGroup
	defer func() {
		stopWorker()
		wg.Wait()
	}()
	worker := ingest.NewWorker(st, extract.NewPDFToText(), em, cfg)
	wg.Go(func() { worker.Run(workerCtx) })

	// Poll until every document is ready; a single failure stops the run.
	for {
		docs, err := st.ListDocuments(ctx)
		if err != nil {
			return err
		}
		ready := 0
		for _, d := range docs {
			switch d.Status {
			case "ready":
				ready++
			case "failed":
				return fmt.Errorf("ingest %s: %s", d.Filename, *d.Error)
			}
		}
		if ready == len(docs) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}
