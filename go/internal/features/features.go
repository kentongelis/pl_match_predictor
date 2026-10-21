// Package features rebuilds the features tbale by running sql/features/*.sql in order
package features

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Run executes every .sql file in the features dir, sorted by name, inside on transaction. Later files read what
// earlier ones create (02 reads 01's view), so the numeric prefix is the execution order. If any file fails, the 
// whole run rolls back and the previous features table stays as it was. It returns the file names it ran
func Run(ctx context.Context, conn *sql.DB, dir string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", dir, err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no .sql files in %s", dir)
	}
	sort.Strings(paths) // the order is the contract

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() // no-op after Commit

	ran := make([]string, 0, len(paths))
	for _, p := range paths {
		q, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		name := filepath.Base(p)
		if _, err := tx.ExecContext(ctx, string(q)); err != nil {
			return nil, fmt.Errorf("run %s: %w", name, err)
		}
		ran = append(ran, name)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return ran, nil
}