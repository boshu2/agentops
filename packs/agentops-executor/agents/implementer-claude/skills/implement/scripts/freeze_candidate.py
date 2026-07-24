#!/usr/bin/env python3
"""Implement-side exact intent consumer and before/final manifest builder."""

from __future__ import annotations

import argparse
import importlib.util
import json
from pathlib import Path
import sys


def find_kernel() -> Path:
    script = Path(__file__).resolve()
    direct = script.parents[2] / "validate" / "scripts" / "kernel_v3.py"
    if direct.is_file():
        return direct
    for ancestor in script.parents:
        projected = (
            ancestor
            / "agents"
            / "validator"
            / "skills"
            / "validate"
            / "scripts"
            / "kernel_v3.py"
        )
        if projected.is_file():
            return projected
    raise FileNotFoundError("cannot locate the projected validate kernel")


KERNEL_PATH = find_kernel()
SPEC = importlib.util.spec_from_file_location("agentops_kernel_v3", KERNEL_PATH)
assert SPEC and SPEC.loader
kernel = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(kernel)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--intent-snapshot", required=True)
    parser.add_argument("--expected-intent-digest", required=True)
    parser.add_argument("--root", required=True)
    parser.add_argument("--observation-roots", required=True)
    parser.add_argument("--exclude", action="append", default=[])
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    try:
        kernel.consume_intent_snapshot(
            Path(args.intent_snapshot),
            args.expected_intent_digest,
        )
        roots = kernel.load_json(Path(args.observation_roots))
        if set(roots) != {"observation_roots"}:
            raise kernel.ContractError(
                "observation roots file must contain only observation_roots"
            )
        manifest = kernel.build_manifest_v2(
            Path(args.root),
            roots["observation_roots"],
            args.exclude,
        )
        kernel.atomic_write_json(Path(args.output), manifest)
        return 0
    except (kernel.ContractError, OSError, json.JSONDecodeError) as exc:
        print(f"implement-freeze-candidate: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
