#!/usr/bin/env python3
"""Extract the marker-bounded triangulated kernel; validate markers + minimum counts.

Prints the kernel body to stdout on success. Exit non-zero if markers absent or counts below minimum.
"""
from __future__ import annotations
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
KERNEL = ROOT / "corpus" / "specs" / "triangulated_kernel.md"
MIN_AXIOMS = 10
MIN_OPERATORS = 8


def main() -> int:
    if not KERNEL.is_file():
        print("FAIL: missing triangulated_kernel.md", file=sys.stderr)
        return 1
    text = KERNEL.read_text(encoding="utf-8")
    m = re.search(
        r"<!--\s*TRIANGULATED_KERNEL_START\s+(v\d+\.\d+)\s*-->(.*?)<!--\s*TRIANGULATED_KERNEL_END\s+(v\d+\.\d+)\s*-->",
        text, re.S,
    )
    if not m:
        print("FAIL: kernel START/END markers absent or malformed", file=sys.stderr)
        return 1
    vstart, body, vend = m.group(1), m.group(2), m.group(3)
    if vstart != vend:
        print(f"FAIL: version mismatch {vstart} != {vend}", file=sys.stderr)
        return 1

    axioms = re.findall(r"^\s*-\s*\*\*A\d+\s*—", body, re.M)
    operators = re.findall(r"`[A-Z][A-Za-z-]+`\s*\(A", body)
    if len(axioms) < MIN_AXIOMS:
        print(f"FAIL: only {len(axioms)} axioms (min {MIN_AXIOMS})", file=sys.stderr)
        return 1
    if len(operators) < MIN_OPERATORS:
        print(f"FAIL: only {len(operators)} operators (min {MIN_OPERATORS})", file=sys.stderr)
        return 1

    sys.stderr.write(f"OK kernel {vstart}: {len(axioms)} axioms, {len(operators)} operators\n")
    sys.stdout.write(body.strip() + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
