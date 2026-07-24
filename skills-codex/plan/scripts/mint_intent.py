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


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", required=True, help="file path or - for stdin")
    parser.add_argument("--intent-dir", required=True)
    parser.add_argument("--expected-digest")
    args = parser.parse_args()
    try:
        payload = (
            sys.stdin.buffer.read()
            if args.source == "-"
            else Path(args.source).read_bytes()
        )
        path, digest, existed = kernel.mint_intent_snapshot(
            payload,
            Path(args.intent_dir),
            expected_digest=args.expected_digest,
        )
        print(
            json.dumps(
                {
                    "intent_ref": str(path),
                    "intent_digest": digest,
                    "idempotent": existed,
                },
                sort_keys=True,
            )
        )
        return 0
    except (kernel.ContractError, OSError) as exc:
        print(f"plan-mint-intent: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
