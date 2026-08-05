"""§6.5 verdict computation.

Spec (SCHEMA.md §6.5 "Verdict computation"):

    Result                                           | Verdict
    Bootstrap input has zero variance (degenerate)   | inconclusive_degenerate
    decision_rule passes after correction            | improved (delta>0) or regressed (delta<0)
    decision_rule fails AND n>=n_required AND
        CI is wider than 2x MDE                      | inconclusive_high_variance
    decision_rule fails AND n>=n_required            | no_change
    n < n_required                                   | underpowered (NEVER no_change)

`underpowered` is distinct from `no_change` — the second-pass council called
this a methodology gap. A null result that's underpowered is not evidence of
equivalence.
"""
from __future__ import annotations

from dataclasses import dataclass
from enum import Enum
from typing import Optional

from .bootstrap import BootstrapResult


class VerdictKind(str, Enum):
    IMPROVED = "improved"
    REGRESSED = "regressed"
    NO_CHANGE = "no_change"
    UNDERPOWERED = "underpowered"
    INCONCLUSIVE_HIGH_VARIANCE = "inconclusive_high_variance"
    INCONCLUSIVE_DEGENERATE = "inconclusive_degenerate"


@dataclass
class Verdict:
    kind: VerdictKind
    delta_point: float
    ci_low: float
    ci_high: float
    n: int
    n_required: int
    rule_kind: str
    rule_passed: bool
    mde: Optional[float] = None
    notes: str = ""

    def to_manifest_field(self) -> str:
        return self.kind.value


def compute_verdict(
    result: BootstrapResult,
    *,
    rule_kind: str = "ci_excludes_zero",
    min_delta: Optional[float] = None,
    n_required: int,
    mde: Optional[float] = None,
) -> Verdict:
    """Map a BootstrapResult + decision_rule to a §6.5 Verdict.

    Args:
        result: output of paired_cluster_bootstrap.
        rule_kind: "ci_excludes_zero" | "min_delta".
        min_delta: required delta floor when rule_kind == "min_delta".
        n_required: derived per §6.5 power_analysis (Day-3) or static
                    Task.stats.min_n_samples (Day-2 fallback).
        mde: minimum_detectable_effect (used to flag inconclusive_high_variance).
    """
    n = result.n_clusters

    # Degenerate first — overrides everything, including underpowered.
    if result.degenerate:
        return Verdict(
            kind=VerdictKind.INCONCLUSIVE_DEGENERATE,
            delta_point=result.delta_point,
            ci_low=result.ci_low,
            ci_high=result.ci_high,
            n=n,
            n_required=n_required,
            rule_kind=rule_kind,
            rule_passed=False,
            mde=mde,
            notes="zero variance / constant deltas; metric collapsed",
        )

    # Underpowered — even if rule passes, n must meet the floor.
    if n < n_required:
        return Verdict(
            kind=VerdictKind.UNDERPOWERED,
            delta_point=result.delta_point,
            ci_low=result.ci_low,
            ci_high=result.ci_high,
            n=n,
            n_required=n_required,
            rule_kind=rule_kind,
            rule_passed=False,
            mde=mde,
            notes=f"n={n} < n_required={n_required}",
        )

    # Decision rule application
    if rule_kind == "ci_excludes_zero":
        rule_passed = (result.ci_low > 0.0) or (result.ci_high < 0.0)
    elif rule_kind == "min_delta":
        if min_delta is None:
            raise ValueError("compute_verdict: min_delta required for rule_kind=min_delta")
        # one-sided: passes when ci_low > min_delta (positive direction)
        rule_passed = result.ci_low > min_delta
    else:
        raise ValueError(f"compute_verdict: unsupported rule_kind={rule_kind!r}")

    if rule_passed:
        kind = VerdictKind.IMPROVED if result.delta_point > 0 else VerdictKind.REGRESSED
        return Verdict(
            kind=kind,
            delta_point=result.delta_point,
            ci_low=result.ci_low,
            ci_high=result.ci_high,
            n=n,
            n_required=n_required,
            rule_kind=rule_kind,
            rule_passed=True,
            mde=mde,
        )

    # Rule fails AND n >= n_required
    if mde is not None and (result.ci_high - result.ci_low) > (2.0 * float(mde)):
        return Verdict(
            kind=VerdictKind.INCONCLUSIVE_HIGH_VARIANCE,
            delta_point=result.delta_point,
            ci_low=result.ci_low,
            ci_high=result.ci_high,
            n=n,
            n_required=n_required,
            rule_kind=rule_kind,
            rule_passed=False,
            mde=mde,
            notes=f"CI width {result.ci_high - result.ci_low:.4f} > 2*MDE={2*float(mde):.4f}",
        )

    return Verdict(
        kind=VerdictKind.NO_CHANGE,
        delta_point=result.delta_point,
        ci_low=result.ci_low,
        ci_high=result.ci_high,
        n=n,
        n_required=n_required,
        rule_kind=rule_kind,
        rule_passed=False,
        mde=mde,
    )
