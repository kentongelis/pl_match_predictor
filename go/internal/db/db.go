// Package db opens the DuckDB database and applies the schema.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/marcboeker/go-duckdb/v2" // registers the "duckdb: driver
)

// Open opens the DuckDB file at path, creating it (and its directory) if needed.
// The caller must close it when its pipeline stage ends.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	conn, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, fmt.Errorf("open duckdb %s: %w", path, err)
	}

	//sql.Open is lazy; Ping forces a real connection so errors surface here.
	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("connect duckdb %s: %w", path, err)
	}
	return conn, nil
}

// ApplySchema runs schema.sql against conn. Every statement in the file is idempotent
// (CREATE ... IF NOT EXISTS), os this is safe to run on every opne
func ApplySchema(ctx context.Context, conn *sql.DB, schemaPath string) error {
	ddl, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}

	if _, err := conn.ExecContext(ctx, string(ddl)); err != nil {
		return fmt.Errorf("apply schema %s: %w", schemaPath, err)
	}
	return nil
}