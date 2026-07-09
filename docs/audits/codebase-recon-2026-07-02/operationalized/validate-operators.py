#!/usr/bin/env python3
"""Validate the operationalized bundle: marker integrity, operator-card completeness,
and anchor resolution. Exit 0 = PASS, 1 = FAIL. Deterministic; no external deps.

Enforces the operationalizing-expertise invariants:
- kernel + operators are marker-bounded (deterministic parsing)
- every operator card carries TRIGGER / ACTION / AVOIDS / EVIDENCE
- every [[Qn]] anchor cited anywhere resolves to a definition in quote-bank.md
- disputed/unproven claims stay OUT of the KERNEL:START..END block
"""
import re
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
QUOTE_BANK = HERE / "quote-bank.md"
KERNEL = HERE / "triangulated-kernel.md"
OPERATORS = HERE / "operator-library.md"

REQUIRED_CARD_FIELDS = ("TRIGGER", "ACTION", "AVOIDS", "EVIDENCE")
errors: list[str] = []


def read(p: Path) -> str:
    if not p.exists():
        errors.append(f"missing file: {p.name}")
        return ""
    return p.read_text(encoding="utf-8")


def between(text: str, start: str, end: str) -> str | None:
    m = re.search(re.escape(start) + r"(.*?)" + re.escape(end), text, re.S)
    return m.group(1) if m else None


def main() -> int:
    qb = read(QUOTE_BANK)
    kernel = read(KERNEL)
    ops = read(OPERATORS)

    # 1. quote-bank defines anchors Q1.. as "- **Qn** —"
    defined = set(re.findall(r"\*\*(Q\d+)\*\*", qb))
    if not defined:
        errors.append("quote-bank.md defines no Qn anchors")

    # 2. every [[Qn]] reference across kernel+operators resolves
    for fname, txt in (("triangulated-kernel.md", kernel), ("operator-library.md", ops)):
        for ref in set(re.findall(r"\[\[(Q\d+)\]\]", txt)):
            if ref not in defined:
                errors.append(f"{fname}: [[{ref}]] has no definition in quote-bank.md")

    # 3. kernel is marker-bounded
    kbody = between(kernel, "<!-- KERNEL:START -->", "<!-- KERNEL:END -->")
    if kbody is None:
        errors.append("triangulated-kernel.md: missing KERNEL:START/END markers")
    else:
        # disputed content must not leak into the consensus kernel
        if re.search(r"\bunproven\b|\bDISPUTED\b|flywheel.{0,20}moat", kbody, re.I):
            errors.append("KERNEL block contains disputed/unproven language (must be quarantined below END)")

    # 4. operators are marker-bounded and each card has required fields
    obody = between(ops, "<!-- OPERATORS:START -->", "<!-- OPERATORS:END -->")
    if obody is None:
        errors.append("operator-library.md: missing OPERATORS:START/END markers")
    else:
        cards = re.split(r"\n## (OP-\d+[^\n]*)\n", obody)
        # cards = [pre, title1, body1, title2, body2, ...]
        pairs = list(zip(cards[1::2], cards[2::2]))
        if not pairs:
            errors.append("operator-library.md: no OP-n cards found")
        for title, body in pairs:
            for field in REQUIRED_CARD_FIELDS:
                if f"**{field}:**" not in body:
                    errors.append(f"{title.strip()}: missing required field {field}")

    n_ops = len(re.findall(r"\n## OP-\d+", ops))
    n_anchors = len(defined)
    if errors:
        print("VALIDATE-OPERATORS: FAIL")
        for e in errors:
            print(f"  - {e}")
        return 1
    print(f"VALIDATE-OPERATORS: PASS ({n_ops} operators, {n_anchors} anchors, all references resolve)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
