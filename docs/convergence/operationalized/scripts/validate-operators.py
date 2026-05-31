#!/usr/bin/env python3
"""Validate operator cards have all required fields, valid anchors, and an enforcement verdict.

Exit non-zero on any failure. Pure stdlib.
"""
from __future__ import annotations
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
LIB = ROOT / "corpus" / "specs" / "operator_library.md"
QB = ROOT / "corpus" / "quote_bank" / "quote_bank.md"

REQUIRED = [
    "**Definition**",
    "**When-to-Use Triggers**",
    "**Failure Modes**",
    "**Prompt Module**",
    "**Canonical tag**",
    "**Quote-bank anchors**",
    "**AgentOps-Enforcement**",  # local extension: the audit bridge
]
VALID_VERDICTS = ("ENFORCED", "PARTIAL", "GAP")


def main() -> int:
    errs: list[str] = []
    if not LIB.is_file():
        print("FAIL: missing operator_library.md")
        return 1
    if not QB.is_file():
        print("FAIL: missing quote_bank.md")
        return 1

    known_anchors = {int(n) for n in re.findall(r"^## §(\d+) —", QB.read_text(encoding="utf-8"), re.M)}
    text = LIB.read_text(encoding="utf-8")

    # Operator cards are '### Name' under the library (skip the doc's own H3 in preamble by requiring fields)
    cards = re.findall(r"^### (.+?)\n(.*?)(?=^### |\Z)", text, re.M | re.S)
    operators = [(name, body) for name, body in cards if "**Definition**" in body]
    if not operators:
        print("FAIL: no operator cards found")
        return 1

    for name, body in operators:
        for field in REQUIRED:
            if field not in body:
                errs.append(f"{name}: missing field {field}")
        # Prompt module fenced block present
        if "~~~" not in body:
            errs.append(f"{name}: Prompt Module fenced block (~~~) missing")
        # Enforcement verdict valid
        m = re.search(r"\*\*AgentOps-Enforcement\*\*:\s*(\w+)", body)
        if m and m.group(1) not in VALID_VERDICTS:
            errs.append(f"{name}: enforcement verdict '{m.group(1)}' not in {VALID_VERDICTS}")
        # Anchors reference real quotes
        am = re.search(r"\*\*Quote-bank anchors\*\*:\s*(.+)", body)
        if am:
            refs = {int(x) for x in re.findall(r"§(\d+)", am.group(1))}
            missing = refs - known_anchors
            if not refs:
                errs.append(f"{name}: no §anchors cited")
            if missing:
                errs.append(f"{name}: anchors not in quote bank: {sorted(missing)}")

    if errs:
        print(f"FAIL operator validation ({len(errs)} error(s)):")
        for e in errs:
            print(f"  - {e}")
        return 1
    print(f"OK operator validation ({len(operators)} operators)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
