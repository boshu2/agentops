"""§6.5 bootstrap_inputs canonicalization + hashing.

Spec (SCHEMA.md §6.5):

    bootstrap_inputs_hash = sha256(
        canonical-JSON-serialized
            [{sample_id, seed, score_arm_a, score_arm_b}, ...]
        sorted by (sample_id, seed)
    )

    paired_sample_ids_hash = sha256(sorted(sample_ids))   # set, deduped

`canonical-JSON` here means: sorted keys, no whitespace, ensure_ascii=False,
exact-form numbers (we trust callers to supply Python floats; canonicalization
serializes them via json.dumps default).

Stored input enables two re-runs on the same data to produce bit-exact CIs.
"""
from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass
from typing import Iterable, List


@dataclass(frozen=True)
class BootstrapInput:
    """One per-(sample, seed) draw, with both arm scores already produced."""
    sample_id: str
    seed: int
    score_arm_a: float
    score_arm_b: float


def canonical_inputs_json(inputs: Iterable[BootstrapInput]) -> bytes:
    """Return canonical JSON bytes for the bootstrap inputs.

    Sort key: (sample_id, seed). Keys within each row are sorted by json.dumps.
    """
    rows = sorted(
        (
            {
                "sample_id": x.sample_id,
                "seed": int(x.seed),
                "score_arm_a": float(x.score_arm_a),
                "score_arm_b": float(x.score_arm_b),
            }
            for x in inputs
        ),
        key=lambda r: (r["sample_id"], r["seed"]),
    )
    text = json.dumps(rows, sort_keys=True, separators=(",", ":"), ensure_ascii=False)
    return text.encode("utf-8")


def bootstrap_inputs_hash(inputs: Iterable[BootstrapInput]) -> str:
    """sha256:HEX over canonical-JSON-serialized inputs."""
    canon = canonical_inputs_json(inputs)
    return "sha256:" + hashlib.sha256(canon).hexdigest()


def paired_sample_ids_hash(inputs: Iterable[BootstrapInput]) -> str:
    """sha256:HEX over the sorted distinct set of sample_ids.

    This must match the manifest's paired_sample_ids_hash so the bootstrap
    inputs and the manifest agree on the cluster set.
    """
    ids = sorted({x.sample_id for x in inputs})
    text = json.dumps(ids, sort_keys=True, separators=(",", ":"), ensure_ascii=False)
    return "sha256:" + hashlib.sha256(text.encode("utf-8")).hexdigest()


def load_bootstrap_inputs(path: str) -> List[BootstrapInput]:
    """Load bootstrap inputs from a canonical-JSON file."""
    with open(path, "rb") as f:
        raw = f.read()
    rows = json.loads(raw)
    return [
        BootstrapInput(
            sample_id=r["sample_id"],
            seed=int(r["seed"]),
            score_arm_a=float(r["score_arm_a"]),
            score_arm_b=float(r["score_arm_b"]),
        )
        for r in rows
    ]
