"""The interface every match model implement"""
from typing import Protocol

import numpy as np
import pandas as pd

# Column order for every probability array: home win, draw, away win
OUTCOMES = ("H", "D", "A")

class MatchModel(Protocol):
    name: str

    def fit(self, X: pd.DataFrame, y: np.ndarray) -> "MatchModel":
        pass
    
    def predict_proba(self, X: pd.DataFrame) -> np.ndarray:
        """Return an (n, 3) array ordered [H, D, A], each row summing to 1."""
        ...

def encode_result(results: pd.Series) -> np.ndarray:
    """Map 'H'/'D'/'A' labels to 0/1/2 so they index the probability columns. """
    mapping = {o: i for i, o in enumerate(OUTCOMES)}
    encoded = results.map(mapping)
    if encoded.isna().any():
        raise ValueError(f"unexpected result label: {set(results) - set(OUTCOMES)}")
    return encoded.to_numpy(dtype=int)