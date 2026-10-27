"""Reference models: anything else has to beat BaseRate and be compared against Bookmaker."""
import numpy as np
import pandas as pd

ODDS_COLS = ["odds_h", "odds_d", "odds_a"]

class BaseRate:
    """Predicts the training-set share of home wins, draws and away wins for every match."""

    name = "base_rate"

    def __init__(self) -> None:
        self.rates_: np.ndarray | None = None

    def fit(self, X: pd.DataFrame, y: np.ndarray) -> "BaseRate":
        counts = np.bincount(y, minlength=3)
        self.rates_ = counts / counts.sum()
        return self
    def predict_proba(self, X: pd.DataFrame) -> np.ndarray:
        if self.rates_ is None:
            raise RuntimeError("BaseRate.predict_proba called before fit")
        return np.tile(self.rates_, (len(X), 1))

class Bookmaker:
    """Turns pre-match decimal odds into probabilities."""
    
    name = "bookmaker"

    def fit(self, X: pd.DataFrame, y: np.ndarray) -> "Bookmaker":
        """ Nothing to learn so fit does nothing"""
        return self

    def predict_proba(self, X: pd.DataFrame) -> np.ndarray:
        odds = X[ODDS_COLS].to_numpy(dtype=float)
        if np.isnan(odds).any():
            raise ValueError("Bookmaker needs odds for every match; found missing odds")
        implied = 1.0 / odds
        # Implied probs sum to a bit over 1 (the overround) so scale each row back to 1
        return implied / implied.sum(axis=1, keepdims=True)