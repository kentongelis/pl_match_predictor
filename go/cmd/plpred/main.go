package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var errNotImplemented = errors.New("not implemented yet")

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "plpred",
		Short:         "Premier League match predictor pipeline",
		SilenceUsage:  true, // to not dump text on every runtime error
		SilenceErrors: true, // main prints the error once
	}

	root.PersistentFlags().String("config", "config.yaml", "path to config file")

	root.AddCommand(
		stubCmd("migrate", "Create or update the DuckDB schema"),
		stubCmd("ingest", "Fetch raw data into data/raw/"),
		stubCmd("clean", "Normalize raw data and load matches, elo, fixtures"),
		stubCmd("features", "Run sql/features/*.sql to rebuild the features table"),
		stubCmd("simulate", "Monte Carlo simulation of the remaining season"),
		stubCmd("run", "Full weekly pipeline: ingest, clean, features, predict, simulate"),
	)
	return root
}

// stubCmd builds a placeholder subcommand. Each one gets replaced as its features get built
func stubCmd(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("%s: %w", use, errNotImplemented)
		},
	}
}
