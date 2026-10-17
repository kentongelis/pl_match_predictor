package ingest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kentongelis/match_predictor/go/internal/config"
)

// FootballData downloads season results CSVs from footbal-data.co.uk
type FootballData struct {
	fetcher *Fetcher
	urlTemplate string // contains {code}, e.g. ".../mmz4281/{code}/E0.csv"
	outDir string // <raw_dir>/footballdata
}

func NewFootbalData(cfg *config.Config) *FootballData {
	return &FootballData{
		fetcher: 	newFetcher(cfg.HTTP),
		urlTemplate: cfg.Sources.FootballData.URLTemplate,
		outDir: filepath.Join(cfg.Paths.RawDir, "footballdata"),
	}
}

// Download fetches every season in seasons into outDir/<day>/<code?.csv and returns the paths it wrote.
// Files that already exist are left alone, so running it twice on the same day is safe and doesn't re-download
func (fd *FootballData) Download(ctx context.Context, seasons []int, day time.Time) ([]string, error) {
	dir := filepath.Join(fd.outDir, day.Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}

	var written []string
	for _, season := range seasons {
		code := SeasonCode(season)
		path := filepath.Join(dir, code+".csv")

		if _, err := os.Stat(path); err == nil {
			continue // already downloaded today
		}

		url := strings.ReplaceAll(fd.urlTemplate, "{code}", code)
		body, err := fd.fetcher.Get(ctx, url)
		if err != nil {
			return written, fmt.Errorf("footballdata season %d: %w", season, err)
		}
		if err := writeNew(path, body); err != nil {
			return written, fmt.Errorf("footbaldata season %d: %w", season, err)
		}
		written = append(written, path)
	}
	return written, nil
}

// SeasonCode turns a start year into football-data's code: 2014 -> "1415"
func SeasonCode(start int) string {
	return fmt.Sprintf("%02d%02d", start%100, (start+1)%100)
}

// writeNew writes data to path but refuses to replace an existing file. It writes to a temp file first, then hard-links
// it into place. Link fails if path exists, and a crash mid-write leaves only a temp file, never a half-written CSV.
func writeNew(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create tmep: %w", err)
	}
	defer os.Remove(tmp.Name()) // always clean up the temp name

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", tmp.Name(), err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmp.Name(), err)
	}

	if err := os.Link(tmp.Name(), path); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%s already exists, refusing to overwrite raw data", path)
		}
		return fmt.Errorf("link %s: %w", path, err)
	}
	return nil
}