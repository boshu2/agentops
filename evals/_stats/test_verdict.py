"""Tests for §6.5 verdict computation."""
import pytest

from _stats.bootstrap import BootstrapResult
from _stats.verdict import compute_verdict, VerdictKind


def _result(*, delta_point, ci_low, ci_high, n=100, degenerate=False):
    return BootstrapResult(
        delta_point=delta_point,
        ci_low=ci_low,
        ci_high=ci_high,
        confidence=0.95,
        B=10000,
        bootstrap_seed=42,
        n_clusters=n,
        degenerate=degenerate,
    )


def test_verdict_improved_when_ci_excludes_zero_positive():
    r = _result(delta_point=0.05, ci_low=0.01, ci_high=0.09, n=100)
    v = compute_verdict(r, n_required=50, mde=0.03)
    assert v.kind == VerdictKind.IMPROVED
    assert v.rule_passed is True


def test_verdict_regressed_when_ci_excludes_zero_negative():
    r = _result(delta_point=-0.05, ci_low=-0.09, ci_high=-0.01, n=100)
    v = compute_verdict(r, n_required=50, mde=0.03)
    assert v.kind == VerdictKind.REGRESSED


def test_verdict_no_change_when_ci_includes_zero_n_sufficient_low_variance():
    r = _result(delta_point=0.005, ci_low=-0.01, ci_high=0.02, n=100)
    v = compute_verdict(r, n_required=50, mde=0.05)
    # CI width 0.03 < 2*MDE 0.10 → no_change
    assert v.kind == VerdictKind.NO_CHANGE
    assert v.rule_passed is False


def test_verdict_inconclusive_high_variance_when_ci_wide():
    # CI width = 0.20, 2*MDE = 0.10 → high variance
    r = _result(delta_point=0.0, ci_low=-0.10, ci_high=0.10, n=100)
    v = compute_verdict(r, n_required=50, mde=0.05)
    assert v.kind == VerdictKind.INCONCLUSIVE_HIGH_VARIANCE


def test_verdict_underpowered_when_n_below_required():
    r = _result(delta_point=0.05, ci_low=0.01, ci_high=0.09, n=20)
    v = compute_verdict(r, n_required=50, mde=0.03)
    assert v.kind == VerdictKind.UNDERPOWERED
    assert "n=20" in v.notes


def test_verdict_underpowered_overrides_significant_result():
    """Even if rule passes, n < n_required forces underpowered (not improved)."""
    r = _result(delta_point=0.5, ci_low=0.4, ci_high=0.6, n=10)
    v = compute_verdict(r, n_required=50, mde=0.05)
    assert v.kind == VerdictKind.UNDERPOWERED


def test_verdict_inconclusive_degenerate_overrides_everything():
    """Degenerate flag overrides underpowered + improved/regressed."""
    r = _result(delta_point=0.0, ci_low=0.0, ci_high=0.0, n=10, degenerate=True)
    v = compute_verdict(r, n_required=50, mde=0.05)
    assert v.kind == VerdictKind.INCONCLUSIVE_DEGENERATE


def test_verdict_min_delta_rule():
    """min_delta rule: passes when ci_low > min_delta."""
    r = _result(delta_point=0.05, ci_low=0.025, ci_high=0.07, n=100)
    v = compute_verdict(r, rule_kind="min_delta", min_delta=0.02, n_required=50, mde=0.03)
    assert v.kind == VerdictKind.IMPROVED


def test_verdict_min_delta_fails_when_ci_low_at_threshold():
    """min_delta strict: ci_low must exceed min_delta, not equal."""
    r = _result(delta_point=0.05, ci_low=0.02, ci_high=0.07, n=100)
    v = compute_verdict(r, rule_kind="min_delta", min_delta=0.02, n_required=50, mde=0.03)
    assert v.kind == VerdictKind.NO_CHANGE  # rule failed, n sufficient, low variance


def test_verdict_min_delta_requires_min_delta_arg():
    r = _result(delta_point=0.05, ci_low=0.01, ci_high=0.09, n=100)
    with pytest.raises(ValueError):
        compute_verdict(r, rule_kind="min_delta", n_required=50)


def test_verdict_unknown_rule_kind():
    r = _result(delta_point=0.05, ci_low=0.01, ci_high=0.09, n=100)
    with pytest.raises(ValueError):
        compute_verdict(r, rule_kind="bogus", n_required=50)


def test_verdict_kind_serializes_to_manifest_string():
    r = _result(delta_point=0.05, ci_low=0.01, ci_high=0.09, n=100)
    v = compute_verdict(r, n_required=50)
    assert v.to_manifest_field() == "improved"
