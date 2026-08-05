"""Tests for §6.5 bootstrap_seed derivation."""
import json
import hashlib

import pytest

from _stats.seed import derive_bootstrap_seed


def test_seed_deterministic():
    """Same inputs MUST produce the same seed across calls."""
    args = dict(
        suite_id="test-suite",
        arm_ids=["ms:b", "ms:a"],
        paired_sample_ids_hash="sha256:" + "0" * 64,
        decision_rule={"kind": "ci_excludes_zero", "confidence": 0.95},
    )
    a = derive_bootstrap_seed(**args)
    b = derive_bootstrap_seed(**args)
    assert a == b
    assert isinstance(a, int)
    assert 0 <= a <= 0x7FFFFFFFFFFFFFFF


def test_seed_arm_order_invariant():
    """Sorting arm_ids inside the function MUST make argument order irrelevant."""
    a = derive_bootstrap_seed(
        suite_id="s",
        arm_ids=["ms:a", "ms:b"],
        paired_sample_ids_hash="sha256:00",
        decision_rule={"kind": "ci_excludes_zero"},
    )
    b = derive_bootstrap_seed(
        suite_id="s",
        arm_ids=["ms:b", "ms:a"],
        paired_sample_ids_hash="sha256:00",
        decision_rule={"kind": "ci_excludes_zero"},
    )
    assert a == b


def test_seed_decision_rule_keys_sorted():
    """Decision rule key order must NOT affect the seed."""
    a = derive_bootstrap_seed(
        suite_id="s",
        arm_ids=["a"],
        paired_sample_ids_hash="sha256:00",
        decision_rule={"kind": "min_delta", "min_delta": 0.02, "confidence": 0.95},
    )
    b = derive_bootstrap_seed(
        suite_id="s",
        arm_ids=["a"],
        paired_sample_ids_hash="sha256:00",
        decision_rule={"confidence": 0.95, "min_delta": 0.02, "kind": "min_delta"},
    )
    assert a == b


def test_seed_changes_when_suite_id_changes():
    a = derive_bootstrap_seed(
        suite_id="s1",
        arm_ids=["a", "b"],
        paired_sample_ids_hash="sha256:00",
        decision_rule={"kind": "ci_excludes_zero"},
    )
    b = derive_bootstrap_seed(
        suite_id="s2",
        arm_ids=["a", "b"],
        paired_sample_ids_hash="sha256:00",
        decision_rule={"kind": "ci_excludes_zero"},
    )
    assert a != b


def test_seed_changes_when_paired_sample_ids_hash_changes():
    a = derive_bootstrap_seed(
        suite_id="s",
        arm_ids=["a"],
        paired_sample_ids_hash="sha256:00",
        decision_rule={"kind": "ci_excludes_zero"},
    )
    b = derive_bootstrap_seed(
        suite_id="s",
        arm_ids=["a"],
        paired_sample_ids_hash="sha256:11",
        decision_rule={"kind": "ci_excludes_zero"},
    )
    assert a != b


def test_seed_matches_published_recipe():
    """Cross-check against a hand-computed reference value.

    Recipe per §6.5:
        sha256("s::a,b::sha256:00::{\"kind\":\"ci_excludes_zero\"}")[:8]
    """
    suite_id = "s"
    arms = "a,b"
    psh = "sha256:00"
    rule = json.dumps({"kind": "ci_excludes_zero"}, sort_keys=True, separators=(",", ":"))
    payload = f"{suite_id}::{arms}::{psh}::{rule}"
    expected = int.from_bytes(
        hashlib.sha256(payload.encode()).digest()[:8], byteorder="big", signed=False
    ) & 0x7FFFFFFFFFFFFFFF

    got = derive_bootstrap_seed(
        suite_id="s",
        arm_ids=["a", "b"],
        paired_sample_ids_hash="sha256:00",
        decision_rule={"kind": "ci_excludes_zero"},
    )
    assert got == expected


def test_seed_rejects_empty_suite_id():
    with pytest.raises(ValueError):
        derive_bootstrap_seed(
            suite_id="",
            arm_ids=["a"],
            paired_sample_ids_hash="sha256:00",
            decision_rule={"kind": "ci_excludes_zero"},
        )


def test_seed_rejects_empty_paired_sample_ids_hash():
    with pytest.raises(ValueError):
        derive_bootstrap_seed(
            suite_id="s",
            arm_ids=["a"],
            paired_sample_ids_hash="",
            decision_rule={"kind": "ci_excludes_zero"},
        )
