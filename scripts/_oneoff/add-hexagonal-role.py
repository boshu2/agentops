#!/usr/bin/env python3
"""Backfill `hexagonal_role` on every skills/<name>/SKILL.md frontmatter.

Reads the classification table from
`.agents/research/2026-05-12-ddd-hexagonal-research.md` Section (d), maps each
skill slug to one of the five hexagonal_role enum values
(`domain`, `driving-adapter`, `driven-adapter`, `supporting`, `generic`),
and writes the field into each SKILL.md's YAML frontmatter if absent.

Idempotent: running the script twice produces zero git diff.

Body content after the frontmatter is preserved byte-for-byte.

Usage:
    python3 scripts/_oneoff/add-hexagonal-role.py
"""

from __future__ import annotations

import io
import re
import sys
from pathlib import Path
from typing import Iterable

import yaml


REPO_ROOT = Path(__file__).resolve().parents[2]
SKILLS_ROOT = REPO_ROOT / "skills"
RESEARCH_PATH = (
    REPO_ROOT / ".agents" / "research" / "2026-05-12-ddd-hexagonal-research.md"
)

ENUM = {"domain", "driving-adapter", "driven-adapter", "supporting", "generic"}

# Research-table classification words → schema enum values.
CLASSIFICATION_MAP = {
    "DOMAIN": "domain",
    "DRIVING ADAPTER": "driving-adapter",
    "DRIVEN ADAPTER": "driven-adapter",
    "SUPPORTING": "supporting",
    "GENERIC": "generic",
}


def parse_research_table(text: str) -> dict[str, str]:
    """Parse Section (d) classification table.

    The table has duplicate rows for some skills (e.g. `crank` appears
    multiple times). We accept the first occurrence; later rows that disagree
    are ignored (the research-doc's narrative makes the canonical row first).
    """
    section_marker = "## (d) Skill Classification"
    next_section = "## (e)"
    start = text.find(section_marker)
    if start < 0:
        return {}
    end = text.find(next_section, start)
    section = text[start:end] if end > 0 else text[start:]

    out: dict[str, str] = {}
    row_re = re.compile(r"^\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|")
    for line in section.splitlines():
        if not line.startswith("|"):
            continue
        m = row_re.match(line)
        if not m:
            continue
        slug, klass = m.group(1).strip(), m.group(2).strip()
        # Skip header / separator rows.
        if slug.lower() in {"skill", "-----", ""}:
            continue
        if set(slug) <= set("-"):
            continue
        # Handle "dependencies → deps"-style relabels.
        if "→" in slug:
            slug = slug.split("→")[-1].strip()
        if klass not in CLASSIFICATION_MAP:
            continue
        role = CLASSIFICATION_MAP[klass]
        out.setdefault(slug, role)
    return out


# Canonical mapping for slugs that exist in skills/ but whose research-table
# entry is missing, duplicated with conflict, or under a different name.
# Built from .agents/research/2026-05-12-ddd-hexagonal-research.md §(d) and the
# plan's category counts. Slugs not present in `skills/` are simply unused.
OVERRIDES: dict[str, str] = {
    # Research lists `product` only implicitly via repo `PRODUCT.md`; the skill
    # exists and is unambiguously a domain artifact (product fit / mission).
    "product": "domain",
    # `using-agentops` → research table places it under GENERIC (education).
    "using-agentops": "generic",
    # `standards` → research-doc narrative names it as domain (loaded JIT by
    # all skills) but it's absent from the table. Plan §Issue #3 explicitly
    # lists `standards` under domain. Source: .agents/plans/2026-05-12-ddd-
    # hexagonal-plan.md L172.
    "standards": "domain",
}


def list_skill_dirs() -> list[Path]:
    return sorted(
        p.parent
        for p in SKILLS_ROOT.glob("*/SKILL.md")
        if p.is_file()
    )


FRONTMATTER_RE = re.compile(
    r"\A(---\s*\n)(.*?\n)(---\s*\n)(.*)\Z",
    re.DOTALL,
)


def split_frontmatter(text: str) -> tuple[str, str, str, str] | None:
    """Return (open_fence, fm_text, close_fence, body) or None."""
    m = FRONTMATTER_RE.match(text)
    if not m:
        return None
    return m.group(1), m.group(2), m.group(3), m.group(4)


