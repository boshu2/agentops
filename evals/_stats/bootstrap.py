"""§6.5 paired cluster-bootstrap CI computation.

Spec (SCHEMA.md §6.5):

    Estimator: paired cluster-bootstrap
    Cluster:   sample_id (resampled WITH replacement at the cluster level;
               within a resampled cluster, all (seed, sample) draws are
               kept paired)
    RNG:       numpy.random.Generator(numpy.random.PCG64(bootstrap_seed))
    Iteration order: sorted by sample_id ascending (C locale, byte-wise)
    Resamples: B (default 10000)
    Percentile: numpy.percentile(..., method='linear')

    Per-sample mean delta = mean_seeds(score_arm_a) - mean_seeds(score_arm_b)
    Bootstrap statistic    = mean across resampled cluster delta means

The output BootstrapResult feeds the verdict computation in verdict.py.
"""
from __future__ import annotations

from collections import defaultdict
from dataclasses import dataclass, field
from typing import Iterable, List

import numpy as np

from .inputs import BootstrapInput


@dataclass
class BootstrapResult:
    delta_point: float           # mean of per-sample mean deltas (no resampling)
    ci_low: float                # `confidence` lower bound from percentile method
    ci_high: float               # `confidence` upper bound
    confidence: float            # the input confidence level (e.g., 0.95)
    B: int                       # number of resamples
    bootstrap_seed: int          # PCG64 seed used
    n_clusters: int              # number of distinct sample_ids
    degenerate: bool = False     # zero-variance flag (forces inconclusive_degenerate)
    resamples: np.ndarray = field(default_factory=lambda: np.array([]))


def _per_sample_mean_deltas(
    inputs: Iterable[BootstrapInput],
) -> tuple[List[str], np.ndarray]:
    """Aggregate (sample_id, seed) draws into one mean delta per sample_id.

    Returns the sorted list of sample_ids and the parallel ndarray of mean
    deltas, in the same order. Mean across seeds within each sample is the
    §6.5 default aggregation; seed_dominates=True graduates to a 2-level
    bootstrap (deferred from Day 3).
    """
    by_sample: dict[str, List[float]] = defaultdict(list)
    for x in inputs:
        by_sample[x.sample_id].append(float(x.score_arm_a) - float(x.score_arm_b))

    sample_ids = sorted(by_sample.keys())
    deltas = np.array([float(np.mean(by_sample[sid])) for sid in sample_ids],
                      dtype=np.float64)
    return sample_ids, deltas


def paired_cluster_bootstrap(
    inputs: Iterable[BootstrapInput],
    *,
    bootstrap_seed: int,
    confidence: float = 0.95,
    B: int = 10000,
) -> BootstrapResult:
    """Compute a paired cluster-bootstrap CI per §6.5.

    Args:
        inputs: per-(sample, seed) score pairs.
        bootstrap_seed: int feeding numpy.random.PCG64.
        confidence: nominal CI level (e.g., 0.95).
        B: number of bootstrap resamples (default 10000 per §6.5).

    Returns:
        BootstrapResult with delta_point, ci_low, ci_high, plus reproducibility
        provenance.
    """
    inputs = list(inputs)
    if not inputs:
        raise ValueError("paired_cluster_bootstrap: no inputs")
    if not (0 < confidence < 1):
        raise ValueError(f"paired_cluster_bootstrap: confidence={confidence} out of (0,1)")
    if B < 1:
        raise ValueError(f"paired_cluster_bootstrap: B={B} must be >=1")

    sample_ids, deltas = _per_sample_mean_deltas(inputs)
    n = len(sample_ids)
    delta_point = float(np.mean(deltas))

    # Degenerate-data check. np.var of a constant float array is not always
    # exactly 0 (e.g., 0.2 isn't representable exactly), so we test for
    # element-wise constancy with tight atol instead.
    constant_deltas = bool(np.allclose(deltas, deltas[0], rtol=0.0, atol=1e-12))
    if n == 0 or (constant_deltas and float(deltas[0]) == 0.0):
        return BootstrapResult(
            delta_point=0.0,
            ci_low=0.0,
            ci_high=0.0,
            confidence=confidence,
            B=B,
            bootstrap_seed=bootstrap_seed,
            n_clusters=n,
            degenerate=True,
        )
    if constant_deltas:
        # Non-zero but constant deltas — also degenerate (no spread to bootstrap).
        return BootstrapResult(
            delta_point=delta_point,
            ci_low=delta_point,
            ci_high=delta_point,
            confidence=confidence,
            B=B,
            bootstrap_seed=bootstrap_seed,
            n_clusters=n,
            degenerate=True,
        )

    rng = np.random.Generator(np.random.PCG64(bootstrap_seed))
    # Cluster resampling: draw n indices WITH replacement from [0, n).
    idx = rng.integers(low=0, high=n, size=(B, n), dtype=np.int64, endpoint=False)
    resamples = deltas[idx].mean(axis=1)

    alpha = 1.0 - confidence
    lo_pct = (alpha / 2.0) * 100.0
    hi_pct = (1.0 - alpha / 2.0) * 100.0
    ci_low = float(np.percentile(resamples, lo_pct, method="linear"))
    ci_high = float(np.percentile(resamples, hi_pct, method="linear"))

    return BootstrapResult(
        delta_point=delta_point,
        ci_low=ci_low,
        ci_high=ci_high,
        confidence=confidence,
        B=B,
        bootstrap_seed=bootstrap_seed,
        n_clusters=n,
        degenerate=False,
        resamples=resamples,
    )
