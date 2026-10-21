# Premier League Match Predictor

Predicts **home / draw / away probabilities** for Premier League fixtures and simulates how the rest of the season plays out.

The goal is calibrated probabilities, not just a "pick" for each match. Models are judged by **Ranked Probability Score (RPS)** with walk-forward validation by season, and compared against the bookmakers' implied probabilities.

> **Status:** work in progress. The data pipeline (ingest → clean → features) works for match data. Modeling, prediction and simulation are next. See [Roadmap](#roadmap).

---

## How it works

The project is split across three languages. A single DuckDB file is the only interface between them.

| Layer | Responsibility |
|---|---|
| **Go** (`go/`) | Ingestion, cleaning, feature-build orchestration, Monte Carlo simulation, CLI |
| **SQL** (`sql/`) | The database schema and **all** feature logic |
| **Python** (`python/`) | Model training, evaluation, batch prediction, API |
| **DuckDB** (`data/pl.duckdb`) | Shared storage. Go and Python never call each other directly |

```
 football-data.co.uk ─┐
 Club Elo ────────────┤ ingest ──► data/raw/ ──► clean ──► DuckDB: teams, matches, elo
                      │                                        │
                      │                     sql/features/*.sql ▼
                      │                                  DuckDB: features
                      │                                        │
                      │            Python: train ──► models/   │
                      │            Python: predict ──► DuckDB: predictions
                      │                                        │
                      └──────────── Go: simulate ──► DuckDB: simulations
```

### Design rules

- **No data leakage.** Every feature for a match uses only data from *strictly before* the match date. Rolling windows end at `1 PRECEDING`.
- **Feature logic lives only in SQL.** Neither Go nor Python reimplements features.
- **Raw data is immutable.** `ingest` writes dated snapshots to `data/raw/`, and `clean` reads them into DuckDB with idempotent upserts.
- **One writer at a time.** Pipeline stages run in sequence, and the API opens the database read-only.
- **Time-based evaluation only.** Validation is walk-forward by season, never a random split.

---

## Current state

### Done

- **Config:** one `config.yaml` shared by Go and Python. It holds seasons, sources, HTTP settings, windows and simulation settings.
- **Schema** (`sql/schema.sql`): `teams`, `matches`, `fixtures`, `elo`, `predictions`, `simulations`, `model_runs`.
- **Go CLI (`plpred`)** built with Cobra:
  - `migrate` applies the schema.
  - `ingest` downloads football-data.co.uk season CSVs (2014-15 onward) and Club Elo rating histories. The HTTP client is context-aware, with timeouts, retries with backoff and rate limiting.
  - `clean` maps team names through `data/mappings/team_names.csv`, parses the data and upserts it into `teams`, `matches` and `elo`. That's roughly 4,600 matches with 20 teams per season.
  - `features` runs `sql/features/*.sql` in order to build the `features` table.
- **SQL features:**
  - `01_team_match.sql` gives one row per team per match.
  - `02_rolling_form.sql` computes pre-match rolling form over the last 5 and 10 matches: points, goals, shots and shots on target, for and against. It also computes venue-specific form and rest days.
  - `04_features.sql` builds one row per match with home and away features side by side.
- **Tests:** Go unit tests cover config, DB migration, the fetcher, team mapping, the parsers (on saved sample files in `testdata/`, not live HTTP), the loaders and features. A hand-built mini season checks the rolling SQL features against known values.
- **Python:** a package skeleton (`pyproject.toml`, `plpred/`).

### Known gaps

- **Club Elo is unavailable.** `api.clubelo.com` has been returning errors, so the Elo feature (`03_elo_asof.sql`) isn't built yet. The Elo ingest and parser are written and tested on sample data. Until real Elo files exist, `clean` loads matches and then stops with an error at the Elo step.
- `simulate` and `run` are placeholder commands for now.

---

## Roadmap

| Phase | What | Status |
|---|---|---|
| 0–1 | Repo setup, config, schema, CLI skeleton, DB layer | ✅ |
| 2 | Ingest and clean match data and Club Elo | ✅ (Elo data blocked by the API) |
| 3 | SQL features (rolling form, Elo as of match date) | ✅ Elo pending |
| 4 | Python foundation: DB access, `MatchModel` interface, metrics (RPS, log loss, Brier, accuracy), walk-forward evaluation, `AlwaysHome` and `Bookmaker` baselines | 🚧 In progress |
| 5 | Multinomial logistic model and training pipeline. Each run is logged to `model_runs` and versioned under `models/` with `meta.json` | Planned |
| 6 | Live loop: fixtures ingestion, `predict` for the next matchweek, and `plpred run` for the whole weekly pipeline | Planned |
| 7 | Monte Carlo season simulation (seeded and reproducible): title, top-4 and relegation odds | Planned |
| 8+ | xG features (Understat), Dixon-Coles and LightGBM models, FastAPI endpoints and dashboard, scheduled weekly runs | Future |

Any new model has to beat the logistic baseline on walk-forward RPS, with the bookmaker benchmark reported alongside it.

---

## Getting started

### Requirements

- Go 1.27+ with cgo enabled (needed by the DuckDB driver)
- Python 3.11+

### Go pipeline

Go commands run from `go/`, and `--config` defaults to `../config.yaml`.

```bash
cd go
go build ./... && go test ./...

go run ./cmd/plpred migrate     # create the schema in data/pl.duckdb
go run ./cmd/plpred ingest      # download raw data to data/raw/
go run ./cmd/plpred clean       # load teams, matches, elo
go run ./cmd/plpred features    # build the features table
```

### Python

```bash
cd python
python3 -m venv venv && source venv/bin/activate
pip install -e ".[dev]"
pytest
```

Training, prediction and the API commands (`python -m plpred.train`, `python -m plpred.predict`, `uvicorn plpred.api.main:app`) arrive with Phases 4–8.

---

## Repository layout

```
config.yaml            shared configuration
data/
  mappings/            team_names.csv (every source name → canonical team)
  raw/                 immutable downloaded snapshots (not committed)
  pl.duckdb            the database (not committed)
go/
  cmd/plpred/          CLI entrypoint
  internal/            config, db, ingest, clean, features
sql/
  schema.sql           the schema contract
  features/            feature SQL, run in filename order
python/
  plpred/              models, evaluation, training, prediction, API
  tests/
```

## Data sources

- [football-data.co.uk](https://www.football-data.co.uk/): match results, shots and bookmaker odds
- [Club Elo](http://clubelo.com/): daily Elo ratings
- [Understat](https://understat.com/): expected goals (planned)
