"""Scoring for [H, D, A] probability forecasts alongside a walk-forward evaluation by season"""
from collections.abc import Callable

import numpy as np
import duckdb
import pandas as pd

from plpred.config import load
from plpred.db import connect
from plpred.models.base import MatchModel, encode_result
from plpred.models.baseline import BaseRate, Bookmaker

EPS = 1e-15

def one_hot(y: np.ndarray, k: int = 3) -> np.ndarray:
    return np.eye(k)[y]

def rps(proba: np.ndarray, y: np.ndarray) -> float:
    """Ranked Probability Score: compares cumulative distributions over ordered outcomes."""
    cum_p = np.cumsum(proba, axis = 1)
    cum_o = np.cumsum(one_hot(y, proba.shape[1]), axis=1)
    # The last cumulative column is always 1-1 = 0, so drop it; divide by k - 1
    per_match = ((cum_p[:, :-1] - cum_o[:, :-1]) ** 2).sum(axis=1) / (proba.shape[1] - 1)
    return float(per_match.mean())

def log_loss(proba: np.ndarray, y: np.ndarray) -> float:
    p_true = np.clip(proba[np.arange(len(y)), y], EPS, 1.0)
    return float(-np.log(p_true).mean())

def brier(proba: np.ndarray, y: np.ndarray) -> float:
    """Multiclass Brier: square error summed over the 3 outcomes."""
    return float(((proba - one_hot(y, proba.shape[1])) ** 2).sum(axis=1).mean())

def accuracy(proba: np.ndarray, y: np.ndarray) -> float:
    return float((proba.argmax(axis=1) == y).mean())

def score(proba: np.ndarray, y: np.ndarray) -> dict[str, float]:
    return {
        "rps" : rps(proba, y),
        "log_loss" : log_loss(proba, y),
        "brier" : brier(proba, y),
        "accuracy" : accuracy(proba, y),
    }

# Odds aren't a feature, so they are only joined here for the benchmark
DATASET_SQL = """
SELECT f.*, m.odds_h, m.odds_d, m.odds_a
FROM features f
JOIN matches m USING (match_id)
WHERE f.is_played
ORDER BY f.date, f.match_id
"""

def load_dataset(con: duckdb.DuckDBPyConnection) -> pd.DataFrame:
    return con.sql(DATASET_SQL).df()

def season_label(start_year: int) -> str:
    """2025 -> '2025-26', matching the season column."""
    return f"{start_year}-{(start_year + 1) % 100:02d}"

def pick_test_seasons(df: pd.DataFrame, n: int, current_season: int) -> list[str]:
    """The last n finished seasons. The current season is partial, so it's skipped."""
    finished = sorted(s for s in df["season"].unique() if s < season_label(current_season))
    if len(finished) <= n:
        raise ValueError(f"nede more than {n} finished seasons to walk forward; have {len(finished)}")
    return finished[-n:]

def walk_forward(
    make_model: Callable[[], MatchModel], df: pd.DataFrame, test_seasons: list[str]
) -> pd.DataFrame:
    """For each test season, fit a fresh model on every earlier saeson, then score it on that season."""
    rows, all_proba, all_y = [], [], []
    for season in test_seasons:
        train = df[df["season"] < season]
        test = df[df["season"] == season]
        model = make_model().fit(train, encode_result(train["result"]))
        proba = model.predict_proba(test)
        y = encode_result(test["result"])
        rows.append({"season": season, "n_train": len(train), "n_test": len(test), **score(proba, y)})
        all_proba.append(proba)
        all_y.append(y)
    
    # Pooled row with every test match score together
    pooled = score(np.vstack(all_proba), np.concatenate(all_y))
    rows.append({"season": "all", "n_train": None, "n_test": sum(len(y) for y in all_y), **pooled})
    table = pd.DataFrame(rows)
    table["n_train"] = table["n_train"].astype("Int64") # nullable int, so the None doesn't make it float
    return table

def main() -> None:
    cfg = load()
    with connect(cfg=cfg) as con:
        df = load_dataset(con)
    seasons = pick_test_seasons(df, cfg.walk_forward_test_seasons, cfg.current_season)
    for make_model in (BaseRate, Bookmaker):
        table = walk_forward(make_model, df, seasons)
        print(f"\n== {make_model.name} ==")
        print(table.to_string(index=False, float_format="{:.4f}.format"))

if __name__ == "__main__":
    main()