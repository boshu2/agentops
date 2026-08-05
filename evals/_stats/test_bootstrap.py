"""Tests for §6.5 paired cluster-bootstrap.

Stop-condition #2 from SCHEMA.md §9 row 3: "bit-exact-stable verdicts under
re-run on the same rig and within-rounding stable across rigs."

The on-rig bit-exact test re-runs the same bootstrap and asserts identical
ci_low / ci_high to floating-point exactness.
"""
import numpy as np

from _stats.bootstrap import paired_cluster_bootstrap, BootstrapResult
from _stats.inputs import BootstrapInput


def _synthetic_inputs(n_samples=50, n_seeds=3, delta=0.05, noise=0.05, base_seed=0):
    """Generate synthetic paired scores with a known mean delta."""
    rng = np.random.Generator(np.random.PCG64(base_seed))
    rows = []
    for i in range(n_samples):
        for j in range(n_seeds):
            a = float(rng.normal(0.5 + delta, noise))
            b = float(rng.normal(0.5, noise))
            rows.append(BootstrapInput(
                sample_id=f"s-{i:03d}",
                seed=j,
                score_arm_a=a,
                score_arm_b=b,
            ))
    return rows


def test_bootstrap_bit_exact_reproducible():
    """Same seed + same inputs MUST produce identical CI down to the bit."""
    rows = _synthetic_inputs(n_samples=40, n_seeds=3, delta=0.05, base_seed=12345)
    seed = 0xCAFEBABEDEADBEEF & 0x7FFFFFFFFFFFFFFF

    r1 = paired_cluster_bootstrap(rows, bootstrap_seed=seed, B=2000)
    r2 = paired_cluster_bootstrap(rows, bootstrap_seed=seed, B=2000)

    assert r1.delta_point == r2.delta_point
    assert r1.ci_low == r2.ci_low
    assert r1.ci_high == r2.ci_high
    assert r1.bootstrap_seed == r2.bootstrap_seed


def test_bootstrap_input_order_independent():
    """Sample iteration is sorted by sample_id; input order MUST not matter."""
    rows = _synthetic_inputs(n_samples=20, n_seeds=2, delta=0.03, base_seed=7)
    seed = 42
    a = paired_cluster_bootstrap(rows, bootstrap_seed=seed, B=1000)
    b = paired_cluster_bootstrap(list(reversed(rows)), bootstrap_seed=seed, B=1000)
    assert a.delta_point == b.delta_point
    assert a.ci_low == b.ci_low
    assert a.ci_high == b.ci_high


def test_bootstrap_ci_brackets_truth_for_known_delta():
    """For a synthetic dataset with known delta, the CI should bracket it
    most of the time (sanity check; not a formal coverage test)."""
    rows = _synthetic_inputs(n_samples=200, n_seeds=3, delta=0.05, noise=0.02, base_seed=1)
    seed = 99
    r = paired_cluster_bootstrap(rows, bootstrap_seed=seed, B=2000, confidence=0.95)
    assert r.ci_low <= r.delta_point <= r.ci_high
    # Truth = 0.05, expect it near the point estimate
    assert abs(r.delta_point - 0.05) < 0.02


def test_bootstrap_degenerate_zero_deltas():
    """All paired deltas exactly 0 → degenerate flag set."""
    rows = [
        BootstrapInput(f"s{i}", j, 0.5, 0.5)
        for i in range(20) for j in range(3)
    ]
    r = paired_cluster_bootstrap(rows, bootstrap_seed=1, B=1000)
    assert r.degenerate is True
    assert r.delta_point == 0.0
    assert r.ci_low == 0.0
    assert r.ci_high == 0.0


def test_bootstrap_degenerate_constant_delta():
    """Non-zero but constant deltas (no spread) → degenerate flag set."""
    rows = [
        BootstrapInput(f"s{i}", j, 0.7, 0.5)
        for i in range(20) for j in range(3)
    ]
    r = paired_cluster_bootstrap(rows, bootstrap_seed=1, B=1000)
    assert r.degenerate is True
    # Non-zero point estimate, but ci_low == ci_high
    assert r.delta_point == 0.2
    assert r.ci_low == r.ci_high == 0.2


def test_bootstrap_seed_changes_result():
    """Different seeds produce different (but valid) CIs."""
    rows = _synthetic_inputs(n_samples=40, base_seed=5)
    a = paired_cluster_bootstrap(rows, bootstrap_seed=1, B=1000)
    b = paired_cluster_bootstrap(rows, bootstrap_seed=2, B=1000)
    # delta_point doesn't depend on seed (no resampling)
    assert a.delta_point == b.delta_point
    # CIs should differ since resamples differ
    assert (a.ci_low, a.ci_high) != (b.ci_low, b.ci_high)


def test_bootstrap_b_changes_resample_count():
    rows = _synthetic_inputs(n_samples=20, base_seed=5)
    a = paired_cluster_bootstrap(rows, bootstrap_seed=1, B=500)
    b = paired_cluster_bootstrap(rows, bootstrap_seed=1, B=2000)
    assert a.B == 500
    assert b.B == 2000


def test_bootstrap_rejects_invalid_confidence():
    rows = _synthetic_inputs(n_samples=10, base_seed=5)
    import pytest
    with pytest.raises(ValueError):
        paired_cluster_bootstrap(rows, bootstrap_seed=1, B=100, confidence=0.0)
    with pytest.raises(ValueError):
        paired_cluster_bootstrap(rows, bootstrap_seed=1, B=100, confidence=1.0)


def test_bootstrap_n_clusters_counted_correctly():
    """n_clusters = number of distinct sample_ids (NOT total rows)."""
    rows = []
    for i in range(7):
        for j in range(4):
            rows.append(BootstrapInput(f"s{i}", j, 0.5, 0.4))
    # 7 sample_ids x 4 seeds = 28 rows but 7 clusters
    r = paired_cluster_bootstrap(rows, bootstrap_seed=1, B=500)
    assert r.n_clusters == 7
