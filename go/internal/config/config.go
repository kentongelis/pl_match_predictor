// Package config loads config.yaml, the settings file shared by Go and Python
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config mirrors config.yaml. Every key in the file must have a field here;
// unknown keys are rejected so typos fail loudly
type Config struct {
	DB         DB         `yaml:"db"`
	Paths      Paths      `yaml:"paths"`
	Seasons    Seasons    `yaml:"seasons"`
	Sources    Sources    `yaml:"sources"`
	HTTP       HTTP       `yaml:"http"`
	Features   Features   `yaml:"features"`
	Simulation Simulation `yaml:"simulation"`
	Model      Model      `yaml:"model"`
}

type DB struct {
	Path string `yaml:"path"`
}

type Paths struct {
	RawDir    string `yaml:"raw_dir"`
	TeamNames string `yaml:"team_names"`
	SQLDir    string `yaml:"sql_dir"`
	ModelsDir string `yaml:"models_dir"`
}

// Season are start years: 2014 means the 2014-15 season
type Seasons struct {
	First   int `yaml:"first"`
	Current int `yaml:"current"`
}

type Sources struct {
	FootballData Source `yaml:"footballdata"`
	ClubElo      ClubEloSource `yaml:"clubelo"`
}

type Source struct {
	URLTemplate string `yaml:"url_template"`
}

// ClubEloSource is fetcher per club, so it also lists which clubs to fetch
type ClubEloSource struct {
	URLTemplate string `yaml:"url_template"`
	Clubs []string `yaml:"clubs"` // ClubElo URL names, e.g. "ManCity"
}

type HTTP struct {
	TimeoutSeconds   int `yaml:"timeout_seconds"`
	MaxRetries       int `yaml:"max_retries"`
	BackoffInitialMS int `yaml:"backoff_initial_ms"`
	RateLimitMS      int `yaml:"rate_limit_ms"`
}

type Features struct {
	RollingWindows []int `yaml:"rolling_windows"`
}

type Simulation struct {
	Iterations int   `yaml:"iterations"`
	Workers    int   `yaml:"workers"`
	Seed       int64 `yaml:"seed"`
}

type Model struct {
	Current                *string `yaml:"current"` // nil until a model is promoted
	WalkForwardTestSeasons int     `yaml:"walk_forward_test_seasons"`
}

// Load reads and validates the config file at path. Relative paths in the file are resolved
// against the file's own directory, so commands work no matter which directly they're run from
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // unkown keys are an err, not silently ignored

	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}

	cfg.resolvePaths(filepath.Dir(path))
	return &cfg, nil
}

// All returns every season start year from First through Current, inclusive.
func (s Seasons) All() []int {
	years := make([]int, 0, s.Current-s.First+1)
	for y := s.First; y <= s.Current; y++ {
		years = append(years, y)
	}
	return years
}

// validate collects every problem at once instead of stopping at the first
func (c *Config) validate() error {
	var errs []error

	if c.DB.Path == "" {
		errs = append(errs, errors.New("db.path is required"))
	}
	if c.Seasons.First > c.Seasons.Current {
		errs = append(errs, fmt.Errorf("seasons.first (%d) is after seasons.current (%d)",
			c.Seasons.First, c.Seasons.Current))
	}
	if c.HTTP.TimeoutSeconds <= 0 {
		errs = append(errs, errors.New("http.timeout_seconds must be positive"))
	}
	if c.Simulation.Iterations <= 0 {
		errs = append(errs, errors.New("simulation.iterations must be positive"))
	}
	if c.Simulation.Workers <= 0 {
		errs = append(errs, errors.New("simulation.workers must be positive"))
	}
	if len(c.Features.RollingWindows) == 0 {
		errs = append(errs, errors.New("features.rolling_windows must not be empty"))
	}
	for _, w := range c.Features.RollingWindows {
		if w <= 0 {
			errs = append(errs, fmt.Errorf("features.rolling_windows: %d is not positive", w))
		}
	}
	if len(c.Sources.ClubElo.Clubs) == 0 {
		errs = append(errs, errors.New("sources.clubelo.clubs must not be empty"))
	}

	return errors.Join(errs...) // nil when errs is empty
}

// resolvePaths makes relative paths relative to the config file's directory
func (c *Config) resolvePaths(baseDir string) {
	for _, p := range []*string{
		&c.DB.Path,
		&c.Paths.RawDir,
		&c.Paths.TeamNames,
		&c.Paths.SQLDir,
		&c.Paths.ModelsDir,
	} {
		if !filepath.IsAbs(*p) {
			*p = filepath.Join(baseDir, *p)
		}
	}
}
