package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/kentongelis/match_predictor/go/internal/config"
	"github.com/kentongelis/match_predictor/go/internal/db"
	"github.com/kentongelis/match_predictor/go/internal/ingest"
	"github.com/kentongelis/match_predictor/go/internal/clean"
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

	root.PersistentFlags().String("config", "../config.yaml", "path to config file")

	root.AddCommand(
		newMigrateCmd(),
		newIngestCmd(),
		newCleanCmd(),
		stubCmd("features", "Run sql/features/*.sql to rebuild the features table"),
		stubCmd("simulate", "Monte Carlo simulation of the remaining season"),
		stubCmd("run", "Full weekly pipeline: ingest, clean, features, predict, simulate"),
	)
	return root
}

func newMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Create or update the DuckDB schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("migrate: %w", err)
			}

			ctx := cmd.Context()
			conn, err := db.Open(ctx, cfg.DB.Path)
			if err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
			defer conn.Close() // one writer at a time: release the file when the stage ends

			schema := filepath.Join(cfg.Paths.SQLDir, "schema.sql")
			if err := db.ApplySchema(ctx, conn, schema); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "schema applied to %s\n", cfg.DB.Path)
			return nil
		},
	}
}

func newIngestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ingest",
		Short: "Fetch raw data into data/raw/",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("ingest: %w", err)
			}

			out := cmd.OutOrStdout()
			fd := ingest.NewFootbalData(cfg)
			paths, err := fd.Download(cmd.Context(), cfg.Seasons.All(), time.Now())
			for _, p := range paths {
				fmt.Fprintf(out, "wrote %s\n", p)
			}
			if err != nil {
				return fmt.Errorf("ingest: %w", err)
			}
			fmt.Fprintf(out, "footballdata: %d new file(s)\n", len(paths))

			ce := ingest.NewClubElo(cfg)
			paths, err = ce.Download(cmd.Context(), time.Now())
			for _, p := range paths {
				fmt.Fprintf(out, "wrote %s\n", p)
			}
			if err != nil {
				return fmt.Errorf("ingest: %w", err)
			}
			fmt.Fprintf(out, "clubelo: %d new file(s)\n", len(paths))
			return nil
		},
	}
}

func newCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use: "clean",
		Short: "Normalize raw data and load it into DuckDB",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("clean: %w", err)
			}
			teams, err := clean.LoadTeamMap(cfg.Paths.TeamNames)
			if err != nil {
				return fmt.Errorf("clean: %w", err)
			}

			ctx := cmd.Context()
			conn, err := db.Open(ctx, cfg.DB.Path)
			if err != nil {
				return fmt.Errorf("clean: %w", err)
			}
			defer conn.Close() // one writer at a time

			schema := filepath.Join(cfg.Paths.SQLDir, "schema.sql")
			if err := db.ApplySchema(ctx, conn, schema); err != nil {
				return fmt.Errorf("clean: %w", err)
			}

			dir, n, err := clean.LoadFootballData(ctx, conn, cfg.Paths.RawDir, cfg.Seasons.All(), teams)
			if err != nil {
				return fmt.Errorf("clean matches: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "footballdata: loaded %d matches from %s\n", n, dir)
			// Elo before the first season's opening day is never needed, so older history isn't loaded
			since := time.Date(cfg.Seasons.First, time.January, 1, 0, 0, 0, 0, time.UTC)
			n, err = clean.LoadClubElo(ctx, conn, cfg.Paths.RawDir, cfg.Sources.ClubElo.Clubs, teams, since)
			if err != nil {
				return fmt.Errorf("clean elo: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "clubelo: loaded %d ratings\n", n)
			return nil
		},
	}
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
