"""Tests for §6.5 bootstrap_inputs canonicalization + hashing."""
import json
import os
import tempfile

from _stats.inputs import (
    BootstrapInput,
    bootstrap_inputs_hash,
    canonical_inputs_json,
    paired_sample_ids_hash,
    load_bootstrap_inputs,
)


def test_canonical_inputs_json_sort_order():
    """Inputs sorted by (sample_id, seed) regardless of input order."""
    rows = [
        BootstrapInput(sample_id="s2", seed=1, score_arm_a=1.0, score_arm_b=0.5),
        BootstrapInput(sample_id="s1", seed=2, score_arm_a=0.3, score_arm_b=0.4),
        BootstrapInput(sample_id="s1", seed=1, score_arm_a=0.7, score_arm_b=0.8),
    ]
    out = json.loads(canonical_inputs_json(rows))
    assert [r["sample_id"] for r in out] == ["s1", "s1", "s2"]
    assert [r["seed"] for r in out] == [1, 2, 1]


def test_canonical_inputs_json_keys_sorted():
    """Per-row keys serialized in alphabetical order."""
    rows = [BootstrapInput(sample_id="s1", seed=1, score_arm_a=0.5, score_arm_b=0.5)]
    text = canonical_inputs_json(rows).decode()
    assert text.startswith('[{"sample_id"')
    # All four keys present, alphabetically: sample_id, score_arm_a, score_arm_b, seed
    assert text == '[{"sample_id":"s1","score_arm_a":0.5,"score_arm_b":0.5,"seed":1}]'


def test_bootstrap_inputs_hash_deterministic():
    rows = [
        BootstrapInput(sample_id=f"s{i}", seed=1, score_arm_a=0.1 * i, score_arm_b=0.2 * i)
        for i in range(20)
    ]
    h1 = bootstrap_inputs_hash(rows)
    h2 = bootstrap_inputs_hash(reversed(rows))
    assert h1 == h2
    assert h1.startswith("sha256:")


def test_paired_sample_ids_hash_dedup_and_sort():
    """Even with duplicates and unsorted input, the hash is stable."""
    rows_a = [
        BootstrapInput("s1", 1, 0, 0),
        BootstrapInput("s2", 1, 0, 0),
        BootstrapInput("s1", 2, 0, 0),  # dup sample_id
    ]
    rows_b = [
        BootstrapInput("s2", 5, 0, 0),
        BootstrapInput("s1", 9, 0, 0),
    ]
    assert paired_sample_ids_hash(rows_a) == paired_sample_ids_hash(rows_b)


def test_load_bootstrap_inputs_round_trip():
    rows = [
        BootstrapInput(sample_id="s1", seed=1, score_arm_a=0.5, score_arm_b=0.4),
        BootstrapInput(sample_id="s2", seed=2, score_arm_a=0.3, score_arm_b=0.7),
    ]
    with tempfile.NamedTemporaryFile("wb", suffix=".json", delete=False) as f:
        f.write(canonical_inputs_json(rows))
        path = f.name
    try:
        loaded = load_bootstrap_inputs(path)
        assert len(loaded) == 2
        assert loaded[0].sample_id == "s1"
        assert loaded[1].sample_id == "s2"
        assert loaded[0].score_arm_a == 0.5
    finally:
        os.unlink(path)
