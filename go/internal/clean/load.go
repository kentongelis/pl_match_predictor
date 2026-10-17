package clean

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kentongelis/match_predictor/go/internal/ingest"

)

// LoadFootballData parses every season's CSV from the newest football-data download and upserts teams and matches
// in one go. Every file is parsed before the database is touched, so a bad row means nothing is written. It returns
// the directory it read and how many matches it loaded
func LoadFootballData(ctx context.Context, conn *sql.DB, rawDir string, seasons []int, teams *TeamMap) (string, int, error) {
	dir, err := latestDir(filepath.Join(rawDir, "footballdata"))
	if err != nil {
		return "", 0, err
	}

	var matches []Match
	var errs []error
	for _, season := range seasons {
		path := filepath.Join(dir, ingest.SeasonCode(season)+".csv")
		ms, err := parseSeasonFile(path, season, teams)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}
		matches = append(matches, ms...)
	}
	if len(errs) > 0 {
		return dir, 0, errors.Join(errs...)
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return dir, 0, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() // no-op after Commit

	if err := upsertTeams(ctx, tx, teams.Teams()); err != nil {
		return dir, 0, err
	}
	if err := upsertMatches(ctx, tx, matches); err != nil {
		return dir, 0, err
	}
	if err := tx.Commit(); err != nil {
		return dir, 0, fmt.Errorf("commit: %w", err)
	}
	return dir, len(matches), nil
}

func parseSeasonFile(path string, season int, teams *TeamMap) ([]Match, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseFootballData(f, season, teams)
}

// latestDir returns the newest YYYY-MM-DD subdirectory of dir. Ingest writes one folder per day and never changes 
// old ones, so the newest folder holds the most recent copy of every season
func latestDir(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", dir, err)
	}
	var latest string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := time.Parse("2006-01-02", e.Name()); err != nil {
			continue // not a download folder
		}
		if e.Name() > latest { // ISO dates sort correctly as strings
			latest = e.Name()	
		}
	} 
	if latest == "" {
		return "", fmt.Errorf("no dated download folders in %s (run plpred ingest first)", dir)
	}
	return filepath.Join(dir, latest), nil
}

func upsertTeams(ctx context.Context, tx *sql.Tx, teams []Team) error {
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO teams (team_id, name) VALUES (?, ?)
		ON CONFLICT (team_id) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("prepare teams upsert: %w", err)
	}
	defer stmt.Close()

	for _, t := range teams {
		if _, err := stmt.ExecContext(ctx, t.ID, t.Name); err != nil {
			return fmt.Errorf("upsert team %s: %w", t.ID, err)
		}
	}
	return nil
}

func upsertMatches(ctx context.Context, tx *sql.Tx, matches []Match) error {
	stmt, err := tx.PrepareContext(ctx, `
	                INSERT INTO matches (
                        match_id, season, date, home_team_id, away_team_id,
                        home_goals, away_goals, result,
                        home_shots, away_shots, home_sot, away_sot,
                        home_xg, away_xg, odds_h, odds_d, odds_a
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                ON CONFLICT (match_id) DO UPDATE SET
                        home_goals = excluded.home_goals,
                        away_goals = excluded.away_goals,
                        result     = excluded.result,
                        home_shots = excluded.home_shots,
                        away_shots = excluded.away_shots,
                        home_sot   = excluded.home_sot,
                        away_sot   = excluded.away_sot,
                        home_xg    = excluded.home_xg,
                        away_xg    = excluded.away_xg,
                        odds_h     = excluded.odds_h,
                        odds_d     = excluded.odds_d,
                        odds_a     = excluded.odds_a`)
	if err != nil {
		return fmt.Errorf("prepare matches upsert: %w", err)
	}
	defer stmt.Close()

	for _, m := range matches {
		_, err := stmt.ExecContext(ctx,
			m.ID, m.Season, m.Date, m.HomeTeamID, m.AwayTeamID,
			m.HomeGoals, m.AwayGoals, m.Result,
			m.HomeShots, m.AwayShots, m.HomeSOT, m.AwaySOT,
			m.HomeXG, m.AwayXG, m.OddsH, m.OddsD, m.OddsA,
        )
		if err != nil {
			return fmt.Errorf("upsert match %s: %w", m.ID, err)
		}
	}
	return nil
}