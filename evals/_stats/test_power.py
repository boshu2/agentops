"""Tests for §6.5 power-derived n_required."""
import pytest

from _stats.power import PowerInputs, power_derived_n_required


def test_power_basic_paired():
    """Sanity: harder-to-detect effect → larger n_required."""
    small = PowerInputs(baseline_rate=0.5, minimum_detectable_effect=0.02, alpha=0.05, power=0.80)
    large = PowerInputs(baseline_rate=0.5, minimum_detectable_effect=0.10, alpha=0.05, power=0.80)
    n_small = power_derived_n_required(small)
    n_large = power_derived_n_required(large)
    assert n_small > n_large
    assert n_large > 0


def test_power_higher_power_means_more_n():
    a = PowerInputs(baseline_rate=0.5, minimum_detectable_effect=0.05, alpha=0.05, power=0.80)
    b = PowerInputs(baseline_rate=0.5, minimum_detectable_effect=0.05, alpha=0.05, power=0.95)
    assert power_derived_n_required(b) > power_derived_n_required(a)


def test_power_smaller_alpha_means_more_n():
    a = PowerInputs(baseline_rate=0.5, minimum_detectable_effect=0.05, alpha=0.05, power=0.80)
    b = PowerInputs(baseline_rate=0.5, minimum_detectable_effect=0.05, alpha=0.01, power=0.80)
    assert power_derived_n_required(b) > power_derived_n_required(a)


def test_power_unpaired_requires_more_than_paired():
    paired = PowerInputs(
        baseline_rate=0.5, minimum_detectable_effect=0.05, alpha=0.05,
        power=0.80, paired=True,
    )
    unpaired = PowerInputs(
        baseline_rate=0.5, minimum_detectable_effect=0.05, alpha=0.05,
        power=0.80, paired=False,
    )
    assert power_derived_n_required(unpaired) > power_derived_n_required(paired)


def test_power_known_value_at_p_half_mde_05():
    """For baseline_rate=0.5, MDE=0.05, alpha=0.05, power=0.80 (paired):

    sigma_d^2 = 2*p*(1-p) = 0.5  (worst-case binomial variance of paired delta
                                  under independence — see power.py docstring)
    z_alpha/2 = norm.ppf(0.975) ≈ 1.95996
    z_beta    = norm.ppf(0.80)  ≈ 0.84162
    n = (z_alpha + z_beta)^2 * sigma_d^2 / MDE^2
      = (2.80158)^2 * 0.5 / 0.0025
      ≈ 1569.6 -> ceil = 1570
    """
    p = PowerInputs(baseline_rate=0.5, minimum_detectable_effect=0.05, alpha=0.05, power=0.80)
    n = power_derived_n_required(p)
    assert 1568 <= n <= 1572


def test_power_explicit_variance_overrides_binomial():
    p_low_var = PowerInputs(
        baseline_rate=0.5, minimum_detectable_effect=0.05, alpha=0.05, power=0.80,
        variance=0.01,
    )
    p_default = PowerInputs(
        baseline_rate=0.5, minimum_detectable_effect=0.05, alpha=0.05, power=0.80,
    )
    assert power_derived_n_required(p_low_var) < power_derived_n_required(p_default)


def test_power_rejects_invalid_alpha():
    with pytest.raises(ValueError):
        power_derived_n_required(
            PowerInputs(baseline_rate=0.5, minimum_detectable_effect=0.05, alpha=0.0, power=0.80)
        )
    with pytest.raises(ValueError):
        power_derived_n_required(
            PowerInputs(baseline_rate=0.5, minimum_detectable_effect=0.05, alpha=1.0, power=0.80)
        )


def test_power_rejects_invalid_mde():
    with pytest.raises(ValueError):
        power_derived_n_required(
            PowerInputs(baseline_rate=0.5, minimum_detectable_effect=0.0, alpha=0.05, power=0.80)
        )
