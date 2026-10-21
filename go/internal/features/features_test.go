package features

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kentongelis/match_predictor/go/internal/db"
)

// writeSQL creates a feature dir holding the given files (name -> contents).
func writeSQL(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestRunExecutesFilesInNameOrder(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.duckdb"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	// 02 reads 01's view, so this only works if 01 runs first. Map order is random, so writing them
	// from a map also checks that Run sorts instead of trusting directory order
	dir := writeSQL(t, map[string]string{
		"02_double.sql": `CREATE OR REPLACE TABLE out AS SELECT x * 2 AS y FROM base;`,
		"01_base.sql":   `CREATE OR REPLACE VIEW base AS SELECT 21 AS x;`,
	})

	// Twice, because the feature stage must be idempotent
	for i := range 2 {
		ran, err := Run(ctx, conn, dir)
		if err != nil {
			t.Fatalf("Run %d: %v", i+1, err)
		}
		if len(ran) != 2 || ran[0] != "01_base.sql" || ran[1] != "02_double.sql" {
			t.Fatalf("ran = %v, want [01_base.sql 02_double.sql]", ran)
		}
	}

	var y int
	if err := conn.QueryRowContext(ctx, `SELECT y FROM out`).Scan(&y); err != nil {
		t.Fatal(err)
	}
	if y != 42 {
		t.Errorf("y = %d, want 42", y)
	}
}

func TestRunRollsBackOnFailure(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.duckdb"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	dir := writeSQL(t, map[string]string{
		"01_ok.sql":     `CREATE TABLE made_by_01 AS SELECT 1 AS x;`,
		"02_broken.sql": `SELECT * FROM no_such_table;`,
	})

	if _, err := Run(ctx, conn, dir); err == nil {
		t.Fatal("Run succeeded, want an error from 02_broken.sql")
	}

	// 01 succeeded inside the transaction, but the rollback must undo it
	var n int
	err = conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'made_by_01'`).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("made_by_01 exists after a failed run; the transaction was not rolled back")
	}
}

func TestRunErrorsOnEmptyDir(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.duckdb"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	if _, err := Run(ctx, conn, t.TempDir()); err == nil {
		t.Fatal("Run on an empty dir succeeded, want an error")
	}
}
