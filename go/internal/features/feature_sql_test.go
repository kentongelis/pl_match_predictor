package features

import (
	"context"
	"database/sql"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/kentongelis/match_predictor/go/internal/db"
)

// TestFeatureSQL runs the real sql/features files against the hand-built season in testdata/mini_season.sql
// and checks values worked out by hand. It's the guard on window frames: if a frame ever ends at CURRENT ROW
// instead of 1 PRECEDING, the numbers below change and this fails.
func TestFeatureSQL(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.duckdb"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	if err := db.ApplySchema(ctx, conn, "../../../sql/schema.sql"); err != nil {
		t.Fatalf("ApplySchema: %v", err)
	}
	seed, err := os.ReadFile("testdata/mini_season.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, string(seed)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := Run(ctx, conn, "../../../sql/features"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Each case reads one column for one team in one match. want nil means the value must be NULL
	cases := []struct {
		name  string
		query string
		args  []any
		want  *float64
	}{
		// First match: nothing before it, so every window is empty
		{"first match pts_l5 is NULL", formCol("pts_l5"), args("2020-21_2020-08-01_ars_b", "ars"), nil},
		{"first match rest_days is NULL", formCol("rest_days"), args("2020-21_2020-08-01_ars_b", "ars"), nil},

		// Match 6: last 5 are matches 1-5. pts (3+1+0+3+0)/5, goals for (2+1+0+3+0)/5
		{"match 6 pts_l5", formCol("pts_l5"), args("2020-21_2020-09-05_d_ars", "ars"), f(1.4)},
		{"match 6 gf_l5", formCol("gf_l5"), args("2020-21_2020-09-05_d_ars", "ars"), f(1.2)},
		// Match 6 is away, so venue form uses only the earlier away matches 2 and 4: (1+3)/2
		{"match 6 venue_pts_l5", formCol("venue_pts_l5"), args("2020-21_2020-09-05_d_ars", "ars"), f(2.0)},

		// Match 7: the window has slid to matches 2-6, so match 1's 3 points drop out: (1+0+3+0+1)/5
		{"match 7 pts_l5", formCol("pts_l5"), args("2020-21_2020-09-12_ars_b", "ars"), f(1.0)},
		// Match 7's pts_l10 sees all six earlier matches: (3+1+0+3+0+1)/6
		{"match 7 pts_l10", formCol("pts_l10"), args("2020-21_2020-09-12_ars_b", "ars"), f(8.0 / 6)},

		// 2020-08-12 minus 2020-08-22
		{"match 4 rest_days", formCol("rest_days"), args("2020-21_2020-08-22_b_ars", "ars"), f(10)},

		// features (wide): match 7 is ars at home to b. b lost both earlier games (0-2, 1-3), so 0.0
		{"features home_pts_l5", featCol("home_pts_l5"), args("2020-21_2020-09-12_ars_b"), f(1.0)},
		{"features away_pts_l5", featCol("away_pts_l5"), args("2020-21_2020-09-12_ars_b"), f(0.0)},
		{"features diff_pts_l5", featCol("diff_pts_l5"), args("2020-21_2020-09-12_ars_b"), f(1.0)},

		// Upcoming fixture: c drew 1-1 at home and won 2-0 away, so (1+3)/2. It gets features like any match
		{"fixture home_pts_l5", featCol("home_pts_l5"), args("2020-21_2020-09-19_c_ars"), f(2.0)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got sql.NullFloat64
			if err := conn.QueryRowContext(ctx, tc.query, tc.args...).Scan(&got); err != nil {
				t.Fatalf("query: %v", err)
			}
			switch {
			case tc.want == nil && got.Valid:
				t.Errorf("got %v, want NULL", got.Float64)
			case tc.want != nil && !got.Valid:
				t.Errorf("got NULL, want %v", *tc.want)
			case tc.want != nil && math.Abs(got.Float64-*tc.want) > 1e-9:
				t.Errorf("got %v, want %v", got.Float64, *tc.want)
			}
		})
	}

	// Labels: played matches carry their result, the fixture has none and is marked unplayed
	var result sql.NullString
	var played bool
	err = conn.QueryRowContext(ctx,
		`SELECT result, is_played FROM features WHERE match_id = '2020-21_2020-09-19_c_ars'`).Scan(&result, &played)
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid || played {
		t.Errorf("fixture: result = %v, is_played = %v; want NULL, false", result, played)
	}
}

// formCol selects one column of team_form for a (match_id, team_id). DOUBLE cast so rest_days scans as a float
func formCol(col string) string {
	return `SELECT CAST(` + col + ` AS DOUBLE) FROM team_form WHERE match_id = ? AND team_id = ?`
}

// featCol selects one column of the wide features table for a match_id
func featCol(col string) string {
	return `SELECT CAST(` + col + ` AS DOUBLE) FROM features WHERE match_id = ?`
}

func args(a ...any) []any { return a }

func f(v float64) *float64 { return &v }
