"""§6.5 bootstrap_seed derivation.

Spec (SCHEMA.md §6.5):

    bootstrap_seed = sha256(
        suite_id || "::" ||
        ",".join(sorted(arm_ids)) || "::" ||
        paired_sample_ids_hash || "::" ||
        json.dumps(decision_rule, sort_keys=True)
    )[:16] then big-endian to int64

The seed feeds numpy.random.PCG64 — never numpy.random.seed (mtrand) and
never Python's random module. PCG64 is reproducible across numpy versions
1.22+ for the same seed.
"""
from __future__ import annotations

import hashlib
import json
from typing import Iterable


def derive_bootstrap_seed(
    suite_id: str,
    arm_ids: Iterable[str],
    paired_sample_ids_hash: str,
    decision_rule: dict,
) -> int:
    """Derive a deterministic int64 seed from Suite identity.

    Returns a non-negative int that fits in int64 (top bit cleared) so it
    can be passed straight to numpy.random.PCG64 without overflow concerns.
    """
    if not suite_id:
        raise ValueError("derive_bootstrap_seed: empty suite_id")
    if not paired_sample_ids_hash:
        raise ValueError("derive_bootstrap_seed: empty paired_sample_ids_hash")

    arms_str = ",".join(sorted(arm_ids))
    rule_str = json.dumps(decision_rule, sort_keys=True, separators=(",", ":"))
    payload = "::".join([suite_id, arms_str, paired_sample_ids_hash, rule_str])

    digest = hashlib.sha256(payload.encode("utf-8")).digest()
    # First 8 bytes (64 bits), big-endian, mask top bit so it's non-negative.
    seed = int.from_bytes(digest[:8], byteorder="big", signed=False)
    return seed & 0x7FFFFFFFFFFFFFFF
