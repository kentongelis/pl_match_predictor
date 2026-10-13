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
	DB			DB			`yaml:"db"`
	Paths		Paths		`yaml:"paths"`
	Seasons 	Seasons		`yaml:"seasons"`
	Sources 	Sources		`yaml:"sources"`
	HTTP		HTTP		`yaml:"http"`
	Features	Features	`yaml:"featuers"`
	Simulation	simulation	`yaml:"simulation"`
	Model		Model		`yaml:"mode"`
}

type DB struct {
	Path string `yaml:"path"`
}

type Paths struct {
	RawDir	  	string	`yaml:"raw_dir"`
	TeamNames 	string	`yaml:"team_names"`
	SQLDir	  	string 	`yaml:"sql_dir"`
	ModelsDir	string	`yaml:"models_dir"`
}

// Season are start years: 2014 means the 2014-15 season
type Seasons struct {
	First	int	`yaml:"first"`
	Current	int `yaml:"current"`
}

type Sources struct {
	FootballData	Source	`yaml:"footballdata"`
	ClubElo			Source  `yaml:"clubelo"`
}

type Source struct {
	URLTemplate string `yaml:"url_template"`
}

type HTTP struct {
	TimeoutSeconds		int `yaml:"timeout_seconds"`
	MaxRetries			int `yaml:"max_retries"`
	BackoffInitialMS	int `yaml:"backoff_initial_ms"`
	RateLimitMS			int `yaml:"rate_limit_ms"`
}

type Features struct {
	RollingWindows []int `yaml:"rolling_windows"`
}

type Simulation struct {
	Iterations int 		`yaml:"iterations"`
	Workers	   int 		`yaml:"workers"`
	Seed 	   int64 	`yaml:"seed"`
}

type Model struct {
	Current					*string `yaml:"current"` // nil until a model is promoted
	WalkForwardTestSeasons	int 	`yaml:"walk_forward_test_seasons"`
}







