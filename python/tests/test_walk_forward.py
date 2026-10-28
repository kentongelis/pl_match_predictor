import pandas as pd
import pytest

from plpred.evaluate import pick_test_seasons, walk_forward
from plpred.models.baseline import BaseRate

DF = pd.DataFrame({
    "season": ["2020-21"] * 4 + ["2021-22"] * 4 + ["2022-23"] * 4 + ["2023-24"] * 2,
    "result": list("HHDA" "HDAA" "HHHD" "AD"),
})


class SeasonSpy(BaseRate):
    """BaseRate that records which seasons it was trained on."""
    seen: list[set[str]] = []

    def fit(self, X, y):
        SeasonSpy.seen.append(set(X["season"]))
        return super().fit(X, y)


def test_trains_only_on_earlier_seasons() -> None:
    SeasonSpy.seen = []
    table = walk_forward(SeasonSpy, DF, ["2021-22", "2022-23"])
    assert SeasonSpy.seen == [{"2020-21"}, {"2020-21", "2021-22"}]
    assert list(table["season"]) == ["2021-22", "2022-23", "all"]
    assert list(table["n_test"]) == [4, 4, 8]


def test_pick_test_seasons_skips_current_season() -> None:
    assert pick_test_seasons(DF, 2, current_season=2023) == ["2021-22", "2022-23"]


def test_pick_test_seasons_needs_a_training_season() -> None:
    with pytest.raises(ValueError):
        pick_test_seasons(DF, 3, current_season=2023)