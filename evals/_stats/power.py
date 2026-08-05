"""§6.5 power-derived n_required.

Spec (SCHEMA.md §6.5):

    n_required = power_analysis(
      baseline_rate=Task.baseline.expected_score,
      min_detectable_effect=Suite.stats.power.minimum_detectable_effect,
      alpha=Suite.stats.power.alpha,
      power=0.80,                  # default; can be overridden
      comparison=Task.stats.paired
    )

Day-3 implementation: paired t-test power formula, Wald-style. The function
returns the smallest integer n that, given the parameters, achieves the
nominal power. Edge cases:

  - baseline_rate near 0 or 1 with proportion-style metric: variance is
    derived from binomial p*(1-p); for a paired comparison we approximate
    the variance of the paired delta as 2*p*(1-p) (worst-case independent).
  - Continuous-metric case: caller supplies a variance via the optional
    `variance` parameter; defaults to 0.25 (max binomial variance) when
    omitted.

For two-sample (unpaired) the formula uses the larger Wald multiplier; this
is conservative but matches Day-3 expectations where unpaired isn't the
dominant case yet.
"""
from __future__ import annotations

import math
from dataclasses import dataclass
from typing import Optional

from scipy.stats import norm


@dataclass(frozen=True)
class PowerInputs:
    baseline_rate: float
    minimum_detectable_effect: float
    alpha: float
    power: float = 0.80
    paired: bool = True
    variance: Optional[float] = None  # override binomial p*(1-p) when known


def power_derived_n_required(p: PowerInputs) -> int:
    """Return the integer n required to detect MDE at the given alpha and power.

    Uses the standard normal approximation:

        n_paired   = ((z_{1-alpha/2} + z_{1-beta})^2 * sigma_d^2) / (MDE^2)
        n_unpaired = 2 * n_paired   # rough approximation, conservative

    where sigma_d^2 is the variance of the paired delta. For binomial
    metrics, we use sigma_d^2 = 2 * p * (1-p) (independent-arm worst case).
    """
    if not (0.0 < p.alpha < 1.0):
        raise ValueError(f"power_derived_n_required: alpha={p.alpha} out of (0,1)")
    if not (0.0 < p.power < 1.0):
        raise ValueError(f"power_derived_n_required: power={p.power} out of (0,1)")
    if p.minimum_detectable_effect <= 0.0:
        raise ValueError("power_derived_n_required: MDE must be > 0")

    z_alpha = norm.ppf(1.0 - p.alpha / 2.0)
    z_beta = norm.ppf(p.power)

    if p.variance is not None:
        sigma_d_sq = float(p.variance)
    else:
        # Binomial variance of the paired delta, worst-case.
        rate = max(min(p.baseline_rate, 0.999), 0.001)
        sigma_d_sq = 2.0 * rate * (1.0 - rate)

    if sigma_d_sq <= 0.0:
        return 0

    n_paired = ((z_alpha + z_beta) ** 2 * sigma_d_sq) / (p.minimum_detectable_effect ** 2)
    if not p.paired:
        n_paired *= 2.0
    return int(math.ceil(n_paired))
