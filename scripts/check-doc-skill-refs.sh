#!/usr/bin/env bash
# check-doc-skill-refs.sh — Audit backtick-slash skill references in doctrine docs.
#
# Doctrine docs cite skills as inline code like `/plan` or `/validate --mode=pr`.
# Skills get renamed, folded, or retired, and the doc citations rot silently —
# nothing checks that a cited `/skillname` still resolves to an existing
# skills/<dir>. This checker closes that gap for the three load-bearing docs:
#
#   CLAUDE.md
#   docs/architecture/operating-loop.md
#   skills/SKILL-TIERS.md
#
# A reference is the first token after the slash inside a backtick span
# (`/deps audit` -> deps). Lines that carry a retirement marker
# (retired|folded|legacy|historical, case-insensitive) are exempt — citing a
# retired skill in a retirement note is correct, not rot.
#
# Usage:
#   bash scripts/check-doc-skill-refs.sh [--strict] [--skills-root DIR] [--docs-root DIR]
#
#   --strict        exit non-zero when any finding exists (default: advisory, exit 0)
#   --skills-root   directory holding skills/<name>/ (default: <repo>/skills)
#   --docs-root     directory the doc paths are resolved against (default: <repo>)
#   -h, --help      show this help
#
# Exit codes: 0 = clean (or advisory), 1 = findings in --strict, 2 = usage error.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

STRICT=0
SKILLS_ROOT="$ROOT/skills"
DOCS_ROOT="$ROOT"

# Docs scanned, relative to --docs-root. Missing docs are skipped (testability:
# a fixture docs-root may carry only one of them).
DOCS=(
    "CLAUDE.md"
    "docs/architecture/operating-loop.md"
    "skills/SKILL-TIERS.md"
)

# Lines carrying any of these markers are exempt from the check.
EXEMPT_RE='retired|folded|legacy|historical'

# A skill reference is a backtick, a slash, a slug, then either the closing
# backtick (`/plan`) or a space introducing args (`/deps audit`). Content with
# a second slash (`/mnt/c/...`) never matches: slugs cannot contain '/'.
# shellcheck disable=SC2016 # the backtick is a literal markdown delimiter, not a subshell
REF_RE='`/[a-z][a-z0-9_-]*[` ]'

usage() {
    cat <<'USAGE'
check-doc-skill-refs.sh — audit backtick-slash skill references in doctrine docs.

Scans CLAUDE.md, docs/architecture/operating-loop.md, and skills/SKILL-TIERS.md
for inline-code skill citations (`/skillname`) and verifies each resolves to an
existing skills/<dir>. Lines containing retired|folded|legacy|historical are
exempt (retirement notes legitimately cite gone skills).

Usage:
  bash scripts/check-doc-skill-refs.sh [--strict] [--skills-root DIR] [--docs-root DIR]

  --strict        exit non-zero when any finding exists (default: advisory, exit 0)
  --skills-root   directory holding skills/<name>/ (default: <repo>/skills)
  --docs-root     directory the doc paths are resolved against (default: <repo>)
  -h, --help      show this help

Exit codes: 0 = clean (or advisory), 1 = findings in --strict, 2 = usage error.
USAGE
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --strict) STRICT=1; shift ;;
        --skills-root) SKILLS_ROOT="${2:-}"; shift 2 ;;
        --docs-root) DOCS_ROOT="${2:-}"; shift 2 ;;
        -h|--help) usage; exit 0 ;;
        --*) echo "ERROR: unknown flag: $1 (try: bash scripts/check-doc-skill-refs.sh --help)" >&2; exit 2 ;;
        *) echo "ERROR: unexpected argument: $1 (try: bash scripts/check-doc-skill-refs.sh --help)" >&2; exit 2 ;;
    esac
done

if [[ ! -d "$SKILLS_ROOT" ]]; then
    echo "ERROR: skills root not found: $SKILLS_ROOT" >&2
    exit 1
fi
if [[ ! -d "$DOCS_ROOT" ]]; then
    echo "ERROR: docs root not found: $DOCS_ROOT" >&2
    exit 1
fi

DOCS_SCANNED=0
FINDINGS=0

for doc in "${DOCS[@]}"; do
    path="$DOCS_ROOT/$doc"
    [[ -f "$path" ]] || continue
    DOCS_SCANNED=$((DOCS_SCANNED + 1))

    while IFS= read -r hit; do
        lineno="${hit%%:*}"
        line="${hit#*:}"
        # Retirement-note exemption: the whole line is out of scope.
        if grep -qiE "$EXEMPT_RE" <<<"$line"; then
            continue
        fi
        while IFS= read -r tok; do
            slug="${tok#\`/}"     # strip leading backtick + slash
            slug="${slug%?}"      # strip trailing backtick-or-space
            if [[ ! -d "$SKILLS_ROOT/$slug" ]]; then
                echo "FINDING ${doc}:${lineno}: \`/${slug}\` does not resolve to a skill under ${SKILLS_ROOT}"
                FINDINGS=$((FINDINGS + 1))
            fi
        done < <(grep -oE "$REF_RE" <<<"$line" | sort -u)
    done < <(grep -nE "$REF_RE" "$path" || true)
done

echo "check-doc-skill-refs: ${DOCS_SCANNED} doc(s) scanned, ${FINDINGS} unresolved skill reference(s)"
if [[ "$FINDINGS" -gt 0 ]]; then
    echo "fix: point each \`/skillname\` at an existing skills/<dir>, or mark the line retired/folded/legacy/historical" >&2
fi

if [[ "$FINDINGS" -gt 0 && "$STRICT" -eq 1 ]]; then
    exit 1
fi
exit 0
