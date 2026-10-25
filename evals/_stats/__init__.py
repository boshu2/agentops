"""§6.5 statistical contract for eval-substrate Suite verdicts.

Implements paired cluster-bootstrap with PCG64 RNG, deterministic
bootstrap_seed derivation, percentile-method CIs, bootstrap_inputs_hash,
power-derived n_required, and the 5-verdict decision tree per §6.5 of
the eval-substrate schema (an operator-local spec outside this repository,
kept at ~/.agents/evals/SCHEMA.md).

This package is the single source of truth for "Suite verdict computation."
Callers outside Python use its CLI entrypoint (cli.py) rather than a second
implementation.
"""

from .seed import derive_bootstrap_seed
from .inputs import (
    bootstrap_inputs_hash,
    canonical_inputs_json,
    paired_sample_ids_hash,
    BootstrapInput,
)
from .bootstrap import (
    paired_cluster_bootstrap,
    BootstrapResult,
)
from .verdict import (
    compute_verdict,
    Verdict,
    VerdictKind,
)
from .power import (
    power_derived_n_required,
    PowerInputs,
)

__all__ = [
    "derive_bootstrap_seed",
    "bootstrap_inputs_hash",
    "canonical_inputs_json",
    "paired_sample_ids_hash",
    "BootstrapInput",
    "paired_cluster_bootstrap",
    "BootstrapResult",
    "compute_verdict",
    "Verdict",
    "VerdictKind",
    "power_derived_n_required",
    "PowerInputs",
]
