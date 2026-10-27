import numpy as np
import pandas as pd
import pytest

from plpred.models.baseline import BaseRate, Bookmaker

ODDS = pd.DataFrame({"odds_h": [2.0, 1.5], "odds_d": [3.5, 4.0], "odds_a": [4.0, 7.0]})


@pytest.mark.parametrize("model_cls", [BaseRate, Bookmaker])
def test_returns_valid_probabilities(model_cls: type) -> None:
    proba = model_cls().fit(ODDS, np.array([0, 2])).predict_proba(ODDS)
    assert proba.shape == (len(ODDS), 3)
    assert (proba >= 0).all()
    np.testing.assert_allclose(proba.sum(axis=1), 1.0)


def test_base_rate_learns_training_shares() -> None:
    y = np.array([0, 0, 1, 2])  # 2 home wins, 1 draw, 1 away win
    model = BaseRate().fit(pd.DataFrame(index=range(4)), y)
    np.testing.assert_allclose(model.predict_proba(ODDS), [[0.5, 0.25, 0.25]] * 2)


def test_base_rate_requires_fit() -> None:
    with pytest.raises(RuntimeError):
        BaseRate().predict_proba(ODDS)


def test_bookmaker_removes_overround() -> None:
    # 1/2 + 1/3.5 + 1/4 = 29/28, so normalizing gives 14/29, 8/29, 7/29
    proba = Bookmaker().predict_proba(ODDS.iloc[:1])
    np.testing.assert_allclose(proba, [[14 / 29, 8 / 29, 7 / 29]])


def test_bookmaker_rejects_missing_odds() -> None:
    odds = ODDS.copy()
    odds.loc[0, "odds_d"] = np.nan
    with pytest.raises(ValueError):
        Bookmaker().predict_proba(odds)