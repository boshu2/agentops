#!/usr/bin/env python3
"""ONE-SHOT MIGRATION — 2026-04-19 Jekyll → MkDocs Material link rewrite.

⚠️  DO NOT run casually on new docs. This script rewrites any cross-repo
    relative link that matches its patterns. On a post-migration docs tree,
    new legitimate relative links may be silently rewritten to absolute
    GitHub URLs with no warning.

Used once during the MkDocs Material migration to rewrite:
  - `../skills/<name>/SKILL.md`    → generated `skills/<name>.md` pages
  - `../cli/docs/COMMANDS.md`      → generated `cli/commands.md`
  - `../README.md`, `../AGENTS.md` → absolute GitHub URLs
  - various two- and three-dot climbs to repo-root meta files

Kept in the repo only for provenance and idempotency checks (`--check`).
To re-run link hygiene going forward, rely on `mkdocs build --strict` and
`tests/docs/validate-links.sh` — they catch the same class of issues
without mutating source.

Idempotent. Run: python3 scripts/migrations/2026-04-19-fix-docs-links.py [--check]
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

# This script now lives under scripts/migrations/, so climb two levels to reach the repo root.
REPO_ROOT = Path(__file__).resolve().parent.parent.parent
DOCS_DIR = REPO_ROOT / "docs"
GITHUB_BASE = "https://github.com/boshu2/agentops/blob/main"

# (regex, replacement) pairs evaluated in order. Order matters — more specific
# patterns first. All patterns match inside markdown `[label](URL)` link targets.
REWRITES: list[tuple[str, str]] = [
    # --- Three-dot climb (some files live three levels deep) ---
    (r"\]\(\.\./\.\./\.\./([^)]+?\.md)\)", rf"]({GITHUB_BASE}/\1)"),
    # --- Two-dot climb (../../..) ---
    # Files that exist inside docs/: rewrite to proper path
    (r"\]\(\.\./\.\./docs/([^)]+)\)", r"](../\1)"),
    (r"\]\(\.\./\.\./CHANGELOG\.md\)", "](../CHANGELOG.md)"),  # from subdirs -> docs/CHANGELOG.md
    (r"\]\(\.\./\.\./cli/docs/COMMANDS\.md\)", "](../cli/commands.md)"),
    (r"\]\(\.\./\.\./cli/docs/HOOKS\.md\)", "](../cli/hooks.md)"),
    (r"\]\(\.\./\.\./cli/docs/([^)]+)\)", rf"]({GITHUB_BASE}/cli/docs/\1)"),
    (r"\]\(\.\./\.\./skills/([a-zA-Z0-9_-]+)/SKILL\.md\)", r"](../skills/\1.md)"),
    (r"\]\(\.\./\.\./skills/SKILL-TIERS\.md\)", rf"]({GITHUB_BASE}/skills/SKILL-TIERS.md)"),
    (r"\]\(\.\./\.\./skills/([^)]+)\)", rf"]({GITHUB_BASE}/skills/\1)"),
    (r"\]\(\.\./\.\./README\.md\)", rf"]({GITHUB_BASE}/README.md)"),
    (r"\]\(\.\./\.\./AGENTS\.md\)", rf"]({GITHUB_BASE}/AGENTS.md)"),
    (r"\]\(\.\./\.\./PRODUCT\.md\)", rf"]({GITHUB_BASE}/PRODUCT.md)"),
    (r"\]\(\.\./\.\./GOALS\.md\)", rf"]({GITHUB_BASE}/GOALS.md)"),
    (r"\]\(\.\./\.\./SKILL-TIERS\.md\)", rf"]({GITHUB_BASE}/SKILL-TIERS.md)"),
    (r"\]\(\.\./\.\./schemas/([^)]+)\)", rf"]({GITHUB_BASE}/schemas/\1)"),
    (r"\]\(\.\./\.\./scripts/([^)]+)\)", rf"]({GITHUB_BASE}/scripts/\1)"),
    (r"\]\(\.\./\.\./hooks/([^)]+)\)", rf"]({GITHUB_BASE}/hooks/\1)"),
    (r"\]\(\.\./\.\./lib/([^)]+)\)", rf"]({GITHUB_BASE}/lib/\1)"),
    (r"\]\(\.\./\.\./bin/([^)]+)\)", rf"]({GITHUB_BASE}/bin/\1)"),
    (r"\]\(\.\./\.\./tests/([^)]+)\)", rf"]({GITHUB_BASE}/tests/\1)"),
    (r"\]\(\.\./\.\./\.github/([^)]+)\)", rf"]({GITHUB_BASE}/.github/\1)"),
    # --- Single ../ climb ---
    (r"\]\(\.\./cli/docs/COMMANDS\.md\)", "](cli/commands.md)"),
    (r"\]\(\.\./cli/docs/HOOKS\.md\)", "](cli/hooks.md)"),
    (r"\]\(\.\./cli/docs/([^)]+)\)", rf"]({GITHUB_BASE}/cli/docs/\1)"),
    # SKILLS.md uses `../<skill>/SKILL.md` to reference sibling skill SKILL.md files
    (r"\]\(\.\./([a-zA-Z0-9_-]+)/SKILL\.md\)", r"](skills/\1.md)"),
    # Skill SKILL.md pages -> generated skill pages
    (r"\]\(\.\./skills/([a-zA-Z0-9_-]+)/SKILL\.md\)", r"](skills/\1.md)"),
    # Skill subfiles (references/, templates/, scripts/, etc.) -> absolute GitHub
    (r"\]\(\.\./skills/([a-zA-Z0-9_-]+)/([^)]+)\)", rf"]({GITHUB_BASE}/skills/\1/\2)"),
    # Top-level skills entries -> absolute GitHub
    (r"\]\(\.\./skills/([^)]+)\)", rf"]({GITHUB_BASE}/skills/\1)"),
    # Repo-root meta files -> absolute GitHub
    (r"\]\(\.\./README\.md\)", rf"]({GITHUB_BASE}/README.md)"),
    (r"\]\(\.\./README\.md#([^)]+)\)", rf"]({GITHUB_BASE}/README.md#\1)"),
    (r"\]\(\.\./AGENTS\.md\)", rf"]({GITHUB_BASE}/AGENTS.md)"),
    (r"\]\(\.\./CLAUDE\.md\)", rf"]({GITHUB_BASE}/CLAUDE.md)"),
    (r"\]\(\.\./GOALS\.md\)", rf"]({GITHUB_BASE}/GOALS.md)"),
    (r"\]\(\.\./PRODUCT\.md\)", rf"]({GITHUB_BASE}/PRODUCT.md)"),
    (r"\]\(\.\./PROGRAM\.md\)", rf"]({GITHUB_BASE}/PROGRAM.md)"),
    (r"\]\(\.\./SKILL-TIERS\.md\)", rf"]({GITHUB_BASE}/SKILL-TIERS.md)"),
    # NOTE: `../CHANGELOG.md` is context-dependent:
    #   - from docs/<file>.md it points outside docs/ (invalid, rewrite to GitHub)
    #   - from docs/<dir>/<file>.md it points to docs/CHANGELOG.md (valid, leave alone)
    # So do not blanket-rewrite. Files at docs/ root are handled below via a
    # path-conditional pass.
    # Hooks, lib, scripts, bin, schemas — absolute GitHub
    (r"\]\(\.\./hooks/([^)]+)\)", rf"]({GITHUB_BASE}/hooks/\1)"),
    (r"\]\(\.\./lib/([^)]+)\)", rf"]({GITHUB_BASE}/lib/\1)"),
    (r"\]\(\.\./scripts/([^)]+)\)", rf"]({GITHUB_BASE}/scripts/\1)"),
    (r"\]\(\.\./bin/([^)]+)\)", rf"]({GITHUB_BASE}/bin/\1)"),
    (r"\]\(\.\./schemas/([^)]+)\)", rf"]({GITHUB_BASE}/schemas/\1)"),
    (r"\]\(\.\./tests/([^)]+)\)", rf"]({GITHUB_BASE}/tests/\1)"),
    (r"\]\(\.\./\.github/([^)]+)\)", rf"]({GITHUB_BASE}/.github/\1)"),
    (r"\]\(\.\./\.claude-plugin/([^)]+)\)", rf"]({GITHUB_BASE}/.claude-plugin/\1)"),
    # Bare CHANGELOG.md references inside docs/releases/ should point up to docs/CHANGELOG.md
    # (release notes historically used bare names assuming cwd was docs/)
    # This is context-sensitive — handled post-hoc in a second pass below.
    # INDEX.md references to renamed section READMEs
    (r"\]\(architecture/README\.md\)", "](architecture/index.md)"),
    (r"\]\(levels/README\.md\)", "](levels/index.md)"),
]


# Context-sensitive fixes: files in docs/releases/ use bare CHANGELOG.md
# expecting cwd to be docs/. MkDocs resolves relative to the file, so we
# rewrite those links to absolute path from docs/releases/.
RELEASES_DIR_REWRITES: list[tuple[str, str]] = [
    (r"\]\(CHANGELOG\.md\)", "](../CHANGELOG.md)"),
]

# Files at docs/ root: `../CHANGELOG.md` points outside docs/, invalid.
ROOT_DOC_REWRITES: list[tuple[str, str]] = [
    (r"\]\(\.\./CHANGELOG\.md\)", "](CHANGELOG.md)"),
]


def fix_content(text: str, rewrites: list[tuple[str, str]]) -> tuple[str, int]:
    total = 0
    for pattern, repl in rewrites:
        text, n = re.subn(pattern, repl, text)
        total += n
    return text, total


def main(argv: list[str]) -> int:
    check = "--check" in argv
    changed = 0
    total_subs = 0
    skipped = 0
    for path in DOCS_DIR.rglob("*.md"):
        if "_hooks" in path.parts:
            continue
        rewrites = list(REWRITES)
        if "releases" in path.parts:
            rewrites = RELEASES_DIR_REWRITES + rewrites
        # Only rewrite bare `../CHANGELOG.md` for files at docs/ root.
        if path.parent == DOCS_DIR:
            rewrites = rewrites + ROOT_DOC_REWRITES
        original = path.read_text(encoding="utf-8")
        fixed, n = fix_content(original, rewrites)
        if n == 0:
            skipped += 1
            continue
        total_subs += n
        if check:
            print(f"[would fix] {path.relative_to(REPO_ROOT)}: {n} substitution(s)")
            changed += 1
        elif fixed != original:
            path.write_text(fixed, encoding="utf-8")
            print(f"[fixed] {path.relative_to(REPO_ROOT)}: {n} substitution(s)")
            changed += 1
    print(f"\nTotal: {changed} file(s) touched, {total_subs} substitution(s), {skipped} unchanged")
    if check and changed > 0:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
