import numpy as np
import pytest

from plpred.evaluate import accuracy, brier, log_loss, rps, score

# One forecast reused below: 60% home, 30% draw, 10% away
P = np.array([[0.6, 0.3, 0.1]])

@pytest.mark.parametrize(
    ("outcome", "expected"),
    [(0, 0.085), (1, 0.185), (2, 0.585)],
)
def test_rps_hand_values(outcome: int, expected: float) -> None:
    assert rps(P, np.array([outcome])) == pytest.approx(expected)

def test_rps_is_mean_over_matches() -> None:
    proba = np.repeat(P, 3, axis=0)
    assert rps(proba, np.array([0, 1, 2])) == pytest.approx(0.285)

def test_rps_rewards_being_close() -> None:
    # Both forecasts give the away win 10%, so log loss can't tell them apart.
    # RPS can: when the away side wins, a draw was the "nearer" miss.
    y = np.array([2])
    home_heavy = np.array([[0.8, 0.1, 0.1]])
    draw_heavy = np.array([[0.1, 0.8, 0.1]])
    assert log_loss(home_heavy, y) == pytest.approx(log_loss(draw_heavy, y))
    assert rps(home_heavy, y) == pytest.approx(0.725)
    assert rps(draw_heavy, y) == pytest.approx(0.41)


def test_perfect_forecast_scores_zero() -> None:
    proba = np.eye(3)
    y = np.array([0, 1, 2])
    assert rps(proba, y) == pytest.approx(0.0)
    assert brier(proba, y) == pytest.approx(0.0)
    assert log_loss(proba, y) == pytest.approx(0.0)
    assert accuracy(proba, y) == 1.0


def test_brier_and_log_loss_hand_values() -> None:
    y = np.array([0])
    assert brier(P, y) == pytest.approx(0.26)
    assert log_loss(P, y) == pytest.approx(-np.log(0.6))


def test_log_loss_is_finite_when_true_outcome_has_zero_prob() -> None:
    proba = np.array([[1.0, 0.0, 0.0]])
    assert np.isfinite(log_loss(proba, np.array([2])))


def test_accuracy_uses_argmax() -> None:
    proba = np.repeat(P, 3, axis=0)  # always picks home
    assert accuracy(proba, np.array([0, 1, 2])) == pytest.approx(1 / 3)


def test_score_returns_all_metrics() -> None:
    assert set(score(P, np.array([0]))) == {"rps", "log_loss", "brier", "accuracy"}