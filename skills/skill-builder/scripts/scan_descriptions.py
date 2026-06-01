#!/usr/bin/env python3
"""Corpus-wide skill description trigger scanner.

The per-skill `skill-auditor` runs `description-has-triggers` / `trigger-clarity`
as WARN checks, so a missing trigger phrase never blocks a merge and the gap
accumulates silently across the corpus. This scanner is the corpus-wide
companion: it walks every `skills/*/SKILL.md`, applies the *same* three-form
trigger detection as `skill-auditor/scripts/audit.sh`, scores each description,
and emits a prioritized remediation list with a suggested `Triggers:` stub for
each skill that lacks one.

Discovery in the runtime is pure LLM reasoning over the `description` field, so
a missing trigger phrase is a material skill-selection risk, not cosmetic. See
`skills/skill-builder/references/skill-authoring-standard.md`.

Usage:
    python3 scan_descriptions.py [SKILLS_DIR] [--json] [--strict] [--quiet]

Exit codes:
    0  every description carries a trigger (or --strict not set)
    1  one or more descriptions lack a trigger AND --strict is set
    2  usage error (skills dir not found)
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path

# Phrases that count as an explicit trigger marker, mirroring the regex in
# skill-auditor/scripts/audit.sh (Form b). Keep these in sync with the auditor.
TRIGGER_MARKERS = (
    "**Use when:",
    "**Triggers:",
    "**Perfect for:",
    "Use when:",
    "Triggers:",
)

# Stop-words stripped when deriving a suggested trigger stub from the name.
_STOPWORDS = frozenset({"the", "a", "an", "for", "and", "to", "of", "with"})


@dataclass
class SkillScan:
    """Result of scanning one SKILL.md for trigger quality."""

    name: str
    path: Path
    description: str
    has_trigger: bool
    forms: list[str] = field(default_factory=list)
    score: int = 0
    suggestion: str = ""

    def to_dict(self) -> dict:
        """Return a JSON-serializable view for --json / robot mode."""
        return {
            "name": self.name,
            "path": str(self.path),
            "has_trigger": self.has_trigger,
            "forms": self.forms,
            "score": self.score,
            "suggestion": self.suggestion,
        }


def split_frontmatter(text: str) -> tuple[str, str]:
    """Split a SKILL.md into (frontmatter, body). Empty frontmatter if absent."""
    if not text.startswith("---"):
        return "", text
    parts = text.split("\n---", 1)
    if len(parts) != 2:
        return "", text
    frontmatter = parts[0][len("---") :]
    body = parts[1].lstrip("-\n")
    return frontmatter, body


def parse_field(frontmatter: str, key: str) -> str:
    """Extract a single top-level scalar field's first line from frontmatter."""
    match = re.search(rf"^{re.escape(key)}:\s*(.*)$", frontmatter, re.MULTILINE)
    return match.group(1).strip() if match else ""


def description_block(frontmatter: str) -> str:
    """Return the full description value, including folded/literal continuations."""
    lines = frontmatter.splitlines()
    out: list[str] = []
    capturing = False
    for line in lines:
        if line.startswith("description:"):
            capturing = True
            out.append(line)
            continue
        if capturing:
            # A new top-level key (no leading whitespace, ends the block).
            if re.match(r"^[A-Za-z_-]+:", line):
                break
            out.append(line)
    return "\n".join(out)


def count_trigger_list(frontmatter: str) -> int:
    """Count items under a `metadata.triggers:` (or `triggers:`) YAML list."""
    lines = frontmatter.splitlines()
    in_list = False
    count = 0
    for line in lines:
        if re.match(r"^\s+triggers:\s*$", line):
            in_list = True
            continue
        if in_list:
            if re.match(r"^\s+-\s+", line):
                count += 1
                continue
            if re.match(r"^\s*[A-Za-z_-]+:", line):
                break
    return count


def detect_trigger(text: str, frontmatter: str) -> list[str]:
    """Return the trigger forms present, matching audit.sh's three forms.

    Form a: `description: |` literal block scalar.
    Form b: an explicit marker (`Use when:` / `Triggers:` / `Perfect for:`)
            anywhere in the file, mirroring audit.sh's whole-file grep.
    Form c: a `triggers:` YAML list with three or more items.
    """
    forms: list[str] = []
    desc_value = parse_field(frontmatter, "description")
    if desc_value.startswith("|"):
        forms.append("block-scalar")
    if any(marker in text for marker in TRIGGER_MARKERS):
        forms.append("explicit-marker")
    if count_trigger_list(frontmatter) >= 3:
        forms.append("triggers-list")
    return forms


