package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestApplySchemaCreatesTablesIdempotently(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(ctx, filepath.Join(t.TempDir(), "test.duckdb"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	// Applying twice must not fail: the schema runs on every open.
	for i := range 2 {
		if err := ApplySchema(ctx, conn, "../../../sql/schema.sql"); err != nil {
			t.Fatalf("ApplySchema run %d: %v", i+1, err)
		}
	}

	want := []string{"elo", "fixtures", "matches", "model_runs", "predictions", "simulations", "teams"}

	rows, err := conn.QueryContext(ctx,
		`SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'main' ORDER BY table_name`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	if len(got) != len(want) {
		t.Fatalf("tables = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("table %d = %q, want %q", i, got[i], want[i])
		}
	}
}