def add_role_to_yaml(fm_text: str, role: str) -> tuple[str, bool, bool]:
    """Return (new_fm_text, changed, was_present).

    Re-serializes with PyYAML safe_dump preserving key order via sort_keys=False.
    """
    data = yaml.safe_load(fm_text)
    if not isinstance(data, dict):
        raise ValueError("frontmatter did not parse as a mapping")
    if "hexagonal_role" in data:
        return fm_text, False, True
    # Build a new ordered mapping: insert hexagonal_role after `practices` if
    # present, otherwise after `description`, otherwise at the end. This keeps
    # the file readable but is deterministic.
    items = list(data.items())
    keys = [k for k, _ in items]
    anchor = None
    for cand in ("practices", "description", "name"):
        if cand in keys:
            anchor = cand
            break
    new_items: list[tuple[str, object]] = []
    inserted = False
    for k, v in items:
        new_items.append((k, v))
        if not inserted and k == anchor:
            new_items.append(("hexagonal_role", role))
            inserted = True
    if not inserted:
        new_items.append(("hexagonal_role", role))
    new_data = dict(new_items)
    buf = io.StringIO()
    yaml.safe_dump(
        new_data,
        buf,
        sort_keys=False,
        allow_unicode=True,
        default_flow_style=False,
    )
    new_text = buf.getvalue()
    # safe_dump appends a final newline; ensure no doubling.
    if not new_text.endswith("\n"):
        new_text += "\n"
    return new_text, True, False


def process_one(skill_dir: Path, role: str) -> tuple[str, bool]:
    """Return (status, changed). status in {classified, already-set, unmapped}."""
    skill_md = skill_dir / "SKILL.md"
    raw = skill_md.read_text(encoding="utf-8")
    parts = split_frontmatter(raw)
    if parts is None:
        raise ValueError(f"{skill_md}: no YAML frontmatter detected")
    open_fence, fm_text, close_fence, body = parts
    new_fm_text, changed, was_present = add_role_to_yaml(fm_text, role)
    if not changed:
        return "already-set", False
    new_raw = open_fence + new_fm_text + close_fence + body
    if new_raw != raw:
        skill_md.write_text(new_raw, encoding="utf-8")
    return "classified", True


def main(argv: list[str]) -> int:
    if not RESEARCH_PATH.exists():
        print(f"error: research file not found: {RESEARCH_PATH}", file=sys.stderr)
        return 1
    research_text = RESEARCH_PATH.read_text(encoding="utf-8")
    table = parse_research_table(research_text)

    skill_dirs = list_skill_dirs()
    skill_slugs = {p.name for p in skill_dirs}

    # Merge overrides on top of parsed table.
    classification: dict[str, str] = {**table, **OVERRIDES}

    # Diagnostics.
    table_only = sorted(set(classification.keys()) - skill_slugs)
    if table_only:
        print(
            "note: research table mentions slugs not present in skills/: "
            + ", ".join(table_only)
        )

    classified = 0
    already_set = 0
    unmapped_slugs: list[str] = []

    for skill_dir in skill_dirs:
        slug = skill_dir.name
        role = classification.get(slug)
        if role is None:
            unmapped_slugs.append(slug)
            continue
        if role not in ENUM:
            print(
                f"error: {slug} mapped to non-enum role '{role}'", file=sys.stderr
            )
            return 1
        try:
            status, _changed = process_one(skill_dir, role)
        except Exception as exc:  # noqa: BLE001
            print(f"error: processing {slug}: {exc}", file=sys.stderr)
            return 1
        if status == "classified":
            classified += 1
        elif status == "already-set":
            already_set += 1

    total = len(skill_dirs)
    if total != 77:
        print(
            f"note: filesystem skill count = {total} (plan expected 78; "
            f"SHARED_TASK_NOTES says 77)"
        )

    print(
        "add-hexagonal-role: classified {c}, already-set {a}, unmapped {u}".format(
            c=classified, a=already_set, u=len(unmapped_slugs)
        )
    )
    if unmapped_slugs:
        print("unmapped: " + ", ".join(unmapped_slugs))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