def score_trigger(description: str) -> int:
    """Score 0-3, mirroring skill-auditor/scripts/score_agentops_skill.py."""
    signals = sum(
        marker.lower().strip("*").rstrip(":") in description.lower()
        for marker in ("Use when", "Triggers", "Perfect for")
    )
    return min(3, int(bool(description.strip())) + signals)


def suggest_triggers(name: str, description: str) -> str:
    """Derive a deterministic `Triggers:` stub from the skill name + first verb."""
    tokens = [t for t in name.split("-") if t not in _STOPWORDS]
    spaced = " ".join(tokens)
    first_sentence = re.split(r"[.\n]", description.strip(), maxsplit=1)[0]
    words = first_sentence.split()
    verb = words[0].lower().strip("'\"") if words else ""
    candidates = [name, spaced]
    # Only add a verb phrase when the verb adds a word not already in the name.
    if verb and tokens and verb not in tokens:
        candidates.append(f"{verb} {tokens[-1]}")
    seen: list[str] = []
    for phrase in candidates:
        cleaned = " ".join(dict.fromkeys(phrase.strip().lower().split()))
        if cleaned and cleaned not in seen:
            seen.append(cleaned)
    quoted = ", ".join(f'"{p}"' for p in seen)
    return f"Triggers: {quoted}"


def scan_skill(skill_md: Path) -> SkillScan | None:
    """Scan one SKILL.md. Returns None if the file is unreadable/empty."""
    try:
        text = skill_md.read_text(encoding="utf-8")
    except OSError:
        return None
    frontmatter, _body = split_frontmatter(text)
    name = parse_field(frontmatter, "name") or skill_md.parent.name
    description = description_block(frontmatter)
    forms = detect_trigger(text, frontmatter)
    has_trigger = bool(forms)
    scan = SkillScan(
        name=name,
        path=skill_md,
        description=description,
        has_trigger=has_trigger,
        forms=forms,
        score=score_trigger(description),
    )
    if not has_trigger:
        scan.suggestion = suggest_triggers(name, parse_field(frontmatter, "description"))
    return scan


def scan_corpus(skills_dir: Path) -> list[SkillScan]:
    """Scan every `<skill>/SKILL.md` under skills_dir, sorted by name."""
    results: list[SkillScan] = []
    for skill_md in sorted(skills_dir.glob("*/SKILL.md")):
        scan = scan_skill(skill_md)
        if scan is not None:
            results.append(scan)
    return results


def render_markdown(results: list[SkillScan]) -> str:
    """Render a human-readable remediation report."""
    total = len(results)
    missing = [r for r in results if not r.has_trigger]
    lines = [
        "# Skill description trigger scan",
        "",
        f"- Skills scanned: **{total}**",
        f"- With trigger marker: **{total - len(missing)}**",
        f"- Missing trigger marker: **{len(missing)}** "
        f"({(len(missing) / total * 100):.0f}%)" if total else "- Missing: 0",
        "",
    ]
    if not missing:
        lines.append("All descriptions carry a trigger marker. ✅")
        return "\n".join(lines)
    lines += [
        "## Remediation backlog (add a trigger marker to each)",
        "",
        "| Skill | Score | Suggested stub |",
        "|-------|-------|----------------|",
    ]
    for r in missing:
        lines.append(f"| `{r.name}` | {r.score}/3 | `{r.suggestion}` |")
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    """CLI entry point."""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "skills_dir",
        nargs="?",
        default="skills",
        help="Path to the skills/ directory (default: skills)",
    )
    parser.add_argument("--json", action="store_true", help="Emit JSON (robot mode)")
    parser.add_argument(
        "--strict", action="store_true", help="Exit 1 if any description lacks a trigger"
    )
    parser.add_argument("--quiet", action="store_true", help="Suppress the human report")
    args = parser.parse_args(argv)

    skills_dir = Path(args.skills_dir)
    if not skills_dir.is_dir():
        print(f"error: skills dir not found: {skills_dir}", file=sys.stderr)
        return 2

    results = scan_corpus(skills_dir)
    missing = [r for r in results if not r.has_trigger]

    if args.json:
        payload = {
            "scanned": len(results),
            "missing": len(missing),
            "skills": [r.to_dict() for r in results],
        }
        print(json.dumps(payload, indent=2))
    elif not args.quiet:
        print(render_markdown(results))

    return 1 if (args.strict and missing) else 0


if __name__ == "__main__":
    sys.exit(main())
