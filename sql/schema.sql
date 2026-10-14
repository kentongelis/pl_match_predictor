-- schema.sql: the DuckDB contract between Go and Python
-- Applied on every open by go/internal/db, os every statement must be idempotent
-- Change deliberately: update Go and Python in the same commit

-- Canonical teams. team_id is a stable slug (.e.g. 'man_city') taken from 
-- data/mappings/team_names.csv, so match_ids stay human-readable.
CREATE TABLE IF NOT EXISTS teams (
    team_id VARCHAR PRIMARY KEY,
    name    VARCHAR NOT NULL UNIQUE
);

-- Played matches. One row per match.
-- match_id = {season}_{date}_{home_team_id}_{away_team_id}.
CREATE TABLE IF NOT EXISTS matches (
    match_id VARCHAR PRIMARY KEY,
    season VARCHAR NOT NULL,               -- '2014-15'
    date DATE NOT NULL,
    home_team_id VARCHAR NOT NULL,
    away_team_id VARCHAR NOT NULL,
    home_goals INTEGER NOT NULL,
    away_goals INTEGER NOT NULL,
    result VARCHAR NOT NULL CHECK (result IN ('H', 'D', 'A')),
    home_shots INTEGER,
    away_shots INTEGER,
    home_sot INTEGER,
    away_sot INTEGER,
    home_xg DOUBLE,
    away_xg DOUBLE,
    odds_h DOUBLE,
    odds_d DOUBLE,
    odds_a DOUBLE,
    CHECK (home_team_id <> away_team_id)
);

-- Upcoming matches. Same key format as matches.
CREATE TABLE IF NOT EXISTS fixtures (
    match_id VARCHAR PRIMARY KEY,
    season  VARCHAR NOT NULL,
    date DATE NOT NULL,
    home_team_id VARCHAR NOT NULL,
    away_team_id VARCHAR NOT NULL,
    CHECK (home_team_id <> away_team_id)
);

-- ClubElo ratings as validity ranges: the rating holds from date_from to date_to.
CREATE TABLE IF NOT EXISTS elo (
    team_id VARCHAR NOT NULL,
    date_from DATE NOT NULL,
    date_to DATE NOT NULL,
    elo DOUBLE NOT NULL,
    PRIMARY KEY (team_id, date_from),
    CHECK (date_to >= date_from)
);

-- Model outputs. One row per match per model per prediction run.
CREATE TABLE IF NOT EXISTS predictions (
    match_id VARCHAR NOT NULL,
    model_version VARCHAR NOT NULL,
    p_home DOUBLE NOT NULL CHECK (p_home BETWEEN 0 AND 1),
    p_draw DOUBLE NOT NULL CHECK (p_draw BETWEEN 0 AND 1),
    p_away DOUBLE NOT NULL CHECK (p_away BETWEEN 0 AND 1),
    predicted_at TIMESTAMP NOT NULL,
    PRIMARY KEY (match_id, model_version, predicted_at),
    CHECK (abs(p_home + p_draw + p_away - 1) < 1e-6) -- prevent float noise
);

-- Monte Carlo season outcomes. One row per team per run
-- seed + iterations are stored so any run can be reproduced exactly.
CREATE TABLE IF NOT EXISTS simulations (
    run_id VARCHAR NOT NULL,
    team_id VARCHAR NOT NULL,
    exp_points DOUBLE NOT NULL,
    p_title DOUBLE NOT NULL,
    p_top4 DOUBLE NOT NULL,
    p_relegation DOUBLE NOT NULL,
    seed BIGINT NOT NULL,
    iterations INTEGER NOT NULL,
    simulated_at TIMESTAMP NOT NULL,
    PRIMARY KEY (run_id, team_id)
);

-- One row per (model_version, walk_forward test session).
CREATE TABLE IF NOT EXISTS model_runs (
    model_version VARCHAR NOT NULL,
    model_type VARCHAR NOT NULL,
    train_seasons VARCHAR NOT NULL,      -- e.g. '2014-15....2021-22'
    test_season VARCHAR NOT NULL,
    rps DOUBLE NOT NULL,
    log_loss DOUBLE NOT NULL,
    brier DOUBLE NOT NULL,
    accuracy DOUBLE NOT NULL,
    created_at TIMESTAMP NOT NULL,
    PRIMARY KEY (model_version, test_season)
);

