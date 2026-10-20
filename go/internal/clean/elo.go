package clean

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kentongelis/match_predictor/go/internal/ingest"
)

// EloRating is one row of the elo table: the club's rating from DateFrom through DateTo
type EloRating struct {
	TeamID string
	DateFrom time.Time
	DateTo time.Time
	Elo float64
}

// ParseClubElo reads one club's ClubElo history CSV. Rows that ended before since are dropped, because the first
// season only needs the rating in force on its opening day
func ParseClubElo(r io.Reader, teams *TeamMap, since time.Time) ([]EloRating, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = 7 // Rank,Club, Country, Level, Elo, From, To

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	if got := strings.Join(header, ","); got != ingest.ClubEloHeader {
		return nil, fmt.Errorf("header is %q, want %q", got, ingest.ClubEloHeader)
	}

	var ratings []EloRating
	var teamID string
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		line, _ := cr.FieldPos(0)

		team, err := teams.Lookup(rec[1])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if teamID == "" {
			teamID = team.ID
		} else if team.ID != teamID {
			return nil, fmt.Errorf("line %d: file mixes %s and %s", line, teamID, team.ID)
		}

		from, err := time.Parse("2006-01-02", rec[5])
		if err != nil {
			return nil, fmt.Errorf("line %d: From: %w", line, err)
		}
		to, err := time.Parse("2006-01-02", rec[6])
		if err != nil {
			return nil, fmt.Errorf("line %d: To: %w", line, err)
		}
		if to.Before(since) {
			continue
		}
		if to.Before(from) {
			return nil, fmt.Errorf("line %d: To %s is before From %s", line, rec[6], rec[5])
		}
		elo, err := strconv.ParseFloat(rec[4], 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: Elo: %w", line, err)
		}

		ratings = append(ratings, EloRating{TeamID: teamID, DateFrom: from, DateTo: to, Elo: elo})
	}
	if len(ratings) == 0 {
		return nil, fmt.Errorf("no ratings on or after %s", since.Format("2006-01-02"))
	}
	return ratings, nil 
}

// LoadClubElo parse the newest download of every club and upserts all of them into elo in one transaction. Like
// LoadFootballData, everything is parsed before the database is touched. It returns how many rows it loaded
func LoadClubElo(ctx context.Context, conn *sql.DB, rawDir string, clubs []string, teams *TeamMap, since time.Time) (int, error) {
	root := filepath.Join(rawDir, "clubelo")

	var ratings []EloRating
	var errs []error
	for _, club := range clubs {
		path, err := latestFile(root, club+".csv")
		if err != nil {
			errs = append(errs, err)
			continue
		}
		rs, err := parseClubEloFile(path, teams, since)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}
		ratings = append(ratings, rs...)
	}
	if len(errs) > 0 {
		return 0, errors.Join(errs...)
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() // no-op after Commit

	if err := upsertElo(ctx, tx, ratings); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return len(ratings), nil
}

func parseClubEloFile(path string, teams *TeamMap, since time.Time) ([]EloRating, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseClubElo(f, teams, since)
}

// latestFile returns the newest copy of name across dir's YYYY-MM-DD folders. Unlike football-data, the newest
// ClubElo folder can be missing clubs: Download stops at the first failure, so an outage leaves it partial or empty
func latestFile(dir, name string) (string, error) {
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
		if _, err := os.Stat(filepath.Join(dir, e.Name(), name)); err != nil {
			continue // this club wasn't downloaded that day
		}
		if e.Name() > latest {
			latest = e.Name()
		}
	}
	if latest == "" {
		return "", fmt.Errorf("no download of %s in %s (run plpred ingest first)", name, dir)
	}
	return filepath.Join(dir, latest, name), nil
}

func upsertElo(ctx context.Context, tx *sql.Tx, ratings []EloRating) error {
	stmt, err := tx.PrepareContext(ctx, `
	            INSERT INTO elo (team_id, date_from, date_to, elo) VALUES (?, ?, ?, ?)
                ON CONFLICT (team_id, date_from) DO UPDATE SET
                date_to = excluded.date_to,
                elo     = excluded.elo`)
	if err != nil {
		return fmt.Errorf("prepare elo upsert: %w", err)
	}
	defer stmt.Close()

	for _, r := range ratings {
		if _, err := stmt.ExecContext(ctx, r.TeamID, r.DateFrom, r.DateTo, r.Elo); err != nil {
			return fmt.Errorf("upsert elo %s %s: %w", r.TeamID, r.DateFrom.Format("2006-01-02"), err)
		}
	}
	return nil
}