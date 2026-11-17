"""Leakage test (rule 1): a match's features must not change when its own day and later are removed.

For each sampled match, this rebuilds the features from a copy of the database cut off before the
match's date, with the match itself turned into an unplayed fixture. If any feature column differs
from the real features table, that feature was reading data from the match's own day or later.
"""
from collections.abc import Iterator
from pathlib import Path

import duckdb
import numpy as np
import pandas as pd
import pytest

from plpred.config import load

CFG = load()
N_RANDOM = 40  # random matches on top of every season's opening day
SEED = 0
# Not features: the label and the played flag are meant to change when the result is removed
NOT_FEATURES = {"result", "is_played"}
# One column per feature family. Add new features here, so a dropped column fails loudly
MUST_COMPARE = {
    "home_pts_l5", "away_venue_pts_l5", "away_rest_days", "home_is_promoted",
    "home_elo", "away_elo", "diff_elo",
}

pytestmark = pytest.mark.skipif(
    not CFG.db_path.exists(), reason="needs a built database: plpred clean && plpred features"
)


@pytest.fixture(scope="module")
def con() -> Iterator[duckdb.DuckDBPyConnection]:
    """An in-memory database with the real schema, plus the real database attached read-only."""
    con = duckdb.connect()
    con.execute((CFG.sql_dir / "schema.sql").read_text())
    con.execute(f"ATTACH '{CFG.db_path}' AS real (READ_ONLY)")
    yield con
    con.close()


def sample_matches(con: duckdb.DuckDBPyConnection) -> list[str]:
    """Every season's opening-day matches (promotion, summer regression, empty windows) plus a
    seeded random sample of the rest."""
    openers = con.execute("""
        SELECT match_id FROM real.features f
        WHERE is_played
          AND date = (SELECT min(date) FROM real.features WHERE season = f.season)
    """).fetchall()
    played = con.execute(
        "SELECT match_id FROM real.features WHERE is_played ORDER BY match_id"
    ).fetchall()
    rng = np.random.default_rng(SEED)
    randoms = rng.choice([r[0] for r in played], size=N_RANDOM, replace=False).tolist()
    return sorted({r[0] for r in openers} | set(randoms))


def run_features(con: duckdb.DuckDBPyConnection, sql_dir: Path) -> None:
    """Run sql/features/*.sql in filename order, like `plpred features` does."""
    for path in sorted((sql_dir / "features").glob("*.sql")):
        con.execute(path.read_text())


def rebuild_before(con: duckdb.DuckDBPyConnection, match_id: str) -> pd.DataFrame:
    """Rebuild features from rows dated strictly before the match, and return the match's row."""
    (date,) = con.execute(
        "SELECT date FROM real.matches WHERE match_id = ?", [match_id]
    ).fetchone()
    con.execute("DELETE FROM matches")
    con.execute("DELETE FROM fixtures")
    con.execute("DELETE FROM elo")
    con.execute("INSERT INTO matches SELECT * FROM real.matches WHERE date < ?", [date])
    # The match becomes a fixture: its teams and date are known in advance, its result isn't
    con.execute("""
        INSERT INTO fixtures
        SELECT match_id, season, date, home_team_id, away_team_id
        FROM real.matches WHERE match_id = ?""", [match_id])
    con.execute("INSERT INTO elo SELECT * FROM real.elo WHERE date_from < ?", [date])
    run_features(con, CFG.sql_dir)
    return con.execute("SELECT * FROM features WHERE match_id = ?", [match_id]).df()


def test_features_only_use_the_past(con):
    all_columns = con.execute("SELECT * FROM real.features LIMIT 0").df().columns
    columns = [c for c in all_columns if c not in NOT_FEATURES]
    missing = MUST_COMPARE - set(columns)
    assert not missing, f"leakage test would skip these columns: {sorted(missing)}"

    failures = []
    for match_id in sample_matches(con):
        real = con.execute("SELECT * FROM real.features WHERE match_id = ?", [match_id]).df()
        rebuilt = rebuild_before(con, match_id)
        assert len(rebuilt) == 1, f"{match_id}: rebuilt features has {len(rebuilt)} rows"
        for col in columns:
            a, b = real.at[0, col], rebuilt.at[0, col]
            if pd.isna(a) and pd.isna(b):
                continue
            if pd.isna(a) or pd.isna(b) or (a != b and not np.isclose(a, b)):
                failures.append(f"{match_id} {col}: real {a!r}, from past only {b!r}")
    assert not failures, "features use data from on or after the match date:\n" + "\n".join(
        failures
    )


def test_elo_is_never_null_for_played_matches(con):
    """Every team has a start-of-season row before its first match, so NULL means a bad join."""
    (n,) = con.execute("""
        SELECT count(*) FROM real.features
        WHERE is_played AND (home_elo IS NULL OR away_elo IS NULL)
    """).fetchone()
    assert n == 0
