"""§6.5 statistical contract for eval-substrate Suite verdicts.

Implements paired cluster-bootstrap with PCG64 RNG, deterministic
bootstrap_seed derivation, percentile-method CIs, bootstrap_inputs_hash,
power-derived n_required, and the 5-verdict decision tree per
~/.agents/evals/SCHEMA.md §6.5.

This package is the single source of truth for "Suite verdict computation."
Both `ao eval suite verdict` (Go CLI) and `ao eval doctor` (Day-5 verifier)
shell out to its CLI entrypoint to avoid two implementations diverging.
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
