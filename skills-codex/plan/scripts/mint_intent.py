#!/usr/bin/env python3
"""Plan-side adapter that mints one exact intent snapshot and nothing else."""

from __future__ import annotations

import argparse
import importlib.util
import json
from pathlib import Path
import sys


KERNEL_PATH = Path(__file__).parents[2] / "validate" / "scripts" / "kernel_v3.py"
SPEC = importlib.util.spec_from_file_location("agentops_kernel_v3", KERNEL_PATH)
assert SPEC and SPEC.loader
kernel = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(kernel)


def mint_intent_identity(
    payload: bytes,
    *,
    intent_dir: Path,
    intent_ref_root: str,
    expected_digest: str | None = None,
) -> dict[str, str | int]:
    """Mint once and return the only packet later RPI phases may consume."""
    path, digest, _existed = kernel.mint_intent_snapshot(
        payload,
        intent_dir,
        expected_digest=expected_digest,
    )
    reference_root = kernel.normalize_rel(intent_ref_root)
    return {
        "intent_ref": f"{reference_root}/{path.name}",
        "intent_digest": digest,
        "byte_length": len(payload),
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", required=True, help="file path or - for stdin")
    parser.add_argument("--intent-dir", required=True)
    parser.add_argument(
        "--intent-ref-root",
        default=".agents/ao/intents",
        help="repository-relative reference root carried to later phases",
    )
    parser.add_argument("--expected-digest")
    args = parser.parse_args()
    try:
        payload = (
            sys.stdin.buffer.read()
            if args.source == "-"
            else Path(args.source).read_bytes()
        )
        identity = mint_intent_identity(
            payload,
            intent_dir=Path(args.intent_dir),
            intent_ref_root=args.intent_ref_root,
            expected_digest=args.expected_digest,
        )
        print(json.dumps(identity, sort_keys=True))
        return 0
    except (kernel.ContractError, OSError) as exc:
        print(f"plan-mint-intent: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
