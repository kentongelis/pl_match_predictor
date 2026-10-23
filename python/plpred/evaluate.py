"""Scoring for [H, D, A] probability forecasts. Lower is better except for accuracy"""
import numpy as np

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