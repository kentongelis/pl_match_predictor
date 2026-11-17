package clean

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/kentongelis/match_predictor/go/internal/config"
)

// EloRating is one row of the elo table: the club's rating from DateFrom through DateTo
type EloRating struct {
	TeamID   string
	DateFrom time.Time
	DateTo   time.Time
	Elo      float64
}

// LoadElo replays every match in the matches table through ComputeElo and replaces the elo table with the result
// The read, delete and insert share one transaction, so the elo table always matches the matches it was computed from
// It returns how many ratings it wrote
func LoadElo(ctx context.Context, conn *sql.DB, p config.Elo) (int, error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() // no-op after Commit

	matches, err := readMatches(ctx, tx)
	if err != nil {
		return 0, err
	}
	ratings, err := ComputeElo(matches, p)
	if err != nil {
		return 0, fmt.Errorf("compute elo: %w", err)
	}

	// Every rating depends on every earlier match, so a fix to one old result changes all the rows after it.
	// Rebuilding the table is simpler and safer than upserting
	if _, err := tx.ExecContext(ctx, `DELETE FROM elo`); err != nil {
		return 0, fmt.Errorf("clear elo: %w", err)
	}
	if err := insertElo(ctx, tx, ratings); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return len(ratings), nil
}

// readMatches loads the column ComputeElo needs. ComputeElo sorrts its input itself, so no ORDER BY
func readMatches(ctx context.Context, tx *sql.Tx) ([]Match, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT match_id, season, date, home_team_id, away_team_id, home_goals, away_goals FROM matches`)
	if err != nil {
		return nil, fmt.Errorf("query matches: %w", err)
	}
	defer rows.Close()

	var matches []Match
	for rows.Next() {
		var m Match
		if err := rows.Scan(&m.ID, &m.Season, &m.Date, &m.HomeTeamID, &m.AwayTeamID,
			&m.HomeGoals, &m.AwayGoals); err != nil {
			return nil, fmt.Errorf("scan match: %w", err)	
			}
			matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read matches: %w", err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("matches table is empty (load football-date first)")
	}
	return matches, nil
}

func insertElo(ctx context.Context, tx *sql.Tx, ratings []EloRating) error {
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO elo (team_id, date_from, date_to, elo) VALUES (?, ?, ?, ?) `)
	if err != nil {
		return fmt.Errorf("prepare elo insert: %w", err)
	}
	defer stmt.Close()

	for _, r := range ratings {
		if _, err := stmt.ExecContext(ctx, r.TeamID, r.DateFrom, r.DateTo, r.Elo); err != nil {
			return fmt.Errorf("insert elo %s %s: %w", r.TeamID, r.DateFrom.Format("2006-01-02"), err)
		}
	}
	return nil
}
