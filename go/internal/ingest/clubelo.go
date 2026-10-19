package ingest

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kentongelis/match_predictor/go/internal/config"
)

// clubEloHeader is the first line of every ClubElo history CSV
const clubEloHeader = "Rank,Club,Country,Level,Elo,From,To"

// ClubElo downloads each club's full Elo history from api.clubelo.com
type ClubElo struct {
	fetcher *Fetcher
	urlTemplate string // contains {club}, e.g. "http://api.clubelo.com/{club}"
	clubs []string // ClubElo URL names, e.g. "ManCity"
	outDir	string // <raw_dir>/clubelo
}

func NewClubElo(cfg *config.Config) *ClubElo {
	return &ClubElo{
		fetcher: newFetcher(cfg.HTTP),
		urlTemplate: cfg.Sources.ClubElo.URLTemplate,
		clubs: cfg.Sources.ClubElo.Clubs,
		outDir: filepath.Join(cfg.Paths.RawDir, "clubelo"),
	}
}

// Download fetches every club into outDir/<day>/<club>.csv and returns the paths it wrote. 
// Similar to FootballData, files already downloaded that day are skipped.
func (ce *ClubElo) Download(ctx context.Context, day time.Time) ([]string, error) {
	dir := filepath.Join(ce.outDir, day.Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}

	var written []string
	for _, club := range ce.clubs {
		path := filepath.Join(dir, club+".csv")

		if _, err := os.Stat(path); err == nil {
			continue // already downloaded today
		}

		url := strings.ReplaceAll(ce.urlTemplate, "{club}", club)
		body, err := ce.fetcher.Get(ctx, url)
		if err != nil {
			return written, fmt.Errorf("clubelo %s: %w", club, err)
		}
		if err := checkClubElo(body); err != nil {
			return written, fmt.Errorf("clubelo %s: %w", club, err)
		}
		if err := writeNew(path, body); err != nil {
			return written, fmt.Errorf("clubelo %s: %w", club, err)
		}
		written = append(written, path)
	}
	return written, nil
}

// checkClubElo rejects a body that isn't a ratings CSV. ClubElo answers a misspelled club with 200 and just the
// header, so without this a typo in config.yaml would silently save an empty file
func checkClubElo(body []byte) error {
	header, rest, _ := bytes.Cut(body, []byte("\n"))
	if got := strings.TrimSpace(string(header)); got != clubEloHeader {
		return fmt.Errorf("unexpected header %q, want %q", got, clubEloHeader)
	}
	if len(bytes.TrimSpace(rest)) == 0 {
		return fmt.Errorf("no ratings returned; is the club name right in config.yaml?")
	}
	return nil
}