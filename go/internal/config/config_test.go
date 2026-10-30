package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRepoConfig(t *testing.T) {
	cfg, err := Load("../../../config.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Seasons.First != 2014 {
		t.Errorf("Seasons.First = %d, want 2014", cfg.Seasons.First)
	}
	if cfg.Model.Current != nil {
		t.Errorf("Model.Current = %q, want nil", *cfg.Model.Current)
	}
	want := cfg.Seasons.Current - cfg.Seasons.First + 1
	if got := len(cfg.Seasons.All()); got != want {
		t.Errorf("len(Seasons.All()) = %d, want %d", got, want)
	}
}

// writeConfig writes body to a temp config.yaml and returns its path
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadRejectsUnknownKey(t *testing.T) {
	path := writeConfig(t, "db:\n path: x.duckdb\n pth: typo\n")
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}

func TestLoadRejectsFirstAfterCurrent(t *testing.T) {
	path := writeConfig(t, "seasons:\n first: 2026\n current: 2014\n")
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "seasons.first") {
		t.Fatalf("expected seasons.first error, got %v", err)
	}
}

func TestLoadRejectsBadElo(t *testing.T) {
        cases := map[string]string{
                "elo.k":                 "elo:\n initial: 1500\n k: 0\n",
                "elo.initial":           "elo:\n initial: -1\n k: 20\n",
                "elo.home_advantage":    "elo:\n initial: 1500\n k: 20\n home_advantage: -5\n",
                "elo.season_regression": "elo:\n initial: 1500\n k: 20\n season_regression: 1.5\n",
        }
        for want, body := range cases {
                t.Run(want, func(t *testing.T) {
                        _, err := Load(writeConfig(t, body))
                        if err == nil || !strings.Contains(err.Error(), want) {
                                t.Fatalf("expected %s error, got %v", want, err)
                        }
                })
        }
}