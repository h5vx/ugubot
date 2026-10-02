// Package pg holds Postgres helpers shared by services that own a schema.
package pg

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// Connect opens a pool and waits until the database answers.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}

	for attempt := 1; ; attempt++ {
		err = pool.Ping(ctx)
		if err == nil {
			return pool, nil
		}
		if attempt == 30 {
			pool.Close()
			return nil, fmt.Errorf("database is unreachable: %w", err)
		}
		slog.Warn("waiting for database", "err", err)
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// Migrate applies migrations from fsys. Each service keeps its own goose
// version table, so schemas evolve independently.
func Migrate(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS, versionTable string) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys, goose.WithTableName(versionTable))
	if err != nil {
		return err
	}

	results, err := provider.Up(ctx)
	for _, r := range results {
		slog.Info("migration applied", "table", versionTable, "source", r.Source.Path, "duration", r.Duration)
	}
	return err
}

// MustSub returns a subdirectory of an embedded filesystem.
func MustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
