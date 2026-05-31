#!/usr/bin/env python3
"""Validate the corpus structure and quote bank against the operationalizing-expertise contract.

Exit non-zero on any failure. Pure stdlib.
"""
from __future__ import annotations
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
CORPUS = ROOT / "corpus"
TAXONOMY = {
    "velocity", "verification", "safety", "autonomy", "evaluation",
    "provenance", "context", "abstraction", "actuation", "governance",
}
MIN_PRIMARY_BYTES = 2500  # a reference stub (citation + outline + fair-use excerpt) is valid; full verbatim text is not re-hosted in a public repo
MIN_QUOTE_LEN = 40
MAX_QUOTE_LEN = 900


def fail(errs: list[str], msg: str) -> None:
    errs.append(msg)


def main() -> int:
    errs: list[str] = []

    # Required directories
    for d in ("primary_sources", "quote_bank"):
        if not (CORPUS / d).is_dir():
            fail(errs, f"missing required dir: corpus/{d}")

    # Primary sources non-empty and above minimum size
    primaries = list((CORPUS / "primary_sources").glob("*.md")) if (CORPUS / "primary_sources").is_dir() else []
    if not primaries:
        fail(errs, "no primary sources found")
    for p in primaries:
        if p.stat().st_size < MIN_PRIMARY_BYTES:
            fail(errs, f"primary source too small ({p.stat().st_size}B): {p.name}")

    # Quote bank
    qb = CORPUS / "quote_bank" / "quote_bank.md"
    if not qb.is_file():
        fail(errs, "missing corpus/quote_bank/quote_bank.md")
        return report(errs)

    text = qb.read_text(encoding="utf-8")
    # Split into entries by '## §N — ...'
    entries = re.findall(r"^## §(\d+) —[^\n]*\n(.*?)(?=^## §|\Z)", text, re.M | re.S)
    if not entries:
        fail(errs, "quote bank has no §-anchored entries")
        return report(errs)

    nums = [int(n) for n, _ in entries]
    # Sequential, no gaps, starting at 1
    expected = list(range(1, len(nums) + 1))
    if nums != expected:
        fail(errs, f"quote anchors not sequential 1..N: got {nums}")

    for n, body in entries:
        quote = re.search(r'^>\s*"(.+?)"\s*$', body, re.M | re.S)
        source = re.search(r"^—\s*Source:", body, re.M)
        tags = re.search(r"^Tags:\s*(.+)$", body, re.M)
        if not quote:
            fail(errs, f"§{n}: missing quote line (>\"...\")")
        else:
            qlen = len(quote.group(1))
            if qlen < MIN_QUOTE_LEN:
                fail(errs, f"§{n}: quote too short ({qlen} chars)")
            if qlen > MAX_QUOTE_LEN:
                fail(errs, f"§{n}: quote too long ({qlen} chars)")
        if not source:
            fail(errs, f"§{n}: missing source line (— Source:)")
        if not tags:
            fail(errs, f"§{n}: missing Tags line")
        else:
            tagset = {t.strip() for t in tags.group(1).split(",") if t.strip()}
            bad = tagset - TAXONOMY
            if bad:
                fail(errs, f"§{n}: tags outside taxonomy: {sorted(bad)}")

    return report(errs)


def report(errs: list[str]) -> int:
    if errs:
        print(f"FAIL corpus validation ({len(errs)} error(s)):")
        for e in errs:
            print(f"  - {e}")
        return 1
    print("OK corpus validation")
    return 0


if __name__ == "__main__":
    sys.exit(main())
