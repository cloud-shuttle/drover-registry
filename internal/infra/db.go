package infra

import (
	"context"
	"fmt"

	"github.com/cloud-shuttle/drover-libs/pkg/pgpool"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	pool *pgxpool.Pool
}

func NewDB(ctx context.Context, databaseURL string) (*DB, error) {
	config, err := PoolConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{pool: pool}, nil
}

// PoolConfig is the registry's only pool. Every query is a short statement or
// an explicit transaction with no session state (no LISTEN, session advisory
// locks, temp tables or session SET), so it is Pooled: on a PlanetScale host
// it goes through PgBouncer on 6432 with 3 connections. Other hosts keep their
// port. pool_max_conns in the URL overrides the size; DROVER_PG_POOLER=off
// disables the rewrite.
func PoolConfig(databaseURL string) (*pgxpool.Config, error) {
	return pgpool.Config(databaseURL, pgpool.Pooled)
}

func (db *DB) Close() {
	db.pool.Close()
}

func (db *DB) GetPool() *pgxpool.Pool {
	return db.pool
}

// Query / Exec helpers that respect a transaction in context if present (for future RLS / atomic ops)
func (db *DB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx, ok := txFromContext(ctx); ok {
		return tx.Query(ctx, sql, args...)
	}
	return db.pool.Query(ctx, sql, args...)
}

func (db *DB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx, ok := txFromContext(ctx); ok {
		return tx.QueryRow(ctx, sql, args...)
	}
	return db.pool.QueryRow(ctx, sql, args...)
}

func (db *DB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx, ok := txFromContext(ctx); ok {
		return tx.Exec(ctx, sql, args...)
	}
	return db.pool.Exec(ctx, sql, args...)
}

func (db *DB) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return db.pool.Begin(ctx)
}

// --- tiny tx context helpers (same pattern as muster) ---

type txKey struct{}

func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

func txFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}
