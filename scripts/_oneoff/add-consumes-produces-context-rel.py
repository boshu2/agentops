#!/usr/bin/env python3
"""Backfill `consumes`, `produces`, and `context_rel` on every
skills/<name>/SKILL.md frontmatter.

Reads the per-skill data table from the embedded YAML block in
`.agents/research/2026-05-12-skill-consumes-produces-context-rel-mapping.md`
and inserts the three fields immediately after `hexagonal_role:` (or another
anchor key if absent) in each SKILL.md frontmatter.

PyYAML reformat disclosure (per finding
`validation-gap|pyyaml-safe-dump-reformats-frontmatter-bytes`): this script
uses `yaml.safe_dump(..., sort_keys=False, default_flow_style=False)`. Inline
flow-style arrays in existing frontmatter will be reformatted to block style,
and redundant quotes around strings may be stripped. Body bytes after the
closing `---` are preserved byte-for-byte. Commit body should disclose this.

Idempotent: running the script twice produces zero `git diff` on the second
run (the script re-reads each file and short-circuits when the three fields
are already present with matching values).

Usage:
    python3 scripts/_oneoff/add-consumes-produces-context-rel.py
        [--research PATH] [--dry-run]
"""
from __future__ import annotations

import argparse
import io
import re
import sys
from pathlib import Path

import yaml


REPO_ROOT = Path(__file__).resolve().parents[2]
SKILLS_ROOT = REPO_ROOT / "skills"
DEFAULT_RESEARCH = (
    REPO_ROOT
    / ".agents"
    / "research"
    / "2026-05-12-skill-consumes-produces-context-rel-mapping.md"
)

FRONTMATTER_RE = re.compile(
    r"\A(---\s*\n)(.*?\n)(---\s*\n)(.*)\Z",
    re.DOTALL,
)
# Capture the LAST ```yaml ... ``` fenced block whose payload starts with
# `skills:` at column 0 — this is the canonical per-skill data table. Today
# there is exactly one such block in the research artifact, but anchoring on
# `skills:` future-proofs against later additions of example yaml snippets.
YAML_BLOCK_RE = re.compile(r"```yaml\n(skills:\n.*?)```", re.DOTALL)


def parse_research_yaml(path: Path) -> dict[str, dict]:
    """Return slug -> {consumes, produces, context_rel}."""
    text = path.read_text(encoding="utf-8")
    matches = list(YAML_BLOCK_RE.finditer(text))
    if not matches:
        raise SystemExit(
            f"no ```yaml block starting with `skills:` in {path}"
        )
    payload = matches[-1].group(1)
    data = yaml.safe_load(payload)
    skills = data.get("skills") if isinstance(data, dict) else None
    if not isinstance(skills, list):
        raise SystemExit(
            f"expected `skills:` list at top of YAML block in {path}"
        )
    out: dict[str, dict] = {}
    for row in skills:
        if not isinstance(row, dict):
            continue
        slug = row.get("skill")
        if not slug:
            continue
        out[slug] = {
            "consumes": list(row.get("consumes") or []),
            "produces": list(row.get("produces") or []),
            "context_rel": list(row.get("context_rel") or []),
        }
    return out


def split_frontmatter(text: str) -> tuple[str, str, str, str] | None:
    m = FRONTMATTER_RE.match(text)
    if not m:
        return None
    return m.group(1), m.group(2), m.group(3), m.group(4)


def add_fields_to_yaml(fm_text: str, row: dict) -> tuple[str, bool]:
    """Insert consumes/produces/context_rel at the canonical anchor.

    Returns (new_fm_text, changed). The function is idempotent: if all three
    fields are already present with matching values, returns the input
    unchanged.
    """
    data = yaml.safe_load(fm_text)
    if not isinstance(data, dict):
        raise ValueError("frontmatter did not parse as a mapping")
    if (
        all(k in data for k in ("consumes", "produces", "context_rel"))
        and data.get("consumes") == row["consumes"]
        and data.get("produces") == row["produces"]
        and data.get("context_rel") == row["context_rel"]
    ):
        return fm_text, False

    items = list(data.items())
    keys = [k for k, _ in items]
    anchor = None
    for cand in (
        "hexagonal_role",
        "skill_api_version",
        "practices",
        "description",
        "name",
    ):
        if cand in keys:
            anchor = cand
            break

    new_items: list[tuple[str, object]] = []
    inserted = False
    for k, v in items:
        if k in ("consumes", "produces", "context_rel"):
            continue
        new_items.append((k, v))
        if not inserted and k == anchor:
            new_items.append(("consumes", row["consumes"]))
            new_items.append(("produces", row["produces"]))
            new_items.append(("context_rel", row["context_rel"]))
            inserted = True
    if not inserted:
        new_items.append(("consumes", row["consumes"]))
        new_items.append(("produces", row["produces"]))
        new_items.append(("context_rel", row["context_rel"]))

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
    if not new_text.endswith("\n"):
        new_text += "\n"
    return new_text, True


def list_skill_dirs() -> list[Path]:
    return sorted(
        p.parent
        for p in SKILLS_ROOT.glob("*/SKILL.md")
        if p.is_file()
    )


def process_one(
    skill_dir: Path, row: dict, dry_run: bool
) -> tuple[str, bool]:
    """Return (status, changed). status in {written, unchanged}."""
    skill_md = skill_dir / "SKILL.md"
    raw = skill_md.read_text(encoding="utf-8")
    parts = split_frontmatter(raw)
    if parts is None:
        raise SystemExit(f"{skill_md}: no YAML frontmatter detected")
    open_fence, fm_text, close_fence, body = parts
    new_fm, did_change = add_fields_to_yaml(fm_text, row)
    if did_change:
        if not dry_run:
            skill_md.write_text(
                open_fence + new_fm + close_fence + body,
                encoding="utf-8",
            )
        return "written", True
    return "unchanged", False


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument(
        "--research",
        type=Path,
        default=DEFAULT_RESEARCH,
        help="Path to the research markdown with embedded ```yaml block",
    )
    ap.add_argument(
        "--dry-run",
        action="store_true",
        help="Do not write any SKILL.md files; report what would change",
    )
    args = ap.parse_args(argv)

    table = parse_research_yaml(args.research)
    skill_dirs = list_skill_dirs()

    changed = 0
    unchanged = 0
    missing_in_research: list[str] = []
    stale_in_research: list[str] = []

    fs_slugs = {d.name for d in skill_dirs}
    for slug in table.keys():
        if slug not in fs_slugs:
            stale_in_research.append(slug)
    for slug in stale_in_research:
        print(
            f"STALE-IN-RESEARCH    {slug}  (no skills/{slug}/SKILL.md)",
            file=sys.stderr,
        )

    for d in skill_dirs:
        slug = d.name
        row = table.get(slug)
        if row is None:
            missing_in_research.append(slug)
            print(
                f"MISSING-IN-RESEARCH  {slug}  (no row in research YAML)",
                file=sys.stderr,
            )
            continue
        status, did_change = process_one(d, row, args.dry_run)
        prefix = "DRY   " if args.dry_run else ("WRITE " if did_change else "OK    ")
        print(f"{prefix} {d.relative_to(REPO_ROOT)}/SKILL.md")
        if did_change:
            changed += 1
        else:
            unchanged += 1

    print(
        f"summary: changed={changed} unchanged={unchanged} "
        f"missing-in-research={len(missing_in_research)} "
        f"stale-in-research={len(stale_in_research)}",
        file=sys.stderr,
    )
    return 1 if (missing_in_research or stale_in_research) else 0


if __name__ == "__main__":
    sys.exit(main())
