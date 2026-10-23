package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
	"github.com/pressly/goose/v3"
)

// The SQL migrations are compiled into the binary, so it can set up its own
// schema without the files being present at runtime.
//
//go:embed migrations/*.sql
var migrations embed.FS

// Store is the Postgres-backed persistence layer. It is safe for concurrent
// use: queries share a connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// Open applies pending migrations and connects to the database at url.
func Open(ctx context.Context, url string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("store: parse url: %w", err)
	}
	if err := migrate(ctx, cfg.ConnConfig); err != nil {
		return nil, err
	}

	// The vector type is defined by the pgvector extension, not by Postgres
	// itself, so every new connection has to learn how to encode it. This
	// runs after the migrations, because they create the extension.
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvec.RegisterTypes(ctx, conn)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

// migrate applies every migration that has not run yet; goose records the
// applied versions in its own table. goose works with database/sql, so it
// gets a separate short-lived connection instead of the pgx pool.
func migrate(ctx context.Context, connCfg *pgx.ConnConfig) error {
	files, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("store: migrations: %w", err)
	}

	db := stdlib.OpenDB(*connCfg)
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, files)
	if err != nil {
		return fmt.Errorf("store: migrations: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	for _, r := range results {
		slog.Info("migration applied", "version", r.Source.Version, "file", r.Source.Path)
	}
	return nil
}
